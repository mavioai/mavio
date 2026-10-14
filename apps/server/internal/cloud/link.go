package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// Defaults of the Link.
const (
	// SyncInterval is how often a connected server fetches its changes
	// without a notice.
	SyncInterval = 6 * time.Hour
	// PingInterval is how often the server pings mavio.dev.
	PingInterval = 30 * time.Second
	// LibrariesInterval is how often the server checks whether its
	// libraries changed since it published them.
	LibrariesInterval = time.Minute
	// changesPage is how many changes the server fetches at a time.
	changesPage = 500
)

// Link keeps a claimed server's connection to mavio.dev and applies the
// changes mavio.dev has for it, after the cursor in its account
// settings: when it connects, when mavio.dev notices a later cursor, and
// every SyncInterval.
type Link struct {
	Store  core.Store
	Client *Client
	// Apply applies one change in the transaction that advances the
	// cursor; nil applies nothing.
	Apply  func(ctx context.Context, tx core.Store, c *cloudv1.Change) error
	Logger *slog.Logger
	// MinBackoff and MaxBackoff bound the wait before reconnecting,
	// doubled after every failure, with jitter; zero uses a second and
	// five minutes.
	MinBackoff, MaxBackoff time.Duration
	// SyncInterval, PingInterval and LibrariesInterval override the
	// defaults when not zero.
	SyncInterval, PingInterval, LibrariesInterval time.Duration

	wake chan struct{}
}

// NewLink returns a link of store to mavio.dev through client.
func NewLink(store core.Store, client *Client, logger *slog.Logger) *Link {
	if logger == nil {
		logger = slog.Default()
	}
	return &Link{Store: store, Client: client, Logger: logger, wake: make(chan struct{}, 1)}
}

// Wake has the link check again whether the server is claimed.
func (l *Link) Wake() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// Run keeps the link until ctx ends, while the server is claimed,
// reconnecting with exponential backoff and jitter.
func (l *Link) Run(ctx context.Context) {
	minBackoff, maxBackoff := or(l.MinBackoff, time.Second), or(l.MaxBackoff, 5*time.Minute)
	backoff := minBackoff
	for ctx.Err() == nil {
		settings, err := l.Store.Settings().Accounts(ctx)
		if err != nil {
			l.Logger.ErrorContext(ctx, "read the account settings", "err", err)
		} else if !settings.Claimed() {
			select {
			case <-ctx.Done():
			case <-l.wake:
			}
			continue
		}
		if err == nil {
			var synced bool
			synced, err = l.connect(ctx)
			if synced {
				backoff = minBackoff
			}
			if ctx.Err() != nil {
				return
			}
			l.Logger.WarnContext(ctx, "lost the connection to mavio.dev", "err", err)
		}
		wait := backoff/2 + rand.N(backoff/2+1)
		backoff = min(2*backoff, maxBackoff)
		select {
		case <-ctx.Done():
		case <-time.After(wait):
		}
	}
}

// connect holds one connection until it fails, reporting whether it
// fetched the changes.
func (l *Link) connect(ctx context.Context) (synced bool, err error) {
	conn, err := l.Client.Notices(ctx)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	notices := make(chan int64, 1)
	go l.receive(ctx, conn, notices, cancel)
	go l.ping(ctx, conn, cancel)

	cursor, err := l.Sync(ctx)
	if err != nil {
		return false, err
	}
	l.Logger.InfoContext(ctx, "connected to mavio.dev", "cursor", cursor)
	published, err := l.publish(ctx, "")
	if err != nil {
		return true, err
	}
	ticker := time.NewTicker(or(l.SyncInterval, SyncInterval))
	defer ticker.Stop()
	libraries := time.NewTicker(or(l.LibrariesInterval, LibrariesInterval))
	defer libraries.Stop()
	for {
		select {
		case <-ctx.Done():
			return true, context.Cause(ctx)
		case n := <-notices:
			if n <= cursor {
				continue
			}
		case <-ticker.C:
		case <-libraries.C:
			if published, err = l.publish(ctx, published); err != nil {
				return true, err
			}
			continue
		}
		if cursor, err = l.Sync(ctx); err != nil {
			return true, err
		}
	}
}

// receive passes on the cursors of the notices on conn, keeping only the
// latest one not yet taken.
func (l *Link) receive(ctx context.Context, conn *websocket.Conn, notices chan int64, cancel context.CancelCauseFunc) {
	for {
		var frame []byte
		if err := websocket.Message.Receive(conn, &frame); err != nil {
			cancel(fmt.Errorf("receive a notice: %w", err))
			return
		}
		var notice cloudv1.Notice
		if err := proto.Unmarshal(frame, &notice); err != nil {
			l.Logger.WarnContext(ctx, "ignored a notice from mavio.dev", "err", err)
			continue
		}
		select {
		case <-notices:
		default:
		}
		notices <- notice.GetCursor()
	}
}

// ping pings mavio.dev, whose pongs the connection consumes, so that a
// dead connection fails a write.
func (l *Link) ping(ctx context.Context, conn *websocket.Conn, cancel context.CancelCauseFunc) {
	ticker := time.NewTicker(or(l.PingInterval, PingInterval))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		conn.PayloadType = websocket.PingFrame
		if _, err := conn.Write(nil); err != nil {
			cancel(fmt.Errorf("ping mavio.dev: %w", err))
			return
		}
	}
}

// Sync applies the server's changes after its cursor and returns the new
// cursor.
func (l *Link) Sync(ctx context.Context) (int64, error) {
	for {
		settings, err := l.Store.Settings().Accounts(ctx)
		if err != nil {
			return 0, err
		}
		if !settings.Claimed() {
			return 0, errors.New("the server is no longer claimed")
		}
		after := settings.Cursor
		resp, err := l.Client.ListChanges(ctx, cloudv1.ListChangesRequest_builder{After: &after, PageSize: proto.Int32(changesPage)}.Build())
		if err != nil {
			return 0, fmt.Errorf("fetch the changes: %w", err)
		}
		err = l.Store.InTx(ctx, func(tx core.Store) error {
			settings, err := tx.Settings().Accounts(ctx)
			if err != nil {
				return err
			}
			if settings.Cursor != after {
				return errors.New("the cursor moved while fetching the changes")
			}
			for _, c := range resp.GetChanges() {
				if l.Apply != nil {
					if err := l.Apply(ctx, tx, c); err != nil {
						return fmt.Errorf("apply change %d: %w", c.GetCursor(), err)
					}
				}
			}
			settings.Cursor = max(after, resp.GetCursor())
			return tx.Settings().PutAccounts(ctx, &settings)
		})
		if err != nil {
			return 0, err
		}
		if !resp.GetMore() {
			return max(after, resp.GetCursor()), nil
		}
	}
}

// publish publishes the server's libraries unless they are those last
// published, whose fingerprint it is given and returns.
func (l *Link) publish(ctx context.Context, last string) (string, error) {
	libs, err := l.Store.Libraries().List(ctx)
	if err != nil {
		return last, err
	}
	out := make([]*cloudv1.Library, len(libs))
	h := sha256.New()
	for i, lib := range libs {
		id, kind := lib.ID.String(), string(lib.Kind)
		out[i] = cloudv1.Library_builder{Id: &id, Name: &lib.Name, Kind: &kind}.Build()
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", id, lib.Name, kind)
	}
	fingerprint := hex.EncodeToString(h.Sum(nil))
	if fingerprint == last {
		return last, nil
	}
	if _, err := l.Client.PublishLibraries(ctx, cloudv1.PublishLibrariesRequest_builder{Libraries: out}.Build()); err != nil {
		return last, fmt.Errorf("publish the libraries: %w", err)
	}
	return fingerprint, nil
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

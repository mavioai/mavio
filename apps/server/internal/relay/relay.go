package relay

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// The protocol with the relay, as apps/relay serves it.
const (
	connectPath      = "/servers/connect"
	acceptPath       = "/servers/accept"
	protocol         = "mavio-relay"
	identityHeader   = "Mavio-Identity-Key"
	connectionHeader = "Mavio-Relay-Connection"
	frameSize        = 16
	// heartbeatTimeout is how long the control connection may stay silent;
	// the relay writes a heartbeat every 30 seconds.
	heartbeatTimeout = 90 * time.Second
)

// MavioDev is what the client calls mavio.dev for.
type MavioDev interface {
	ListRelays(ctx context.Context) ([]*cloudv1.Relay, error)
	SetRelayRegion(ctx context.Context, region string) error
}

// Client keeps the server's connection to a relay and hands the
// connections it relays to Listener.
type Client struct {
	Key      *identity.Key
	MavioDev MavioDev
	// Allowed reports whether the server may use the relay: claimed, with
	// relaying enabled in its settings and its certificate ready.
	Allowed func(ctx context.Context) (bool, error)
	Logger  *slog.Logger
	// TLSConfig, when set, is the base of the TLS configuration relays are
	// dialed with.
	TLSConfig *tls.Config
	// MinRetry and MaxRetry bound the wait before connecting again after a
	// failure; zero is 5 seconds and 10 minutes.
	MinRetry, MaxRetry time.Duration

	wake  chan struct{}
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

// NewClient returns a client for the server with an identity key.
// A nil logger uses slog.Default.
func NewClient(key *identity.Key, mavioDev MavioDev, allowed func(context.Context) (bool, error), logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		Key: key, MavioDev: mavioDev, Allowed: allowed, Logger: logger,
		wake: make(chan struct{}, 1), conns: make(chan net.Conn), done: make(chan struct{}),
	}
}

// Wake has the client check again whether it may use the relay, as after
// the server is claimed, its settings change or its certificate comes.
func (c *Client) Wake() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Run keeps a connection to the nearest relay while the server may use
// one, until ctx ends.
func (c *Client) Run(ctx context.Context) {
	minRetry, maxRetry := or(c.MinRetry, 5*time.Second), or(c.MaxRetry, 10*time.Minute)
	retry := minRetry
	recorded := false
	for ctx.Err() == nil {
		allowed, err := c.Allowed(ctx)
		if err != nil {
			c.Logger.ErrorContext(ctx, "relay: read the settings", "err", err)
			c.sleep(ctx, minRetry)
			continue
		}
		if !allowed {
			if recorded {
				if err := c.MavioDev.SetRelayRegion(ctx, ""); err != nil {
					c.Logger.WarnContext(ctx, "relay: clear the region at mavio.dev", "err", err)
					c.sleep(ctx, minRetry)
					continue
				}
				recorded = false
			}
			select {
			case <-ctx.Done():
			case <-c.wake:
			}
			continue
		}
		connected, err := c.session(ctx, &recorded)
		if ctx.Err() != nil {
			return
		}
		if connected {
			retry = minRetry
		}
		if err != nil {
			c.Logger.WarnContext(ctx, "relay connection", "err", err)
			c.sleep(ctx, retry/2+rand.N(retry/2+1))
			retry = min(2*retry, maxRetry)
		}
	}
}

// session holds one control connection until it fails or the relay is no
// longer allowed, reporting whether it connected.
func (c *Client) session(ctx context.Context, recorded *bool) (bool, error) {
	relays, err := c.MavioDev.ListRelays(ctx)
	if err != nil {
		return false, fmt.Errorf("list relays: %w", err)
	}
	r, err := c.nearest(ctx, relays)
	if err != nil {
		return false, err
	}
	conn, br, err := c.open(ctx, r.GetUrl(), connectPath, "")
	if err != nil {
		return false, fmt.Errorf("connect to the relay in %s: %w", r.GetRegion(), err)
	}
	defer conn.Close()
	if err := c.MavioDev.SetRelayRegion(ctx, r.GetRegion()); err != nil {
		return false, fmt.Errorf("record the relay region: %w", err)
	}
	*recorded = true
	c.Logger.InfoContext(ctx, "connected to the relay", "region", r.GetRegion())

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Close the connection when ctx ends or the relay is no longer allowed.
	go func() {
		for {
			select {
			case <-sctx.Done():
				_ = conn.Close()
				return
			case <-c.wake:
				if ok, err := c.Allowed(sctx); err == nil && !ok {
					cancel()
				}
			}
		}
	}()
	frame := make([]byte, frameSize)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(heartbeatTimeout))
		if _, err := io.ReadFull(br, frame); err != nil {
			if sctx.Err() != nil {
				return true, nil
			}
			return true, fmt.Errorf("relay in %s: %w", r.GetRegion(), err)
		}
		if allZero(frame) {
			continue
		}
		go c.accept(sctx, r.GetUrl(), hex.EncodeToString(frame))
	}
}

// accept opens the connection back for a client the relay announced.
func (c *Client) accept(ctx context.Context, base, id string) {
	conn, br, err := c.open(ctx, base, acceptPath, id)
	if err != nil {
		c.Logger.WarnContext(ctx, "relay: accept a connection", "err", err)
		return
	}
	_ = conn.SetDeadline(time.Time{})
	select {
	case c.conns <- &relayed{Conn: conn, r: br}:
	case <-ctx.Done():
		_ = conn.Close()
	case <-c.done:
		_ = conn.Close()
	}
}

// open upgrades a connection to a relay's path.
func (c *Client) open(ctx context.Context, base, path, id string) (net.Conn, *bufio.Reader, error) {
	u, err := url.Parse(strings.TrimSuffix(base, "/") + path)
	if err != nil {
		return nil, nil, err
	}
	conn, err := c.dial(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	// The handshake is bounded; the connection then lives on.
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", protocol)
	req.Header.Set("Authorization", "Bearer "+cloud.Token(c.Key, strings.TrimSuffix(base, "/"), time.Now()))
	req.Header.Set(identityHeader, base64.RawURLEncoding.EncodeToString(c.Key.Public()))
	if id != "" {
		req.Header.Set(connectionHeader, id)
	}
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = conn.Close()
		return nil, nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return conn, br, nil
}

func (c *Client) dial(ctx context.Context, u *url.URL) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second}
	switch u.Scheme {
	case "https":
		cfg := &tls.Config{}
		if c.TLSConfig != nil {
			cfg = c.TLSConfig.Clone()
		}
		cfg.ServerName, cfg.NextProtos = u.Hostname(), []string{"http/1.1"}
		return (&tls.Dialer{NetDialer: d, Config: cfg}).DialContext(ctx, "tcp", hostPort(u))
	case "http":
		return d.DialContext(ctx, "tcp", hostPort(u))
	default:
		return nil, fmt.Errorf("relay URL %s is neither HTTPS nor HTTP", u)
	}
}

// nearest returns the relay that accepts a TCP connection soonest.
func (c *Client) nearest(ctx context.Context, relays []*cloudv1.Relay) (*cloudv1.Relay, error) {
	if len(relays) == 0 {
		return nil, errors.New("mavio.dev has no relays")
	}
	if len(relays) == 1 {
		return relays[0], nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	first := make(chan *cloudv1.Relay, len(relays))
	for _, r := range relays {
		go func() {
			u, err := url.Parse(r.GetUrl())
			if err != nil {
				first <- nil
				return
			}
			var d net.Dialer
			conn, err := d.DialContext(ctx, "tcp", hostPort(u))
			if err != nil {
				first <- nil
				return
			}
			_ = conn.Close()
			first <- r
		}()
	}
	for range relays {
		if r := <-first; r != nil {
			return r, nil
		}
	}
	return nil, errors.New("no relay is reachable")
}

func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	if u.Scheme == "http" {
		return net.JoinHostPort(u.Hostname(), "80")
	}
	return net.JoinHostPort(u.Hostname(), "443")
}

// Listener returns the connections the relay hands the server: each
// starts with the client's TLS ClientHello.
func (c *Client) Listener() net.Listener { return listener{c} }

type listener struct{ c *Client }

func (l listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.c.conns:
		return conn, nil
	case <-l.c.done:
		return nil, net.ErrClosed
	}
}

func (l listener) Close() error {
	l.c.close()
	return nil
}

func (l listener) Addr() net.Addr { return addr{} }

type addr struct{}

func (addr) Network() string { return "relay" }
func (addr) String() string  { return "relay" }

func (c *Client) close() { c.once.Do(func() { close(c.done) }) }

// relayed is a connection the relay handed over, whose first bytes may
// be buffered.
type relayed struct {
	net.Conn
	r *bufio.Reader
}

func (c *relayed) Read(p []byte) (int, error) { return c.r.Read(p) }

func (c *Client) sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	case <-c.wake:
	}
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

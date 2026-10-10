package warming

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

// Defaults of a Warmer.
const (
	// DefaultPresence is how recently a client must have made a request
	// to count as in use; an open event stream alone does not, so that a
	// client left on overnight lets the disks sleep.
	DefaultPresence = 5 * time.Minute
	// DefaultRecent is how many items in progress of each user are kept
	// awake, about a home screen's row.
	DefaultRecent = 8
	// DefaultRefresh is how often the media kept awake is looked up.
	DefaultRefresh = 30 * time.Second
)

// Config configures a Warmer.
type Config struct {
	Store  core.Store
	Keeper *storage.Keeper
	// Online lists the sessions with an event stream open, by user.
	Online func() map[core.ID][]core.ID
	// Presence, Recent and Refresh default to DefaultPresence,
	// DefaultRecent and DefaultRefresh.
	Presence time.Duration
	Recent   int
	Refresh  time.Duration
	// Now returns the current time; default time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}

// Warmer keeps the volumes of clients' media in progress awake.
type Warmer struct {
	cfg Config
	log *slog.Logger

	// paths are the media kept awake, looked up at refreshed.
	paths     []string
	refreshed time.Time
}

// New returns a warmer.
func New(cfg Config) *Warmer {
	if cfg.Presence <= 0 {
		cfg.Presence = DefaultPresence
	}
	if cfg.Recent <= 0 {
		cfg.Recent = DefaultRecent
	}
	if cfg.Refresh <= 0 {
		cfg.Refresh = DefaultRefresh
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Warmer{cfg: cfg, log: log}
}

// Run beats the volumes of the media in progress of the users with a
// client in use every heartbeat interval until ctx is canceled.
func (w *Warmer) Run(ctx context.Context) error {
	interval := w.cfg.Keeper.Interval
	if interval <= 0 {
		interval = storage.DefaultHeartbeatInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-t.C:
			w.Tick(ctx)
		}
	}
}

// Tick looks up the media to keep awake when due, and beats its volumes.
func (w *Warmer) Tick(ctx context.Context) {
	if now := w.cfg.Now(); now.Sub(w.refreshed) >= w.cfg.Refresh {
		paths, err := w.lookUp(ctx, now)
		if err != nil {
			w.log.WarnContext(ctx, "look up media to keep awake", "err", err)
		}
		w.paths, w.refreshed = paths, now
	}
	if len(w.paths) > 0 {
		w.cfg.Keeper.Beat(w.paths...)
	}
}

// lookUp returns the files of the items in progress of the users with a
// client in use: one with an event stream open that made a request within
// Presence.
func (w *Warmer) lookUp(ctx context.Context, now time.Time) ([]string, error) {
	var paths []string
	seen := map[string]bool{}
	for userID, online := range w.cfg.Online() {
		sessions, err := w.cfg.Store.AuthSessions().ListForUser(ctx, userID)
		if err != nil {
			return paths, err
		}
		if !slices.ContainsFunc(sessions, func(s core.AuthSession) bool {
			return slices.Contains(online, s.ID) && now.Sub(s.LastSeenAt) <= w.cfg.Presence
		}) {
			continue
		}
		page, err := w.cfg.Store.Items().Query(ctx, core.ItemQuery{
			UserID: userID, Resumable: true, Sort: []core.SortSpec{{Field: core.SortLastPlayed, Desc: true}}, Limit: w.cfg.Recent,
		})
		if err != nil {
			return paths, err
		}
		for _, it := range page.Items {
			sources, err := w.cfg.Store.MediaSources().ListForItem(ctx, it.ID)
			if err != nil {
				return paths, err
			}
			for _, ms := range sources {
				if ms.Path != "" && ms.Disc == "" && !seen[ms.Path] {
					seen[ms.Path] = true
					paths = append(paths, ms.Path)
				}
			}
		}
	}
	return paths, nil
}

// Wake wakes the volume of an item's media, which a client opened and may
// play next, and warms the media's container header and index. It returns
// at once: finding the volume of a sleeping share may take a while.
func (w *Warmer) Wake(sources []core.MediaSource) {
	var paths []string
	for _, ms := range sources {
		if ms.Path != "" && ms.Disc == "" {
			paths = append(paths, ms.Path)
		}
	}
	if len(paths) == 0 {
		return
	}
	go func() {
		for _, p := range paths {
			w.cfg.Keeper.Wake(p)
		}
	}()
}

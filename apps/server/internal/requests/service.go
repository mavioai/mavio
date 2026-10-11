package requests

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// Errors of requests and issues, besides the core errors: core.ErrConflict
// for titles requested, wanted or available already, core.ErrNotFound for
// unknown titles, requests and issues.
var (
	// ErrPermission is a user lacking the permission to do something.
	ErrPermission = errors.New("not permitted")
	// ErrRating is a title rated above the user's limit.
	ErrRating = errors.New("rated above the user's limit")
	// ErrQuota is a request past the user's quota.
	ErrQuota = errors.New("quota exceeded")
	// ErrState is a request or issue whose status does not allow a change,
	// such as approving a request that is not pending.
	ErrState = errors.New("not allowed in this status")
)

// Acquirer makes movies and series wanted: the acquisition runtime.
type Acquirer interface {
	AddMovie(ctx context.Context, n acquisition.NewMovie) (core.Item, core.Wanted, error)
	AddSeries(ctx context.Context, n acquisition.NewSeries) (core.Item, core.Wanted, error)
	SetMonitored(ctx context.Context, itemIDs []core.ID, monitored bool) error
	QueueSearch(ctx context.Context, itemIDs []core.ID, userInvoked bool) (int, error)
	QueueEpisodeSearch(ctx context.Context, p acquisition.EpisodeSearchPayload) (int, error)
}

// Recorder records activities: activity.Log.
type Recorder interface {
	Record(ctx context.Context, a core.Activity)
}

// Service carries out requests, issues and discovery.
type Service struct {
	Store       core.Store
	Acquisition Acquirer
	// Settings returns the current request settings; nil uses the
	// defaults.
	Settings func() core.RequestSettings
	// Metadata returns the metadata providers titles are looked up with,
	// for their names, ratings and TVDB IDs; Episodes the episode listers
	// listing series' seasons. Nil returns none.
	Metadata func() []library.Provider
	Episodes func() []library.EpisodeLister
	// Discovery returns the discovery providers; nil returns none.
	Discovery func() []DiscoveryProvider
	// Activity records notifications; nil records none.
	Activity Recorder
	Now      func() time.Time
	Logger   *slog.Logger

	// users serializes each user's requests, for quotas; titles each
	// title's requests and approvals, for duplicates. A user's lock is
	// taken before a title's.
	users, titles keyLocks
	cache         titleCache
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (s *Service) settings() core.RequestSettings {
	if s.Settings == nil {
		return core.DefaultRequestSettings()
	}
	return s.Settings()
}

func (s *Service) record(ctx context.Context, a core.Activity) {
	if s.Activity != nil {
		s.Activity.Record(ctx, a)
	}
}

// keyLocks are mutexes by key, kept while held.
type keyLocks struct {
	mu   sync.Mutex
	held map[string]*keyLock
}

type keyLock struct {
	mu      sync.Mutex
	waiters int
}

// lock locks a key and returns its unlock.
func (k *keyLocks) lock(key string) func() {
	k.mu.Lock()
	if k.held == nil {
		k.held = map[string]*keyLock{}
	}
	l := k.held[key]
	if l == nil {
		l = &keyLock{}
		k.held[key] = l
	}
	l.waiters++
	k.mu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		k.mu.Lock()
		if l.waiters--; l.waiters == 0 {
			delete(k.held, key)
		}
		k.mu.Unlock()
	}
}

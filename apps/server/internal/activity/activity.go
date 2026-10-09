// Package activity records what happens on the server in the activity log
// and sends each activity to the notification plugins.
package activity

import (
	"context"
	"log/slog"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Notifier delivers activities elsewhere, such as a notification plugin.
type Notifier interface {
	Name() string
	Notify(ctx context.Context, a core.Activity) error
}

// Config configures a Log.
type Config struct {
	Store core.Store
	// Notifiers returns the notifiers to send to now; nil sends to none.
	Notifiers func() []Notifier
	Logger    *slog.Logger
	// Retention is how long activities are kept; zero keeps them for 90
	// days.
	Retention time.Duration
}

// Log records activities.
type Log struct {
	cfg   Config
	log   *slog.Logger
	queue chan core.Activity
}

// queueSize bounds the activities waiting for notifiers.
const queueSize = 256

// New returns a log; Run must run for notifiers to be told.
func New(cfg Config) *Log {
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 90 * 24 * time.Hour
	}
	return &Log{cfg: cfg, log: log, queue: make(chan core.Activity, queueSize)}
}

// Record stores an activity and queues it for the notifiers. Failing to
// record is logged, never returned: what happened has happened.
func (l *Log) Record(ctx context.Context, a core.Activity) {
	if l == nil {
		return
	}
	if a.Severity == "" {
		a.Severity = core.SeverityInfo
	}
	if err := l.cfg.Store.Activities().Add(context.WithoutCancel(ctx), &a); err != nil {
		l.log.ErrorContext(ctx, "record activity", "type", a.Type, "err", err)
		return
	}
	select {
	case l.queue <- a:
	default:
		l.log.WarnContext(ctx, "notifiers behind; activity not sent", "type", a.Type)
	}
}

// Run sends queued activities to the notifiers and purges old ones daily,
// until ctx ends.
func (l *Log) Run(ctx context.Context) error {
	purge := time.NewTicker(24 * time.Hour)
	defer purge.Stop()
	l.purge(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-purge.C:
			l.purge(ctx)
		case a := <-l.queue:
			l.notify(ctx, a)
		}
	}
}

func (l *Log) purge(ctx context.Context) {
	if n, err := l.cfg.Store.Activities().Purge(ctx, time.Now().Add(-l.cfg.Retention)); err != nil {
		l.log.ErrorContext(ctx, "purge activities", "err", err)
	} else if n > 0 {
		l.log.InfoContext(ctx, "purged old activities", "count", n)
	}
}

func (l *Log) notify(ctx context.Context, a core.Activity) {
	if l.cfg.Notifiers == nil {
		return
	}
	for _, n := range l.cfg.Notifiers() {
		nctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := n.Notify(nctx, a); err != nil {
			l.log.WarnContext(ctx, "notifier failed", "notifier", n.Name(), "activity", a.Type, "err", err)
		}
		cancel()
	}
}

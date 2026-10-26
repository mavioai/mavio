package streaming

import (
	"context"
	"log/slog"
	"time"
)

// slowRequest is how long a request takes before its trace is logged at
// info rather than debug level.
const slowRequest = 5 * time.Second

// trace records what a segment or subtitle request waited for and how
// ffmpeg's run was chosen for it, for the log: a request that takes long
// tells whether it waited for another request, for a run going on, or for
// a run it started.
type trace struct {
	kind    string
	index   int
	begun   time.Time
	lockFor time.Duration
	// decision is "written", "restart" or "wait"; the run's state is
	// that seen when deciding.
	decision string
	runFirst int
	reached  time.Duration
	running  bool
}

func (s *Stream) trace(kind string, index int) *trace {
	return &trace{kind: kind, index: index, begun: time.Now(), runFirst: -1}
}

// locked records how long the request waited for the one before it.
func (t *trace) locked() { t.lockFor = time.Since(t.begun) }

// decide reports whether ffmpeg must start anew for segment index, as
// [Stream.restart], and records why.
func (t *trace) decide(s *Stream, index int) bool {
	if s.job != nil {
		t.runFirst = s.job.first
		t.running = !s.ended()
		if t.running {
			t.reached = s.reached()
		}
	}
	restart := s.restart(index)
	t.decision = "wait"
	if restart {
		t.decision = "restart"
	}
	return restart
}

// done logs the request's trace, at info level when it was slow or
// failed for another reason than the client leaving.
func (s *Stream) done(ctx context.Context, t *trace, err error) {
	took := time.Since(t.begun)
	level := slog.LevelDebug
	if took >= slowRequest || (err != nil && ctx.Err() == nil) {
		level = slog.LevelInfo
	}
	if !s.cfg.Logger.Enabled(ctx, level) {
		return
	}
	attrs := []slog.Attr{
		slog.String("kind", t.kind),
		slog.Int("segment", t.index),
		slog.Duration("took", took),
		slog.Duration("lock_wait", t.lockFor),
		slog.String("decision", t.decision),
	}
	if t.index >= 0 && t.index < len(s.cfg.Layout.Segments) {
		attrs = append(attrs, slog.Duration("start", s.cfg.Layout.Start(t.index)))
	}
	if t.runFirst >= 0 {
		attrs = append(attrs, slog.Int("run_first", t.runFirst), slog.Bool("run_going", t.running))
		if t.running {
			attrs = append(attrs, slog.Duration("run_reached", t.reached))
		}
	}
	if err != nil {
		attrs = append(attrs, slog.Any("err", err), slog.Bool("client_gone", ctx.Err() != nil))
	}
	s.cfg.Logger.LogAttrs(ctx, level, "stream request", attrs...)
}

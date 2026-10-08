package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// Config configures a session.
type Config struct {
	// Args are the ffmpeg arguments, e.g. from the planner.
	Args []string
	// IdleTimeout ends the transcode when no client activity was reported
	// for this long; 0 never does.
	IdleTimeout time.Duration
	// ThrottleAhead pauses ffmpeg while its output is this far ahead of the
	// client's position, and resumes it when the client catches up; 0
	// never pauses.
	ThrottleAhead time.Duration
	// CheckInterval is how often idleness and throttling are checked;
	// default 5 seconds.
	CheckInterval time.Duration
	// PauseKey says the build pauses on "p" and resumes on "u", a
	// jellyfin-ffmpeg extension; others pause on "c" (the command prompt)
	// and resume on a newline.
	PauseKey bool
	// StopGrace is how long ffmpeg may take to finish after "q" before it
	// is killed; default 5 seconds.
	StopGrace time.Duration
	Logger    *slog.Logger
}

// Session supervises one ffmpeg run.
type Session struct {
	cfg  Config
	proc Process
	log  *slog.Logger

	mu           sync.Mutex
	progress     Progress
	lastActivity time.Time
	clientPos    time.Duration
	segments     map[int]time.Duration
	paused       bool
	cause        error

	stop chan error
	done chan struct{}
	err  error
}

// Start starts ffmpeg and supervises it until it exits, ctx is canceled,
// the session idles out or Stop is called.
func Start(ctx context.Context, start Starter, cfg Config) (*Session, error) {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 5 * time.Second
	}
	if cfg.StopGrace <= 0 {
		cfg.StopGrace = 5 * time.Second
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	proc, err := start(ctx, cfg.Args)
	if err != nil {
		return nil, err
	}
	s := &Session{
		cfg: cfg, proc: proc, log: log,
		lastActivity: time.Now(),
		segments:     map[int]time.Duration{},
		stop:         make(chan error, 1),
		done:         make(chan struct{}),
	}
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	go parseProgress(proc.Progress(), s.setProgress)
	go s.supervise(ctx, exited)
	return s, nil
}

func (s *Session) supervise(ctx context.Context, exited <-chan error) {
	defer close(s.done)
	tick := time.NewTicker(s.cfg.CheckInterval)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			s.finish(err)
			return
		case cause := <-s.stop:
			s.terminate(ctx, cause, exited)
			return
		case <-ctx.Done():
			s.terminate(ctx, context.Cause(ctx), exited)
			return
		case <-tick.C:
			if s.idle() {
				s.log.DebugContext(ctx, "transcode idle", "timeout", s.cfg.IdleTimeout)
				s.terminate(ctx, ErrIdle, exited)
				return
			}
			s.throttle(ctx)
		}
	}
}

func (s *Session) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cause != nil {
		err = s.cause
	} else if err != nil {
		err = fmt.Errorf("ffmpeg: %w", err)
	}
	s.err = err
}

// terminate asks ffmpeg to finish with "q", resuming it first, and kills
// it when it does not exit within the grace period.
func (s *Session) terminate(ctx context.Context, cause error, exited <-chan error) {
	s.mu.Lock()
	s.cause = cause
	paused := s.paused
	s.mu.Unlock()
	keys := "q"
	if paused {
		keys = s.resumeKey() + keys
	}
	if _, err := io.WriteString(s.proc.Stdin(), keys); err != nil {
		s.log.DebugContext(ctx, "asking ffmpeg to quit failed", "err", err)
	}
	grace := time.NewTimer(s.cfg.StopGrace)
	defer grace.Stop()
	select {
	case err := <-exited:
		s.finish(err)
		return
	case <-grace.C:
	}
	if err := s.proc.Kill(); err != nil {
		s.log.WarnContext(ctx, "killing ffmpeg failed", "err", err)
	}
	s.finish(<-exited)
}

func (s *Session) idle() bool {
	if s.cfg.IdleTimeout <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.lastActivity) >= s.cfg.IdleTimeout
}

func (s *Session) pauseKey() string {
	if s.cfg.PauseKey {
		return "p"
	}
	return "c"
}

func (s *Session) resumeKey() string {
	if s.cfg.PauseKey {
		return "u"
	}
	return "\n"
}

// throttle pauses ffmpeg while it is far enough ahead of the client.
func (s *Session) throttle(ctx context.Context) {
	if s.cfg.ThrottleAhead <= 0 {
		return
	}
	s.mu.Lock()
	ahead := s.clientPos > 0 && s.progress.Position > 0 && s.progress.Position-s.clientPos >= s.cfg.ThrottleAhead
	var key string
	switch {
	case ahead && !s.paused:
		key, s.paused = s.pauseKey(), true
	case !ahead && s.paused:
		key, s.paused = s.resumeKey(), false
	}
	s.mu.Unlock()
	if key == "" {
		return
	}
	s.log.DebugContext(ctx, "throttling ffmpeg", "paused", ahead)
	if _, err := io.WriteString(s.proc.Stdin(), key); err != nil {
		s.log.WarnContext(ctx, "throttling ffmpeg failed", "err", err)
	}
}

func (s *Session) setProgress(p Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = p
}

// Progress returns the latest progress report.
func (s *Session) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

// Paused reports whether the session throttled ffmpeg.
func (s *Session) Paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// Touch records client activity, postponing idle reaping.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()
}

// ReportPosition records the client's playback position, for throttling;
// it counts as activity.
func (s *Session) ReportPosition(pos time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientPos = pos
	s.lastActivity = time.Now()
}

// ReportSegment records that segment index, ending at end, was served;
// negative indexes, such as the initialization segment, are ignored. It
// counts as activity and advances the client position.
func (s *Session) ReportSegment(index int, end time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastActivity = time.Now()
	if index < 0 {
		return
	}
	s.segments[index] = end
	s.clientPos = max(s.clientPos, end)
}

// HighestServedSegmentEndingBy returns the highest served segment that ends
// at or before pos, which may be deleted once the client has moved on.
func (s *Session) HighestServedSegmentEndingBy(pos time.Duration) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	best, ok := 0, false
	for index, end := range s.segments {
		if end <= pos && (!ok || index > best) {
			best, ok = index, true
		}
	}
	return best, ok
}

// Stop ends the transcode and waits for ffmpeg to exit.
func (s *Session) Stop() error {
	select {
	case s.stop <- ErrStopped:
	default:
	}
	<-s.done
	return s.Err()
}

// Done is closed when ffmpeg has exited.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns why the session ended: nil when ffmpeg finished on its own,
// its failure, or the cause it was stopped for, such as [ErrIdle]. It is
// nil until Done is closed.
func (s *Session) Err() error {
	select {
	case <-s.done:
	default:
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if errors.Is(s.err, ErrStopped) {
		return nil
	}
	return s.err
}

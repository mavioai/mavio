package flaresolverr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// SessionPrefix starts the names of the sessions a Solver creates.
const SessionPrefix = "mavio-"

// Options are how a Solver solves.
type Options struct {
	// TTL rotates a session once it is that old; 0 never.
	TTL time.Duration
	// Idle is how long a session goes unused before Sweep destroys it,
	// 10 minutes when 0.
	Idle time.Duration
	// MaxConcurrent caps the challenges solved at once, 2 when 0.
	MaxConcurrent int
	// Reuse is how long a solution answers later requests for its host
	// and proxy, 30 seconds when 0.
	Reuse  time.Duration
	Logger *slog.Logger
}

// Job is a challenge to solve.
type Job struct {
	URL string
	// Post POSTs PostData, form-encoded.
	Post     bool
	PostData string
	// Proxy is the proxy to solve through, nil for none.
	Proxy        *Proxy
	Timeout      time.Duration
	DisableMedia bool
}

// Solver solves challenges with one FlareSolverr.
type Solver struct {
	c   *Client
	o   Options
	sem chan struct{}

	mu       sync.Mutex
	sessions map[string]*session
	flights  map[string]*flight
	recent   map[string]recent
}

// session is a FlareSolverr session, a browser, used by one request at a
// time.
type session struct {
	// lock is held while the session is used.
	lock          chan struct{}
	live          bool
	created, used time.Time
}

type flight struct {
	done chan struct{}
	sol  Solution
	err  error
}

type recent struct {
	sol Solution
	at  time.Time
}

// New returns a Solver with FlareSolverr's client.
func New(c *Client, o Options) *Solver {
	if o.Idle == 0 {
		o.Idle = 10 * time.Minute
	}
	if o.MaxConcurrent <= 0 {
		o.MaxConcurrent = 2
	}
	if o.Reuse == 0 {
		o.Reuse = 30 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	return &Solver{
		c: c, o: o, sem: make(chan struct{}, o.MaxConcurrent),
		sessions: map[string]*session{}, flights: map[string]*flight{}, recent: map[string]recent{},
	}
}

// SessionName names the session of a proxy: a hash of the proxy with its
// credentials, "mavio-direct" without one.
func SessionName(p *Proxy) string {
	if p == nil {
		return SessionPrefix + "direct"
	}
	sum := sha256.Sum256([]byte(p.URL + "\x00" + p.Username + "\x00" + p.Password))
	return SessionPrefix + hex.EncodeToString(sum[:8])
}

// Solve solves a challenge. Concurrent jobs for the same host and proxy
// share one solution, which answers those coming within Options.Reuse
// too.
func (s *Solver) Solve(ctx context.Context, j Job) (Solution, error) {
	u, err := url.Parse(j.URL)
	if err != nil || u.Host == "" {
		return Solution{}, fmt.Errorf("invalid URL %q", j.URL)
	}
	name := SessionName(j.Proxy)
	key := strings.ToLower(u.Host) + "|" + name
	for {
		s.mu.Lock()
		if r, ok := s.recent[key]; ok && time.Since(r.at) < s.o.Reuse {
			s.mu.Unlock()
			return r.sol, nil
		}
		if f, ok := s.flights[key]; ok {
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return Solution{}, ctx.Err()
			case <-f.done:
			}
			// The leading job was canceled, not this one: solve again.
			if errors.Is(f.err, context.Canceled) || errors.Is(f.err, context.DeadlineExceeded) {
				continue
			}
			return f.sol, f.err
		}
		f := &flight{done: make(chan struct{})}
		s.flights[key] = f
		s.mu.Unlock()

		f.sol, f.err = s.solve(ctx, name, j)
		s.mu.Lock()
		delete(s.flights, key)
		if f.err == nil {
			s.recent[key] = recent{sol: f.sol, at: time.Now()}
		}
		for k, r := range s.recent {
			if time.Since(r.at) >= s.o.Reuse {
				delete(s.recent, k)
			}
		}
		s.mu.Unlock()
		close(f.done)
		return f.sol, f.err
	}
}

func (s *Solver) solve(ctx context.Context, name string, j Job) (Solution, error) {
	select {
	case <-ctx.Done():
		return Solution{}, ctx.Err()
	case s.sem <- struct{}{}:
	}
	defer func() { <-s.sem }()
	sess := s.session(name)
	select {
	case <-ctx.Done():
		return Solution{}, ctx.Err()
	case sess.lock <- struct{}{}:
	}
	defer func() { <-sess.lock }()

	req := Request{URL: j.URL, Post: j.Post, PostData: j.PostData, MaxTimeout: j.Timeout.Milliseconds(), DisableMedia: j.DisableMedia}
	var sol Solution
	for attempt := 0; ; attempt++ {
		if err := s.ensure(ctx, name, sess, j.Proxy); err != nil {
			return Solution{}, err
		}
		var err error
		sol, err = s.c.Get(ctx, name, req)
		sess.used = time.Now()
		if err == nil {
			break
		}
		// FlareSolverr lost the session: create it again, once.
		var fe *Error
		if attempt == 0 && errors.As(err, &fe) && strings.Contains(strings.ToLower(fe.Message), "session") {
			sess.live = false
			continue
		}
		return Solution{}, err
	}
	if len(sol.Cookies) == 0 || sol.UserAgent == "" {
		return Solution{}, fmt.Errorf("FlareSolverr returned no cookies or user agent for %s (HTTP %d)", j.URL, sol.Status)
	}
	return sol, nil
}

func (s *Solver) session(name string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[name]
	if !ok {
		sess = &session{lock: make(chan struct{}, 1)}
		s.sessions[name] = sess
	}
	return sess
}

// ensure makes the session exist with its proxy: created when it was
// not, rotated after the TTL, and created again when FlareSolverr no
// longer has it, which a FlareSolverr started again would otherwise
// create without the proxy.
func (s *Solver) ensure(ctx context.Context, name string, sess *session, p *Proxy) error {
	if sess.live && s.o.TTL > 0 && time.Since(sess.created) >= s.o.TTL {
		if err := s.c.DestroySession(ctx, name); err != nil {
			s.o.Logger.WarnContext(ctx, "destroy FlareSolverr session", "session", name, "err", err)
		}
		sess.live = false
	}
	if sess.live {
		names, err := s.c.ListSessions(ctx)
		if err != nil {
			return err
		}
		sess.live = slices.Contains(names, name)
	}
	if sess.live {
		return nil
	}
	err := s.c.CreateSession(ctx, name, p)
	var fe *Error
	if errors.As(err, &fe) {
		// A session of the name is left from before: make it again, for
		// it to have the proxy.
		_ = s.c.DestroySession(ctx, name)
		err = s.c.CreateSession(ctx, name, p)
	}
	if err != nil {
		return err
	}
	sess.live, sess.created, sess.used = true, time.Now(), time.Now()
	return nil
}

// Sweep destroys the sessions unused for Options.Idle, and those of the
// prefix the Solver did not create, left from before.
func (s *Solver) Sweep(ctx context.Context) error {
	names, err := s.c.ListSessions(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range names {
		if !strings.HasPrefix(name, SessionPrefix) {
			continue
		}
		s.mu.Lock()
		sess, ok := s.sessions[name]
		s.mu.Unlock()
		if !ok {
			if err := s.c.DestroySession(ctx, name); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		select {
		case sess.lock <- struct{}{}:
		default:
			continue // in use
		}
		if !sess.live || time.Since(sess.used) >= s.o.Idle {
			sess.live = false
			if err := s.c.DestroySession(ctx, name); err != nil {
				errs = append(errs, err)
			}
		}
		<-sess.lock
	}
	return errors.Join(errs...)
}

// Close destroys the sessions the Solver created, waiting for those in
// use.
func (s *Solver) Close(ctx context.Context) error {
	s.mu.Lock()
	sessions := make(map[string]*session, len(s.sessions))
	for name, sess := range s.sessions {
		sessions[name] = sess
	}
	s.mu.Unlock()
	var errs []error
	for name, sess := range sessions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sess.lock <- struct{}{}:
		}
		if sess.live {
			sess.live = false
			if err := s.c.DestroySession(ctx, name); err != nil {
				errs = append(errs, err)
			}
		}
		<-sess.lock
	}
	return errors.Join(errs...)
}

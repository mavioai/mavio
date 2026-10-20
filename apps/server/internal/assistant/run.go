package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var (
	// errSteered cancels an attempt of a run that was steered.
	errSteered = errors.New("the user went on speaking")
	// errAborted cancels an attempt of a run that was aborted.
	errAborted = errors.New("the run was aborted")
	// errDropped fails a held effect of a run that was steered.
	errDropped = errors.New("not done: the user went on speaking, and the request changed")
)

// runKey names a run among the device's.
type runKey struct {
	user, device core.ID
	id           string
}

// run is a request a voice session can steer and abort, whose effects are
// held for the commit delay (docs/assistant.md §8.4).
type run struct {
	mu sync.Mutex
	// text is the request, joined with what steered it.
	text string
	// until is when held effects happen: the commit delay after the turn
	// ended, moved by each steer.
	until time.Time
	// steered is closed and replaced by each steer, dropping held effects.
	steered chan struct{}
	// stopped is closed when the run is aborted.
	stopped chan struct{}
	// cancel cancels the attempt in progress; nil once it has ended.
	cancel context.CancelCauseFunc
	// conversation is the AI path's, while the attempt asks the model.
	conversation string
	delay        time.Duration
}

// hold waits until held effects may happen: it fails when the run is
// steered or aborted meanwhile.
func (r *run) hold(ctx context.Context) error {
	for {
		r.mu.Lock()
		wait, steered, stopped := time.Until(r.until), r.steered, r.stopped
		r.mu.Unlock()
		if wait <= 0 {
			return nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return context.Cause(ctx)
		case <-steered:
			timer.Stop()
			return errDropped
		case <-stopped:
			timer.Stop()
			return errAborted
		case <-timer.C:
		}
	}
}

// askRun carries out a run with an ID: each steer cancels the attempt in
// progress on the fast path and carries the joined request out again,
// and steers the model's run on the AI path; an abort ends it cancelled.
func (s *Service) askRun(ctx context.Context, p auth.Principal, set core.AssistantSettings, req *assistantv1.AskRequest, send func(*assistantv1.AskResponse) error) error {
	key := runKey{user: p.User.ID, device: p.Session.ID, id: req.GetRunId()}
	r := &run{
		text: strings.TrimSpace(req.GetText()), until: time.Now().Add(set.CommitDelay), delay: set.CommitDelay,
		steered: make(chan struct{}), stopped: make(chan struct{}),
	}
	s.mu.Lock()
	if _, ok := s.runs[key]; ok {
		s.mu.Unlock()
		return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("run %s is in progress", key.id))
	}
	s.runs[key] = r
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.runs, key)
		s.mu.Unlock()
	}()
	for {
		attempt, cancel := context.WithCancelCause(ctx)
		r.mu.Lock()
		r.cancel = cancel
		text := r.text
		r.mu.Unlock()
		areq := proto.CloneOf(req)
		if req.GetText() != "" {
			areq.SetText(text)
		}
		t := s.newTurn(attempt, p, set, areq, send)
		t.call.hold = r.hold
		t.steerable = r
		err := t.run()
		r.mu.Lock()
		r.cancel, r.conversation = nil, ""
		r.mu.Unlock()
		cause := context.Cause(attempt)
		cancel(nil)
		switch {
		case errors.Is(cause, errSteered):
			continue
		case errors.Is(cause, errAborted):
			t.done = assistantv1.Done_builder{Cancelled: new(true), Language: &t.lang}.Build()
			t.d.mu.Lock()
			t.done.SetConversationId(t.d.conversation)
			t.d.mu.Unlock()
		case ctx.Err() != nil:
			// The session went away: the model's run is aborted too.
			s.abortAI(context.WithoutCancel(ctx), t)
			return ctx.Err()
		case err != nil:
			return err
		}
		t.emit(assistantv1.AskResponse_builder{Done: t.done}.Build())
		return nil
	}
}

// runOf returns the caller's run in progress.
func (s *Service) runOf(ctx context.Context, id string) (*run, error) {
	p, ok := auth.FromContext(ctx)
	if !ok || p.User.ID.IsZero() {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	s.mu.Lock()
	r := s.runs[runKey{user: p.User.ID, device: p.Session.ID, id: id}]
	s.mu.Unlock()
	if r == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("run %s is not in progress", id))
	}
	return r, nil
}

// SteerAsk adds what the user went on to say to a run in progress.
func (s *Service) SteerAsk(ctx context.Context, req *assistantv1.SteerAskRequest) (*assistantv1.SteerAskResponse, error) {
	r, err := s.runOf(ctx, req.GetRunId())
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(req.GetText())
	r.mu.Lock()
	if r.cancel == nil {
		r.mu.Unlock()
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("run %s has ended", req.GetRunId()))
	}
	r.text = strings.TrimSpace(r.text + " " + text)
	r.until = time.Now().Add(r.delay)
	close(r.steered)
	r.steered = make(chan struct{})
	conv, cancel := r.conversation, r.cancel
	r.mu.Unlock()
	if conv == "" {
		cancel(errSteered)
		return &assistantv1.SteerAskResponse{}, nil
	}
	// The model reads it at its run's next boundary, after the held calls
	// it made fail.
	ag, ok := s.cfg.Agents()
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AGENT_SESSION plugin is running"))
	}
	if _, err := ag.SteerAgentMessage(ctx, pluginv1.SteerAgentMessageRequest_builder{
		SessionId: &conv, Text: &text, RequestId: new(core.NewID().String()),
	}.Build()); err != nil {
		return nil, err
	}
	return &assistantv1.SteerAskResponse{}, nil
}

// AbortAsk stops a run in progress.
func (s *Service) AbortAsk(ctx context.Context, req *assistantv1.AbortAskRequest) (*assistantv1.AbortAskResponse, error) {
	r, err := s.runOf(ctx, req.GetRunId())
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	select {
	case <-r.stopped:
	default:
		close(r.stopped)
	}
	conv, cancel := r.conversation, r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel(errAborted)
	}
	if conv != "" {
		if ag, ok := s.cfg.Agents(); ok {
			if _, err := ag.AbortAgentMessage(ctx, pluginv1.AbortAgentMessageRequest_builder{SessionId: &conv}.Build()); err != nil {
				s.log.WarnContext(ctx, "assistant: abort the model's run", "err", err)
			}
		}
	}
	return &assistantv1.AbortAskResponse{}, nil
}

// abortAI aborts the model's run of a turn that asked it.
func (s *Service) abortAI(ctx context.Context, t *turn) {
	if t.conv == "" || s.cfg.Agents == nil {
		return
	}
	if ag, ok := s.cfg.Agents(); ok {
		if _, err := ag.AbortAgentMessage(ctx, pluginv1.AbortAgentMessageRequest_builder{SessionId: &t.conv}.Build()); err != nil {
			s.log.WarnContext(ctx, "assistant: abort the model's run", "err", err)
		}
	}
}

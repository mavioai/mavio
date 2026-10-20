package assistant

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
)

// started is a run in progress and what it sent.
type started struct {
	mu   sync.Mutex
	sent []*assistantv1.AskResponse
	err  chan error
}

func (r *started) responses() []*assistantv1.AskResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.sent)
}

// startRun starts a run with an ID, as a voice session does.
func startRun(ctx context.Context, s *Service, id, s0 string) *started {
	r := &started{err: make(chan error, 1)}
	req := text(s0, "en")
	req.SetRunId(id)
	go func() {
		r.err <- s.askRun(ctx, signedIn, core.DefaultAssistantSettings(), req, func(resp *assistantv1.AskResponse) error {
			r.mu.Lock()
			r.sent = append(r.sent, resp)
			r.mu.Unlock()
			return nil
		})
	}()
	return r
}

func played(sent []*assistantv1.AskResponse) [][]string {
	var out [][]string
	for _, a := range actions(sent) {
		if a.HasPlay() {
			out = append(out, a.GetPlay().GetItemIds())
		}
	}
	return out
}

func TestRunHoldsItsEffects(t *testing.T) {
	s := newTestService(t, movie("a", "Alien"), movie("c", "Alien Covenant"))
	synctest.Test(t, func(t *testing.T) {
		ctx := auth.WithPrincipal(t.Context(), signedIn)
		r := startRun(ctx, s, "r1", "play Alien")
		synctest.Wait()
		if got := played(r.responses()); len(got) != 0 {
			t.Fatalf("plays before the commit delay = %v, want none", got)
		}
		time.Sleep(core.DefaultAssistantSettings().CommitDelay)
		if err := <-r.err; err != nil {
			t.Fatal(err)
		}
		if got := played(r.responses()); !slices.EqualFunc(got, [][]string{{"a"}}, slices.Equal) {
			t.Errorf("plays = %v, want = [[a]]", got)
		}
	})
}

func TestSteeredRunDropsWhatItHeld(t *testing.T) {
	s := newTestService(t, movie("a", "Alien"), movie("c", "Alien Covenant"))
	synctest.Test(t, func(t *testing.T) {
		ctx := auth.WithPrincipal(t.Context(), signedIn)
		r := startRun(ctx, s, "r1", "play Alien")
		synctest.Wait()
		time.Sleep(300 * time.Millisecond)
		if _, err := s.SteerAsk(ctx, assistantv1.SteerAskRequest_builder{RunId: new("r1"), Text: new("Covenant")}.Build()); err != nil {
			t.Fatal(err)
		}
		// The commit delay starts again with the steer.
		time.Sleep(500 * time.Millisecond)
		synctest.Wait()
		if got := played(r.responses()); len(got) != 0 {
			t.Fatalf("plays within the steer's commit delay = %v, want none", got)
		}
		if err := <-r.err; err != nil {
			t.Fatal(err)
		}
		sent := r.responses()
		if got := played(sent); !slices.EqualFunc(got, [][]string{{"c"}}, slices.Equal) {
			t.Errorf("plays = %v, want = [[c]]", got)
		}
		var understood []string
		for _, resp := range sent {
			if resp.HasUnderstood() {
				understood = append(understood, resp.GetUnderstood().GetText())
			}
		}
		if want := []string{"play Alien", "play Alien Covenant"}; !slices.Equal(understood, want) {
			t.Errorf("understood = %q, want = %q", understood, want)
		}
		if n := len(sent); !sent[n-1].HasDone() || sent[n-1].GetDone().GetCancelled() {
			t.Errorf("last = %v, want done", sent[n-1])
		}
		_, err := s.SteerAsk(ctx, assistantv1.SteerAskRequest_builder{RunId: new("r1"), Text: new("no")}.Build())
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("SteerAsk of an ended run = %v, want failed_precondition", err)
		}
	})
}

func TestAbortedRunEndsCancelled(t *testing.T) {
	s := newTestService(t, movie("a", "Alien"))
	synctest.Test(t, func(t *testing.T) {
		ctx := auth.WithPrincipal(t.Context(), signedIn)
		r := startRun(ctx, s, "r1", "play Alien")
		synctest.Wait()
		// Another device's runs are its own.
		other := auth.WithPrincipal(ctx, auth.Principal{User: signedIn.User, Session: core.AuthSession{ID: core.NewID()}})
		_, err := s.AbortAsk(other, assistantv1.AbortAskRequest_builder{RunId: new("r1")}.Build())
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("AbortAsk from another device = %v, want failed_precondition", err)
		}
		if _, err := s.AbortAsk(ctx, assistantv1.AbortAskRequest_builder{RunId: new("r1")}.Build()); err != nil {
			t.Fatal(err)
		}
		if err := <-r.err; err != nil {
			t.Fatal(err)
		}
		sent := r.responses()
		if got := played(sent); len(got) != 0 {
			t.Errorf("plays = %v, want none", got)
		}
		if last := sent[len(sent)-1]; !last.GetDone().GetCancelled() {
			t.Errorf("last = %v, want done cancelled", last)
		}
	})
}

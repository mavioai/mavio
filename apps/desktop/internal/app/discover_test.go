package app

import (
	"slices"
	"testing"
	"time"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func TestDiscover(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.settle()
	click := h.click
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	click("Discover")
	if got, want := h.app.router.Path(), "/discover"; got != want {
		t.Fatalf("path = %q, want = %q", got, want)
	}
	want("Trending", "Popular Movies", "Popular Series", "Starfall", "2025 · Movie", "Northern Lights", "Requests")
	h.save(shots, "discover")

	click("Starfall")
	want("Starfall", "Every orbit ends.", "A crew far from home finds the way back.", "Mara Quinn", "Commander Vale", "Request", "Similar Titles")
	h.save(shots, "discover-title")
	click("Request")
	if n := len(h.core.titleRequests); n != 1 {
		t.Fatalf("requests = %d, want = 1", n)
	}
	want("Requested", "Approve")

	h.app.router.Push("/title/" + h.app.servers[0].GetId() + "/series/201")
	h.settle()
	want("Northern Lights", "Season 1", "Season 2")
	click("Season 2")
	want("1 - Aurora 1", "3 - Aurora 3")
	// A season opens at the click, rather than when another frame comes:
	// the season below it, Season 1 as the latest is first, moves down.
	below := func() float32 {
		t.Helper()
		r, ok := h.tt.Find("Season 1")
		if !ok {
			t.Fatal("no Season 1")
		}
		return r.Y
	}
	click("Season 2")
	closed := below()
	if err := h.tt.Click("Season 2"); err != nil {
		t.Fatal(err)
	}
	h.tt.Advance(time.Second)
	if got := below(); got <= closed+50 {
		t.Errorf("Season 1 at %v after Season 2 opened, want below %v", got, closed+50)
	}
	click("Request")
	want("Request Northern Lights", "Request 2 Seasons")
	h.app.discoverState.chosen[1] = false
	h.tt.Frame()
	want("Request 1 Season")
	h.save(shots, "discover-seasons")
	click("Request 1 Season")
	if n := len(h.core.titleRequests); n != 2 {
		t.Fatalf("requests = %d, want = 2", n)
	}
	var seasons []int32
	for _, s := range h.core.titleRequests[1].GetSeasons() {
		seasons = append(seasons, s.GetSeasonNumber())
	}
	if !slices.Equal(seasons, []int32{2}) {
		t.Errorf("seasons = %v, want = [2]", seasons)
	}

	h.app.router.Push("/requests/" + h.app.servers[0].GetId())
	h.settle()
	want("Requests", "Northern Lights", "Season 2", "Starfall", "Pending")
	h.save(shots, "discover-requests")
	click("Approve")
	if got := h.core.titleRequests[1].GetStatus(); got != acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED {
		t.Errorf("status = %v, want = approved", got)
	}
}

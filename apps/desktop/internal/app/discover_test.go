package app

import (
	"slices"
	"testing"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func TestDiscover(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.settle()
	click := func(label string) {
		t.Helper()
		if err := h.tt.Click(label); err != nil {
			t.Fatalf("click %q: %v; texts = %q", label, err, h.tt.Texts())
		}
		h.settle()
	}
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

package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// Search runs as it is confirmed, and lists the server's row titled From
// the Server, then the titles discovery finds, as the web UI's.
func TestSearch(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.app.router.Push("/search")
	h.settle()
	h.tt.ClickAt(574, 70) // the field
	h.tt.Type("orb")
	h.settle()
	if h.tt.HasText("From the Server") {
		t.Fatalf("searched before Return; texts = %q", h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyEnter)
	for range 20 {
		if h.tt.HasText("From Discover") {
			break
		}
		h.settle()
	}
	for _, w := range []string{"From the Server", "From Discover", "The Long Orbit"} {
		if !h.tt.HasText(w) {
			t.Errorf("no %q; texts = %q", w, h.tt.Texts())
		}
	}
	h.save(shots, "search")

	// Confirming the query again searches again, keeping the results
	// shown meanwhile.
	before := len(h.core.requestsOf((*capiv1.Request).HasSearch))
	h.tt.Key(0, ui.KeyEnter)
	if !h.tt.HasText("The Long Orbit") {
		t.Errorf("the results went as the search ran again; texts = %q", h.tt.Texts())
	}
	h.settle()
	after := len(h.core.requestsOf((*capiv1.Request).HasSearch))
	if after != before+1 {
		t.Errorf("searches = %d, want = %d", after, before+1)
	}

	// A query nothing matches says so, with no row.
	h.click("Clear")
	h.tt.ClickAt(574, 70)
	h.tt.Type("zzz")
	h.tt.Key(0, ui.KeyEnter)
	for range 10 {
		if h.tt.HasText("No results") {
			break
		}
		h.settle()
	}
	if !h.tt.HasText("No results") || h.tt.HasText("From the Server") {
		t.Errorf("no results: texts = %q", h.tt.Texts())
	}
}

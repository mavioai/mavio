package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
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
}

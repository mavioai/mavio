package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// Search runs as the expanded sidebar's field confirms it, on the Search
// page, and lists the server's row titled From the Server, then the
// titles discovery finds, as the web UI's.
func TestSearch(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.settle()
	h.save(shots, "search-sidebar")
	h.tt.Key(ui.Cmd, ui.KeyK) // the sidebar's field
	h.tt.Type("orb")
	h.settle()
	if p := h.app.router.Path(); p != "/home" || h.tt.HasText("From the Server") {
		t.Fatalf("typing left the page or searched: path = %q, texts = %q", p, h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyEnter)
	for range 20 {
		if h.tt.HasText("From Discover") {
			break
		}
		h.settle()
	}
	if l := h.app.router.Location(); l != "/search?q=orb" {
		t.Errorf("location = %q, want = %q", l, "/search?q=orb")
	}
	for _, w := range []string{"From the Server", "From Discover", "The Long Orbit"} {
		if !h.tt.HasText(w) {
			t.Errorf("no %q; texts = %q", w, h.tt.Texts())
		}
	}
	h.save(shots, "search")

	// Confirming the query again searches again, keeping the results
	// shown meanwhile.
	before := len(h.core.requestsOf((*capiv1.Request).HasSearchServer))
	h.tt.Key(0, ui.KeyEnter)
	if !h.tt.HasText("The Long Orbit") {
		t.Errorf("the results went as the search ran again; texts = %q", h.tt.Texts())
	}
	h.settle()
	after := len(h.core.requestsOf((*capiv1.Request).HasSearchServer))
	if after != before+1 {
		t.Errorf("searches = %d, want = %d", after, before+1)
	}

	// A query nothing matches says so, with no row.
	h.click("Clear")
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

	// Each search is an entry of the history: Back returns to the one
	// before, the field following it.
	h.app.router.Go(-1)
	h.settle()
	if got := h.app.searchState.query; got != "orb" || !h.tt.HasText("The Long Orbit") {
		t.Errorf("back: query = %q, texts = %q", got, h.tt.Texts())
	}
	// Leaving Search empties the field; Search, chosen again, returns
	// to its query.
	h.click("Home")
	if got := h.app.searchState.query; got != "" {
		t.Errorf("query on Home = %q, want = empty", got)
	}
	// Confirming no query stays where it is.
	h.tt.Key(ui.Cmd, ui.KeyK)
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if p := h.app.router.Path(); p != "/home" {
		t.Errorf("path = %q, want = /home", p)
	}
	h.app.openPlace("/search")
	h.settle()
	if l, q := h.app.router.Location(), h.app.searchState.query; l != "/search?q=orb" || q != "orb" {
		t.Errorf("location = %q, query = %q, want = /search?q=orb", l, q)
	}
}

// A server the core cannot search shows as unreachable, the results of
// discovery still shown.
func TestSearchFailed(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.core.searchErr = &capi.Error{Code: capiv1.Error_CODE_INVALID, Reason: "unreachable"}
	h.app.searchFor("orb")
	for range 20 {
		if h.tt.HasText("From Discover") {
			break
		}
		h.settle()
	}
	want := h.core.s.Server.Name + " is unreachable"
	for _, w := range []string{want, "From Discover"} {
		if !h.tt.HasText(w) {
			t.Errorf("no %q; texts = %q", w, h.tt.Texts())
		}
	}
}

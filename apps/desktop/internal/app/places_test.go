package app

import "testing"

func TestPlaces(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	item := "/item/" + h.core.s.Server.ID + "/orbit"
	path := func(want, after string) {
		t.Helper()
		if got := h.app.router.Path(); got != want {
			t.Fatalf("after %s: path = %q, want = %q", after, got, want)
		}
	}
	if _, ok := h.tt.Find("Back"); ok {
		t.Errorf("Home shows Back; a place has none")
	}
	h.app.router.Push(item)
	h.settle()
	if _, ok := h.tt.Find("Back"); !ok {
		t.Fatalf("a page opened from Home shows no Back; texts = %q", h.tt.Texts())
	}
	// Another place and back returns to the page last shown in Home.
	h.click("Discover")
	path("/discover", "choosing Discover")
	h.click("Home")
	path(item, "choosing Home again")
	if got := h.app.places.current; got != "/home" {
		t.Errorf("place shown = %q, want = /home", got)
	}
	// Home chosen while it is shown returns to its top.
	h.click("Home")
	path("/home", "choosing Home while shown")

	h.app.router.Push(item)
	h.settle()
	h.click("Back")
	path("/home", "Back")

	// Back from pages two deep goes back through the history, where the
	// router keeps the pages, rather than opening Home anew.
	crew := "/credits/" + h.core.s.Server.ID + "/orbit"
	h.app.router.Push(item)
	h.settle()
	h.app.router.Push(crew)
	h.settle()
	h.click("Back")
	path(item, "Back from the credits")
	h.click("Back")
	path("/home", "Back from the item")
	if !h.app.router.CanGoForward() {
		t.Errorf("Back opened Home anew; want it back through the history")
	}
}

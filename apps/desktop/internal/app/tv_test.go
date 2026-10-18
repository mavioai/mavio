package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTVMode(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	focused := func(label, after string) {
		t.Helper()
		if !h.tt.Focused(label) {
			t.Fatalf("after %s: %q has no focus; texts = %q", after, label, h.tt.Texts())
		}
	}
	for _, tab := range []string{"Home", "Libraries", "Search"} {
		if _, ok := h.tt.Find(tab); !ok {
			t.Fatalf("no tab %q; texts = %q", tab, h.tt.Texts())
		}
	}
	if _, ok := h.tt.Find("Back"); ok {
		t.Errorf("the TV mode shows Back")
	}
	h.save(shots, "tv-home")

	// The page opens with focus on its first element, the hero's.
	focused("Play", "opening Home")
	key(ui.KeyRight)
	focused("Details", "right from Play")
	// Up from the page's first row is the current tab.
	key(ui.KeyUp)
	focused("Home", "up from the hero")
	// On a tab's page, focus on another tab opens it in its place.
	key(ui.KeyRight)
	focused("Libraries", "right from Home")
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("path = %q, want = /libraries", got)
	}
	h.save(shots, "tv-libraries")
	// Back from a tab goes to Home's.
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from a tab: path = %q, want = /home", got)
	}

	// Down into the rows, and a card opened and left with back.
	key(ui.KeyDown)
	key(ui.KeyDown)
	h.save(shots, "tv-home-row")
	key(ui.KeyEnter)
	if got := h.app.router.Path(); got == "/home" {
		t.Fatalf("Return on a card opened nothing")
	}
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from the card's page: path = %q, want = /home", got)
	}

	// The user menu opens with focus on Settings.
	h.app.userMenuOpen = true
	h.settle()
	focused("Settings", "opening the menu")
	h.save(shots, "tv-menu")
	key(ui.KeyEscape)
	if h.app.userMenuOpen {
		t.Errorf("back left the menu open")
	}

	// Videos play in the app's window, which a remote stays in.
	h.app.opts.NewWindow = func(_ func(*ui.Context), closed func()) Window { return &fakeWindow{closed: closed} }
	h.app.play(h.core.s.Server.ID, "orbit", nil)
	h.settle()
	if h.app.playerOf(nil) == nil {
		t.Errorf("the TV mode played in a window of the player's")
	}
}

func TestTVDetailsAndLibrary(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	server := h.core.s.Server.ID

	// A title's details hide the bar, and open with focus on Play.
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	if _, ok := h.tt.Find("Libraries"); ok {
		t.Errorf("the details show the tabs")
	}
	if !h.tt.Focused("Play") && !h.tt.Focused("Resume") {
		t.Fatalf("the details opened without focus on their primary action")
	}
	h.save(shots, "tv-details")
	for range 3 {
		key(ui.KeyDown)
	}
	h.save(shots, "tv-details-down")
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from the details: path = %q, want = /home", got)
	}

	// A library opens with focus on Recommended; focus on Library shows
	// the grid of all its items.
	h.app.openPlace("/libraries")
	h.settle()
	key(ui.KeyDown)
	key(ui.KeyEnter)
	if !h.tt.Focused("Recommended") {
		t.Fatalf("the library opened without focus on Recommended; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-library")
	key(ui.KeyRight)
	if !h.tt.Focused("Library") {
		t.Fatalf("right from Recommended: Library has no focus; texts = %q", h.tt.Texts())
	}
	for k, st := range h.app.libraryStates {
		if st.tab != 1 {
			t.Errorf("%s: focus on Library left Recommended shown", k)
		}
	}
	h.save(shots, "tv-library-all")
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("back from a library: path = %q, want = /libraries", got)
	}
}

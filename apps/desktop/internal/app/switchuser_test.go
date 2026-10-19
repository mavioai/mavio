package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSwitchUser(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	user := func() string {
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		return h.app.servers[0].GetUserName()
	}
	me := user()
	h.app.userMenuOpen = true
	h.settle()
	for _, want := range []string{"Switch User", "Anna", "Kids"} {
		if !h.tt.HasText(want) {
			t.Errorf("the user menu has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	if h.tt.HasText("Guest") {
		t.Errorf("the user menu offers a disabled user")
	}
	h.save(shots, "switch-menu")

	// Kids has a PIN: the pad asks for it, a wrong one is refused.
	h.click("Kids")
	h.settle()
	if !h.tt.HasText("Enter Kids's PIN") {
		t.Fatalf("no PIN pad; texts = %q", h.tt.Texts())
	}
	for _, k := range "9999" {
		h.click(string(k))
		h.settle()
	}
	if !h.tt.HasText("The code is incorrect.") {
		t.Errorf("a wrong PIN shows no error; texts = %q", h.tt.Texts())
	}
	if got := user(); got != me {
		t.Errorf("user = %q after a wrong PIN, want = %q", got, me)
	}
	h.save(shots, "switch-pin")
	for _, k := range switchPin {
		h.tt.Key(0, ui.Key0+ui.Key(k-'0'))
		h.settle()
	}
	if got := user(); got != "Kids" {
		t.Errorf("user = %q, want = Kids", got)
	}
	if h.app.switching.asking != nil {
		t.Errorf("the PIN pad stayed open")
	}

	// Anna has none: choosing her switches at once.
	h.app.userMenuOpen = true
	h.settle()
	h.click("Anna")
	h.settle()
	if got := user(); got != "Anna" {
		t.Errorf("user = %q, want = Anna", got)
	}
	if h.app.userMenuOpen {
		t.Errorf("the user menu stayed open")
	}
}

func TestTVMembers(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.app.router.Push("/settings")
	h.settle()
	for _, want := range []string{"Switch User", "Anna", "Kids"} {
		if !h.tt.HasText(want) {
			t.Errorf("the TV settings have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "tv-members")
	h.click("Kids")
	h.settle()
	if !h.tt.HasText("Enter Kids's PIN") {
		t.Fatalf("no PIN pad; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-pin")
	// Back with no digit leaves the pad.
	h.tt.Key(0, ui.KeyBackspace)
	h.settle()
	if h.app.switching.asking != nil {
		t.Errorf("back left the PIN pad open")
	}
}

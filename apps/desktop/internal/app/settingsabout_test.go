package app

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

func TestSettingsAbout(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.app.opts.Device = capiv1.Device_builder{Id: "device-1", Name: "Studio", Client: "Mavio for macOS", ClientVersion: "0.9.0"}.Build()
	h.app.opts.System = func(context.Context) System {
		return System{Version: "macOS 26.1", Manufacturer: "Apple", Model: "Mac16,1"}
	}
	h.app.router.Push("/settings/about")
	h.settle()
	for _, want := range []string{
		"About", "App Name", "Mavio for macOS", "Client Version", "0.9.0", "Core Version", "1.2.3", "GPL-3.0",
		"Device Name", "Studio", "Device ID", "device-1", "Apple", "Mac16,1", "macOS 26.1", "The servers know this device by its name and ID.",
	} {
		if !h.tt.HasText(want) {
			t.Errorf("no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "settings-about")
}

// The server's sections are flat in the sidebar, under the server's
// heading alone.
func TestSettingsSidebarFlat(t *testing.T) {
	h := newHarness(t, true, 1280, 1400)
	h.app.router.Push("/settings/overview")
	h.settle()
	for _, want := range []string{"Libraries", "Storage", "Transcoding", "Acquisition", "Downloads", "Activity", "Logs", "About"} {
		if !h.tt.HasText(want) {
			t.Errorf("no %q in the sidebar; texts = %q", want, h.tt.Texts())
		}
	}
	for _, title := range []string{"Media", "Acquisition & Downloads", "Monitoring & Maintenance"} {
		if h.tt.HasText(title) {
			t.Errorf("the sidebar has the heading %q", title)
		}
	}
	h.save(shots, "settings-sidebar")
}

// Going between the settings' layout and the app's, the page shows at
// once, both ways, the frame after the change as it is once settled.
func TestSettingsLayoutInstant(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	steps := []struct {
		name string
		go_  func()
	}{
		{"to the settings by the shortcut", func() { h.tt.Key(ui.Cmd, ui.KeyComma) }},
		{"to Home", func() { h.app.router.Push("/home") }},
		{"to the settings by the menu", func() { h.app.router.Push("/settings") }},
		{"to Home by Back", func() { h.app.router.Back() }},
	}
	for _, s := range steps {
		s.go_()
		h.tt.Frame()
		first := slices.Clone(h.tt.Image().Pix)
		h.settle()
		if !bytes.Equal(first, h.tt.Image().Pix) {
			t.Errorf("%s: the page faded in", s.name)
		}
	}
}

// With a Mavio account the server's group starts with the choice of the
// server, as the web UI's, even with one server.
func TestSettingsServerChoice(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.app.account = capiv1.Account_builder{Id: "acc"}.Build()
	h.app.router.Push("/settings/overview")
	h.settle()
	r, ok := h.tt.Find(h.core.s.Server.Name)
	if !ok {
		t.Fatalf("no choice of the server; texts = %q", h.tt.Texts())
	}
	if r.X > 200 {
		t.Errorf("the choice is at x = %v, want = in the sidebar", r.X)
	}
	h.save(shots, "settings-server-choice")
	// Its options open in a popup, the chosen one checked.
	if err := h.tt.Click(h.core.s.Server.Name); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.save(shots, "settings-server-choice-open")
}

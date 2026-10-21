package app

import (
	"context"
	"testing"

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

// Going from the settings to Home, Home shows at once, not fading in.
func TestSettingsToHome(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	h.app.router.Push("/settings/appearance")
	h.settle()
	h.app.router.Push("/home")
	h.tt.Frame()
	if !h.tt.HasText("Continue Watching") {
		t.Errorf("Home did not show at once; texts = %q", h.tt.Texts())
	}
}

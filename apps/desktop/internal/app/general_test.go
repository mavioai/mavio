package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

func TestServerSettings(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	settings := func() (*systemv1.GeneralSettings, *playbackv1.TranscodingSettings) {
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		return h.core.general, h.core.transcoding
	}
	ss := &h.app.settingsState.admin.server

	// General: the name, metadata, sign-in and network.
	h.app.router.Push("/settings/general")
	h.settle()
	want("Server name", "Metadata language", "Follow client language (default)", "Stay signed in", "Password resets", "Network", "Local discovery")
	ss.general.draft.name = "Den"
	ss.general.draft.resetPlugin = "dev.mavio.mail"
	ss.general.draft.httpsPort = 8920
	h.settle()
	want("Certificate", "Private key")
	h.save(shots, "settings-general")
	h.click("Save")
	want("Saved.")
	if s, _ := settings(); s.GetNetwork().GetServerName() != "Den" || s.GetPasswordResetPlugin() != "dev.mavio.mail" || s.GetNetwork().GetHttpsPort() != 8920 {
		t.Errorf("settings = %v, want the name Den, the reset plugin and HTTPS on 8920", s)
	}

	// Transcoding.
	h.app.router.Push("/settings/transcoding")
	h.settle()
	want("Acceleration", "Automatic", "Hardware encoding", "Preset", "By the source", "Tone mapping", "bt2390", "Downmix boost", "Transcode folder")
	h.click("Crop black borders")
	h.save(shots, "settings-transcoding")
	h.click("Save")
	want("Saved.")
	if s, tr := settings(); !tr.GetCropBlackBorders() || tr.GetH264Crf() != 23 || s.GetNetwork().GetServerName() != "Den" {
		t.Errorf("transcoding = %v, want borders cropped and the rest kept", tr)
	}

	// API keys: making one, whose token shows once, and revoking it.
	h.app.router.Push("/settings/api-keys")
	h.settle()
	want("Home Assistant", "As Anna", "Keys")
	ss.apiKeys.name = "Scripts"
	h.click("Create")
	want("Key for Scripts", "mk_secret-token", "Copy", "Never used")
	h.save(shots, "settings-api-keys")
	h.click("Done")
	if h.tt.HasText("mk_secret-token") {
		t.Errorf("the token still shows after Done")
	}
	h.click("Revoke")
	want("Revoke Home Assistant?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.tt.HasText("Home Assistant") {
		t.Errorf("Home Assistant was not revoked")
	}
}

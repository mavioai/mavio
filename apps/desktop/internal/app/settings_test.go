package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

func TestSettings(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.tt.Key(ui.Cmd, ui.KeyComma)
	h.settle()
	if got, want := h.app.router.Path(), "/settings/appearance"; got != want {
		t.Fatalf("path = %q, want = %q", got, want)
	}
	for _, want := range []string{"Settings", "Appearance", "Playback", "Subtitles and Audio", "Devices", "TV Mode", "System Language"} {
		if !h.tt.HasText(want) {
			t.Errorf("settings have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "settings-appearance")
	if err := h.tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := h.app.Appearance(); got != AppearanceDark {
		t.Errorf("appearance = %q, want = dark", got)
	}

	if err := h.tt.Click("Playback"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.Click("Play the Next Episode Automatically"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.app.autoplayNext() {
		t.Errorf("autoplay next = true, want = false after turning it off")
	}

	if err := h.tt.Click("Subtitles and Audio"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	f := &h.app.settingsState.languages
	if f.audio != "jpn, eng" || !h.tt.HasText("When the Audio Is in Another Language") {
		t.Errorf("audio = %q, texts = %q, want = the user's preferences", f.audio, h.tt.Texts())
	}
	h.save(shots, "settings-languages")
	f.subtitles = "chi, eng"
	h.tt.Frame()
	if err := h.tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.core.prefs.GetSubtitleLanguages(), []string{"chi", "eng"}; !slices.Equal(got, want) {
		t.Errorf("subtitle languages = %q, want = %q", got, want)
	}
	if got := h.core.prefs.GetSubtitleMode(); got != userv1.SubtitleMode_SUBTITLE_MODE_FOREIGN {
		t.Errorf("subtitle mode = %v, want = kept", got)
	}

	if err := h.tt.Click("Devices"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"MacBook Pro", "This Device", "Living Room TV", "Mavio for Android TV · Playing The Long Orbit"} {
		if !h.tt.HasText(want) {
			t.Errorf("devices have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "settings-devices")
	if err := h.tt.Click("Sign Out"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.tt.HasText("Living Room TV") || len(h.core.sessions) != 1 {
		t.Errorf("sessions = %d, want = the television signed out", len(h.core.sessions))
	}

	if err := h.tt.Click(h.core.record().GetUserName()); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.app.router.Path(), "/settings/account"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
	for _, want := range []string{"Authorize a Device", "Servers", h.core.record().GetName()} {
		if !h.tt.HasText(want) {
			t.Errorf("account has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "settings-account")

	if err := h.tt.Click("Home"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.app.router.Path(), "/home"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
}

// TestSettingsNotAdmin checks that one who does not administer the server
// finds the issues, the only section of the maintenance's menu left, in
// the server's group.
func TestSettingsNotAdmin(t *testing.T) {
	h := newHarness(t, true, 1280, 1000)
	h.core.notAdmin = true
	// The sidebar loaded the user, an administrator, for its pinned tools.
	h.app.settingsState.users = nil
	h.app.router.Push("/settings/issues")
	h.settle()
	texts := h.tt.Texts()
	if !slices.Contains(texts, "Issues") || !slices.Contains(texts, "1 issue") {
		t.Errorf("no issues; texts = %q", texts)
	}
	for _, no := range []string{"Monitoring & Maintenance", "Backups", "Overview", "Advanced"} {
		if slices.Contains(texts, no) {
			t.Errorf("shows %q; texts = %q", no, texts)
		}
	}
	if got := settingsParent("/settings/issues", false); got != "/settings" {
		t.Errorf("settingsParent = %q, want = %q", got, "/settings")
	}
}

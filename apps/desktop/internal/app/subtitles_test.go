package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSubtitles(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	path := func(want string) {
		t.Helper()
		if got := h.app.router.Location(); got != want {
			t.Errorf("path = %q, want = %q", got, want)
		}
	}
	subs := h.core.subtitles
	locked := func(fn func()) {
		subs.mu.Lock()
		defer subs.mu.Unlock()
		fn()
	}
	st := &h.app.settingsState.admin.subtitles

	// The providers, translators, profiles and libraries.
	h.app.router.Push("/settings/subtitles")
	h.settle()
	want("OpenSubtitles", "TMDB · Movies + Episodes · Paused until in 2 hours: too many requests", "podnapisi", "TMDB · Movies · Off",
		"Google Translate", "French", "French (Forced), English", "Movies", "Shows", "Searching and Saving", "Synchronization", "Translation")
	h.save(shots, "settings-subtitles")

	// A profile edited: a language added, the cutoff and the names it must contain.
	h.click("French")
	path(subtitlesPath("profile", "sp1"))
	h.click("Add")
	if n := len(st.profile.GetItems()); n != 3 {
		t.Fatalf("len(items) = %d, want = 3", n)
	}
	st.mustContain = "WEB\n\nBluRay "
	h.settle()
	h.save(shots, "settings-subtitles-profile")
	h.click("Save")
	locked(func() {
		p := subs.profiles[0]
		if len(p.GetItems()) != 3 || !slices.Equal(p.GetMustContain(), []string{"WEB", "BluRay"}) {
			t.Errorf("profile = %v, want 3 languages and WEB, BluRay", p)
		}
	})

	// A new profile, then deleted.
	h.app.router.Push(subtitlesPath("profile", "new"))
	h.settle()
	want("New Subtitle Profile")
	st.profile.SetName("Anime")
	h.settle()
	h.click("Save")
	path(subtitlesPath("profile", "new1"))
	h.click("Delete Profile")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	path(subtitlesPath("", ""))
	locked(func() {
		if n := len(subs.profiles); n != 1 {
			t.Errorf("len(profiles) = %d, want = 1", n)
		}
	})

	// Searching and saving: a mod and an equivalence.
	h.app.router.Push(subtitlesPath("settings", ""))
	h.settle()
	want("Episodes", "None.")
	h.click("Remove SDH descriptions")
	h.click("Add")
	h.save(shots, "settings-subtitles-searching")
	h.click("Save")
	want("Saved.")
	locked(func() {
		if !slices.Contains(subs.settings.GetMods(), "remove_HI") || len(subs.settings.GetLanguageEquals()) != 1 {
			t.Errorf("settings = %v, want remove_HI and an equivalence", subs.settings)
		}
	})

	// Synchronization turned on.
	h.app.router.Push(subtitlesPath("sync", ""))
	h.settle()
	h.click("Synchronize downloads")
	want("Alignment")
	h.click("Save")
	locked(func() {
		if !subs.settings.GetSync().GetEnabled() {
			t.Errorf("sync = %v, want enabled", subs.settings.GetSync())
		}
	})

	// Translation: an AI model as a fallback.
	h.app.router.Push(subtitlesPath("translation", ""))
	h.settle()
	want("Google Translate", "Retries")
	st.draft.GetTranslation().SetFallbackTranslators([]string{"ai/anthropic/claude-sonnet-5-5"})
	h.settle()
	want("Claude Sonnet 5.5 (anthropic)")
	h.save(shots, "settings-subtitles-translation")
	h.click("Save")
	locked(func() {
		if got := subs.settings.GetTranslation().GetFallbackTranslators(); !slices.Equal(got, []string{"ai/anthropic/claude-sonnet-5-5"}) {
			t.Errorf("fallbacks = %q, want the model", got)
		}
	})
}

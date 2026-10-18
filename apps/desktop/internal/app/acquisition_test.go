package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func TestAcquisition(t *testing.T) {
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
	acq := h.core.acq
	locked := func(fn func()) {
		acq.mu.Lock()
		defer acq.mu.Unlock()
		fn()
	}
	st := &h.app.settingsState.admin.acquisition

	// The indexers, download clients, profiles and rules.
	h.app.router.Push("/settings/acquisition")
	h.settle()
	want("Nyaa", "TMDB · Torrent · Public · Priority 25", "Broken", "TMDB · Torrent · Priority 25 · Left out until in 3 hours: timed out",
		"dev.mavio.prowlarr: not reached", "qBittorrent", "Mail · Torrent · Priority 1 · Off", "HD-1080p", "Upgrades",
		"Quality Profiles: Series", "Quality Definitions", "Custom Formats", "Delay Profiles", "Release Profiles", "Remote Path Mappings")
	h.save(shots, "settings-acquisition")

	// A profile, allowing the best quality.
	h.click("HD-1080p")
	path(acquisitionPath("profile", "hd"))
	want("Movies", "Bluray-2160p", "Bluray-1080p", "x265")
	h.save(shots, "settings-acquisition-profile")
	items := slices.Clone(st.profile.GetItems())
	items[2].SetAllowed(true)
	st.profile.SetItems(items)
	h.settle()
	h.click("Save")
	want("Saved.")
	locked(func() {
		if !acq.profiles[0].GetItems()[2].GetAllowed() {
			t.Errorf("profile = %v, want Bluray-2160p allowed", acq.profiles[0])
		}
	})

	// A new one of series.
	h.app.router.Push(acquisitionPath("profile", "new-series"))
	h.settle()
	want("New Quality Profile", "Series", "WEBDL-1080p")
	st.profile.SetName("Anime")
	st.profile.GetItems()[0].SetAllowed(true)
	h.settle()
	h.click("Add")
	path(acquisitionPath("profile", "qp1"))
	locked(func() {
		if n := len(acq.profiles); n != 2 || acq.profiles[1].GetKind() != acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
			t.Errorf("profiles = %v, want Anime of series", acq.profiles)
		}
	})
	h.click("Delete Profile")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	path(acquisitionPath("", ""))
	locked(func() {
		if n := len(acq.profiles); n != 1 {
			t.Errorf("len(profiles) = %d, want = 1", n)
		}
	})

	// The qualities' sizes.
	h.app.router.Push(acquisitionPath("qualities", ""))
	h.settle()
	want("WEBDL-1080p", "Bluray-2160p", "Minimum")
	d := acquisitionv1.QualityDefinition_builder{Quality: new(int32(3)), Title: new("WEBDL-1080p"), MinSize: new(5.0), MaxSize: new(100.0), PreferredSize: new(50.0)}.Build()
	st.sizes = map[int32]*acquisitionv1.QualityDefinition{3: d}
	h.settle()
	h.click("Save")
	locked(func() {
		if got := acq.definitions[acquisitionv1.MediaKind_MEDIA_KIND_MOVIE][0].GetMinSize(); got != 5 {
			t.Errorf("min size = %v, want = 5", got)
		}
	})

	// A custom format.
	h.app.router.Push(acquisitionPath("formats", ""))
	h.settle()
	want("x265", "Release title")
	h.click("Add Custom Format")
	want("New Custom Format", "No conditions.")
	st.format.SetName("HDR")
	h.click("Add Condition")
	spec := st.format.GetSpecifications()[0]
	spec.SetName("HDR10")
	spec.SetValue(`\bHDR10\b`)
	h.settle()
	h.save(shots, "settings-acquisition-format")
	h.click("Add")
	path(acquisitionPath("formats", ""))
	want("HDR")

	// Delay profiles.
	h.app.router.Push(acquisitionPath("delays", ""))
	h.settle()
	want("Default", "Usenet + Torrent · Prefers Usenet · Usenet 0 min · Torrent 30 min")
	h.click("Add")
	st.delay.tags = "anime"
	h.settle()
	h.click("Save")
	want("anime")
	locked(func() {
		if n := len(acq.delays); n != 2 || acq.delays[0].GetTags()[0] != "anime" {
			t.Errorf("delays = %v, want anime first", acq.delays)
		}
	})

	// Release profiles.
	h.app.router.Push(acquisitionPath("releases", ""))
	h.settle()
	want("No release profiles.")
	h.click("Add")
	st.release.name, st.release.ignored = "No CAM", "CAM\nTS"
	h.settle()
	h.click("Save")
	want("No CAM", "2 ignored")

	// Remote path mappings.
	h.app.router.Push(acquisitionPath("mappings", ""))
	h.settle()
	want("seedbox", "/data/ → /mnt/seedbox/")
	h.click("Delete")
	locked(func() {
		if n := len(acq.mappings); n != 0 {
			t.Errorf("len(mappings) = %d, want = 0", n)
		}
	})
}

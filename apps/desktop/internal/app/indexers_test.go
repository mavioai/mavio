package app

import (
	"slices"
	"testing"
)

func TestIndexerSearch(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.app.router.Push("/settings/indexer-search")
	h.settle()
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	// The Advanced group, for an administrator, before a search.
	want("Advanced", "Indexers", "Enter what to search for.")

	// A search lists the releases in a table, the failed indexers above.
	h.app.settingsState.indexers.query = "orbit"
	h.settle()
	h.click("Search")
	want("Orbit.2024.1080p.BluRay.x265-GRP", "Orbit.2024.CAM", "Torrent", "Movies", "42 / 50", "12", "6.4 GB")
	if !h.tt.HasText("timed out") {
		t.Errorf("no failure; texts = %q", h.tt.Texts())
	}
	h.save(shots, "settings-indexer-search")

	// Sorting by size puts the larger first; again, the smaller.
	first := func() string {
		cam, ok1 := h.tt.Find("Orbit.2024.CAM")
		hd, ok2 := h.tt.Find("Orbit.2024.1080p.BluRay.x265-GRP")
		if !ok1 || !ok2 {
			t.Fatalf("releases not shown")
		}
		if cam.Y < hd.Y {
			return "cam"
		}
		return "hd"
	}
	h.click("Size")
	if got := first(); got != "hd" {
		t.Errorf("first by size = %s, want = hd", got)
	}
	h.click("Size")
	if got := first(); got != "cam" {
		t.Errorf("first by size ascending = %s, want = cam", got)
	}

	// Sending a release to a download client without an item.
	h.click("Send to Download Client")
	if got, want := h.core.downloads.grabbed, []string{" Orbit.2024.CAM"}; !slices.Equal(got, want) {
		t.Errorf("grabbed = %q, want = %q", got, want)
	}
	h.click("Open Release Page")
	if got, want := h.opened, []string{"https://nyaa.example/view/1"}; !slices.Equal(got, want) {
		t.Errorf("opened = %q, want = %q", got, want)
	}

	// Pinned, the tool is a place of the sidebar's Features.
	h.app.togglePin("indexer-search")
	h.app.router.Push("/home")
	h.settle()
	want("Features", "Indexers")
	h.click("Indexers")
	if got := h.app.router.Path(); got != "/tools/indexer-search" {
		t.Errorf("path = %q, want = /tools/indexer-search", got)
	}
	want("Orbit.2024.CAM")
	h.save(shots, "tool-indexer-search")
}

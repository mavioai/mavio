package app

import (
	"slices"
	"testing"
)

func TestWanted(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	w := h.core.wanted
	locked := func(fn func()) {
		w.mu.Lock()
		defer w.mu.Unlock()
		fn()
	}
	st := &h.app.settingsState.admin.wanted

	// The missing movies, all searched for, and an RSS sync.
	h.app.router.Push("/settings/wanted")
	h.settle()
	want("Dune (2021)", "Arrival (2016)", "Not available yet", "2 items")
	h.save(shots, "settings-wanted")
	h.click("Search All")
	h.click("RSS Sync")
	want("2 searches queued · RSS sync queued")
	locked(func() {
		if !slices.Equal(w.searched, []string{"all"}) || !w.rssSynced {
			t.Errorf("searched = %q, rssSynced = %v, want all searched and synced", w.searched, w.rssSynced)
		}
	})

	// A movie unmonitored.
	h.click("Unmonitor")
	if h.tt.HasText("Dune (2021)") {
		t.Errorf("Dune was not unmonitored")
	}

	// An episode searched for.
	st.kind = 1
	h.settle()
	want("Severance · S02E03 · Who Is Alive?", "Has WEBDL-720p", "1 item")
	h.click("Search")
	want("Searching")
	locked(func() {
		if !slices.Contains(w.searched, "e1") {
			t.Errorf("searched = %q, want e1", w.searched)
		}
	})
}

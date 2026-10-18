package app

import (
	"slices"
	"testing"
)

func TestDownloads(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	d := h.core.downloads
	locked := func(fn func()) {
		d.mu.Lock()
		defer d.mu.Unlock()
		fn()
	}
	st := &h.app.settingsState.admin.downloads

	// The queue: a download's progress, and one blocked, imported by hand.
	h.app.router.Push("/settings/downloads")
	h.settle()
	want("Dune", "Downloading", "Dune.2021.1080p.BluRay.x265-GRP", "Bluray-1080p · English · 4.2 GB · Nyaa · 25 min left · 2 hours ago",
		"Arrival", "Import blocked", "350 MB", "No files found are eligible for import", "2 downloads")
	h.save(shots, "settings-downloads")
	h.click("Import Manually")
	want("Arrival.mkv", "2.5 GB · WEBDL-2160p · GRP")
	h.click("Import")
	want("Imported")
	locked(func() {
		if len(d.imported) != 1 || d.imported[0].GetId() != "q2" || d.imported[0].GetPath() != "/downloads/Arrival/Arrival.mkv" {
			t.Errorf("imported = %v, want Arrival.mkv of q2", d.imported)
		}
	})

	// A download removed, its release blocklisted.
	st.panels = nil
	h.settle()
	h.click("Remove")
	h.click("Blocklist the release")
	want("Search for another release")
	h.save(shots, "settings-downloads-remove")
	st.removals["q1"].search = true
	h.settle()
	// The panel's Remove, after its Cancel.
	r, _ := h.tt.Find("Cancel")
	h.tt.ClickAt(r.X+r.W+30, r.Y+r.H/2)
	h.settle()
	locked(func() {
		if len(d.removed) != 1 || d.removed[0].GetId() != "q1" || !d.removed[0].GetRemoveFromClient() || !d.removed[0].GetBlocklist() || !d.removed[0].GetSearch() {
			t.Errorf("removed = %v, want q1 from the client, blocklisted and searched", d.removed)
		}
	})

	// The history, a grab marked failed.
	st.tab = 1
	h.settle()
	want("Grabbed", "indexer=Nyaa", "Heat.1995.720p", "2 events")
	h.click("Mark Failed")
	want("Blocklist this release and search again?")
	h.click("Mark Failed")
	locked(func() {
		if !slices.Equal(d.failed, []string{"h1"}) {
			t.Errorf("failed = %q, want h1", d.failed)
		}
	})
	st.event = 2
	h.settle()
	want("1 event")

	// The blocklist, an entry removed.
	st.tab = 2
	h.settle()
	want("Dune.2021.CAM", "Manually marked failed")
	h.click("Remove")
	want("The blocklist is empty.")

	// A folder's files.
	st.tab = 3
	st.path = "/mnt/incoming"
	h.settle()
	h.click("Scan")
	want("Unknown.mkv", "Unknown Movie")
}

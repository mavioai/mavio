package app

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

func TestLibraries(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	library := func(id string) *libraryv1.Library {
		t.Helper()
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		i := slices.IndexFunc(h.core.libraries, func(l *libraryv1.Library) bool { return l.GetId() == id })
		if i < 0 {
			return nil
		}
		return h.core.libraries[i]
	}
	form := &h.app.settingsState.admin.libraries.form

	h.app.router.Push("/settings/libraries")
	h.settle()
	want("Movies", "Shows", "Music", "Add Library")

	// A library's page: its folders, with their storage, and options.
	h.click("Movies")
	want("/media/movies", "/media/4k", "Automatic (Local Disk)", "Automatic (Network Share)", "Metadata Language", "Server Default",
		"Group Movies into Collections", "Scan Automatically", "Never", "Seek Thumbnails")
	h.click("Seek Thumbnails")
	h.click("Save")
	want("Saved")
	if !library("movies").GetExtractTrickplay() {
		t.Errorf("trickplay of movies = false, want = true")
	}

	// Deleting it, once confirmed.
	h.click("Delete")
	want("Delete Movies?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if library("movies") != nil {
		t.Errorf("movies was not deleted")
	}
	want("Add Library")

	// Adding one, of a folder browsed to and one of a remote.
	h.click("Add Library")
	want("Drives", "remote:drive:")
	h.click("/")
	h.click("media")
	want("anime", "movies", "Up")
	h.click("anime")
	h.click("Add This Folder")
	h.click("Up")
	h.click("Up")
	h.click("Up")
	h.click("remote:drive:")
	want("films")
	h.click("films")
	h.click("Add This Folder")
	form.draft.name = "Anime"
	h.click("Content")
	form.draft.kind = libraryv1.LibraryKind_LIBRARY_KIND_SHOWS
	h.settle()
	want("Missing Episodes", "Keep for Days")
	h.save(shots, "settings-library-new")
	// The button adding the library is the second "Add", after the
	// folder's.
	h.app.saveLibrary(nil)
	h.settle()
	want("Anime", "Seek Thumbnails")
	l := library("new-lib")
	if l == nil || !slices.Equal(l.GetPaths(), []string{"/media/anime", "remote:drive:films"}) || l.GetKind() != libraryv1.LibraryKind_LIBRARY_KIND_SHOWS ||
		l.GetScanInterval().AsDuration() != 12*time.Hour {
		t.Errorf("library = %v, want = Anime of shows in /media/anime and remote:drive:films, scanned every 12 hours", l)
	}
	h.save(shots, "settings-library")
}

func TestLibraryScans(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	h.core.mu.Lock()
	for i, l := range h.core.libraries {
		if l.GetId() == "movies" {
			l = proto.CloneOf(l)
			l.SetScanProgress(libraryv1.ScanProgress_builder{
				LibraryId: new("movies"), Phase: new(libraryv1.ScanPhase_SCAN_PHASE_FOLDERS), FoldersDone: new(int32(2)), FoldersTotal: new(int32(5)), Fraction: new(0.1),
			}.Build())
			h.core.libraries[i] = l
		}
	}
	h.core.mu.Unlock()

	// A scan under way when the libraries are listed.
	h.app.router.Push("/settings/libraries")
	h.settle()
	want("Reading folders · 2 of 5")

	// Its progress as the server tells it, then its end.
	server := h.app.chosenServer().GetId()
	h.app.Event(capiv1.Event_builder{ServerId: server, ScanProgressChanged: capiv1.ScanProgressChanged_builder{
		LibraryId: "movies", Progress: capiv1.ScanProgress_builder{ItemsDone: 3, ItemsTotal: 10, Fraction: 0.5}.Build(),
	}.Build()}.Build())
	h.settle()
	want("Analyzing media · 3 of 10")
	h.save(shots, "settings-libraries-scan")
	h.app.Event(capiv1.Event_builder{ServerId: server, ScanProgressChanged: capiv1.ScanProgressChanged_builder{LibraryId: "movies"}.Build()}.Build())
	h.settle()
	want("Scan finished")
}

package store_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestScanGenerationsAndMissingItems(t *testing.T) {
	byName := []core.SortSpec{{Field: core.SortName}}
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		gen, err := s.Scans().NextGeneration(ctx, lib.ID)
		if err != nil || gen != 1 {
			t.Fatalf("first generation = %d, %v", gen, err)
		}
		if gen, _ = s.Scans().NextGeneration(ctx, lib.ID); gen != 2 {
			t.Fatalf("second generation = %d", gen)
		}
		if _, err := s.Scans().NextGeneration(ctx, core.NewID()); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unknown library: %v", err)
		}

		mk := func(name, path string, g int64) core.Item {
			it := newItem(lib, core.KindMovie, name)
			it.Path, it.ScanGeneration = path, g
			return it
		}
		seen := mk("Seen", "/media/Seen/Seen.mkv", 2)
		gone := mk("Gone", "/media/Gone/Gone.mkv", 1)
		unreadable := mk("Unreadable", "/media/Disk/A.mkv", 1)
		// Shares a prefix with /media/Disk but lies outside it; differs in case.
		sibling := mk("Sibling", "/media/Disk2/B.mkv", 1)
		upper := mk("Upper", "/media/DISK/C.mkv", 1)
		virtual := newItem(lib, core.KindSeries, "Virtual")
		upsert(t, s, seen, gone, unreadable, sibling, upper, virtual)

		// The scan could not read /media/Disk, so its items count as seen.
		if err := s.Items().MarkSeen(ctx, lib.ID, "/media/Disk/", 2); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		n, err := s.Items().MarkMissing(ctx, lib.ID, 2, now)
		if err != nil || n != 3 {
			t.Fatalf("MarkMissing = %d, %v; want 3 (gone, sibling, upper)", n, err)
		}
		page, err := s.Items().Query(ctx, core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, Sort: byName})
		if err != nil {
			t.Fatal(err)
		}
		if got := names(page.Items); !slices.Equal(got, []string{"Seen", "Unreadable", "Virtual"}) {
			t.Errorf("present items = %v", got)
		}
		page, _ = s.Items().Query(ctx, core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, IncludeMissing: true, Sort: byName})
		if len(page.Items) != 6 {
			t.Errorf("with missing = %v", names(page.Items))
		}
		g, err := s.Items().Get(ctx, gone.ID)
		if err != nil || g.MissingSince == nil || !g.MissingSince.Equal(now) {
			t.Errorf("gone = %+v, %v", g.MissingSince, err)
		}
		// Marking again keeps the first time.
		if n, _ := s.Items().MarkMissing(ctx, lib.ID, 2, now.Add(time.Hour)); n != 0 {
			t.Errorf("second MarkMissing = %d", n)
		}

		// Touch marks exact paths only.
		if err := s.Items().Touch(ctx, lib.ID, 3, "/media/Seen/Seen.mkv", "/media/Disk"); err != nil {
			t.Fatal(err)
		}
		if g, _ := s.Items().Get(ctx, seen.ID); g.ScanGeneration != 3 {
			t.Errorf("touched = %d", g.ScanGeneration)
		}
		if g, _ := s.Items().Get(ctx, unreadable.ID); g.ScanGeneration != 2 {
			t.Errorf("below a touched path = %d", g.ScanGeneration)
		}

		// A file back in place is seen again.
		if err := s.Items().MarkSeen(ctx, lib.ID, "/media/Gone/Gone.mkv", 3); err != nil {
			t.Fatal(err)
		}
		if g, _ := s.Items().Get(ctx, gone.ID); g.MissingSince != nil || g.ScanGeneration != 3 {
			t.Errorf("gone after return = %+v", g)
		}

		ids, err := s.Items().PurgeMissing(ctx, lib.ID, now.Add(time.Minute))
		slices.SortFunc(ids, compareIDs)
		want := []core.ID{sibling.ID, upper.ID}
		slices.SortFunc(want, compareIDs)
		if err != nil || !slices.Equal(ids, want) {
			t.Errorf("PurgeMissing = %v, %v; want %v", ids, err, want)
		}
		if _, err := s.Items().Get(ctx, sibling.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("purged item: %v", err)
		}
	})
}

func TestFolderStates(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
		a := core.FolderState{
			LibraryID: lib.ID, Path: "/media/A", ModTime: mod, FileID: "1:42", Generation: 1,
			Entries: []core.FolderEntry{{Name: "A.mkv", Size: 100, ModTime: mod}, {Name: "extras", IsDir: true}},
		}
		b := core.FolderState{LibraryID: lib.ID, Path: "/media/B", Generation: 1}
		if err := s.Scans().PutFolders(ctx, a, b); err != nil {
			t.Fatal(err)
		}
		got, err := s.Scans().Folder(ctx, lib.ID, "/media/A")
		if err != nil || !got.ModTime.Equal(mod) || got.FileID != "1:42" || len(got.Entries) != 2 ||
			got.Entries[0].Size != 100 || !got.Entries[0].ModTime.Equal(mod) || !got.Entries[1].IsDir {
			t.Fatalf("Folder = %+v, %v", got, err)
		}
		a.Generation, a.FileID = 2, "1:43"
		if err := s.Scans().PutFolders(ctx, a); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Scans().Folder(ctx, lib.ID, "/media/A"); got.FileID != "1:43" || got.Generation != 2 {
			t.Errorf("replaced = %+v", got)
		}
		if err := s.Scans().DeleteFolders(ctx, lib.ID, 2); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Scans().Folder(ctx, lib.ID, "/media/B"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("stale folder: %v", err)
		}
		// Deleting the library removes its folder states.
		if err := s.Libraries().Delete(ctx, lib.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Scans().Folder(ctx, lib.ID, "/media/A"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("after library delete: %v", err)
		}
	})
}

func TestMediaSourceScanFields(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		movie := newItem(lib, core.KindMovie, "Rip")
		upsert(t, s, movie)
		mod := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
		src := core.MediaSource{Path: "/media/Rip/Rip-cd1.avi", Parts: []string{"/media/Rip/Rip-cd2.avi"}, Disc: "", Modified: mod, Size: 7}
		disc := core.MediaSource{Path: "/media/Rip", Disc: core.DiscBluRay}
		if err := s.MediaSources().Replace(ctx, movie.ID, []core.MediaSource{src, disc}); err != nil {
			t.Fatal(err)
		}
		got, err := s.MediaSources().ListForItem(ctx, movie.ID)
		if err != nil || len(got) != 2 || !slices.Equal(got[0].Parts, src.Parts) || !got[0].Modified.Equal(mod) || got[1].Disc != core.DiscBluRay {
			t.Errorf("round trip = %+v, %v", got, err)
		}
		if err := s.MediaSources().Replace(ctx, movie.ID, []core.MediaSource{{Path: "/x", Disc: "hddvd"}}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("unknown disc kind: %v", err)
		}
	})
}

func compareIDs(a, b core.ID) int { return bytes.Compare(a[:], b[:]) }

package events

import (
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/store"
)

func TestScanProgress(t *testing.T) {
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()
	films := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	if err := s.Libraries().Create(ctx, &films); err != nil {
		t.Fatal(err)
	}
	admin := core.User{ID: core.NewID(), Name: "admin", Kind: core.UserAdmin}

	synctest.Test(t, func(t *testing.T) {
		hub := New(Config{Store: s})
		db := Observe(s, hub)
		tv := hub.Subscribe(admin, core.NewID())
		drain(tv)
		// progress returns the last progress sent, and whether the scan
		// ended.
		progress := func() (*libraryv1.ScanProgress, bool) {
			t.Helper()
			var last *libraryv1.ScanProgress
			ended := false
			for e := next(tv); e != nil; e = next(tv) {
				if c := e.GetScanProgressChanged(); c != nil {
					last, ended = c.GetProgress(), !c.HasProgress()
				}
			}
			return last, ended
		}
		run := func(kind string) core.Job {
			t.Helper()
			job, err := db.Jobs().Lease(ctx, "test", []string{kind}, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			return job
		}

		scan := library.ScanJob(films, time.Now(), false)
		if _, err := db.Jobs().Enqueue(ctx, &scan); err != nil {
			t.Fatal(err)
		}
		scan = run(library.JobScan)
		if p, _ := progress(); p.GetPhase() != libraryv1.ScanPhase_SCAN_PHASE_FOLDERS || p.GetFraction() != 0 {
			t.Errorf("begun = %v, want reading folders", p)
		}

		// Reading folders is sent at most twice a second.
		hub.ScanFolders(films.ID, 0, 1)
		hub.ScanFolders(films.ID, 1, 4)
		if p, _ := progress(); p != nil {
			t.Errorf("progress = %v, sent sooner than the interval", p)
		}
		time.Sleep(progressInterval)
		synctest.Wait()
		if p, _ := progress(); p.GetFoldersDone() != 1 || p.GetFoldersTotal() != 4 || p.GetFraction() != foldersShare/4 {
			t.Errorf("folders = %v, want 1 of 4", p)
		}

		// The probes the scan queues are counted; other jobs are not.
		for range 2 {
			job := core.Job{
				ID: core.NewID(), Kind: library.JobProbe, MaxAttempts: 1, RunAt: time.Now(),
				Payload: []byte(`{"item_id":"` + core.NewID().String() + `","library_id":"` + films.ID.String() + `"}`),
			}
			if _, err := db.Jobs().Enqueue(ctx, &job); err != nil {
				t.Fatal(err)
			}
		}
		other := library.RefreshJob(core.NewID(), time.Now())
		if _, err := db.Jobs().Enqueue(ctx, &other); err != nil {
			t.Fatal(err)
		}
		hub.ScanFolders(films.ID, 4, 4)
		if err := db.Jobs().Complete(ctx, scan.ID, "test", ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(progressInterval)
		synctest.Wait()
		p, _ := progress()
		if p.GetPhase() != libraryv1.ScanPhase_SCAN_PHASE_ITEMS || p.GetItemsDone() != 0 || p.GetItemsTotal() != 2 || p.GetFraction() != foldersShare {
			t.Errorf("after the folders = %v, want 0 of 2 items", p)
		}
		if got := hub.ScanProgress(films.ID); got.GetItemsTotal() != 2 {
			t.Errorf("ScanProgress = %v, want 2 items", got)
		}

		// A failed probe counts as done; the last one ends the scan.
		first := run(library.JobProbe)
		if err := db.Jobs().Fail(ctx, first.ID, "test", errors.New("boom")); err != nil {
			t.Fatal(err)
		}
		time.Sleep(progressInterval)
		synctest.Wait()
		if p, _ := progress(); p.GetItemsDone() != 1 || p.GetFraction() != foldersShare+(1-foldersShare)/2 {
			t.Errorf("half done = %v", p)
		}
		second := run(library.JobProbe)
		if err := db.Jobs().Complete(ctx, second.ID, "test", ""); err != nil {
			t.Fatal(err)
		}
		if _, ended := progress(); !ended || hub.ScanProgress(films.ID) != nil {
			t.Error("the scan did not end with its last probe")
		}

		// The next scan expects the folders of the last.
		scan = library.ScanJob(films, time.Now(), false)
		if _, err := db.Jobs().Enqueue(ctx, &scan); err != nil {
			t.Fatal(err)
		}
		run(library.JobScan)
		if p, _ := progress(); p.GetFoldersTotal() != 4 {
			t.Errorf("next scan = %v, want 4 folders expected", p)
		}
	})
}

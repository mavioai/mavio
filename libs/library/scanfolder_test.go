package library

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestScanFolder(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "Brazil (1985)/Brazil (1985).mkv")
	f.scan()

	// A new movie folder and a file gone elsewhere: only the new folder
	// is scanned, and the other movie is left for the library's scan.
	tree(t, f.root, "Heat (1995)/Heat (1995).mkv", "Heat (1995)/Heat (1995)-trailer.mkv")
	if err := os.Remove(f.root + "/Brazil (1985)/Brazil (1985).mkv"); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Hour)
	st, err := f.sc.ScanFolder(t.Context(), f.lib, f.root+"/Heat (1995)")
	if err != nil {
		t.Fatal(err)
	}
	got := f.items()
	if got["Heat (1995)/Heat (1995).mkv"] != core.KindMovie || got["Heat (1995)/Heat (1995)-trailer.mkv"] != core.KindVideo {
		t.Errorf("items = %v, want = Heat and its trailer", got)
	}
	if _, ok := got["Brazil (1985)/Brazil (1985).mkv"]; !ok || st.Missing != 0 {
		t.Errorf("Brazil present = %v, missing = %d, want = kept until the library's scan", ok, st.Missing)
	}
	if heat := f.item("Heat (1995)/Heat (1995).mkv"); heat.Name != "Heat" || heat.ProductionYear != 1995 {
		t.Errorf("Heat = %q %d", heat.Name, heat.ProductionYear)
	}

	// The library's scan then marks the gone movie missing, and keeps
	// what the folder's scan found.
	if st := f.scan(); st.Missing != 1 {
		t.Errorf("library scan missing = %d, want = 1", st.Missing)
	}
	got = f.items()
	if _, ok := got["Brazil (1985)/Brazil (1985).mkv"]; ok || got["Heat (1995)/Heat (1995).mkv"] != core.KindMovie || got["Up (2009)/Up (2009).mkv"] != core.KindMovie {
		t.Errorf("items after the library's scan = %v", got)
	}

	if _, err := f.sc.ScanFolder(t.Context(), f.lib, t.TempDir()); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("ScanFolder(outside) err = %v, want = ErrInvalid", err)
	}

	// The job scans the folder.
	f.store.libs[f.lib.ID] = f.lib
	tree(t, f.root, "Ran (1985)/Ran (1985).mkv")
	j := &Jobs{Store: f.store, Scanner: f.sc}
	if _, err := j.Handlers()[JobScanFolder](t.Context(), ScanFolderJob(f.lib, f.root+"/Ran (1985)", f.clock)); err != nil {
		t.Fatal(err)
	}
	if f.items()["Ran (1985)/Ran (1985).mkv"] != core.KindMovie {
		t.Errorf("items after the job = %v, want = Ran", f.items())
	}
}

func TestScanFolderJob(t *testing.T) {
	lib := core.Library{ID: core.NewID()}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	job := ScanFolderJob(lib, "/movies/Heat (1995)/", at)
	var p FolderPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if job.Kind != JobScanFolder || p.LibraryID != lib.ID || p.Path != "/movies/Heat (1995)" || !job.RunAt.Equal(at) ||
		job.UniqueKey != JobScanFolder+":"+lib.ID.String()+":/movies/Heat (1995)" {
		t.Errorf("ScanFolderJob = %+v, payload %+v", job, p)
	}
}

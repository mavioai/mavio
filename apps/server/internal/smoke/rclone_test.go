package smoke_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/rclone"
	"github.com/mavioai/mavio/libs/store"
	"github.com/mavioai/mavio/tools/fixtures"
)

// TestRcloneLibrary scans a movie library on an rclone remote through an
// rcd the test runs: the scanner lists the remote, ffprobe reads the movie
// through the media proxy, and its keyframes come from its index, read in
// ranges through the rcd.
func TestRcloneLibrary(t *testing.T) {
	ctx := t.Context()
	bin, err := exec.LookPath("rclone")
	if err != nil {
		t.Skip("smoke test: rclone not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("smoke test: ffprobe not found on PATH")
	}
	film := fixtures.Require(t, "keyframes.mkv")

	dir := t.TempDir()
	disk := filepath.Join(dir, "disk")
	movie := filepath.Join(disk, "Movies", "Film (2020)", "Film (2020).mkv")
	if err := os.MkdirAll(filepath.Dir(movie), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(film)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(movie, data, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := rclone.StartDaemon(ctx, rclone.DaemonOptions{Path: bin, Config: filepath.Join(dir, "rclone", "rclone.conf")})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Client().CreateRemote(ctx, "disk", "alias", map[string]string{"remote": disk}); err != nil {
		t.Fatal(err)
	}
	proxy := rclone.NewProxy(d.Client(), nil)
	if err := proxy.Listen(); err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	remotes := &rclone.Remotes{Client: d.Client(), MediaURL: proxy.URL}
	storages := &library.Storages{Remote: remotes.Open}

	s, err := store.Open(ctx, "sqlite:"+filepath.Join(dir, "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	lib := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{"rclone:disk:Movies"}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	jobs := &library.Jobs{
		Store:     s,
		Scanner:   &library.Scanner{Store: s, Resolver: library.NewResolver(), Storages: storages},
		Storages:  storages,
		Prober:    providers.FFprobe{Prober: &probe.Prober{FFprobe: ffprobe}},
		Keyframes: providers.Keyframes{Extractor: &keyframes.Extractor{FFprobe: ffprobe}},
		Refresher: &library.Refresher{Store: s, Storages: storages},
	}
	worker := &library.Worker{Queue: s.Jobs(), Owner: "smoke", Handlers: jobs.Handlers()}
	scan := library.ScanJob(lib, time.Now(), false)
	if _, err := s.Jobs().Enqueue(ctx, &scan); err != nil {
		t.Fatal(err)
	}
	for {
		ran, err := worker.RunOne(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !ran {
			break
		}
	}

	const path = "rclone:disk:Movies/Film (2020)/Film (2020).mkv"
	it, err := s.Items().GetByPath(ctx, lib.ID, path)
	if err != nil {
		t.Fatalf("GetByPath(%s): %v", path, err)
	}
	if it.Kind != core.KindMovie || it.Name != "Film" || it.ProductionYear != 2020 {
		t.Errorf("item = %s %q (%d), want movie Film (2020)", it.Kind, it.Name, it.ProductionYear)
	}
	sources, err := s.MediaSources().ListForItem(ctx, it.ID)
	if err != nil || len(sources) != 1 {
		t.Fatalf("media sources = %v, %v", sources, err)
	}
	ms := sources[0]
	if ms.Container == "" || len(ms.Streams) == 0 || ms.Duration <= 0 {
		t.Errorf("media source = %s, %d streams, %v, want it probed", ms.Container, len(ms.Streams), ms.Duration)
	}
	if len(ms.Keyframes) == 0 {
		t.Error("media source has no keyframes, want them from the index")
	}

	// Direct play reads the file through its storage.
	f, err := storages.OpenFile(ctx, "rclone:disk:Movies", "Film (2020)/Film (2020).mkv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Seek(int64(len(data)/2), io.SeekStart); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(f)
	if err != nil || len(rest) != len(data)-len(data)/2 {
		t.Errorf("read the second half = %d bytes, %v, want = %d", len(rest), err, len(data)-len(data)/2)
	}
}

package blackhole

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/blackhole/disk"
)

const title = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"

// clock is a clock the tests move.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func env(c *clock) client.Env {
	return client.Env{Now: c.now, Log: slog.New(slog.DiscardHandler)}
}

func write(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// givenCompletedItem is Radarr's GivenCompletedItem: a folder of the
// title holding a video file.
func givenCompletedItem(t *testing.T, watch string) string {
	t.Helper()
	file := filepath.Join(watch, title, "somefile.mkv")
	write(t, file, strings.Repeat("x", 1000))
	return file
}

// givenChangedItem is Radarr's GivenChangedItem: the file grows by a byte.
func givenChangedItem(t *testing.T, file string) {
	t.Helper()
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("x"); err != nil {
		t.Fatal(err)
	}
}

func scanItems(t *testing.T, s *scanner, watch string) []watchItem {
	t.Helper()
	items, err := s.items(context.Background(), watch, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("items() error = %v", err)
	}
	return items
}

func verifySingleItem(t *testing.T, s *scanner, watch string, want client.Status) {
	t.Helper()
	items := scanItems(t, s, watch)
	if len(items) != 1 {
		t.Fatalf("items() = %d items, want = 1", len(items))
	}
	if items[0].Status != want {
		t.Errorf("Status = %v, want = %v", items[0].Status, want)
	}
}

func TestScanLockedFilesQueued(t *testing.T) {
	c := newClock()
	watch := t.TempDir()
	givenCompletedItem(t, watch)
	s := newScanner(env(c))
	s.locked = func(string) bool { return true }
	c.advance(60 * time.Millisecond)
	verifySingleItem(t, s, watch, client.Downloading)
}

func TestScanChangingFilesQueued(t *testing.T) {
	c := newClock()
	watch := t.TempDir()
	file := givenCompletedItem(t, watch)
	s := newScanner(env(c))
	verifySingleItem(t, s, watch, client.Downloading)
	// Changed every 10ms, the item stays downloading.
	for range 10 {
		givenChangedItem(t, file)
		verifySingleItem(t, s, watch, client.Downloading)
		c.advance(10 * time.Millisecond)
	}
	// Until it remains unchanged for the wait period.
	c.advance(60 * time.Millisecond)
	verifySingleItem(t, s, watch, client.Completed)
}

func TestScanSpecialSubfolders(t *testing.T) {
	for _, name := range []string{"@eaDir", ".@__thumb"} {
		t.Run(name, func(t *testing.T) {
			watch := t.TempDir()
			write(t, filepath.Join(watch, name, "somefile.mkv"), "x")
			if got := scanItems(t, newScanner(env(newClock())), watch); len(got) != 0 {
				t.Errorf("items() = %+v, want = none", got)
			}
		})
	}
}

func TestScanItems(t *testing.T) {
	c := newClock()
	watch := t.TempDir()
	write(t, filepath.Join(watch, title, "somefile.mkv"), "12345")
	write(t, filepath.Join(watch, title, "sub", "sample.nfo"), "123")
	write(t, filepath.Join(watch, "Movie.2020.mkv"), "1234567")
	for _, skipped := range []string{"notes.txt", ".DS_Store", "Movie.2020-trailer.mkv", "Thumbs.db"} {
		write(t, filepath.Join(watch, skipped), "x")
	}
	s := newScanner(env(c))
	c.advance(time.Second)
	items := scanItems(t, s, watch)
	if len(items) != 2 {
		t.Fatalf("items() = %+v, want = the folder and the video file", items)
	}
	folder, file := items[0], items[1]
	fi, _ := os.Stat(filepath.Join(watch, title))
	if want := title + "_" + strconv.FormatInt(disk.Ticks(disk.CreationTime(fi)), 10); folder.ID != want {
		t.Errorf("folder ID = %q, want = %q", folder.ID, want)
	}
	if folder.Title != title || folder.TotalSize != 8 || folder.OutputPath != filepath.Join(watch, title) {
		t.Errorf("folder = %+v, want = the title, 8 bytes and the folder", folder)
	}
	fi, _ = os.Stat(filepath.Join(watch, "Movie.2020.mkv"))
	if want := "Movie.2020.mkv_" + strconv.FormatInt(disk.Ticks(fi.ModTime()), 10); file.ID != want {
		t.Errorf("file ID = %q, want = %q", file.ID, want)
	}
	if file.Title != "Movie.2020.mkv" || file.TotalSize != 7 || file.OutputPath != filepath.Join(watch, "Movie.2020.mkv") {
		t.Errorf("file = %+v, want = the file name, 7 bytes and the file", file)
	}
}

// An item unchanged for an hour is no longer read: what it grows to is
// not seen while the scans are cached.
func TestScanStableItemNotReread(t *testing.T) {
	c := newClock()
	watch := t.TempDir()
	file := givenCompletedItem(t, watch)
	s := newScanner(env(c))
	scanItems(t, s, watch)
	for range 16 {
		c.advance(4 * time.Minute)
		scanItems(t, s, watch)
	}
	givenChangedItem(t, file)
	c.advance(time.Minute)
	items := scanItems(t, s, watch)
	if len(items) != 1 || items[0].TotalSize != 1000 || items[0].Status != client.Completed {
		t.Errorf("items() = %+v, want = the item as it was, completed", items)
	}
	// Once the scans expire, the item is read again.
	c.advance(cacheTTL + time.Second)
	items = scanItems(t, s, watch)
	if len(items) != 1 || items[0].TotalSize != 1001 || items[0].Status != client.Downloading {
		t.Errorf("items() = %+v, want = the item read again, downloading", items)
	}
}

func TestScanMissingFolder(t *testing.T) {
	s := newScanner(env(newClock()))
	if _, err := s.items(context.Background(), filepath.Join(t.TempDir(), "missing"), time.Second); err == nil {
		t.Errorf("items() error = nil, want an error")
	}
}

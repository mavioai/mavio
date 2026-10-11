package pneumatic

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/blackhole/disk"
)

const title = "30.Rock.S01E05.hdtv.xvid-LoL"

var nzb = []byte("<nzb/>")

// fixture is the folders of Radarr's PneumaticProviderFixture in a
// temporary folder.
type fixture struct {
	nzbFolder, strmFolder string
}

func newFixture(t *testing.T) *fixture {
	dir := t.TempDir()
	f := &fixture{nzbFolder: filepath.Join(dir, "pneumatic"), strmFolder: filepath.Join(dir, "unsorted tv")}
	for _, d := range []string{f.nzbFolder, f.strmFolder} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) client(t *testing.T) *pneumatic {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"nzb_folder": f.nzbFolder, "strm_folder": f.strmFolder})
	c, err := Definitions()[0].New(raw, client.Env{Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c.(*pneumatic)
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestDownload(t *testing.T) {
	tests := []struct{ title, want string }{
		// should_download_file_if_it_doesnt_exist
		{title, title},
		// should_replace_illegal_characters_in_title
		{"Saturday Night Live - S38E08 - Jeremy Renner/Maroon 5 [SDTV]", "Saturday Night Live - S38E08 - Jeremy Renner+Maroon 5 [SDTV]"},
	}
	for _, tt := range tests {
		f := newFixture(t)
		id, err := f.client(t).Add(context.Background(), &client.Release{Title: tt.title, Data: nzb})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		nzbFile := filepath.Join(f.nzbFolder, tt.want+".nzb")
		if got := read(t, nzbFile); got != string(nzb) {
			t.Errorf("NZB = %q, want = %q", got, nzb)
		}
		strmFile := filepath.Join(f.strmFolder, tt.want+".strm")
		want := "plugin://plugin.program.pneumatic/?mode=strm&type=add_file&nzb=" + nzbFile + "&nzbname=" + tt.want
		if got := read(t, strmFile); got != want {
			t.Errorf("strm = %q, want = %q", got, want)
		}
		fi, _ := os.Stat(strmFile)
		if want := tt.want + ".strm_" + strconv.FormatInt(disk.Ticks(fi.ModTime()), 10); id != want {
			t.Errorf("Add() = %q, want = %q", id, want)
		}
	}
}

// should_throw_on_failed_download: the NZB cannot be written.
func TestDownloadFailed(t *testing.T) {
	f := newFixture(t)
	c := f.client(t)
	if err := os.Remove(f.nzbFolder); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Add(context.Background(), &client.Release{Title: title, Data: nzb}); err == nil {
		t.Errorf("Add() error = nil, want an error")
	}
	if _, err := c.Add(context.Background(), &client.Release{Title: title}); err == nil {
		t.Errorf("Add() without an NZB error = nil, want an error")
	}
}

// should_throw_if_full_season_download is Sonarr's.
func TestDownloadFullSeason(t *testing.T) {
	t.Skip("should_throw_if_full_season_download: releases carry no parsed episode info to tell full seasons by")
}

// should_throw_item_is_removed
func TestRemove(t *testing.T) {
	f := newFixture(t)
	err := f.client(t).Remove(context.Background(), client.Item{ID: "_Droned.S01E01.Pilot.1080p.WEB-DL-DRONE_0"}, true)
	if !errors.Is(err, client.ErrNotSupported) {
		t.Errorf("Remove() error = %v, want = %v", err, client.ErrNotSupported)
	}
}

func TestItems(t *testing.T) {
	f := newFixture(t)
	c := f.client(t)
	id, err := c.Add(context.Background(), &client.Release{Title: title, Data: nzb})
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{"notes.txt", "upper.STRM"} {
		if err := os.WriteFile(filepath.Join(f.strmFolder, other), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, locked := range []bool{false, true} {
		c.locked = func(string) bool { return locked }
		items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("Items() = %+v, want = the .strm file", items)
		}
		item := items[0]
		path := filepath.Join(f.strmFolder, title+".strm")
		fi, _ := os.Stat(path)
		if item.ID != id || item.Title != title+".strm" || item.OutputPath != path || item.TotalSize != fi.Size() {
			t.Errorf("item = %+v, want = the .strm file of ID %s", item, id)
		}
		if want := map[bool]client.Status{false: client.Completed, true: client.Downloading}[locked]; item.Status != want {
			t.Errorf("locked %v: Status = %v, want = %v", locked, item.Status, want)
		}
		if !item.CanMoveFiles || !item.CanBeRemoved {
			t.Errorf("CanMoveFiles, CanBeRemoved = %v, %v, want = true, true", item.CanMoveFiles, item.CanBeRemoved)
		}
	}
}

func TestInfoAndTest(t *testing.T) {
	f := newFixture(t)
	c := f.client(t)
	if info, err := c.Info(context.Background()); err != nil || !info.IsLocalhost || c.Host() != "" {
		t.Errorf("Info(), Host() = %+v, %v, %q, want = localhost and no host", info, err, c.Host())
	}
	if err := c.Test(context.Background()); err != nil {
		t.Errorf("Test() error = %v, want = nil", err)
	}
	if err := os.Remove(f.strmFolder); err != nil {
		t.Fatal(err)
	}
	if err := c.Test(context.Background()); err == nil {
		t.Errorf("Test() without the strm folder error = nil, want an error")
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		settings string
		ok       bool
	}{
		{`{"nzb_folder": "/n", "strm_folder": "/s"}`, true},
		{`{"nzb_folder": "/n"}`, false},
		{`{"nzb_folder": "n", "strm_folder": "/s"}`, false},
	}
	for _, tt := range tests {
		if _, err := Definitions()[0].New([]byte(tt.settings), client.Env{}); (err == nil) != tt.ok {
			t.Errorf("New(%s) error = %v, want ok = %v", tt.settings, err, tt.ok)
		}
	}
}

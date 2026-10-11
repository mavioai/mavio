package blackhole

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const magnetURL = "magnet:?xt=urn:btih:755248817d32b00cc853e633ecdc48e4c21bff15&dn=Series.S05E10.PROPER.HDTV.x264-DEFiNE%5Brartv%5D&tr=http%3A%2F%2Ftracker.trackerfix.com%3A80%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2710&tr=udp%3A%2F%2F9.rarbg.to%3A2710"

// torrentFile is a torrent file of one piece.
var torrentFile = []byte("d4:infod6:lengthi1e4:name4:test12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")

// fixture is the folders of Radarr's blackhole fixtures in a temporary
// folder: the blackhole folder and the watch folder of completed
// downloads.
type fixture struct {
	blackhole, watch string
	clock            *clock
}

func newFixture(t *testing.T) *fixture {
	dir := t.TempDir()
	f := &fixture{blackhole: filepath.Join(dir, "blackhole"), watch: filepath.Join(dir, "completed"), clock: newClock()}
	for _, d := range []string{f.blackhole, f.watch} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *fixture) client(t *testing.T, def string, settings map[string]any) client.Client {
	t.Helper()
	s := map[string]any{"watch_folder": f.watch}
	if def == "torrent-blackhole" {
		s["torrent_folder"] = f.blackhole
	} else {
		s["nzb_folder"] = f.blackhole
	}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	for _, d := range Definitions() {
		if d.ID == def {
			c, err := d.New(raw, env(f.clock))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			return c
		}
	}
	t.Fatalf("no definition %q", def)
	return nil
}

// files lists the names in the blackhole folder.
func (f *fixture) files(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.blackhole)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func (f *fixture) content(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.blackhole, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func torrentRelease() *client.Release {
	return &client.Release{Title: title, Kind: client.Series, Data: torrentFile, FileName: "x.torrent", MagnetURL: magnetURL}
}

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true, RemoveFailedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Items() = %d items, want = 1", len(items))
	}
	return items[0]
}

var defs = []string{"torrent-blackhole", "usenet-blackhole"}

// completed_download_should_have_required_properties
func TestCompletedDownload(t *testing.T) {
	for _, def := range defs {
		t.Run(def, func(t *testing.T) {
			f := newFixture(t)
			c := f.client(t, def, nil)
			c.(interface{ setGrace(time.Duration) }).setGrace(0)
			givenCompletedItem(t, f.watch)
			item := single(t, c)
			if item.ID == "" || item.Title != title || item.RemainingSize != 0 || item.Status != client.Completed {
				t.Errorf("item = %+v, want = an ID, the title, nothing remaining and completed", item)
			}
			if item.RemainingTime == nil || *item.RemainingTime != 0 {
				t.Errorf("RemainingTime = %v, want = 0", item.RemainingTime)
			}
			if item.OutputPath != filepath.Join(f.watch, title) || item.TotalSize != 1000 || item.Kind != 0 {
				t.Errorf("item = %+v, want = the folder of 1000 bytes of either kind", item)
			}
			// The torrent blackhole is read-only by default.
			if want := def == "usenet-blackhole"; !item.CanBeRemoved || item.CanMoveFiles != want {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = true, %v", item.CanBeRemoved, item.CanMoveFiles, want)
			}
		})
	}
}

func (b *base) setGrace(d time.Duration) { b.grace = d }

func TestCanMoveFiles(t *testing.T) {
	tests := []struct {
		readOnly, remove bool
	}{{false, true}, {true, false}, {false, false}}
	for _, tt := range tests {
		f := newFixture(t)
		c := f.client(t, "torrent-blackhole", map[string]any{"read_only": tt.readOnly})
		givenCompletedItem(t, f.watch)
		items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: tt.remove})
		if err != nil || len(items) != 1 {
			t.Fatalf("Items() = %v, %v, want one item", items, err)
		}
		if items[0].CanMoveFiles == tt.readOnly || items[0].CanBeRemoved != tt.remove {
			t.Errorf("read only %v, remove %v: CanMoveFiles, CanBeRemoved = %v, %v, want = %v, %v",
				tt.readOnly, tt.remove, items[0].CanMoveFiles, items[0].CanBeRemoved, !tt.readOnly, tt.remove)
		}
	}
}

// partial_download_should_have_required_properties
func TestPartialDownload(t *testing.T) {
	for _, def := range defs {
		t.Run(def, func(t *testing.T) {
			f := newFixture(t)
			c := f.client(t, def, nil)
			givenCompletedItem(t, f.watch)
			if item := single(t, c); item.ID == "" || item.Title == "" || item.Status != client.Downloading {
				t.Errorf("item = %+v, want = downloading", item)
			}
		})
	}
}

// should_return_category
func TestCategory(t *testing.T) {
	for _, def := range defs {
		t.Run(def, func(t *testing.T) {
			f := newFixture(t)
			givenCompletedItem(t, f.watch)
			if item := single(t, f.client(t, def, nil)); item.Category == "" {
				t.Errorf("Category is empty, want one")
			}
		})
	}
}

func TestTorrentDownload(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		release  func() *client.Release
		// want is the file written and its content; empty for none.
		want, content string
		err           error
	}{
		{
			// Download_should_download_file_if_it_doesnt_exist
			name: "torrent file", release: torrentRelease,
			want: title + ".torrent", content: string(torrentFile),
		},
		{
			name: "save magnet if enabled", settings: map[string]any{"save_magnet_files": true},
			release: func() *client.Release { r := torrentRelease(); r.Data = nil; return r },
			want:    title + ".magnet", content: magnetURL,
		},
		{
			name:     "save magnet using specified extension",
			settings: map[string]any{"save_magnet_files": true, "magnet_file_extension": ".url"},
			release:  func() *client.Release { r := torrentRelease(); r.Data = nil; return r },
			want:     title + ".url", content: magnetURL,
		},
		{
			// Download_should_not_save_magnet_if_disabled and
			// Download_should_throw_if_magnet_and_torrent_url_does_not_exist
			name:    "not save magnet if disabled",
			release: func() *client.Release { r := torrentRelease(); r.Data = nil; return r },
			err:     client.ErrMagnetNotSupported,
		},
		{
			name: "prefer torrent over magnet", settings: map[string]any{"save_magnet_files": true}, release: torrentRelease,
			want: title + ".torrent", content: string(torrentFile),
		},
		{
			name: "replace illegal characters in title",
			release: func() *client.Release {
				r := torrentRelease()
				r.Title = "Saturday Night Live - S38E08 - Jeremy Renner/Maroon 5 [SDTV]"
				return r
			},
			want: "Saturday Night Live - S38E08 - Jeremy Renner+Maroon 5 [SDTV].torrent", content: string(torrentFile),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			// should_return_null_hash
			id, err := f.client(t, "torrent-blackhole", tt.settings).Add(context.Background(), tt.release())
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Errorf("Add() error = %v, want = %v", err, tt.err)
				}
				if got := f.files(t); len(got) != 0 {
					t.Errorf("files = %v, want = none", got)
				}
				return
			}
			if err != nil || id != "" {
				t.Fatalf("Add() = %q, %v, want = \"\", nil", id, err)
			}
			if got := f.files(t); len(got) != 1 || got[0] != tt.want {
				t.Fatalf("files = %v, want = [%s]", got, tt.want)
			}
			if got := f.content(t, tt.want); got != tt.content {
				t.Errorf("content = %q, want = %q", got, tt.content)
			}
		})
	}
}

func TestTorrentDownloadNothing(t *testing.T) {
	f := newFixture(t)
	c := f.client(t, "torrent-blackhole", map[string]any{"save_magnet_files": true})
	if _, err := c.Add(context.Background(), &client.Release{Title: title}); err == nil {
		t.Errorf("Add() error = nil, want an error")
	}
}

func TestUsenetDownload(t *testing.T) {
	tests := []struct{ title, want string }{
		// Download_should_download_file_if_it_doesnt_exist
		{title, title + ".nzb"},
		// Download_should_replace_illegal_characters_in_title
		{"Saturday Night Live - S38E08 - Jeremy Renner/Maroon 5 [SDTV]", "Saturday Night Live - S38E08 - Jeremy Renner+Maroon 5 [SDTV].nzb"},
	}
	for _, tt := range tests {
		f := newFixture(t)
		id, err := f.client(t, "usenet-blackhole", nil).Add(context.Background(), &client.Release{Title: tt.title, Data: []byte("<nzb/>")})
		if err != nil || id != "" {
			t.Fatalf("Add() = %q, %v, want = \"\", nil", id, err)
		}
		if got := f.files(t); len(got) != 1 || got[0] != tt.want {
			t.Fatalf("files = %v, want = [%s]", got, tt.want)
		}
		if got := f.content(t, tt.want); got != "<nzb/>" {
			t.Errorf("content = %q, want = the NZB", got)
		}
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name       string
		file       bool // the download is a file rather than a folder
		missing    bool
		deleteData bool
		err        error
	}{
		{name: "delete file", file: true, deleteData: true},
		{name: "delete directory", deleteData: true},
		{name: "ignore unknown item", missing: true, deleteData: true},
		{name: "throw if deleteData is false", deleteData: false, err: client.ErrNotSupported},
	}
	for _, def := range defs {
		for _, tt := range tests {
			t.Run(def+"/"+tt.name, func(t *testing.T) {
				f := newFixture(t)
				c := f.client(t, def, nil)
				path := filepath.Join(f.watch, title)
				switch {
				case tt.missing:
				case tt.file:
					path += ".mkv"
					write(t, path, "x")
				default:
					givenCompletedItem(t, f.watch)
				}
				err := c.Remove(context.Background(), client.Item{ID: "_" + title + "_0", Title: title, OutputPath: path}, tt.deleteData)
				if !errors.Is(err, tt.err) {
					t.Fatalf("Remove() error = %v, want = %v", err, tt.err)
				}
				_, statErr := os.Stat(path)
				if gone := errors.Is(statErr, os.ErrNotExist); gone != (tt.deleteData || tt.missing) {
					t.Errorf("deleted = %v, want = %v", gone, tt.deleteData)
				}
				if _, err := os.Stat(f.watch); err != nil {
					t.Errorf("watch folder: %v, want it kept", err)
				}
			})
		}
	}
}

func TestRemoveOutsideWatchFolder(t *testing.T) {
	f := newFixture(t)
	outside := filepath.Join(f.blackhole, "keep.mkv")
	write(t, outside, "x")
	c := f.client(t, "usenet-blackhole", nil)
	for _, p := range []string{outside, f.watch, filepath.Join(f.watch, "..", "blackhole", "keep.mkv")} {
		if err := c.Remove(context.Background(), client.Item{OutputPath: p}, true); err == nil {
			t.Errorf("Remove(%s) error = nil, want an error", p)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("file outside the watch folder: %v, want it kept", err)
	}
}

// should_return_status_with_outputdirs
func TestInfo(t *testing.T) {
	for _, def := range defs {
		f := newFixture(t)
		c := f.client(t, def, nil)
		info, err := c.Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		if !info.IsLocalhost || len(info.OutputRootFolders) != 1 || info.OutputRootFolders[0] != f.watch {
			t.Errorf("%s: Info() = %+v, want = localhost and the watch folder", def, info)
		}
		if c.Host() != "" {
			t.Errorf("%s: Host() = %q, want = \"\"", def, c.Host())
		}
	}
}

func TestTest(t *testing.T) {
	for _, def := range defs {
		f := newFixture(t)
		if err := f.client(t, def, nil).Test(context.Background()); err != nil {
			t.Errorf("%s: Test() error = %v, want = nil", def, err)
		}
		if err := os.Remove(f.watch); err != nil {
			t.Fatal(err)
		}
		if err := f.client(t, def, nil).Test(context.Background()); err == nil {
			t.Errorf("%s: Test() without the watch folder error = nil, want an error", def)
		}
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		def      string
		settings string
		ok       bool
	}{
		{"torrent-blackhole", `{"torrent_folder": "/b", "watch_folder": "/w"}`, true},
		{"torrent-blackhole", `{"torrent_folder": "b", "watch_folder": "/w"}`, false},
		{"torrent-blackhole", `{"torrent_folder": "/b"}`, false},
		{"torrent-blackhole", `{"torrent_folder": "/b", "watch_folder": "/w", "magnet_file_extension": ""}`, false},
		{"usenet-blackhole", `{"nzb_folder": "/b", "watch_folder": "/w"}`, true},
		{"usenet-blackhole", `{"nzb_folder": "", "watch_folder": "/w"}`, false},
		{"usenet-blackhole", `{"nzb_folder": "/b", "watch_folder": "/w", "unknown": 1}`, false},
	}
	for _, tt := range tests {
		for _, d := range Definitions() {
			if d.ID != tt.def {
				continue
			}
			if _, err := d.New([]byte(tt.settings), env(newClock())); (err == nil) != tt.ok {
				t.Errorf("%s %s: New() error = %v, want ok = %v", tt.def, tt.settings, err, tt.ok)
			}
		}
	}
}

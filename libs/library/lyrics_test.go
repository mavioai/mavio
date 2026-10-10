package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// lyricsSite has synced lyrics "1" and plain lyrics "2" of every track,
// and garbage "3".
type lyricsSite struct {
	fail bool
	got  LyricsQuery
}

func (*lyricsSite) Name() string { return "site" }

func (s *lyricsSite) SearchLyrics(_ context.Context, q LyricsQuery) ([]RemoteLyrics, error) {
	if s.fail {
		return nil, errors.New("down")
	}
	s.got = q
	return []RemoteLyrics{{ID: "1", Name: q.Name, Synced: true}, {ID: "2", Name: q.Name}}, nil
}

func (s *lyricsSite) DownloadLyrics(_ context.Context, id string) (string, bool, error) {
	switch id {
	case "1":
		return "[00:01.00]One\n", true, nil
	case "2":
		return "One\n", false, nil
	}
	return "[ar:nobody]\n", true, nil
}

func TestLyrics(t *testing.T) {
	ctx := t.Context()
	f := newScan(t, core.LibraryMusic)
	tree(t, f.root, "Band/Album/01 - One.flac", "Band/Album/01 - One.elrc")
	f.scan()
	track := f.item("Band/Album/01 - One.flac")
	site := &lyricsSite{}
	l := &Lyrics{Store: f.store, Source: func() []LyricsProvider { return []LyricsProvider{&lyricsSite{fail: true}, site} }}

	found, err := l.Search(ctx, f.lib, track.ID)
	if err != nil || len(found) != 2 || found[0].Provider != "site" || found[0].ID != "1" || site.got.Name != track.Name {
		t.Fatalf("Search() = %+v, %v; query %+v", found, err, site.got)
	}
	album := f.item("Band/Album")
	if _, err := l.Search(ctx, f.lib, album.ID); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("Search(album) = %v, want ErrInvalid", err)
	}

	// Synced lyrics replace the track's other lyric files.
	if err := l.Download(ctx, f.lib, track.ID, "site", "1"); err != nil {
		t.Fatalf("Download(synced) = %v", err)
	}
	if got := readFile(t, f.root, "Band/Album/01 - One.lrc"); got != "[00:01.00]One\n" || exists(f.root, "Band/Album/01 - One.elrc") {
		t.Errorf("synced lyrics = %q; .elrc kept = %v", got, exists(f.root, "Band/Album/01 - One.elrc"))
	}
	if err := l.Download(ctx, f.lib, track.ID, "site", "2"); err != nil {
		t.Fatalf("Download(plain) = %v", err)
	}
	if got := readFile(t, f.root, "Band/Album/01 - One.txt"); got != "One\n" || exists(f.root, "Band/Album/01 - One.lrc") {
		t.Errorf("plain lyrics = %q; .lrc kept = %v", got, exists(f.root, "Band/Album/01 - One.lrc"))
	}
	// Garbage and unknown providers save nothing.
	if err := l.Download(ctx, f.lib, track.ID, "site", "3"); err == nil || !exists(f.root, "Band/Album/01 - One.txt") {
		t.Errorf("Download(garbage) = %v", err)
	}
	if err := l.Download(ctx, f.lib, track.ID, "nobody", "1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Download(unknown provider) = %v, want ErrNotFound", err)
	}
}

// namedSite is a lyricsSite with another name.
type namedSite struct {
	lyricsSite
	name string
}

func (s *namedSite) Name() string { return s.name }

func TestLyricsOrderAndFetch(t *testing.T) {
	ctx := t.Context()
	f := newScan(t, core.LibraryMusic)
	tree(t, f.root, "Band/Album/01 - One.flac", "Band/Album/02 - Two.flac", "Band/Album/02 - Two.txt")
	f.scan()
	one, two := f.item("Band/Album/01 - One.flac"), f.item("Band/Album/02 - Two.flac")
	l := &Lyrics{Store: f.store, Source: func() []LyricsProvider {
		return []LyricsProvider{&namedSite{name: "a"}, &namedSite{name: "b"}, &namedSite{name: "c"}}
	}}
	lib := f.lib
	lib.Providers.Lyrics = []string{"c", "a", "gone"}
	found, err := l.Search(ctx, lib, one.ID)
	if err != nil || len(found) != 4 || found[0].Provider != "c" || found[2].Provider != "a" {
		t.Errorf("Search() = %+v, %v; want c's then a's", found, err)
	}
	if err := l.Download(ctx, lib, one.ID, "b", "1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Download(unused provider) = %v, want ErrNotFound", err)
	}
	lib.Providers.Lyrics = []string{}
	if found, err := l.Search(ctx, lib, one.ID); err != nil || len(found) != 0 {
		t.Errorf("Search() with no providers = %+v, %v", found, err)
	}

	// Fetch saves the first lyrics found, and leaves tracks with lyrics be.
	lib.Providers.Lyrics = nil
	if err := l.Fetch(ctx, lib, one); err != nil {
		t.Fatalf("Fetch() = %v", err)
	}
	if got := readFile(t, f.root, "Band/Album/01 - One.lrc"); got != "[00:01.00]One\n" {
		t.Errorf("fetched lyrics = %q", got)
	}
	if err := l.Fetch(ctx, lib, two); err != nil || exists(f.root, "Band/Album/02 - Two.lrc") {
		t.Errorf("Fetch() of a track with lyrics = %v; lrc saved = %v", err, exists(f.root, "Band/Album/02 - Two.lrc"))
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

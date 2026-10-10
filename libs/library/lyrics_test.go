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

	found, err := l.Search(ctx, track.ID)
	if err != nil || len(found) != 2 || found[0].Provider != "site" || found[0].ID != "1" || site.got.Name != track.Name {
		t.Fatalf("Search() = %+v, %v; query %+v", found, err, site.got)
	}
	album := f.item("Band/Album")
	if _, err := l.Search(ctx, album.ID); !errors.Is(err, core.ErrInvalid) {
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

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

package mpv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// load loads the libmpv that scripts/resources.sh copied into the app's
// resources, skipping the test without it.
func load(t *testing.T) *Library {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("libmpv ships with the app on macOS only")
	}
	dir := filepath.Join("..", "..", "resources", "darwin", "lib")
	path := filepath.Join(dir, "libmpv.2.dylib")
	if _, err := os.Stat(path); err != nil {
		t.Skip("no libmpv: run pnpm nx run desktop:resources")
	}
	l, err := Load(path, filepath.Join(dir, "libMoltenVK.dylib"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// fixture returns a test media file, skipping the test without it.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", ".fixtures", name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no fixture %s: run pnpm nx run fixtures:media", name)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestPlay(t *testing.T) {
	l := load(t)
	file := fixture(t, "multi_track.mkv")
	states := make(chan State, 256)
	p, err := l.New([][2]string{{"vo", "null"}, {"ao", "null"}, {"config", "no"}, {"idle", "yes"}}, func(s State) {
		select {
		case states <- s:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Load(file, time.Second); err != nil {
		t.Fatal(err)
	}
	wait := func(what string, ok func(State) bool) State {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case s := <-states:
				if ok(s) {
					return s
				}
			case <-timeout:
				t.Fatalf("no state with %s; got = %+v", what, p.State())
			}
		}
	}
	s := wait("a duration", func(s State) bool {
		return s.Loaded && s.Duration > 0 && s.Position >= time.Second && len(s.Tracks) > 0
	})
	if s.Err != nil {
		t.Fatalf("err = %v, want = nil", s.Err)
	}
	var audio, subs int
	for _, tr := range s.Tracks {
		switch tr.Type {
		case "audio":
			audio++
		case "sub":
			subs++
		}
		if tr.Type != "video" && tr.Index < 0 {
			t.Errorf("track %+v has no stream index", tr)
		}
	}
	if audio < 2 {
		t.Errorf("audio tracks = %d, want = 2 or more", audio)
	}
	if err := p.Set("pause", "yes"); err != nil {
		t.Fatal(err)
	}
	wait("pause", func(s State) bool { return s.Paused })
	if err := p.Command("seek", "100", "absolute-percent"); err != nil {
		t.Fatal(err)
	}
	if err := p.Set("pause", "no"); err != nil {
		t.Fatal(err)
	}
	wait("the end", func(s State) bool { return s.Ended })
}

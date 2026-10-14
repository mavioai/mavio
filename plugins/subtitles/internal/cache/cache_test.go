package cache

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func env(dir string, c *clock) provider.Env {
	return provider.Env{Now: c.now, Log: slog.New(slog.DiscardHandler), DataDir: dir}
}

func TestRegion(t *testing.T) {
	dir := t.TempDir()
	c := &clock{time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	r := New(env(dir, c))

	if _, ok := Get[int](t.Context(), r, "a"); ok {
		t.Fatal("Get of an empty region got = true, want = false")
	}
	Set(t.Context(), r, "a", 1, time.Hour)
	Set(t.Context(), r, "b", map[string]int{"x": 2}, 2*time.Hour)
	if v, ok := Get[int](t.Context(), r, "a"); !ok || v != 1 {
		t.Errorf("Get(a) got = %v, %v, want = 1, true", v, ok)
	}

	// Another region of the same folder, as after a restart, reads them.
	c.t = c.t.Add(90 * time.Minute)
	r = New(env(dir, c))
	if _, ok := Get[int](t.Context(), r, "a"); ok {
		t.Error("Get(a) after it expired got = true, want = false")
	}
	if v, ok := Get[map[string]int](t.Context(), r, "b"); !ok || v["x"] != 2 {
		t.Errorf("Get(b) after a restart got = %v, %v, want = map[x:2], true", v, ok)
	}

	// A value of another type is a miss.
	if _, ok := Get[string](t.Context(), r, "b"); ok {
		t.Error("Get(b) as a string got = true, want = false")
	}

	Delete(t.Context(), r, "b")
	if _, ok := Get[map[string]int](t.Context(), New(env(dir, c)), "b"); ok {
		t.Error("Get(b) after Delete got = true, want = false")
	}
}

func TestCached(t *testing.T) {
	c := &clock{time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	r := New(env("", c))
	calls := 0
	f := func() (int, error) {
		calls++
		return calls, nil
	}
	for range 2 {
		if v, err := Cached(t.Context(), r, "k", time.Hour, f); err != nil || v != 1 {
			t.Errorf("Cached got = %v, %v, want = 1, nil", v, err)
		}
	}
	c.t = c.t.Add(2 * time.Hour)
	if v, _ := Cached(t.Context(), r, "k", time.Hour, f); v != 2 {
		t.Errorf("Cached after expiry got = %v, want = 2", v)
	}

	// Failures are not cached.
	failed := errors.New("failed")
	if _, err := Cached(t.Context(), r, "e", time.Hour, func() (int, error) { return 0, failed }); !errors.Is(err, failed) {
		t.Errorf("Cached error got = %v, want = %v", err, failed)
	}
	if v, _ := Cached(t.Context(), r, "e", time.Hour, func() (int, error) { return 5, nil }); v != 5 {
		t.Errorf("Cached after a failure got = %v, want = 5", v)
	}
}

func TestUnreadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName+".json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &clock{time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)}
	r := New(env(dir, c))
	if _, ok := Get[int](t.Context(), r, "a"); ok {
		t.Error("Get of an unreadable cache got = true, want = false")
	}
	Set(t.Context(), r, "a", 1, time.Hour)
	if v, ok := Get[int](t.Context(), New(env(dir, c)), "a"); !ok || v != 1 {
		t.Errorf("Get after rewriting an unreadable cache got = %v, %v, want = 1, true", v, ok)
	}
}

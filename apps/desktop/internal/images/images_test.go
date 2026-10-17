package images

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSized(t *testing.T) {
	for _, tc := range []struct {
		url          string
		width, scale float32
		want         string
	}{
		{"http://s/i", 160, 2, "http://s/i?maxWidth=360"},
		{"http://s/i?tag=a", 100, 1, "http://s/i?tag=a&maxWidth=120"},
		{"http://s/i", 0, 2, "http://s/i"},
	} {
		if got := Sized(tc.url, tc.width, tc.scale); got != tc.want {
			t.Errorf("Sized(%q, %v, %v) = %q, want = %q", tc.url, tc.width, tc.scale, got, tc.want)
		}
	}
}

func TestLoader(t *testing.T) {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.White)
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b.Bytes())
	}))
	defer srv.Close()

	dir := t.TempDir()
	updates := make(chan func(), 4)
	l := New(context.Background(), srv.Client(), dir, 1, func(fn func()) { updates <- fn })
	if got := l.Get(srv.URL+"/a", 0, 1); got != nil {
		t.Fatalf("Get() before loading = %v, want = nil", got)
	}
	(<-updates)()
	got := l.Get(srv.URL+"/a", 0, 1)
	if got == nil {
		t.Fatal("Get() after loading = nil")
	}
	if w, h := got.Size(); w != 2 || h != 3 {
		t.Errorf("Size() = %d, %d, want = 2, 3", w, h)
	}
	l.Get(srv.URL+"/missing", 0, 1)
	(<-updates)()
	if got := l.Get(srv.URL+"/missing", 0, 1); got != nil {
		t.Errorf("Get() of a failed image = %v, want = nil", got)
	}

	// Another loader reads the first image from the disk cache.
	before := requests.Load()
	l = New(context.Background(), srv.Client(), dir, 4, func(fn func()) { updates <- fn })
	l.Get(srv.URL+"/a", 0, 1)
	(<-updates)()
	if l.Get(srv.URL+"/a", 0, 1) == nil || requests.Load() != before {
		t.Errorf("the cached image was downloaded again")
	}
}

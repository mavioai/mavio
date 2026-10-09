package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.Black)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestServer(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	root, outside := t.TempDir(), t.TempDir()
	lib := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{root}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	movie := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "Film"}
	if err := s.Items().Upsert(ctx, movie); err != nil {
		t.Fatal(err)
	}
	poster := jpegBytes(t, 400, 600)
	files := map[string][]byte{
		filepath.Join(root, "poster.jpg"):    poster,
		filepath.Join(outside, "secret.jpg"): poster,
		filepath.Join(root, "logo.svg"):      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`),
		filepath.Join(root, "evil.svg"):      []byte(`<svg xmlns="http://www.w3.org/2000/svg"><image href="http://tracker.example/x.png"/></svg>`),
	}
	for p, data := range files {
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var downloads atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backdrop.jpg" {
			http.NotFound(w, r)
			return
		}
		downloads.Add(1)
		_, _ = w.Write(jpegBytes(t, 1920, 1080))
	}))
	t.Cleanup(provider.Close)
	imgs := []core.Image{
		{Kind: core.ImagePrimary, Path: filepath.Join(root, "poster.jpg")},
		{Kind: core.ImageBackdrop, RemoteURL: provider.URL + "/backdrop.jpg"},
		{Kind: core.ImageLogo, Path: filepath.Join(root, "logo.svg")},
		{Kind: core.ImageArt, Path: filepath.Join(root, "evil.svg")},
		{Kind: core.ImageThumb, Path: filepath.Join(outside, "secret.jpg")},
		{Kind: core.ImageBanner, RemoteURL: provider.URL + "/missing.jpg"},
	}
	if err := s.Images().Replace(ctx, movie.ID, imgs); err != nil {
		t.Fatal(err)
	}
	stored, err := s.Images().ListForOwner(ctx, movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := map[core.ImageKind]string{}
	for _, img := range stored {
		id[img.Kind] = img.ID.String()
	}

	srv := httptest.NewServer(New(Config{Store: s, Dir: t.TempDir()}).Handler())
	t.Cleanup(srv.Close)
	get := func(path string, header ...string) (*http.Response, []byte) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+path, nil)
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp, body
	}
	size := func(data []byte) (int, int) {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return cfg.Width, cfg.Height
	}

	// The original as is, then resized, then from the cache by ETag.
	if resp, body := get("/images/" + id[core.ImagePrimary]); resp.StatusCode != http.StatusOK || !bytes.Equal(body, poster) || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("original poster = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, body := get("/images/" + id[core.ImagePrimary] + "?maxWidth=200")
	if w, h := size(body); resp.StatusCode != http.StatusOK || w != 200 || h != 300 {
		t.Errorf("resized poster = %d, %d×%d", resp.StatusCode, w, h)
	}
	if again, _ := get("/images/"+id[core.ImagePrimary]+"?maxWidth=200", "If-None-Match", resp.Header.Get("ETag")); again.StatusCode != http.StatusNotModified {
		t.Errorf("conditional request = %d, want 304", again.StatusCode)
	}

	// A provider image is downloaded once.
	for range 2 {
		resp, body := get("/images/" + id[core.ImageBackdrop] + "?fillWidth=480&fillHeight=270")
		if w, h := size(body); resp.StatusCode != http.StatusOK || w != 480 || h != 270 {
			t.Errorf("backdrop = %d, %d×%d", resp.StatusCode, w, h)
		}
	}
	if n := downloads.Load(); n != 1 {
		t.Errorf("downloads = %d, want 1", n)
	}

	if resp, _ := get("/images/" + id[core.ImageLogo] + "?maxWidth=10"); resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/svg+xml" {
		t.Errorf("SVG = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	for name, path := range map[string]string{
		"unsafe SVG":          "/images/" + id[core.ImageArt],
		"outside the library": "/images/" + id[core.ImageThumb],
		"provider 404":        "/images/" + id[core.ImageBanner],
		"unknown image":       "/images/" + core.NewID().String(),
		"not an ID":           "/images/poster",
	} {
		if resp, _ := get(path); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", name, resp.StatusCode)
		}
	}
	// Analyzing measures images and computes their placeholders.
	img, err := s.Images().Get(ctx, core.MustParseID(id[core.ImagePrimary]))
	if err != nil {
		t.Fatal(err)
	}
	server := New(Config{Store: s, Dir: t.TempDir()})
	if f, err := server.Analyze(ctx, img); err != nil || f.Width != 400 || f.Height != 600 || f.Blurhash == "" || len(f.Thumbhash) == 0 {
		t.Errorf("Analyze(poster) = %+v, %v", f, err)
	}
	svg, _ := s.Images().Get(ctx, core.MustParseID(id[core.ImageLogo]))
	if f, err := server.Analyze(ctx, svg); err != nil || f.Blurhash != "" {
		t.Errorf("Analyze(SVG) = %+v, %v; want zero facts", f, err)
	}
	for _, q := range []string{"?width=0", "?maxWidth=x", "?quality=101"} {
		if resp, _ := get("/images/" + id[core.ImagePrimary] + q); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", q, resp.StatusCode)
		}
	}
}

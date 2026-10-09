package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

// solid encodes a PNG of one color.
func solid(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCollages(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	root := t.TempDir()
	films := core.Library{Name: "Films", Kind: core.LibraryShows, Paths: []string{root}}
	collections := core.Library{Name: "Collections", Kind: core.LibraryCollections}
	playlists := core.Library{Name: "Playlists", Kind: core.LibraryPlaylists}
	for _, lib := range []*core.Library{&films, &collections, &playlists} {
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	u := core.User{Name: "viewer", PasswordHash: "x"}
	if err := s.Users().Create(ctx, &u); err != nil {
		t.Fatal(err)
	}

	// A red film, a green series with an episode, and an empty collection.
	item := func(lib core.Library, parent core.ID, kind core.ItemKind, name string) core.Item {
		return core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: parent, Kind: kind, Name: name}
	}
	film := item(films, core.NilID, core.KindMovie, "Film")
	series := item(films, core.NilID, core.KindSeries, "Series")
	episode := item(films, series.ID, core.KindEpisode, "Episode")
	saga := item(collections, core.NilID, core.KindCollection, "Saga")
	empty := item(collections, core.NilID, core.KindCollection, "Empty")
	mix := item(playlists, core.NilID, core.KindPlaylist, "Mix")
	mix.UserID = u.ID
	if err := s.Items().Upsert(ctx, film, series, episode, saga, empty, mix); err != nil {
		t.Fatal(err)
	}
	for owner, c := range map[core.ID]color.RGBA{film.ID: {255, 0, 0, 255}, series.ID: {0, 255, 0, 255}} {
		file := filepath.Join(root, owner.String()+".png")
		if err := os.WriteFile(file, solid(t, 200, 300, c), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.Images().Replace(ctx, owner, []core.Image{{Kind: core.ImagePrimary, Path: file}}); err != nil {
			t.Fatal(err)
		}
	}
	for container, items := range map[core.ID][]core.ID{saga.ID: {film.ID, series.ID}, mix.ID: {episode.ID, film.ID}} {
		var links []core.Link
		for _, id := range items {
			links = append(links, core.Link{ItemID: id})
		}
		if err := s.Items().ReplaceLinks(ctx, container, links); err != nil {
			t.Fatal(err)
		}
	}

	srv := httptest.NewServer(New(Config{Store: s, Dir: t.TempDir()}).Handler())
	t.Cleanup(srv.Close)
	get := func(path string) (int, image.Image) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, nil
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return resp.StatusCode, img
	}
	isRed := func(c color.Color) bool { r, g, _, _ := c.RGBA(); return r>>8 > 200 && g>>8 < 60 }
	isGreen := func(c color.Color) bool { r, g, _, _ := c.RGBA(); return g>>8 > 200 && r>>8 < 60 }

	// The library shows its latest items side by side at 16:9.
	code, img := get("/images/collages/" + films.ID.String())
	if code != http.StatusOK || img.Bounds().Dx() != 960 || img.Bounds().Dy() != 540 {
		t.Fatalf("library collage = %d %v", code, img)
	}
	// The collection is a poster grid, here scaled down: red, green, red.
	code, img = get("/images/collages/" + saga.ID.String() + "?maxWidth=300&format=png")
	if code != http.StatusOK || img.Bounds().Dx() != 300 || img.Bounds().Dy() != 450 ||
		!isRed(img.At(75, 100)) || !isGreen(img.At(225, 100)) || !isRed(img.At(75, 350)) {
		t.Errorf("collection collage = %d %v", code, img.Bounds())
	}
	// The playlist shows the episode's series first, in a square.
	code, img = get("/images/collages/" + mix.ID.String())
	if code != http.StatusOK || img.Bounds().Dx() != 600 || img.Bounds().Dy() != 600 || !isGreen(img.At(150, 150)) || !isRed(img.At(450, 150)) {
		t.Errorf("playlist collage = %d %v %v %v", code, img.Bounds(), img.At(150, 150), img.At(450, 150))
	}
	for name, path := range map[string]string{
		"empty collection":    "/images/collages/" + empty.ID.String(),
		"collections library": "/images/collages/" + collections.ID.String(),
		"a film":              "/images/collages/" + film.ID.String(),
		"unknown":             "/images/collages/" + core.NewID().String(),
		"not an ID":           "/images/collages/x",
	} {
		if code, _ := get(path); code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, code)
		}
	}
}

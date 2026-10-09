package library

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Local images an NFO file names are kept when they exist in the library.
func TestNFOLocalImages(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	outside := filepath.Join(t.TempDir(), "fanart.jpg")
	write(t, outside, "jpeg")
	tree(t, f.root, "Up (2009)/Up (2009).mkv")
	poster := filepath.Join(f.root, "Up (2009)", "poster.jpg")
	write(t, poster, "jpeg")
	write(t, filepath.Join(f.root, "Up (2009)", "movie.nfo"), `<movie><title>Up</title>`+
		`<thumb aspect="poster">`+poster+`</thumb>`+
		`<thumb aspect="banner">`+filepath.Join(f.root, "Up (2009)", "missing.jpg")+`</thumb>`+
		`<fanart><thumb>`+outside+`</thumb></fanart></movie>`)
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return f.clock }
	jobs := &Jobs{Store: f.store, Scanner: f.sc, Prober: &fakeProber{}, Now: now, Refresher: &Refresher{Store: f.store, Now: now}}
	if err := jobs.Schedule(t.Context()); err != nil {
		t.Fatal(err)
	}
	drain(t, &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()})
	images := f.store.images[f.item("Up (2009)/Up (2009).mkv").ID]
	if len(images) != 1 || images[0].Kind != core.ImagePrimary || filepath.FromSlash(images[0].Path) != poster {
		t.Errorf("images = %+v, want the poster only", images)
	}
}

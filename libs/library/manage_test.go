package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

var (
	jpegData = []byte("\xff\xd8\xff\xe0 a JPEG")
	pngData  = []byte("\x89PNG\r\n\x1a\n a PNG")
)

// catalog is a provider knowing films by TMDB ID; items without an ID are
// taken for the first film of their name, as a careless search would.
type catalog struct{ films []catalogFilm }

type catalogFilm struct {
	id, name, overview, collection, collectionID string
	year                                         int
}

func (c *catalog) Name() string { return "catalog" }

func (c *catalog) find(l Lookup) (catalogFilm, bool) {
	for _, f := range c.films {
		if id := l.ExternalIDs[core.ProviderTMDB]; id != "" && f.id == id ||
			l.ExternalIDs[core.ProviderTMDB] == "" && strings.EqualFold(f.name, l.Name) {
			return f, true
		}
	}
	return catalogFilm{}, false
}

func (c *catalog) Metadata(_ context.Context, l Lookup) (*metadata.Result, error) {
	f, ok := c.find(l)
	if !ok || l.Kind != core.KindMovie {
		return nil, nil
	}
	ids := map[core.Provider]string{core.ProviderTMDB: f.id}
	if f.collectionID != "" {
		ids[core.ProviderTMDBCollection] = f.collectionID
	}
	return &metadata.Result{
		Item: core.Item{
			Name: f.name, Overview: f.overview, ProductionYear: f.year, Genres: []string{"Film " + f.id},
			CollectionName: f.collection, ExternalIDs: ids,
		},
		People: []metadata.Person{{Name: "Director " + f.id, Kind: core.CreditDirector}},
		RemoteImages: []metadata.RemoteImage{
			{Kind: core.ImagePrimary, URL: "https://image.example/" + f.id + "/poster.jpg"},
			{Kind: core.ImageBackdrop, URL: "https://image.example/" + f.id + "/fanart.jpg"},
		},
	}, nil
}

func (c *catalog) Search(_ context.Context, l Lookup, limit int) ([]SearchResult, error) {
	var out []SearchResult
	for _, f := range c.films {
		if strings.EqualFold(f.name, l.Name) && (l.Year == 0 || l.Year == f.year) && len(out) < limit {
			out = append(out, SearchResult{Name: f.name, Year: f.year, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: f.id}})
		}
	}
	return out, nil
}

var heatCatalog = &catalog{films: []catalogFilm{
	{id: "2", name: "Heat", year: 1986, overview: "A Las Vegas bodyguard."},
	{id: "949", name: "Heat", year: 1995, overview: "A group of bank robbers.", collection: "Crime Saga", collectionID: "77"},
}}

type manageFixture struct {
	*scanFixture
	r       *Refresher
	fetched []string
}

func newManage(t *testing.T, saveLocal bool) *manageFixture {
	f := &manageFixture{scanFixture: newScan(t, core.LibraryMovies)}
	f.lib.SaveLocalMetadata, f.lib.AutoCollections = saveLocal, true
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	f.r = &Refresher{
		Store: f.store, Providers: []Provider{heatCatalog}, MetadataDir: filepath.ToSlash(t.TempDir()),
		Fetch: func(_ context.Context, url string) ([]byte, error) {
			f.fetched = append(f.fetched, url)
			return jpegData, nil
		},
	}
	tree(t, f.root, "Heat (1995)/Heat (1995).mkv")
	f.scan()
	return f
}

func (f *manageFixture) refresh(opts RefreshOptions) core.Item {
	f.t.Helper()
	it, err := f.r.RefreshWith(f.t.Context(), f.lib, f.item("Heat (1995)/Heat (1995).mkv").ID, opts)
	if err != nil {
		f.t.Fatal(err)
	}
	return it
}

func (f *manageFixture) images(id core.ID) map[core.ImageKind]string {
	out := map[core.ImageKind]string{}
	for _, img := range f.store.images[id] {
		out[img.Kind] = img.Path + img.RemoteURL
	}
	return out
}

func (f *manageFixture) file(rel string) string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(rel)))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func TestIdentifyEditAndSaveLocal(t *testing.T) {
	f := newManage(t, true)
	ctx := t.Context()

	// The scan's refresh takes the film for the wrong Heat.
	it := f.refresh(RefreshOptions{})
	if it.ExternalIDs[core.ProviderTMDB] != "2" || it.Overview != "A Las Vegas bodyguard." {
		t.Fatalf("first refresh: %+v", it)
	}
	if !strings.Contains(f.file("Heat (1995)/Heat (1995).nfo"), `<uniqueid type="tmdb">2</uniqueid>`) {
		t.Errorf("NFO after the first refresh:\n%s", f.file("Heat (1995)/Heat (1995).nfo"))
	}
	if f.file("Heat (1995)/poster.jpg") != string(jpegData) || !exists(f.root, "Heat (1995)/fanart.jpg") {
		t.Error("provider images not saved beside the film")
	}

	// Searching by year finds the right one.
	found, err := f.r.Search(ctx, f.lib, it.ID, SearchQuery{Year: 1995})
	if err != nil || len(found) != 1 || found[0].ExternalIDs[core.ProviderTMDB] != "949" || found[0].Provider != "catalog" {
		t.Fatalf("Search = %+v, %v", found, err)
	}
	f.fetched = nil
	it, err = f.r.Identify(ctx, f.lib, it.ID, found[0].ExternalIDs)
	if err != nil {
		t.Fatal(err)
	}
	if it.Overview != "A group of bank robbers." || it.ExternalIDs[core.ProviderTMDBCollection] != "77" ||
		!slices.Equal(it.Genres, []string{"Film 949"}) {
		t.Errorf("identified: %+v", it)
	}
	if credits := f.store.credits[it.ID]; len(credits) != 1 || f.store.people[credits[0].PersonID].Name != "Director 949" {
		t.Errorf("credits = %+v", credits)
	}
	// The NFO and artwork beside the film are replaced, not read back.
	nfo := f.file("Heat (1995)/Heat (1995).nfo")
	if !strings.Contains(nfo, `<uniqueid type="tmdb">949</uniqueid>`) || strings.Contains(nfo, "Las Vegas") {
		t.Errorf("NFO after identifying:\n%s", nfo)
	}
	if !slices.Contains(f.fetched, "https://image.example/949/poster.jpg") {
		t.Errorf("fetched = %v, want the new poster", f.fetched)
	}

	// The film went into its collection.
	var coll core.Item
	for _, c := range f.store.items {
		if c.Kind == core.KindCollection {
			coll = c
		}
	}
	if coll.Name != "Crime Saga" || coll.ExternalIDs[core.ProviderTMDBCollection] != "77" ||
		len(f.store.links[coll.ID]) != 1 || f.store.links[coll.ID][0].ItemID != it.ID {
		t.Errorf("collection = %+v, links %+v", coll, f.store.links[coll.ID])
	}

	// An edit with a lock survives refreshes, and the NFO keeps the lock.
	it, err = f.r.Update(ctx, f.lib, it.ID, func(it *core.Item) error {
		it.Overview = "Edited."
		it.LockedFields = append(it.LockedFields, core.FieldOverview)
		return nil
	})
	if err != nil || it.Overview != "Edited." {
		t.Fatalf("Update = %+v, %v", it, err)
	}
	if it = f.refresh(RefreshOptions{ReplaceMetadata: true}); it.Overview != "Edited." {
		t.Errorf("overview after a replacing refresh = %q", it.Overview)
	}
	if nfo := f.file("Heat (1995)/Heat (1995).nfo"); !strings.Contains(nfo, "<lockedfields>Overview</lockedfields>") ||
		!strings.Contains(nfo, "<plot>Edited.</plot>") {
		t.Errorf("NFO after editing:\n%s", nfo)
	}

	// A chosen poster replaces the saved one, in its format.
	img, err := f.r.SetImage(ctx, f.lib, it.ID, core.ImagePrimary, pngData)
	if err != nil {
		t.Fatal(err)
	}
	if img.Path != f.root+"/Heat (1995)/poster.png" || exists(f.root, "Heat (1995)/poster.jpg") {
		t.Errorf("chosen poster at %s; poster.jpg left: %v", img.Path, exists(f.root, "Heat (1995)/poster.jpg"))
	}
	remote, err := f.r.RemoteImages(ctx, f.lib, it.ID, core.ImagePrimary)
	if err != nil || len(remote) != 1 || remote[0].URL != "https://image.example/949/poster.jpg" || remote[0].Provider != "catalog" {
		t.Errorf("RemoteImages = %+v, %v", remote, err)
	}

	// A new library reads it all back from beside the film.
	g := &manageFixture{scanFixture: newScan(t, core.LibraryMovies)}
	g.root, g.lib.Paths = f.root, f.lib.Paths
	g.r = &Refresher{Store: g.store}
	g.scan()
	it = g.refresh(RefreshOptions{})
	if it.Overview != "Edited." || it.ExternalIDs[core.ProviderTMDB] != "949" || !slices.Contains(it.LockedFields, core.FieldOverview) ||
		it.CollectionName != "Crime Saga" {
		t.Errorf("read back: %+v", it)
	}
	if got := g.images(it.ID)[core.ImagePrimary]; got != f.root+"/Heat (1995)/poster.png" {
		t.Errorf("poster read back = %q", got)
	}

	// Deleting the poster deletes its file.
	heat := f.item("Heat (1995)/Heat (1995).mkv").ID
	if err := f.r.DeleteImage(ctx, f.lib, heat, img.ID); err != nil {
		t.Fatal(err)
	}
	if exists(f.root, "Heat (1995)/poster.png") || f.images(heat)[core.ImagePrimary] != "" {
		t.Error("the deleted poster remains")
	}
}

func TestChosenImageWithoutSaveLocal(t *testing.T) {
	f := newManage(t, false)
	it := f.refresh(RefreshOptions{})
	if exists(f.root, "Heat (1995)/Heat (1995).nfo") || exists(f.root, "Heat (1995)/poster.jpg") || len(f.fetched) > 0 {
		t.Fatal("metadata written beside the film of a library not saving local metadata")
	}
	img, err := f.r.SetImage(t.Context(), f.lib, it.ID, core.ImageLogo, pngData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(img.Path, f.r.MetadataDir+"/") || !strings.HasSuffix(img.Path, "/logo.png") {
		t.Errorf("chosen logo at %s", img.Path)
	}
	// Refreshes keep the chosen image over the providers'.
	tree(t, f.root, "Heat (1995)/logo.jpg")
	f.refresh(RefreshOptions{ReplaceMetadata: true})
	if got := f.images(it.ID)[core.ImageLogo]; got != img.Path {
		t.Errorf("logo after a refresh = %q, want %q", got, img.Path)
	}
	if _, err := f.r.SetImage(t.Context(), f.lib, it.ID, core.ImagePrimary, []byte("<html>")); err == nil {
		t.Error("SetImage took a page for an image")
	}
}

func TestArtBase(t *testing.T) {
	root := t.TempDir()
	tree(t, root, "Movie/Movie.mkv", "Mixed/A.mkv", "Mixed/B.mkv", "Show/Season 1/S01E01.mkv", "Album/01.flac")
	fsys := os.DirFS(root)
	tests := []struct {
		kind  core.ItemKind
		rel   string
		image core.ImageKind
		index int
		want  string
	}{
		{core.KindMovie, "Movie/Movie.mkv", core.ImagePrimary, 0, "Movie/poster"},
		{core.KindMovie, "Movie/Movie.mkv", core.ImageBackdrop, 2, "Movie/fanart-2"},
		{core.KindMovie, "Movie/Movie.mkv", core.ImageLogo, 1, ""},
		{core.KindMovie, "Mixed/A.mkv", core.ImageThumb, 0, "Mixed/A-landscape"},
		{core.KindSeries, "Show", core.ImageBanner, 0, "Show/banner"},
		{core.KindSeason, "Show/Season 1", core.ImagePrimary, 0, ""},
		{core.KindEpisode, "Show/Season 1/S01E01.mkv", core.ImagePrimary, 0, "Show/Season 1/S01E01-thumb"},
		{core.KindEpisode, "Show/Season 1/S01E01.mkv", core.ImageBackdrop, 0, ""},
		{core.KindMusicAlbum, "Album", core.ImagePrimary, 0, "Album/folder"},
		{core.KindMovie, "Movie/Movie.mkv", core.ImageScreenshot, 0, ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s %s %d", tt.kind, tt.rel, tt.image, tt.index), func(t *testing.T) {
			got, ok := artBase(fsys, &core.Item{Kind: tt.kind}, tt.rel, tt.image, tt.index)
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("artBase = %q, %v, want = %q", got, ok, tt.want)
			}
			// What is saved is found again.
			if !ok {
				return
			}
			tree(t, root, got+".jpg")
			found := false
			isFolder := !strings.Contains(filepath.Base(tt.rel), ".")
			for _, img := range localImages(fsys, &core.Item{Kind: tt.kind}, tt.rel, isFolder, strings.HasPrefix(tt.rel, "Mixed")) {
				found = found || img.Kind == tt.image && img.Path == got+".jpg"
			}
			if !found {
				t.Errorf("%s.jpg is not found as the %s image", got, tt.image)
			}
		})
	}
}

// subtitleSource offers one subtitle per language.
type subtitleSource struct{ queries []SubtitleQuery }

func (s *subtitleSource) Name() string { return "subs" }

func (s *subtitleSource) SearchSubtitles(_ context.Context, q SubtitleQuery) ([]RemoteSubtitle, error) {
	s.queries = append(s.queries, q)
	return []RemoteSubtitle{{ID: "1-" + q.Language, Name: q.FileName, Language: q.Language, Format: "srt", HashMatch: q.FileHash != ""}}, nil
}

func (s *subtitleSource) DownloadSubtitle(_ context.Context, id string) (DownloadedSubtitle, error) {
	_, lang, _ := strings.Cut(id, "-")
	return DownloadedSubtitle{Data: []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), Format: "srt", Language: lang, HearingImpaired: true}, nil
}

func TestDownloadSubtitle(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	tree(t, f.root, "Up (2009)/Up (2009).mkv", "Up (2009)/Up (2009).en.srt")
	// Big enough to hash.
	if err := os.WriteFile(filepath.Join(f.root, "Up (2009)", "Up (2009).mkv"), make([]byte, 200<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	f.scan()
	up := f.item("Up (2009)/Up (2009).mkv")
	src := &subtitleSource{}
	s := &Subtitles{Store: f.store, Providers: []SubtitleProvider{src}}
	found, err := s.Search(t.Context(), f.lib, up.ID, core.ID{}, "fr", true, false)
	if err != nil || len(found) != 1 || found[0].Provider != "subs" || !found[0].HashMatch {
		t.Fatalf("Search = %+v, %v", found, err)
	}
	if q := src.queries[0]; q.FileName != "Up (2009).mkv" || q.FileSize != 200<<10 || len(q.FileHash) != 16 || !q.HearingImpaired {
		t.Errorf("query = %+v", q)
	}
	for range 2 {
		st, err := s.Download(t.Context(), f.lib, up.ID, core.ID{}, "subs", found[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if st.Language != "fre" || !st.HearingImpaired || st.Codec != "subrip" {
			t.Errorf("stream = %+v", st)
		}
	}
	if !exists(f.root, "Up (2009)/Up (2009).fr.sdh.srt") || !exists(f.root, "Up (2009)/Up (2009).fr.sdh.2.srt") {
		t.Error("downloaded subtitles not saved beside the film")
	}
	if n := len(sidecarsOf(f.store.sources[up.ID][0].Streams)); n != 3 {
		t.Errorf("subtitle streams = %d, want 3", n)
	}
	if _, err := s.Download(t.Context(), f.lib, up.ID, core.ID{}, "nobody", "1"); err == nil {
		t.Error("Download from an unknown provider: no error")
	}
}

package library

import (
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// fakeFS is a Lister over a set of paths; folders end with "/".
type fakeFS map[string]bool

func newFS(paths ...string) fakeFS {
	fs := fakeFS{}
	for _, p := range paths {
		for p != "/" && p != "." && p != "" {
			isDir := strings.HasSuffix(p, "/")
			p = strings.TrimSuffix(p, "/")
			if !fs[p] {
				fs[p] = isDir
			}
			p = path.Dir(p) + "/"
			if p == "//" {
				break
			}
		}
	}
	return fs
}

func (f fakeFS) List(dir string) ([]Entry, error) {
	var out []Entry
	for p, isDir := range f {
		if path.Dir(p) == dir {
			out = append(out, Entry{Path: p, IsDir: isDir})
		}
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

func resolve(t *testing.T, kind core.LibraryKind, root, dir string, parent Container, parentPath string, fs fakeFS) Result {
	t.Helper()
	entries, _ := fs.List(dir)
	res, err := NewResolver().Resolve(Scope{Kind: kind, Root: root, Parent: parent, ParentPath: parentPath}, dir, entries, fs)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

var providers = map[string]core.Provider{
	"Tvdb": core.ProviderTVDB, "TvMaze": core.ProviderTVMaze, "Tmdb": core.ProviderTMDB, "Imdb": core.ProviderIMDb,
	"AniDB": core.ProviderAniDB, "AniList": core.ProviderAniList, "AniSearch": core.ProviderAniSearch,
}

func TestMovieResolverCases(t *testing.T) {
	portedCases(t, "movie_resolver.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ResolvePath_MovieFolderWithSampleSubfolder_ResolvesToMovie": func(t *testing.T, a args) {
				fs := newFS("/media/Outer Colony (2026)/Outer Colony (2026).mkv", "/media/Outer Colony (2026)/"+a.str(t, "sampleDirName")+"/")
				res := resolve(t, core.LibraryMovies, "/media", "/media/Outer Colony (2026)", InFolder, "", fs)
				if res.Item == nil || res.Item.Kind != core.KindMovie {
					t.Errorf("got = %+v, want a movie", res)
				}
			},
		},
		facts: map[string]string{
			"Resolve_GivenLocalAlternateVersion_ResolvesToVideo":                   "TestMovieFolders",
			"ResolveMultiple_GivenTvShowsCollection_CreatesEpisodeItems":           "TestMovieFolders",
			"ResolveMultiple_GivenMoviesCollection_CreatesMovieItems":              "TestMovieFolders",
			"ResolveMultiple_GivenNumberedSampleFiles_IgnoresSamples":              "TestMovieFolders",
			"AllExtrasTypesFolderNames_ContainsSampleSingularAndPlural":            "TestMovieFolders",
			"ResolvePath_MovieFolderWithRealSubfolder_DoesNotResolveToSingleMovie": "TestMovieFolders",
		},
	})
}

func TestMovieFolders(t *testing.T) {
	t.Run("local alternate version", func(t *testing.T) {
		fs := newFS("/movies/Black Panther (2018)/Black Panther (2018) - 1080p 3D.mk3d")
		if res := resolve(t, core.LibraryMovies, "/movies", "/movies/Black Panther (2018)", InFolder, "", fs); res.Item == nil {
			t.Error("got = no item")
		}
	})
	t.Run("versions of episodes", func(t *testing.T) {
		fs := newFS("/TV/Show/Season 1/Show - S01E01 - 1080p.mkv", "/TV/Show/Season 1/Show - S01E01 - 720p.mkv", "/TV/Show/Season 1/Show - S01E02.mkv")
		res := resolve(t, core.LibraryShows, "/TV", "/TV/Show/Season 1", InSeries, "/TV/Show", fs)
		if len(res.Items) != 2 {
			t.Fatalf("got = %d items, want 2", len(res.Items))
		}
		for _, n := range res.Items {
			if n.Kind != core.KindEpisode {
				t.Errorf("%s: got = %s, want episode", n.Path, n.Kind)
			}
			if strings.Contains(n.Path, "S01E01") && len(n.Versions) != 1 {
				t.Errorf("S01E01 versions: got = %v", n.Versions)
			}
		}
	})
	t.Run("versions of a movie", func(t *testing.T) {
		fs := newFS("/movies/Inception (2010)/Inception (2010) - 1080p.mkv", "/movies/Inception (2010)/Inception (2010) - 720p.mkv")
		res := resolve(t, core.LibraryMovies, "/movies", "/movies/Inception (2010)", InFolder, "", fs)
		if res.Item == nil || res.Item.Kind != core.KindMovie || len(res.Item.Versions) != 1 {
			t.Errorf("got = %+v", res.Item)
		}
	})
	t.Run("numbered samples", func(t *testing.T) {
		fs := newFS("/movies/La Chimera (2023)/La Chimera (2023).mkv", "/movies/La Chimera (2023)/Sample1.mkv", "/movies/La Chimera (2023)/Sample2.mkv")
		res := resolve(t, core.LibraryMovies, "/movies", "/movies/La Chimera (2023)", InFolder, "", fs)
		if res.Item == nil || res.Item.Path != "/movies/La Chimera (2023)/La Chimera (2023).mkv" || len(res.Items) != 0 {
			t.Errorf("got = %+v", res)
		}
	})
	t.Run("sample folder names", func(t *testing.T) {
		r := NewResolver()
		for _, n := range []string{"sample", "Sample", "samples"} {
			if _, ok := r.extrasFolderKind(n); !ok {
				t.Errorf("%s: got = not an extras folder", n)
			}
		}
	})
	t.Run("real subfolder", func(t *testing.T) {
		fs := newFS("/media/Outer Colony (2026)/Outer Colony (2026).mkv", "/media/Outer Colony (2026)/Feature/")
		if res := resolve(t, core.LibraryMovies, "/media", "/media/Outer Colony (2026)", InFolder, "", fs); res.Item != nil {
			t.Errorf("got = %+v, want no single movie", res.Item)
		}
	})
}

func TestEpisodeResolverCases(t *testing.T) {
	portedCases(t, "episode_resolver.json", ported{
		run: map[string]func(t *testing.T, a args){
			"Resolve_EpisodeFileWithProviderId_SetsProviderId": func(t *testing.T, a args) {
				p := a.str(t, "path")
				fs := newFS(p)
				res := resolve(t, core.LibraryShows, "/media", path.Dir(p), InSeries, "/media/Show", fs)
				if len(res.Items) != 1 {
					t.Fatalf("got = %d episodes", len(res.Items))
				}
				if got, want := res.Items[0].ExternalIDs[providers[a.symbol(t, "provider")]], a.str(t, "expectedId"); got != want {
					t.Errorf("got = %q, want = %q", got, want)
				}
			},
		},
		facts: map[string]string{
			"Resolve_GivenVideoInExtrasFolder_DoesNotResolveToEpisode":             "TestEpisodes",
			"Resolve_GivenVideoInExtrasSeriesFolder_ResolvesToEpisode":             "TestEpisodes",
			"Resolve_EpisodeFileWithProviderIdsOnAllLevels_OnlyUsesEpisodeLevelId": "TestEpisodes",
			"Resolve_EpisodeFileWithMultipleProviderIds_SetsAll":                   "TestEpisodes",
		},
	})
}

func TestEpisodes(t *testing.T) {
	t.Run("extras folder is not descended", func(t *testing.T) {
		fs := newFS("/tv/All My Children/Season 01/Extras/All My Children S01E01 - Behind The Scenes.mkv")
		res := resolve(t, core.LibraryShows, "/tv", "/tv/All My Children/Season 01", InSeries, "/tv/All My Children", fs)
		if len(res.Items) != 0 || len(res.Subfolders) != 0 {
			t.Errorf("got = %+v, want nothing", res)
		}
	})
	t.Run("series named Extras", func(t *testing.T) {
		fs := newFS("/tv/Extras/Extras S01E01.mkv")
		res := resolve(t, core.LibraryShows, "/tv", "/tv/Extras", InFolder, "/tv", fs)
		if len(res.Items) != 1 || res.Items[0].Kind != core.KindEpisode {
			t.Errorf("got = %+v, want one episode", res.Items)
		}
	})
	t.Run("only the episode's own IDs", func(t *testing.T) {
		p := "/media/Show [tvdbid=11111]/Season 01 [tvdbid=22222]/Show S01E01 [tvdbid=33333].mkv"
		res := resolve(t, core.LibraryShows, "/media", path.Dir(p), InSeries, "/media/Show [tvdbid=11111]", newFS(p))
		if len(res.Items) != 1 || res.Items[0].ExternalIDs[core.ProviderTVDB] != "33333" {
			t.Errorf("got = %+v", res.Items)
		}
	})
	t.Run("several IDs", func(t *testing.T) {
		p := "/media/Show/Season 01/Show S01E01 [tvdbid=12345][tmdbid=99999].mkv"
		res := resolve(t, core.LibraryShows, "/media", path.Dir(p), InSeries, "/media/Show", newFS(p))
		if len(res.Items) != 1 || res.Items[0].ExternalIDs[core.ProviderTVDB] != "12345" || res.Items[0].ExternalIDs[core.ProviderTMDB] != "99999" {
			t.Errorf("got = %+v", res.Items)
		}
	})
}

func resolveSeries(t *testing.T, dir string, kind core.LibraryKind) *Node {
	t.Helper()
	return resolve(t, kind, "/media", dir, InFolder, "/media", newFS(dir+"/")).Item
}

func TestSeriesResolverCases(t *testing.T) {
	check := func(t *testing.T, n *Node, provider core.Provider, want string) {
		t.Helper()
		if n == nil || n.Kind != core.KindSeries {
			t.Fatalf("got = %+v, want a series", n)
		}
		if got := n.ExternalIDs[provider]; got != want {
			t.Errorf("got = %q, want = %q", got, want)
		}
	}
	portedCases(t, "series_resolver.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ResolvePath_SeriesFolderWithProviderId_SetsProviderId": func(t *testing.T, a args) {
				check(t, resolveSeries(t, a.str(t, "path"), core.LibraryShows), providers[a.symbol(t, "provider")], a.str(t, "expectedId"))
			},
			"ResolvePath_SeriesFolderWithAniProviderId_SetsProviderId": func(t *testing.T, a args) {
				check(t, resolveSeries(t, a.str(t, "path"), core.LibraryShows), providers[a.str(t, "providerKey")], a.str(t, "expectedId"))
			},
		},
		facts: map[string]string{
			"ResolvePath_SeriesFolderWithMultipleProviderIds_SetsAll":       "TestSeriesFolders",
			"ResolvePath_SeriesFolderWithNoProviderId_HasNoProviderIds":     "TestSeriesFolders",
			"ResolvePath_SeriesFolderNotInTvShowsCollection_DoesNotResolve": "TestSeriesFolders",
		},
	})
}

func TestSeriesFolders(t *testing.T) {
	n := resolveSeries(t, "/media/Show [tvdbid=12345][tmdbid=99999]", core.LibraryShows)
	if n == nil || n.ExternalIDs[core.ProviderTVDB] != "12345" || n.ExternalIDs[core.ProviderTMDB] != "99999" {
		t.Errorf("several IDs: got = %+v", n)
	}
	if n := resolveSeries(t, "/media/Show", core.LibraryShows); n == nil || len(n.ExternalIDs) != 0 {
		t.Errorf("no IDs: got = %+v", n)
	}
	if n := resolveSeries(t, "/media/Show [tvdbid=12345]", core.LibraryMixed); n != nil && n.Kind == core.KindSeries {
		t.Errorf("mixed library: got = %+v, want no series", n)
	}
}

func resolveSeason(t *testing.T, dir, series string) *Node {
	t.Helper()
	return resolve(t, core.LibraryShows, "/media", dir, InSeries, series, newFS(dir+"/x.mkv")).Item
}

func TestSeasonResolverCases(t *testing.T) {
	check := func(t *testing.T, n *Node, provider core.Provider, want string) {
		t.Helper()
		if n == nil || n.Kind != core.KindSeason {
			t.Fatalf("got = %+v, want a season", n)
		}
		if got := n.ExternalIDs[provider]; got != want {
			t.Errorf("got = %q, want = %q", got, want)
		}
	}
	portedCases(t, "season_resolver.json", ported{
		run: map[string]func(t *testing.T, a args){
			"Resolve_SeasonFolderWithProviderId_SetsProviderId": func(t *testing.T, a args) {
				check(t, resolveSeason(t, a.str(t, "path"), "/media/Show"), providers[a.symbol(t, "provider")], a.str(t, "expectedId"))
			},
			"Resolve_SeasonFolderWithAniProviderId_SetsProviderId": func(t *testing.T, a args) {
				check(t, resolveSeason(t, a.str(t, "path"), "/media/Show"), providers[a.str(t, "providerKey")], a.str(t, "expectedId"))
			},
		},
		facts: map[string]string{
			"Resolve_SeasonFolderWithMultipleProviderIds_SetsAll":                         "TestSeasonFolders",
			"Resolve_SeasonFolderWithSeriesProviderIdInParentPath_DoesNotInheritSeriesId": "TestSeasonFolders",
			"Resolve_SeasonFolderWithNoProviderId_HasNoProviderIds":                       "TestSeasonFolders",
		},
	})
}

func TestSeasonFolders(t *testing.T) {
	n := resolveSeason(t, "/media/Show/Season 01 [tvdbid=12345][tmdbid=99999]", "/media/Show")
	if n == nil || n.ExternalIDs[core.ProviderTVDB] != "12345" || n.ExternalIDs[core.ProviderTMDB] != "99999" {
		t.Errorf("several IDs: got = %+v", n)
	}
	n = resolveSeason(t, "/media/Show [tvdbid=11111]/Season 01 [tvdbid=22222]", "/media/Show [tvdbid=11111]")
	if n == nil || n.ExternalIDs[core.ProviderTVDB] != "22222" {
		t.Errorf("series ID in path: got = %+v", n)
	}
	n = resolveSeason(t, "/media/Show/Season 01", "/media/Show")
	if n == nil || len(n.ExternalIDs) != 0 || n.Name != "Season 1" || n.Index == nil || *n.Index != 1 {
		t.Errorf("no IDs: got = %+v", n)
	}
}

func TestAudioResolverCases(t *testing.T) {
	book := func(t *testing.T, dir string, a args) *Node {
		var paths []string
		for _, c := range a.strs(t, "children") {
			paths = append(paths, dir+"/"+c)
		}
		fs := newFS(append(paths, dir+"/")...)
		entries, _ := fs.List(dir)
		n, ok := NewResolver().AudioBookFolder(dir, entries)
		if !ok {
			return nil
		}
		return &n
	}
	portedCases(t, "audio_resolver.json", ported{
		run: map[string]func(t *testing.T, a args){
			"Resolve_AudiobookDirectory_SingleResult": func(t *testing.T, a args) {
				if book(t, "/parent/title", a) == nil {
					t.Error("got = no audiobook")
				}
			},
			"Resolve_AudiobookDirectory_NoResult": func(t *testing.T, a args) {
				if n := book(t, "/parent/book title", a); n != nil {
					t.Errorf("got = %+v, want none", n)
				}
			},
		},
	})
}

func TestCoreResolutionIgnoreRuleCases(t *testing.T) {
	portedCases(t, "core_resolution_ignore_rule.json", ported{
		skip: map[string]string{
			"TestApplicationFolder": "Mavio keeps no application data inside libraries",
		},
		facts: map[string]string{
			"TestTopLevelDirectory":      "TestIgnoreRule",
			"TestIgnorePatterns":         "TestIgnoreRule",
			"TestExtrasTypesFolderNames": "TestIgnoreRule",
			"TestThemeSong":              "TestIgnoreRule",
		},
	})
}

func TestIgnoreRule(t *testing.T) {
	r := NewResolver()
	tests := []struct {
		e    Entry
		top  bool
		want bool
	}{
		// Extras folders at the top of a library are not extras.
		{Entry{"/tv/Extras", true}, true, false},
		{Entry{"/Movies/Up/extras", true}, false, true},
		{Entry{"/Media/big.jpg", false}, false, false},
		{Entry{"/Media/small.jpg", false}, false, true},
		{Entry{"/Movies/Up/intro.mp3", false}, false, false},
		{Entry{"/Movies/Up/theme.mp3", false}, false, true},
	}
	for _, tt := range tests {
		if got := r.ignored(tt.e, tt.top); got != tt.want {
			t.Errorf("%s: got = %v, want = %v", tt.e.Path, got, tt.want)
		}
	}
}

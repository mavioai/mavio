package library

import (
	"maps"
	"path"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestFindExtrasCases(t *testing.T) {
	facts := map[string]string{}
	for _, f := range []string{
		"FindExtras_SeparateMovieFolder_FindsCorrectExtras", "FindExtras_SeparateMovieFolder_CleanExtraNames",
		"FindExtras_SeparateMovieFolderWithMixedExtras_FindsCorrectExtras", "FindExtras_SeparateMovieFolderWithMixedExtras_FindsOnlyExtrasInMovieFolder",
		"FindExtras_SeparateMovieFolderWithParts_FindsCorrectExtras", "FindExtras_WrongExtensions_FindsNoExtras",
		"FindExtras_TrailerWithYearInFilename_SetsProductionYearFromFilename", "FindExtras_SeriesWithTrailers_FindsCorrectExtras",
		"FindExtras_SameExtraInSeveralContainers_ReturnsEach", "FindExtras_SameExtraInSeveralResolutions_ReturnsEach",
		"FindExtras_NumberedExtras_AreKeptApart", "FindExtras_ExtraNamedByLocalMetadata_KeepsItsNameOnRescan",
		"FindExtras_ExtraKeptItsGeneratedName_IsRenumberedOnRescan", "FindExtras_ExtraWithOwnTitleBesideOwner_KeepsTitle",
		"FindExtras_ExtraInOwnFolder_IsNamedAfterItsFile", "FindExtras_DistinctExtrasInSameFolder_AreKeptApart",
	} {
		facts[f] = "TestFindExtras"
	}
	portedCases(t, "library_manager/find_extras.json", ported{facts: facts})
	portedCases(t, "library_manager/resolve_alternate_version.json", ported{skip: map[string]string{
		"ResolveAlternateVersion_StaleWrongTypeItem_DropsRowWithoutResavingPrimary": "Jellyfin stores alternate versions as items typed by resolver; Mavio keeps them as media sources of one item",
		"ResolveAlternateVersion_StaleWrongTypeItem_DropsCachedParentListing":       "Jellyfin stores alternate versions as items typed by resolver; Mavio keeps them as media sources of one item",
	}})
}

// entries turns paths into entries of fs, as the Jellyfin tests do:
// paths without an extension are folders unless files is set.
func entries(paths []string, files bool) []Entry {
	var out []Entry
	for _, p := range paths {
		out = append(out, Entry{Path: p, IsDir: !files && path.Ext(p) == ""})
	}
	return out
}

func findExtras(t *testing.T, owner Node, ownerIsFolder bool, es []Entry, fs fakeFS) []Node {
	t.Helper()
	extras, err := NewResolver().Extras(owner, ownerIsFolder, es, fs)
	if err != nil {
		t.Fatal(err)
	}
	return extras
}

func byKind(extras []Node) []Node {
	// Jellyfin's ExtraType order.
	order := []core.ExtraKind{
		core.ExtraOther, core.ExtraClip, core.ExtraTrailer, core.ExtraBehindTheScene, core.ExtraDeletedScene, core.ExtraInterview,
		core.ExtraScene, core.ExtraSample, core.ExtraThemeSong, core.ExtraThemeVideo, core.ExtraFeaturette, core.ExtraShort,
	}
	rank := func(k core.ExtraKind) int {
		if i := slices.Index(order, k); i >= 0 {
			return i
		}
		return len(order)
	}
	out := slices.Clone(extras)
	slices.SortStableFunc(out, func(a, b Node) int { return rank(a.Extra) - rank(b.Extra) })
	return out
}

func names(extras []Node) map[string]string {
	m := map[string]string{}
	for _, e := range extras {
		m[e.Path] = e.Name
	}
	return m
}

func kinds(extras []Node) []core.ExtraKind {
	var out []core.ExtraKind
	for _, e := range extras {
		out = append(out, e.Extra)
	}
	return out
}

func TestFindExtras(t *testing.T) {
	up := Node{Kind: core.KindMovie, Name: "Up", Path: "/movies/Up/Up.mkv"}
	t.Run("separate movie folder", func(t *testing.T) {
		es := entries([]string{"/movies/Up/Up.mkv", "/movies/Up/Up - trailer.mkv", "/movies/Up/Up - sample.mkv", "/movies/Up/Up something else.mkv", "/movies/Up/Up-extra.mkv"}, true)
		got := kinds(byKind(findExtras(t, up, false, es, nil)))
		if want := []core.ExtraKind{core.ExtraOther, core.ExtraTrailer, core.ExtraSample}; !slices.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	t.Run("clean names", func(t *testing.T) {
		fs := newFS("/movies/Up/shorts/Balloons[1080p].mkv")
		es := entries([]string{"/movies/Up/Up.mkv", "/movies/Up/Recording the audio[Bluray]-behindthescenes.mkv", "/movies/Up/Interview with the dog-interview.mkv", "/movies/Up/shorts/Balloons[1080p].mkv"}, true)
		got := byKind(findExtras(t, up, false, es, fs))
		want := []struct {
			kind core.ExtraKind
			name string
		}{{core.ExtraBehindTheScene, "Recording the audio"}, {core.ExtraInterview, "Interview with the dog"}, {core.ExtraShort, "Balloons"}}
		if len(got) != len(want) {
			t.Fatalf("got = %+v", got)
		}
		for i, w := range want {
			if got[i].Extra != w.kind || got[i].Name != w.name {
				t.Errorf("%d: got = %s %q, want = %s %q", i, got[i].Extra, got[i].Name, w.kind, w.name)
			}
		}
	})
	t.Run("mixed extras", func(t *testing.T) {
		fs := newFS("/movies/Up/trailers/some trailer.mkv", "/movies/Up/behind the scenes/the making of Up.mkv",
			"/movies/Up/theme-music/theme2.mp3", "/movies/Up/extras/Honest Trailer.mkv")
		es := entries([]string{
			"/movies/Up/Up.mkv", "/movies/Up/Up - trailer.mkv", "/movies/Up/trailers", "/movies/Up/theme-music",
			"/movies/Up/theme.mp3", "/movies/Up/not a theme.mp3", "/movies/Up/behind the scenes", "/movies/Up/behind the scenes.mkv",
			"/movies/Up/Up - sample.mkv", "/movies/Up/Up something else.mkv", "/movies/Up/extras",
		}, false)
		got := byKind(findExtras(t, up, false, es, fs))
		want := []core.ExtraKind{core.ExtraOther, core.ExtraTrailer, core.ExtraTrailer, core.ExtraBehindTheScene, core.ExtraSample, core.ExtraThemeSong, core.ExtraThemeSong}
		if !slices.Equal(kinds(got), want) {
			t.Fatalf("got = %v, want = %v", kinds(got), want)
		}
		for _, e := range got {
			wantKind := core.KindVideo
			if e.Extra == core.ExtraThemeSong {
				wantKind = core.KindTrack
			}
			if e.Kind != wantKind {
				t.Errorf("%s: got = %s, want = %s", e.Path, e.Kind, wantKind)
			}
		}
	})
	t.Run("only extras in the movie folder", func(t *testing.T) {
		for _, owner := range []Node{up, {Kind: core.KindMovie, Name: "Up", Path: "/movies/Up/Up - part1.mkv"}} {
			paths := []string{owner.Path, "/movies/Up/trailer.mkv", "/movies/Another Movie/trailer.mkv"}
			if owner.Path != up.Path {
				paths = append(paths, "/movies/Up/Up - part2.mkv")
			}
			got := findExtras(t, owner, false, entries(paths, true), nil)
			if len(got) != 1 || got[0].Path != "/movies/Up/trailer.mkv" || got[0].Extra != core.ExtraTrailer {
				t.Errorf("%s: got = %+v", owner.Path, got)
			}
		}
	})
	t.Run("wrong extensions", func(t *testing.T) {
		fs := newFS("/movies/Up/trailers/trailer.jpg")
		es := entries([]string{"/movies/Up/Up.mkv", "/movies/Up/trailer.noext", "/movies/Up/theme.png", "/movies/Up/trailers"}, false)
		es[1].IsDir = false
		if got := findExtras(t, up, false, es, fs); len(got) != 0 {
			t.Errorf("got = %+v", got)
		}
	})
	t.Run("year in trailer name", func(t *testing.T) {
		fs := newFS("/movies/Up/trailers/Trailer 1 (2013).mkv")
		got := findExtras(t, up, false, entries([]string{"/movies/Up/Up.mkv", "/movies/Up/trailers"}, false), fs)
		if len(got) != 1 || got[0].Extra != core.ExtraTrailer || got[0].Year == nil || *got[0].Year != 2013 {
			t.Errorf("got = %+v", got)
		}
	})
	t.Run("series trailers", func(t *testing.T) {
		dexter := Node{Kind: core.KindSeries, Name: "Dexter", Path: "/series/Dexter"}
		es := entries([]string{"/series/Dexter/Season 1/S01E01.mkv", "/series/Dexter/trailer.mkv", "/series/Dexter/trailers/trailer2.mkv"}, true)
		got := findExtras(t, dexter, true, es, nil)
		if len(got) != 2 || got[0].Path != "/series/Dexter/trailer.mkv" || got[1].Path != "/series/Dexter/trailers/trailer2.mkv" {
			t.Errorf("got = %+v", got)
		}
	})
	t.Run("same extra in several containers", func(t *testing.T) {
		dir := "/movies/Skyscraper (2018)/"
		owner := Node{Kind: core.KindMovie, Name: "Skyscraper", Path: dir + "Skyscraper (2018) - [1080p HEVC].mkv"}
		es := entries([]string{
			owner.Path, dir + "Skyscraper (2018) - [1080p HEVC]-trailer.mkv", dir + "Skyscraper (2018) - [1080p HEVC]-trailer.mp4",
			dir + "Skyscraper (2018) - [1080p HEVC]-behindthescenes.mkv", dir + "Skyscraper (2018) - [1080p HEVC]-behindthescenes.mp4",
		}, true)
		want := map[string]string{
			dir + "Skyscraper (2018) - [1080p HEVC]-behindthescenes.mkv": "Behind The Scenes",
			dir + "Skyscraper (2018) - [1080p HEVC]-behindthescenes.mp4": "Behind The Scenes 2",
			dir + "Skyscraper (2018) - [1080p HEVC]-trailer.mkv":         "Trailer",
			dir + "Skyscraper (2018) - [1080p HEVC]-trailer.mp4":         "Trailer 2",
		}
		if got := names(findExtras(t, owner, false, es, nil)); !maps.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	t.Run("same extra in several resolutions", func(t *testing.T) {
		dir := "/movies/Dragon 2 (2014)/"
		owner := Node{Kind: core.KindMovie, Name: "Dragon 2", Path: dir + "Dragon 2 (2014) - [2160p].mkv"}
		es := entries([]string{owner.Path, dir + "Dragon 2 (2014) - [1080p]-trailer.mkv", dir + "Dragon 2 (2014) - [2160p]-trailer.mkv"}, true)
		want := map[string]string{dir + "Dragon 2 (2014) - [1080p]-trailer.mkv": "Trailer", dir + "Dragon 2 (2014) - [2160p]-trailer.mkv": "Trailer 2"}
		if got := names(findExtras(t, owner, false, es, nil)); !maps.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	up2009 := Node{Kind: core.KindMovie, Name: "Up", Path: "/movies/Up (2009)/Up (2009).mkv"}
	dir := "/movies/Up (2009)/"
	t.Run("numbered extras", func(t *testing.T) {
		es := entries([]string{up2009.Path, dir + "Up (2009)-trailer.mkv", dir + "Up (2009)-trailer2.mkv", dir + "Up (2009)-trailer2.mp4", dir + "Up (2009)-trailer3.mkv"}, true)
		got := findExtras(t, up2009, false, es, nil)
		want := []string{"Trailer", "Trailer 2", "Trailer 3", "Trailer 4"}
		var gotNames []string
		for _, e := range got {
			gotNames = append(gotNames, e.Name)
		}
		if !slices.Equal(gotNames, want) || got[1].Path != dir+"Up (2009)-trailer2.mkv" {
			t.Errorf("got = %v, want = %v", gotNames, want)
		}
	})
	t.Run("name from local metadata survives a rescan", func(t *testing.T) {
		es := entries([]string{up2009.Path, dir + "Up (2009)-trailer.mkv"}, true)
		got := findExtras(t, up2009, false, es, nil)
		if len(got) != 1 || got[0].Name != "Trailer" {
			t.Fatalf("got = %+v", got)
		}
		if name := RenewExtraName("Cannes Teaser", false, got[0]); name != "Cannes Teaser" {
			t.Errorf("got = %q, want = Cannes Teaser", name)
		}
	})
	t.Run("generated name is renumbered", func(t *testing.T) {
		es := entries([]string{up2009.Path, dir + "Up (2009)-trailer2.mkv"}, true)
		first := findExtras(t, up2009, false, es, nil)
		if len(first) != 1 || first[0].Name != "Trailer" {
			t.Fatalf("got = %+v", first)
		}
		es = append(es, Entry{Path: dir + "Up (2009)-trailer1.mkv"})
		got := map[string]string{}
		for _, e := range findExtras(t, up2009, false, es, nil) {
			stored := ""
			if e.Path == first[0].Path {
				stored = first[0].Name
			}
			got[e.Path] = RenewExtraName(stored, false, e)
		}
		want := map[string]string{dir + "Up (2009)-trailer1.mkv": "Trailer", dir + "Up (2009)-trailer2.mkv": "Trailer 2"}
		if !maps.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	t.Run("own title beside the owner", func(t *testing.T) {
		es := entries([]string{up2009.Path, dir + "Up (2009)-trailer.mkv", dir + "Recording the audio-behindthescenes.mkv", dir + "Up (2009)-behindthescenes.mkv"}, true)
		want := map[string]string{
			dir + "Up (2009)-trailer.mkv":                   "Trailer",
			dir + "Recording the audio-behindthescenes.mkv": "Recording the audio",
			dir + "Up (2009)-behindthescenes.mkv":           "Behind The Scenes",
		}
		if got := names(findExtras(t, up2009, false, es, nil)); !maps.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	t.Run("extras in their own folder keep their names", func(t *testing.T) {
		fs := newFS("/movies/Up/trailers/Teaser.mkv", "/movies/Up/trailers/Comic-Con Reel.mkv")
		got := names(findExtras(t, up, false, entries([]string{"/movies/Up/Up.mkv", "/movies/Up/trailers"}, false), fs))
		want := map[string]string{"/movies/Up/trailers/Teaser.mkv": "Teaser", "/movies/Up/trailers/Comic-Con Reel.mkv": "Comic-Con Reel"}
		if !maps.Equal(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
	t.Run("distinct extras in one folder", func(t *testing.T) {
		fs := newFS("/movies/Up/trailers/Teaser.mkv", "/movies/Up/trailers/Official.mkv", "/movies/Up/trailers/Official.mp4")
		got := findExtras(t, up, false, entries([]string{"/movies/Up/Up.mkv", "/movies/Up/trailers"}, false), fs)
		var paths []string
		for _, e := range got {
			paths = append(paths, e.Path)
		}
		want := []string{"/movies/Up/trailers/Official.mkv", "/movies/Up/trailers/Official.mp4", "/movies/Up/trailers/Teaser.mkv"}
		if !slices.Equal(paths, want) {
			t.Errorf("got = %v, want = %v", paths, want)
		}
	})
}

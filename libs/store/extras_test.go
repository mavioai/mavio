package store_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestTrickplaySegmentsAndLoudness(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{"/media/shows"}, ExtractTrickplay: true, AnalyzeLoudness: true}
		if err := s.Libraries().Create(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Libraries().Get(ctx, lib.ID); !got.ExtractTrickplay || got.ExtractChapterImages || !got.AnalyzeLoudness {
			t.Errorf("library options = %+v", got)
		}
		lufs := -14.5
		ep := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindEpisode, Name: "Pilot", Path: "/media/shows/a.mkv", DateAdded: time.Now(), Loudness: &lufs}
		if err := s.Items().Upsert(ctx, ep); err != nil {
			t.Fatal(err)
		}
		got, err := s.Items().Get(ctx, ep.ID)
		if err != nil || got.Loudness == nil || *got.Loudness != lufs || *got.NormalizationGain() != -3.5 {
			t.Errorf("loudness = %v, %v", got.Loudness, err)
		}
		ep.Loudness = nil
		if err := s.Items().Upsert(ctx, ep); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Items().Get(ctx, ep.ID); got.Loudness != nil {
			t.Errorf("loudness after clearing = %v", *got.Loudness)
		}

		tp := core.Trickplay{ItemID: ep.ID, Width: 320, Height: 180, TileWidth: 10, TileHeight: 10, ThumbnailCount: 250, Interval: 10 * time.Second}
		if err := s.Trickplay().Put(ctx, &tp); err != nil {
			t.Fatal(err)
		}
		tp.ThumbnailCount = 260
		if err := s.Trickplay().Put(ctx, &tp); err != nil {
			t.Fatal(err)
		}
		list, err := s.Trickplay().List(ctx, ep.ID)
		if err != nil || len(list) != 1 || list[0].ThumbnailCount != 260 || list[0].Sheets() != 3 || list[0].Interval != 10*time.Second {
			t.Errorf("trickplay = %+v, %v", list, err)
		}
		if err := s.Trickplay().Put(ctx, &core.Trickplay{ItemID: ep.ID}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("empty trickplay: %v", err)
		}

		segs := []core.MediaSegment{
			{Kind: core.SegmentOutro, Start: 40 * time.Minute, End: 42 * time.Minute, Provider: "p"},
			{Kind: core.SegmentIntro, Start: 30 * time.Second, End: 90 * time.Second, Provider: "p"},
		}
		if err := s.MediaSegments().Replace(ctx, ep.ID, segs); err != nil {
			t.Fatal(err)
		}
		got2, err := s.MediaSegments().List(ctx, ep.ID)
		if err != nil || len(got2) != 2 || got2[0].Kind != core.SegmentIntro || got2[1].End != 42*time.Minute {
			t.Errorf("segments = %+v, %v", got2, err)
		}
		if err := s.MediaSegments().Replace(ctx, ep.ID, []core.MediaSegment{{Kind: "ad", Start: 0, End: time.Second}}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("unknown segment kind: %v", err)
		}
		// They go with their item.
		if err := s.Items().Delete(ctx, ep.ID); err != nil {
			t.Fatal(err)
		}
		if l, _ := s.Trickplay().List(ctx, ep.ID); len(l) != 0 {
			t.Errorf("trickplay of a deleted item: %+v", l)
		}
		if l, _ := s.MediaSegments().List(ctx, ep.ID); len(l) != 0 {
			t.Errorf("segments of a deleted item: %+v", l)
		}
	})
}

func TestProviderOrder(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := core.Library{
			Name: "Music", Kind: core.LibraryMusic, Paths: []string{"/media/music"}, DownloadLyrics: true,
			Providers:       core.ProviderOrder{Lyrics: []string{"lrclib", "genius"}, Images: []string{}, Episodes: []string{"scraper-tmdb"}},
			VirtualEpisodes: core.VirtualEpisodes{Unaired: true, GraceDays: 7},
		}
		if err := s.Libraries().Create(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		got, err := s.Libraries().Get(ctx, lib.ID)
		if err != nil || !got.DownloadLyrics || !reflect.DeepEqual(got.Providers, lib.Providers) || got.VirtualEpisodes != lib.VirtualEpisodes {
			t.Errorf("got = %+v, %v; want providers %+v", got.Providers, err, lib.Providers)
		}
		lib.Providers = core.ProviderOrder{}
		if err := s.Libraries().Update(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Libraries().Get(ctx, lib.ID); !reflect.DeepEqual(got.Providers, core.ProviderOrder{}) {
			t.Errorf("got = %+v, want every provider", got.Providers)
		}
	})
}

func TestVirtualItems(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		lib := newLibrary(t, s, "/media/shows")
		series := newItem(lib, core.KindSeries, "Show")
		series.Path, series.Monitored = "/media/shows/Show", true
		season := newItem(lib, core.KindSeason, "Season 1")
		season.ParentID, season.IndexNumber, season.Virtual = series.ID, ptr(1), true
		present := newItem(lib, core.KindEpisode, "Present")
		present.ParentID, present.Path = series.ID, "/media/shows/Show/S01E01.mkv"
		present.ParentIndexNumber, present.IndexNumber = ptr(1), ptr(1)
		wanted := newItem(lib, core.KindEpisode, "Wanted")
		wanted.ParentID, wanted.Virtual, wanted.Monitored = season.ID, true, true
		wanted.ParentIndexNumber, wanted.IndexNumber = ptr(1), ptr(2)
		upsert(t, s, series, season, present, wanted)

		under := core.ItemQuery{ParentID: series.ID, Recursive: true, Sort: []core.SortSpec{{Field: core.SortName}}}
		if got := query(t, s, under); !slices.Equal(got, []string{"Present"}) {
			t.Errorf("default = %v, want = [Present]", got)
		}
		with := under
		with.IncludeVirtual = true
		if got := query(t, s, with); !slices.Equal(got, []string{"Present", "Season 1", "Wanted"}) {
			t.Errorf("with virtual = %v, want = [Present Season 1 Wanted]", got)
		}
		only := under
		only.OnlyVirtual, only.Kinds = true, []core.ItemKind{core.KindEpisode}
		if got := query(t, s, only); !slices.Equal(got, []string{"Wanted"}) {
			t.Errorf("only virtual = %v, want = [Wanted]", got)
		}
		numbered := with
		numbered.ParentIndexNumber, numbered.IndexNumber = ptr(1), ptr(2)
		if got := query(t, s, numbered); !slices.Equal(got, []string{"Wanted"}) {
			t.Errorf("numbered = %v, want = [Wanted]", got)
		}
		monitored := core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, IncludeVirtual: true, Monitored: ptr(true), Sort: []core.SortSpec{{Field: core.SortName}}}
		if got := query(t, s, monitored); !slices.Equal(got, []string{"Show", "Wanted"}) {
			t.Errorf("monitored = %v, want = [Show Wanted]", got)
		}
	})
}

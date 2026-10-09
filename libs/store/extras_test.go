package store_test

import (
	"errors"
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

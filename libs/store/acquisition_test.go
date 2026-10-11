package store_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func movieProfile(name string) core.QualityProfile {
	return core.QualityProfile{
		ID: core.NewID(), Kind: core.KindMovie, Name: name, UpgradeAllowed: true, Cutoff: 7,
		Items: []core.QualityProfileItem{
			{Quality: 3, Allowed: true},
			{ID: 1001, Name: "HD", Allowed: true, Items: []core.QualityProfileItem{{Quality: 7, Allowed: true}, {Quality: 9, Allowed: true}}},
		},
		MinFormatScore: -10, CutoffFormatScore: 100, MinUpgradeFormatScore: 1, Language: "English",
	}
}

func TestQualityDefinitions(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.QualityDefinitions()
		if got, err := repo.List(ctx, core.KindMovie); err != nil || len(got) != 0 {
			t.Fatalf("List before Replace = %+v, %v; want none", got, err)
		}
		movies := []core.QualityDefinition{
			{Quality: 7, Title: "Bluray-1080p", MinSize: 2, MaxSize: 100, PreferredSize: 95},
			{Kind: core.KindMovie, Quality: 3, Title: "WEBDL-1080p", MinSize: 1.5},
		}
		if err := repo.Replace(ctx, core.KindMovie, movies); err != nil {
			t.Fatal(err)
		}
		if err := repo.Replace(ctx, core.KindSeries, []core.QualityDefinition{{Quality: 7, Title: "Bluray-1080p"}}); err != nil {
			t.Fatal(err)
		}
		got, err := repo.List(ctx, core.KindMovie)
		want := []core.QualityDefinition{movies[1], movies[0]}
		want[1].Kind = core.KindMovie
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("List = %+v, %v; want = %+v", got, err, want)
		}
		if err := repo.Replace(ctx, core.KindMovie, movies[:1]); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.List(ctx, core.KindMovie); len(got) != 1 || got[0].Quality != 7 {
			t.Errorf("List after second Replace = %+v, want one", got)
		}
		if got, _ := repo.List(ctx, core.KindSeries); len(got) != 1 {
			t.Errorf("series definitions = %+v, want one", got)
		}
		for name, defs := range map[string][]core.QualityDefinition{
			"other kind": {{Kind: core.KindSeries, Quality: 1, Title: "SDTV"}},
			"no title":   {{Quality: 1}},
		} {
			if err := repo.Replace(ctx, core.KindMovie, defs); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Replace with %s: %v, want ErrInvalid", name, err)
			}
		}
		if err := repo.Replace(ctx, core.KindEpisode, nil); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Replace of episodes: %v, want ErrInvalid", err)
		}
		dup := []core.QualityDefinition{{Quality: 1, Title: "SDTV"}, {Quality: 1, Title: "SDTV"}}
		if err := repo.Replace(ctx, core.KindMovie, dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Replace with a duplicate: %v, want ErrConflict", err)
		}
		if got, _ := repo.List(ctx, core.KindMovie); len(got) != 1 {
			t.Errorf("failed Replace changed the definitions: %+v", got)
		}
	})
}

func TestQualityProfilesAndCustomFormats(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		formats, profiles := s.CustomFormats(), s.QualityProfiles()

		x265 := core.CustomFormat{
			Name: "x265", IncludeWhenRenaming: true,
			Specifications: []core.CustomFormatSpec{
				{Kind: core.SpecReleaseTitle, Name: "x265", Value: `[xh]\.?265`, Required: true},
				{Kind: core.SpecSize, Name: "Big", Min: 1.5, Max: 20, Negate: true},
			},
		}
		hdr := core.CustomFormat{ID: core.NewID(), Name: "HDR"}
		for _, f := range []*core.CustomFormat{&x265, &hdr} {
			if err := formats.Put(ctx, f); err != nil {
				t.Fatal(err)
			}
		}
		got, err := formats.Get(ctx, x265.ID)
		if err != nil || !reflect.DeepEqual(got, x265) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, x265)
		}
		list, err := formats.List(ctx)
		if err != nil || len(list) != 2 || list[0].Name != "HDR" || list[1].Name != "x265" {
			t.Errorf("List = %+v, %v; want HDR, x265", list, err)
		}
		if err := formats.Put(ctx, &core.CustomFormat{ID: core.NewID(), Name: "HDR"}); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a second HDR: %v, want ErrConflict", err)
		}
		if err := formats.Put(ctx, &core.CustomFormat{ID: core.NewID()}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without a name: %v, want ErrInvalid", err)
		}

		hd := movieProfile("HD")
		hd.FormatScores = []core.FormatScore{{FormatID: x265.ID, Score: 50}, {FormatID: hdr.ID, Score: 20}}
		uhd := movieProfile("Ultra-HD")
		uhd.FormatScores = []core.FormatScore{{FormatID: x265.ID, Score: 10}}
		show := movieProfile("HD")
		show.Kind = core.KindSeries
		for _, p := range []*core.QualityProfile{&uhd, &hd, &show} {
			if err := profiles.Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		gotP, err := profiles.Get(ctx, hd.ID)
		if err != nil || !reflect.DeepEqual(gotP, hd) {
			t.Errorf("Get = %+v, %v; want = %+v", gotP, err, hd)
		}
		movies, err := profiles.List(ctx, core.KindMovie)
		if err != nil || len(movies) != 2 || movies[0].Name != "HD" || movies[1].Name != "Ultra-HD" {
			t.Errorf("List(movie) = %+v, %v; want HD, Ultra-HD", movies, err)
		}
		if all, err := profiles.List(ctx, ""); err != nil || len(all) != 3 {
			t.Errorf("List(all) = %d profiles, %v; want 3", len(all), err)
		}
		dup := movieProfile("Ultra-HD")
		if err := profiles.Put(ctx, &dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a second movie Ultra-HD: %v, want ErrConflict", err)
		}
		hd.Cutoff = 9
		if err := profiles.Put(ctx, &hd); err != nil {
			t.Fatal(err)
		}
		if gotP, _ := profiles.Get(ctx, hd.ID); gotP.Cutoff != 9 {
			t.Errorf("Cutoff after update = %d, want = 9", gotP.Cutoff)
		}
		if err := profiles.Put(ctx, &core.QualityProfile{ID: core.NewID(), Kind: core.KindMovie, Name: "Empty"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without qualities: %v, want ErrInvalid", err)
		}

		// Deleting a format removes its scores.
		if err := formats.Delete(ctx, x265.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := formats.Get(ctx, x265.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted format: %v, want ErrNotFound", err)
		}
		if gotP, _ := profiles.Get(ctx, hd.ID); !reflect.DeepEqual(gotP.FormatScores, []core.FormatScore{{FormatID: hdr.ID, Score: 20}}) {
			t.Errorf("HD scores = %+v, want HDR's only", gotP.FormatScores)
		}
		if gotP, _ := profiles.Get(ctx, uhd.ID); len(gotP.FormatScores) != 0 {
			t.Errorf("Ultra-HD scores = %+v, want none", gotP.FormatScores)
		}
		if err := formats.Delete(ctx, x265.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}

		// A profile wanted items use cannot be deleted.
		lib := newLibrary(t, s, "/movies")
		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		if err := s.Wanted().Put(ctx, &core.Wanted{ItemID: movie.ID, QualityProfileID: hd.ID}); err != nil {
			t.Fatal(err)
		}
		if err := profiles.Delete(ctx, hd.ID); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Delete of a used profile: %v, want ErrConflict", err)
		}
		if err := profiles.Delete(ctx, uhd.ID); err != nil {
			t.Errorf("Delete of an unused profile: %v", err)
		}
		if _, err := profiles.Get(ctx, uhd.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted profile: %v, want ErrNotFound", err)
		}
		if err := profiles.Delete(ctx, uhd.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
		if err := s.Wanted().Delete(ctx, movie.ID); err != nil {
			t.Fatal(err)
		}
		if err := profiles.Delete(ctx, hd.ID); err != nil {
			t.Errorf("Delete of a profile no longer used: %v", err)
		}
	})
}

func TestDelayProfiles(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.DelayProfiles()
		def := core.DelayProfile{
			Default: true, Order: 100, EnableUsenet: true, EnableTorrent: true,
			PreferredProtocol: core.ProtocolUsenet,
		}
		anime := core.DelayProfile{
			ID: core.NewID(), Order: 1, EnableTorrent: true, PreferredProtocol: core.ProtocolTorrent,
			TorrentDelay: 90 * time.Minute, BypassIfHighestQuality: true, BypassIfAboveCustomFormatScore: true,
			MinimumCustomFormatScore: 50, Tags: []string{"anime"},
		}
		for _, p := range []*core.DelayProfile{&def, &anime} {
			if err := repo.Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.Get(ctx, anime.ID)
		if err != nil || !reflect.DeepEqual(got, anime) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, anime)
		}
		list, err := repo.List(ctx)
		if err != nil || len(list) != 2 || list[0].ID != anime.ID || list[1].ID != def.ID {
			t.Errorf("List = %+v, %v; want anime, default", list, err)
		}
		other := def
		other.ID = core.NewID()
		if err := repo.Put(ctx, &other); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a second default: %v, want ErrConflict", err)
		}
		if err := repo.Put(ctx, &core.DelayProfile{ID: core.NewID(), EnableUsenet: true, PreferredProtocol: core.ProtocolUsenet}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without tags: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, def.ID); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Delete of the default: %v, want ErrConflict", err)
		}
		if err := repo.Delete(ctx, anime.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, anime.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted profile: %v, want ErrNotFound", err)
		}
		if err := repo.Delete(ctx, anime.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
	})
}

func TestReleaseProfiles(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.ReleaseProfiles()
		b := core.ReleaseProfile{
			Name: "B", Enabled: true, Required: []string{"x265", "/hevc/i"}, IndexerID: "indexers/1", Tags: []string{"4k"},
		}
		a := core.ReleaseProfile{ID: core.NewID(), Name: "A", Ignored: []string{"CAM"}}
		for _, p := range []*core.ReleaseProfile{&b, &a} {
			if err := repo.Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.Get(ctx, b.ID)
		if err != nil || !reflect.DeepEqual(got, b) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, b)
		}
		list, err := repo.List(ctx)
		if err != nil || len(list) != 2 || list[0].Name != "A" || list[1].Name != "B" {
			t.Errorf("List = %+v, %v; want A, B", list, err)
		}
		a.Enabled = true
		if err := repo.Put(ctx, &a); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.Get(ctx, a.ID); !got.Enabled {
			t.Error("update was not stored")
		}
		if err := repo.Put(ctx, &core.ReleaseProfile{ID: core.NewID()}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without terms: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
	})
}

func TestWanted(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		p := movieProfile("HD")
		if err := s.QualityProfiles().Put(ctx, &p); err != nil {
			t.Fatal(err)
		}
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		repo := s.Wanted()
		if _, err := repo.Get(ctx, heat.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get before Put: %v, want ErrNotFound", err)
		}
		added := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		w := core.Wanted{
			ItemID: heat.ID, QualityProfileID: p.ID, MinimumAvailability: core.AvailabilityReleased,
			Tags: []string{"4k"}, AddedAt: added,
		}
		if err := repo.Put(ctx, &w); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, heat.ID)
		if err != nil || !reflect.DeepEqual(got, w) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, w)
		}
		// An update keeps the time it was added.
		w.AddedAt = time.Time{}
		w.LastSearchedAt = added.Add(time.Hour)
		if err := repo.Put(ctx, &w); err != nil {
			t.Fatal(err)
		}
		got, _ = repo.Get(ctx, heat.ID)
		if !got.AddedAt.Equal(added) || !got.LastSearchedAt.Equal(added.Add(time.Hour)) {
			t.Errorf("after update AddedAt = %v, LastSearchedAt = %v; want %v, %v", got.AddedAt, got.LastSearchedAt, added, added.Add(time.Hour))
		}
		wa := core.Wanted{ItemID: alien.ID, QualityProfileID: p.ID}
		if err := repo.Put(ctx, &wa); err != nil {
			t.Fatal(err)
		}
		if wa.AddedAt.IsZero() {
			t.Error("Put left AddedAt zero")
		}
		if list, err := repo.List(ctx); err != nil || len(list) != 2 || list[0].ItemID != heat.ID {
			t.Errorf("List() = %+v, %v; want Heat, Alien", list, err)
		}
		if list, err := repo.List(ctx, alien.ID); err != nil || len(list) != 1 || list[0].ItemID != alien.ID {
			t.Errorf("List(Alien) = %+v, %v", list, err)
		}
		if err := repo.Put(ctx, &core.Wanted{ItemID: heat.ID}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without a profile: %v, want ErrInvalid", err)
		}
		if err := repo.Put(ctx, &core.Wanted{ItemID: heat.ID, QualityProfileID: p.ID, MonitorNewItems: "some"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put with unknown new item monitoring: %v, want ErrInvalid", err)
		}
		// A series keeps its new item monitoring and season folder flag.
		shows := newLibrary(t, s, "/shows")
		show := newItem(shows, core.KindSeries, "Show")
		upsert(t, s, show)
		ws := core.Wanted{ItemID: show.ID, QualityProfileID: p.ID, MonitorNewItems: core.NewItemsNone, SeasonFolder: true, AddedAt: added}
		if err := repo.Put(ctx, &ws); err != nil {
			t.Fatal(err)
		}
		if got, err := repo.Get(ctx, show.ID); err != nil || !reflect.DeepEqual(got, ws) {
			t.Errorf("Get(series) = %+v, %v; want = %+v", got, err, ws)
		}
		if err := repo.Delete(ctx, show.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Put(ctx, &core.Wanted{ItemID: heat.ID, QualityProfileID: core.NewID()}); err == nil {
			t.Error("Put with an unknown profile succeeded")
		}
		if err := repo.Delete(ctx, alien.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, alien.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
		// The wanted state goes with its item.
		if err := s.Items().Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, heat.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted item: %v, want ErrNotFound", err)
		}
	})
}

func TestDownloads(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/shows")
		series, episodes := showTree(t, s, lib)
		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		repo := s.Downloads()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		season := core.Download{
			PluginID: "download-clients", ClientID: "transmission", DownloadID: "ABC",
			ItemID: series.ID, ItemIDs: []core.ID{episodes[0].ID, episodes[1].ID}, Title: "The.Wire.S01.1080p",
			IndexerID: "indexers/1", Indexer: "Nyaa", Protocol: core.ProtocolTorrent,
			Quality: core.QualityRef{Quality: 7, Version: 2, IsRepack: true}, Languages: []string{"English"},
			Size: 4 << 30, State: core.DownloadDownloading, Messages: []string{"stalled"},
			GrabbedAt: base, UpdatedAt: base.Add(time.Minute),
		}
		heat := core.Download{
			ID: core.NewID(), PluginID: "download-clients", ClientID: "sabnzbd", DownloadID: "nzo_1",
			ItemID: movie.ID, Protocol: core.ProtocolUsenet, State: core.DownloadImported, GrabbedAt: base.Add(time.Hour),
		}
		for _, d := range []*core.Download{&season, &heat} {
			if err := repo.Put(ctx, d); err != nil {
				t.Fatal(err)
			}
		}
		if heat.UpdatedAt.IsZero() {
			t.Error("Put left UpdatedAt zero")
		}
		got, err := repo.Get(ctx, season.ID)
		if err != nil || !reflect.DeepEqual(got, season) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, season)
		}
		got, err = repo.GetByDownloadID(ctx, "download-clients", "transmission", "ABC")
		if err != nil || got.ID != season.ID {
			t.Errorf("GetByDownloadID = %+v, %v", got, err)
		}
		if _, err := repo.GetByDownloadID(ctx, "download-clients", "sabnzbd", "ABC"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("GetByDownloadID of another client: %v, want ErrNotFound", err)
		}
		dup := season
		dup.ID = core.NewID()
		if err := repo.Put(ctx, &dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a tracked download: %v, want ErrConflict", err)
		}
		season.State = core.DownloadImportBlocked
		if err := repo.Put(ctx, &season); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.Get(ctx, season.ID); got.State != core.DownloadImportBlocked {
			t.Errorf("State after update = %q, want = import_blocked", got.State)
		}

		ids := func(p core.Page[core.Download]) []core.ID {
			out := make([]core.ID, len(p.Items))
			for i, d := range p.Items {
				out[i] = d.ID
			}
			return out
		}
		for _, tc := range []struct {
			name  string
			q     core.DownloadQuery
			total int
			want  []core.ID
		}{
			{"all", core.DownloadQuery{}, 2, []core.ID{heat.ID, season.ID}},
			{"page", core.DownloadQuery{Limit: 1, Offset: 1}, 2, []core.ID{season.ID}},
			{"item", core.DownloadQuery{ItemID: series.ID}, 1, []core.ID{season.ID}},
			{"states", core.DownloadQuery{States: []core.DownloadState{core.DownloadImported}}, 1, []core.ID{heat.ID}},
			{"unfinished", core.DownloadQuery{Unfinished: true}, 1, []core.ID{season.ID}},
		} {
			page, err := repo.List(ctx, tc.q)
			if err != nil || page.Total != tc.total || !reflect.DeepEqual(ids(page), tc.want) {
				t.Errorf("List(%s) = %v (total %d), %v; want = %v (total %d)", tc.name, ids(page), page.Total, err, tc.want, tc.total)
			}
		}
		if _, err := repo.List(ctx, core.DownloadQuery{Limit: -1}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("List with a negative limit: %v, want ErrInvalid", err)
		}
		if err := repo.Put(ctx, &core.Download{ID: core.NewID(), ItemID: movie.ID}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put without a client: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, heat.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
		// Downloads go with their item.
		if err := s.Items().Delete(ctx, series.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, season.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get of a deleted item's download: %v, want ErrNotFound", err)
		}
	})
}

func TestHistory(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		series, episodes := showTree(t, s, lib)
		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		repo := s.History()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		grab := core.HistoryEvent{
			Type: core.HistoryGrabbed, ItemID: movie.ID, Date: base, SourceTitle: "Heat.1995.1080p",
			Quality: core.QualityRef{Quality: 7, Version: 1}, Languages: []string{"English"}, DownloadID: "ABC",
			Data: map[string]string{"indexer": "Nyaa"},
		}
		imported := core.HistoryEvent{ID: core.NewID(), Type: core.HistoryImported, ItemID: movie.ID, Date: base.Add(time.Hour), DownloadID: "ABC"}
		episode := core.HistoryEvent{
			ID: core.NewID(), Type: core.HistoryGrabbed, ItemID: episodes[0].ID, ParentID: series.ID, Date: base.Add(2 * time.Hour),
		}
		events := []core.HistoryEvent{grab, imported, episode}
		if err := repo.Add(ctx, events...); err != nil {
			t.Fatal(err)
		}
		grab.ID = events[0].ID
		if grab.ID.IsZero() {
			t.Fatal("Add left the ID zero")
		}

		ids := func(p core.Page[core.HistoryEvent]) []core.ID {
			out := make([]core.ID, len(p.Items))
			for i, h := range p.Items {
				out[i] = h.ID
			}
			return out
		}
		for _, tc := range []struct {
			name  string
			q     core.HistoryQuery
			total int
			want  []core.ID
		}{
			{"all", core.HistoryQuery{}, 3, []core.ID{episode.ID, imported.ID, grab.ID}},
			{"page", core.HistoryQuery{Limit: 1, Offset: 1}, 3, []core.ID{imported.ID}},
			{"movie", core.HistoryQuery{ItemIDs: []core.ID{movie.ID}}, 2, []core.ID{imported.ID, grab.ID}},
			{"series", core.HistoryQuery{ItemIDs: []core.ID{series.ID}}, 1, []core.ID{episode.ID}},
			{"episode", core.HistoryQuery{ItemIDs: []core.ID{episodes[0].ID}}, 1, []core.ID{episode.ID}},
			{"types", core.HistoryQuery{Types: []core.HistoryEventType{core.HistoryGrabbed}}, 2, []core.ID{episode.ID, grab.ID}},
			{"download", core.HistoryQuery{DownloadID: "ABC"}, 2, []core.ID{imported.ID, grab.ID}},
			{"since", core.HistoryQuery{Since: base.Add(time.Hour)}, 2, []core.ID{episode.ID, imported.ID}},
		} {
			page, err := repo.List(ctx, tc.q)
			if err != nil || page.Total != tc.total || !reflect.DeepEqual(ids(page), tc.want) {
				t.Errorf("List(%s) = %v (total %d), %v; want = %v (total %d)", tc.name, ids(page), page.Total, err, tc.want, tc.total)
			}
		}
		page, err := repo.List(ctx, core.HistoryQuery{ItemIDs: []core.ID{movie.ID}, Types: []core.HistoryEventType{core.HistoryGrabbed}})
		if err != nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], grab) {
			t.Errorf("List = %+v, %v; want = %+v", page.Items, err, grab)
		}
		if _, err := repo.List(ctx, core.HistoryQuery{Limit: core.MaxPageSize + 1}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("List with a large limit: %v, want ErrInvalid", err)
		}
		if err := repo.Add(ctx, core.HistoryEvent{ID: core.NewID(), Type: "bogus", ItemID: movie.ID, Date: base}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Add of an unknown type: %v, want ErrInvalid", err)
		}

		// The events of an item move to the item taking its place, which
		// keeps them when the item goes.
		if err := repo.MoveItems(ctx, episodes[1].ID, episodes[0].ID, episodes[1].ID); err != nil {
			t.Fatal(err)
		}
		if err := s.Items().Delete(ctx, episodes[0].ID); err != nil {
			t.Fatal(err)
		}
		page, err = repo.List(ctx, core.HistoryQuery{ItemIDs: []core.ID{episodes[1].ID}})
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != episode.ID || page.Items[0].ItemID != episodes[1].ID {
			t.Errorf("events of the item taking the place = %+v, %v; want = the moved grab", page.Items, err)
		}
		if err := repo.MoveItems(ctx, episodes[1].ID); err != nil {
			t.Errorf("MoveItems of none: %v", err)
		}

		n, err := repo.Purge(ctx, base.Add(90*time.Minute))
		if err != nil || n != 2 {
			t.Errorf("Purge = %d, %v; want = 2", n, err)
		}
		// Events go with their item.
		if err := s.Items().Delete(ctx, series.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.HistoryQuery{}); page.Total != 0 {
			t.Errorf("events after deleting the series = %d, want = 0", page.Total)
		}
	})
}

func TestBlocklist(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		repo := s.Blocklist()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		old := core.BlocklistEntry{
			ItemID: heat.ID, ItemIDs: []core.ID{heat.ID}, SourceTitle: "Heat.1995.CAM", Quality: core.QualityRef{Quality: 1, Real: 1},
			Languages: []string{"English"}, Date: base, PublishedDate: base.Add(-24 * time.Hour), Size: 700 << 20,
			Protocol: core.ProtocolTorrent, IndexerID: "indexers/1", Indexer: "Nyaa", IndexerFlags: []string{"freeleech"},
			Message: "failed", TorrentInfoHash: "abc",
		}
		recent := core.BlocklistEntry{ID: core.NewID(), ItemID: heat.ID, SourceTitle: "Heat.1995.TS", Date: base.Add(time.Hour), Protocol: core.ProtocolUsenet}
		other := core.BlocklistEntry{ID: core.NewID(), ItemID: alien.ID, SourceTitle: "Alien.1979", Date: base.Add(2 * time.Hour), Protocol: core.ProtocolUsenet}
		for _, b := range []*core.BlocklistEntry{&old, &recent, &other} {
			if err := repo.Add(ctx, b); err != nil {
				t.Fatal(err)
			}
		}
		page, err := repo.List(ctx, heat.ID, 0, 0)
		if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Items[0].ID != recent.ID || !reflect.DeepEqual(page.Items[1], old) {
			t.Errorf("List(Heat) = %+v, %v; want TS then %+v", page, err, old)
		}
		page, err = repo.List(ctx, core.NilID, 1, 1)
		if err != nil || page.Total != 3 || len(page.Items) != 1 || page.Items[0].ID != recent.ID {
			t.Errorf("List(all, page 2) = %+v, %v; want TS", page, err)
		}
		if _, err := repo.List(ctx, core.NilID, 0, -1); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("List with a negative offset: %v, want ErrInvalid", err)
		}
		if err := repo.Add(ctx, &core.BlocklistEntry{ID: core.NewID(), ItemID: heat.ID, Date: base, Protocol: core.ProtocolUsenet}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Add without a title: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, old.ID, recent.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.NilID, 0, 0); page.Total != 1 {
			t.Errorf("entries after Delete = %d, want = 1", page.Total)
		}
		// Entries go with their item.
		if err := s.Items().Delete(ctx, alien.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.NilID, 0, 0); page.Total != 0 {
			t.Errorf("entries after deleting the item = %d, want = 0", page.Total)
		}
	})
}

func TestPendingReleases(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		repo := s.PendingReleases()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		later := core.PendingRelease{
			ItemID: heat.ID, ItemIDs: []core.ID{heat.ID}, Title: "Heat.1995.2160p", Added: base.Add(time.Hour),
			Reason: core.PendingDelay, Release: json.RawMessage(`{"title":"Heat.1995.2160p","size":42}`),
		}
		first := core.PendingRelease{ID: core.NewID(), ItemID: heat.ID, Title: "Heat.1995.1080p", Added: base, Reason: core.PendingFallback, Release: json.RawMessage(`{}`)}
		other := core.PendingRelease{ID: core.NewID(), ItemID: alien.ID, Added: base.Add(time.Minute), Reason: core.PendingClientUnavailable, Release: json.RawMessage(`[]`)}
		for _, p := range []*core.PendingRelease{&later, &first, &other} {
			if err := repo.Add(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		list, err := repo.List(ctx, heat.ID)
		if err != nil || len(list) != 2 || list[0].ID != first.ID || !reflect.DeepEqual(list[1], later) {
			t.Errorf("List(Heat) = %+v, %v; want 1080p then %+v", list, err, later)
		}
		list, err = repo.List(ctx, core.NilID)
		if err != nil || len(list) != 3 || list[1].ID != other.ID {
			t.Errorf("List(all) = %+v, %v; want 1080p, Alien, 2160p", list, err)
		}
		if err := repo.Add(ctx, &core.PendingRelease{ID: core.NewID(), ItemID: heat.ID, Added: base, Reason: core.PendingDelay, Release: json.RawMessage(`{`)}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Add of invalid JSON: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, first.ID); err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(ctx, heat.ID); len(list) != 1 {
			t.Errorf("releases after Delete = %d, want = 1", len(list))
		}
		// Pending releases go with their item.
		if err := s.Items().Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(ctx, core.NilID); len(list) != 1 || list[0].ID != other.ID {
			t.Errorf("releases after deleting Heat = %+v, want Alien's", list)
		}
	})
}

func TestRemotePathMappings(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.RemotePathMappings()
		b := core.RemotePathMapping{Host: "seedbox", RemotePath: "/downloads/", LocalPath: "/mnt/seedbox/"}
		a := core.RemotePathMapping{ID: core.NewID(), Host: "nas", RemotePath: `D:\Downloads\`, LocalPath: "/mnt/nas/"}
		for _, m := range []*core.RemotePathMapping{&b, &a} {
			if err := repo.Put(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
		list, err := repo.List(ctx)
		if err != nil || !reflect.DeepEqual(list, []core.RemotePathMapping{a, b}) {
			t.Errorf("List = %+v, %v; want = nas, seedbox", list, err)
		}
		dup := core.RemotePathMapping{ID: core.NewID(), Host: "seedbox", RemotePath: "/downloads/", LocalPath: "/elsewhere/"}
		if err := repo.Put(ctx, &dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a mapped remote path: %v, want ErrConflict", err)
		}
		b.LocalPath = "/srv/seedbox/"
		if err := repo.Put(ctx, &b); err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(ctx); list[1].LocalPath != "/srv/seedbox/" {
			t.Errorf("LocalPath after update = %q, want = /srv/seedbox/", list[1].LocalPath)
		}
		if err := repo.Put(ctx, &core.RemotePathMapping{ID: core.NewID(), Host: "nas", RemotePath: "/x", LocalPath: "relative"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put of a relative local path: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, a.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
	})
}

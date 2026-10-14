package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func subtitleProfile(name string) core.SubtitleProfile {
	return core.SubtitleProfile{
		ID: core.NewID(), Name: name, Cutoff: 2,
		Items: []core.SubtitleProfileItem{
			{ID: 1, Language: "en", HI: core.SubtitleHIExcluded},
			{ID: 2, Language: "zh-Hans", Forced: true, AudioExclude: true},
		},
		MustContain: []string{"WEB"}, MustNotContain: []string{"HC"}, OriginalFormat: true, Tag: "anime",
	}
}

func TestSubtitleProfiles(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.SubtitleProfiles()
		zh, en := subtitleProfile("Chinese"), subtitleProfile("English")
		for _, p := range []*core.SubtitleProfile{&zh, &en} {
			if err := repo.Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.Get(ctx, zh.ID)
		if err != nil || !reflect.DeepEqual(got, zh) {
			t.Errorf("Get = %+v, %v; want = %+v", got, err, zh)
		}
		zh.Items = zh.Items[:1]
		zh.Cutoff = core.SubtitleCutoffAny
		if err := repo.Put(ctx, &zh); err != nil {
			t.Fatal(err)
		}
		list, err := repo.List(ctx)
		if err != nil || len(list) != 2 || !reflect.DeepEqual(list[0], zh) || list[1].ID != en.ID {
			t.Errorf("List = %+v, %v; want = Chinese, English", list, err)
		}
		dup := subtitleProfile("English")
		if err := repo.Put(ctx, &dup); !errors.Is(err, core.ErrConflict) {
			t.Errorf("Put of a duplicate name: %v, want ErrConflict", err)
		}
		bad := subtitleProfile("Bad")
		bad.Cutoff = 9
		if err := repo.Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put with a cutoff of no item: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, en.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, en.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get after Delete: %v, want ErrNotFound", err)
		}
	})
}

func TestSubtitleProfileAssignments(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		profile := subtitleProfile("English")
		if err := s.SubtitleProfiles().Put(ctx, &profile); err != nil {
			t.Fatal(err)
		}
		repo := s.SubtitleProfileAssignments()
		toLib := core.SubtitleProfileAssignment{TargetID: lib.ID, ProfileID: profile.ID}
		toHeat := core.SubtitleProfileAssignment{TargetID: heat.ID, ProfileID: profile.ID}
		none := core.SubtitleProfileAssignment{TargetID: alien.ID}
		for _, a := range []core.SubtitleProfileAssignment{toLib, toHeat, none} {
			if err := repo.Put(ctx, a); err != nil {
				t.Fatal(err)
			}
		}
		for _, want := range []core.SubtitleProfileAssignment{toLib, toHeat, none} {
			if got, err := repo.Get(ctx, want.TargetID); err != nil || got != want {
				t.Errorf("Get(%s) = %+v, %v; want = %+v", want.TargetID, got, err, want)
			}
		}
		// Put replaces the target's assignment.
		toHeat.ProfileID = core.NilID
		if err := repo.Put(ctx, toHeat); err != nil {
			t.Fatal(err)
		}
		list, err := repo.List(ctx, heat.ID, lib.ID)
		if err != nil || len(list) != 2 || !slicesContain(list, toHeat) || !slicesContain(list, toLib) {
			t.Errorf("List(Heat, library) = %+v, %v; want = %+v, %+v", list, err, toHeat, toLib)
		}
		for _, tc := range []struct {
			name string
			a    core.SubtitleProfileAssignment
			want error
		}{
			{"unknown target", core.SubtitleProfileAssignment{TargetID: core.NewID(), ProfileID: profile.ID}, core.ErrNotFound},
			{"unknown profile", core.SubtitleProfileAssignment{TargetID: heat.ID, ProfileID: core.NewID()}, core.ErrNotFound},
			{"no target", core.SubtitleProfileAssignment{ProfileID: profile.ID}, core.ErrInvalid},
		} {
			if err := repo.Put(ctx, tc.a); !errors.Is(err, tc.want) {
				t.Errorf("Put(%s): %v, want %v", tc.name, err, tc.want)
			}
		}
		if err := repo.Delete(ctx, alien.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, alien.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get after Delete: %v, want ErrNotFound", err)
		}
		// Assignments go with their profile and their target.
		if err := repo.Put(ctx, core.SubtitleProfileAssignment{TargetID: alien.ID, ProfileID: profile.ID}); err != nil {
			t.Fatal(err)
		}
		if err := s.Items().Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(ctx); len(list) != 2 {
			t.Errorf("assignments after deleting Heat = %+v, want the library's and Alien's", list)
		}
		if err := s.SubtitleProfiles().Delete(ctx, profile.ID); err != nil {
			t.Fatal(err)
		}
		if list, _ := repo.List(ctx); len(list) != 0 {
			t.Errorf("assignments after deleting the profile = %+v, want none", list)
		}
	})
}

func slicesContain(list []core.SubtitleProfileAssignment, a core.SubtitleProfileAssignment) bool {
	for _, x := range list {
		if x == a {
			return true
		}
	}
	return false
}

func TestSubtitleHistory(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		series, episodes := showTree(t, s, lib)
		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		repo := s.SubtitleHistory()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		download := core.SubtitleHistoryEvent{
			Action: core.SubtitleDownloaded, ItemID: movie.ID, Date: base,
			Language: core.SubtitleLanguage{Language: "en", HearingImpaired: true}, Provider: "subtitles/opensubtitlescom",
			SubtitleID: "123", Score: 150, MaxScore: 180, Path: "/media/Heat.en.hi.srt", VideoPath: "/media/Heat.mkv",
			Description: "English HI subtitles downloaded from opensubtitlescom with a score of 83.33%.",
			Matched:     []string{"title", "year"}, NotMatched: []string{"release_group"},
		}
		events := []core.SubtitleHistoryEvent{download}
		if err := repo.Add(ctx, events...); err != nil {
			t.Fatal(err)
		}
		download.ID = events[0].ID
		upgrade := core.SubtitleHistoryEvent{
			ID: core.NewID(), Action: core.SubtitleUpgraded, ItemID: movie.ID, Date: base.Add(time.Hour),
			Language: core.SubtitleLanguage{Language: "en"}, Score: 170, MaxScore: 180, UpgradedFrom: download.ID,
		}
		synced := core.SubtitleHistoryEvent{
			ID: core.NewID(), Action: core.SubtitleSynced, ItemID: episodes[0].ID, ParentID: series.ID, Date: base.Add(2 * time.Hour),
			Language: core.SubtitleLanguage{Language: "zh-Hant", Forced: true},
			Translation: &core.SubtitleTranslationRecord{
				SourcePath: "/media/Show/S01E01.en.srt", SourceLanguage: "en", Translators: []string{"ai/openai/gpt", "subtitles/deepl"},
				Cues: 400, Characters: 12000, InputTokens: 9000, OutputTokens: 7000, Cost: 0.25,
			},
		}
		if err := repo.Add(ctx, upgrade, synced); err != nil {
			t.Fatal(err)
		}
		ids := func(p core.Page[core.SubtitleHistoryEvent]) []core.ID {
			out := make([]core.ID, len(p.Items))
			for i, h := range p.Items {
				out[i] = h.ID
			}
			return out
		}
		for _, tc := range []struct {
			name  string
			q     core.SubtitleHistoryQuery
			total int
			want  []core.ID
		}{
			{"all", core.SubtitleHistoryQuery{}, 3, []core.ID{synced.ID, upgrade.ID, download.ID}},
			{"page", core.SubtitleHistoryQuery{Limit: 1, Offset: 1}, 3, []core.ID{upgrade.ID}},
			{"movie", core.SubtitleHistoryQuery{ItemIDs: []core.ID{movie.ID}}, 2, []core.ID{upgrade.ID, download.ID}},
			{"series", core.SubtitleHistoryQuery{ItemIDs: []core.ID{series.ID}}, 1, []core.ID{synced.ID}},
			{"actions", core.SubtitleHistoryQuery{Actions: []core.SubtitleAction{core.SubtitleDownloaded, core.SubtitleUpgraded}}, 2, []core.ID{upgrade.ID, download.ID}},
			{"since", core.SubtitleHistoryQuery{Since: base.Add(time.Hour)}, 2, []core.ID{synced.ID, upgrade.ID}},
		} {
			page, err := repo.List(ctx, tc.q)
			if err != nil || page.Total != tc.total || !reflect.DeepEqual(ids(page), tc.want) {
				t.Errorf("List(%s) = %v (total %d), %v; want = %v (total %d)", tc.name, ids(page), page.Total, err, tc.want, tc.total)
			}
		}
		page, err := repo.List(ctx, core.SubtitleHistoryQuery{})
		if err != nil || !reflect.DeepEqual(page.Items[2], download) || !reflect.DeepEqual(page.Items[1], upgrade) ||
			!reflect.DeepEqual(page.Items[0], synced) {
			t.Errorf("List = %+v, %v; want = %+v, %+v, %+v", page.Items, err, synced, upgrade, download)
		}
		proofread := *synced.Translation
		proofread.Proofread, proofread.Revised = true, 12
		if err := repo.SetTranslation(ctx, synced.ID, proofread); err != nil {
			t.Fatal(err)
		}
		if page, err := repo.List(ctx, core.SubtitleHistoryQuery{Limit: 1}); err != nil || !reflect.DeepEqual(page.Items[0].Translation, &proofread) {
			t.Errorf("translation after SetTranslation = %+v, %v; want = %+v", page.Items[0].Translation, err, proofread)
		}
		if err := repo.SetTranslation(ctx, core.NewID(), proofread); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("SetTranslation(unknown) = %v, want ErrNotFound", err)
		}
		for _, bad := range []core.SubtitleHistoryEvent{
			{ID: core.NewID(), Action: "bogus", ItemID: movie.ID, Date: base, Language: core.SubtitleLanguage{Language: "en"}},
			{ID: core.NewID(), Action: core.SubtitleDownloaded, ItemID: movie.ID, Date: base, Language: core.SubtitleLanguage{Language: "en"}, Score: 200, MaxScore: 180},
			{ID: core.NewID(), Action: core.SubtitleDownloaded, ItemID: movie.ID, Date: base, Language: core.SubtitleLanguage{Language: "en us"}},
		} {
			if err := repo.Add(ctx, bad); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Add(%+v): %v, want ErrInvalid", bad, err)
			}
		}
		n, err := repo.Purge(ctx, base.Add(90*time.Minute))
		if err != nil || n != 2 {
			t.Errorf("Purge = %d, %v; want = 2", n, err)
		}
		// Events go with their item.
		if err := s.Items().Delete(ctx, series.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.SubtitleHistoryQuery{}); page.Total != 0 {
			t.Errorf("events after deleting the series = %d, want = 0", page.Total)
		}
	})
}

func TestSubtitleBlocklist(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		repo := s.SubtitleBlocklist()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		old := core.SubtitleBlocklistEntry{
			ItemID: heat.ID, Provider: "subtitles/subdl", SubtitleID: "a1",
			Language: core.SubtitleLanguage{Language: "pt-BR", HearingImpaired: true}, Date: base, Message: "undecodable",
		}
		recent := core.SubtitleBlocklistEntry{ID: core.NewID(), ItemID: heat.ID, Provider: "subtitles/subdl", SubtitleID: "a2", Language: core.SubtitleLanguage{Language: "en"}, Date: base.Add(time.Hour)}
		other := core.SubtitleBlocklistEntry{ID: core.NewID(), ItemID: alien.ID, Provider: "subtitles/subdl", SubtitleID: "b", Language: core.SubtitleLanguage{Language: "en"}, Date: base.Add(2 * time.Hour)}
		for _, b := range []*core.SubtitleBlocklistEntry{&old, &recent, &other} {
			if err := repo.Add(ctx, b); err != nil {
				t.Fatal(err)
			}
		}
		page, err := repo.List(ctx, heat.ID, 0, 0)
		if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Items[0].ID != recent.ID || !reflect.DeepEqual(page.Items[1], old) {
			t.Errorf("List(Heat) = %+v, %v; want = a2 then %+v", page, err, old)
		}
		page, err = repo.List(ctx, core.NilID, 1, 1)
		if err != nil || page.Total != 3 || len(page.Items) != 1 || page.Items[0].ID != recent.ID {
			t.Errorf("List(all, page 2) = %+v, %v; want = a2", page, err)
		}
		if err := repo.Add(ctx, &core.SubtitleBlocklistEntry{ID: core.NewID(), ItemID: heat.ID, Date: base, Language: core.SubtitleLanguage{Language: "en"}}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Add without a provider: %v, want ErrInvalid", err)
		}
		if err := repo.Delete(ctx, old.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.NilID, 0, 0); page.Total != 2 {
			t.Errorf("entries after Delete = %d, want = 2", page.Total)
		}
		// Entries go with their item.
		if err := s.Items().Delete(ctx, alien.ID); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.NilID, 0, 0); page.Total != 1 {
			t.Errorf("entries after deleting Alien = %d, want = 1", page.Total)
		}
		if err := repo.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		if page, _ := repo.List(ctx, core.NilID, 0, 0); page.Total != 0 {
			t.Errorf("entries after Clear = %d, want = 0", page.Total)
		}
	})
}

func TestSubtitleSearchAttempts(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/movies")
		heat := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, heat)
		repo := s.SubtitleSearchAttempts()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		en, enHI := core.SubtitleLanguage{Language: "en"}, core.SubtitleLanguage{Language: "en", HearingImpaired: true}
		for _, r := range []struct {
			lang core.SubtitleLanguage
			at   time.Time
		}{{en, base}, {enHI, base.Add(time.Hour)}, {en, base.Add(24 * time.Hour)}, {en, base.Add(48 * time.Hour)}} {
			if err := repo.Record(ctx, heat.ID, r.lang, r.at); err != nil {
				t.Fatal(err)
			}
		}
		want := []core.SubtitleSearchAttempt{
			{ItemID: heat.ID, Language: en, First: base, Last: base.Add(48 * time.Hour)},
			{ItemID: heat.ID, Language: enHI, First: base.Add(time.Hour), Last: base.Add(time.Hour)},
		}
		if got, err := repo.List(ctx, heat.ID); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("List = %+v, %v; want = %+v", got, err, want)
		}
		if err := repo.Record(ctx, heat.ID, core.SubtitleLanguage{}, base); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Record without a language: %v, want ErrInvalid", err)
		}
		// Attempts go with their item.
		if err := s.Items().Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.List(ctx, heat.ID); len(got) != 0 {
			t.Errorf("attempts after deleting the item = %+v, want none", got)
		}
	})
}

func TestSubtitleWanted(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := newLibrary(t, s, "/media")
		series, episodes := showTree(t, s, lib)
		heat, alien := newItem(lib, core.KindMovie, "Heat"), newItem(lib, core.KindMovie, "Alien")
		upsert(t, s, heat, alien)
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		en, fr := core.SubtitleLanguage{Language: "en"}, core.SubtitleLanguage{Language: "fr", Forced: true}
		repo := s.SubtitleWanted()
		wanted := []core.SubtitleWanted{
			{ItemID: heat.ID, Kind: core.KindMovie, LibraryID: lib.ID, Path: "/media/Heat.mkv", Missing: []core.SubtitleLanguage{en}, Updated: at},
			{ItemID: alien.ID, Kind: core.KindMovie, LibraryID: lib.ID, Path: "/media/Alien.mkv", Missing: []core.SubtitleLanguage{en, fr}, Updated: at},
			{ItemID: episodes[0].ID, Kind: core.KindEpisode, LibraryID: lib.ID, ParentID: series.ID, Path: "/media/e.mkv", Missing: []core.SubtitleLanguage{fr}, Updated: at},
		}
		for _, w := range wanted {
			if err := repo.Put(ctx, w); err != nil {
				t.Fatal(err)
			}
		}
		list := func(kind core.ItemKind) []core.SubtitleWanted {
			t.Helper()
			page, err := repo.List(ctx, core.SubtitleWantedQuery{Kind: kind})
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != len(page.Items) {
				t.Errorf("List(%s).Total = %d, want = %d", kind, page.Total, len(page.Items))
			}
			return page.Items
		}
		if got, want := list(core.KindMovie), []core.SubtitleWanted{wanted[1], wanted[0]}; !reflect.DeepEqual(got, want) {
			t.Errorf("List(movies) = %+v, want = %+v", got, want)
		}
		if got := list(core.KindEpisode); !reflect.DeepEqual(got, wanted[2:]) {
			t.Errorf("List(episodes) = %+v, want = %+v", got, wanted[2:])
		}

		// Put replaces an entry, and deletes it when nothing is missing.
		wanted[1].Missing, wanted[1].Updated = []core.SubtitleLanguage{fr}, at.Add(time.Hour)
		wanted[0].Missing = nil
		for _, w := range wanted[:2] {
			if err := repo.Put(ctx, w); err != nil {
				t.Fatal(err)
			}
		}
		if got := list(core.KindMovie); !reflect.DeepEqual(got, wanted[1:2]) {
			t.Errorf("List after Put = %+v, want = %+v", got, wanted[1:2])
		}

		// Missing items are left out until found again; deleted ones
		// take their entries with them.
		alien.MissingSince = &at
		upsert(t, s, alien)
		if got := list(core.KindMovie); len(got) != 0 {
			t.Errorf("List with Alien missing = %+v, want none", got)
		}
		if err := s.Items().Delete(ctx, alien.ID, episodes[0].ID); err != nil {
			t.Fatal(err)
		}
		alien.MissingSince = nil
		upsert(t, s, alien)
		if got := list(core.KindMovie); len(got) != 0 {
			t.Errorf("List after deleting Alien = %+v, want none", got)
		}
		if got := list(core.KindEpisode); len(got) != 0 {
			t.Errorf("List after deleting the episode = %+v, want none", got)
		}
		bad := core.SubtitleWanted{ItemID: series.ID, Kind: core.KindSeries, LibraryID: lib.ID, Missing: []core.SubtitleLanguage{en}, Updated: at}
		if err := repo.Put(ctx, bad); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put of a series: %v, want ErrInvalid", err)
		}
	})
}

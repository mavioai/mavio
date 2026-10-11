package library

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

var missingToday = time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

func TestMissingEpisodeProviderCases(t *testing.T) {
	const premiereReason = "listed air dates are calendar dates, which the host reads as UTC midnight " +
		"(apps/server/internal/providers), not local dates converted to UTC"
	portedCases(t, "tmdb_missing_episode_provider.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ShouldImportEpisode_RespectsAirDateAndOptions": func(t *testing.T, a args) {
				var premiere *time.Time
				if !a.null("dayOffset") {
					var days int
					if err := json.Unmarshal(a["dayOffset"], &days); err != nil {
						t.Fatal(err)
					}
					d := missingToday.AddDate(0, 0, days)
					premiere = &d
				}
				got := shouldImportEpisode(premiere, missingToday, a.boolean(t, "importUnaired"), a.boolean(t, "importMissing"),
					a.boolean(t, "isSpecial"), a.boolean(t, "importSpecials"))
				if want := a.boolean(t, "expected"); got != want {
					t.Errorf("shouldImportEpisode() = %v, want = %v", got, want)
				}
			},
		},
		skip: map[string]string{
			"GetPremiereDate_NullAirDate_ReturnsNull": premiereReason,
			"GetPremiereDate_AirDate_ReturnsUtc":      premiereReason,
		},
		facts: map[string]string{
			"ShouldPrune_AgedOutVirtualTmdbEpisode_ReturnsTrue":                   "TestShouldPrune",
			"ShouldPrune_NotInPruningMode_ReturnsFalse":                           "TestShouldPrune",
			"ShouldPrune_StillUpcoming_ReturnsFalse":                              "TestShouldPrune",
			"ShouldPrune_VirtualEpisodeFromAnotherProvider_ReturnsFalse":          "TestShouldPrune",
			"ShouldPrune_PhysicalEpisode_ReturnsFalse":                            "TestShouldPrune",
			"ShouldPrune_AiredWithinGracePeriod_ReturnsFalse":                     "TestShouldPrune",
			"ShouldPrune_AiredBeyondGracePeriod_ReturnsTrue":                      "TestShouldPrune",
			"ShouldPrune_SpecialWithSpecialsDisabled_ReturnsTrue":                 "TestShouldPrune",
			"ShouldPrune_SpecialWithSpecialsEnabled_FollowsNormalRules":           "TestShouldPrune",
			"UpdateVirtualEpisode_PlaceholderTitleReplaced_UpdatesAndReturnsTrue": "TestUpdateVirtualEpisode",
			"UpdateVirtualEpisode_NoChanges_ReturnsFalse":                         "TestUpdateVirtualEpisode",
			"UpdateVirtualEpisode_EmptyTmdbValues_DoNotOverwrite":                 "TestUpdateVirtualEpisode",
			"UpdateVirtualEpisode_RescheduledAirDate_UpdatesPremiereAndYear":      "TestUpdateVirtualEpisode",
			"BuildSeasonSortNameTemplate_NoNameSortedPhysicalSeason_ReturnsNull":  "TestSeasonSortNameTemplate",
			"BuildSeasonSortNameTemplate_MirrorsSiblingConventionAndSwapsNumber":  "TestSeasonSortNameTemplate",
			"BuildSeasonSortNameTemplate_PreservesNonEnglishToken":                "TestSeasonSortNameTemplate",
			"BuildSeasonSortNameTemplate_SiblingWithoutDigits_ReturnsNull":        "TestSeasonSortNameTemplate",
			"BuildSeasonSortNameTemplate_IgnoresVirtualSeasonsAsReference":        "TestSeasonSortNameTemplate",
		},
	})
}

func virtualEpisodeAt(premiere time.Time, withID bool, season *int) core.Item {
	it := core.Item{Kind: core.KindEpisode, PremiereDate: &premiere, Virtual: true, ParentIndexNumber: season}
	if withID {
		it.ExternalIDs = map[core.Provider]string{core.ProviderTMDB: "123"}
	}
	return it
}

func TestShouldPrune(t *testing.T) {
	physical := core.Item{
		Kind: core.KindEpisode, Path: "/media/show/Season 01/s01e01.mkv",
		PremiereDate: new(missingToday.AddDate(0, 0, -1)), ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "123"},
	}
	tests := []struct {
		name         string
		ep           core.Item
		pruneAgedOut bool
		graceDays    int
		specials     bool
		want         bool
	}{
		{"aged out", virtualEpisodeAt(missingToday.AddDate(0, 0, -1), true, nil), true, 0, true, true},
		{"not pruning", virtualEpisodeAt(missingToday.AddDate(0, 0, -1), true, nil), false, 0, true, false},
		{"still upcoming", virtualEpisodeAt(missingToday.AddDate(0, 0, 1), true, nil), true, 0, true, false},
		{"another provider's", virtualEpisodeAt(missingToday.AddDate(0, 0, -1), false, nil), true, 0, true, false},
		{"physical", physical, true, 0, true, false},
		{"within grace period", virtualEpisodeAt(missingToday.AddDate(0, 0, -2), true, nil), true, 7, true, false},
		{"beyond grace period", virtualEpisodeAt(missingToday.AddDate(0, 0, -10), true, nil), true, 7, true, true},
		{"special without specials", virtualEpisodeAt(missingToday.AddDate(0, 0, 5), true, new(0)), false, 7, false, true},
		{"special with specials", virtualEpisodeAt(missingToday.AddDate(0, 0, 5), true, new(0)), true, 7, true, false},
	}
	for _, tt := range tests {
		if got := shouldPrune(tt.ep, core.ProviderTMDB, tt.pruneAgedOut, missingToday, tt.graceDays, tt.specials); got != tt.want {
			t.Errorf("%s: shouldPrune() = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

func TestUpdateVirtualEpisode(t *testing.T) {
	t.Run("placeholder title replaced", func(t *testing.T) {
		it := core.Item{Name: "Episode 14"}
		if !updateVirtualEpisode(&it, core.Item{Name: "The Real Title"}) || it.Name != "The Real Title" {
			t.Errorf("got = %q, want = The Real Title, changed", it.Name)
		}
	})
	t.Run("no changes", func(t *testing.T) {
		it := core.Item{Name: "Same", Overview: "Description", PremiereDate: &missingToday}
		if updateVirtualEpisode(&it, core.Item{Name: "Same", Overview: "Description", PremiereDate: new(missingToday)}) {
			t.Error("changed = true, want = false")
		}
	})
	t.Run("empty values do not overwrite", func(t *testing.T) {
		it := core.Item{Name: "Existing", Overview: "Existing overview"}
		if updateVirtualEpisode(&it, core.Item{}) || it.Name != "Existing" || it.Overview != "Existing overview" {
			t.Errorf("got = %+v, want = unchanged", it)
		}
	})
	t.Run("rescheduled air date", func(t *testing.T) {
		it := core.Item{Name: "X", PremiereDate: new(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))}
		aired := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
		if !updateVirtualEpisode(&it, core.Item{Name: "X", PremiereDate: &aired}) || !it.PremiereDate.Equal(aired) || it.ProductionYear != 2026 {
			t.Errorf("got = %v, %d, want = %v, 2026", it.PremiereDate, it.ProductionYear, aired)
		}
	})
}

func TestSeasonSortNameTemplate(t *testing.T) {
	season := func(n int, sortName string, virtual bool) core.Item {
		return core.Item{Kind: core.KindSeason, Name: seasonName(n), IndexNumber: &n, SortName: sortName, Virtual: virtual}
	}
	if seasonSortNameTemplate([]core.Item{season(1, "", false), season(3, "", true)}) != nil {
		t.Error("no name-sorted physical season: template, want = nil")
	}
	if tpl := seasonSortNameTemplate([]core.Item{season(1, "Season 01", false), season(3, "", true)}); tpl == nil || tpl(3) != "Season 03" || tpl(12) != "Season 12" {
		t.Error("sibling convention: want = Season 03 and Season 12")
	}
	if tpl := seasonSortNameTemplate([]core.Item{season(1, "Staffel 1", false), season(2, "", true)}); tpl == nil || tpl(2) != "Staffel 2" {
		t.Error("non-English token: want = Staffel 2")
	}
	if seasonSortNameTemplate([]core.Item{season(1, "Miniseries", false), season(2, "", true)}) != nil {
		t.Error("sibling without digits: template, want = nil")
	}
	if seasonSortNameTemplate([]core.Item{season(1, "", false), season(3, "Season 03", true)}) != nil {
		t.Error("virtual reference: template, want = nil")
	}
}

// fakeLister lists a fixed set of episodes and records lookups.
type fakeLister struct {
	list    EpisodeList
	lookups []Lookup
}

func (l *fakeLister) Name() string { return "lister" }

func (l *fakeLister) Episodes(_ context.Context, lookup Lookup) (*EpisodeList, error) {
	l.lookups = append(l.lookups, lookup)
	return &l.list, nil
}

func listed(season, episode int, absolute int, aired *time.Time, name string) metadata.Result {
	it := core.Item{
		Name: name, ParentIndexNumber: &season, IndexNumber: &episode, AbsoluteNumber: &absolute, PremiereDate: aired,
		ExternalIDs: map[core.Provider]string{core.ProviderTMDB: name},
	}
	return metadata.Result{Item: it, RemoteImages: []metadata.RemoteImage{{Kind: core.ImagePrimary, URL: "https://img/" + name + ".jpg"}}}
}

// virtuals returns the virtual items under a series as "kind name".
func (f *scanFixture) virtuals(series core.ID) []string {
	f.t.Helper()
	var out []string
	for it := range f.store.Items().Walk(f.t.Context(), core.ItemQuery{ParentID: series, Recursive: true, OnlyVirtual: true}) {
		out = append(out, string(it.Kind)+" "+it.Name)
	}
	slices.Sort(out)
	return out
}

func TestListEpisodes(t *testing.T) {
	f := newScan(t, core.LibraryShows)
	tree(t, f.root, "Show/Season 01/Show S01E01.mkv")
	f.scan()
	series := f.item("Show")
	series.ExternalIDs = map[core.Provider]string{core.ProviderTMDB: "1"}
	series.DisplayOrder = core.OrderDVD
	if err := f.store.Items().Upsert(t.Context(), series); err != nil {
		t.Fatal(err)
	}
	aired := func(days int) *time.Time { d := f.clock.AddDate(0, 0, days); return &d }
	lister := &fakeLister{list: EpisodeList{
		Provider: core.ProviderTMDB,
		Seasons:  []metadata.Result{{Item: core.Item{Name: "The Second", IndexNumber: new(2)}}},
		Episodes: []metadata.Result{
			listed(1, 1, 1, aired(-30), "Pilot"),
			listed(1, 2, 2, aired(-23), "Second"),
			listed(1, 3, 3, nil, "Undated"),
			listed(2, 1, 4, aired(30), "Premiere"),
			listed(0, 1, 0, aired(-60), "Special"),
		},
	}}
	r := &Refresher{Store: f.store, Episodes: func() []EpisodeLister { return []EpisodeLister{lister} }, Now: func() time.Time { return f.clock }}
	refresh := func() {
		t.Helper()
		if err := r.Refresh(t.Context(), f.lib, series.ID); err != nil {
			t.Fatal(err)
		}
	}

	// Without options, nothing is listed.
	refresh()
	if got := f.virtuals(series.ID); len(got) != 0 {
		t.Errorf("without options = %v, want = none", got)
	}

	f.lib.VirtualEpisodes = core.VirtualEpisodes{Missing: true, Unaired: true}
	refresh()
	if got, want := f.virtuals(series.ID), []string{"episode Premiere", "episode Second", "season The Second"}; !slices.Equal(got, want) {
		t.Errorf("listed = %v, want = %v", got, want)
	}
	if l := lister.lookups[len(lister.lookups)-1]; l.DisplayOrder != core.OrderDVD || l.ExternalIDs[core.ProviderTMDB] != "1" {
		t.Errorf("lookup = %+v, want the series' display order and IDs", l)
	}
	pilot := f.item("Show/Season 01/Show S01E01.mkv")
	if pilot.AbsoluteNumber == nil || *pilot.AbsoluteNumber != 1 {
		t.Errorf("pilot absolute number = %v, want = 1", pilot.AbsoluteNumber)
	}
	var second core.Item
	for it := range f.store.Items().Walk(t.Context(), core.ItemQuery{ParentID: series.ID, Recursive: true, Kinds: []core.ItemKind{core.KindEpisode}, OnlyVirtual: true, IndexNumber: new(2)}) {
		second = it
	}
	if second.ParentID != f.item("Show/Season 01").ID || *second.AbsoluteNumber != 2 || second.Monitored {
		t.Errorf("second = %+v, want an unmonitored episode of season 1", second)
	}
	if images, _ := f.store.Images().ListForOwner(t.Context(), second.ID); len(images) != 1 || images[0].RemoteURL != "https://img/Second.jpg" {
		t.Errorf("second's images = %+v, want its still", images)
	}

	// Listing again changes nothing; a renamed episode is updated.
	lister.list.Episodes[1].Item.Name = "Second Renamed"
	refresh()
	if got, want := f.virtuals(series.ID), []string{"episode Premiere", "episode Second Renamed", "season The Second"}; !slices.Equal(got, want) {
		t.Errorf("relisted = %v, want = %v", got, want)
	}

	// A file for a virtual episode takes it over, and a season folder the
	// virtual season.
	f.clock = f.clock.Add(time.Hour)
	tree(t, f.root, "Show/Season 01/Show S01E02.mkv", "Show/Season 02/Show S02E01.mkv")
	f.scan()
	if got, want := f.virtuals(series.ID), []string(nil); !slices.Equal(got, want) {
		t.Errorf("after the files came = %v, want = none", got)
	}
	if got := f.item("Show/Season 01/Show S01E02.mkv"); got.ID != second.ID || got.Name != "Second Renamed" {
		t.Errorf("adopted = %+v, want the virtual episode %s", got, second.ID)
	}
	if got := f.item("Show/Season 02"); got.Name != "The Second" || got.Virtual {
		t.Errorf("season 2 = %+v, want the adopted virtual season", got)
	}

	// Once the files go and the options are off, nothing virtual is kept;
	// a monitored series lists every episode, monitored but its specials.
	if err := f.store.Items().Delete(t.Context(), f.item("Show/Season 02").ID); err != nil {
		t.Fatal(err)
	}
	refresh()
	if got, want := f.virtuals(series.ID), []string{"episode Premiere", "season The Second"}; !slices.Equal(got, want) {
		t.Errorf("after season 2 went = %v, want = %v", got, want)
	}
	f.lib.VirtualEpisodes = core.VirtualEpisodes{}
	refresh()
	if got := f.virtuals(series.ID); len(got) != 0 {
		t.Errorf("options off = %v, want = none", got)
	}
	series, _ = f.store.Items().Get(t.Context(), series.ID)
	series.Monitored = true
	if err := f.store.Items().Upsert(t.Context(), series); err != nil {
		t.Fatal(err)
	}
	refresh()
	var monitored []string
	for it := range f.store.Items().Walk(t.Context(), core.ItemQuery{ParentID: series.ID, Recursive: true, OnlyVirtual: true, Monitored: new(true)}) {
		monitored = append(monitored, it.Name)
	}
	slices.Sort(monitored)
	if want := []string{"Premiere", "The Second"}; !slices.Equal(monitored, want) {
		t.Errorf("monitored series = %v, want = %v", monitored, want)
	}
}

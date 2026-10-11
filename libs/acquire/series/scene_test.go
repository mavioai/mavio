package series

import (
	"errors"
	"testing"
)

// The tests here port Sonarr's SceneMappingServiceFixture
// (NzbDrone.Core.Test/DataAugmentation/Scene/SceneMappingServiceFixture.cs);
// its tests of updating the mappings from the providers are those of the
// server's scene mapping cache.

func TestSceneMappingsBlank(t *testing.T) {
	// should_not_add_mapping_with_blank_title,
	// should_not_add_mapping_with_blank_search_title
	m := NewSceneMappings([]SceneMapping{
		{Title: "Title", SearchTerm: "Title", TvdbID: 1},
		{Title: "", SearchTerm: "Title", TvdbID: 2},
		{Title: "Title", SearchTerm: " ", TvdbID: 3},
	})
	if got := m.Len(); got != 1 {
		t.Errorf("Len() = %d, want = 1", got)
	}
}

func TestSceneMappingsSingleMatch(t *testing.T) {
	// should_return_single_match
	m := NewSceneMappings([]SceneMapping{
		{Title: "Working!!", ParseTerm: "working", SearchTerm: "Working!!", TvdbID: 100, SceneSeasonNumber: intPtr(1)},
		{Title: "Working`!!", ParseTerm: "working", SearchTerm: "Working`!!", TvdbID: 100, SceneSeasonNumber: intPtr(2)},
		{Title: "Working!!!", ParseTerm: "working", SearchTerm: "Working!!!", TvdbID: 100, SceneSeasonNumber: intPtr(3)},
	})
	tests := []struct {
		parse      string
		wantSeason int
	}{
		{"Working!!", 1},
		{"Working`!!", 2},
		{"Working!!!", 3},
		{"Working!!!!", 3},
		{"Working !!", 1},
	}
	for _, tt := range tests {
		id, ok, err := m.FindTvdbID(tt.parse, "", -1)
		if err != nil || !ok || id != 100 {
			t.Errorf("FindTvdbID(%q) = %d, %v, %v, want = 100", tt.parse, id, ok, err)
		}
		season, ok, err := m.GetSceneSeasonNumber(tt.parse, "")
		if err != nil || !ok || season != tt.wantSeason {
			t.Errorf("GetSceneSeasonNumber(%q) = %d, %v, %v, want = %d", tt.parse, season, ok, err, tt.wantSeason)
		}
	}
}

func TestSceneMappingsGetSceneNames(t *testing.T) {
	fudanshi := func(season, sceneSeason *int) *SceneMappings {
		return NewSceneMappings([]SceneMapping{{
			Title: "Fudanshi Koukou Seikatsu", SearchTerm: "Fudanshi Koukou Seikatsu", TvdbID: 100,
			SeasonNumber: season, SceneSeasonNumber: sceneSeason,
		}})
	}
	global := NewSceneMappings([]SceneMapping{
		{Title: "Fudanshi Koukou Seikatsu 1", SearchTerm: "Fudanshi Koukou Seikatsu 1", TvdbID: 100},
		{Title: "Fudanshi Koukou Seikatsu 2", SearchTerm: "Fudanshi Koukou Seikatsu 2", TvdbID: 100, SeasonNumber: intPtr(-1)},
		{Title: "Fudanshi Koukou Seikatsu 3", SearchTerm: "Fudanshi Koukou Seikatsu 3", TvdbID: 100, SceneSeasonNumber: intPtr(-1)},
		{Title: "Fudanshi Koukou Seikatsu 4", SearchTerm: "Fudanshi Koukou Seikatsu 4", TvdbID: 100, SeasonNumber: intPtr(-1), SceneSeasonNumber: intPtr(-1)},
	})
	fairyTail := NewSceneMappings([]SceneMapping{{Title: "Fairy Tail S2", SearchTerm: "Fairy Tail S2", TvdbID: 100, SceneSeasonNumber: intPtr(2)}})
	sceneSeason1 := fudanshi(nil, intPtr(1))
	tests := []struct {
		name                 string
		m                    *SceneMappings
		seasons, sceneSeason int
		want                 int
	}{
		{"should_return_alternate_title_for_global_season", global, 10, 10, 4},
		{"should_return_alternate_title_for_season", fudanshi(intPtr(1), nil), 1, 10, 1},
		{"should_not_return_alternate_title_for_season", fudanshi(intPtr(1), nil), 2, 10, 0},
		{"should_return_alternate_title_for_sceneseason", sceneSeason1, 10, 1, 1},
		{"should_not_return_alternate_title_for_sceneseason", sceneSeason1, 10, 2, 0},
		{"should_return_alternate_title_for_fairy_tail(4,20)", fairyTail, 4, 20, 0},
		{"should_return_alternate_title_for_fairy_tail(7,20)", fairyTail, 7, 20, 0},
		{"should_return_alternate_title_for_fairy_tail(20,1)", fairyTail, 20, 1, 0},
		{"should_return_alternate_title_for_fairy_tail(20,2)", fairyTail, 20, 2, 1},
		{"should_return_alternate_title_for_fairy_tail(20,3)", fairyTail, 20, 3, 0},
		{"should_return_alternate_title_for_fudanshi(1,20)", sceneSeason1, 1, 20, 0},
		{"should_return_alternate_title_for_fudanshi(1,1)", sceneSeason1, 1, 1, 1},
		{"should_return_alternate_title_for_fudanshi(2,2)", sceneSeason1, 2, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.GetSceneNames(100, []int{tt.seasons}, []int{tt.sceneSeason}); len(got) != tt.want {
				t.Errorf("GetSceneNames = %q, want = %d names", got, tt.want)
			}
		})
	}
}

func TestSceneMappingsFind(t *testing.T) {
	amareto := func(id int, rest ...func(*SceneMapping)) SceneMapping {
		sm := SceneMapping{Title: "Amareto", ParseTerm: "amareto", SearchTerm: "Amareto", TvdbID: id}
		for _, f := range rest {
			f(&sm)
		}
		return sm
	}
	seasons := func(scene, season int) func(*SceneMapping) {
		return func(sm *SceneMapping) { sm.SceneSeasonNumber, sm.SeasonNumber = intPtr(scene), intPtr(season) }
	}
	tests := []struct {
		name        string
		mappings    []SceneMapping
		release     string
		sceneSeason int
		want        int
		wantErr     error
	}{
		{
			"should_filter_by_regex(Viva)",
			[]SceneMapping{amareto(100), amareto(101, func(sm *SceneMapping) { sm.FilterRegex = "-Viva$" })},
			"Amareto.S01E01.720p.WEB-DL-Viva", -1, 101, nil,
		},
		{
			"should_filter_by_regex(DMO)",
			[]SceneMapping{amareto(100), amareto(101, func(sm *SceneMapping) { sm.FilterRegex = "-Viva$" })},
			"Amareto.S01E01.720p.WEB-DL-DMO", -1, 100, nil,
		},
		{"should_throw_if_multiple_mappings", []SceneMapping{amareto(100), amareto(101)}, "Amareto.S01E01.720p.WEB-DL-Viva", -1, 0, ErrInvalidSceneMapping},
		{"should_not_throw_if_multiple_mappings_with_same_tvdbid", []SceneMapping{amareto(100), amareto(100)}, "Amareto.S01E01.720p.WEB-DL-Viva", -1, 100, nil},
		{"should_pick_best_season", []SceneMapping{amareto(100, seasons(2, 3)), amareto(101, seasons(3, 3))}, "Amareto.S01E01.720p.WEB-DL-Viva", 4, 101, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := NewSceneMappings(tt.mappings).FindTvdbID("Amareto", tt.release, tt.sceneSeason)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Errorf("FindTvdbID = %d, %v, want = %d, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

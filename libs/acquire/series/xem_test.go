package series

import (
	"slices"
	"testing"
)

// Ported from Sonarr's XemServiceFixture.

func xemEpisodes() [][2]int {
	return [][2]int{{1, 1}, {1, 2}, {2, 1}, {2, 2}, {2, 3}, {2, 4}, {2, 5}, {3, 1}, {3, 2}}
}

func xemMap(sceneAbsolute, sceneSeason, sceneEpisode, tvdbAbsolute, tvdbSeason, tvdbEpisode int) XemMapping {
	return XemMapping{
		Scene: XemNumber{Season: sceneSeason, Episode: sceneEpisode, Absolute: sceneAbsolute},
		Tvdb:  XemNumber{Season: tvdbSeason, Episode: tvdbEpisode, Absolute: tvdbAbsolute},
	}
}

// xemMappings is GivenTvdbMappings.
func xemMappings() []XemMapping {
	return []XemMapping{
		xemMap(1, 1, 1, 1, 1, 1), // 1x01 -> 1x01
		xemMap(2, 1, 2, 2, 1, 2), // 1x02 -> 1x02
		xemMap(3, 2, 1, 3, 2, 1), // 2x01 -> 2x01
		xemMap(4, 2, 2, 4, 2, 2), // 2x02 -> 2x02
		xemMap(5, 2, 3, 5, 2, 3), // 2x03 -> 2x03
		xemMap(6, 3, 1, 6, 2, 4), // 3x01 -> 2x04
		xemMap(7, 3, 2, 7, 2, 5), // 3x02 -> 2x05
	}
}

func xemEpisode(t *testing.T, got []XemNumbering, season, episode int) XemNumbering {
	t.Helper()
	i := slices.IndexFunc(got, func(e XemNumbering) bool { return e.Season == season && e.Episode == episode })
	if i < 0 {
		t.Fatalf("no episode %dx%d", season, episode)
	}
	return got[i]
}

func intOrNil(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestNumberScenes(t *testing.T) {
	without := func(drop func(XemMapping) bool) []XemMapping {
		return slices.DeleteFunc(xemMappings(), drop)
	}
	tests := []struct {
		name             string
		mappings         []XemMapping
		season, episode  int
		unverified       bool
		sceneSeason      any
		sceneEpisode     any
		checkSceneNumber bool
	}{
		{
			name:     "should_flag_unknown_future_episodes_if_existing_season_is_mapped",
			mappings: without(func(m XemMapping) bool { return m.Tvdb.Season == 2 && m.Tvdb.Episode == 5 }),
			season:   2, episode: 5, unverified: true,
		},
		{
			name:     "should_flag_unknown_future_season_if_future_season_is_shifted",
			mappings: xemMappings(), season: 3, episode: 1, unverified: true,
		},
		{
			name:     "should_not_flag_unknown_future_season_if_future_season_is_not_shifted",
			mappings: without(func(m XemMapping) bool { return m.Scene.Season == 3 }),
			season:   3, episode: 1,
		},
		{
			name:     "should_not_flag_past_episodes_if_not_causing_overlaps",
			mappings: without(func(m XemMapping) bool { return m.Scene.Season == 2 }),
			season:   2, episode: 1,
		},
		{
			name: "should_flag_past_episodes_if_causing_overlap",
			mappings: func() []XemMapping {
				ms := without(func(m XemMapping) bool { return m.Scene.Season == 2 && m.Tvdb.Episode <= 1 })
				i := slices.IndexFunc(ms, func(m XemMapping) bool { return m.Scene.Season == 2 && m.Scene.Episode == 2 })
				ms[i].Scene.Episode = 1
				return ms
			}(),
			season: 2, episode: 1, unverified: true,
		},
		{
			name: "should_not_extrapolate_season_with_specials",
			mappings: func() []XemMapping {
				ms := xemMappings()
				i := slices.IndexFunc(ms, func(m XemMapping) bool { return m.Tvdb.Season == 2 && m.Tvdb.Episode == 5 })
				ms[i].Tvdb.Season, ms[i].Tvdb.Episode = 0, 1
				return ms
			}(),
			season: 2, episode: 5, unverified: true, checkSceneNumber: true,
		},
		{
			name:     "should_extrapolate_season_with_future_episodes",
			mappings: without(func(m XemMapping) bool { return m.Tvdb.Season == 2 && m.Tvdb.Episode == 5 }),
			season:   2, episode: 5, unverified: true, sceneSeason: 3, sceneEpisode: 2, checkSceneNumber: true,
		},
		{
			name: "should_extrapolate_season_with_shifted_episodes",
			mappings: func() []XemMapping {
				ms := without(func(m XemMapping) bool { return m.Tvdb.Season == 2 && m.Tvdb.Episode == 5 })
				i := slices.IndexFunc(ms, func(m XemMapping) bool { return m.Tvdb.Season == 2 && m.Tvdb.Episode == 4 })
				ms[i].Scene.Season, ms[i].Scene.Episode = 2, 3
				return ms
			}(),
			season: 2, episode: 5, unverified: true, sceneSeason: 2, sceneEpisode: 4, checkSceneNumber: true,
		},
		{
			name:     "should_extrapolate_shifted_future_seasons",
			mappings: xemMappings(), season: 3, episode: 2, unverified: true, sceneSeason: 4, sceneEpisode: 2, checkSceneNumber: true,
		},
		{
			name:     "should_not_extrapolate_matching_future_seasons",
			mappings: without(func(m XemMapping) bool { return m.Scene.Season != 1 }),
			season:   3, episode: 2, checkSceneNumber: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := xemEpisode(t, NumberScenes(xemEpisodes(), tt.mappings), tt.season, tt.episode)
			if e.Unverified != tt.unverified {
				t.Errorf("%dx%d unverified = %t, want = %t", tt.season, tt.episode, e.Unverified, tt.unverified)
			}
			if !tt.checkSceneNumber {
				return
			}
			if got := intOrNil(e.SceneSeason); got != tt.sceneSeason {
				t.Errorf("%dx%d scene season = %v, want = %v", tt.season, tt.episode, got, tt.sceneSeason)
			}
			if got := intOrNil(e.SceneEpisode); got != tt.sceneEpisode {
				t.Errorf("%dx%d scene episode = %v, want = %v", tt.season, tt.episode, got, tt.sceneEpisode)
			}
		})
	}
}

func TestNumberScenesMapsEpisodes(t *testing.T) {
	got := NumberScenes(xemEpisodes(), xemMappings())
	e := xemEpisode(t, got, 2, 4)
	if intOrNil(e.SceneSeason) != 3 || intOrNil(e.SceneEpisode) != 1 || intOrNil(e.SceneAbsolute) != 6 || e.Unverified {
		t.Errorf("2x04 = %+v, want = scene 3x01 (6), verified", e)
	}
	// should_clear_scenenumbering_if_removed_from_thexem: without
	// mappings no episode is numbered apart from TVDB.
	for _, e := range NumberScenes(xemEpisodes(), nil) {
		if e.SceneSeason != nil || e.SceneEpisode != nil || e.SceneAbsolute != nil || e.Unverified {
			t.Errorf("%dx%d without mappings = %+v, want = as TVDB", e.Season, e.Episode, e)
		}
	}
}

// should_skip_mapping_when_scene_information_is_all_zero
func TestNumberScenesSkipsZeroMappings(t *testing.T) {
	ms := append(xemMappings(), xemMap(0, 0, 0, 8, 3, 1), xemMap(0, 0, 0, 9, 3, 2))
	for _, e := range NumberScenes(xemEpisodes(), ms) {
		if e.SceneSeason != nil && *e.SceneSeason == 0 && e.SceneEpisode != nil && *e.SceneEpisode == 0 {
			t.Errorf("%dx%d = scene 0x00, want = the mapping skipped", e.Season, e.Episode)
		}
	}
}

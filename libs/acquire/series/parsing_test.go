package series

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// fakeServices are the series and episode services of Sonarr's mocked
// ISeriesService and IEpisodeService: each call is recorded as its name
// and arguments, and returns what the test set, else nothing.
type fakeServices struct {
	calls []string

	byTitle, byTitleYear, byTvdb, byTvRage, byImdb *Series

	episode          func(season, number int) *Episode
	byAbsolute       *Episode
	byAirDate        *Episode
	bySceneNumbering func(season, number int) []*Episode
	bySceneAbsolute  []*Episode
	bySeason         []*Episode
	bySceneSeason    []*Episode
	byEpisodeTitle   func(season int, title string) *Episode
}

func (f *fakeServices) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

// count returns how many calls start with prefix.
func (f *fakeServices) count(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeServices) FindByTitle(_ context.Context, title string) (*Series, error) {
	f.record("FindByTitle %s", title)
	return f.byTitle, nil
}

func (f *fakeServices) FindByTitleYear(_ context.Context, title string, year int) (*Series, error) {
	f.record("FindByTitleYear %s %d", title, year)
	return f.byTitleYear, nil
}

func (f *fakeServices) FindByTitleInexact(_ context.Context, title string) (*Series, error) {
	f.record("FindByTitleInexact %s", title)
	return nil, nil
}

func (f *fakeServices) FindByTvdbID(_ context.Context, id int) (*Series, error) {
	f.record("FindByTvdbID %d", id)
	return f.byTvdb, nil
}

func (f *fakeServices) FindByTvRageID(_ context.Context, id int) (*Series, error) {
	f.record("FindByTvRageID %d", id)
	return f.byTvRage, nil
}

func (f *fakeServices) FindByImdbID(_ context.Context, id string) (*Series, error) {
	f.record("FindByImdbID %s", id)
	return f.byImdb, nil
}

func (f *fakeServices) FindEpisode(_ context.Context, _ core.ID, season, number int) (*Episode, error) {
	f.record("FindEpisode %d %d", season, number)
	if f.episode == nil {
		return nil, nil
	}
	return f.episode(season, number), nil
}

func (f *fakeServices) FindEpisodeByAbsolute(_ context.Context, _ core.ID, abs int) (*Episode, error) {
	f.record("FindEpisodeByAbsolute %d", abs)
	return f.byAbsolute, nil
}

func (f *fakeServices) FindEpisodeByAirDate(_ context.Context, _ core.ID, date string, part *int) (*Episode, error) {
	p := "nil"
	if part != nil {
		p = fmt.Sprint(*part)
	}
	f.record("FindEpisodeByAirDate %s %s", date, p)
	return f.byAirDate, nil
}

func (f *fakeServices) FindEpisodesBySceneNumbering(_ context.Context, _ core.ID, season, number int) ([]*Episode, error) {
	f.record("FindEpisodesBySceneNumbering %d %d", season, number)
	if f.bySceneNumbering == nil {
		return nil, nil
	}
	return f.bySceneNumbering(season, number), nil
}

func (f *fakeServices) FindEpisodesBySceneAbsolute(_ context.Context, _ core.ID, abs int) ([]*Episode, error) {
	f.record("FindEpisodesBySceneAbsolute %d", abs)
	return f.bySceneAbsolute, nil
}

func (f *fakeServices) EpisodesBySeason(_ context.Context, _ core.ID, season int) ([]*Episode, error) {
	f.record("EpisodesBySeason %d", season)
	return f.bySeason, nil
}

func (f *fakeServices) EpisodesBySceneSeason(_ context.Context, _ core.ID, season int) ([]*Episode, error) {
	f.record("EpisodesBySceneSeason %d", season)
	return f.bySceneSeason, nil
}

func (f *fakeServices) FindEpisodeByTitle(_ context.Context, _ core.ID, season int, title string) (*Episode, error) {
	f.record("FindEpisodeByTitle %d %s", season, title)
	if f.byEpisodeTitle == nil {
		return nil, nil
	}
	return f.byEpisodeTitle(season, title), nil
}

// fakeScene is Sonarr's mocked ISceneMappingService.
type fakeScene struct {
	mapping     func(seriesTitle string) *SceneMapping
	sceneSeason *int
}

func (f *fakeScene) FindSceneMapping(seriesTitle, _ string, _ int) (*SceneMapping, error) {
	if f.mapping == nil {
		return nil, nil
	}
	return f.mapping(seriesTitle), nil
}

func (f *fakeScene) GetSceneSeasonNumber(string, string) (int, bool, error) {
	if f.sceneSeason == nil {
		return 0, false, nil
	}
	return *f.sceneSeason, true, nil
}

// parsingFixture is the setup of Sonarr's GetEpisodesFixture and
// MapFixture: the series "30 Stone" with one episode, as NBuilder builds
// them (every number 1), and a release of S01E01.
type parsingFixture struct {
	series   *Series
	episodes []*Episode
	parsed   *release.ParsedEpisodeInfo
	search   *Search
	services *fakeServices
	scene    *fakeScene
}

func newParsingFixture() *parsingFixture {
	today := time.Now().Format(time.DateOnly)
	s := &Series{Target: tidTarget(1, "30 Stone"), CleanTitle: "stone", TvdbID: 1, TvRageID: 1, ImdbID: "ImdbId1"}
	e := &Episode{
		SeasonNumber: 1, EpisodeNumber: 1, AbsoluteEpisodeNumber: 1, AirDate: today,
		SceneSeasonNumber: intPtr(1), SceneEpisodeNumber: intPtr(1), SceneAbsoluteEpisodeNumber: intPtr(1),
	}
	e.ID = tid(1)
	f := &parsingFixture{
		series: s, episodes: []*Episode{e},
		parsed: &release.ParsedEpisodeInfo{
			SeriesTitle: s.Title, SeasonNumber: 1, EpisodeNumbers: []int{1}, Languages: []release.Language{release.English},
		},
		services: &fakeServices{},
		scene:    &fakeScene{},
	}
	f.search = &Search{Kind: SingleEpisodeSearch, Series: s, Episodes: f.episodes, SeasonNumber: 1, EpisodeNumber: 1}
	return f
}

func tidTarget(n int, title string) acquire.Target {
	return acquire.Target{ID: tid(n), Title: title, Kind: release.Series}
}

func (f *parsingFixture) parsing() *Parsing {
	return &Parsing{Series: f.services, Episodes: f.services, Scene: f.scene}
}

func (f *parsingFixture) mapRelease(t *testing.T, tvdbID, tvRageID int, imdbID string, s *Search) *Match {
	t.Helper()
	m, err := f.parsing().Map(t.Context(), f.parsed, tvdbID, tvRageID, imdbID, nil, s)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	return m
}

func (f *parsingFixture) mapToSeries(t *testing.T) *Match {
	t.Helper()
	m, err := f.parsing().Map(t.Context(), f.parsed, 0, 0, "", f.series, nil)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	return m
}

func (f *parsingFixture) getEpisodes(t *testing.T, sceneSource bool) []*Episode {
	t.Helper()
	eps, err := f.parsing().GetEpisodes(t.Context(), f.parsed, f.series, sceneSource, nil)
	if err != nil {
		t.Fatalf("GetEpisodes: %v", err)
	}
	return eps
}

// calls is how often calls starting with each prefix are wanted.
type calls map[string]int

func (f *parsingFixture) verify(t *testing.T, want calls) {
	t.Helper()
	for prefix, n := range want {
		if got := f.services.count(prefix); got != n {
			t.Errorf("calls of %q = %d, want = %d; calls: %q", prefix, got, n, f.services.calls)
		}
	}
}

// TestGetEpisodes ports Sonarr's GetEpisodesFixture
// (NzbDrone.Core.Test/ParserTests/ParsingServiceTests/GetEpisodesFixture.cs):
// which episode lookups mapping a release makes.
func TestGetEpisodes(t *testing.T) {
	daily := func(f *parsingFixture) {
		f.series.Type = Daily
		f.parsed.AirDate = time.Now().Format(time.DateOnly)
	}
	absolute := func(f *parsingFixture) { f.parsed.AbsoluteEpisodeNumbers = []int{1} }
	fullSeason := func(f *parsingFixture) { f.parsed.FullSeason, f.parsed.EpisodeNumbers = true, nil }
	sceneSeason := func(n int) func(f *parsingFixture) {
		return func(f *parsingFixture) { absolute(f); f.scene.sceneSeason = intPtr(n) }
	}
	special := func(f *parsingFixture) {
		f.parsed.SeasonNumber, f.parsed.EpisodeNumbers = 1, []int{0}
		f.parsed.ReleaseTitle = "Series.Title.S01E00.My.Special.Episode.1080p.AMZN.WEB-DL.DDP5.1.H264-TEPES"
	}
	type act int
	const (
		mapNoSearch act = iota
		mapSearch
		getScene
		getNoScene
	)
	tests := []struct {
		name  string
		setup func(f *parsingFixture)
		act   act
		want  calls
		count int // episodes wanted when set
	}{
		{"should_get_daily_episode_episode_when_search_criteria_is_null", daily, mapNoSearch, calls{"FindEpisodeByAirDate": 1}, 0},
		{"should_use_search_criteria_episode_when_it_matches_daily", daily, mapSearch, calls{"FindEpisodeByAirDate": 0}, 0},
		{"should_fallback_to_daily_episode_lookup_when_search_criteria_episode_doesnt_match", func(f *parsingFixture) {
			f.series.Type = Daily
			f.parsed.AirDate = time.Now().AddDate(0, 0, -5).Format(time.DateOnly)
		}, mapSearch, calls{"FindEpisodeByAirDate": 1}, 0},
		{"should_get_daily_episode_episode_should_lookup_including_daily_part", func(f *parsingFixture) {
			daily(f)
			f.parsed.DailyPart = intPtr(1)
		}, mapNoSearch, calls{"FindEpisodeByAirDate " + time.Now().Format(time.DateOnly) + " 1": 1}, 0},
		{"should_use_search_criteria_episode_when_it_matches_absolute", absolute, mapSearch, calls{"FindEpisodeByAirDate": 0}, 0},
		{
			"should_use_scene_numbering_when_series_uses_scene_numbering", func(f *parsingFixture) { f.series.UseSceneNumbering = true }, mapNoSearch,
			calls{"FindEpisodesBySceneNumbering": 1},
			0,
		},
		{
			"should_match_search_criteria_by_scene_numbering", func(f *parsingFixture) { f.series.UseSceneNumbering = true }, mapSearch,
			calls{"FindEpisodesBySceneNumbering": 0},
			0,
		},
		{"should_fallback_to_findEpisode_when_search_criteria_match_fails_for_scene_numbering", func(f *parsingFixture) {
			f.series.UseSceneNumbering = true
			f.episodes[0].SceneEpisodeNumber = intPtr(10)
		}, mapSearch, calls{"FindEpisodesBySceneNumbering": 1}, 0},
		{"should_find_episode", nil, mapNoSearch, calls{"FindEpisode ": 1}, 0},
		{"should_match_episode_with_search_criteria", nil, mapSearch, calls{"FindEpisode ": 0}, 0},
		{
			"should_fallback_to_findEpisode_when_search_criteria_match_fails", func(f *parsingFixture) { f.episodes[0].EpisodeNumber = 10 }, mapSearch,
			calls{"FindEpisode ": 1},
			0,
		},
		{"should_look_for_episode_in_season_zero_if_absolute_special", func(f *parsingFixture) {
			absolute(f)
			f.parsed.Special = true
		}, getScene, calls{"FindEpisodesBySceneNumbering 0 ": 0, "FindEpisode 0 ": 1}, 0},
		{"should_use_tvdb_season_number_when_available_and_a_scene_source", func(f *parsingFixture) {
			f.scene.mapping = func(string) *SceneMapping {
				return &SceneMapping{SceneSeasonNumber: intPtr(1), SeasonNumber: intPtr(5)}
			}
		}, getScene, calls{"FindEpisode 1 1": 0, "FindEpisode 5 1": 1}, 0},
		{"should_not_use_tvdb_season_number_when_available_for_a_different_season_and_a_scene_source", func(f *parsingFixture) {
			f.scene.mapping = func(string) *SceneMapping {
				return &SceneMapping{SceneSeasonNumber: intPtr(101), SeasonNumber: intPtr(5)}
			}
		}, getScene, calls{"FindEpisode 5 1": 0, "FindEpisode 1 1": 1}, 0},
		{"should_not_use_tvdb_season_when_not_a_scene_source", nil, getNoScene, calls{"FindEpisode 5 1": 0, "FindEpisode 1 1": 1}, 0},
		{"should_not_use_tvdb_season_when_tvdb_season_number_is_less_than_zero", func(f *parsingFixture) {
			f.scene.mapping = func(string) *SceneMapping {
				return &SceneMapping{SeasonNumber: intPtr(-1), SceneSeasonNumber: intPtr(1)}
			}
		}, getScene, calls{"FindEpisode -1 1": 0, "FindEpisode 1 1": 1}, 0},
		{"should_lookup_full_season_by_season_number_if_series_does_not_use_scene_numbering", func(f *parsingFixture) {
			fullSeason(f)
			f.services.bySeason = f.episodes
		}, getScene, calls{"EpisodesBySeason": 1, "EpisodesBySceneSeason": 0}, 1},
		{"should_lookup_full_season_by_scene_season_number_if_series_uses_scene_numbering", func(f *parsingFixture) {
			f.series.UseSceneNumbering = true
			fullSeason(f)
			f.services.bySceneSeason = f.episodes
		}, getScene, calls{"EpisodesBySeason": 0, "EpisodesBySceneSeason": 1}, 1},
		{"should_fallback_to_lookup_full_season_by_season_number_if_series_uses_scene_numbering_and_no_epsiodes_are_found_by_scene_season_number", func(f *parsingFixture) { //nolint:misspell // Sonarr's test name.
			f.series.UseSceneNumbering = true
			fullSeason(f)
			f.services.bySeason = f.episodes
		}, getScene, calls{"EpisodesBySeason": 1, "EpisodesBySceneSeason": 1}, 1},
		{"should_use_season_zero_when_looking_up_is_partial_special_episode_found_by_title", func(f *parsingFixture) {
			special(f)
			f.services.byTitle = f.series
			f.services.byEpisodeTitle = func(season int, title string) *Episode {
				if season == 0 && title == f.parsed.ReleaseTitle {
					return &Episode{SeasonNumber: 0, EpisodeNumber: 1}
				}
				return nil
			}
		}, mapNoSearch, calls{"FindEpisode 0 1": 1}, 0},
		{"should_use_original_parse_result_when_special_episode_lookup_by_title_fails", func(f *parsingFixture) {
			special(f)
			f.services.byTitle = f.series
		}, mapNoSearch, calls{"FindEpisode 1 0": 1}, 0},
	}
	for _, n := range []int{0, 1, 2} {
		tests = append(tests,
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{
				fmt.Sprintf("should_use_scene_numbering_when_scene_season_number_has_value(%d)", n), sceneSeason(n), getScene,
				calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 1},
				0,
			},
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{fmt.Sprintf("should_find_episode_by_season_and_scene_absolute_episode_number(%d)", n), func(f *parsingFixture) {
				sceneSeason(n)(f)
				f.services.bySceneNumbering = func(int, int) []*Episode { return f.episodes }
			}, getScene, calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 0}, 0},
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{fmt.Sprintf("should_return_episodes_when_scene_absolute_episode_number_returns_multiple_results(%d)", n), func(f *parsingFixture) {
				sceneSeason(n)(f)
				five := make([]*Episode, 5)
				for i := range five {
					five[i] = ep(i+1, 1, i+1)
				}
				f.services.bySceneNumbering = func(int, int) []*Episode { return five }
			}, getScene, calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 0}, 5},
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{
				fmt.Sprintf("should_find_episode_by_season_and_absolute_episode_number_when_scene_absolute_episode_number_returns_no_results(%d)", n), sceneSeason(n), getScene,
				calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 1},
				0,
			},
		)
	}
	for _, n := range []int{2, 20} {
		season := func(f *parsingFixture) {
			absolute(f)
			f.parsed.SeasonNumber, f.parsed.EpisodeNumbers = n, nil
		}
		tests = append(tests,
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{fmt.Sprintf("should_find_episode_by_parsed_season_and_absolute_episode_number_when_season_number_is_2_or_higher(%d)", n), func(f *parsingFixture) {
				season(f)
				f.services.bySceneNumbering = func(int, int) []*Episode { return f.episodes }
			}, getScene, calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 0}, 0},
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{fmt.Sprintf("should_find_episode_by_parsed_season_and_absolute_episode_number_when_season_number_is_2_or_higher_and_scene_season_number_lookup_failed(%d)", n), func(f *parsingFixture) {
				season(f)
				f.services.episode = func(int, int) *Episode { return f.episodes[0] }
			}, getScene, calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 1, fmt.Sprintf("FindEpisode %d ", n): 1}, 0},
			struct {
				name  string
				setup func(f *parsingFixture)
				act   act
				want  calls
				count int
			}{fmt.Sprintf("should_not_find_episode_by_parsed_season_and_absolute_episode_number_when_season_number_is_2_or_higher_and_a_episode_number_was_parsed(%d)", n), func(f *parsingFixture) {
				season(f)
				f.parsed.EpisodeNumbers = []int{1}
				f.services.bySceneAbsolute = f.episodes
			}, getScene, calls{fmt.Sprintf("FindEpisodesBySceneNumbering %d ", n): 0, fmt.Sprintf("FindEpisode %d ", n): 0}, 0},
		)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newParsingFixture()
			// GetEpisodesFixture finds the series by any title.
			f.services.byTitle = f.series
			if tt.setup != nil {
				tt.setup(f)
			}
			var got []*Episode
			switch tt.act {
			case mapNoSearch:
				got = f.mapRelease(t, f.series.TvdbID, f.series.TvRageID, f.series.ImdbID, nil).Episodes
			case mapSearch:
				got = f.mapRelease(t, f.series.TvdbID, f.series.TvRageID, f.series.ImdbID, f.search).Episodes
			case getScene:
				got = f.getEpisodes(t, true)
			case getNoScene:
				got = f.getEpisodes(t, false)
			}
			f.verify(t, tt.want)
			if tt.count > 0 && len(got) != tt.count {
				t.Errorf("episodes = %d, want = %d", len(got), tt.count)
			}
		})
	}
}

// TestMap ports Sonarr's MapFixture
// (NzbDrone.Core.Test/ParserTests/ParsingServiceTests/MapFixture.cs):
// how mapping a release finds its series.
func TestMap(t *testing.T) {
	otherName := func(f *parsingFixture) { f.parsed.SeriesTitle = "Another Name" }
	tests := []struct {
		name  string
		setup func(f *parsingFixture)
		// ids are the release's TVDB, TVRage and IMDb IDs; nil for the
		// series'.
		ids        []any
		search     bool
		want       calls
		wantSeries *bool
	}{
		{name: "should_lookup_series_by_name", setup: func(f *parsingFixture) { f.services.byTitle = f.series }, want: calls{"FindByTitle ": 1}},
		{name: "should_use_tvdbid_when_series_title_lookup_fails", setup: func(f *parsingFixture) { f.services.byTvdb = f.series }, want: calls{"FindByTvdbID": 1}},
		{
			name: "should_use_tvrageid_when_series_title_lookup_fails", setup: func(f *parsingFixture) { f.services.byTvRage = f.series }, ids: []any{0, 1, ""},
			want: calls{"FindByTvRageID": 1},
		},
		{name: "should_not_use_tvrageid_when_scene_naming_exception_exists", setup: func(f *parsingFixture) {
			f.services.byTvRage = f.series
			f.scene.mapping = func(string) *SceneMapping { return &SceneMapping{TvdbID: 10} }
		}, want: calls{"FindByTvRageID": 0}, wantSeries: new(false)},
		{
			name: "should_use_search_criteria_series_title", setup: func(f *parsingFixture) { f.services.byTitle = f.series }, search: true,
			want: calls{"FindByTitle ": 0},
		},
		{
			name: "should_FindByTitle_when_search_criteria_matching_fails", setup: otherName, ids: []any{10, 10, ""}, search: true,
			want: calls{"FindByTitle ": 1},
		},
		{name: "should_FindByTitle_using_year_when_FindByTitle_matching_fails", setup: func(f *parsingFixture) {
			otherName(f)
			f.parsed.SeriesTitleInfo = release.SeriesTitleInfo{Title: "Series Title 2017", TitleWithoutYear: "Series Title", Year: 2017}
			f.services.byTitleYear = f.series
		}, ids: []any{10, 10, ""}, search: true, want: calls{"FindByTitleYear": 1}},
		{
			name: "should_FindByTvdbId_when_search_criteria_and_FindByTitle_matching_fails", setup: otherName, ids: []any{10, 10, ""}, search: true,
			want: calls{"FindByTvdbID": 1},
		},
		{
			name: "should_FindByTvRageId_when_search_criteria_and_FindByTitle_matching_fails", setup: otherName, ids: []any{0, 10, ""}, search: true,
			want: calls{"FindByTvRageID": 1},
		},
		{
			name: "should_not_FindByTvRageId_when_search_criteria_and_FindByTitle_matching_fails_and_tvdb_id_is_specified", setup: otherName,
			ids: []any{10, 10, ""}, search: true, want: calls{"FindByTvRageID": 0},
		},
		{
			name: "should_FindByImdbId_when_search_criteria_and_FindByTitle_matching_fails", setup: otherName, ids: []any{0, 0, "tt12345"}, search: true,
			want: calls{"FindByImdbID": 1},
		},
		{
			name: "should_not_FindByImdbId_when_search_criteria_and_FindByTitle_matching_fails_and_tvdb_id_is_specified", setup: otherName,
			ids: []any{10, 10, "tt12345"}, search: true, want: calls{"FindByImdbID": 0},
		},
		{name: "should_use_tvdbid_matching_when_alias_is_found", setup: func(f *parsingFixture) {
			f.scene.mapping = func(string) *SceneMapping { return &SceneMapping{TvdbID: f.series.TvdbID} }
		}, search: true, want: calls{"FindByTitle ": 0}},
		{name: "should_use_tvrageid_match_from_search_criteria_when_title_match_fails", setup: otherName, search: true, want: calls{"FindByTitle ": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newParsingFixture()
			if tt.setup != nil {
				tt.setup(f)
			}
			tvdb, tvRage, imdb := f.series.TvdbID, f.series.TvRageID, f.series.ImdbID
			if tt.ids != nil {
				tvdb, tvRage, imdb = tt.ids[0].(int), tt.ids[1].(int), tt.ids[2].(string)
			}
			var s *Search
			if tt.search {
				s = f.search
			}
			m := f.mapRelease(t, tvdb, tvRage, imdb, s)
			f.verify(t, tt.want)
			if tt.wantSeries != nil && (m.Series != nil) != *tt.wantSeries {
				t.Errorf("series = %v, want found = %v", m.Series, *tt.wantSeries)
			}
		})
	}

	// should_use_scene_season_number_from_xem_mapping_if_alias_matches_a_specific_season_number
	// and its counterpart for a parsed season other than 1.
	for _, season := range []int{1, 2} {
		f := newParsingFixture()
		f.parsed.SeasonNumber = season
		f.scene.mapping = func(string) *SceneMapping { return &SceneMapping{Type: "XemService", SceneSeasonNumber: intPtr(2)} }
		if got := f.mapToSeries(t).MappedSeasonNumber; got != 2 {
			t.Errorf("parsed season %d: MappedSeasonNumber = %d, want = 2", season, got)
		}
	}
}

// TestGetSeries ports Sonarr's GetSeriesFixture
// (NzbDrone.Core.Test/ParserTests/ParsingServiceTests/GetSeriesFixture.cs).
func TestGetSeries(t *testing.T) {
	t.Run("should_use_passed_in_title_when_it_cannot_be_parsed", func(t *testing.T) {
		f := newParsingFixture()
		if _, err := f.parsing().GetSeries(t.Context(), "30 Stone"); err != nil {
			t.Fatal(err)
		}
		f.verify(t, calls{"FindByTitle 30 Stone": 1})
	})
	t.Run("should_use_parsed_series_title", func(t *testing.T) {
		f := newParsingFixture()
		const title = "30.Stone.S01E01.720p.hdtv"
		if _, err := f.parsing().GetSeries(t.Context(), title); err != nil {
			t.Fatal(err)
		}
		f.verify(t, calls{"FindByTitle " + release.ParseSeriesTitle(title).SeriesTitle: 1})
	})
	t.Run("should_fallback_to_title_without_year_and_year_when_title_lookup_fails", func(t *testing.T) {
		f := newParsingFixture()
		const title = "Show.2004.S01E01.720p.hdtv"
		parsed := release.ParseSeriesTitle(title)
		if _, err := f.parsing().GetSeries(t.Context(), title); err != nil {
			t.Fatal(err)
		}
		f.verify(t, calls{fmt.Sprintf("FindByTitleYear %s %d", parsed.SeriesTitleInfo.TitleWithoutYear, parsed.SeriesTitleInfo.Year): 1})
	})
	t.Run("should_parse_concatenated_title", func(t *testing.T) {
		f := newParsingFixture()
		series := &Series{TvdbID: 100}
		f.services.byTitle = nil
		services := &titleServices{fakeServices: f.services, titles: map[string]*Series{"Welcome": series}}
		f.scene.mapping = func(title string) *SceneMapping {
			if title == "Mairimashita" {
				return &SceneMapping{TvdbID: 100}
			}
			return nil
		}
		services.byTvdb = series
		p := &Parsing{Series: services, Episodes: f.services, Scene: f.scene}
		got, err := p.GetSeries(t.Context(), "Welcome (Mairimashita).S01E01.720p.WEB-DL-Viva")
		if err != nil || got == nil || got.TvdbID != 100 {
			t.Errorf("GetSeries = %+v, %v; want TVDB 100", got, err)
		}
	})
}

// titleServices finds series by exact titles.
type titleServices struct {
	*fakeServices
	titles map[string]*Series
}

func (s *titleServices) FindByTitle(ctx context.Context, title string) (*Series, error) {
	_, _ = s.fakeServices.FindByTitle(ctx, title)
	return s.titles[title], nil
}

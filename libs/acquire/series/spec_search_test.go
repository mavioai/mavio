package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestRepack ports Sonarr's RepackSpecificationFixture. The release is
// an SDTV v2 by Sonarr.
func TestRepack(t *testing.T) {
	tests := []struct {
		name         string
		propers      acquire.ProperDownload
		repack       bool
		fileQuality  release.Quality
		fileGroup    string
		releaseGroup string
		want         bool
	}{
		{name: "should_return_true_if_it_is_not_a_repack", want: true},
		{name: "should_return_true_if_there_are_is_no_episode_file", repack: true, want: true},
		{name: "should_return_true_if_is_a_repack_for_a_different_quality", repack: true, fileQuality: release.DVD, fileGroup: "Sonarr", want: true},
		{name: "should_return_true_if_is_a_repack_for_existing_file", repack: true, fileQuality: release.SDTV, fileGroup: "Sonarr", want: true},
		{name: "should_return_false_if_is_a_repack_for_a_different_file", repack: true, fileQuality: release.SDTV, fileGroup: "NotSonarr", want: false},
		{name: "should_return_false_if_release_group_for_existing_file_is_unknown", repack: true, fileQuality: release.SDTV, fileGroup: " ", want: false},
		{name: "should_return_false_if_release_group_for_release_is_unknown", repack: true, fileQuality: release.SDTV, fileGroup: "Sonarr", releaseGroup: " ", want: false},
		{name: "should_return_true_when_repacks_are_not_preferred", propers: acquire.PropersDoNotPrefer, repack: true, fileQuality: release.SDTV, fileGroup: "Sonarr", want: true},
		{name: "should_return_true_when_repack_but_auto_download_repacks_is_true", propers: acquire.PropersPreferAndUpgrade, repack: true, fileQuality: release.SDTV, fileGroup: "Sonarr", want: true},
		{name: "should_return_false_when_repack_but_auto_download_repacks_is_false", propers: acquire.PropersDoNotUpgrade, repack: true, fileQuality: release.SDTV, fileGroup: "Sonarr", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			if tt.propers != 0 {
				e.Config.Propers = tt.propers
			}
			episode := ep(1, 1, 1)
			if tt.fileQuality != 0 {
				f := file(1, model(tt.fileQuality, 1))
				f.ReleaseGroup = tt.fileGroup
				episode.File = f
			}
			q := acquire.QualityModel{Quality: release.SDTV, Revision: release.Revision{Version: 2, IsRepack: tt.repack}}
			c := testCandidate(testSeries(testProfile(release.SDTV, release.SDTV)), q, episode)
			c.ReleaseGroup = "Sonarr"
			if tt.releaseGroup != "" {
				c.ReleaseGroup = tt.releaseGroup
			}
			want(t, Root(acquire.Repack{}), e, c, nil, tt.want)
		})
	}
}

// TestSeries ports Sonarr's Search/SeriesSpecificationFixture, whose
// specification is the root SearchTarget.
func TestSeries(t *testing.T) {
	tests := []struct {
		name     string
		searched int
		want     bool
	}{
		{name: "should_return_false_if_series_doesnt_match", searched: 2, want: false},
		{name: "should_return_true_when_series_ids_match", searched: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), ep(1, 1, 1))
			searched := testSeries(c.Series.Profile)
			searched.ID = tid(tt.searched)
			r := check(t, Root(acquire.SearchTarget{}), e, c, &Search{Kind: SingleEpisodeSearch, Series: searched})
			if got := r == nil; got != tt.want {
				t.Errorf("accepted = %v (%+v), want = %v", got, r, tt.want)
			}
			if r != nil && (r.Reason != acquire.ReasonWrongTarget || r.Message != "Wrong series") {
				t.Errorf("rejection = %+v, want = %q", r, "Wrong series")
			}
		})
	}
}

// TestAnimeSearch ports Sonarr's
// Search/SingleEpisodeSearchMatchSpecificationTests/AnimeSearchFixture.
func TestAnimeSearch(t *testing.T) {
	tests := []struct {
		name                   string
		fullSeason, seasonWide bool
		want                   bool
	}{
		{name: "should_return_false_if_full_season_result_for_single_episode_search", fullSeason: true, want: false},
		{name: "should_return_true_if_not_a_full_season_result", want: true},
		{name: "should_return_true_if_full_season_result_for_full_season_search", fullSeason: true, seasonWide: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := NewCandidate(&acquire.Release{}, &release.ParsedEpisodeInfo{FullSeason: tt.fullSeason}, nil, nil)
			want(t, SingleEpisodeSearchMatch{}, e, c, &Search{Kind: AnimeEpisodeSearch, IsSeasonSearch: tt.seasonWide}, tt.want)
		})
	}
}

// TestStandardEpisodeSearch ports Sonarr's
// Search/SingleEpisodeSearchMatchSpecificationTests/StandardEpisodeSearch.
// The release is 5x01, searched for as 5x01, unless the case changes
// them; the mapped season plays no part.
func TestStandardEpisodeSearch(t *testing.T) {
	tests := []struct {
		name           string
		season, mapped int
		episodes       []int
		searchSeason   int
		want           bool
	}{
		{name: "should_return_false_if_season_does_not_match", season: 10, mapped: 10, episodes: []int{1}, searchSeason: 5, want: false},
		{name: "should_return_true_if_season_matches_after_scenemapping", season: 10, mapped: 5, episodes: []int{1}, searchSeason: 10, want: true},
		{name: "should_return_false_if_season_does_not_match_after_scenemapping", season: 10, mapped: 6, episodes: []int{1}, searchSeason: 9, want: false},
		{name: "should_return_false_if_full_season_result_for_single_episode_search", season: 5, mapped: 5, episodes: []int{}, searchSeason: 5, want: false},
		{name: "should_return_false_if_episode_number_does_not_match_search_criteria", season: 5, mapped: 5, episodes: []int{2}, searchSeason: 5, want: false},
		{name: "should_return_true_if_full_season_result_for_full_season_search", season: 5, mapped: 5, episodes: []int{1}, searchSeason: 5, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := NewCandidate(&acquire.Release{}, &release.ParsedEpisodeInfo{SeasonNumber: tt.season, EpisodeNumbers: tt.episodes}, nil, nil)
			c.MappedSeasonNumber = tt.mapped
			s := &Search{Kind: SingleEpisodeSearch, SeasonNumber: tt.searchSeason, EpisodeNumber: 1}
			want(t, SingleEpisodeSearchMatch{}, e, c, s, tt.want)
		})
	}
}

// TestSingleEpisodeAge ports Sonarr's
// SingleEpisodeAgeDownloadDecisionFixture: a season search for season 1,
// whose episodes aired 400, 370, 340 and 310 days ago.
func TestSingleEpisodeAge(t *testing.T) {
	episodes := []*Episode{ep(1, 1, 1), ep(2, 1, 2), ep(3, 1, 3), ep(4, 1, 4)}
	for i, age := range []int{400, 370, 340, 310} {
		episodes[i].AirDateUTC = testNow.AddDate(0, 0, -age)
	}
	tests := []struct {
		name    string
		episode int // 0 for the season pack
		maxAge  int
		want    bool
	}{
		{name: "single_episode_release(1,200,false)", episode: 1, maxAge: 200, want: false},
		{name: "single_episode_release(4,200,false)", episode: 4, maxAge: 200, want: false},
		{name: "single_episode_release(1,600,true)", episode: 1, maxAge: 600, want: true},
		{name: "single_episode_release(1,365,true)", episode: 1, maxAge: 365, want: true},
		{name: "single_episode_release(4,365,true)", episode: 4, maxAge: 365, want: true},
		{name: "single_episode_release(1,0,true)", episode: 1, maxAge: 0, want: true},
		{name: "multi_episode_release(200,true)", maxAge: 200, want: true},
		{name: "multi_episode_release(600,true)", maxAge: 600, want: true},
		{name: "multi_episode_release(365,true)", maxAge: 365, want: true},
		{name: "multi_episode_release(0,true)", maxAge: 0, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			s := testSeries(testProfile(release.SDTV, release.SDTV))
			s.Type = Standard
			var c *Candidate
			if tt.episode == 0 {
				c = testCandidate(s, model(release.SDTV, 2), episodes...)
				c.Parsed.FullSeason = true
			} else {
				c = testCandidate(s, model(release.SDTV, 2), episodes[tt.episode-1])
			}
			c.SeasonSearchMaximumSingleEpisodeAge = tt.maxAge
			search := &Search{Kind: SeasonSearch, Episodes: episodes, SeasonNumber: 1}
			want(t, SeasonPackOnly{}, e, c, search, tt.want)
		})
	}
}

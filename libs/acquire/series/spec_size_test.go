package series

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestAcceptableSize ports Sonarr's AcceptableSizeSpecificationFixture.
// SDTV is 2 to 10 MB a minute; the series has seasons 1 and 2, and
// season 1 has ten 30-minute episodes, the first airing at testNow and
// each a day after the previous one. Releases are SDTV v2 of one episode
// (1x01), two or six episodes of season 1.
func TestAcceptableSize(t *testing.T) {
	season := make([]*Episode, 10)
	for i := range season {
		season[i] = ep(i+1, 1, i+1)
		season[i].Runtime = 30
		season[i].AirDateUTC = testNow.AddDate(0, 0, i)
	}
	pilotAir := season[0].AirDateUTC
	type sizeCase struct {
		name          string
		runtime       int // the series'
		count         int // episodes in the release
		epRuntime     int // each episode's
		sizeMB        int64
		size          int64 // in bytes, when sizeMB is 0
		quality       release.Quality
		seriesType    Type
		special       bool
		minMB, maxMB  float64
		episodeSeason int
		airDate       time.Time
		want          bool
	}
	tests := []sizeCase{
		{name: "single_episode(30,50,false)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 50, want: false},
		{name: "single_episode(30,250,true)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 250, want: true},
		{name: "single_episode(30,500,false)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 500, want: false},
		{name: "single_episode(60,100,false)", runtime: 60, count: 1, epRuntime: 60, sizeMB: 100, want: false},
		{name: "single_episode(60,500,true)", runtime: 60, count: 1, epRuntime: 60, sizeMB: 500, want: true},
		{name: "single_episode(60,1000,false)", runtime: 60, count: 1, epRuntime: 60, sizeMB: 1000, want: false},
		// The first and last episodes of the season take the 30-minute
		// runtime of the season's episodes.
		{name: "should_return_expected_result_for_first_episode_of_season(30,250,true)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 250, want: true},
		{name: "should_return_expected_result_for_first_episode_of_season(30,1000,false)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 1000, want: false},
		{name: "should_return_expected_result_for_first_episode_of_season(60,250,true)", runtime: 60, count: 1, epRuntime: 30, sizeMB: 250, want: true},
		{name: "should_return_expected_result_for_first_episode_of_season(60,2000,false)", runtime: 60, count: 1, epRuntime: 30, sizeMB: 2000, want: false},
		{name: "should_return_expected_result_for_last_episode_of_season(30,250,true)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 250, want: true},
		{name: "should_return_expected_result_for_last_episode_of_season(30,1000,false)", runtime: 30, count: 1, epRuntime: 30, sizeMB: 1000, want: false},
		{name: "should_return_expected_result_for_last_episode_of_season(60,250,true)", runtime: 60, count: 1, epRuntime: 30, sizeMB: 250, want: true},
		{name: "should_return_expected_result_for_last_episode_of_season(60,2000,false)", runtime: 60, count: 1, epRuntime: 30, sizeMB: 2000, want: false},
		{name: "multi_episode(30,100,false)", runtime: 30, count: 2, epRuntime: 30, sizeMB: 50 * 2, want: false},
		{name: "multi_episode(30,500,true)", runtime: 30, count: 2, epRuntime: 30, sizeMB: 250 * 2, want: true},
		{name: "multi_episode(30,1000,false)", runtime: 30, count: 2, epRuntime: 30, sizeMB: 500 * 2, want: false},
		{name: "multi_episode(60,200,false)", runtime: 60, count: 2, epRuntime: 60, sizeMB: 100 * 2, want: false},
		{name: "multi_episode(60,1000,true)", runtime: 60, count: 2, epRuntime: 60, sizeMB: 500 * 2, want: true},
		{name: "multi_episode(60,2000,false)", runtime: 60, count: 2, epRuntime: 60, sizeMB: 1000 * 2, want: false},
		{name: "multiset_episode(30,300,false)", runtime: 30, count: 6, epRuntime: 30, sizeMB: 50 * 6, want: false},
		{name: "multiset_episode(30,1500,true)", runtime: 30, count: 6, epRuntime: 30, sizeMB: 250 * 6, want: true},
		{name: "multiset_episode(30,3000,false)", runtime: 30, count: 6, epRuntime: 30, sizeMB: 500 * 6, want: false},
		{name: "multiset_episode(60,600,false)", runtime: 60, count: 6, epRuntime: 60, sizeMB: 100 * 6, want: false},
		{name: "multiset_episode(60,3000,true)", runtime: 60, count: 6, epRuntime: 60, sizeMB: 500 * 6, want: true},
		{name: "multiset_episode(60,6000,false)", runtime: 60, count: 6, epRuntime: 60, sizeMB: 1000 * 6, want: false},
		{name: "should_return_true_if_size_is_zero", runtime: 30, count: 1, epRuntime: 30, minMB: 10, maxMB: 20, want: true},
		{name: "should_return_true_if_unlimited_30_minute", runtime: 30, count: 1, epRuntime: 30, size: 18457280000, maxMB: -1, want: true},
		{name: "should_return_true_if_unlimited_60_minute", runtime: 60, count: 1, epRuntime: 60, size: 36857280000, maxMB: -1, want: true},
		{name: "should_treat_daily_series_as_single_episode", runtime: 60, count: 1, epRuntime: 60, seriesType: Daily, sizeMB: 300, maxMB: 10, want: true},
		{name: "should_return_true_if_RAWHD", runtime: 45, count: 1, epRuntime: 30, seriesType: Daily, quality: release.RawHD, sizeMB: 8000, want: true},
		// Sonarr's case leaves the size and runtime unset.
		{name: "should_return_true_for_special", count: 1, special: true, want: true},
		{name: "should_return_false_if_series_runtime_is_zero_and_single_episode_is_not_from_first_season", count: 1, sizeMB: 200, episodeSeason: 2, want: false},
		{name: "should_return_false_if_series_runtime_is_zero_and_single_episode_aired_more_than_24_hours_after_first_aired_episode", count: 1, sizeMB: 200, airDate: pilotAir.AddDate(0, 0, 7), want: false},
		{name: "should_return_true_if_series_runtime_is_zero_and_single_episode_aired_less_than_24_hours_after_first_aired_episode", count: 1, sizeMB: 200, airDate: pilotAir.Add(time.Hour), want: true},
		{name: "should_return_false_if_series_runtime_is_zero_and_multi_episode_is_not_from_first_season", count: 2, sizeMB: 200, episodeSeason: 2, want: false},
		{name: "should_return_false_if_series_runtime_is_zero_and_multi_episode_aired_more_than_24_hours_after_first_aired_episode", count: 2, sizeMB: 200, airDate: pilotAir.AddDate(0, 0, 7), want: false},
		{name: "should_return_true_if_series_runtime_is_zero_and_multi_episode_aired_less_than_24_hours_after_first_aired_episode", count: 2, sizeMB: 200, airDate: pilotAir.Add(time.Hour), want: true},
		{name: "should_use_series_runtime_if_episode_runtime_is_not_set", runtime: 30, count: 1, sizeMB: 200, airDate: pilotAir.Add(time.Hour), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, eps := newTestEnv()
			eps.bySeason = map[int][]*Episode{1: season}
			sdtv := acquire.Definition{Quality: release.SDTV, MinSize: 2, MaxSize: 10}
			if tt.minMB != 0 {
				sdtv.MinSize = tt.minMB
			}
			switch {
			case tt.maxMB < 0:
				sdtv.MaxSize = 0
			case tt.maxMB > 0:
				sdtv.MaxSize = tt.maxMB
			}
			for i, d := range e.Definitions {
				if d.Quality == release.SDTV {
					e.Definitions[i] = sdtv
				}
			}
			s := testSeries(testProfile(release.SDTV, release.SDTV))
			s.Seasons = []int{1, 2}
			s.Runtime = tt.runtime
			s.Type = tt.seriesType
			var episodes []*Episode
			for i := range tt.count {
				x := ep(100+i, 1, i+1)
				x.Runtime = tt.epRuntime
				x.AirDateUTC = testNow.AddDate(0, 0, i)
				if tt.episodeSeason != 0 {
					x.SeasonNumber = tt.episodeSeason
				}
				if !tt.airDate.IsZero() {
					x.AirDateUTC = tt.airDate
				}
				episodes = append(episodes, x)
			}
			q := tt.quality
			if q == 0 {
				q = release.SDTV
			}
			c := testCandidate(s, model(q, 2), episodes...)
			c.Parsed.Special = tt.special
			c.Release.Size = tt.sizeMB << 20
			if tt.size != 0 {
				c.Release.Size = tt.size
			}
			want(t, AcceptableSize{}, e, c, nil, tt.want)
		})
	}
}

// TestAcceptableSizeMessages checks the messages of AcceptableSize
// rejections against Sonarr's.
func TestAcceptableSizeMessages(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		runtime int
		sizeMB  int64
		want    string
	}{
		{name: "below", count: 1, runtime: 30, sizeMB: 50, want: "50.0 MB is smaller than minimum allowed 60.0 MB (for 30min)"},
		{name: "above", count: 2, runtime: 30, sizeMB: 1000, want: "1,000.0 MB is larger than maximum allowed 600.0 MB (for 2x 60min)"},
		{name: "unknown_runtime", count: 1, runtime: 0, sizeMB: 50, want: "Runtime of all episodes is 0, unable to validate size until it is available"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, eps := newTestEnv()
			e.Definitions = []acquire.Definition{{Quality: release.SDTV, MinSize: 2, MaxSize: 10}}
			eps.bySeason = map[int][]*Episode{1: {ep(1, 1, 1)}}
			s := testSeries(testProfile(release.SDTV, release.SDTV))
			s.Runtime = tt.runtime
			var episodes []*Episode
			for i := range tt.count {
				x := ep(10+i, 2, i+1)
				episodes = append(episodes, x)
			}
			c := testCandidate(s, model(release.SDTV, 1), episodes...)
			c.Release.Size = tt.sizeMB << 20
			r := check(t, AcceptableSize{}, e, c, nil)
			if r == nil || r.Message != tt.want {
				t.Errorf("rejection = %+v, want = %q", r, tt.want)
			}
		})
	}
}

// TestDelay ports Sonarr's RssSync/DelaySpecificationFixture. The
// profile allows HDTV-720p, WEBDL-720p and Bluray-720p with WEBDL-720p
// the cutoff; usenet is preferred; the release is a usenet release of
// one episode without a file, published at testNow unless the case says
// otherwise.
func TestDelay(t *testing.T) {
	tests := []struct {
		name          string
		search        *Search
		quality       acquire.QualityModel
		age           time.Duration
		delayMinutes  int
		bypassQuality bool
		bypassFormat  bool
		minFormat     int
		formatScore   int
		file          *acquire.QualityModel
		want          bool
	}{
		{name: "should_be_true_when_user_invoked_search", search: &Search{Kind: SingleEpisodeSearch, Search: acquire.Search{UserInvoked: true}}, delayMinutes: 720, want: true},
		{name: "should_be_false_when_system_invoked_search_and_release_is_younger_than_delay", search: &Search{Kind: SingleEpisodeSearch}, quality: model(release.SDTV, 1), delayMinutes: 720, want: false},
		{name: "should_be_true_when_profile_does_not_have_a_delay", delayMinutes: 0, want: true},
		{name: "should_be_false_when_quality_and_language_is_last_allowed_in_profile_and_bypass_disabled", quality: model(release.Bluray720p, 1), delayMinutes: 720, want: false},
		{name: "should_be_true_when_quality_and_language_is_last_allowed_in_profile_and_bypass_enabled", quality: model(release.Bluray720p, 1), delayMinutes: 720, bypassQuality: true, want: true},
		{name: "should_be_true_when_release_is_older_than_delay", quality: model(release.HDTV720p, 1), age: 10 * time.Hour, delayMinutes: 60, want: true},
		{name: "should_be_false_when_release_is_younger_than_delay", quality: model(release.SDTV, 1), delayMinutes: 720, want: false},
		{name: "should_be_true_when_release_is_a_proper_for_existing_episode", quality: model(release.HDTV720p, 2), file: &acquire.QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1}}, delayMinutes: 720, want: true},
		{name: "should_be_true_when_release_is_a_real_for_existing_episode", quality: acquire.QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1, Real: 1}}, file: &acquire.QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1}}, delayMinutes: 720, want: true},
		{name: "should_be_false_when_release_is_proper_for_existing_episode_of_different_quality", quality: model(release.WEBDL720p, 2), file: &acquire.QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1}}, delayMinutes: 720, want: false},
		{name: "should_be_false_when_custom_format_score_is_above_minimum_but_bypass_disabled", formatScore: 100, delayMinutes: 720, minFormat: 50, want: false},
		{name: "should_be_false_when_custom_format_score_is_above_minimum_and_bypass_enabled_but_under_minimum", formatScore: 5, delayMinutes: 720, bypassFormat: true, minFormat: 50, want: false},
		{name: "should_be_true_when_custom_format_score_is_above_minimum_and_bypass_enabled", formatScore: 100, delayMinutes: 720, bypassFormat: true, minFormat: 50, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			e.DelayProfiles = []acquire.DelayProfile{{
				Default: true, EnableUsenet: true, EnableTorrent: true, PreferredProtocol: acquire.ProtocolUsenet,
				UsenetDelay: time.Duration(tt.delayMinutes) * time.Minute, BypassIfHighestQuality: tt.bypassQuality,
				BypassIfAboveCustomFormatScore: tt.bypassFormat, MinimumCustomFormatScore: tt.minFormat,
			}}
			p := testProfile(release.WEBDL720p, release.HDTV720p, release.WEBDL720p, release.Bluray720p)
			episode := ep(1, 1, 1)
			if tt.file != nil {
				episode.File = file(1, *tt.file)
				episode.File.Languages = []release.Language{release.English}
			}
			q := tt.quality
			if q.Quality == 0 {
				q = model(release.Unknown, 1)
			}
			c := testCandidate(testSeries(p), q, episode)
			c.Release.PublishDate = testNow.Add(-tt.age)
			c.FormatScore = tt.formatScore
			want(t, Delay{}, e, c, tt.search, tt.want)
		})
	}
}

// TestDelayPending checks that a release pending for longer than the
// delay lets the release through, as Sonarr's OldestPendingRelease does.
func TestDelayPending(t *testing.T) {
	tests := []struct {
		name       string
		pendingAge time.Duration
		want       bool
	}{
		{name: "pending_older_than_delay", pendingAge: 2 * time.Hour, want: true},
		{name: "pending_younger_than_delay", pendingAge: 30 * time.Minute, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, eps := newTestEnv()
			e.DelayProfiles = []acquire.DelayProfile{{Default: true, EnableUsenet: true, PreferredProtocol: acquire.ProtocolUsenet, UsenetDelay: time.Hour}}
			eps.pending = []acquire.Release{{PublishDate: testNow.Add(-tt.pendingAge)}}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.SDTV, release.HDTV720p)), model(release.SDTV, 1), ep(1, 1, 1))
			c.Release.PublishDate = testNow
			want(t, Delay{}, e, c, nil, tt.want)
		})
	}
}

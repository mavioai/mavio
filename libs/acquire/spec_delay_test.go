package acquire

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/release"
)

// TestDelaySpec ports Radarr's RssSync/DelaySpecificationFixture.
func TestDelaySpec(t *testing.T) {
	type given struct {
		quality      QualityModel
		age          time.Duration
		file         *QualityModel
		usenetDelay  time.Duration
		bypassHigh   bool
		bypassScore  bool
		minimumScore int
		formatScore  int
		search       *Search
	}
	tests := []struct {
		name string
		g    given
		want bool
	}{
		{
			name: "should_be_false_when_system_invoked_search_and_release_is_younger_than_delay",
			g:    given{quality: NewQualityModel(release.SDTV), usenetDelay: 720 * time.Minute, search: &Search{TargetID: tid(1)}},
			want: false,
		},
		{
			name: "should_be_true_when_profile_does_not_have_a_delay",
			g:    given{usenetDelay: 0},
			want: true,
		},
		{
			name: "should_be_false_when_quality_is_last_allowed_in_profile_and_bypass_disabled",
			g:    given{quality: NewQualityModel(release.Bluray720p), usenetDelay: 720 * time.Minute},
			want: false,
		},
		{
			name: "should_be_true_when_quality_is_last_allowed_in_profile_and_bypass_enabled",
			g:    given{quality: NewQualityModel(release.Bluray720p), usenetDelay: 720 * time.Minute, bypassHigh: true},
			want: true,
		},
		{
			name: "should_be_true_when_release_is_older_than_delay",
			g:    given{quality: NewQualityModel(release.HDTV720p), age: 10 * time.Hour, usenetDelay: 60 * time.Minute},
			want: true,
		},
		{
			name: "should_be_false_when_release_is_younger_than_delay",
			g:    given{quality: NewQualityModel(release.SDTV), usenetDelay: 720 * time.Minute},
			want: false,
		},
		{
			name: "should_be_true_when_release_is_a_proper_for_existing_movie",
			g: given{
				quality:     QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 2}},
				file:        &QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1}},
				usenetDelay: 720 * time.Minute,
			},
			want: true,
		},
		{
			name: "should_be_true_when_release_is_a_real_for_existing_movie",
			g: given{
				quality:     QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1, Real: 1}},
				file:        &QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 1}},
				usenetDelay: 720 * time.Minute,
			},
			want: true,
		},
		{
			name: "should_be_false_when_release_is_proper_for_existing_movie_of_different_quality",
			g: given{
				quality:     QualityModel{Quality: release.HDTV720p, Revision: release.Revision{Version: 2}},
				file:        &QualityModel{Quality: release.SDTV, Revision: release.Revision{Version: 1}},
				usenetDelay: 720 * time.Minute,
			},
			want: false,
		},
		{
			name: "should_be_false_when_custom_format_score_is_above_minimum_but_bypass_disabled",
			g:    given{formatScore: 100, usenetDelay: 720 * time.Minute, minimumScore: 50},
			want: false,
		},
		{
			name: "should_be_false_when_custom_format_score_is_above_minimum_and_bypass_enabled_but_under_minimum",
			g:    given{formatScore: 5, usenetDelay: 720 * time.Minute, bypassScore: true, minimumScore: 50},
			want: false,
		},
		{
			name: "should_be_true_when_custom_format_score_is_above_minimum_and_bypass_enabled",
			g:    given{formatScore: 100, usenetDelay: 720 * time.Minute, bypassScore: true, minimumScore: 50},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.DelayProfiles = []DelayProfile{{
				ID: tid(1), Default: true, EnableUsenet: true, EnableTorrent: true,
				PreferredProtocol: ProtocolUsenet, UsenetDelay: tt.g.usenetDelay,
				BypassIfHighestQuality: tt.g.bypassHigh, BypassIfAboveCustomFormatScore: tt.g.bypassScore,
				MinimumCustomFormatScore: tt.g.minimumScore, Order: 1<<31 - 1,
			}}
			p := testProfile(release.WEBDL720p, release.HDTV720p, release.WEBDL720p, release.Bluray720p)
			c := testMovie(p, release.Unknown)
			c.Quality = tt.g.quality
			c.FormatScore = tt.g.formatScore
			c.Release.PublishDate = testNow.Add(-tt.g.age)
			if tt.g.file != nil {
				c.Items[0].File = &File{ID: tid(1), Quality: *tt.g.file}
			}
			if got := accepted(t, Delay{}, e, c, tt.g.search); got != tt.want {
				t.Errorf("accepted = %v, want = %v", got, tt.want)
			}
		})
	}

	t.Run("should_be_true_when_user_invoked_search", func(t *testing.T) {
		e, _ := newTestEnv()
		// Radarr passes an empty RemoteMovie: the check must not look at
		// it.
		if got := accepted(t, Delay{}, e, &Candidate{}, &Search{UserInvoked: true}); !got {
			t.Errorf("accepted = %v, want = %v", got, true)
		}
	})
}

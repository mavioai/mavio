package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestQueue ports Sonarr's QueueSpecificationFixture. The release is a
// DVD v1 of episode 1 of series 1, or of episodes 1 and 2; the profile
// allows upgrades of the default qualities, with the cutoff of the case
// (Sonarr's unset cutoff is Unknown). Languages play no part in Sonarr's
// queue check and are left out.
func TestQueue(t *testing.T) {
	myFormat := acquire.CustomFormat{ID: tid(1), Name: "My Format"}
	badFormat := acquire.CustomFormat{ID: tid(2), Name: "Bad Format"}
	type queued struct {
		series   int
		episodes []int
		quality  release.Quality
		formats  []acquire.CustomFormat
	}
	hdtv := func(episodes ...int) queued { return queued{series: 1, episodes: episodes, quality: release.HDTV720p} }
	tests := []struct {
		name          string
		cutoff        release.Quality
		noUpgrades    bool
		propers       acquire.ProperDownload
		multi         bool
		newQuality    acquire.QualityModel
		formats       bool // the release matches My Format, scored 1, and Bad Format scores -2
		queue         []queued
		failedPending bool
		want          bool
	}{
		{name: "should_return_true_when_queue_is_empty", want: true},
		{name: "should_return_true_when_series_doesnt_match", queue: []queued{{series: 2, episodes: []int{1}, quality: release.DVD}}, want: true},
		{name: "should_return_false_if_everything_is_the_same", cutoff: release.Bluray1080p, queue: []queued{{series: 1, episodes: []int{1}, quality: release.DVD}}, want: false},
		{name: "should_return_true_when_quality_in_queue_is_lower", cutoff: release.Bluray1080p, queue: []queued{{series: 1, episodes: []int{1}, quality: release.SDTV}}, want: true},
		{name: "should_return_true_when_quality_in_queue_is_lower_but_language_is_higher", cutoff: release.Bluray1080p, queue: []queued{{series: 1, episodes: []int{1}, quality: release.SDTV}}, want: true},
		{name: "should_return_true_when_episode_doesnt_match", queue: []queued{{series: 1, episodes: []int{2}, quality: release.DVD}}, want: true},
		{
			name: "should_return_true_when_qualities_are_the_same_and_languages_are_the_same_with_higher_custom_format_score", formats: true,
			queue: []queued{{series: 1, episodes: []int{1}, quality: release.DVD, formats: []acquire.CustomFormat{badFormat}}}, want: true,
		},
		{name: "should_return_false_when_qualities_are_the_same_and_languages_are_the_same", queue: []queued{{series: 1, episodes: []int{1}, quality: release.DVD}}, want: false},
		{name: "should_return_false_when_quality_in_queue_is_better", cutoff: release.Bluray1080p, queue: []queued{hdtv(1)}, want: false},
		{name: "should_return_false_if_matching_multi_episode_is_in_queue", queue: []queued{hdtv(1, 2)}, want: false},
		{name: "should_return_false_if_multi_episode_has_one_episode_in_queue", multi: true, queue: []queued{hdtv(1)}, want: false},
		{name: "should_return_false_if_multi_part_episode_is_already_in_queue", multi: true, queue: []queued{hdtv(1, 2)}, want: false},
		{name: "should_return_false_if_multi_part_episode_has_two_episodes_in_queue", multi: true, queue: []queued{hdtv(1), hdtv(2)}, want: false},
		{name: "should_return_false_if_quality_in_queue_meets_cutoff", cutoff: release.DVD, queue: []queued{hdtv(1)}, want: false},
		{name: "should_return_false_when_quality_are_the_same_language_is_better_and_upgrade_allowed_is_false_for_language_profile", queue: []queued{{series: 1, episodes: []int{1}, quality: release.DVD}}, want: false},
		{
			name: "should_return_false_when_quality_is_better_languages_are_the_same_and_upgrade_allowed_is_false_for_quality_profile", cutoff: release.Bluray1080p, noUpgrades: true,
			queue: []queued{{series: 1, episodes: []int{1}, quality: release.Bluray1080p}}, want: false,
		},
		{name: "should_return_true_if_everything_is_the_same_for_failed_pending", cutoff: release.Bluray1080p, failedPending: true, queue: []queued{{series: 1, episodes: []int{1}, quality: release.DVD}}, want: true},
		{
			name: "should_return_false_if_same_quality_non_proper_in_queue_and_download_propers_is_do_not_upgrade", cutoff: release.HDTV720p,
			propers: acquire.PropersDoNotUpgrade, newQuality: model(release.HDTV720p, 2), queue: []queued{hdtv(1)}, want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			if tt.propers != 0 {
				e.Config.Propers = tt.propers
			}
			p := &acquire.QualityProfile{UpgradeAllowed: !tt.noUpgrades, Cutoff: acquire.QualityID(tt.cutoff), Items: defaultQualities()}
			if tt.formats {
				p.FormatItems = sampleFormatItems([]acquire.CustomFormat{myFormat, badFormat}, "My Format")
			}
			for _, q := range tt.queue {
				ports.queue = append(ports.queue, acquire.QueueEntry{
					TargetID: tid(q.series), ItemIDs: ids(q.episodes...), Quality: model(q.quality, 1),
					Formats: q.formats, FailedPending: tt.failedPending,
				})
			}
			episodes := []*Episode{ep(1, 1, 1)}
			if tt.multi {
				episodes = append(episodes, ep(2, 2, 2))
			}
			newQuality := tt.newQuality
			if newQuality.Quality == 0 {
				newQuality = model(release.DVD, 1)
			}
			c := testCandidate(testSeries(p), newQuality, episodes...)
			if tt.formats {
				c.Formats = []acquire.CustomFormat{myFormat}
			}
			want(t, Queue{}, e, c, nil, tt.want)
		})
	}
}

package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestQueue ports Radarr's QueueSpecificationFixture. The release is a
// DVD of movie 1, whose profile allows upgrades with no cutoff set (ID
// 0, Unknown) unless the case sets one.
func TestQueue(t *testing.T) {
	tests := []struct {
		name           string
		propers        ProperDownload
		cutoff         release.Quality
		upgradeAllowed bool
		newQuality     QualityModel
		queue          []QueueEntry
		want           bool
	}{
		{
			name: "should_return_true_when_queue_is_empty", upgradeAllowed: true,
			newQuality: upModel(release.DVD, 1), want: true,
		},
		{
			name: "should_return_true_when_movie_doesnt_match", upgradeAllowed: true,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(2), ItemIDs: []core.ID{tid(2)}, Quality: upModel(release.DVD, 1)}},
			want:       true,
		},
		{
			name: "should_return_true_when_quality_in_queue_is_lower", upgradeAllowed: true, cutoff: release.Bluray1080p,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.SDTV, 1)}},
			want:       true,
		},
		{
			name: "should_return_false_when_qualities_are_the_same", upgradeAllowed: true,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.DVD, 1)}},
			want:       false,
		},
		{
			name: "should_return_false_when_quality_in_queue_is_better", upgradeAllowed: true, cutoff: release.Bluray1080p,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.HDTV720p, 1)}},
			want:       false,
		},
		{
			name: "should_return_false_if_quality_in_queue_meets_cutoff", upgradeAllowed: true, cutoff: release.DVD,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.HDTV720p, 1)}},
			want:       false,
		},
		{
			name: "should_return_false_when_quality_is_better_and_upgrade_allowed_is_false_for_quality_profile", cutoff: release.Bluray1080p,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.Bluray1080p, 1)}},
			want:       false,
		},
		{
			name: "should_return_true_if_everything_is_the_same_for_failed_pending", upgradeAllowed: true, cutoff: release.Bluray1080p,
			newQuality: upModel(release.DVD, 1),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.DVD, 1), FailedPending: true}},
			want:       true,
		},
		{
			name: "should_return_false_if_same_quality_non_proper_in_queue_and_download_propers_is_do_not_upgrade",
			// The queued release's English language plays no part.
			propers: PropersDoNotUpgrade, upgradeAllowed: true, cutoff: release.HDTV720p,
			newQuality: upModel(release.HDTV720p, 2),
			queue:      []QueueEntry{{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}, Quality: upModel(release.HDTV720p, 1)}},
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			e.Config.Propers = tt.propers
			ports.queue = tt.queue
			c := testMovie(upProfile(tt.upgradeAllowed, tt.cutoff), tt.newQuality.Quality)
			c.Quality = tt.newQuality
			r := check(t, Queue{}, e, c, nil)
			if got := r == nil; got != tt.want {
				t.Errorf("Queue accepted got = %v (%v), want = %v", got, r, tt.want)
			}
		})
	}
}

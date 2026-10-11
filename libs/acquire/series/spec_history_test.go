package series

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestHistory ports Sonarr's RssSync/HistorySpecificationFixture. Its
// commented-out case, should_return_true_if_latest_history_has_a_download_id_and_cdh_is_enabled,
// is not ported. The release is a DVD v2 of episode 1 (12x03), or of
// episodes 1 to 3; the profile allows upgrades of the default qualities
// to Bluray-1080p unless the case replaces it with Sonarr's bare profile,
// which does not. The history is each episode's most recent event.
func TestHistory(t *testing.T) {
	upgradable := model(release.SDTV, 1)
	notUpgradable := model(release.HDTV1080p, 2)
	grab := func(id int, q acquire.QualityModel, age time.Duration) acquire.HistoryEvent {
		return acquire.HistoryEvent{Type: acquire.HistoryGrabbed, ItemID: tid(id), Date: testNow.Add(-age), Quality: q}
	}
	tests := []struct {
		name       string
		search     *Search
		multi      bool
		cdhOff     bool
		bare       release.Quality // the cutoff of a bare profile, or 0
		newQuality acquire.QualityModel
		history    []acquire.HistoryEvent
		want       bool
	}{
		{name: "should_return_true_if_it_is_a_search", search: &Search{Kind: SeasonSearch}, multi: true, want: true},
		{name: "should_return_true_if_latest_history_item_is_null", multi: true, want: true},
		{
			name: "should_return_true_if_latest_history_item_is_not_grabbed", multi: true,
			history: []acquire.HistoryEvent{{Type: acquire.HistoryDownloadFailed, ItemID: tid(1), Date: testNow, Quality: notUpgradable}},
			want:    true,
		},
		{name: "should_return_true_if_latest_history_item_is_older_than_twelve_hours", multi: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, 13*time.Hour)}, want: true},
		{name: "should_be_upgradable_if_only_episode_is_upgradable", history: []acquire.HistoryEvent{grab(1, upgradable, 0)}, want: true},
		{name: "should_be_upgradable_if_both_episodes_are_upgradable", multi: true, history: []acquire.HistoryEvent{grab(1, upgradable, 0), grab(2, upgradable, 0)}, want: true},
		{name: "should_not_be_upgradable_if_both_episodes_are_not_upgradable", multi: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, 0), grab(2, notUpgradable, 0)}, want: false},
		// Sonarr's case sets episode 1's most recent event twice; the
		// second wins.
		{name: "should_be_not_upgradable_if_only_first_episodes_is_upgradable", multi: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, 0)}, want: false},
		{name: "should_be_not_upgradable_if_only_second_episodes_is_upgradable", multi: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, 0), grab(2, upgradable, 0)}, want: false},
		{
			name: "should_not_be_upgradable_if_episode_is_of_same_quality_as_existing", bare: release.Bluray1080p,
			newQuality: model(release.WEBDL1080p, 1), history: []acquire.HistoryEvent{grab(1, model(release.WEBDL1080p, 1), 0)}, want: false,
		},
		{
			name: "should_not_be_upgradable_if_cutoff_already_met", bare: release.WEBDL1080p,
			newQuality: model(release.WEBDL1080p, 1), history: []acquire.HistoryEvent{grab(1, model(release.WEBDL1080p, 1), 0)}, want: false,
		},
		{name: "should_return_false_if_latest_history_item_is_only_one_hour_old", multi: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, time.Hour)}, want: false},
		// Sonarr's case is named "false" but expects true.
		{name: "should_return_false_if_latest_history_has_a_download_id_and_cdh_is_disabled", multi: true, cdhOff: true, history: []acquire.HistoryEvent{grab(1, upgradable, 100*24*time.Hour)}, want: true},
		{
			name: "should_return_false_if_cutoff_already_met_and_cdh_is_disabled", cdhOff: true, bare: release.WEBDL1080p,
			newQuality: model(release.Bluray1080p, 1), history: []acquire.HistoryEvent{grab(1, model(release.WEBDL1080p, 1), 100*24*time.Hour)}, want: false,
		},
		{name: "should_return_false_if_only_episode_is_not_upgradable_and_cdh_is_disabled", cdhOff: true, history: []acquire.HistoryEvent{grab(1, notUpgradable, 100*24*time.Hour)}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			e.Config.CompletedDownloadHandling = !tt.cdhOff
			ports.history = tt.history
			p := &acquire.QualityProfile{UpgradeAllowed: true, Cutoff: acquire.QualityID(release.Bluray1080p), Items: defaultQualities()}
			if tt.bare != 0 {
				p = &acquire.QualityProfile{Cutoff: acquire.QualityID(tt.bare), Items: defaultQualities()}
			}
			episodes := []*Episode{ep(1, 12, 3)}
			if tt.multi {
				episodes = append(episodes, ep(2, 12, 4), ep(3, 12, 5))
			}
			q := tt.newQuality
			if q.Quality == 0 {
				q = model(release.DVD, 2)
			}
			c := testCandidate(testSeries(p), q, episodes...)
			c.Languages = []release.Language{release.English}
			want(t, Root(acquire.History{}), e, c, tt.search, tt.want)
		})
	}
}

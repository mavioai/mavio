package acquire

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestHistory ports Radarr's HistorySpecificationFixture. The release is
// a DVD v2 of movie 1, whose profile allows upgrades to Bluray-1080p,
// unless the case gives another profile; completed download handling is
// on unless the case turns it off. The download IDs of the history
// events play no part, so they are left out.
func TestHistory(t *testing.T) {
	upgradable := upModel(release.SDTV, 1)
	notUpgradable := upModel(release.HDTV1080p, 2)
	grabbed := func(itemID int, q QualityModel, age time.Duration) HistoryEvent {
		return HistoryEvent{Type: HistoryGrabbed, ItemID: tid(itemID), Date: testNow.Add(-age), Quality: q}
	}
	// noUpgrades is the profile the cases that replace the fixture's
	// give: it leaves UpgradeAllowed unset.
	noUpgrades := func(cutoff release.Quality) *QualityProfile { return upProfile(false, cutoff) }

	tests := []struct {
		name       string
		skip       string
		search     *Search
		cdhOff     bool
		profile    *QualityProfile
		newQuality QualityModel
		// items are the IDs of the release's items; nil for movie 1.
		items   []int
		history []HistoryEvent
		want    bool
	}{
		{
			name: "should_return_true_if_it_is_a_search", search: &Search{TargetID: tid(1), ItemIDs: []core.ID{tid(1)}},
			history: []HistoryEvent{grabbed(1, notUpgradable, 0)}, want: true,
		},
		{name: "should_return_true_if_latest_history_item_is_null", want: true},
		{
			name: "should_return_true_if_latest_history_item_is_not_grabbed",
			history: []HistoryEvent{{
				Type: HistoryDownloadFailed, ItemID: tid(1), Date: testNow, Quality: notUpgradable,
			}},
			want: true,
		},
		{
			name: "should_return_true_if_latest_history_has_a_download_id_and_cdh_is_enabled",
			skip: "commented out in Radarr: a multi-episode case whose expectation predates the 12-hour check",
		},
		{
			name:    "should_return_true_if_latest_history_item_is_older_than_twelve_hours",
			history: []HistoryEvent{grabbed(1, notUpgradable, 13*time.Hour)}, want: true,
		},
		{
			name:    "should_be_upgradable_if_only_episode_is_upgradable",
			history: []HistoryEvent{grabbed(1, upgradable, 0)}, want: true,
		},
		// The four multi-episode cases below are commented out in Radarr,
		// which has one item per release; the engine checks every item, as
		// Sonarr does, so they run here.
		{
			name: "should_be_upgradable_if_both_episodes_are_upgradable", items: []int{1, 2},
			history: []HistoryEvent{grabbed(1, upgradable, 0), grabbed(2, upgradable, 0)}, want: true,
		},
		{
			name: "should_not_be_upgradable_if_both_episodes_are_not_upgradable", items: []int{1, 2},
			history: []HistoryEvent{grabbed(1, notUpgradable, 0), grabbed(2, notUpgradable, 0)}, want: false,
		},
		{
			// Radarr's case gives the first episode twice, the second
			// overriding the first: the first is not upgradable and the
			// second has no history.
			name: "should_be_not_upgradable_if_only_first_episodes_is_upgradable", items: []int{1, 2},
			history: []HistoryEvent{grabbed(1, notUpgradable, 0)}, want: false,
		},
		{
			name: "should_be_not_upgradable_if_only_second_episodes_is_upgradable", items: []int{1, 2},
			history: []HistoryEvent{grabbed(1, notUpgradable, 0), grabbed(2, upgradable, 0)}, want: false,
		},
		{
			name: "should_not_be_upgradable_if_episode_is_of_same_quality_as_existing", profile: noUpgrades(release.Bluray1080p),
			newQuality: upModel(release.WEBDL1080p, 1),
			history:    []HistoryEvent{grabbed(1, upModel(release.WEBDL1080p, 1), 0)}, want: false,
		},
		{
			name: "should_not_be_upgradable_if_cutoff_already_met", profile: noUpgrades(release.WEBDL1080p),
			newQuality: upModel(release.WEBDL1080p, 1),
			history:    []HistoryEvent{grabbed(1, upModel(release.Bluray1080p, 1), 0)}, want: false,
		},
		{
			name:    "should_return_false_if_latest_history_item_is_only_one_hour_old",
			history: []HistoryEvent{grabbed(1, notUpgradable, time.Hour)}, want: false,
		},
		{
			// Despite its name, Radarr's case expects acceptance.
			name: "should_return_false_if_latest_history_has_a_download_id_and_cdh_is_disabled", cdhOff: true,
			history: []HistoryEvent{grabbed(1, upgradable, 100*24*time.Hour)}, want: true,
		},
		{
			name: "should_return_false_if_cutoff_already_met_and_cdh_is_disabled", cdhOff: true, profile: noUpgrades(release.WEBDL1080p),
			newQuality: upModel(release.Bluray1080p, 1),
			history:    []HistoryEvent{grabbed(1, upModel(release.WEBDL1080p, 1), 100*24*time.Hour)}, want: false,
		},
		{
			name: "should_return_false_if_only_episode_is_not_upgradable_and_cdh_is_disabled", cdhOff: true,
			history: []HistoryEvent{grabbed(1, notUpgradable, 100*24*time.Hour)}, want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			e, ports := newTestEnv()
			e.Config.CompletedDownloadHandling = !tt.cdhOff
			ports.history = tt.history
			p := tt.profile
			if p == nil {
				p = upProfile(true, release.Bluray1080p)
			}
			newQuality := tt.newQuality
			if newQuality == (QualityModel{}) {
				newQuality = upModel(release.DVD, 2)
			}
			c := testMovie(p, newQuality.Quality)
			c.Quality = newQuality
			if tt.items != nil {
				c.Items = nil
				for _, id := range tt.items {
					c.Items = append(c.Items, &Item{ID: tid(id), Monitored: true})
				}
			}
			r := check(t, History{}, e, c, tt.search)
			if got := r == nil; got != tt.want {
				t.Errorf("History accepted got = %v (%v), want = %v", got, r, tt.want)
			}
		})
	}
}

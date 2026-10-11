package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestMonitoredEpisode ports Sonarr's MonitoredEpisodeSpecificationFixture.
func TestMonitoredEpisode(t *testing.T) {
	tests := []struct {
		name                       string
		seriesUnmonitored          bool
		firstOff, secondOff, multi bool
		fullSeason                 bool
		search                     *Search
		want                       bool
	}{
		{name: "setup_should_return_monitored_episode_should_return_true(single)", want: true},
		{name: "setup_should_return_monitored_episode_should_return_true(multi)", multi: true, want: true},
		{name: "not_monitored_series_should_be_skipped", seriesUnmonitored: true, multi: true, want: false},
		{name: "only_episode_not_monitored_should_return_false", firstOff: true, want: false},
		{name: "both_episodes_not_monitored_should_return_false", firstOff: true, secondOff: true, multi: true, want: false},
		{name: "only_first_episode_not_monitored_should_return_false", firstOff: true, multi: true, want: false},
		{name: "only_second_episode_not_monitored_should_return_false", secondOff: true, multi: true, want: false},
		{name: "should_return_true_for_single_episode_search", seriesUnmonitored: true, search: &Search{Kind: SingleEpisodeSearch}, want: true},
		{name: "should_return_true_if_episode_is_monitored_for_season_search", search: &Search{Kind: SeasonSearch}, want: true},
		{name: "should_return_false_if_episode_is_not_monitored_for_season_search", firstOff: true, search: &Search{Kind: SeasonSearch, MonitoredEpisodesOnly: true}, want: false},
		{name: "should_return_true_if_episode_is_not_monitored_and_monitoredEpisodesOnly_flag_is_false", firstOff: true, search: &Search{Kind: SingleEpisodeSearch}, want: true},
		{name: "should_return_false_if_episode_is_not_monitored_and_monitoredEpisodesOnly_flag_is_true", firstOff: true, search: &Search{Kind: SingleEpisodeSearch, MonitoredEpisodesOnly: true}, want: false},
		{name: "should_return_false_if_all_episodes_are_not_monitored_for_season_pack_release", secondOff: true, multi: true, fullSeason: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			s := testSeries(testProfile(release.HDTV720p, release.HDTV720p))
			s.Monitored = !tt.seriesUnmonitored
			first, second := ep(1, 1, 1), ep(2, 1, 2)
			first.Monitored, second.Monitored = !tt.firstOff, !tt.secondOff
			episodes := []*Episode{first}
			if tt.multi {
				episodes = append(episodes, second)
			}
			c := testCandidate(s, model(release.HDTV720p, 1), episodes...)
			c.Parsed.FullSeason = tt.fullSeason
			want(t, MonitoredEpisode{}, e, c, tt.search, tt.want)
		})
	}
}

// TestAlreadyImported ports Sonarr's AlreadyImportedSpecificationFixture.
// The release is Builder's: its title is "Title1", so only the torrent
// hash can match the grabbed one.
func TestAlreadyImported(t *testing.T) {
	const title = "Series.Title.S01E01.720p.HDTV.x264-Sonarr"
	const downloadID = "35B5A5B4-6E1F-4C1A-9E68-2D7C8B6F1A2D"
	hdtv720p, hdtv1080p := model(release.HDTV720p, 1), model(release.HDTV1080p, 1)
	grabbed := func(id string, q acquire.QualityModel) acquire.HistoryEvent {
		return acquire.HistoryEvent{Type: acquire.HistoryGrabbed, ItemID: tid(1), Date: testNow, DownloadID: id, SourceTitle: title, Quality: q}
	}
	imported := func(id string, q acquire.QualityModel) acquire.HistoryEvent {
		return acquire.HistoryEvent{Type: acquire.HistoryImported, ItemID: tid(1), Date: testNow, DownloadID: id, SourceTitle: title, Quality: q}
	}
	tests := []struct {
		name     string
		cdhOff   bool
		noFile   bool
		history  []acquire.HistoryEvent
		torrent  bool
		infoHash string
		want     bool
	}{
		{name: "should_be_accepted_if_CDH_is_disabled", cdhOff: true, want: true},
		{name: "should_be_accepted_if_episode_does_not_have_a_file", noFile: true, want: true},
		{name: "should_be_accepted_if_episode_does_not_have_grabbed_event", want: true},
		{
			name:    "should_be_accepted_if_episode_does_not_have_imported_event",
			history: []acquire.HistoryEvent{grabbed(downloadID, hdtv720p)}, want: true,
		},
		{
			name:    "should_be_accepted_if_grabbed_and_imported_quality_is_the_same",
			history: []acquire.HistoryEvent{grabbed(downloadID, hdtv720p), imported(downloadID, hdtv720p)}, want: true,
		},
		{
			name:    "should_be_accepted_if_grabbed_download_id_and_release_torrent_hash_is_unknown",
			history: []acquire.HistoryEvent{grabbed(downloadID, hdtv720p), imported(downloadID, hdtv1080p)},
			torrent: true, want: true,
		},
		{
			name:    "should_be_accepted_if_grabbed_download_does_not_have_an_id",
			history: []acquire.HistoryEvent{grabbed("", hdtv720p), imported(downloadID, hdtv1080p)},
			torrent: true, infoHash: downloadID, want: true,
		},
		{
			name:    "should_be_rejected_if_grabbed_download_id_matches_release_torrent_hash",
			history: []acquire.HistoryEvent{grabbed(downloadID, hdtv720p), imported(downloadID, hdtv1080p)},
			torrent: true, infoHash: downloadID, want: false,
		},
		{
			name:    "should_be_rejected_if_release_title_matches_grabbed_event_source_title",
			history: []acquire.HistoryEvent{grabbed(downloadID, hdtv720p), imported(downloadID, hdtv1080p)},
			torrent: true, infoHash: downloadID, want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			e.Config.CompletedDownloadHandling = !tt.cdhOff
			ports.history = tt.history
			episode := ep(1, 12, 3)
			if !tt.noFile {
				episode.File = file(1, hdtv720p)
			}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), hdtv720p, episode)
			c.Release.Title = "Title1"
			if tt.torrent {
				c.Release.Protocol = acquire.ProtocolTorrent
				c.Release.InfoHash = tt.infoHash
			}
			want(t, AlreadyImported{}, e, c, nil, tt.want)
		})
	}
}

package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// TestAlreadyImportedSpec ports Radarr's
// AlreadyImportedSpecificationFixture. History is newest first, so the
// import comes before the grab.
func TestAlreadyImportedSpec(t *testing.T) {
	const (
		title      = "Movie.Title.2018.720p.HDTV.x264-Radarr"
		downloadID = "C7A4F2E8-3B1D-4F6A-9E0C-5D2B8A1F7E34"
		// builderTitle is the title NBuilder gives Radarr's releases.
		builderTitle = "Title1"
	)
	hdtv720p := NewQualityModel(release.HDTV720p)
	hdtv1080p := NewQualityModel(release.HDTV1080p)
	grabbed := func(id string) HistoryEvent {
		return HistoryEvent{Type: HistoryGrabbed, ItemID: tid(1), Date: testNow, DownloadID: id, SourceTitle: title, Quality: hdtv720p}
	}
	imported := func(q QualityModel) HistoryEvent {
		return HistoryEvent{Type: HistoryImported, ItemID: tid(1), Date: testNow, DownloadID: downloadID, SourceTitle: title, Quality: q}
	}
	torrent := &Release{GUID: "guid", Title: builderTitle, Size: 1 << 30, Protocol: ProtocolTorrent, IndexerID: ixID(1), InfoHash: downloadID}
	tests := []struct {
		name    string
		cdh     bool
		noFile  bool
		history []HistoryEvent
		release *Release
		want    bool
	}{
		{"should_be_accepted_if_CDH_is_disabled", false, false, nil, nil, true},
		{"should_be_accepted_if_movie_does_not_have_a_file", true, true, nil, nil, true},
		{"should_be_accepted_if_movie_does_not_have_grabbed_event", true, false, nil, nil, true},
		{"should_be_accepted_if_movie_does_not_have_imported_event", true, false, []HistoryEvent{grabbed(downloadID)}, nil, true},
		{
			"should_be_accepted_if_grabbed_and_imported_quality_is_the_same", true, false,
			[]HistoryEvent{imported(hdtv720p), grabbed(downloadID)},
			nil, true,
		},
		{
			"should_be_accepted_if_grabbed_download_id_and_release_torrent_hash_is_unknown", true, false,
			[]HistoryEvent{imported(hdtv1080p), grabbed(downloadID)},
			&Release{GUID: "guid", Title: builderTitle, Protocol: ProtocolTorrent, IndexerID: ixID(1)}, true,
		},
		{
			"should_be_accepted_if_grabbed_download_does_not_have_an_id", true, false,
			[]HistoryEvent{imported(hdtv1080p), grabbed("")},
			torrent, true,
		},
		{
			"should_be_rejected_if_grabbed_download_id_matches_release_torrent_hash", true, false,
			[]HistoryEvent{imported(hdtv1080p), grabbed(downloadID)},
			torrent, false,
		},
		// Radarr's case gives the release NBuilder's title and the grab's
		// hash, so it is rejected for the hash; "(by title)" checks the
		// title it is named for.
		{
			"should_be_rejected_if_release_title_matches_grabbed_event_source_title", true, false,
			[]HistoryEvent{imported(hdtv1080p), grabbed(downloadID)},
			torrent, false,
		},
		{
			"should_be_rejected_if_release_title_matches_grabbed_event_source_title (by title)", true, false,
			[]HistoryEvent{imported(hdtv1080p), grabbed(downloadID)},
			&Release{GUID: "guid", Title: "movie.title.2018.720p.hdtv.x264-radarr", Protocol: ProtocolUsenet, IndexerID: ixID(1)}, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports := newTestEnv()
			e.Config.CompletedDownloadHandling = tt.cdh
			ports.history = tt.history
			c := testMovie(testProfile(release.HDTV1080p, release.HDTV720p, release.HDTV1080p), release.HDTV720p)
			c.Release.Title = title
			if tt.release != nil {
				c.Release = tt.release
			}
			if !tt.noFile {
				c.Items[0].File = &File{ID: tid(1), Quality: hdtv720p}
			}
			spWant(t, AlreadyImported{}, e, c, nil, tt.want)
		})
	}
}

// TestRepackSpec ports Radarr's RepackSpecificationFixture. The release is
// SDTV v2 of the group Radarr.
func TestRepackSpec(t *testing.T) {
	tests := []struct {
		name         string
		propers      ProperDownload
		repack       bool
		releaseGroup string
		file         *File
		want         bool
	}{
		{"should_return_true_if_it_is_not_a_repack", PropersPreferAndUpgrade, false, "Radarr", nil, true},
		{"should_return_true_if_there_are_is_no_movie_file", PropersPreferAndUpgrade, true, "Radarr", nil, true},
		{
			"should_return_true_if_is_a_repack_for_a_different_quality", PropersPreferAndUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.DVD), ReleaseGroup: "Radarr"}, true,
		},
		{
			"should_return_true_if_is_a_repack_for_existing_file", PropersPreferAndUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "Radarr"}, true,
		},
		{
			"should_return_false_if_is_a_repack_for_a_different_file", PropersPreferAndUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "NotRadarr"}, false,
		},
		{
			"should_return_false_if_release_group_for_existing_file_is_unknown", PropersPreferAndUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: ""}, false,
		},
		{
			"should_return_false_if_release_group_for_release_is_unknown", PropersPreferAndUpgrade, true, "",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "Radarr"}, false,
		},
		{
			"should_return_false_when_repack_but_auto_download_repack_is_false", PropersDoNotUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "Radarr"}, false,
		},
		{
			"should_return_true_when_repack_but_auto_download_repack_is_true", PropersPreferAndUpgrade, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "Radarr"}, true,
		},
		{
			"should_return_true_when_repacks_are_not_preferred", PropersDoNotPrefer, true, "Radarr",
			&File{ID: tid(1), Quality: NewQualityModel(release.SDTV), ReleaseGroup: "Radarr"}, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.Propers = tt.propers
			c := spCandidate()
			c.Quality = QualityModel{Quality: release.SDTV, Revision: release.Revision{Version: 2, IsRepack: tt.repack}}
			c.ReleaseGroup = tt.releaseGroup
			c.Items[0].File = tt.file
			spWant(t, Repack{}, e, c, nil, tt.want)
		})
	}
}

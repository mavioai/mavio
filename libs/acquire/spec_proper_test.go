package acquire

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/release"
)

// TestProperSpec ports Radarr's RssSync/ProperSpecificationFixture, including
// should_return_true_when_propers_are_not_preferred, which lacks the
// [Test] attribute in Radarr and so never ran there.
func TestProperSpec(t *testing.T) {
	today := time.Date(testNow.Year(), testNow.Month(), testNow.Day(), 0, 0, 0, 0, testNow.Location())
	tests := []struct {
		name      string
		propers   ProperDownload
		file      release.Quality
		dateAdded time.Time
		search    *Search
		want      Reason // "" when accepted
	}{
		{
			name:      "should_return_false_when_movieFile_was_added_more_than_7_days_ago",
			file:      release.DVD,
			dateAdded: today.AddDate(0, 0, -30),
			want:      ReasonProperForOldFile,
		},
		{
			name:      "should_return_true_when_movieFile_was_added_more_than_7_days_ago_but_proper_is_for_better_quality",
			file:      release.SDTV,
			dateAdded: today.AddDate(0, 0, -30),
		},
		{
			name:      "should_return_true_when_episodeFile_was_added_more_than_7_days_ago_but_is_for_search",
			file:      release.SDTV,
			dateAdded: today.AddDate(0, 0, -30),
			search:    &Search{TargetID: tid(1)},
		},
		{
			name:      "should_return_false_when_proper_but_auto_download_propers_is_false",
			propers:   PropersDoNotUpgrade,
			file:      release.DVD,
			dateAdded: today,
			want:      ReasonPropersDisabled,
		},
		{
			name:      "should_return_true_when_movieFile_was_added_today",
			propers:   PropersPreferAndUpgrade,
			file:      release.DVD,
			dateAdded: today,
		},
		{
			name:      "should_return_true_when_propers_are_not_preferred",
			propers:   PropersDoNotPrefer,
			file:      release.DVD,
			dateAdded: today,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEnv()
			e.Config.Propers = tt.propers
			p := &QualityProfile{ID: tid(1), Cutoff: QualityID(release.Bluray1080p)}
			c := testMovie(p, release.DVD)
			c.Quality = QualityModel{Quality: release.DVD, Revision: release.Revision{Version: 2}}
			c.Items[0].File = &File{ID: tid(1), Quality: NewQualityModel(tt.file), DateAdded: tt.dateAdded}
			var got Reason
			if r := check(t, Proper{}, e, c, tt.search); r != nil {
				got = r.Reason
			}
			if got != tt.want {
				t.Errorf("rejection = %q, want = %q", got, tt.want)
			}
		})
	}
}

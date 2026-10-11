package series

import (
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// TestIndexerTag ports Sonarr's RssSync/IndexerTagSpecificationFixture.
// Indexer 1 exists, others do not.
func TestIndexerTag(t *testing.T) {
	tests := []struct {
		name                    string
		indexerTags, seriesTags []string
		indexer                 int
		want                    bool
	}{
		{name: "indexer_and_series_without_tags_should_return_true", indexer: 1, want: true},
		{name: "indexer_with_tags_series_without_tags_should_return_false", indexerTags: []string{"123"}, indexer: 1, want: false},
		{name: "indexer_without_tags_series_with_tags_should_return_true", seriesTags: []string{"123"}, indexer: 1, want: true},
		{name: "indexer_with_tags_series_with_matching_tags_should_return_true", indexerTags: []string{"123", "456"}, seriesTags: []string{"123", "789"}, indexer: 1, want: true},
		{name: "indexer_with_tags_series_with_different_tags_should_return_false", indexerTags: []string{"456"}, seriesTags: []string{"123", "789"}, indexer: 1, want: false},
		{name: "release_without_indexerid_should_return_true", indexerTags: []string{"456"}, seriesTags: []string{"123", "789"}, indexer: 0, want: true},
		{name: "release_with_invalid_indexerid_should_return_true", indexerTags: []string{"456"}, seriesTags: []string{"123", "789"}, indexer: 2, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ports, _ := newTestEnv()
			ports.indexers = map[string]*acquire.IndexerInfo{ixID(1): {ID: ixID(1), Tags: tt.indexerTags}}
			s := testSeries(testProfile(release.HDTV720p, release.HDTV720p))
			s.Tags = tt.seriesTags
			c := testCandidate(s, model(release.HDTV720p, 1), ep(1, 1, 1), ep(2, 1, 2))
			c.Release.IndexerID = ixID(tt.indexer)
			r := check(t, Root(acquire.IndexerTag{}), e, c, &Search{Kind: SingleEpisodeSearch, MonitoredEpisodesOnly: true})
			if got := r == nil; got != tt.want {
				t.Errorf("accepted = %v (%+v), want = %v", got, r, tt.want)
			}
			if r != nil && r.Message != "Series tags do not match any of the indexer tags" {
				t.Errorf("message = %q, want = %q", r.Message, "Series tags do not match any of the indexer tags")
			}
		})
	}
}

// TestDeletedEpisodeFile ports Sonarr's
// RssSync/DeletedEpisodeFileSpecificationFixture.
func TestDeletedEpisodeFile(t *testing.T) {
	tests := []struct {
		name     string
		off      bool
		search   *Search
		multi    bool
		existing []int
		want     bool
	}{
		{name: "should_return_true_when_unmonitor_deleted_episdes_is_off", off: true, want: true},
		{name: "should_return_true_when_searching", search: &Search{Kind: SeasonSearch}, want: true},
		{name: "should_return_true_if_file_exists", existing: []int{1}, want: true},
		{name: "should_return_false_if_file_is_missing", want: false},
		{name: "should_return_true_if_both_of_multiple_episode_exist", multi: true, existing: []int{1, 2}, want: true},
		{name: "should_return_false_if_one_of_multiple_episode_is_missing", multi: true, existing: []int{1}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, eps := newTestEnv()
			e.AutoUnmonitorPreviouslyDownloaded = !tt.off
			eps.missing = []core.ID{tid(1), tid(2)}
			for _, n := range tt.existing {
				eps.missing = removeID(eps.missing, tid(n))
			}
			p := &acquire.QualityProfile{Cutoff: acquire.QualityID(release.Bluray1080p)}
			first, second := ep(1, 1, 1), ep(2, 1, 2)
			first.File = file(1, model(release.Bluray1080p, 1))
			second.File = file(2, model(release.Bluray1080p, 1))
			episodes := []*Episode{first}
			if tt.multi {
				episodes = append(episodes, second)
			}
			c := testCandidate(testSeries(p), model(release.DVD, 2), episodes...)
			spec := DeletedEpisodeFile{}
			want(t, spec, e, c, tt.search, tt.want)
			if spec.Priority() != acquire.PriorityDisk || !spec.Temporary() {
				t.Errorf("Priority(), Temporary() = %v, %v, want = PriorityDisk, true", spec.Priority(), spec.Temporary())
			}
		})
	}
}

func removeID(ids []core.ID, id core.ID) []core.ID {
	var out []core.ID
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}

// TestProper ports Sonarr's RssSync/ProperSpecificationFixture. The
// release is a DVD v2; both files are Bluray-1080p v1 added now unless
// the case changes them, and an episode without a file goes along.
func TestProper(t *testing.T) {
	today := time.Date(testNow.Year(), testNow.Month(), testNow.Day(), 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name                string
		propers             acquire.ProperDownload
		multi               bool
		first, second       release.Quality
		firstAdd, secondAdd time.Time
		search              *Search
		want                bool
	}{
		{name: "should_return_false_when_episodeFile_was_added_more_than_7_days_ago", first: release.DVD, firstAdd: today.AddDate(0, 0, -30), want: false},
		{name: "should_return_false_when_first_episodeFile_was_added_more_than_7_days_ago", multi: true, first: release.DVD, second: release.DVD, firstAdd: today.AddDate(0, 0, -30), want: false},
		{name: "should_return_false_when_second_episodeFile_was_added_more_than_7_days_ago", multi: true, first: release.DVD, second: release.DVD, secondAdd: today.AddDate(0, 0, -30), want: false},
		{name: "should_return_true_when_episodeFile_was_added_more_than_7_days_ago_but_proper_is_for_better_quality", first: release.SDTV, firstAdd: today.AddDate(0, 0, -30), want: true},
		{name: "should_return_true_when_episodeFile_was_added_more_than_7_days_ago_but_is_for_search", first: release.SDTV, firstAdd: today.AddDate(0, 0, -30), search: &Search{Kind: SingleEpisodeSearch}, want: true},
		{name: "should_return_false_when_proper_but_auto_download_propers_is_false", propers: acquire.PropersDoNotUpgrade, first: release.DVD, firstAdd: today, want: false},
		{name: "should_return_true_when_episodeFile_was_added_today", propers: acquire.PropersPreferAndUpgrade, first: release.DVD, firstAdd: today, want: true},
		{name: "should_return_true_when_propers_are_not_preferred", propers: acquire.PropersDoNotPrefer, first: release.DVD, firstAdd: today, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			if tt.propers != 0 {
				e.Config.Propers = tt.propers
			}
			newFile := func(q release.Quality, added time.Time) *acquire.File {
				if q == 0 {
					q = release.Bluray1080p
				}
				if added.IsZero() {
					added = testNow
				}
				f := file(1, model(q, 1))
				f.DateAdded = added
				return f
			}
			first, second, none := ep(1, 1, 1), ep(2, 1, 2), ep(3, 1, 3)
			first.File = newFile(tt.first, tt.firstAdd)
			second.File = newFile(tt.second, tt.secondAdd)
			episodes := []*Episode{first, none}
			if tt.multi {
				episodes = []*Episode{first, second, none}
			}
			p := &acquire.QualityProfile{Cutoff: acquire.QualityID(release.Bluray1080p)}
			c := testCandidate(testSeries(p), model(release.DVD, 2), episodes...)
			want(t, Root(acquire.Proper{}), e, c, tt.search, tt.want)
		})
	}
}

// TestReleaseRestrictions ports Sonarr's
// ReleaseRestrictionsSpecificationFixture.
func TestReleaseRestrictions(t *testing.T) {
	const dexter = "Dexter.S08E01.EDITED.WEBRip.x264-KYR"
	tests := []struct {
		name              string
		title             string
		required, ignored []string
		none              bool
		want              bool
	}{
		{name: "should_be_true_when_restrictions_are_empty", none: true, want: true},
		{name: "should_be_true_when_title_contains_one_required_term", required: []string{"WEBRip"}, want: true},
		{name: "should_be_false_when_title_does_not_contain_any_required_terms", required: []string{"doesnt", "exist"}, want: false},
		{name: "should_be_true_when_title_does_not_contain_any_ignored_terms", ignored: []string{"ignored"}, want: true},
		{name: "should_be_false_when_title_contains_one_anded_ignored_terms", ignored: []string{"edited"}, want: false},
		{name: "should_ignore_case_when_matching_required(EdiTED)", required: []string{"EdiTED"}, want: true},
		{name: "should_ignore_case_when_matching_required(webrip)", required: []string{"webrip"}, want: true},
		{name: "should_ignore_case_when_matching_required(X264)", required: []string{"X264"}, want: true},
		{name: "should_ignore_case_when_matching_required(X264,NOTTHERE)", required: strings.Split("X264,NOTTHERE", ","), want: true},
		{name: "should_ignore_case_when_matching_ignored(EdiTED)", ignored: []string{"EdiTED"}, want: false},
		{name: "should_ignore_case_when_matching_ignored(webrip)", ignored: []string{"webrip"}, want: false},
		{name: "should_ignore_case_when_matching_ignored(X264)", ignored: []string{"X264"}, want: false},
		{name: "should_ignore_case_when_matching_ignored(X264,NOTTHERE)", ignored: strings.Split("X264,NOTTHERE", ","), want: false},
		{
			name:     "should_be_false_when_release_contains_one_restricted_word_and_one_required_word",
			title:    "[ www.Speed.cd ] -Whose.Line.is.it.Anyway.US.S10E24.720p.HDTV.x264-BAJSKORV",
			required: []string{"x264"}, ignored: []string{"www.Speed.cd"}, want: false,
		},
		{name: "should_match_perl_regex(/WEB/)", required: []string{"/WEB/"}, want: true},
		// C#'s "\b" in a regular string is a backspace, as in Go.
		{name: "should_match_perl_regex(/WEB\\b/)", required: []string{"/WEB\b/"}, want: false},
		{name: "should_match_perl_regex(/WEb/)", required: []string{"/WEb/"}, want: false},
		{name: "should_match_perl_regex(/\\.WEB/)", required: []string{`/\.WEB/`}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			if !tt.none {
				e.ReleaseProfiles = []acquire.ReleaseProfile{{Enabled: true, Required: tt.required, Ignored: tt.ignored}}
			}
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.WEBRip720p, 1), ep(1, 8, 1))
			c.Release.Title = dexter
			if tt.title != "" {
				c.Release.Title = tt.title
			}
			want(t, Root(acquire.ReleaseRestrictions{}), e, c, nil, tt.want)
		})
	}
}

package series

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// fakeSpec is a specification that always gives the same answer and
// counts its calls, as the mocked specifications of Sonarr's
// DownloadDecisionMakerFixture.
type fakeSpec struct {
	name     string
	priority acquire.Priority
	reason   acquire.Reason // empty to accept
	calls    int
}

func (f *fakeSpec) Name() string               { return f.name }
func (f *fakeSpec) Priority() acquire.Priority { return f.priority }
func (f *fakeSpec) Temporary() bool            { return false }
func (f *fakeSpec) Check(context.Context, *Env, *Candidate, *Search) (*acquire.Rejection, error) {
	f.calls++
	if f.reason == "" {
		return nil, nil
	}
	return &acquire.Rejection{Reason: f.reason, Message: f.name}, nil
}

// TestDecideEpisodes ports Sonarr's DownloadDecisionMakerFixture. The
// release is The.Office.S03E115.DVDRip.XviD-OSiTV unless the case gives
// another title; it maps to a series with one episode unless the case
// changes the match. Sonarr's ParsingService.Map is the FindEpisodes
// callback, which counts its calls.
func TestDecideEpisodes(t *testing.T) {
	const office = "The.Office.S03E115.DVDRip.XviD-OSiTV"
	errBroken := errors.New("broken")
	tests := []struct {
		name      string
		specs     []string // "pass", "fail" or "failDelayed", numbered in order
		titles    []string
		search    *Search
		noSeries  bool
		alias     int
		noEpisode bool
		findErr   bool
		// want
		decisions     int
		approved      *bool
		rejections    int
		findCalls     int
		specCalls     map[int]int // calls of the specs at these positions
		downloadAllow *bool
		message       string // a rejection message contains it
	}{
		{name: "should_call_all_specifications", specs: []string{"pass", "pass", "pass", "fail", "fail", "fail"}, decisions: 1, findCalls: 1, specCalls: map[int]int{0: 1, 1: 1, 2: 1, 3: 1, 4: 1, 5: 1}},
		{name: "should_call_delayed_specifications_if_non_delayed_passed", specs: []string{"pass", "failDelayed"}, decisions: 1, findCalls: 1, specCalls: map[int]int{1: 1}},
		{name: "should_not_call_delayed_specifications_if_non_delayed_failed", specs: []string{"fail", "failDelayed"}, decisions: 1, findCalls: 1, specCalls: map[int]int{1: 0}},
		{name: "should_return_rejected_if_single_specs_fail", specs: []string{"fail"}, decisions: 1, findCalls: 1, approved: new(false)},
		{name: "should_return_rejected_if_one_of_specs_fail", specs: []string{"pass", "fail", "pass", "pass"}, decisions: 1, findCalls: 1, approved: new(false)},
		{name: "should_return_pass_if_all_specs_pass", specs: []string{"pass", "pass", "pass"}, decisions: 1, findCalls: 1, approved: new(true)},
		{name: "should_have_same_number_of_rejections_as_specs_that_failed", specs: []string{"pass", "pass", "pass", "fail", "fail", "fail"}, decisions: 1, findCalls: 1, rejections: 3},
		// Sonarr's RSS sync keeps no decision on a title that does not
		// parse; neither does DecideEpisodes.
		{name: "should_not_attempt_to_map_episode_if_not_parsable", specs: []string{"pass", "pass", "pass"}, titles: []string{"Not parsable"}, decisions: 0, specCalls: map[int]int{0: 0, 1: 0, 2: 0}},
		{name: "should_not_attempt_to_map_episode_if_series_title_is_blank", specs: []string{"pass", "pass", "pass"}, titles: []string{"1937 - Snow White and the Seven Dwarves"}, decisions: 0, specCalls: map[int]int{0: 0, 1: 0, 2: 0}},
		{
			name: "should_return_rejected_result_for_unparsable_search", specs: []string{"pass", "pass", "pass"}, titles: []string{"1937 - Snow White and the Seven Dwarves"},
			search: &Search{Kind: SingleEpisodeSearch}, decisions: 1, approved: new(false), specCalls: map[int]int{0: 0, 1: 0, 2: 0},
		},
		{name: "should_not_attempt_to_make_decision_if_series_is_unknown", specs: []string{"pass", "pass", "pass"}, noSeries: true, decisions: 1, findCalls: 1, specCalls: map[int]int{0: 0, 1: 0, 2: 0}},
		{name: "broken_report_shouldnt_blowup_the_process", specs: []string{"pass"}, titles: []string{office, office, office}, findErr: true, decisions: 3, findCalls: 3},
		{name: "should_return_unknown_series_rejection_if_series_is_unknown", specs: []string{"pass", "pass", "pass"}, noSeries: true, decisions: 1, findCalls: 1},
		{name: "should_not_allow_download_if_series_is_unknown", specs: []string{"pass", "pass", "pass"}, noSeries: true, decisions: 1, findCalls: 1, downloadAllow: new(false)},
		{name: "should_not_allow_download_if_no_episodes_found", specs: []string{"pass", "pass", "pass"}, noEpisode: true, decisions: 1, findCalls: 1, downloadAllow: new(false)},
		{name: "should_return_a_decision_when_exception_is_caught", specs: []string{"pass"}, findErr: true, decisions: 1, findCalls: 1},
		{
			name: "should_return_unknown_series_rejection_if_series_title_is_an_alias_for_another_series", specs: []string{"pass", "pass", "pass"},
			noSeries: true, alias: 12345, decisions: 1, findCalls: 1, message: "12345",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _, _ := newTestEnv()
			var fakes []*fakeSpec
			var all []Spec
			for i, kind := range tt.specs {
				f := &fakeSpec{name: fmt.Sprintf("%s%d", kind, i)}
				switch kind {
				case "fail":
					f.reason = acquire.ReasonUnknown
				case "failDelayed":
					f.reason, f.priority = acquire.ReasonMinimumAgeDelay, acquire.PriorityDisk
				}
				fakes = append(fakes, f)
				all = append(all, f)
			}
			titles := tt.titles
			if titles == nil {
				titles = []string{office}
			}
			var releases []*acquire.Release
			for _, title := range titles {
				releases = append(releases, &acquire.Release{Title: title})
			}
			calls := 0
			find := func(context.Context, *release.ParsedEpisodeInfo, *acquire.Release, *Search) (*Match, error) {
				calls++
				if tt.findErr {
					return nil, errBroken
				}
				m := &Match{Series: testSeries(testProfile(release.DVD, release.DVD)), Episodes: []*Episode{ep(1, 3, 115)}, AliasTvdbID: tt.alias}
				if tt.noSeries {
					m.Series = nil
				}
				if tt.noEpisode {
					m.Episodes = nil
				}
				return m, nil
			}
			e := &Engine{Env: env, Specs: all}
			got := e.DecideEpisodes(context.Background(), releases, tt.search, false, find, nil)
			if len(got) != tt.decisions {
				t.Fatalf("len(DecideEpisodes()) = %d, want = %d", len(got), tt.decisions)
			}
			if calls != tt.findCalls {
				t.Errorf("FindEpisodes calls = %d, want = %d", calls, tt.findCalls)
			}
			for i, n := range tt.specCalls {
				if fakes[i].calls != n {
					t.Errorf("%s calls = %d, want = %d", fakes[i].name, fakes[i].calls, n)
				}
			}
			if tt.approved != nil && got[0].Approved() != *tt.approved {
				t.Errorf("Approved() = %v, want = %v", got[0].Approved(), *tt.approved)
			}
			if tt.rejections != 0 && len(got[0].Rejections) != tt.rejections {
				t.Errorf("len(Rejections) = %d, want = %d", len(got[0].Rejections), tt.rejections)
			}
			if tt.downloadAllow != nil && got[0].Candidate.DownloadAllowed != *tt.downloadAllow {
				t.Errorf("DownloadAllowed = %v, want = %v", got[0].Candidate.DownloadAllowed, *tt.downloadAllow)
			}
			if tt.message != "" && (len(got[0].Rejections) == 0 || !strings.Contains(got[0].Rejections[0].Message, tt.message)) {
				t.Errorf("Rejections = %+v, want = a message containing %q", got[0].Rejections, tt.message)
			}
		})
	}
}

// TestDecideEpisodesRequested ports
// should_only_include_reports_for_requested_episodes of Sonarr's
// DownloadDecisionMakerFixture: a season search for one of two episodes
// whose scene numbering puts them in season 2.
func TestDecideEpisodesRequested(t *testing.T) {
	env, _, _ := newTestEnv()
	s := testSeries(testProfile(release.WEBDL720p, release.WEBDL720p))
	s.Title = "Title1"
	episodes := []*Episode{ep(1, 1, 1), ep(2, 1, 2)}
	var releases []*acquire.Release
	for i := range episodes {
		releases = append(releases, &acquire.Release{Title: fmt.Sprintf("%s.S%02dE%02d.720p.WEB-DL-DRONE", s.Title, 2, i+1)})
	}
	find := func(_ context.Context, p *release.ParsedEpisodeInfo, _ *acquire.Release, _ *Search) (*Match, error) {
		return &Match{Series: s, Episodes: []*Episode{episodes[p.EpisodeNumbers[0]-1]}}, nil
	}
	e := &Engine{Env: env, Specs: []Spec{EpisodeRequested{}}}
	search := &Search{Kind: SeasonSearch, Series: s, Episodes: episodes[:1], SeasonNumber: 1}
	approved := 0
	for _, d := range e.DecideEpisodes(context.Background(), releases, search, false, find, nil) {
		if d.Approved() {
			approved++
		}
	}
	if approved != 1 {
		t.Errorf("approved = %d, want = 1", approved)
	}
}

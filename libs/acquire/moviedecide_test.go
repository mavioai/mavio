package acquire

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// rkSpec is a specification that accepts or rejects every candidate and
// counts its calls, as the mocked specifications of Radarr's
// DownloadDecisionMakerFixture.
type rkSpec struct {
	name   string
	reject bool
	calls  int
}

func (s *rkSpec) Name() string       { return s.name }
func (s *rkSpec) Priority() Priority { return PriorityDefault }
func (s *rkSpec) Temporary() bool    { return false }

func (s *rkSpec) Check(context.Context, *Env, *Candidate, *Search) (*Rejection, error) {
	s.calls++
	if s.reject {
		return reject(ReasonUnknown, "%s", s.name), nil
	}
	return nil, nil
}

// rkFinder is the FindMovie of the decision maker tests, as Radarr's
// mocked ParsingService.Map.
type rkFinder struct {
	target *Target
	err    error
	calls  int
}

func (f *rkFinder) find(context.Context, *release.ParsedMovieInfo, *Release, *Search) (*Target, *Item, error) {
	f.calls++
	if f.err != nil || f.target == nil {
		return nil, nil, f.err
	}
	return f.target, &Item{ID: f.target.ID, Monitored: true}, nil
}

const rkTrolls = "Trolls.2016.720p.WEB-DL.DD5.1.H264-FGT"

// TestDecideMovies ports Radarr's DownloadDecisionMakerFixture.
func TestDecideMovies(t *testing.T) {
	type specs struct{ pass1, pass2, pass3, fail1, fail2, fail3 *rkSpec }
	newSpecs := func() specs {
		return specs{
			pass1: &rkSpec{name: "pass1"}, pass2: &rkSpec{name: "pass2"}, pass3: &rkSpec{name: "pass3"},
			fail1: &rkSpec{name: "fail1", reject: true}, fail2: &rkSpec{name: "fail2", reject: true}, fail3: &rkSpec{name: "fail3", reject: true},
		}
	}
	type fixture struct {
		specs  specs
		finder *rkFinder
		engine *Engine
	}
	setup := func(pick func(s specs) []*rkSpec) *fixture {
		e, _ := newTestEnv()
		f := &fixture{specs: newSpecs(), finder: &rkFinder{target: &Target{ID: tid(1), Kind: release.Movie}}}
		f.engine = &Engine{Env: e}
		for _, s := range pick(f.specs) {
			f.engine.Specs = append(f.engine.Specs, s)
		}
		return f
	}
	all := func(s specs) []*rkSpec { return []*rkSpec{s.pass1, s.pass2, s.pass3, s.fail1, s.fail2, s.fail3} }
	passing := func(s specs) []*rkSpec { return []*rkSpec{s.pass1, s.pass2, s.pass3} }
	rss := func(f *fixture, titles ...string) []Decision {
		var releases []*Release
		for _, title := range titles {
			releases = append(releases, &Release{Title: title})
		}
		return f.engine.DecideMovies(context.Background(), releases, nil, false, f.finder.find)
	}
	wantCalls := func(t *testing.T, want int, ss ...*rkSpec) {
		t.Helper()
		for _, s := range ss {
			if s.calls != want {
				t.Errorf("%s calls = %d, want = %d", s.name, s.calls, want)
			}
		}
	}

	t.Run("should_call_all_specifications", func(t *testing.T) {
		f := setup(all)
		rss(f, rkTrolls)
		wantCalls(t, 1, all(f.specs)...)
	})

	approvalTests := []struct {
		name string
		pick func(s specs) []*rkSpec
		want bool
	}{
		{"should_return_rejected_if_single_specs_fail", func(s specs) []*rkSpec { return []*rkSpec{s.fail1} }, false},
		{"should_return_rejected_if_one_of_specs_fail", func(s specs) []*rkSpec { return []*rkSpec{s.pass1, s.fail1, s.pass2, s.pass3} }, false},
		{"should_return_pass_if_all_specs_pass", passing, true},
	}
	for _, tt := range approvalTests {
		t.Run(tt.name, func(t *testing.T) {
			got := rss(setup(tt.pick), rkTrolls)
			if len(got) != 1 {
				t.Fatalf("len(decisions) = %d, want = 1", len(got))
			}
			if got[0].Approved() != tt.want {
				t.Errorf("Approved() = %v, want = %v", got[0].Approved(), tt.want)
			}
		})
	}

	t.Run("should_have_same_number_of_rejections_as_specs_that_failed", func(t *testing.T) {
		got := rss(setup(all), rkTrolls)
		if len(got) != 1 {
			t.Fatalf("len(decisions) = %d, want = 1", len(got))
		}
		if len(got[0].Rejections) != 3 {
			t.Errorf("len(Rejections) = %d, want = 3", len(got[0].Rejections))
		}
	})

	t.Run("should_not_attempt_to_map_episode_if_not_parsable", func(t *testing.T) {
		f := setup(passing)
		got := rss(f, "Not parsable")
		if len(got) != 0 {
			t.Errorf("len(decisions) = %d, want = 0", len(got))
		}
		if f.finder.calls != 0 {
			t.Errorf("FindMovie calls = %d, want = 0", f.finder.calls)
		}
		wantCalls(t, 0, passing(f.specs)...)
	})

	t.Run("should_return_rejected_result_for_unparsable_search", func(t *testing.T) {
		f := setup(passing)
		releases := []*Release{{Title: "1937 - Snow White and the Seven Dwarves"}}
		got := f.engine.DecideMovies(context.Background(), releases, &Search{TargetID: tid(1)}, false, f.finder.find)
		if f.finder.calls != 0 {
			t.Errorf("FindMovie calls = %d, want = 0", f.finder.calls)
		}
		wantCalls(t, 0, passing(f.specs)...)
		// Radarr returns the decision without asserting it; it is
		// rejected as unparsable.
		if len(got) != 1 || len(got[0].Rejections) != 1 || got[0].Rejections[0].Reason != ReasonUnableToParse {
			t.Errorf("decisions = %+v, want = one rejected with %q", got, ReasonUnableToParse)
		}
	})

	t.Run("should_not_attempt_to_make_decision_if_series_is_unknown", func(t *testing.T) {
		f := setup(passing)
		f.finder.target = nil
		rss(f, rkTrolls)
		wantCalls(t, 0, passing(f.specs)...)
	})

	t.Run("broken_report_shouldnt_blowup_the_process", func(t *testing.T) {
		// Radarr also expects three errors logged; the port logs them
		// with slog, which the test does not capture.
		f := setup(func(s specs) []*rkSpec { return []*rkSpec{s.pass1} })
		f.finder.err = errors.New("test exception")
		got := rss(f, rkTrolls, rkTrolls, rkTrolls)
		if f.finder.calls != 3 {
			t.Errorf("FindMovie calls = %d, want = 3", f.finder.calls)
		}
		if len(got) != 3 {
			t.Errorf("len(decisions) = %d, want = 3", len(got))
		}
	})

	for _, name := range []string{"should_return_unknown_series_rejection_if_series_is_unknown", "should_not_allow_download_if_series_is_unknown"} {
		t.Run(name, func(t *testing.T) {
			f := setup(passing)
			f.finder.target = nil
			got := rss(f, rkTrolls)
			if len(got) != 1 {
				t.Fatalf("len(decisions) = %d, want = 1", len(got))
			}
			// Radarr's DownloadAllowed assertion is commented out; the
			// decision is rejected as for an unknown movie.
			if got[0].Approved() || got[0].Rejections[0].Reason != ReasonUnknownTarget {
				t.Errorf("Rejections = %+v, want = one of %q", got[0].Rejections, ReasonUnknownTarget)
			}
		})
	}

	t.Run("should_not_allow_download_if_no_episodes_found", func(t *testing.T) {
		t.Skip("ignored in Radarr (\"Series\"): a movie release has no episodes to find")
	})

	t.Run("should_return_a_decision_when_exception_is_caught", func(t *testing.T) {
		f := setup(func(s specs) []*rkSpec { return []*rkSpec{s.pass1} })
		f.finder.err = errors.New("test exception")
		got := rss(f, rkTrolls)
		if len(got) != 1 {
			t.Fatalf("len(decisions) = %d, want = 1", len(got))
		}
		if got[0].Approved() || got[0].Rejections[0].Reason != ReasonError {
			t.Errorf("Rejections = %+v, want = one of %q", got[0].Rejections, ReasonError)
		}
	})
}

// TestMovieLanguages ports Radarr's AggregateLanguagesFixture.
// movieLanguages takes the MULTi languages of the release's indexer
// already resolved; DecideMovies resolves them, which
// TestDecideMoviesIndexerLanguages covers.
func TestMovieLanguages(t *testing.T) {
	const simpleTitle = "Series.Title.S01E01.xyz-RlsGroup"
	const multiTitle = "Series.Title.S01E01.MULTi.1080p.WEB.H265-RlsGroup"
	multi := []release.Language{release.Original, release.French}
	tests := []struct {
		name         string
		movieTitle   string
		languages    []release.Language
		releaseTitle string
		tokens       string
		// title is the indexer's title of the release; multi the MULTi
		// languages of its indexer.
		title string
		multi []release.Language
		want  []release.Language
		// ordered compares the order too, as FluentAssertions' Equal;
		// otherwise as BeEquivalentTo.
		ordered bool
		// contains wants want among the languages, as Contain.
		contains bool
	}{
		{
			name:      "should_return_existing_language_if_episode_title_does_not_have_language",
			languages: []release.Language{release.Original}, releaseTitle: simpleTitle,
			want: []release.Language{release.English}, contains: true,
		},
		{
			name:      "should_return_parsed_language",
			languages: []release.Language{release.French}, releaseTitle: simpleTitle,
			want: []release.Language{release.French}, ordered: true,
		},
		{
			name:         "should_return_multi_languages_when_indexer_id_has_multi_languages_configuration",
			releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
		{
			// The indexer found by ID wins over the one named.
			name:         "should_return_multi_languages_from_indexer_with_id_when_indexer_id_and_name_are_set",
			releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
		{
			name:         "should_return_multi_languages_when_indexer_name_has_multi_languages_configuration",
			releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
		{
			name:      "should_return_multi_languages_when_release_as_unknown_as_default_language_and_indexer_has_multi_languages_configuration",
			languages: []release.Language{release.LanguageUnknown}, releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
		{
			name:         "should_return_multi_languages_when_release_as_specified_language_and_indexer_has_multi_languages_configuration",
			languages:    []release.Language{release.French},
			releaseTitle: "Some.Movie.2024.MULTi.VFF.VFQ.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			title:        "Some.Movie.2024.MULTi.VFF.VFQ.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			multi:        multi,
			want:         []release.Language{release.English, release.French},
		},
		{
			name:         "should_return_multi_languages_when_release_as_other_language_and_indexer_has_multi_languages_configuration",
			languages:    []release.Language{release.German},
			releaseTitle: "Some.Movie.2024.MULTi.GERMAN.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			title:        "Some.Movie.2024.MULTi.GERMAN.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			multi:        multi,
			want:         []release.Language{release.English, release.French, release.German},
		},
		{
			name:         "should_return_original_when_indexer_has_no_multi_languages_configuration",
			releaseTitle: multiTitle, title: multiTitle,
			want: []release.Language{release.English},
		},
		{
			name:         "should_return_original_when_no_indexer_value",
			releaseTitle: multiTitle, title: multiTitle,
			want: []release.Language{release.English},
		},
		{
			name:         "should_exclude_language_that_is_part_of_episode_title_when_release_tokens_contains_episode_title",
			movieTitle:   "Jimmy The Greek",
			languages:    []release.Language{release.Greek},
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.xyz-RlsGroup",
			tokens:       ".Jimmy.The.Greek.xyz-RlsGroup",
			want:         []release.Language{release.English}, ordered: true,
		},
		{
			name:         "should_remove_parsed_language_that_is_part_of_episode_title_when_release_tokens_contains_episode_title",
			movieTitle:   "Jimmy The Greek",
			languages:    []release.Language{release.Greek, release.French},
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.French.xyz-RlsGroup",
			tokens:       ".Jimmy.The.Greek.French.xyz-RlsGroup",
			want:         []release.Language{release.French}, ordered: true,
		},
		{
			name:         "should_not_exclude_language_that_is_part_of_episode_title_when_release_tokens_does_not_contain_episode_title",
			movieTitle:   "Jimmy The Greek",
			languages:    []release.Language{release.Greek},
			releaseTitle: "Series.Title.S01E01.xyz-RlsGroup",
			tokens:       ".xyz-RlsGroup",
			want:         []release.Language{release.Greek}, ordered: true,
		},
		{
			name:         "should_use_reparse_language_after_determining_languages_that_are_in_episode_titles",
			movieTitle:   "Jimmy The Greek",
			languages:    []release.Language{release.Greek},
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.Greek.xyz-RlsGroup",
			tokens:       ".Jimmy.The.Greek.Greek.xyz-RlsGroup",
			want:         []release.Language{release.Greek}, ordered: true,
		},
		{
			name:         "should_return_multi_languages_from_indexer_with_name_when_indexer_id_does_not_exist",
			releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
		{
			name:         "should_return_multi_languages_from_indexer_with_name_when_indexer_id_not_available",
			releaseTitle: multiTitle, title: multiTitle, multi: multi,
			want: []release.Language{release.English, release.French},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := tt.movieTitle
			if title == "" {
				title = "Some Movie"
			}
			target := &Target{ID: tid(1), Kind: release.Movie, Title: title, OriginalLanguage: release.English}
			parsed := &release.ParsedMovieInfo{Languages: tt.languages, ReleaseTitle: tt.releaseTitle, SimpleReleaseTitle: tt.tokens}
			got := movieLanguages(parsed, &Release{Title: tt.title}, target, tt.multi)
			var ok bool
			switch {
			case tt.contains:
				ok = !slices.ContainsFunc(tt.want, func(l release.Language) bool { return !slices.Contains(got, l) })
			case tt.ordered:
				ok = slices.Equal(got, tt.want)
			default:
				ok = slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(tt.want)))
			}
			if !ok {
				t.Errorf("movieLanguages() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// TestDecideMoviesIndexerLanguages ports the indexer lookups of Radarr's
// AggregateLanguagesFixture: DecideMovies takes the MULTi languages of
// the indexer the release's IndexerID names.
func TestDecideMoviesIndexerLanguages(t *testing.T) {
	const multiTitle = "Some.Movie.2024.MULTi.1080p.WEB.H265-RlsGroup"
	tests := []struct {
		name      string
		indexerID int
		indexers  map[string]*IndexerInfo
		want      []release.Language
		skip      string
	}{
		{
			name:      "should_return_multi_languages_when_indexer_id_has_multi_languages_configuration",
			indexerID: 1,
			indexers:  map[string]*IndexerInfo{ixID(1): {ID: ixID(1), MultiLanguages: []release.Language{release.Original, release.French}}},
			want:      []release.Language{release.English, release.French},
		},
		{
			name:      "should_return_multi_languages_from_indexer_with_id_when_indexer_id_and_name_are_set",
			indexerID: 1,
			indexers: map[string]*IndexerInfo{
				ixID(1): {ID: ixID(1), Name: "MyIndexer1", MultiLanguages: []release.Language{release.Original, release.French}},
				ixID(2): {ID: ixID(2), Name: "MyIndexer2", MultiLanguages: []release.Language{release.Original, release.German}},
			},
			want: []release.Language{release.English, release.French},
		},
		{
			name:      "should_return_original_when_indexer_has_no_multi_languages_configuration",
			indexerID: 1,
			indexers:  map[string]*IndexerInfo{ixID(1): {ID: ixID(1)}},
			want:      []release.Language{release.English},
		},
		{
			name: "should_return_original_when_no_indexer_value",
			want: []release.Language{release.English},
		},
		{
			name: "should_return_multi_languages_when_indexer_name_has_multi_languages_configuration",
			skip: "Ports cannot find an indexer by name; Radarr falls back to the indexer's name when its ID finds none",
		},
		{
			name: "should_return_multi_languages_from_indexer_with_name_when_indexer_id_does_not_exist",
			skip: "Ports cannot find an indexer by name; Radarr falls back to the indexer's name when its ID finds none",
		},
		{
			name: "should_return_multi_languages_from_indexer_with_name_when_indexer_id_not_available",
			skip: "Ports cannot find an indexer by name; Radarr falls back to the indexer's name when its ID finds none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			e, ports := newTestEnv()
			ports.indexers = tt.indexers
			engine := &Engine{Env: e}
			finder := &rkFinder{target: &Target{ID: tid(1), Kind: release.Movie, Title: "Some Movie", OriginalLanguage: release.English}}
			got := engine.DecideMovies(context.Background(), []*Release{{Title: multiTitle, IndexerID: ixID(tt.indexerID)}}, nil, false, finder.find)
			if len(got) != 1 {
				t.Fatalf("len(decisions) = %d, want = 1", len(got))
			}
			langs := slices.Sorted(slices.Values(got[0].Candidate.Languages))
			if !slices.Equal(langs, slices.Sorted(slices.Values(tt.want))) {
				t.Errorf("Languages = %v, want = %v", got[0].Candidate.Languages, tt.want)
			}
		})
	}
}

package series

import (
	"context"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/release"
)

// TestAggregateLanguages ports Sonarr's
// Download/Aggregation/Aggregators/AggregateLanguagesFixture through
// augment, which looks the indexer up by ID and works out the languages.
// The series' original language is English; the episode is titled
// "Title1" unless the case names it. Cases Sonarr compares with
// BeEquivalentTo ignore order; those with Equal do not.
func TestAggregateLanguages(t *testing.T) {
	const multi = "Series.Title.S01E01.MULTi.1080p.WEB.H265-RlsGroup"
	const simple = "Series.Title.S01E01.xyz-RlsGroup"
	langs := func(l ...release.Language) []release.Language { return l }
	originalFrench := langs(release.Original, release.French)
	tests := []struct {
		name          string
		skip          string
		parsed        []release.Language
		releaseTitle  string // the parsed release title
		releaseTokens string
		title         string // the release's title as the indexer gave it
		indexer       int    // the release's indexer, 0 for none
		multiLangs    []release.Language
		episodeTitle  string
		want          []release.Language
		ordered       bool
		contains      bool // want is a subset of the result
		lookups       int
	}{
		{name: "should_return_existing_language_if_episode_title_does_not_have_language", parsed: langs(release.Original), releaseTitle: simple, want: langs(release.English), contains: true},
		{name: "should_return_parsed_language", parsed: langs(release.French), releaseTitle: simple, want: langs(release.French), ordered: true},
		{name: "should_return_multi_languages_when_indexer_id_has_multi_languages_configuration", parsed: langs(), releaseTitle: multi, title: multi, indexer: 1, multiLangs: originalFrench, want: langs(release.English, release.French), lookups: 1},
		{name: "should_return_multi_languages_from_indexer_with_id_when_indexer_id_and_name_are_set", parsed: langs(), releaseTitle: multi, title: multi, indexer: 1, multiLangs: originalFrench, want: langs(release.English, release.French), lookups: 1},
		{name: "should_return_multi_languages_when_indexer_name_has_multi_languages_configuration", skip: "Ports.Indexer finds indexers by ID only; Mavio has no lookup by name"},
		{
			name: "should_return_multi_languages_when_release_as_unknown_as_default_language_and_indexer_has_multi_languages_configuration", parsed: langs(release.LanguageUnknown),
			releaseTitle: multi, title: multi, indexer: 1, multiLangs: originalFrench, want: langs(release.English, release.French), lookups: 1,
		},
		{
			name: "should_return_multi_languages_when_release_as_specified_language_and_indexer_has_multi_languages_configuration", parsed: langs(release.French),
			releaseTitle: "Series.Title.S01E01.MULTi.VFF.VFQ.1080p.BluRay.DTS.HDMA.x264-RlsGroup", title: "Series.Title.S01E01.MULTi.VFF.VFQ.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			indexer: 1, multiLangs: originalFrench, want: langs(release.English, release.French), lookups: 1,
		},
		{
			name: "should_return_multi_languages_when_release_as_other_language_and_indexer_has_multi_languages_configuration", parsed: langs(release.German),
			releaseTitle: "Series.Title.S01E01.MULTi.GERMAN.1080p.BluRay.DTS.HDMA.x264-RlsGroup", title: "Series.Title.S01E01.MULTi.GERMAN.1080p.BluRay.DTS.HDMA.x264-RlsGroup",
			indexer: 1, multiLangs: originalFrench, want: langs(release.English, release.French, release.German), lookups: 1,
		},
		{name: "should_return_original_when_indexer_has_no_multi_languages_configuration", parsed: langs(), releaseTitle: multi, title: multi, indexer: 1, want: langs(release.English), lookups: 1},
		{name: "should_return_original_when_no_indexer_value", parsed: langs(), releaseTitle: multi, title: multi, want: langs(release.English)},
		{
			name: "should_exclude_language_that_is_part_of_episode_title_when_release_tokens_contains_episode_title", parsed: langs(release.Greek),
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.xyz-RlsGroup", releaseTokens: ".Jimmy.The.Greek.xyz-RlsGroup", episodeTitle: "Jimmy The Greek",
			want: langs(release.English), ordered: true,
		},
		{
			name: "should_remove_parsed_language_that_is_part_of_episode_title_when_release_tokens_contains_episode_title", parsed: langs(release.Greek, release.French),
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.French.xyz-RlsGroup", releaseTokens: ".Jimmy.The.Greek.French.xyz-RlsGroup", episodeTitle: "Jimmy The Greek",
			want: langs(release.French), ordered: true,
		},
		{
			name: "should_not_exclude_language_that_is_part_of_episode_title_when_release_tokens_does_not_contain_episode_title", parsed: langs(release.Greek),
			releaseTitle: simple, releaseTokens: ".xyz-RlsGroup", episodeTitle: "Jimmy The Greek", want: langs(release.Greek), ordered: true,
		},
		{
			name: "should_use_reparse_language_after_determining_languages_that_are_in_episode_titles", parsed: langs(release.Greek),
			releaseTitle: "Series.Title.S01E01.Jimmy.The.Greek.Greek.xyz-RlsGroup", releaseTokens: ".Jimmy.The.Greek.Greek.xyz-RlsGroup", episodeTitle: "Jimmy The Greek",
			want: langs(release.Greek), ordered: true,
		},
		{name: "should_return_multi_languages_from_indexer_with_name_when_indexer_id_does_not_exist", skip: "Ports.Indexer finds indexers by ID only; Mavio has no fallback to the name"},
		{name: "should_return_multi_languages_from_indexer_with_name_when_indexer_id_not_available", skip: "Ports.Indexer finds indexers by ID only; Mavio has no lookup by name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			env, ports, _ := newTestEnv()
			ports.indexers = map[string]*acquire.IndexerInfo{ixID(1): {ID: ixID(1), MultiLanguages: tt.multiLangs}}
			lookups := 0
			counting := &countingPorts{fakePorts: ports, lookups: &lookups}
			env.Ports = counting
			s := testSeries(testProfile(release.HDTV720p, release.HDTV720p))
			episode := ep(1, 1, 1)
			episode.Title = "Title1"
			if tt.episodeTitle != "" {
				episode.Title = tt.episodeTitle
			}
			parsed := &release.ParsedEpisodeInfo{Languages: tt.parsed, ReleaseTitle: tt.releaseTitle, ReleaseTokens: tt.releaseTokens}
			c := NewCandidate(&acquire.Release{Title: tt.title, IndexerID: ixID(tt.indexer)}, parsed, s, []*Episode{episode})
			e := &Engine{Env: env}
			if err := e.augment(context.Background(), c); err != nil {
				t.Fatalf("augment() error = %v", err)
			}
			got := c.Languages
			switch {
			case tt.contains:
				for _, l := range tt.want {
					if !slices.Contains(got, l) {
						t.Errorf("Languages = %v, want = %v among them", got, l)
					}
				}
			case tt.ordered:
				if !slices.Equal(got, tt.want) {
					t.Errorf("Languages = %v, want = %v", got, tt.want)
				}
			default:
				if !sameElements(got, tt.want) {
					t.Errorf("Languages = %v, want = %v in any order", got, tt.want)
				}
			}
			if lookups != tt.lookups {
				t.Errorf("indexer lookups = %d, want = %d", lookups, tt.lookups)
			}
		})
	}
}

// countingPorts counts indexer lookups.
type countingPorts struct {
	*fakePorts
	lookups *int
}

func (c *countingPorts) Indexer(ctx context.Context, id string) (*acquire.IndexerInfo, error) {
	*c.lookups++
	return c.fakePorts.Indexer(ctx, id)
}

// sameElements reports whether a and b hold the same elements in any
// order.
func sameElements[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !slices.Contains(b, v) {
			return false
		}
	}
	return true
}

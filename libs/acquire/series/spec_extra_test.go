package series

import (
	"testing"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// The tests in this file are Mavio's own: Sonarr has no fixtures for
// these specifications.

func TestEpisodeRequested(t *testing.T) {
	tests := []struct {
		name     string
		search   *Search
		episodes []*Episode
		want     string // the rejection message, empty to accept
	}{
		{name: "rss", episodes: []*Episode{ep(1, 1, 1)}},
		{name: "requested", search: &Search{Episodes: []*Episode{ep(1, 1, 1)}}, episodes: []*Episode{ep(1, 1, 1), ep(2, 1, 2)}},
		{name: "no episodes", search: &Search{Episodes: []*Episode{ep(1, 1, 1)}}, want: "Episode wasn't requested"},
		{name: "single", search: &Search{Episodes: []*Episode{ep(1, 1, 1)}}, episodes: []*Episode{ep(3, 2, 5)}, want: "Episode wasn't requested: 2x5"},
		{name: "range", search: &Search{Episodes: []*Episode{ep(1, 1, 1)}}, episodes: []*Episode{ep(4, 2, 7), ep(3, 2, 5)}, want: "Episode wasn't requested: 2x5-7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := NewCandidate(&acquire.Release{}, nil, nil, tt.episodes)
			r := check(t, EpisodeRequested{}, e, c, tt.search)
			got := ""
			if r != nil {
				got = r.Message
			}
			if got != tt.want {
				t.Errorf("rejection = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestSeasonMatch(t *testing.T) {
	tests := []struct {
		name   string
		search *Search
		season int
		want   bool
	}{
		{name: "rss", season: 2, want: true},
		{name: "episode search", search: &Search{Kind: SingleEpisodeSearch, SeasonNumber: 1}, season: 2, want: true},
		{name: "same season", search: &Search{Kind: SeasonSearch, SeasonNumber: 2}, season: 2, want: true},
		{name: "other season", search: &Search{Kind: SeasonSearch, SeasonNumber: 1}, season: 2, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := NewCandidate(&acquire.Release{}, &release.ParsedEpisodeInfo{SeasonNumber: tt.season}, nil, nil)
			want(t, SeasonMatch{}, e, c, tt.search, tt.want)
		})
	}
}

func TestSceneNumbering(t *testing.T) {
	tests := []struct {
		name    string
		mapping *SceneMapping
		want    string
	}{
		{name: "no mapping"},
		{name: "no origin", mapping: &SceneMapping{}},
		{name: "scene origin", mapping: &SceneMapping{SceneOrigin: "tvdb"}},
		{name: "mixed", mapping: &SceneMapping{SceneOrigin: "mixed"}, want: "Ambiguous numbering"},
		{name: "mixed with season", mapping: &SceneMapping{SceneOrigin: "mixed:2"}, want: "Ambiguous numbering"},
		{name: "mixed with comment", mapping: &SceneMapping{SceneOrigin: "mixed", Comment: "Specials"}, want: "{0} has ambiguous numbering"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, _ := newTestEnv()
			c := NewCandidate(&acquire.Release{}, nil, nil, nil)
			c.SceneMapping = tt.mapping
			spec := SceneNumbering{}
			r := check(t, spec, e, c, nil)
			got := ""
			if r != nil {
				got = r.Message
			}
			if got != tt.want {
				t.Errorf("rejection = %q, want = %q", got, tt.want)
			}
			if !spec.Temporary() {
				t.Errorf("Temporary() = false, want = true")
			}
		})
	}
}

func TestSplitEpisode(t *testing.T) {
	for _, split := range []bool{false, true} {
		e, _, _ := newTestEnv()
		c := NewCandidate(&acquire.Release{}, &release.ParsedEpisodeInfo{IsSplitEpisode: split}, nil, nil)
		want(t, SplitEpisode{}, e, c, nil, !split)
	}
}

func TestSameEpisodesGrab(t *testing.T) {
	tests := []struct {
		name   string
		inFile []int
		want   bool
	}{
		{name: "file holds the same episodes", inFile: []int{1, 2}, want: true},
		{name: "file holds more episodes", inFile: []int{1, 2, 3}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _, eps := newTestEnv()
			eps.inFile = map[core.ID][]core.ID{tid(1): ids(tt.inFile...)}
			first, second := ep(1, 1, 1), ep(2, 1, 2)
			first.File = file(1, model(release.HDTV720p, 1))
			second.File = first.File
			c := testCandidate(testSeries(testProfile(release.HDTV720p, release.HDTV720p)), model(release.HDTV720p, 1), first, second)
			want(t, SameEpisodesGrab{}, e, c, nil, tt.want)
		})
	}
}

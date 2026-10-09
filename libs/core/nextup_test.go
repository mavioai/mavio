package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPortedSortingCases checks that every test of the case files ported
// by tools/testport (testdata/cases, see docs/testing.md §2) is ported by
// hand or skipped with a reason.
func TestPortedSortingCases(t *testing.T) {
	ported := map[string]string{
		"AiredEpisodeOrderCompareTest": "TestCompareAiredOrder",
	}
	skipped := map[string]string{
		"Compare_GivenNull_ThrowsArgumentNullException": "CompareAiredOrder takes items, never nil",
	}
	data, err := os.ReadFile(filepath.Join("testdata", "cases", "aired_episode_order_comparer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Unsupported []struct {
			Method string `json:"method"`
		} `json:"unsupported"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, m := range f.Unsupported {
		if ported[m.Method] == "" && skipped[m.Method] == "" {
			t.Errorf("%s is neither ported nor skipped", m.Method)
		}
	}
}

// episode builds an episode: season and number are -1 when unset.
func episode(season, number int, opts ...func(*Item)) Item {
	it := Item{ID: NewID(), Kind: KindEpisode}
	if season >= 0 {
		it.ParentIndexNumber = &season
	}
	if number >= 0 {
		it.IndexNumber = &number
	}
	for _, o := range opts {
		o(&it)
	}
	return it
}

func airsAfter(season int) func(*Item) { return func(it *Item) { it.AirsAfterSeasonNumber = &season } }

func airsBefore(season int) func(*Item) {
	return func(it *Item) { it.AirsBeforeSeasonNumber = &season }
}

func beforeEpisode(n int) func(*Item) { return func(it *Item) { it.AirsBeforeEpisodeNumber = &n } }

func premiered(day int) func(*Item) {
	return func(it *Item) { it.PremiereDate = new(time.Date(2021, 9, day, 0, 0, 0, 0, time.UTC)) }
}

// TestCompareAiredOrder ports the AiredEpisodeOrderCompareTest cases of
// Jellyfin's AiredEpisodeOrderComparerTests.EpisodeTestData, in order.
func TestCompareAiredOrder(t *testing.T) {
	movie := Item{ID: NewID(), Kind: KindMovie}
	tests := []struct {
		x, y Item
		want int
	}{
		{movie, movie, 0},
		{movie, episode(-1, -1), 1},
		// Good cases
		{episode(-1, -1), episode(-1, -1), 0},
		{episode(1, 1), episode(1, 1), 0},
		{episode(1, 2), episode(1, 1), 1},
		{episode(2, 1), episode(1, 1), 1},
		// Good specials
		{episode(0, 1), episode(0, 1), 0},
		{episode(0, 2), episode(0, 1), 1},
		// Specials to episodes
		{episode(1, 1), episode(0, 1), 1},
		{episode(1, 1), episode(0, 2), 1},
		{episode(1, 2), episode(0, 1), 1},
		{episode(1, 2), episode(0, 1), 1},
		{episode(1, 1), episode(0, 2), 1},
		{episode(0, 1, airsAfter(1)), episode(1, 1), 1},
		{episode(3, 1), episode(0, 1, airsAfter(1)), 1},
		{episode(3, 1), episode(0, 1, airsAfter(1), beforeEpisode(2)), 1},
		{episode(1, 1), episode(0, 1, airsBefore(1)), 1},
		{episode(1, 2), episode(0, 1, airsBefore(1), beforeEpisode(2)), 1},
		{episode(1, -1), episode(0, 1, airsBefore(1), beforeEpisode(2)), 0},
		{episode(1, 3), episode(0, 1, airsBefore(1), beforeEpisode(2)), 1},
		// Premiere date
		{episode(1, 1, premiered(12)), episode(1, 1, premiered(12)), 0},
		{episode(1, 1, premiered(11)), episode(1, 1, premiered(12)), -1},
		{episode(1, 1, premiered(12)), episode(1, 1, premiered(11)), 1},
	}
	for i, tt := range tests {
		if got := CompareAiredOrder(&tt.x, &tt.y); got != tt.want {
			t.Errorf("case %d: CompareAiredOrder(x, y) got = %d, want = %d", i+1, got, tt.want)
		}
		if got := CompareAiredOrder(&tt.y, &tt.x); got != -tt.want {
			t.Errorf("case %d: CompareAiredOrder(y, x) got = %d, want = %d", i+1, got, -tt.want)
		}
	}
}

func TestNextEpisode(t *testing.T) {
	s1e1, s1e2, s1e3 := episode(1, 1), episode(1, 2), episode(1, 3)
	s2e1, s2e2 := episode(2, 1), episode(2, 2)
	unnumbered := episode(2, -1)
	beforeS2 := episode(0, 1, airsBefore(2))
	afterS1 := episode(0, 2, airsAfter(1))
	unplaced := episode(0, 3)

	played := UserData{Played: true}
	resumed := UserData{Position: time.Minute}
	data := func(states map[*Item]UserData) map[ID]UserData {
		out := make(map[ID]UserData, len(states))
		for it, d := range states {
			out[it.ID] = d
		}
		return out
	}
	tests := []struct {
		name      string
		episodes  []Item
		data      map[ID]UserData
		resumable bool
		want      *Item
	}{
		{"nothing played: the first", []Item{s1e2, s1e1}, nil, false, &s1e1},
		{"after the last played", []Item{s1e1, s1e2, s1e3}, data(map[*Item]UserData{&s1e1: played, &s1e2: played}), false, &s1e3},
		{"skipped episodes stay behind", []Item{s1e1, s1e2, s1e3}, data(map[*Item]UserData{&s1e2: played}), false, &s1e3},
		{"into the next season", []Item{s1e1, s2e1, s2e2}, data(map[*Item]UserData{&s1e1: played}), false, &s2e1},
		{"a later season without episode number", []Item{s1e1, unnumbered}, data(map[*Item]UserData{&s1e1: played}), false, &unnumbered},
		{"all played", []Item{s1e1, s1e2}, data(map[*Item]UserData{&s1e1: played, &s1e2: played}), false, nil},
		{"resumed belongs to continue watching", []Item{s1e1, s1e2}, data(map[*Item]UserData{&s1e1: played, &s1e2: resumed}), false, nil},
		{"resumed when asked", []Item{s1e1, s1e2}, data(map[*Item]UserData{&s1e1: played, &s1e2: resumed}), true, &s1e2},
		{"special airing before the season", []Item{s1e1, s1e2, s2e1, beforeS2}, data(map[*Item]UserData{&s1e1: played, &s1e2: played}), false, &beforeS2},
		{"special airing after the season", []Item{s1e1, s1e2, s2e1, afterS1}, data(map[*Item]UserData{&s1e1: played, &s1e2: played}), false, &afterS1},
		{"played special is passed", []Item{s1e2, s2e1, afterS1}, data(map[*Item]UserData{&s1e2: played, &afterS1: played}), false, &s2e1},
		{"unplaced specials are left out", []Item{unplaced, s1e1}, data(map[*Item]UserData{&s1e1: played}), false, nil},
		{"specials never count as last played", []Item{s1e1, afterS1, s2e1}, data(map[*Item]UserData{&afterS1: played}), false, &s1e1},
	}
	for _, tt := range tests {
		got := NextEpisode(tt.episodes, tt.data, tt.resumable)
		switch {
		case tt.want == nil && got != nil:
			t.Errorf("%s: got = S%vE%v, want = none", tt.name, intOr(got.ParentIndexNumber, -1), intOr(got.IndexNumber, -1))
		case tt.want != nil && (got == nil || got.ID != tt.want.ID):
			t.Errorf("%s: got = %v, want = S%dE%d", tt.name, got, intOr(tt.want.ParentIndexNumber, -1), intOr(tt.want.IndexNumber, -1))
		}
	}
}

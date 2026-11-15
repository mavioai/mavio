package core

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestSearchKey(t *testing.T) {
	for _, tc := range []struct{ query, text, key string }{
		{"  The   Matrix ", "The Matrix", "the matrix"},
		{"\t\n", "", ""},
		{"Amélie", "Amélie", "amélie"},
	} {
		if got := SearchText(tc.query); got != tc.text {
			t.Errorf("SearchText(%q) got = %q, want = %q", tc.query, got, tc.text)
		}
		if got := SearchKey(tc.query); got != tc.key {
			t.Errorf("SearchKey(%q) got = %q, want = %q", tc.query, got, tc.key)
		}
	}
	a, b := NewID(), NewID()
	if SearchVoter(a, "up") == SearchVoter(b, "up") || SearchVoter(a, "up") == SearchVoter(a, "heat") || len(SearchVoter(a, "up")) != 32 {
		t.Errorf("SearchVoter does not tell users and queries apart")
	}
}

func TestSearchRecordValidate(t *testing.T) {
	now := time.Now()
	voter := SearchVoter(NewID(), "up")
	for _, tc := range []struct {
		name string
		r    SearchRecord
		ok   bool
	}{
		{"valid", SearchRecord{Key: "up", Text: "Up", Voter: voter, SearchedAt: now}, true},
		{"key of another text", SearchRecord{Key: "heat", Text: "Up", Voter: voter, SearchedAt: now}, false},
		{"empty", SearchRecord{Voter: voter, SearchedAt: now}, false},
		{"too long", SearchRecord{Key: strings.Repeat("a", 101), Text: strings.Repeat("a", 101), Voter: voter, SearchedAt: now}, false},
		{"no voter", SearchRecord{Key: "up", Text: "Up", SearchedAt: now}, false},
		{"no time", SearchRecord{Key: "up", Text: "Up", Voter: voter}, false},
	} {
		err := tc.r.Validate()
		if tc.ok != (err == nil) || err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Validate() got = %v, want ok = %v", tc.name, err, tc.ok)
		}
	}
}

func TestRankSearches(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	alice, bob, carol := NewID(), NewID(), NewID()
	rec := func(user ID, text string, age time.Duration) SearchRecord {
		return SearchRecord{Key: SearchKey(text), Text: text, Voter: SearchVoter(user, SearchKey(text)), SearchedAt: now.Add(-age)}
	}
	records := []SearchRecord{
		// Two users long ago, against two users just now.
		rec(alice, "heat", 30*24*time.Hour), rec(bob, "Heat", 28*24*time.Hour),
		rec(alice, "Up", time.Hour), rec(bob, "up", 2*time.Hour),
		// Carol alone searched these.
		rec(carol, "Lost", time.Minute),
		// Three users, one of them two half-lives ago.
		rec(alice, "Bluey", 0), rec(bob, "Bluey", 0), rec(carol, "Bluey", 2*SearchHalfLife),
	}
	keys := func(list []PopularSearch) string {
		var out []string
		for _, p := range list {
			out = append(out, p.Text)
		}
		return strings.Join(out, ",")
	}
	if got := keys(RankSearches(records, now, alice)); got != "Bluey,Up,Heat" {
		t.Errorf("RankSearches(alice) got = %s, want = Bluey,Up,Heat", got)
	}
	if got := keys(RankSearches(records, now, carol)); got != "Bluey,Up,Lost,Heat" {
		t.Errorf("RankSearches(carol) got = %s, want = Bluey,Up,Lost,Heat", got)
	}
	if got := keys(RankSearches(records, now, NilID)); got != "Bluey,Up,Heat" {
		t.Errorf("RankSearches(nobody) got = %s, want = Bluey,Up,Heat", got)
	}
	top := RankSearches(records, now, NilID)[0]
	if top.Users != 3 || math.Abs(top.Score-2.25) > 1e-9 {
		t.Errorf("Bluey got = %d users, score %v, want = 3 users, score 2.25", top.Users, top.Score)
	}
}

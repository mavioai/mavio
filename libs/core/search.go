package core

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// SearchRecord is one user's latest search for a query, which ranks the
// server's popular searches. It names no user: Voter is a hash of the user
// and the query, so that a user's searches cannot be gathered from the
// records, and a user searching a query again replaces their record of it.
type SearchRecord struct {
	// Key is the query's normalized form (SearchKey), which groups records.
	Key string
	// Text is the query as the user typed it, whitespace collapsed.
	Text string
	// Voter is SearchVoter of the user and Key.
	Voter string
	// SearchedAt is when the user last searched the query.
	SearchedAt time.Time
}

// PopularSearch is a query ranked among a server's popular searches.
type PopularSearch struct {
	Key  string
	Text string
	// Users is the number of users who searched it.
	Users int
	// Score sums each user's search, halved for every SearchHalfLife since.
	Score float64
}

// Limits of popular searches.
const (
	// MaxSearchLength is the longest query recorded, in characters.
	MaxSearchLength = 100
	// SearchHalfLife is the age at which a search counts half.
	SearchHalfLife = 14 * 24 * time.Hour
	// SearchRetention is how long searches are kept.
	SearchRetention = 90 * 24 * time.Hour
	// MaxSearchRecords is how many search records are kept, the newest.
	MaxSearchRecords = 10_000
	// MinSearchUsers is how many users must have searched a query before
	// others are shown it; those who did are shown it at once.
	MinSearchUsers = 2
)

// SearchText collapses the whitespace of a query; it is empty for a query
// of whitespace alone.
func SearchText(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

// SearchKey is the normalized form of a query: its SearchText in lower case.
func SearchKey(query string) string {
	return strings.ToLower(SearchText(query))
}

// SearchVoter is the hash that stands for a user in their record of a
// query's key.
func SearchVoter(userID ID, key string) string {
	h := sha256.New()
	h.Write(userID[:])
	h.Write([]byte{0})
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Validate checks the record's invariants.
func (r *SearchRecord) Validate() error {
	switch {
	case r.Key == "" || r.Key != SearchKey(r.Text):
		return fmt.Errorf("%w: search record requires the key of its text", ErrInvalid)
	case utf8.RuneCountInString(r.Text) > MaxSearchLength:
		return fmt.Errorf("%w: search %q is longer than %d characters", ErrInvalid, r.Text, MaxSearchLength)
	case len(r.Voter) != 32:
		return fmt.Errorf("%w: search record requires a voter", ErrInvalid)
	case r.SearchedAt.IsZero():
		return fmt.Errorf("%w: search record requires a time", ErrInvalid)
	}
	return nil
}

// RankSearches ranks the queries of records at now, best first: each user
// who searched a query adds 1, halved for every SearchHalfLife since, and
// ties go to the query searched last. A query is shown to the viewer when
// MinSearchUsers searched it, or the viewer did; its Text is the latest
// spelling.
func RankSearches(records []SearchRecord, now time.Time, viewer ID) []PopularSearch {
	type ranked struct {
		PopularSearch
		last  time.Time
		voted bool
	}
	byKey := map[string]*ranked{}
	for _, r := range records {
		q := byKey[r.Key]
		if q == nil {
			q = &ranked{PopularSearch: PopularSearch{Key: r.Key}}
			byKey[r.Key] = q
		}
		age := max(now.Sub(r.SearchedAt), 0)
		q.Users++
		q.Score += math.Exp2(-float64(age) / float64(SearchHalfLife))
		if !r.SearchedAt.Before(q.last) {
			q.last, q.Text = r.SearchedAt, r.Text
		}
		if !viewer.IsZero() && r.Voter == SearchVoter(viewer, r.Key) {
			q.voted = true
		}
	}
	list := make([]*ranked, 0, len(byKey))
	for _, q := range byKey {
		if q.Users >= MinSearchUsers || q.voted {
			list = append(list, q)
		}
	}
	slices.SortFunc(list, func(a, b *ranked) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), b.last.Compare(a.last), cmp.Compare(a.Key, b.Key))
	})
	out := make([]PopularSearch, len(list))
	for i, q := range list {
		out[i] = q.PopularSearch
	}
	return out
}

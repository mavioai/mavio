package indexer

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// SearchType is what a search looks for, as Newznab's t parameter tells.
type SearchType int

// The search types.
const (
	SearchBasic SearchType = iota
	SearchTV
	SearchMovie
	SearchMusic
	SearchBook
)

func (t SearchType) String() string {
	switch t {
	case SearchTV:
		return "tv"
	case SearchMovie:
		return "movie"
	case SearchMusic:
		return "music"
	case SearchBook:
		return "book"
	}
	return "basic"
}

// Query is a search of an indexer, as Prowlarr's search criteria are. The
// fields beyond Term and Categories that a search type does not take are
// ignored.
type Query struct {
	Type       SearchType
	Term       string
	Categories []int
	// Zero for the indexer's default.
	Limit, Offset int
	// The search was made by someone, not to watch for new releases.
	Interactive bool

	IMDbID                                           string
	TMDBID, TVDBID, RID, TVMazeID, TraktID, DoubanID int
	Season                                           int
	// An episode number, or a date's month and day ("03/01") with Season
	// its year.
	Episode string
	Year    int
	Genre   string

	Artist, Album, Label, Track string
	Author, Title, Publisher    string
}

// IsIDSearch reports whether the query names what it looks for by more
// than its term: IDs for movies and series, or the artist, title and so
// on of music and books.
func (q *Query) IsIDSearch() bool {
	switch q.Type {
	case SearchMovie:
		return q.IMDbID != "" || q.Genre != "" || q.TMDBID != 0 || q.TraktID != 0 || q.DoubanID != 0 || q.Year != 0
	case SearchTV:
		return q.IMDbID != "" || q.TVDBID != 0 || q.RID != 0 || q.TraktID != 0 || q.TVMazeID != 0 || q.TMDBID != 0 || q.DoubanID != 0
	case SearchMusic:
		return q.Album != "" || q.Artist != "" || q.Label != "" || q.Genre != "" || q.Track != "" || q.Year != 0
	case SearchBook:
		return q.Author != "" || q.Title != "" || q.Publisher != "" || q.Genre != "" || q.Year != 0
	}
	return false
}

// IsRSS reports whether the query only reads the indexer's recent
// releases.
func (q *Query) IsRSS() bool {
	return strings.TrimSpace(q.Term) == "" && !q.IsIDSearch()
}

var (
	dashes       = regexp.MustCompile(`\p{Pd}+`)
	singleQuotes = regexp.MustCompile("[`´‘’]")
)

// SanitizedTerm returns the term without characters indexers choke on.
func (q *Query) SanitizedTerm() string {
	t := singleQuotes.ReplaceAllString(dashes.ReplaceAllString(q.Term, "-"), "'")
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || strings.ContainsRune("-._()@/'[]+%", r) {
			return r
		}
		return -1
	}, t)
}

// EpisodeString returns the season and episode as releases write them:
// "S01", "S01E02" or, for a date, "2024.03.01"; "" without a season.
func (q *Query) EpisodeString() string {
	if q.Season == 0 {
		return ""
	}
	if d, err := time.Parse("2006 01/02", fmt.Sprintf("%d %s", q.Season, q.Episode)); err == nil {
		return d.Format("2006.01.02")
	}
	if strings.TrimSpace(q.Episode) == "" {
		return fmt.Sprintf("S%02d", q.Season)
	}
	if n, err := parse.Int(q.Episode); err == nil {
		return fmt.Sprintf("S%02dE%02d", q.Season, n)
	}
	return fmt.Sprintf("S%02dE%s", q.Season, q.Episode)
}

// SanitizedTVString returns the sanitized term and the episode string.
func (q *Query) SanitizedTVString() string {
	return strings.TrimSpace(q.SanitizedTerm() + " " + q.EpisodeString())
}

// FullIMDbID returns the IMDb ID as "tt" and seven or more digits; "" for
// none.
func (q *Query) FullIMDbID() string {
	return parse.FullIMDbID(q.IMDbID)
}

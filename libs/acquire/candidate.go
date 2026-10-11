package acquire

import (
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// Release is a release an indexer offers, as Radarr's ReleaseInfo and
// TorrentInfo.
type Release struct {
	GUID        string
	Title       string
	Size        int64
	PublishDate time.Time
	Protocol    Protocol
	// IndexerID names the indexer as "<plugin ID>/<indexer ID>"; it is
	// empty when the release did not come from a configured indexer, as
	// when it was pushed.
	IndexerID       string
	Indexer         string
	IndexerPriority int
	// Flags are the indexer flags, Flag* constants.
	Flags []string
	// Languages are those the indexer or the user gave; when set they
	// replace the parsed ones.
	Languages []release.Language
	// Seeders and Peers are nil when the indexer did not say.
	Seeders, Peers *int
	InfoHash       string
	// Container is the container the indexer gave, such as "iso" or
	// "m2ts".
	Container string
	ImdbID    string
	TmdbID    int
	TvdbID    int
}

// Age returns how old the release is at now.
func (r *Release) Age(now time.Time) time.Duration {
	return now.Sub(r.PublishDate)
}

// HasFlag reports whether the release carries the indexer flag.
func (r *Release) HasFlag(flag string) bool {
	for _, f := range r.Flags {
		if strings.EqualFold(f, flag) {
			return true
		}
	}
	return false
}

// Source is how a release reached the decision engine.
type Source int

// The release sources.
const (
	SourceRSS Source = iota
	SourceSearch
	SourceUserInvokedSearch
	SourceInteractiveSearch
	SourceReleasePush
)

// Search is what a search was for. A nil *Search means an RSS sync or a
// pushed release.
type Search struct {
	// TargetID is the movie or series searched for.
	TargetID core.ID
	// ItemIDs are the episodes searched for; a movie's is its own ID.
	ItemIDs []core.ID
	// UserInvoked is set when a user started the search, Interactive
	// when they pick the release themselves.
	UserInvoked bool
	Interactive bool
}

// sourceOf returns the source of a release found by s, or pushed.
func sourceOf(s *Search, pushed bool) Source {
	switch {
	case s == nil && pushed:
		return SourceReleasePush
	case s == nil:
		return SourceRSS
	case s.Interactive:
		return SourceInteractiveSearch
	case s.UserInvoked:
		return SourceUserInvokedSearch
	default:
		return SourceSearch
	}
}

// Target is the movie or series a release is for.
type Target struct {
	ID    core.ID
	Kind  release.Kind
	Title string
	Year  int
	// OriginalLanguage is the language the movie or series was made in.
	OriginalLanguage release.Language
	Tags             []string
	Profile          *QualityProfile
	Monitored        bool
	// Path is the folder files are imported into.
	Path string
}

// Item is a movie, or an episode of a series, a release would fill.
type Item struct {
	ID        core.ID
	Monitored bool
	// Runtime is in minutes; 0 when unknown.
	Runtime int
	// Availability is when a movie counts as available; nil for
	// episodes.
	Availability *Availability
	// File is the file the item has, or nil.
	File *File
}

// File is a file already imported for an item.
type File struct {
	ID           core.ID
	Quality      QualityModel
	Formats      []CustomFormat
	ReleaseGroup string
	Languages    []release.Language
	DateAdded    time.Time
	Size         int64
}

// Candidate is a release mapped to what it is for, with what was parsed
// from its title, as Radarr's RemoteMovie.
type Candidate struct {
	Release *Release
	Source  Source

	Quality       QualityModel
	Languages     []release.Language
	ReleaseGroup  string
	Edition       string
	HardcodedSubs string
	Year          int
	ReleaseType   release.ReleaseType

	// Target is nil when the release matched nothing.
	Target *Target
	Items  []*Item

	Formats     []CustomFormat
	FormatScore int
}

// Files returns the files of the candidate's items.
func (c *Candidate) Files() []*File {
	var files []*File
	for _, it := range c.Items {
		if it.File != nil {
			files = append(files, it.File)
		}
	}
	return files
}

// formatInput returns the input custom formats are matched against.
func (c *Candidate) formatInput(simpleTitle string) *Input {
	in := &Input{
		Title:        simpleTitle,
		Quality:      release.ParsedQuality{Quality: c.Quality.Quality, Revision: c.Quality.Revision},
		Languages:    c.Languages,
		ReleaseGroup: c.ReleaseGroup,
		Edition:      c.Edition,
		Year:         c.Year,
		ReleaseType:  c.ReleaseType,
	}
	if c.Release != nil {
		in.Size = c.Release.Size
		in.Flags = c.Release.Flags
	}
	if c.Target != nil {
		in.OriginalLanguage = c.Target.OriginalLanguage
		if in.Year == 0 {
			in.Year = c.Target.Year
		}
	}
	return in
}

// MovieStatus is how far a movie's release has come, as Radarr's
// MovieStatusType.
type MovieStatus int

// The movie statuses.
const (
	StatusTBA MovieStatus = iota
	StatusAnnounced
	StatusInCinemas
	StatusReleased
)

// Availability is what decides when a movie counts as available.
type Availability struct {
	// Minimum is the status from which the movie is wanted.
	Minimum MovieStatus
	// The release dates; zero when unknown.
	InCinemas, Digital, Physical time.Time
}

// Available reports whether the movie is available at now, delay days
// after its minimum availability, as Radarr's Movie.IsAvailable.
func (a *Availability) Available(now time.Time, delay int) bool {
	var from time.Time
	switch {
	case a.Minimum == StatusTBA || a.Minimum == StatusAnnounced:
		return true
	case a.Minimum == StatusInCinemas && !a.InCinemas.IsZero():
		from = a.InCinemas
	case !a.Physical.IsZero() && !a.Digital.IsZero():
		from = a.Physical
		if a.Digital.Before(from) {
			from = a.Digital
		}
	case !a.Physical.IsZero():
		from = a.Physical
	case !a.Digital.IsZero():
		from = a.Digital
	case !a.InCinemas.IsZero():
		from = a.InCinemas.AddDate(0, 0, 90)
	default:
		return false
	}
	return !now.Before(from.AddDate(0, 0, delay))
}

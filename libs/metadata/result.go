package metadata

import (
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Result is the metadata read for one item.
type Result struct {
	Item core.Item
	// People are the cast and crew, in file order.
	People []Person
	// RemoteImages and LocalImages hold at most one image of each kind.
	RemoteImages []RemoteImage
	LocalImages  []LocalImage
	// UserData is the watched state recorded in the file, to be applied to
	// the configured user.
	UserData UserData
	// SeriesName is the series an episode file names; the item's series is
	// its parent.
	SeriesName string
	// Video holds stream details recorded in the file; probing the media
	// file supersedes them.
	Video VideoDetails
	// Facts are what a title's page tells of it beyond the item, which
	// is not stored.
	Facts Facts
}

// Facts are what a metadata provider tells of a title for its page, as
// Seerr shows of a movie or series, beyond what the item keeps.
type Facts struct {
	// Budget and Revenue are a movie's budget and box office takings, in
	// US dollars; 0 when unknown.
	Budget, Revenue int64
	// NextAirDate is the air date of a series' next episode, if known.
	NextAirDate *time.Time
	// WatchProviders are the services streaming it by subscription in
	// the lookup's country.
	WatchProviders []WatchProvider
}

// WatchProvider is a service streaming a title.
type WatchProvider struct {
	Name, LogoURL string
}

// Person is a credited person.
type Person struct {
	Name string
	Kind core.CreditKind
	Role string
	// Order is the billing order, nil when unknown.
	Order    *int
	ImageURL string
}

// RemoteImage is an image at a URL. Providers may describe it further:
// its size in pixels, the language of its text, empty for images without
// text, and how good it is, the best highest.
type RemoteImage struct {
	Kind          core.ImageKind
	URL           string
	Width, Height int
	Language      string
	Score         float64
}

// LocalImage is an image file.
type LocalImage struct {
	Kind core.ImageKind
	Path string
}

// UserData is a watched state; nil fields are not recorded.
type UserData struct {
	Played       *bool
	PlayCount    *int
	LastPlayedAt *time.Time
}

// VideoDetails are stream details of a video.
type VideoDetails struct {
	Width, Height int
	HasSubtitles  bool
}

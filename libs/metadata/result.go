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

// RemoteImage is an image at a URL.
type RemoteImage struct {
	Kind core.ImageKind
	URL  string
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

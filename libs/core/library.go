package core

import (
	"fmt"
	"slices"
	"time"
)

// LibraryKind determines how a library's folders are interpreted.
type LibraryKind string

// Library kinds.
const (
	LibraryMovies      LibraryKind = "movies"
	LibraryShows       LibraryKind = "shows"
	LibraryMusic       LibraryKind = "music"
	LibraryMusicVideos LibraryKind = "music_videos"
	LibraryHomeVideos  LibraryKind = "home_videos"
	LibraryBooks       LibraryKind = "books"
	LibraryPhotos      LibraryKind = "photos"
	LibraryMixed       LibraryKind = "mixed"
	// LibraryCollections and LibraryPlaylists hold the collections and
	// playlists users curate. The server creates one of each when first
	// needed; they have no folders and are never scanned.
	LibraryCollections LibraryKind = "collections"
	LibraryPlaylists   LibraryKind = "playlists"
)

// LibraryKinds lists every library kind.
var LibraryKinds = []LibraryKind{
	LibraryMovies, LibraryShows, LibraryMusic, LibraryMusicVideos,
	LibraryHomeVideos, LibraryBooks, LibraryPhotos, LibraryMixed,
	LibraryCollections, LibraryPlaylists,
}

// Valid reports whether k is a known library kind.
func (k LibraryKind) Valid() bool { return slices.Contains(LibraryKinds, k) }

// Curated reports whether k holds curated items instead of scanned
// folders.
func (k LibraryKind) Curated() bool { return k == LibraryCollections || k == LibraryPlaylists }

// Library is a named set of root folders scanned as one collection.
type Library struct {
	ID   ID
	Name string
	Kind LibraryKind
	// Paths are absolute root folders. A path belongs to at most one library.
	Paths []string
	// ScanInterval is the period between scheduled reconciliation scans; zero
	// disables scheduled scans.
	ScanInterval time.Duration
	// PreferredLanguage and MetadataCountry steer metadata providers, as
	// ISO 639-1 and ISO 3166-1 alpha-2 codes.
	PreferredLanguage string
	MetadataCountry   string
	// SaveLocalMetadata writes NFO files and chosen artwork next to the
	// media, where scans read them back.
	SaveLocalMetadata bool
	// AutoCollections puts movies into collections named after the
	// providers' collections (movie sets).
	AutoCollections bool
	// ExtractTrickplay makes thumbnail sheets of videos for seeking, and
	// ExtractChapterImages an image of each chapter.
	ExtractTrickplay     bool
	ExtractChapterImages bool
	// AnalyzeLoudness measures the loudness of audio, for normalization.
	AnalyzeLoudness bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Validate checks the library's invariants.
func (l *Library) Validate() error {
	switch {
	case l.Name == "":
		return fmt.Errorf("%w: library name is required", ErrInvalid)
	case !l.Kind.Valid():
		return fmt.Errorf("%w: unknown library kind %q", ErrInvalid, l.Kind)
	case len(l.Paths) == 0 && !l.Kind.Curated():
		return fmt.Errorf("%w: library %q has no paths", ErrInvalid, l.Name)
	case len(l.Paths) > 0 && l.Kind.Curated():
		return fmt.Errorf("%w: %s library %q cannot have paths", ErrInvalid, l.Kind, l.Name)
	case l.ScanInterval < 0:
		return fmt.Errorf("%w: negative scan interval", ErrInvalid)
	}
	return nil
}

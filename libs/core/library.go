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
	// DownloadLyrics has refreshes download lyrics for tracks without any
	// from the library's lyrics providers.
	DownloadLyrics bool
	// VirtualEpisodes configures the episodes a series lacks that
	// refreshes add to it as virtual items.
	VirtualEpisodes VirtualEpisodes
	// Providers names the providers the library uses.
	Providers ProviderOrder
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProviderOrder names the providers a library uses of each capability, by
// plugin ID, in the order they are asked. A nil list uses every provider
// in its default order; an empty one uses none.
type ProviderOrder struct {
	Metadata  []string `json:"metadata"`
	Images    []string `json:"images"`
	Subtitles []string `json:"subtitles"`
	Lyrics    []string `json:"lyrics"`
	Segments  []string `json:"segments"`
	// Episodes are the episode listers; the first that knows a series
	// lists its episodes.
	Episodes []string `json:"episodes"`
}

// VirtualEpisodes configures which episodes refreshes list as virtual
// items in series without their files, as Jellyfin's options of the TMDB
// missing episode provider. Monitored series list every episode, aired
// or not, and keep them; their specials follow Specials.
type VirtualEpisodes struct {
	// Missing lists episodes that have aired; Unaired lists those airing
	// today or later. Episodes without an air date are not listed.
	Missing bool `json:"missing"`
	Unaired bool `json:"unaired"`
	// Specials lists the episodes of season 0 too.
	Specials bool `json:"specials"`
	// GraceDays keeps virtual episodes for this many days after they air
	// when Missing is off, for their files to arrive.
	GraceDays int `json:"grace_days"`
}

// validate checks that the lists name providers once each.
func (o *ProviderOrder) validate() error {
	for _, list := range [][]string{o.Metadata, o.Images, o.Subtitles, o.Lyrics, o.Segments, o.Episodes} {
		seen := make(map[string]bool, len(list))
		for _, id := range list {
			if id == "" || seen[id] {
				return fmt.Errorf("%w: provider order names %q twice or empty", ErrInvalid, id)
			}
			seen[id] = true
		}
	}
	return nil
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
	case l.VirtualEpisodes.GraceDays < 0:
		return fmt.Errorf("%w: negative grace period of virtual episodes", ErrInvalid)
	}
	return l.Providers.validate()
}

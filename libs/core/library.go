package core

import (
	"fmt"
	"slices"
	"strings"
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
	// Paths are root folders: absolute paths of the server's file system,
	// or folders of rclone remotes ("rclone:<remote>:<path>", see
	// [Storage]). A path belongs to at most one library.
	Paths []string
	// StorageClasses sets the storage class of some of the paths, by path;
	// the others are classified by the device holding them.
	StorageClasses map[string]StorageClass
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
	// Hubs are the hubs of the library on Home and on its Recommended tab.
	Hubs      LibraryHubs
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
	for _, p := range l.Paths {
		if err := validLibraryPath(p); err != nil {
			return err
		}
	}
	for path, class := range l.StorageClasses {
		switch {
		case !slices.Contains(l.Paths, path):
			return fmt.Errorf("%w: storage class of %q, which is not a path of library %q", ErrInvalid, path, l.Name)
		case class == StorageAuto || !class.Valid():
			return fmt.Errorf("%w: storage class %q of %q", ErrInvalid, class, path)
		}
	}
	if err := l.Hubs.validate(l.Kind); err != nil {
		return err
	}
	return l.Providers.validate()
}

// StorageClass tells what reading a file of a storage costs, which decides
// how much of the media the server reads on its own.
type StorageClass string

// Storage classes.
const (
	// StorageAuto classifies the storage by the device holding it.
	StorageAuto StorageClass = ""
	// StorageLocal is a disk of the server: reads are cheap.
	StorageLocal StorageClass = "local"
	// StorageNetwork is a share on the local network, such as SMB or NFS:
	// reads are slower but not counted.
	StorageNetwork StorageClass = "network"
	// StorageCloud is a cloud drive, mounted or reached through a storage
	// backend: reads are counted against quotas and rate limits, so the
	// server reads only what playback needs and the few bytes around it.
	StorageCloud StorageClass = "cloud"
)

// StorageClasses lists every storage class.
var StorageClasses = []StorageClass{StorageAuto, StorageLocal, StorageNetwork, StorageCloud}

// Valid reports whether c is a known storage class.
func (c StorageClass) Valid() bool { return slices.Contains(StorageClasses, c) }

// StorageClassOf returns the storage class the library sets for the path
// holding the file at p, or StorageAuto: the class of the longest path of
// the library that p is in or below.
func (l *Library) StorageClassOf(p string) StorageClass {
	best, class := -1, StorageAuto
	for _, root := range l.Paths {
		if len(root) > best && InFolder(p, root) {
			best, class = len(root), l.StorageClasses[root]
		}
	}
	return class
}

// InFolder reports whether p is folder or a path below it; '/' and '\'
// both separate names, and a trailing separator of folder is ignored.
func InFolder(p, folder string) bool {
	folder = strings.TrimRight(folder, `/\`)
	if !strings.HasPrefix(p, folder) {
		return false
	}
	rest := p[len(folder):]
	return rest == "" || rest[0] == '/' || rest[0] == '\\' || folder == ""
}

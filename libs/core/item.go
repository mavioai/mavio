package core

import (
	"fmt"
	"slices"
	"time"
)

// ItemKind is the type of a library item.
type ItemKind string

// Item kinds.
const (
	KindMovie       ItemKind = "movie"
	KindSeries      ItemKind = "series"
	KindSeason      ItemKind = "season"
	KindEpisode     ItemKind = "episode"
	KindVideo       ItemKind = "video" // home videos and other standalone videos
	KindMusicArtist ItemKind = "music_artist"
	KindMusicAlbum  ItemKind = "music_album"
	KindTrack       ItemKind = "track"
	KindMusicVideo  ItemKind = "music_video"
	KindAudioBook   ItemKind = "audiobook"
	KindBook        ItemKind = "book"
	KindPhotoAlbum  ItemKind = "photo_album"
	KindPhoto       ItemKind = "photo"
	KindFolder      ItemKind = "folder"
	KindCollection  ItemKind = "collection"
	KindPlaylist    ItemKind = "playlist"
)

// ItemKinds lists every item kind.
var ItemKinds = []ItemKind{
	KindMovie, KindSeries, KindSeason, KindEpisode, KindVideo,
	KindMusicArtist, KindMusicAlbum, KindTrack, KindMusicVideo,
	KindAudioBook, KindBook, KindPhotoAlbum, KindPhoto,
	KindFolder, KindCollection, KindPlaylist,
}

// Valid reports whether k is a known item kind.
func (k ItemKind) Valid() bool { return slices.Contains(ItemKinds, k) }

// IsContainer reports whether items of this kind group other items rather
// than being playable themselves.
func (k ItemKind) IsContainer() bool {
	switch k {
	case KindSeries, KindSeason, KindMusicArtist, KindMusicAlbum, KindPhotoAlbum,
		KindFolder, KindCollection, KindPlaylist:
		return true
	}
	return false
}

// HasMedia reports whether items of this kind are backed by a playable or
// viewable media file.
func (k ItemKind) HasMedia() bool {
	switch k {
	case KindMovie, KindEpisode, KindVideo, KindTrack, KindMusicVideo, KindAudioBook:
		return true
	}
	return false
}

// ExtraKind classifies supplementary videos and audio attached to an item.
type ExtraKind string

// Extra kinds.
const (
	ExtraTrailer        ExtraKind = "trailer"
	ExtraClip           ExtraKind = "clip"
	ExtraBehindTheScene ExtraKind = "behind_the_scenes"
	ExtraDeletedScene   ExtraKind = "deleted_scene"
	ExtraInterview      ExtraKind = "interview"
	ExtraScene          ExtraKind = "scene"
	ExtraSample         ExtraKind = "sample"
	ExtraFeaturette     ExtraKind = "featurette"
	ExtraShort          ExtraKind = "short"
	ExtraThemeSong      ExtraKind = "theme_song"
	ExtraThemeVideo     ExtraKind = "theme_video"
	ExtraOther          ExtraKind = "other"
)

// ExtraKinds lists every extra kind.
var ExtraKinds = []ExtraKind{
	ExtraTrailer, ExtraClip, ExtraBehindTheScene, ExtraDeletedScene, ExtraInterview,
	ExtraScene, ExtraSample, ExtraFeaturette, ExtraShort, ExtraThemeSong, ExtraThemeVideo, ExtraOther,
}

// Valid reports whether k is a known extra kind.
func (k ExtraKind) Valid() bool { return slices.Contains(ExtraKinds, k) }

// SeriesStatus is the airing status of a series.
type SeriesStatus string

// Series statuses.
const (
	SeriesContinuing SeriesStatus = "continuing"
	SeriesEnded      SeriesStatus = "ended"
	SeriesUnreleased SeriesStatus = "unreleased"
)

// Provider names an external metadata source in ExternalIDs.
type Provider string

// Well-known providers. Plugins may use other names.
const (
	ProviderTMDB        Provider = "tmdb"
	ProviderIMDb        Provider = "imdb"
	ProviderTVDB        Provider = "tvdb"
	ProviderMusicBrainz Provider = "musicbrainz"
)

// Item is a node in a library: a playable media item, a container such as a
// series or album, or a user-curated collection or playlist.
//
// Hierarchies use ParentID: library root → series → season → episode,
// artist → album → track, and folders for everything else. Episode and season
// numbers are IndexNumber / ParentIndexNumber, as are track and disc numbers.
type Item struct {
	ID        ID
	LibraryID ID
	ParentID  ID // NilID for top-level items
	Kind      ItemKind

	Name          string
	SortName      string // derived from Name when empty
	OriginalTitle string
	Overview      string
	Tagline       string

	// Path is the file or folder the item was resolved from; empty for
	// virtual items such as collections.
	Path string

	// IndexNumber is the episode, track or season number; ParentIndexNumber
	// is the season number of an episode or the disc number of a track.
	IndexNumber       *int
	ParentIndexNumber *int
	// IndexNumberEnd is the last episode number of a multi-episode file.
	IndexNumberEnd *int

	ProductionYear  int
	PremiereDate    *time.Time
	EndDate         *time.Time
	Runtime         time.Duration
	OfficialRating  string  // content rating, e.g. "PG-13"
	CommunityRating float64 // 0–10
	CriticRating    float64 // 0–100

	Genres      []string
	Tags        []string
	Studios     []string
	ExternalIDs map[Provider]string

	// Music.
	Artists      []string
	AlbumArtists []string

	SeriesStatus SeriesStatus // series only

	// Extra is set for trailers, featurettes and other extras; OwnerID is the
	// item they belong to.
	Extra   ExtraKind
	OwnerID ID

	// DateAdded is when the item first appeared in the library; FileModified
	// is the media file's modification time, used to detect changes.
	DateAdded           time.Time
	FileModified        time.Time
	MetadataRefreshedAt time.Time
}

// Validate checks the item's invariants.
func (it *Item) Validate() error {
	switch {
	case it.ID.IsZero():
		return fmt.Errorf("%w: item has no ID", ErrInvalid)
	case it.LibraryID.IsZero():
		return fmt.Errorf("%w: item %s has no library", ErrInvalid, it.ID)
	case !it.Kind.Valid():
		return fmt.Errorf("%w: item %s has unknown kind %q", ErrInvalid, it.ID, it.Kind)
	case it.Name == "":
		return fmt.Errorf("%w: item %s has no name", ErrInvalid, it.ID)
	case it.ParentID == it.ID:
		return fmt.Errorf("%w: item %s is its own parent", ErrInvalid, it.ID)
	case it.Extra != "" && !it.Extra.Valid():
		return fmt.Errorf("%w: item %s has unknown extra kind %q", ErrInvalid, it.ID, it.Extra)
	case it.Extra != "" && it.OwnerID.IsZero():
		return fmt.Errorf("%w: extra %s has no owner", ErrInvalid, it.ID)
	case it.CommunityRating < 0 || it.CommunityRating > 10:
		return fmt.Errorf("%w: item %s community rating %v out of range", ErrInvalid, it.ID, it.CommunityRating)
	case it.CriticRating < 0 || it.CriticRating > 100:
		return fmt.Errorf("%w: item %s critic rating %v out of range", ErrInvalid, it.ID, it.CriticRating)
	}
	return nil
}

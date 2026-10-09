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
	ProviderTMDB Provider = "tmdb"
	// ProviderTMDBCollection is the TMDB collection a movie belongs to.
	ProviderTMDBCollection Provider = "tmdb_collection"
	ProviderIMDb           Provider = "imdb"
	ProviderTVDB           Provider = "tvdb"
	ProviderTVMaze         Provider = "tvmaze"
	// Anime databases, which have an entry per season rather than per
	// series.
	ProviderAniDB             Provider = "anidb"
	ProviderAniList           Provider = "anilist"
	ProviderAniSearch         Provider = "anisearch"
	ProviderMusicBrainzArtist Provider = "musicbrainz_artist"
	// MusicBrainz IDs of a track's album artist, album (release), release
	// group and track.
	ProviderMusicBrainzAlbumArtist  Provider = "musicbrainz_album_artist"
	ProviderMusicBrainzAlbum        Provider = "musicbrainz_album"
	ProviderMusicBrainzReleaseGroup Provider = "musicbrainz_release_group"
	ProviderMusicBrainzTrack        Provider = "musicbrainz_track"
)

// Video3DFormat is the stereoscopic layout of a 3D video.
type Video3DFormat string

// 3D formats.
const (
	Video3DHalfSideBySide   Video3DFormat = "half_sbs"
	Video3DFullSideBySide   Video3DFormat = "full_sbs"
	Video3DHalfTopAndBottom Video3DFormat = "half_tab"
	Video3DFullTopAndBottom Video3DFormat = "full_tab"
	Video3DMVC              Video3DFormat = "mvc"
)

// Video3DFormats lists every 3D format.
var Video3DFormats = []Video3DFormat{
	Video3DHalfSideBySide, Video3DFullSideBySide, Video3DHalfTopAndBottom, Video3DFullTopAndBottom, Video3DMVC,
}

// Valid reports whether f is a known 3D format.
func (f Video3DFormat) Valid() bool { return slices.Contains(Video3DFormats, f) }

// MetadataField names a group of item fields that metadata refreshes can
// be prevented from changing; see Item.LockedFields.
type MetadataField string

// Lockable field groups.
const (
	FieldCast                MetadataField = "cast"
	FieldGenres              MetadataField = "genres"
	FieldProductionLocations MetadataField = "production_locations"
	FieldStudios             MetadataField = "studios"
	FieldTags                MetadataField = "tags"
	FieldName                MetadataField = "name"
	FieldOverview            MetadataField = "overview"
	FieldRuntime             MetadataField = "runtime"
	FieldOfficialRating      MetadataField = "official_rating"
)

// MetadataFields lists every lockable field group.
var MetadataFields = []MetadataField{
	FieldCast, FieldGenres, FieldProductionLocations, FieldStudios, FieldTags,
	FieldName, FieldOverview, FieldRuntime, FieldOfficialRating,
}

// Valid reports whether f is a known field group.
func (f MetadataField) Valid() bool { return slices.Contains(MetadataFields, f) }

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

	ProductionYear int
	PremiereDate   *time.Time
	EndDate        *time.Time
	Runtime        time.Duration
	OfficialRating string // content rating, e.g. "PG-13"
	// CustomRating is a content rating set by the user, which takes
	// precedence over OfficialRating.
	CustomRating string
	// ParentalRating is the score of OfficialRating in its country's rating
	// system, used by rating filters; zero means unrated.
	ParentalRating int
	// InheritedRating is the score rating filters use: ParentalRating, or
	// for an unrated item the nearest rated ancestor's. The store computes
	// it on every write; values given to Upsert are ignored.
	InheritedRating int
	CommunityRating float64 // 0–10
	CriticRating    float64 // 0–100

	Genres      []string
	Tags        []string
	Studios     []string
	ExternalIDs map[Provider]string
	// ProductionLocations are the countries the item was produced in.
	ProductionLocations []string
	// RemoteTrailers are URLs of trailers hosted elsewhere, such as
	// YouTube.
	RemoteTrailers []string

	// Movie.
	// CollectionName is the collection (movie set) the movie belongs to.
	CollectionName string

	// Video.
	AspectRatio   string // display aspect ratio, e.g. "16:9" or "2.35:1"
	Video3DFormat Video3DFormat

	// Music.
	Artists      []string
	AlbumArtists []string
	Album        string // tracks and music videos

	// Series.
	SeriesStatus SeriesStatus
	AirDays      []time.Weekday
	AirTime      string // as published, e.g. "9 PM"
	// DisplayOrder is the episode order of a series, e.g. "aired", "dvd"
	// or "absolute"; empty means aired order.
	DisplayOrder string

	// Episode. A special airs before season AirsBeforeSeasonNumber, before
	// episode AirsBeforeEpisodeNumber of it, or after season
	// AirsAfterSeasonNumber.
	AirsBeforeSeasonNumber  *int
	AirsAfterSeasonNumber   *int
	AirsBeforeEpisodeNumber *int

	// Extra is set for trailers, featurettes and other extras; OwnerID is the
	// item they belong to.
	Extra   ExtraKind
	OwnerID ID

	// MetadataLanguage (ISO 639-1) and MetadataCountry (ISO 3166-1
	// alpha-2) override the library's settings for this item.
	MetadataLanguage string
	MetadataCountry  string
	// Locked keeps metadata refreshes from changing the item;
	// LockedFields protects only some field groups.
	Locked       bool
	LockedFields []MetadataField

	// DateAdded is when the item first appeared in the library; FileModified
	// is the media file's modification time, used to detect changes.
	DateAdded           time.Time
	FileModified        time.Time
	MetadataRefreshedAt time.Time

	// ScanGeneration is the library scan that last saw the item. Items a
	// complete scan did not see are missing from MissingSince, hidden from
	// queries, and purged after a grace period.
	ScanGeneration int64
	MissingSince   *time.Time
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
	case it.ParentalRating < 0:
		return fmt.Errorf("%w: item %s has a negative parental rating", ErrInvalid, it.ID)
	case it.CriticRating < 0 || it.CriticRating > 100:
		return fmt.Errorf("%w: item %s critic rating %v out of range", ErrInvalid, it.ID, it.CriticRating)
	case it.Video3DFormat != "" && !it.Video3DFormat.Valid():
		return fmt.Errorf("%w: item %s has unknown 3D format %q", ErrInvalid, it.ID, it.Video3DFormat)
	}
	for _, f := range it.LockedFields {
		if !f.Valid() {
			return fmt.Errorf("%w: item %s locks unknown field %q", ErrInvalid, it.ID, f)
		}
	}
	for _, d := range it.AirDays {
		if d < time.Sunday || d > time.Saturday {
			return fmt.Errorf("%w: item %s has invalid air day %d", ErrInvalid, it.ID, d)
		}
	}
	return nil
}

package core

import (
	"fmt"
	"slices"
	"strings"
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

// SeriesType is how a series' episodes are numbered in release names, as
// Sonarr's series types.
type SeriesType string

// Series types.
const (
	// SeriesStandard numbers episodes by season and episode.
	SeriesStandard SeriesType = ""
	// SeriesDaily identifies episodes by air date.
	SeriesDaily SeriesType = "daily"
	// SeriesAnime numbers episodes absolutely, across seasons.
	SeriesAnime SeriesType = "anime"
)

// SeriesTypes lists every series type.
var SeriesTypes = []SeriesType{SeriesStandard, SeriesDaily, SeriesAnime}

// Valid reports whether t is a known series type.
func (t SeriesType) Valid() bool { return slices.Contains(SeriesTypes, t) }

// DisplayOrder is the order a series' seasons and episodes are numbered
// and shown in, with Jellyfin's values. Every order but the aired one is a
// type of TMDB's episode groups.
type DisplayOrder string

// Display orders.
const (
	OrderAired           DisplayOrder = ""
	OrderOriginalAirDate DisplayOrder = "originalAirDate"
	OrderAbsolute        DisplayOrder = "absolute"
	OrderDVD             DisplayOrder = "dvd"
	OrderDigital         DisplayOrder = "digital"
	OrderStoryArc        DisplayOrder = "storyArc"
	OrderProduction      DisplayOrder = "production"
	OrderTV              DisplayOrder = "tv"
)

// DisplayOrders lists every display order.
var DisplayOrders = []DisplayOrder{
	OrderAired, OrderOriginalAirDate, OrderAbsolute, OrderDVD, OrderDigital, OrderStoryArc, OrderProduction, OrderTV,
}

// Valid reports whether o is a known display order.
func (o DisplayOrder) Valid() bool { return slices.Contains(DisplayOrders, o) }

// ParseDisplayOrder reads a display order case-insensitively, also Kodi's
// "aired"; ok is false for unknown orders.
func ParseDisplayOrder(s string) (o DisplayOrder, ok bool) {
	if strings.EqualFold(s, "aired") {
		return OrderAired, true
	}
	for _, o := range DisplayOrders {
		if strings.EqualFold(s, string(o)) {
			return o, true
		}
	}
	return "", false
}

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
	// items without one, such as collections and virtual items.
	Path string
	// Virtual marks a movie, series, season or episode known from metadata
	// whose file or folder the library does not have. Virtual items are hidden from queries
	// unless requested; a scan that finds the file turns the item into a
	// physical one.
	Virtual bool
	// Monitored marks a movie, series, season or episode as wanted: its
	// file is to be acquired when missing, and upgraded.
	Monitored bool

	// IndexNumber is the episode, track or season number; ParentIndexNumber
	// is the season number of an episode or the disc number of a track.
	IndexNumber       *int
	ParentIndexNumber *int
	// IndexNumberEnd is the last episode number of a multi-episode file.
	IndexNumberEnd *int
	// AbsoluteNumber is an episode's number counted across seasons, as
	// anime releases number episodes.
	AbsoluteNumber *int

	ProductionYear int
	PremiereDate   *time.Time
	EndDate        *time.Time
	Runtime        time.Duration
	OfficialRating string // content rating, e.g. "PG-13"
	// CustomRating is a content rating set by the user, which takes
	// precedence over OfficialRating.
	CustomRating string
	// ParentalRating is the score of the rating (CustomRating, else
	// OfficialRating) in its country's rating system: the minimum age it
	// stands for, from zero; nil means unrated.
	ParentalRating *int
	// InheritedRating is the score rating filters use: ParentalRating, or
	// for an unrated item the nearest rated ancestor's. The store computes
	// it on every write; values given to Upsert are ignored.
	InheritedRating *int
	CommunityRating float64 // 0–10
	CriticRating    float64 // 0–100

	Genres []string
	Tags   []string
	// InheritedTags are the LabelKeys of the item's tags and its
	// ancestors', which access records' labels match. The store computes
	// them on every write; values given to Upsert are ignored.
	InheritedTags []string
	Studios       []string
	ExternalIDs   map[Provider]string
	// ProductionLocations are the countries the item was produced in.
	ProductionLocations []string
	// RemoteTrailers are URLs of trailers hosted elsewhere, such as
	// YouTube.
	RemoteTrailers []string

	// Movie.
	// CollectionName is the collection (movie set) the movie belongs to.
	CollectionName string
	// OriginalLanguage is the language (ISO 639-1) the movie was made in;
	// empty when unknown.
	OriginalLanguage string
	// DigitalReleaseDate and PhysicalReleaseDate are when the movie came
	// out digitally and on disc; PremiereDate is its release in cinemas.
	DigitalReleaseDate  *time.Time
	PhysicalReleaseDate *time.Time
	// AlternativeTitles are other titles of the movie, and
	// TranslatedTitles its titles in other languages, by which releases
	// may name it.
	AlternativeTitles []string
	TranslatedTitles  []string
	// SecondaryYear is another year releases may give the movie, that of
	// its premiere when it differs from ProductionYear; 0 for none.
	SecondaryYear int

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
	DisplayOrder DisplayOrder
	SeriesType   SeriesType

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

	// UserID is the user a playlist belongs to.
	UserID ID

	// MetadataLanguage (ISO 639-1) and MetadataCountry (ISO 3166-1
	// alpha-2) override the library's settings for this item.
	MetadataLanguage string
	MetadataCountry  string
	// Locked keeps metadata refreshes from changing the item;
	// LockedFields protects only some field groups.
	Locked       bool
	LockedFields []MetadataField

	// Loudness is the integrated loudness of a track, or of an album's
	// tracks together, in LUFS (EBU R128); nil until measured.
	Loudness *float64

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
	case it.Kind == KindPlaylist && it.UserID.IsZero():
		return fmt.Errorf("%w: playlist %s has no user", ErrInvalid, it.ID)
	case it.Kind != KindPlaylist && !it.UserID.IsZero():
		return fmt.Errorf("%w: item %s is not a playlist but has a user", ErrInvalid, it.ID)
	case it.CommunityRating < 0 || it.CommunityRating > 10:
		return fmt.Errorf("%w: item %s community rating %v out of range", ErrInvalid, it.ID, it.CommunityRating)
	case it.ParentalRating != nil && *it.ParentalRating < 0:
		return fmt.Errorf("%w: item %s has a negative parental rating", ErrInvalid, it.ID)
	case it.CriticRating < 0 || it.CriticRating > 100:
		return fmt.Errorf("%w: item %s critic rating %v out of range", ErrInvalid, it.ID, it.CriticRating)
	case it.Video3DFormat != "" && !it.Video3DFormat.Valid():
		return fmt.Errorf("%w: item %s has unknown 3D format %q", ErrInvalid, it.ID, it.Video3DFormat)
	case !it.DisplayOrder.Valid():
		return fmt.Errorf("%w: item %s has unknown display order %q", ErrInvalid, it.ID, it.DisplayOrder)
	case !it.SeriesType.Valid():
		return fmt.Errorf("%w: item %s has unknown series type %q", ErrInvalid, it.ID, it.SeriesType)
	case it.Virtual && !virtualKinds[it.Kind]:
		return fmt.Errorf("%w: %s %s cannot be virtual", ErrInvalid, it.Kind, it.ID)
	case it.Virtual && (it.Path != "" || it.Extra != ""):
		return fmt.Errorf("%w: virtual item %s has a path or is an extra", ErrInvalid, it.ID)
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

// virtualKinds are the kinds of items that can be virtual.
var virtualKinds = map[ItemKind]bool{KindMovie: true, KindSeries: true, KindSeason: true, KindEpisode: true}

// Link is an entry of a collection or playlist: an item at a position.
// A playlist may hold an item several times, each entry with its own ID.
type Link struct {
	ID          ID
	ContainerID ID
	ItemID      ID
}

// IsCurated reports whether items link other items into it rather than
// containing them as children.
func (k ItemKind) IsCurated() bool { return k == KindCollection || k == KindPlaylist }

// NormalizationTarget is the loudness, in LUFS, normalization brings audio
// to, as Jellyfin's.
const NormalizationTarget = -18.0

// NormalizationGain returns the gain in dB that brings the item to
// NormalizationTarget, nil when its loudness is unknown.
func (it *Item) NormalizationGain() *float64 {
	if it.Loudness == nil {
		return nil
	}
	g := NormalizationTarget - *it.Loudness
	return &g
}

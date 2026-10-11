package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// The acquisition model: what libs/acquire decides with and what the
// server keeps of grabs, downloads and imports. Qualities are Radarr's
// quality IDs; languages are the names libs/release gives them, such as
// "English" or "Portuguese (Brazil)"; tags are free-form labels.

// acquisitionKinds are the item kinds acquisition settings apply to.
var acquisitionKinds = []ItemKind{KindMovie, KindSeries}

func validAcquisitionKind(k ItemKind) bool { return slices.Contains(acquisitionKinds, k) }

// QualityDefinition is the size a quality may have, in megabytes per
// minute of runtime, for movies or for series.
type QualityDefinition struct {
	Kind ItemKind
	// Quality is Radarr's quality ID.
	Quality int
	Title   string
	// The sizes are 0 for none.
	MinSize, MaxSize, PreferredSize float64
}

// Validate checks the definition's invariants.
func (d *QualityDefinition) Validate() error {
	switch {
	case !validAcquisitionKind(d.Kind):
		return fmt.Errorf("%w: quality definition of kind %q", ErrInvalid, d.Kind)
	case d.Title == "":
		return fmt.Errorf("%w: quality definition %d requires a title", ErrInvalid, d.Quality)
	case d.MinSize < 0 || d.MaxSize < 0 || d.PreferredSize < 0:
		return fmt.Errorf("%w: quality definition %d has a negative size", ErrInvalid, d.Quality)
	case d.MaxSize > 0 && d.MinSize > d.MaxSize:
		return fmt.Errorf("%w: quality definition %d has its minimum above its maximum", ErrInvalid, d.Quality)
	case d.PreferredSize > 0 && (d.PreferredSize < d.MinSize || d.MaxSize > 0 && d.PreferredSize > d.MaxSize):
		return fmt.Errorf("%w: quality definition %d prefers a size outside its limits", ErrInvalid, d.Quality)
	}
	return nil
}

// QualityProfile ranks qualities and custom formats for movies or for
// series: what to grab and when to stop upgrading. libs/acquire checks
// its items against the qualities of its kind.
type QualityProfile struct {
	ID             ID
	Kind           ItemKind
	Name           string
	UpgradeAllowed bool
	// Cutoff is the ID of a quality or of a group of Items.
	Cutoff int
	// Items are the qualities and groups, lowest first.
	Items                 []QualityProfileItem
	MinFormatScore        int
	CutoffFormatScore     int
	MinUpgradeFormatScore int
	FormatScores          []FormatScore
	// Language is the language movie releases must have: a language
	// name, "Original" for the movie's own, or empty for any.
	Language string
}

// QualityProfileItem is a quality, or a group of qualities ranked as
// one, in a profile.
type QualityProfileItem struct {
	// ID is a group's, 1000 or more; 0 for a quality.
	ID   int
	Name string
	// Quality is Radarr's quality ID, for a quality.
	Quality int
	Items   []QualityProfileItem
	Allowed bool
}

// FormatScore is the score of a custom format in a profile.
type FormatScore struct {
	FormatID ID
	Score    int
}

// Validate checks the profile's own invariants; libs/acquire checks its
// qualities.
func (p *QualityProfile) Validate() error {
	switch {
	case p.ID.IsZero():
		return fmt.Errorf("%w: quality profile requires an ID", ErrInvalid)
	case !validAcquisitionKind(p.Kind):
		return fmt.Errorf("%w: quality profile of kind %q", ErrInvalid, p.Kind)
	case strings.TrimSpace(p.Name) == "":
		return fmt.Errorf("%w: quality profile requires a name", ErrInvalid)
	case len(p.Items) == 0:
		return fmt.Errorf("%w: quality profile %q has no qualities", ErrInvalid, p.Name)
	}
	seen := map[ID]bool{}
	for _, s := range p.FormatScores {
		if s.FormatID.IsZero() || seen[s.FormatID] {
			return fmt.Errorf("%w: quality profile %q scores a custom format twice or none", ErrInvalid, p.Name)
		}
		seen[s.FormatID] = true
	}
	return nil
}

// CustomFormatSpecKind is what a custom format specification matches.
type CustomFormatSpecKind string

// The custom format specification kinds.
const (
	SpecReleaseTitle    CustomFormatSpecKind = "release_title"
	SpecEdition         CustomFormatSpecKind = "edition"
	SpecLanguage        CustomFormatSpecKind = "language"
	SpecIndexerFlag     CustomFormatSpecKind = "indexer_flag"
	SpecSource          CustomFormatSpecKind = "source"
	SpecResolution      CustomFormatSpecKind = "resolution"
	SpecQualityModifier CustomFormatSpecKind = "quality_modifier"
	SpecSize            CustomFormatSpecKind = "size"
	SpecReleaseGroup    CustomFormatSpecKind = "release_group"
	SpecYear            CustomFormatSpecKind = "year"
	SpecReleaseType     CustomFormatSpecKind = "release_type"
)

// CustomFormatSpecKinds lists the custom format specification kinds.
var CustomFormatSpecKinds = []CustomFormatSpecKind{
	SpecReleaseTitle, SpecEdition, SpecLanguage, SpecIndexerFlag, SpecSource, SpecResolution,
	SpecQualityModifier, SpecSize, SpecReleaseGroup, SpecYear, SpecReleaseType,
}

// Valid reports whether k is a known kind.
func (k CustomFormatSpecKind) Valid() bool { return slices.Contains(CustomFormatSpecKinds, k) }

// CustomFormat is a named set of conditions on releases and files that
// profiles score.
type CustomFormat struct {
	ID   ID
	Name string
	// IncludeWhenRenaming puts the format's name into file names.
	IncludeWhenRenaming bool
	Specifications      []CustomFormatSpec
}

// CustomFormatSpec is a condition of a custom format.
type CustomFormatSpec struct {
	Kind CustomFormatSpecKind
	Name string
	// Negate inverts the condition; Required makes it necessary among
	// the conditions of its kind.
	Negate, Required bool
	// Value is the regular expression of title, edition and release
	// group conditions; the language, indexer flag, source, resolution,
	// modifier or release type of the others.
	Value string
	// ExceptLanguage, for language conditions, matches any language but
	// Value.
	ExceptLanguage bool
	// Min and Max bound size (in gigabytes) and year conditions.
	Min, Max float64
}

// Validate checks the custom format's own invariants; libs/acquire
// checks its values.
func (f *CustomFormat) Validate() error {
	switch {
	case f.ID.IsZero():
		return fmt.Errorf("%w: custom format requires an ID", ErrInvalid)
	case strings.TrimSpace(f.Name) == "":
		return fmt.Errorf("%w: custom format requires a name", ErrInvalid)
	}
	for _, s := range f.Specifications {
		switch {
		case !s.Kind.Valid():
			return fmt.Errorf("%w: custom format %q has a condition of kind %q", ErrInvalid, f.Name, s.Kind)
		case strings.TrimSpace(s.Name) == "":
			return fmt.Errorf("%w: custom format %q has a condition without a name", ErrInvalid, f.Name)
		}
	}
	return nil
}

// Protocol is how a release is downloaded.
type Protocol string

// The protocols.
const (
	ProtocolUsenet  Protocol = "usenet"
	ProtocolTorrent Protocol = "torrent"
)

// Protocols lists the protocols.
var Protocols = []Protocol{ProtocolUsenet, ProtocolTorrent}

// Valid reports whether p is a known protocol.
func (p Protocol) Valid() bool { return slices.Contains(Protocols, p) }

// DelayProfile holds releases back for a while, waiting for better
// ones, for the movies and series sharing one of its tags. The default
// profile has no tags and comes last.
type DelayProfile struct {
	ID                ID
	Default           bool
	Order             int
	EnableUsenet      bool
	EnableTorrent     bool
	PreferredProtocol Protocol
	UsenetDelay       time.Duration
	TorrentDelay      time.Duration
	// The delay is bypassed for releases of the profile's best quality,
	// or scoring at least MinimumCustomFormatScore.
	BypassIfHighestQuality         bool
	BypassIfAboveCustomFormatScore bool
	MinimumCustomFormatScore       int
	Tags                           []string
}

// Validate checks the delay profile's invariants.
func (p *DelayProfile) Validate() error {
	switch {
	case p.ID.IsZero():
		return fmt.Errorf("%w: delay profile requires an ID", ErrInvalid)
	case !p.EnableUsenet && !p.EnableTorrent:
		return fmt.Errorf("%w: delay profile enables no protocol", ErrInvalid)
	case !p.PreferredProtocol.Valid():
		return fmt.Errorf("%w: delay profile prefers protocol %q", ErrInvalid, p.PreferredProtocol)
	case p.UsenetDelay < 0 || p.TorrentDelay < 0:
		return fmt.Errorf("%w: delay profile has a negative delay", ErrInvalid)
	case p.Default && len(p.Tags) > 0:
		return fmt.Errorf("%w: the default delay profile has no tags", ErrInvalid)
	case !p.Default && len(p.Tags) == 0:
		return fmt.Errorf("%w: delay profile requires a tag", ErrInvalid)
	}
	return nil
}

// ReleaseProfile requires or ignores terms in the titles of releases, for
// the movies and series sharing one of its tags (all when it has none)
// and the releases of one indexer (all when none).
type ReleaseProfile struct {
	ID      ID
	Name    string
	Enabled bool
	// Terms are plain, matched case-insensitively, or regular
	// expressions as /pattern/flags.
	Required []string
	Ignored  []string
	// IndexerID names an indexer as "<plugin ID>/<indexer ID>".
	IndexerID string
	Tags      []string
}

// Validate checks the release profile's own invariants; libs/acquire
// checks its terms.
func (p *ReleaseProfile) Validate() error {
	switch {
	case p.ID.IsZero():
		return fmt.Errorf("%w: release profile requires an ID", ErrInvalid)
	case len(p.Required) == 0 && len(p.Ignored) == 0:
		return fmt.Errorf("%w: release profile requires or ignores no term", ErrInvalid)
	case slices.ContainsFunc(append(slices.Clone(p.Required), p.Ignored...), func(t string) bool { return strings.TrimSpace(t) == "" }):
		return fmt.Errorf("%w: release profile has an empty term", ErrInvalid)
	}
	return nil
}

// MovieAvailability is the status from which a movie is wanted.
type MovieAvailability string

// The movie availabilities.
const (
	AvailabilityAnnounced MovieAvailability = "announced"
	AvailabilityInCinemas MovieAvailability = "in_cinemas"
	AvailabilityReleased  MovieAvailability = "released"
)

// MovieAvailabilities lists the movie availabilities.
var MovieAvailabilities = []MovieAvailability{AvailabilityAnnounced, AvailabilityInCinemas, AvailabilityReleased}

// Valid reports whether a is a known availability.
func (a MovieAvailability) Valid() bool { return slices.Contains(MovieAvailabilities, a) }

// NewItemMonitor is whether a wanted series monitors the seasons and
// episodes a refresh adds, as Sonarr's NewItemMonitorTypes.
type NewItemMonitor string

// The new item monitoring options.
const (
	NewItemsAll  NewItemMonitor = "all"
	NewItemsNone NewItemMonitor = "none"
)

// Valid reports whether m is a known option.
func (m NewItemMonitor) Valid() bool { return m == NewItemsAll || m == NewItemsNone }

// Wanted is how a monitored movie or series is acquired.
type Wanted struct {
	// ItemID is the movie or series.
	ItemID           ID
	QualityProfileID ID
	// MinimumAvailability is for movies.
	MinimumAvailability MovieAvailability
	// MonitorNewItems is for series; empty is NewItemsAll.
	MonitorNewItems NewItemMonitor
	// SeasonFolder is for series: whether episodes are imported into
	// season folders.
	SeasonFolder bool
	Tags         []string
	AddedAt      time.Time
	// LastSearchedAt is when the item was last searched for, if ever.
	LastSearchedAt time.Time
}

// Validate checks the wanted state's invariants.
func (w *Wanted) Validate() error {
	switch {
	case w.ItemID.IsZero() || w.QualityProfileID.IsZero():
		return fmt.Errorf("%w: wanted requires an item and a quality profile", ErrInvalid)
	case w.MinimumAvailability != "" && !w.MinimumAvailability.Valid():
		return fmt.Errorf("%w: unknown minimum availability %q", ErrInvalid, w.MinimumAvailability)
	case w.MonitorNewItems != "" && !w.MonitorNewItems.Valid():
		return fmt.Errorf("%w: unknown new item monitoring %q", ErrInvalid, w.MonitorNewItems)
	}
	return nil
}

// EpisodeNumber is an episode of a series by its TVDB season and number.
type EpisodeNumber struct {
	Season, Episode int
}

// EpisodeState is what acquisition keeps of an episode of a wanted series
// apart from its item, which a scan may replace, by its TVDB season and
// number, as Sonarr keeps it on its Episode: how the scene numbers it and
// when it was last searched for.
type EpisodeState struct {
	SeriesID ID
	EpisodeNumber
	// SceneSeason, SceneEpisode and SceneAbsolute are how the scene
	// numbers the episode; nil when as TVDB does.
	SceneSeason, SceneEpisode, SceneAbsolute *int
	// UnverifiedSceneNumbering is set when the scene numbering of the
	// episode is extrapolated from that of others, not yet confirmed.
	UnverifiedSceneNumbering bool
	// LastSearchedAt is when the episode was last searched for, if ever.
	LastSearchedAt time.Time
}

// HasSceneNumbering reports whether the scene numbers the episode apart
// from TVDB.
func (s *EpisodeState) HasSceneNumbering() bool {
	return s.SceneSeason != nil || s.SceneEpisode != nil || s.SceneAbsolute != nil || s.UnverifiedSceneNumbering
}

// QualityRef is a quality with its revision.
type QualityRef struct {
	// Quality is Radarr's quality ID.
	Quality int
	// Version counts propers and repacks; Real counts REAL releases.
	Version, Real int
	IsRepack      bool
}

// DownloadState is where a tracked download is, as Radarr's
// TrackedDownloadState.
type DownloadState string

// The download states.
const (
	DownloadDownloading   DownloadState = "downloading"
	DownloadImportBlocked DownloadState = "import_blocked"
	DownloadImportPending DownloadState = "import_pending"
	DownloadImporting     DownloadState = "importing"
	DownloadImported      DownloadState = "imported"
	DownloadFailedPending DownloadState = "failed_pending"
	DownloadFailed        DownloadState = "failed"
	DownloadIgnored       DownloadState = "ignored"
)

// DownloadStates lists the download states.
var DownloadStates = []DownloadState{
	DownloadDownloading, DownloadImportBlocked, DownloadImportPending, DownloadImporting,
	DownloadImported, DownloadFailedPending, DownloadFailed, DownloadIgnored,
}

// Valid reports whether s is a known state.
func (s DownloadState) Valid() bool { return slices.Contains(DownloadStates, s) }

// Finished reports whether the download needs no more tracking.
func (s DownloadState) Finished() bool {
	return s == DownloadImported || s == DownloadFailed || s == DownloadIgnored
}

// Download is a release sent to a download client and tracked until it
// is imported, fails or is ignored: the queue.
type Download struct {
	ID ID
	// PluginID and ClientID name the download client; DownloadID is the
	// client's ID of the download.
	PluginID, ClientID, DownloadID string
	// ItemID is the movie or series; ItemIDs are the episodes.
	ItemID  ID
	ItemIDs []ID
	Title   string
	// IndexerID names the indexer as "<plugin ID>/<indexer ID>".
	IndexerID, Indexer string
	Protocol           Protocol
	Quality            QualityRef
	Languages          []string
	Size               int64
	State              DownloadState
	// Messages explain warnings and errors, such as why an import is
	// blocked.
	Messages  []string
	GrabbedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the download's invariants.
func (d *Download) Validate() error {
	switch {
	case d.ID.IsZero() || d.ItemID.IsZero():
		return fmt.Errorf("%w: download requires an ID and an item", ErrInvalid)
	case d.PluginID == "" || d.ClientID == "" || d.DownloadID == "":
		return fmt.Errorf("%w: download %s requires its client and ID", ErrInvalid, d.ID)
	case !d.Protocol.Valid():
		return fmt.Errorf("%w: download %s has protocol %q", ErrInvalid, d.ID, d.Protocol)
	case !d.State.Valid():
		return fmt.Errorf("%w: download %s has state %q", ErrInvalid, d.ID, d.State)
	}
	return nil
}

// DownloadQuery selects downloads; zero values do not filter.
type DownloadQuery struct {
	ItemID ID
	// States are the states wanted; Unfinished selects those not
	// finished.
	States     []DownloadState
	Unfinished bool
	Limit      int
	Offset     int
}

// HistoryEventType is what a history event records.
type HistoryEventType string

// The history event types.
const (
	HistoryGrabbed        HistoryEventType = "grabbed"
	HistoryImported       HistoryEventType = "imported"
	HistoryDownloadFailed HistoryEventType = "download_failed"
	HistoryFileDeleted    HistoryEventType = "file_deleted"
	HistoryFileRenamed    HistoryEventType = "file_renamed"
	HistoryIgnored        HistoryEventType = "ignored"
)

// HistoryEventTypes lists the history event types.
var HistoryEventTypes = []HistoryEventType{
	HistoryGrabbed, HistoryImported, HistoryDownloadFailed, HistoryFileDeleted, HistoryFileRenamed, HistoryIgnored,
}

// Valid reports whether t is a known type.
func (t HistoryEventType) Valid() bool { return slices.Contains(HistoryEventTypes, t) }

// HistoryEvent is something that happened in acquiring an item.
type HistoryEvent struct {
	ID   ID
	Type HistoryEventType
	// ItemID is the movie or episode; ParentID the series of an episode.
	ItemID, ParentID ID
	Date             time.Time
	SourceTitle      string
	Quality          QualityRef
	Languages        []string
	// DownloadID is the download client's ID of the download, upper case
	// for torrent hashes.
	DownloadID string
	// Data are type-specific details, such as the indexer of a grab or
	// the reason of a failure.
	Data map[string]string
}

// Validate checks the event's invariants.
func (h *HistoryEvent) Validate() error {
	switch {
	case h.ID.IsZero() || h.ItemID.IsZero() || h.Date.IsZero():
		return fmt.Errorf("%w: history event requires an ID, an item and a date", ErrInvalid)
	case !h.Type.Valid():
		return fmt.Errorf("%w: history event of type %q", ErrInvalid, h.Type)
	}
	return nil
}

// HistoryQuery selects history events, newest first; zero values do not
// filter.
type HistoryQuery struct {
	// ItemIDs select events of these items, or of episodes of these
	// series.
	ItemIDs    []ID
	Types      []HistoryEventType
	DownloadID string
	Since      time.Time
	Limit      int
	Offset     int
}

// BlocklistEntry is a release not to grab again for an item.
type BlocklistEntry struct {
	ID ID
	// ItemID is the movie or series; ItemIDs the episodes.
	ItemID        ID
	ItemIDs       []ID
	SourceTitle   string
	Quality       QualityRef
	Languages     []string
	Date          time.Time
	PublishedDate time.Time
	Size          int64
	Protocol      Protocol
	IndexerID     string
	Indexer       string
	IndexerFlags  []string
	// Message is why the release was blocklisted.
	Message         string
	TorrentInfoHash string
}

// Validate checks the entry's invariants.
func (b *BlocklistEntry) Validate() error {
	switch {
	case b.ID.IsZero() || b.ItemID.IsZero() || b.Date.IsZero():
		return fmt.Errorf("%w: blocklist entry requires an ID, an item and a date", ErrInvalid)
	case b.SourceTitle == "":
		return fmt.Errorf("%w: blocklist entry %s requires a title", ErrInvalid, b.ID)
	case !b.Protocol.Valid():
		return fmt.Errorf("%w: blocklist entry %s has protocol %q", ErrInvalid, b.ID, b.Protocol)
	}
	return nil
}

// PendingReason is why a release is kept pending.
type PendingReason string

// The pending reasons.
const (
	// PendingDelay waits out a delay profile.
	PendingDelay PendingReason = "delay"
	// PendingClientUnavailable waits for a download client.
	PendingClientUnavailable PendingReason = "client_unavailable"
	// PendingFallback keeps a release to grab should better ones fail.
	PendingFallback PendingReason = "fallback"
)

// PendingReasons lists the pending reasons.
var PendingReasons = []PendingReason{PendingDelay, PendingClientUnavailable, PendingFallback}

// Valid reports whether r is a known reason.
func (r PendingReason) Valid() bool { return slices.Contains(PendingReasons, r) }

// PendingRelease is a release kept to grab later.
type PendingRelease struct {
	ID ID
	// ItemID is the movie or series; ItemIDs the episodes.
	ItemID  ID
	ItemIDs []ID
	Title   string
	Added   time.Time
	Reason  PendingReason
	// Release is the release as libs/acquire encodes it.
	Release json.RawMessage
}

// Validate checks the pending release's invariants.
func (p *PendingRelease) Validate() error {
	switch {
	case p.ID.IsZero() || p.ItemID.IsZero() || p.Added.IsZero():
		return fmt.Errorf("%w: pending release requires an ID, an item and a time", ErrInvalid)
	case !p.Reason.Valid():
		return fmt.Errorf("%w: pending release %s has reason %q", ErrInvalid, p.ID, p.Reason)
	case !json.Valid(p.Release):
		return fmt.Errorf("%w: pending release %s has no valid release", ErrInvalid, p.ID)
	}
	return nil
}

// RemotePathMapping maps the paths a download client on another host
// reports to the paths the server sees.
type RemotePathMapping struct {
	ID ID
	// Host is the download client's host, as its settings name it.
	Host string
	// RemotePath and LocalPath are folders.
	RemotePath, LocalPath string
}

// Validate checks the mapping's invariants.
func (m *RemotePathMapping) Validate() error {
	switch {
	case m.ID.IsZero() || strings.TrimSpace(m.Host) == "":
		return fmt.Errorf("%w: remote path mapping requires an ID and a host", ErrInvalid)
	case strings.TrimSpace(m.RemotePath) == "":
		return fmt.Errorf("%w: remote path mapping requires a remote path", ErrInvalid)
	case !isAbs(m.LocalPath):
		return fmt.Errorf("%w: remote path mapping requires an absolute local path", ErrInvalid)
	}
	return nil
}

// Repositories of acquisition.
type (
	// QualityDefinitionRepository stores the quality definitions.
	QualityDefinitionRepository interface {
		// List returns the definitions of a kind, by quality; none until
		// stored.
		List(ctx context.Context, kind ItemKind) ([]QualityDefinition, error)
		// Replace sets the definitions of a kind.
		Replace(ctx context.Context, kind ItemKind, defs []QualityDefinition) error
	}

	// QualityProfileRepository stores quality profiles. Names are unique
	// per kind (ErrConflict); a profile wanted items use cannot be deleted
	// (ErrConflict).
	QualityProfileRepository interface {
		Get(ctx context.Context, id ID) (QualityProfile, error)
		// List returns the profiles of a kind by name, or all when kind is
		// empty.
		List(ctx context.Context, kind ItemKind) ([]QualityProfile, error)
		Put(ctx context.Context, p *QualityProfile) error
		Delete(ctx context.Context, id ID) error
	}

	// CustomFormatRepository stores custom formats. Names are unique
	// (ErrConflict); deleting a format removes its scores from profiles.
	CustomFormatRepository interface {
		Get(ctx context.Context, id ID) (CustomFormat, error)
		// List returns the formats by name.
		List(ctx context.Context) ([]CustomFormat, error)
		Put(ctx context.Context, f *CustomFormat) error
		Delete(ctx context.Context, id ID) error
	}

	// DelayProfileRepository stores delay profiles. The default profile
	// cannot be deleted (ErrConflict).
	DelayProfileRepository interface {
		Get(ctx context.Context, id ID) (DelayProfile, error)
		// List returns the profiles by order.
		List(ctx context.Context) ([]DelayProfile, error)
		Put(ctx context.Context, p *DelayProfile) error
		Delete(ctx context.Context, id ID) error
	}

	// ReleaseProfileRepository stores release profiles.
	ReleaseProfileRepository interface {
		Get(ctx context.Context, id ID) (ReleaseProfile, error)
		// List returns the profiles by name.
		List(ctx context.Context) ([]ReleaseProfile, error)
		Put(ctx context.Context, p *ReleaseProfile) error
		Delete(ctx context.Context, id ID) error
	}

	// WantedRepository stores how monitored items are acquired; deleting
	// the item deletes it.
	WantedRepository interface {
		// Get returns ErrNotFound for an item never wanted.
		Get(ctx context.Context, itemID ID) (Wanted, error)
		// List returns the wanted states of items, or all when none are
		// given.
		List(ctx context.Context, itemIDs ...ID) ([]Wanted, error)
		Put(ctx context.Context, w *Wanted) error
		Delete(ctx context.Context, itemID ID) error
	}

	// EpisodeStateRepository stores the states of episodes of series;
	// deleting the series deletes them.
	EpisodeStateRepository interface {
		// List returns the states of a series' episodes by season and
		// episode.
		List(ctx context.Context, seriesID ID) ([]EpisodeState, error)
		// PutSceneNumbering replaces the scene numbering of a series'
		// episodes: those of states take theirs, the others none. Last
		// search times are kept.
		PutSceneNumbering(ctx context.Context, seriesID ID, states []EpisodeState) error
		// Searched records that episodes of a series were searched for at
		// a time.
		Searched(ctx context.Context, seriesID ID, at time.Time, episodes ...EpisodeNumber) error
	}

	// DownloadRepository stores tracked downloads. A client's download is
	// tracked once (ErrConflict); deleting the item deletes its downloads.
	DownloadRepository interface {
		Get(ctx context.Context, id ID) (Download, error)
		// GetByDownloadID returns the download a client knows by an ID.
		GetByDownloadID(ctx context.Context, pluginID, clientID, downloadID string) (Download, error)
		// List returns the downloads q selects, most recently grabbed
		// first.
		List(ctx context.Context, q DownloadQuery) (Page[Download], error)
		Put(ctx context.Context, d *Download) error
		Delete(ctx context.Context, id ID) error
	}

	// HistoryRepository stores acquisition history; deleting an item
	// deletes its events.
	HistoryRepository interface {
		Add(ctx context.Context, events ...HistoryEvent) error
		// List returns the events q selects, newest first.
		List(ctx context.Context, q HistoryQuery) (Page[HistoryEvent], error)
		// Purge deletes the events before a time and reports how many.
		Purge(ctx context.Context, before time.Time) (int, error)
		// MoveItems gives the events of items to another item, which
		// takes their place, as the item of a multi-episode file takes
		// that of the episodes it covers.
		MoveItems(ctx context.Context, to ID, from ...ID) error
	}

	// BlocklistRepository stores blocklisted releases; deleting the item
	// deletes its entries.
	BlocklistRepository interface {
		Add(ctx context.Context, b *BlocklistEntry) error
		// List returns the entries of an item, or all when itemID is
		// zero, newest first.
		List(ctx context.Context, itemID ID, limit, offset int) (Page[BlocklistEntry], error)
		Delete(ctx context.Context, ids ...ID) error
	}

	// PendingReleaseRepository stores releases kept pending; deleting the
	// item deletes them.
	PendingReleaseRepository interface {
		// List returns the pending releases of an item, or all when itemID
		// is zero, oldest first.
		List(ctx context.Context, itemID ID) ([]PendingRelease, error)
		Add(ctx context.Context, p *PendingRelease) error
		Delete(ctx context.Context, ids ...ID) error
	}

	// RemotePathMappingRepository stores remote path mappings. A host's
	// remote path is mapped once (ErrConflict).
	RemotePathMappingRepository interface {
		// List returns the mappings by host and remote path.
		List(ctx context.Context) ([]RemotePathMapping, error)
		Put(ctx context.Context, m *RemotePathMapping) error
		Delete(ctx context.Context, id ID) error
	}
)

// ProperDownload is what to do with propers and repacks.
type ProperDownload string

// The proper download settings.
const (
	// PropersPreferAndUpgrade prefers propers and repacks and upgrades
	// files to them.
	PropersPreferAndUpgrade ProperDownload = "prefer_and_upgrade"
	// PropersDoNotUpgrade prefers them but does not upgrade to them.
	PropersDoNotUpgrade ProperDownload = "do_not_upgrade"
	// PropersDoNotPrefer leaves them to custom formats.
	PropersDoNotPrefer ProperDownload = "do_not_prefer"
)

// ProperDownloads lists the proper download settings.
var ProperDownloads = []ProperDownload{PropersPreferAndUpgrade, PropersDoNotUpgrade, PropersDoNotPrefer}

// Valid reports whether p is a known setting.
func (p ProperDownload) Valid() bool { return slices.Contains(ProperDownloads, p) }

// ColonReplacement is what colons in titles become in file names.
type ColonReplacement string

// The colon replacements.
const (
	ColonDelete         ColonReplacement = "delete"
	ColonDash           ColonReplacement = "dash"
	ColonSpaceDash      ColonReplacement = "space_dash"
	ColonSpaceDashSpace ColonReplacement = "space_dash_space"
	// ColonSmart uses a dash between words and deletes a colon before a
	// space.
	ColonSmart ColonReplacement = "smart"
)

// ColonReplacements lists the colon replacements.
var ColonReplacements = []ColonReplacement{ColonDelete, ColonDash, ColonSpaceDash, ColonSpaceDashSpace, ColonSmart}

// Valid reports whether c is a known replacement.
func (c ColonReplacement) Valid() bool { return slices.Contains(ColonReplacements, c) }

// MultiEpisodeStyle is how the name of a file holding several episodes
// writes their numbers, as Sonarr's MultiEpisodeStyle.
type MultiEpisodeStyle string

// The multi-episode styles, shown for episodes 6 to 8 of season 15 in
// the format "S{season:00}E{episode:00}".
const (
	// MultiEpisodeExtend writes S15E06-07-08.
	MultiEpisodeExtend MultiEpisodeStyle = "extend"
	// MultiEpisodeDuplicate writes S15E06 - S15E07 - S15E08.
	MultiEpisodeDuplicate MultiEpisodeStyle = "duplicate"
	// MultiEpisodeRepeat writes S15E06E07E08.
	MultiEpisodeRepeat MultiEpisodeStyle = "repeat"
	// MultiEpisodeScene writes S15E06-E07-E08.
	MultiEpisodeScene MultiEpisodeStyle = "scene"
	// MultiEpisodeRange writes S15E06-08.
	MultiEpisodeRange MultiEpisodeStyle = "range"
	// MultiEpisodePrefixedRange writes S15E06-E08, Sonarr's default.
	MultiEpisodePrefixedRange MultiEpisodeStyle = "prefixed_range"
)

// MultiEpisodeStyles lists the multi-episode styles.
var MultiEpisodeStyles = []MultiEpisodeStyle{
	MultiEpisodeExtend, MultiEpisodeDuplicate, MultiEpisodeRepeat, MultiEpisodeScene, MultiEpisodeRange, MultiEpisodePrefixedRange,
}

// Valid reports whether s is a known style.
func (s MultiEpisodeStyle) Valid() bool { return slices.Contains(MultiEpisodeStyles, s) }

// EpisodeTitleRequired is when an episode imported under a name that
// shows its title must have one, as Sonarr's EpisodeTitleRequiredType.
type EpisodeTitleRequired string

// The episode title requirements.
const (
	// EpisodeTitleAlways requires the title of episodes aired within
	// the last two days.
	EpisodeTitleAlways EpisodeTitleRequired = "always"
	// EpisodeTitleBulkSeasonReleases requires it but of the episodes of
	// a season released at once.
	EpisodeTitleBulkSeasonReleases EpisodeTitleRequired = "bulk_season_releases"
	// EpisodeTitleNever requires no title.
	EpisodeTitleNever EpisodeTitleRequired = "never"
)

// EpisodeTitleRequirements lists the episode title requirements.
var EpisodeTitleRequirements = []EpisodeTitleRequired{EpisodeTitleAlways, EpisodeTitleBulkSeasonReleases, EpisodeTitleNever}

// Valid reports whether r is a known requirement.
func (r EpisodeTitleRequired) Valid() bool { return slices.Contains(EpisodeTitleRequirements, r) }

// AcquisitionSettings are how releases are chosen, downloaded and
// imported, as Radarr's and Sonarr's settings.
type AcquisitionSettings struct {
	Propers            ProperDownload
	PreferIndexerFlags bool
	// CompletedDownloadHandling imports finished downloads.
	CompletedDownloadHandling bool
	// AutoRedownloadFailed searches again when a download fails.
	AutoRedownloadFailed bool
	// AutoRedownloadFailedFromInteractiveSearch also searches again when
	// the failed release was grabbed from an interactive search.
	AutoRedownloadFailedFromInteractiveSearch bool
	// AutoUnmonitorPreviouslyDownloadedEpisodes unmonitors the episodes
	// of wanted series whose files go missing, as Sonarr's setting.
	AutoUnmonitorPreviouslyDownloadedEpisodes bool
	// EpisodeTitleRequired is when episodes imported under names showing
	// their titles must have one; empty is EpisodeTitleAlways.
	EpisodeTitleRequired EpisodeTitleRequired
	// MaximumSize is in megabytes; 0 for none.
	MaximumSize int
	// MinimumAge is how old usenet releases must be, in minutes.
	MinimumAge int
	// Retention is the usenet retention in days; 0 for none.
	Retention          int
	AllowHardcodedSubs bool
	// WhitelistedHardcodedSubs are the hardcoded subtitles allowed when
	// others are not.
	WhitelistedHardcodedSubs []string
	SkipFreeSpaceCheck       bool
	// MinimumFreeSpace is what an import must leave, in megabytes.
	MinimumFreeSpace int
	// AvailabilityDelay shifts when movies count as available, in days.
	AvailabilityDelay int
	// RSSSyncInterval is in minutes; 0 disables RSS sync.
	RSSSyncInterval int
	// CheckFinishedInterval is how often download clients are polled, in
	// minutes.
	CheckFinishedInterval int
	// RecycleBin is the folder replaced and deleted files go to; empty
	// deletes them.
	RecycleBin            string
	RecycleBinCleanupDays int
	// CopyUsingHardlinks links instead of copying files whose download
	// keeps seeding.
	CopyUsingHardlinks bool
	ImportExtraFiles   bool
	// ExtraFileExtensions are imported with the files, such as "srt".
	ExtraFileExtensions []string
	DeleteEmptyFolders  bool
	Naming              NamingSettings
}

// NamingSettings are how imported files and folders are named.
type NamingSettings struct {
	// RenameMovies names movie files by StandardMovieFormat; otherwise
	// they keep their names.
	RenameMovies             bool
	ReplaceIllegalCharacters bool
	ColonReplacement         ColonReplacement
	// The templates, such as "{Movie Title} ({Release Year})".
	MovieFolderFormat   string
	StandardMovieFormat string
	// RenameEpisodes names episode files by the episode formats;
	// otherwise they keep their names.
	RenameEpisodes bool
	// The templates of episode files of standard, daily and anime series,
	// such as "{Series Title} - S{season:00}E{episode:00}", and of the
	// folders of series, seasons and specials; empty for Sonarr's
	// defaults.
	StandardEpisodeFormat string
	DailyEpisodeFormat    string
	AnimeEpisodeFormat    string
	SeriesFolderFormat    string
	SeasonFolderFormat    string
	SpecialsFolderFormat  string
	// MultiEpisodeStyle is how files of several episodes are named; empty
	// for MultiEpisodePrefixedRange.
	MultiEpisodeStyle MultiEpisodeStyle
}

// DefaultAcquisitionSettings are Radarr's and Sonarr's defaults.
func DefaultAcquisitionSettings() AcquisitionSettings {
	return AcquisitionSettings{
		Propers:                   PropersPreferAndUpgrade,
		CompletedDownloadHandling: true,
		AutoRedownloadFailed:      true,
		AutoRedownloadFailedFromInteractiveSearch: true,
		EpisodeTitleRequired:                      EpisodeTitleAlways,
		MinimumFreeSpace:                          100,
		RSSSyncInterval:                           30,
		CheckFinishedInterval:                     1,
		RecycleBinCleanupDays:                     7,
		CopyUsingHardlinks:                        true,
		ExtraFileExtensions:                       []string{"srt"},
		Naming: NamingSettings{
			ReplaceIllegalCharacters: true,
			ColonReplacement:         ColonSmart,
			MovieFolderFormat:        "{Movie Title} ({Release Year})",
			StandardMovieFormat:      "{Movie Title} ({Release Year}) {Quality Full}",
			StandardEpisodeFormat:    "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
			DailyEpisodeFormat:       "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
			AnimeEpisodeFormat:       "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
			SeriesFolderFormat:       "{Series Title}",
			SeasonFolderFormat:       "Season {season}",
			SpecialsFolderFormat:     "Specials",
			MultiEpisodeStyle:        MultiEpisodePrefixedRange,
		},
	}
}

// Validate checks the settings' invariants.
func (a *AcquisitionSettings) Validate() error {
	switch {
	case !a.Propers.Valid():
		return fmt.Errorf("%w: unknown proper download setting %q", ErrInvalid, a.Propers)
	case a.MaximumSize < 0 || a.MinimumAge < 0 || a.Retention < 0 || a.MinimumFreeSpace < 0 ||
		a.AvailabilityDelay < -365 || a.AvailabilityDelay > 365 || a.RecycleBinCleanupDays < 0:
		return fmt.Errorf("%w: acquisition setting out of range", ErrInvalid)
	case a.RSSSyncInterval != 0 && (a.RSSSyncInterval < 10 || a.RSSSyncInterval > 120):
		return fmt.Errorf("%w: RSS sync interval outside 10–120 minutes", ErrInvalid)
	case a.CheckFinishedInterval < 1 || a.CheckFinishedInterval > 120:
		return fmt.Errorf("%w: download check interval outside 1–120 minutes", ErrInvalid)
	case a.RecycleBin != "" && !isAbs(a.RecycleBin):
		return fmt.Errorf("%w: recycle bin %q is not absolute", ErrInvalid, a.RecycleBin)
	case !a.Naming.ColonReplacement.Valid():
		return fmt.Errorf("%w: unknown colon replacement %q", ErrInvalid, a.Naming.ColonReplacement)
	case strings.TrimSpace(a.Naming.MovieFolderFormat) == "" || strings.TrimSpace(a.Naming.StandardMovieFormat) == "":
		return fmt.Errorf("%w: movie naming requires folder and file formats", ErrInvalid)
	case a.Naming.MultiEpisodeStyle != "" && !a.Naming.MultiEpisodeStyle.Valid():
		return fmt.Errorf("%w: unknown multi-episode style %q", ErrInvalid, a.Naming.MultiEpisodeStyle)
	case a.EpisodeTitleRequired != "" && !a.EpisodeTitleRequired.Valid():
		return fmt.Errorf("%w: unknown episode title requirement %q", ErrInvalid, a.EpisodeTitleRequired)
	}
	return nil
}

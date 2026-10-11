package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Requests and issues, as Seerr's: users request movies and seasons of
// series, which administrators approve, or which are approved at once,
// and report problems with the items they watch.

// RequestStatus is where a request is, as Seerr's MediaRequestStatus.
type RequestStatus string

// The request statuses.
const (
	// RequestPending waits for an administrator.
	RequestPending RequestStatus = "pending"
	// RequestApproved was approved: its movie or seasons are wanted.
	RequestApproved RequestStatus = "approved"
	// RequestDeclined was declined; it uses no quota.
	RequestDeclined RequestStatus = "declined"
	// RequestFailed was approved, but its movie or seasons could not be
	// made wanted; it may be retried.
	RequestFailed RequestStatus = "failed"
	// RequestCompleted has its movie or seasons available.
	RequestCompleted RequestStatus = "completed"
)

// RequestStatuses lists the request statuses.
var RequestStatuses = []RequestStatus{RequestPending, RequestApproved, RequestDeclined, RequestFailed, RequestCompleted}

// Valid reports whether s is a known status.
func (s RequestStatus) Valid() bool { return slices.Contains(RequestStatuses, s) }

// Active reports whether a request in this status holds its movie or
// seasons, so that they cannot be requested again: pending, approved and
// failed requests do.
func (s RequestStatus) Active() bool {
	return s == RequestPending || s == RequestApproved || s == RequestFailed
}

// SeasonRequest is a season a series request asks for, with its own
// status, as Seerr's SeasonRequest.
type SeasonRequest struct {
	// Number is the season's number in TVDB's order.
	Number int
	Status RequestStatus
}

// Request is a user's request of a movie or of seasons of a series, as
// Seerr's MediaRequest.
type Request struct {
	ID ID
	// Kind is KindMovie or KindSeries.
	Kind ItemKind
	// TmdbID identifies the title; TvdbID too for series, once known.
	TmdbID int
	TvdbID int
	// Title and Year name the title as it was requested.
	Title string
	Year  int
	// ItemID is the movie or series item wanted for it, once approved.
	ItemID ID
	// Seasons are the seasons a series request asks for; none for movies.
	Seasons []SeasonRequest
	Status  RequestStatus
	// RequestedBy is the user who asked; ModifiedBy the last who approved,
	// declined or retried it, zero when none did.
	RequestedBy ID
	ModifiedBy  ID
	// IgnoreQuota leaves the request out of its user's quota.
	IgnoreQuota bool
	// Message is why it failed.
	Message   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the request's invariants.
func (r *Request) Validate() error {
	switch {
	case r.ID.IsZero() || r.RequestedBy.IsZero():
		return fmt.Errorf("%w: request requires an ID and a user", ErrInvalid)
	case r.Kind != KindMovie && r.Kind != KindSeries:
		return fmt.Errorf("%w: request of kind %q", ErrInvalid, r.Kind)
	case r.TmdbID <= 0:
		return fmt.Errorf("%w: request %s requires a TMDB ID", ErrInvalid, r.ID)
	case r.TvdbID < 0:
		return fmt.Errorf("%w: request %s has TVDB ID %d", ErrInvalid, r.ID, r.TvdbID)
	case !r.Status.Valid():
		return fmt.Errorf("%w: request %s has unknown status %q", ErrInvalid, r.ID, r.Status)
	case r.Kind == KindMovie && len(r.Seasons) > 0:
		return fmt.Errorf("%w: movie request %s with seasons", ErrInvalid, r.ID)
	case r.Kind == KindSeries && len(r.Seasons) == 0:
		return fmt.Errorf("%w: series request %s without seasons", ErrInvalid, r.ID)
	}
	seen := map[int]bool{}
	for _, s := range r.Seasons {
		switch {
		case s.Number < 0:
			return fmt.Errorf("%w: request %s of season %d", ErrInvalid, r.ID, s.Number)
		case seen[s.Number]:
			return fmt.Errorf("%w: request %s asks for season %d twice", ErrInvalid, r.ID, s.Number)
		case !s.Status.Valid():
			return fmt.Errorf("%w: request %s has season %d of unknown status %q", ErrInvalid, r.ID, s.Number, s.Status)
		}
		seen[s.Number] = true
	}
	return nil
}

// SeasonNumbers returns the numbers of the seasons requested, in order.
func (r *Request) SeasonNumbers() []int {
	out := make([]int, len(r.Seasons))
	for i, s := range r.Seasons {
		out[i] = s.Number
	}
	slices.Sort(out)
	return out
}

// RequestQuery selects requests, newest first unless SortByModified
// orders them by when they last changed; unset fields do not filter.
type RequestQuery struct {
	RequestedBy ID
	Statuses    []RequestStatus
	Kind        ItemKind
	TmdbID      int
	ItemID      ID
	// SortByModified orders by UpdatedAt rather than by creation;
	// Ascending reverses the order.
	SortByModified bool
	Ascending      bool
	Limit          int // zero means MaxPageSize
	Offset         int
}

// Validate checks the query's bounds.
func (q *RequestQuery) Validate() error {
	switch {
	case q.Limit < 0 || q.Limit > MaxPageSize || q.Offset < 0:
		return fmt.Errorf("%w: page outside 0–%d", ErrInvalid, MaxPageSize)
	case q.Kind != "" && q.Kind != KindMovie && q.Kind != KindSeries:
		return fmt.Errorf("%w: request kind %q", ErrInvalid, q.Kind)
	}
	for _, s := range q.Statuses {
		if !s.Valid() {
			return fmt.Errorf("%w: unknown request status %q", ErrInvalid, s)
		}
	}
	return nil
}

// IssueType is what kind of problem an issue reports, as Seerr's.
type IssueType string

// The issue types.
const (
	IssueVideo     IssueType = "video"
	IssueAudio     IssueType = "audio"
	IssueSubtitles IssueType = "subtitles"
	IssueOther     IssueType = "other"
)

// IssueTypes lists the issue types.
var IssueTypes = []IssueType{IssueVideo, IssueAudio, IssueSubtitles, IssueOther}

// Valid reports whether t is a known type.
func (t IssueType) Valid() bool { return slices.Contains(IssueTypes, t) }

// IssueStatus is whether an issue is open.
type IssueStatus string

// The issue statuses.
const (
	IssueOpen     IssueStatus = "open"
	IssueResolved IssueStatus = "resolved"
)

// Valid reports whether s is a known status.
func (s IssueStatus) Valid() bool { return s == IssueOpen || s == IssueResolved }

// Issue is a problem a user reports with an item, as Seerr's Issue: its
// first comment says what it is.
type Issue struct {
	ID ID
	// ItemID is the movie, series, season or episode.
	ItemID ID
	Type   IssueType
	Status IssueStatus
	// SeasonNumber and EpisodeNumber place the problem in a series; 0 for
	// none.
	SeasonNumber, EpisodeNumber int
	CreatedBy                   ID
	// ModifiedBy is who last resolved or reopened it; zero when none did.
	ModifiedBy ID
	// Comments are its comments, oldest first; the repository reads them
	// with the issue and adds them on their own.
	Comments  []IssueComment
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the issue's invariants.
func (i *Issue) Validate() error {
	switch {
	case i.ID.IsZero() || i.ItemID.IsZero() || i.CreatedBy.IsZero():
		return fmt.Errorf("%w: issue requires an ID, an item and a user", ErrInvalid)
	case !i.Type.Valid():
		return fmt.Errorf("%w: issue %s has unknown type %q", ErrInvalid, i.ID, i.Type)
	case !i.Status.Valid():
		return fmt.Errorf("%w: issue %s has unknown status %q", ErrInvalid, i.ID, i.Status)
	case i.SeasonNumber < 0 || i.EpisodeNumber < 0:
		return fmt.Errorf("%w: issue %s has a negative season or episode", ErrInvalid, i.ID)
	}
	return nil
}

// IssueComment is a comment on an issue.
type IssueComment struct {
	ID      ID
	IssueID ID
	UserID  ID
	Message string
	// CreatedAt and UpdatedAt are set by the repository when zero.
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the comment's invariants.
func (c *IssueComment) Validate() error {
	switch {
	case c.ID.IsZero() || c.IssueID.IsZero() || c.UserID.IsZero():
		return fmt.Errorf("%w: issue comment requires an ID, an issue and a user", ErrInvalid)
	case strings.TrimSpace(c.Message) == "":
		return fmt.Errorf("%w: issue comment %s requires a message", ErrInvalid, c.ID)
	}
	return nil
}

// IssueQuery selects issues, newest first unless SortByModified orders
// them by when they last changed; unset fields do not filter.
type IssueQuery struct {
	CreatedBy      ID
	ItemID         ID
	Statuses       []IssueStatus
	SortByModified bool
	Limit          int // zero means MaxPageSize
	Offset         int
}

// Validate checks the query's bounds.
func (q *IssueQuery) Validate() error {
	if q.Limit < 0 || q.Limit > MaxPageSize || q.Offset < 0 {
		return fmt.Errorf("%w: page outside 0–%d", ErrInvalid, MaxPageSize)
	}
	for _, s := range q.Statuses {
		if !s.Valid() {
			return fmt.Errorf("%w: unknown issue status %q", ErrInvalid, s)
		}
	}
	return nil
}

// BlockedTitle is a movie or series administrators blocked from being
// requested, as Seerr's Blocklist.
type BlockedTitle struct {
	// Kind is KindMovie or KindSeries.
	Kind   ItemKind
	TmdbID int
	// Title names it as it was blocklisted; empty when unknown.
	Title string
	// UserID is the administrator who blocklisted it; zero once they are
	// deleted, and for titles blocklisted by tags.
	UserID ID
	// Tags are the blocklisted tags ("<plugin>/<keyword>") of
	// RequestSettings.BlocklistedTags the title has, for which the
	// blocklisted tags job blocklisted it, as Seerr's
	// Blocklist.blocklistedTags; empty for titles blocklisted by hand.
	Tags []string
	// CreatedAt is set by the repository when zero.
	CreatedAt time.Time
}

// Validate checks the blocked title's invariants.
func (b *BlockedTitle) Validate() error {
	switch {
	case b.Kind != KindMovie && b.Kind != KindSeries:
		return fmt.Errorf("%w: blocked title of kind %q", ErrInvalid, b.Kind)
	case b.TmdbID <= 0:
		return fmt.Errorf("%w: blocked title requires a TMDB ID", ErrInvalid)
	}
	return nil
}

// BlockedTitleSource is how titles came on the blocklist, as Seerr's
// blocklist filters.
type BlockedTitleSource string

// The sources of blocked titles.
const (
	// BlockedByHand are the titles administrators blocklisted.
	BlockedByHand BlockedTitleSource = "manual"
	// BlockedByTags are the titles the blocklisted tags job blocklisted.
	BlockedByTags BlockedTitleSource = "tags"
)

// BlockedTitleQuery selects blocked titles, newest first.
type BlockedTitleQuery struct {
	// Search matches titles containing it, ignoring case; empty matches
	// all.
	Search string
	// Source selects the titles blocklisted by hand or by tags; empty
	// selects both.
	Source BlockedTitleSource
	Limit  int // zero means MaxPageSize
	Offset int
}

// Validate checks the query's bounds.
func (q *BlockedTitleQuery) Validate() error {
	switch {
	case q.Limit < 0 || q.Limit > MaxPageSize || q.Offset < 0:
		return fmt.Errorf("%w: page outside 0–%d", ErrInvalid, MaxPageSize)
	case q.Source != "" && q.Source != BlockedByHand && q.Source != BlockedByTags:
		return fmt.Errorf("%w: unknown blocked title source %q", ErrInvalid, q.Source)
	}
	return nil
}

// Repositories of requests and issues.
type (
	// BlockedTitleRepository stores the blocklist of titles, one entry
	// per kind and TMDB ID.
	BlockedTitleRepository interface {
		// Get returns ErrNotFound for a title not blocklisted.
		Get(ctx context.Context, kind ItemKind, tmdbID int) (BlockedTitle, error)
		// List returns the blocked titles q selects.
		List(ctx context.Context, q BlockedTitleQuery) (Page[BlockedTitle], error)
		// Add blocklists a title; ErrConflict when it is already.
		Add(ctx context.Context, b *BlockedTitle) error
		// Delete takes a title off the blocklist; ErrNotFound when it is
		// not on it.
		Delete(ctx context.Context, kind ItemKind, tmdbID int) error
		// DeleteTagged takes the titles blocklisted by tags off the
		// blocklist and returns how many there were.
		DeleteTagged(ctx context.Context) (int, error)
	}

	// RequestRepository stores requests; deleting the user deletes their
	// requests.
	RequestRepository interface {
		// Get returns ErrNotFound for an unknown request.
		Get(ctx context.Context, id ID) (Request, error)
		// List returns the requests q selects.
		List(ctx context.Context, q RequestQuery) (Page[Request], error)
		// Put inserts or replaces a request; zero times are now.
		Put(ctx context.Context, r *Request) error
		Delete(ctx context.Context, id ID) error
		// QuotaUsed returns how much of a user's quota of a kind the
		// requests created after since use, as Seerr's User.getQuota
		// counts it: movie requests, or the seasons of series requests,
		// neither declined nor ignoring quotas. A zero since counts all.
		QuotaUsed(ctx context.Context, userID ID, kind ItemKind, since time.Time) (int, error)
	}

	// IssueRepository stores issues and their comments; deleting the
	// item or the user deletes their issues, deleting a user their
	// comments.
	IssueRepository interface {
		// Get returns an issue with its comments; ErrNotFound when
		// unknown.
		Get(ctx context.Context, id ID) (Issue, error)
		// List returns the issues q selects, with their comments.
		List(ctx context.Context, q IssueQuery) (Page[Issue], error)
		// Put inserts or replaces an issue, leaving its comments; zero
		// times are now.
		Put(ctx context.Context, i *Issue) error
		// AddComment adds a comment to an issue.
		AddComment(ctx context.Context, c *IssueComment) error
		// Delete removes an issue with its comments.
		Delete(ctx context.Context, id ID) error
	}
)

// RequestPermissions are what a user may request and report, Mavio's
// booleans for Seerr's permission bits; administrators may do all and
// manage requests and issues.
type RequestPermissions struct {
	// Movies and Series allow requesting them (Seerr's REQUEST_MOVIE,
	// REQUEST_TV).
	Movies, Series bool
	// AutoApproveMovies and AutoApproveSeries approve the user's
	// requests at once (AUTO_APPROVE_MOVIE, AUTO_APPROVE_TV).
	AutoApproveMovies, AutoApproveSeries bool
	// ReportIssues allows reporting issues and commenting on one's own
	// (CREATE_ISSUES).
	ReportIssues bool
}

// Quota bounds how much a user requests: Limit requests of movies, or
// seasons of series, in the last Days days (all time when 0). A zero
// Limit is unlimited.
type Quota struct {
	Limit int
	Days  int
}

// Validate checks the quota's bounds.
func (q *Quota) Validate() error {
	if q.Limit < 0 || q.Days < 0 {
		return fmt.Errorf("%w: negative quota", ErrInvalid)
	}
	return nil
}

// RequestSettings are the server's request settings: the permissions new
// users get, the quotas of users without their own, and where approved
// requests go.
type RequestSettings struct {
	// DefaultPermissions are given to new users.
	DefaultPermissions RequestPermissions
	// MovieQuota and SeriesQuota apply to users without their own.
	MovieQuota, SeriesQuota Quota
	// MovieLibraryID and MovieQualityProfileID are where approved movie
	// requests are added, with MinimumAvailability; unset, approvals
	// fail.
	MovieLibraryID        ID
	MovieQualityProfileID ID
	MinimumAvailability   MovieAvailability
	// SeriesLibraryID and SeriesQualityProfileID are where approved
	// series requests are added, with SeasonFolder and MonitorNewItems;
	// unset, approvals fail.
	SeriesLibraryID        ID
	SeriesQualityProfileID ID
	SeasonFolder           bool
	MonitorNewItems        NewItemMonitor
	// AnimeLibraryID and AnimeQualityProfileID are where approved anime
	// series are added instead, as Seerr's activeAnimeDirectory and
	// activeAnimeProfileId; unset, those of series apply. Anime series
	// are added as SeriesAnime either way, as Seerr's animeSeriesType.
	AnimeLibraryID        ID
	AnimeQualityProfileID ID
	// Specials allows requesting season 0 (Seerr's
	// enableSpecialEpisodes).
	Specials bool
	// BlocklistedTags are keywords of discovery providers, as
	// "<plugin>/<keyword>", whose movies and series the blocklisted tags
	// job blocklists, as Seerr's blocklistedTags.
	BlocklistedTags []string
	// BlocklistedTagsLimit is how many pages of each order of a keyword's
	// titles the job blocklists, as Seerr's blocklistedTagsLimit; 0 for
	// DefaultBlocklistedTagsLimit.
	BlocklistedTagsLimit int
}

// DefaultBlocklistedTagsLimit is Seerr's default blocklistedTagsLimit.
const DefaultBlocklistedTagsLimit = 50

// MaxBlocklistedTagsLimit is the most pages Seerr's blocklistedTagsLimit
// allows.
const MaxBlocklistedTagsLimit = 250

// TagsLimit returns how many pages of each order of a keyword's titles the
// blocklisted tags job blocklists.
func (s *RequestSettings) TagsLimit() int {
	if s.BlocklistedTagsLimit == 0 {
		return DefaultBlocklistedTagsLimit
	}
	return s.BlocklistedTagsLimit
}

// DefaultRequestSettings are Seerr's defaults: users may request movies
// and series, without quotas.
func DefaultRequestSettings() RequestSettings {
	return RequestSettings{
		DefaultPermissions:  RequestPermissions{Movies: true, Series: true},
		MinimumAvailability: AvailabilityReleased,
		SeasonFolder:        true,
		MonitorNewItems:     NewItemsNone,
	}
}

// Validate checks the settings' invariants.
func (s *RequestSettings) Validate() error {
	switch {
	case s.MovieQuota.Validate() != nil || s.SeriesQuota.Validate() != nil:
		return fmt.Errorf("%w: negative request quota", ErrInvalid)
	case s.MinimumAvailability != "" && !s.MinimumAvailability.Valid():
		return fmt.Errorf("%w: unknown minimum availability %q", ErrInvalid, s.MinimumAvailability)
	case s.MonitorNewItems != "" && !s.MonitorNewItems.Valid():
		return fmt.Errorf("%w: unknown new item monitoring %q", ErrInvalid, s.MonitorNewItems)
	case s.BlocklistedTagsLimit < 0 || s.BlocklistedTagsLimit > MaxBlocklistedTagsLimit:
		return fmt.Errorf("%w: blocklisted tags limit outside 0–%d", ErrInvalid, MaxBlocklistedTagsLimit)
	}
	for _, t := range s.BlocklistedTags {
		if plugin, keyword, ok := strings.Cut(t, "/"); !ok || plugin == "" || keyword == "" {
			return fmt.Errorf("%w: blocklisted tag %q is not <plugin>/<keyword>", ErrInvalid, t)
		}
	}
	return nil
}

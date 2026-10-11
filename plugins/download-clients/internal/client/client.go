package client

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Kind is what a download is of.
type Kind int

// The kinds; zero is unknown.
const (
	Movie Kind = iota + 1
	Series
)

// Kinds are the kinds, in order.
var Kinds = []Kind{Movie, Series}

// Release is a release sent to a client.
type Release struct {
	Title string
	Kind  Kind
	// Recent is set for movies released and episodes aired lately, which
	// take the clients' priority for recent releases.
	Recent bool
	// Data is the torrent file or NZB; empty for a magnet link.
	Data      []byte
	FileName  string
	MagnetURL string
	// InfoHash is the torrent's info hash in upper-case hex, when known.
	InfoHash string
	// SeedRatio and SeedTime are the seeding the indexer asks for; nil for
	// the client's own.
	SeedRatio *float64
	SeedTime  *time.Duration
	// FullSeason is set for releases of a full season.
	FullSeason bool
	// Details are what the release and its movie or series are.
	Details Details
	// BlocklistedHashes are the info hashes, in upper case, of torrents
	// blocklisted for the movie or series, which are refused.
	BlocklistedHashes []string
}

// Details are what a release and its movie or series are, which Radarr's
// and Sonarr's Flood tag downloads with.
type Details struct {
	Quality      string
	Languages    []string
	ReleaseGroup string
	Indexer      string
	Year         int
	Collection   string
	// Studio is a movie's studio or a series' network.
	Studio    string
	TitleSlug string
}

// Status is the state of a download, as Radarr's DownloadItemStatus.
type Status int

// The statuses.
const (
	Queued Status = iota + 1
	Paused
	Downloading
	Completed
	Failed
	Warning
)

// Item is a download in a client, as Radarr's DownloadClientItem.
type Item struct {
	// ID is a torrent's info hash in upper case, or the client's ID.
	ID       string
	Title    string
	Category string
	// Kind is that whose category the download is in; zero when the
	// categories of movies and series are one.
	Kind          Kind
	TotalSize     int64
	RemainingSize int64
	// RemainingTime and SeedRatio are nil when unknown.
	RemainingTime *time.Duration
	SeedRatio     *float64
	// OutputPath is where the files are, as the client sees them.
	OutputPath string
	Message    string
	Status     Status
	Encrypted  bool
	// CanMoveFiles and CanBeRemoved are set once the download is done
	// seeding and the client is to remove completed downloads.
	CanMoveFiles bool
	CanBeRemoved bool
	// HasPostImportCategory is set when the client moves imported
	// downloads to a category of their own.
	HasPostImportCategory bool
}

// Info is what a client tells of itself, as Radarr's DownloadClientInfo.
type Info struct {
	IsLocalhost               bool
	SortingMode               string
	RemovesCompletedDownloads bool
	OutputRootFolders         []string
}

// Options are the settings every configured client has besides those of
// its definition.
type Options struct {
	// RemoveCompletedDownloads removes imported downloads from the client
	// once they are done seeding.
	RemoveCompletedDownloads bool
	RemoveFailedDownloads    bool
	// HistoryLimit is how many downloads of a client's history are read,
	// as Radarr's DownloadClientHistoryLimit; DefaultHistoryLimit when 0.
	HistoryLimit int
}

// DefaultHistoryLimit is Radarr's default DownloadClientHistoryLimit.
const DefaultHistoryLimit = 60

// Limit returns the history limit, DefaultHistoryLimit when unset.
func (o Options) Limit() int {
	if o.HistoryLimit <= 0 {
		return DefaultHistoryLimit
	}
	return o.HistoryLimit
}

// Client is a configured download client.
type Client interface {
	// Add sends a release to the client and returns the download's ID.
	Add(ctx context.Context, r *Release) (string, error)
	// Items lists the downloads in the client's categories of movies and
	// series.
	Items(ctx context.Context, opts Options) ([]Item, error)
	// Remove removes a download, with its files when deleteData is set.
	Remove(ctx context.Context, item Item, deleteData bool) error
	// Info tells of the client.
	Info(ctx context.Context) (Info, error)
	// Test checks that the client answers with its settings.
	Test(ctx context.Context) error
	// Host is the host the client runs on; empty for folders.
	Host() string
}

// Importer is a client that reads a download again before it is
// imported, as Radarr's GetImportItem.
type Importer interface {
	ImportItem(ctx context.Context, item Item, opts Options) (Item, error)
}

// Marker is a client that moves imported downloads to a category, as
// Radarr's MarkItemAsImported.
type Marker interface {
	MarkImported(ctx context.Context, item Item) error
}

// ErrMagnetNotSupported is returned by clients that cannot take a magnet
// link, which is then sent as a torrent file where there is one.
var ErrMagnetNotSupported = errors.New("magnet links are not supported")

// ErrNotSupported is returned for what a client cannot do, such as
// marking downloads imported.
var ErrNotSupported = errors.New("not supported")

// ErrReleaseRejected is a client refusing a release, as Radarr's
// DownloadClientRejectedReleaseException, such as a duplicate.
var ErrReleaseRejected = errors.New("release rejected by the download client")

// Rejected returns an error wrapping ErrReleaseRejected with why.
func Rejected(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrReleaseRejected, fmt.Sprintf(format, args...))
}

// ErrReleaseBlocked is a torrent whose hash is blocklisted, as Radarr's
// ReleaseBlockedException.
var ErrReleaseBlocked = errors.New("release previously added to blocklist")

// TestWarning is what Test returns when the client answers but warns of
// something, as Radarr's validation failures marked IsWarning: the test
// passes with the warning.
type TestWarning struct{ Message string }

func (w *TestWarning) Error() string { return w.Message }

// Warn returns a TestWarning.
func Warn(format string, args ...any) error {
	return &TestWarning{Message: fmt.Sprintf(format, args...)}
}

// AuthError is a client refusing the credentials, as Radarr's
// DownloadClientAuthenticationException.
type AuthError struct{ Err error }

func (e *AuthError) Error() string { return fmt.Sprintf("authentication failed: %v", e.Err) }
func (e *AuthError) Unwrap() error { return e.Err }

// UnavailableError is a client that cannot be reached, as Radarr's
// DownloadClientUnavailableException.
type UnavailableError struct{ Err error }

func (e *UnavailableError) Error() string { return fmt.Sprintf("unable to connect: %v", e.Err) }
func (e *UnavailableError) Unwrap() error { return e.Err }

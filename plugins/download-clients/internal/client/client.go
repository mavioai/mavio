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

package indexer

import (
	"slices"
	"time"
)

// Protocol is how a release is downloaded.
type Protocol int

// The download protocols.
const (
	ProtocolUnknown Protocol = iota
	ProtocolTorrent
	ProtocolUsenet
)

// Privacy is who may use an indexer.
type Privacy int

// The privacy levels.
const (
	PrivacyUnknown Privacy = iota
	Public
	SemiPrivate
	Private
)

// Release is a release an indexer lists, a torrent or an NZB, as
// Prowlarr's ReleaseInfo and TorrentInfo are. Pointers are unset when the
// indexer does not tell.
type Release struct {
	GUID        string
	Title       string
	Description string
	Size        *int64
	DownloadURL string
	InfoURL     string
	CommentsURL string
	Protocol    Protocol
	PublishDate time.Time
	Categories  []int
	Grabs       *int
	Files       *int

	IMDbID, TMDBID, TVDBID, TVRageID, TraktID, TVMazeID, DoubanID int
	Year                                                          int

	Author, BookTitle, Publisher string
	Artist, Album, Label, Track  string

	Genres    []string
	Languages []string
	Subtitles []string
	Flags     []Flag
	PosterURL string

	// Torrents only.
	MagnetURL            string
	InfoHash             string
	Seeders              *int
	Peers                *int
	MinimumRatio         *float64
	MinimumSeedTime      *int64
	DownloadVolumeFactor *float64
	UploadVolumeFactor   *float64
	Scene                bool
}

// HasFlag reports whether the release has the flag.
func (r *Release) HasFlag(f Flag) bool {
	return slices.Contains(r.Flags, f)
}

// Ptr returns a pointer to v, for the optional fields of releases.
func Ptr[T any](v T) *T { return &v }

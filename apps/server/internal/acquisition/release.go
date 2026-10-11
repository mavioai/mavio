package acquisition

import (
	"strconv"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// Release is a release an indexer of an indexer provider plugin found,
// with what grabbing it takes. It is stored as JSON with pending
// releases.
type Release struct {
	PluginID    string `json:"plugin_id"`
	IndexerID   string `json:"indexer_id"`
	IndexerName string `json:"indexer_name,omitempty"`
	// Priority is the indexer's, lower first.
	Priority    int           `json:"priority,omitempty"`
	GUID        string        `json:"guid"`
	Title       string        `json:"title"`
	Size        int64         `json:"size,omitempty"`
	DownloadURL string        `json:"download_url,omitempty"`
	MagnetURL   string        `json:"magnet_url,omitempty"`
	InfoHash    string        `json:"info_hash,omitempty"`
	InfoURL     string        `json:"info_url,omitempty"`
	Protocol    core.Protocol `json:"protocol"`
	PublishDate time.Time     `json:"publish_date"`
	Flags       []string      `json:"flags,omitempty"`
	// Languages are those the indexer gave, by name.
	Languages []string `json:"languages,omitempty"`
	Seeders   *int     `json:"seeders,omitempty"`
	Peers     *int     `json:"peers,omitempty"`
	ImdbID    string   `json:"imdb_id,omitempty"`
	TmdbID    int      `json:"tmdb_id,omitempty"`
	TvdbID    int      `json:"tvdb_id,omitempty"`
	TvRageID  int      `json:"tvrage_id,omitempty"`
	// MinimumRatio and MinimumSeedTime are what the indexer asks torrents
	// to seed; zero for none.
	MinimumRatio    float64       `json:"minimum_ratio,omitempty"`
	MinimumSeedTime time.Duration `json:"minimum_seed_time,omitempty"`
}

// IndexerRef returns the decisions' ID of the release's indexer.
func (r *Release) IndexerRef() string { return r.PluginID + "/" + r.IndexerID }

// FromSearch returns the release of a search result.
func FromSearch(r releases.Release) *Release {
	out := &Release{
		PluginID: r.Provider, IndexerID: r.GetIndexerId(), IndexerName: r.IndexerName, Priority: r.Priority,
		GUID: r.GetGuid(), Title: r.GetTitle(), Size: r.GetSize(),
		DownloadURL: r.GetDownloadUrl(), MagnetURL: r.GetMagnetUrl(), InfoHash: r.GetInfoHash(), InfoURL: r.GetInfoUrl(),
		Protocol: ProtocolFromPlugin(r.GetProtocol()), Flags: r.GetFlags(), Languages: r.GetLanguages(),
		ImdbID: r.GetExternalIds()["imdb"], MinimumRatio: r.GetMinimumRatio(),
		MinimumSeedTime: time.Duration(r.GetMinimumSeedTime()) * time.Second,
	}
	if r.HasPublishDate() {
		out.PublishDate = r.GetPublishDate().AsTime()
	}
	out.TmdbID, out.TvdbID, out.TvRageID = externalID(r.GetExternalIds(), "tmdb"), externalID(r.GetExternalIds(), "tvdb"), externalID(r.GetExternalIds(), "tvrage")
	if r.HasSeeders() {
		out.Seeders = new(int(r.GetSeeders()))
	}
	if r.HasPeers() {
		out.Peers = new(int(r.GetPeers()))
	}
	return out
}

// externalID returns a numeric external ID, 0 when there is none.
func externalID(ids map[string]string, key string) int {
	id, err := strconv.Atoi(ids[key])
	if err != nil || id < 0 {
		return 0
	}
	return id
}

// ProtocolFromPlugin returns the stored protocol of a plugin's; unknown
// protocols are "".
func ProtocolFromPlugin(p pluginv1.DownloadProtocol) core.Protocol {
	switch p {
	case pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT:
		return core.ProtocolTorrent
	case pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET:
		return core.ProtocolUsenet
	}
	return ""
}

// decisionRelease returns the release the decision engine evaluates.
func (r *Release) decisionRelease() *acquire.Release {
	out := &acquire.Release{
		GUID: r.GUID, Title: r.Title, Size: r.Size, PublishDate: r.PublishDate,
		IndexerID: r.IndexerRef(), Indexer: r.IndexerName, IndexerPriority: r.Priority,
		Flags: r.Flags, Seeders: r.Seeders, Peers: r.Peers, InfoHash: r.InfoHash,
		ImdbID: r.ImdbID, TmdbID: r.TmdbID, TvdbID: r.TvdbID, TvRageID: r.TvRageID,
	}
	if p, err := acquire.ProtocolFromCore(r.Protocol); err == nil {
		out.Protocol = p
	}
	for _, name := range r.Languages {
		if l, ok := release.LanguageByName(name); ok {
			out.Languages = append(out.Languages, l)
		}
	}
	return out
}

// batch is releases with the releases the decision engine evaluates.
type batch struct {
	releases []*acquire.Release
	of       map[*acquire.Release]*Release
	// targets are the movies or series of pending releases, which are
	// known.
	targets map[*acquire.Release]core.ID
	// reasons are why pending releases are pending.
	reasons map[*acquire.Release]core.PendingReason
}

func newBatch() *batch {
	return &batch{of: map[*acquire.Release]*Release{}, targets: map[*acquire.Release]core.ID{}, reasons: map[*acquire.Release]core.PendingReason{}}
}

// waitsForClient reports whether a release is pending until a download
// client is back, which leaves out the clients that failed recently even
// when they all did, as Radarr's DownloadService.
func (b *batch) waitsForClient(r *acquire.Release) bool {
	return b.reasons[r] == core.PendingClientUnavailable
}

func (b *batch) add(r *Release) *acquire.Release {
	ar := r.decisionRelease()
	b.releases = append(b.releases, ar)
	b.of[ar] = r
	return ar
}

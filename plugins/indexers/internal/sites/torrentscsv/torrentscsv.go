package torrentscsv

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

var (
	links       = []string{"https://torrents-csv.com/"}
	legacyLinks = []string{"https://torrents-csv.ml/"}
)

// Definition is Torrents.csv.
var Definition = indexer.Definition{
	ID:          "torrentscsv",
	Name:        "TorrentsCSV",
	Description: "Torrents.csv is a self-hostable open source torrent search engine and database",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Public,
	Links:       links,
	LegacyLinks: legacyLinks,
	Fields:      []indexer.Field{indexer.BaseURLField(links)},
	New:         newSite,
}

// Site is Torrents.csv.
type Site struct {
	// baseURL is the site's address without its trailing slash.
	baseURL string
	caps    *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings indexer.BaseSettings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	return &Site{baseURL: strings.TrimRight(settings.URL(links, legacyLinks), "/"), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.Other, "")
	return c
}

// Info implements indexer.Site: the site lists no recent releases.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Public,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: a search of 100 releases, for
// terms of at least three characters; music and books are not searched.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var term string
	switch q.Type {
	case indexer.SearchMusic, indexer.SearchBook:
		return &c, nil
	case indexer.SearchTV:
		term = q.SanitizedTVString()
	default:
		term = q.SanitizedTerm()
	}
	// The search cannot be blank and needs at least 3 characters, counted
	// as .NET counts them.
	if strings.TrimSpace(term) == "" || len(utf16.Encode([]rune(term))) < 3 {
		return &c, nil
	}
	var p indexer.Params
	p.Set("size", "100")
	p.Set("q", term)
	c.AddRequests(indexer.NewRequest(s.baseURL+"/service/search?"+p.Encode(), indexer.AcceptJSON))
	return &c, nil
}

type response struct {
	Torrents []*torrent `json:"torrents"`
}

type torrent struct {
	InfoHash  string `json:"infohash"`
	Name      string `json:"name"`
	Size      int64  `json:"size_bytes"`
	Created   int64  `json:"created_unix"`
	Leechers  *int   `json:"leechers"`
	Seeders   *int   `json:"seeders"`
	Completed *int   `json:"completed"`
}

// ParseResponse implements indexer.Site, as Prowlarr's TorrentsCSVParser
// does: the newest releases first, magnet links built from their hashes
// by whoever downloads them.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	var resp response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("torrentscsv: %w", err)
	}
	var releases []*indexer.Release
	for _, t := range resp.Torrents {
		if t == nil {
			continue
		}
		seeders, leechers, grabs := deref(t.Seeders), deref(t.Leechers), deref(t.Completed)
		releases = append(releases, &indexer.Release{
			GUID: "magnet:?xt=urn:btih:" + t.InfoHash,
			// There is no details page.
			InfoURL:              s.baseURL + "/search?q=" + t.Name,
			Title:                t.Name,
			InfoHash:             t.InfoHash,
			Categories:           []int{indexer.Other.ID},
			PublishDate:          time.Unix(t.Created, 0).UTC(),
			Size:                 indexer.Ptr(t.Size),
			Grabs:                &grabs,
			Seeders:              &seeders,
			Peers:                indexer.Ptr(leechers + seeders),
			DownloadVolumeFactor: indexer.Ptr(0.0),
			UploadVolumeFactor:   indexer.Ptr(1.0),
		})
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int {
		return cmp.Compare(b.PublishDate.UnixNano(), a.PublishDate.UnixNano())
	})
	return releases, nil
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

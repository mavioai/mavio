package nzbindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var links = []string{"https://nzbindex.com/"}

// Definition is NZBIndex.
var Definition = indexer.Definition{
	ID:          "nzbindex",
	Name:        "NZBIndex",
	Description: "A Usenet Indexer",
	Protocol:    indexer.ProtocolUsenet,
	Privacy:     indexer.Public,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{
			Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true,
			HelpText: "Site API Key (optional — searching works without one, but with a lower rate limit)",
		},
	},
	New: newSite,
}

// Settings are NZBIndex's settings.
type Settings struct {
	indexer.BaseSettings
	// APIKey is optional: searches without one are limited more.
	APIKey string `json:"api_key,omitempty"`
}

// Site is NZBIndex.
type Site struct {
	settings Settings
	// baseURL is the site's address without its trailing slash.
	baseURL string
	caps    *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	return &Site{
		settings: settings,
		baseURL:  strings.TrimRight(settings.URL(links, nil), "/"),
		caps:     capabilities(),
	}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:           indexer.ProtocolUsenet,
		Privacy:            indexer.Public,
		SupportsRSS:        true,
		SupportsSearch:     true,
		SupportsPagination: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: one page of the API's search,
// 100 releases unless the query asks for another number.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	term := q.SanitizedTerm()
	if q.Type == indexer.SearchTV {
		term = q.SanitizedTVString()
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	var p indexer.Params
	p.Set("max", strconv.Itoa(limit))
	if strings.TrimSpace(s.settings.APIKey) != "" {
		p.Set("key", strings.TrimSpace(s.settings.APIKey))
	}
	if strings.TrimSpace(term) != "" {
		p.Set("q", term)
	}
	if page := q.Offset / limit; page > 0 {
		p.Set("p", strconv.Itoa(page))
	}
	var c indexer.Chain
	r := indexer.NewRequest(s.baseURL+"/api/search?"+p.Encode(), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// titleRegex finds a post's title between quotes, without the extension
// of the file posted.
var titleRegex = regexp.MustCompile(`"([^:/]*?)(?:\.(rar|nfo|mkv|par2|001|nzb|url|zip|r[0-9]{2}))?"`)

type response struct {
	Error        jsonx.Bool   `json:"error"`
	ErrorMessage jsonx.String `json:"errorMessage"`
	Data         *struct {
		Content json.RawMessage `json:"content"`
	} `json:"data"`
}

type post struct {
	ID        jsonx.String `json:"id"`
	Name      jsonx.String `json:"name"`
	Posted    jsonx.Int64  `json:"posted"`
	Size      jsonx.Int64  `json:"size"`
	FileCount jsonx.Int    `json:"fileCount"`
}

// ParseResponse implements indexer.Site, as Prowlarr's NzbIndexParser
// does: posts whose names have no quoted title are skipped.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("nzbindex: %w", err)
	}
	if resp.Error {
		return nil, indexer.Errorf("NZBIndex API error: %s", resp.ErrorMessage)
	}
	var posts []post
	// Content that is not an array has no releases.
	if resp.Data == nil || json.Unmarshal(resp.Data.Content, &posts) != nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, row := range posts {
		m := titleRegex.FindStringSubmatch(string(row.Name))
		if strings.TrimSpace(string(row.ID)) == "" || m == nil || strings.TrimSpace(m[1]) == "" {
			continue
		}
		details := s.baseURL + "/collection/" + string(row.ID)
		releases = append(releases, &indexer.Release{
			GUID:        details,
			InfoURL:     details,
			DownloadURL: s.baseURL + "/api/download/" + string(row.ID) + ".nzb",
			Title:       m[1],
			Categories:  []int{indexer.Other.ID},
			PublishDate: time.Unix(int64(row.Posted), 0).UTC(),
			Size:        indexer.Ptr(int64(row.Size)),
			Files:       indexer.Ptr(int(row.FileCount)),
		})
	}
	return releases, nil
}

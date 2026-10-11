package torrentpotato

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// Definition is a TorrentPotato API.
var Definition = indexer.Definition{
	ID:          "torrentpotato",
	Name:        "TorrentPotato",
	Description: "A JSON based torrent provider previously developed for CouchPotato",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Fields: []indexer.Field{
		{Name: "base_url", Label: "API URL", Type: indexer.FieldText, HelpText: "URL to TorrentPotato API", Default: "http://127.0.0.1"},
		{Name: "user", Label: "Username", Type: indexer.FieldText, HelpText: "Indexer Username"},
		{Name: "passkey", Label: "Passkey", Type: indexer.FieldText, HelpText: "Indexer Passkey", Secret: true},
	},
	New: newSite,
}

// Settings are a TorrentPotato API's settings.
type Settings struct {
	// BaseURL is the API's URL, requested as it is.
	BaseURL string `json:"base_url"`
	User    string `json:"user,omitempty"`
	Passkey string `json:"passkey,omitempty"`
}

// Validate checks that the user and passkey are set and that the URL is
// an absolute HTTP one.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.User) == "" {
		errs = append(errs, errors.New("user: must not be empty"))
	}
	if strings.TrimSpace(s.Passkey) == "" {
		errs = append(errs, errors.New("passkey: must not be empty"))
	}
	if !validRootURL(s.BaseURL) {
		errs = append(errs, errors.New("base_url: must be valid URL that starts with http(s)://"))
	}
	return errors.Join(errs...)
}

// validRootURL reports whether s is an absolute HTTP URL, as Prowlarr's
// ValidRootUrl checks.
func validRootURL(s string) bool {
	if strings.TrimSpace(s) == "" || strings.TrimSpace(s) != s {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Site is a TorrentPotato API.
type Site struct {
	settings Settings
	caps     *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
	c.Categories.Add("1", indexer.Movies, "")
	return &Site{settings: settings, caps: c}, nil
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: movies, by term and IMDb ID, and
// basic searches; the API takes no other kinds.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var imdb string
	switch q.Type {
	case indexer.SearchMovie:
		imdb = q.FullIMDbID()
	case indexer.SearchBasic:
	default:
		return &c, nil
	}
	u := indexer.AddQueryParam(s.settings.BaseURL, "passkey", s.settings.Passkey)
	u = indexer.AddQueryParam(u, "user", s.settings.User)
	if strings.TrimSpace(imdb) != "" {
		u = indexer.AddQueryParam(u, "imdbid", imdb)
	}
	search := q.Term
	if strings.TrimSpace(search) == "" {
		search = " "
	}
	u = indexer.AddQueryParam(u, "search", search)
	c.AddRequests(indexer.NewRequest(u, indexer.AcceptJSON))
	return &c, nil
}

type response struct {
	Results []result      `json:"results"`
	Error   *jsonx.String `json:"error"`
}

type result struct {
	ReleaseName jsonx.String `json:"release_name"`
	DetailsURL  jsonx.String `json:"details_url"`
	DownloadURL jsonx.String `json:"download_url"`
	IMDbID      jsonx.String `json:"imdb_id"`
	Size        jsonx.Int64  `json:"size"`
	Leechers    jsonx.Int    `json:"leechers"`
	Seeders     jsonx.Int    `json:"seeders"`
	PublishDate jsonx.Time   `json:"publish_date"`
}

// magnetHash finds the info hash of magnet links, which GUIDs are made of.
var magnetHash = regexp.MustCompile(`^magnet:\?xt=urn:btih:([a-f0-9]+)`)

// ParseResponse implements indexer.Site, as Prowlarr's TorrentPotatoParser
// does: the newest releases first, sizes given in megabytes.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Indexer API call returned an unexpected StatusCode [%d]", r.StatusCode)
	}
	var resp *response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("torrentpotato: %w", err)
	}
	if resp == nil {
		return nil, nil
	}
	if resp.Error != nil {
		return nil, indexer.Errorf("Indexer API call returned an error [%s]", *resp.Error)
	}
	var releases []*indexer.Release
	for _, t := range resp.Results {
		dl := string(t.DownloadURL)
		guid := "potato-" + dl
		if m := magnetHash.FindStringSubmatch(dl); m != nil {
			guid = "potato-" + m[1]
		}
		imdb, _ := parse.IMDbID(string(t.IMDbID))
		releases = append(releases, &indexer.Release{
			GUID:        guid,
			Title:       html.UnescapeString(string(t.ReleaseName)),
			Categories:  []int{indexer.Movies.ID},
			Size:        indexer.Ptr(int64(t.Size) * 1000 * 1000),
			DownloadURL: dl,
			InfoURL:     string(t.DetailsURL),
			IMDbID:      imdb,
			PublishDate: t.PublishDate.UTC(),
			Seeders:     indexer.Ptr(int(t.Seeders)),
			Peers:       indexer.Ptr(int(t.Leechers) + int(t.Seeders)),
		})
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int {
		return cmp.Compare(b.PublishDate.UnixNano(), a.PublishDate.UnixNano())
	})
	return releases, nil
}

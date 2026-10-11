package subsplease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var (
	links = []string{
		"https://subsplease.org/",
		"https://subsplease.mrunblock.bond/",
		"https://subsplease.nocensor.click/",
	}
	legacy = []string{"https://subsplease.nocensor.space/"}
)

// Definition is SubsPlease.
var Definition = indexer.Definition{
	ID:          "subsplease",
	Name:        "SubsPlease",
	Description: "SubsPlease - A better HorribleSubs/Erai replacement",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Public,
	Links:       links,
	LegacyLinks: legacy,
	Fields:      []indexer.Field{indexer.BaseURLField(links)},
	New:         newSite,
}

// Site is SubsPlease.
type Site struct {
	baseURL string
	caps    *indexer.Capabilities
}

func newSite(data []byte) (indexer.Site, error) {
	var settings indexer.BaseSettings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	return &Site{baseURL: settings.URL(links, legacy), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.TVAnime, "")
	c.Categories.Add("2", indexer.MoviesOther, "")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Public,
		SupportsRSS:    true,
		SupportsSearch: true,
	}
}

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

var (
	resolution = regexp.MustCompile(`(?i)\d{3,4}p`)
	subsPlease = regexp.MustCompile(`(?i)\[?SubsPlease\]?\s*`)
	sizeParam  = regexp.MustCompile(`(?i)&xl=(\d+)`)
)

// SearchRequests implements indexer.Site, as Prowlarr's
// SubsPleaseRequestGenerator does; music and books are not searched.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	term := q.SanitizedTerm()
	switch q.Type {
	case indexer.SearchMusic, indexer.SearchBook:
		return &c, nil
	case indexer.SearchTV:
		term = strings.TrimSpace(term)
		// Seasons after the first are written S2, not S02.
		if q.Season != nil && *q.Season > 1 {
			term += fmt.Sprintf(" S%d", *q.Season)
		}
		if ep, err := strconv.ParseInt(strings.TrimSpace(q.Episode), 10, 32); err == nil && ep > 0 {
			term += fmt.Sprintf(" %02d", ep)
		}
	}
	var p indexer.Params
	p.Set("tz", "UTC")
	if q.IsRSS() {
		p.Set("f", "latest")
	} else {
		t := strings.TrimSpace(subsPlease.ReplaceAllString(term, ""))
		// The API is not searched by resolution.
		if m := resolution.FindString(t); m != "" {
			t = strings.TrimSpace(strings.ReplaceAll(t, m, ""))
		}
		p.Set("f", "search")
		p.Set("s", t)
	}
	c.AddRequests(indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/api/?"+p.Encode(), indexer.AcceptJSON))
	return &c, nil
}

type release struct {
	ReleaseDate jsonx.Time   `json:"release_date"`
	Show        jsonx.String `json:"show"`
	Episode     jsonx.String `json:"episode"`
	Downloads   []struct {
		Resolution jsonx.String `json:"res"`
		Magnet     jsonx.String `json:"magnet"`
	} `json:"downloads"`
	ImageURL jsonx.String `json:"image_url"`
	Page     jsonx.String `json:"page"`
}

// ParseResponse implements indexer.Site, as Prowlarr's SubsPleaseParser
// does: a release for each resolution of each episode.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	content := r.Content()
	// Without results, the API answers with an empty array or nothing.
	if strings.TrimSpace(content) == "" || content == "[]" {
		return nil, nil
	}
	episodes, err := decodeOrdered(content)
	if err != nil {
		return nil, fmt.Errorf("subsplease: %w", err)
	}
	var releases []*indexer.Release
	for _, e := range episodes {
		for _, d := range e.Downloads {
			rel := &indexer.Release{
				InfoURL:              s.baseURL + "shows/" + string(e.Page) + "/",
				PublishDate:          e.ReleaseDate.Time,
				Files:                indexer.Ptr(1),
				Categories:           []int{indexer.TVAnime.ID},
				Seeders:              indexer.Ptr(1),
				Peers:                indexer.Ptr(2),
				MinimumRatio:         indexer.Ptr(1.0),
				MinimumSeedTime:      indexer.Ptr(int64(172800)), // 48 hours
				DownloadVolumeFactor: indexer.Ptr(0.0),
				UploadVolumeFactor:   indexer.Ptr(1.0),
				Title:                fmt.Sprintf("[SubsPlease] %s - %s (%sp)", e.Show, e.Episode, d.Resolution),
				MagnetURL:            string(d.Magnet),
				GUID:                 string(d.Magnet),
				Size:                 indexer.Ptr(size(string(d.Magnet), string(d.Resolution))),
			}
			if strings.TrimSpace(string(e.ImageURL)) != "" {
				rel.PosterURL = s.baseURL + strings.TrimLeft(string(e.ImageURL), "/")
			}
			if strings.ToLower(string(e.Episode)) == "movie" {
				rel.Categories = append(rel.Categories, indexer.MoviesOther.ID)
			}
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// decodeOrdered decodes the API's object of episodes in the order it
// lists them.
func decodeOrdered(content string) ([]release, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(content)))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("unexpected %v, want an object", tok)
	}
	var out []release
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		var e release
		if err := dec.Decode(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// size returns a release's size from its magnet link, or an estimate by
// its resolution.
func size(magnet, resolution string) int64 {
	if strings.TrimSpace(magnet) != "" {
		if m := sizeParam.FindStringSubmatch(magnet); m != nil {
			if n, err := strconv.ParseInt(m[1], 10, 64); err == nil && n > 0 {
				return n
			}
		}
	}
	switch resolution {
	case "1080":
		return int64(math.RoundToEven(1.3 * (1 << 30)))
	case "720":
		return 700 << 20
	case "480":
		return 350 << 20
	}
	return 1 << 30
}

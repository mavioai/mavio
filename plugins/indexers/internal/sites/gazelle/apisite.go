package gazelle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// apiSite is how a music site searched with an API key differs from the
// others, as Prowlarr's Redacted and Orpheus indexers differ.
type apiSite struct {
	links, legacy []string
	// authorization returns the Authorization header of an API key.
	authorization func(key string) string
	// freeload is whether the site has freeload torrents, which are
	// neither downloaded nor uploaded and can be searched alone.
	freeload bool
	// fuzzyGroupTime reads the times of groups listed as torrents that
	// are not Unix times as fuzzy dates, instead of as the epoch.
	fuzzyGroupTime bool
	// followRedirects is whether searches follow redirects.
	followRedirects bool
}

// APISettings are the settings of a site searched with an API key.
type APISettings struct {
	indexer.BaseSettings
	APIKey            string         `json:"api_key,omitempty"`
	UseFreeleechToken FreeleechToken `json:"use_freeleech_token,omitempty"`
	// FreeloadOnly searches only freeload torrents, on sites having them.
	FreeloadOnly bool `json:"freeload_only,omitempty"`
}

// Validate checks the settings.
func (s APISettings) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api_key: must not be empty")
	}
	return nil
}

// APISite is a music site searched through the Gazelle API with an API
// key, as Prowlarr's Redacted and Orpheus are.
type APISite struct {
	apiSite
	settings APISettings
	baseURL  string
	caps     *indexer.Capabilities
}

// apiDefinition returns the definition of a site searched with an API
// key.
func apiDefinition(d indexer.Definition, s apiSite) indexer.Definition {
	d.Protocol = indexer.ProtocolTorrent
	d.Privacy = indexer.Private
	d.Links, d.LegacyLinks = s.links, s.legacy
	d.Fields = []indexer.Field{indexer.BaseURLField(s.links), indexer.APIKeyField, freeleechTokenField}
	if s.freeload {
		d.Fields = append(d.Fields, indexer.Field{
			Name: "freeload_only", Label: "Freeload Only", Type: indexer.FieldCheckbox, Advanced: true,
			HelpText: "Search freeload torrents only. End date: 31 January 2024, 23:59 UTC.",
		})
	}
	d.New = func(data []byte) (indexer.Site, error) {
		var settings APISettings
		if err := indexer.DecodeSettings(data, &settings); err != nil {
			return nil, err
		}
		if err := settings.Validate(); err != nil {
			return nil, err
		}
		if !s.freeload {
			settings.FreeloadOnly = false
		}
		return &APISite{apiSite: s, settings: settings, baseURL: settings.URL(s.links, s.legacy), caps: musicCaps()}, nil
	}
	return d
}

// musicCaps returns the capabilities of the music sites searched with an
// API key.
func musicCaps() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.LimitsDefault, c.LimitsMax = 50, 50
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ, indexer.ParamArtist, indexer.ParamAlbum, indexer.ParamYear}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("1", indexer.Audio, "Music")
	c.Categories.Add("2", indexer.PC, "Applications")
	c.Categories.Add("3", indexer.BooksEBook, "E-Books")
	c.Categories.Add("4", indexer.AudioAudiobook, "Audiobooks")
	c.Categories.Add("5", indexer.Other, "E-Learning Videos")
	c.Categories.Add("6", indexer.Other, "Comedy")
	c.Categories.Add("7", indexer.BooksComics, "Comics")
	return c
}

// Info implements indexer.Site.
func (s *APISite) Info() indexer.Info {
	i := baseInfo
	i.RateLimit = 3 * time.Second
	return i
}

// Capabilities implements indexer.Site.
func (s *APISite) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

// SearchRequests implements indexer.Site: one browse request of the API;
// none for movies and TV.
func (s *APISite) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	var c indexer.Chain
	var p indexer.Params
	switch q.Type {
	case indexer.SearchMovie, indexer.SearchTV:
		return &c, nil
	case indexer.SearchMusic:
		if strings.TrimSpace(q.Artist) != "" && q.Artist != "VA" {
			p.Set("artistname", q.Artist)
		}
		if strings.TrimSpace(q.Album) != "" {
			p.Set("groupname", q.Album)
		}
		if q.Year != 0 {
			p.Set("year", strconv.Itoa(q.Year))
		}
	}
	p.Set("action", "browse")
	p.Set("order_by", "time")
	p.Set("order_way", "desc")
	if term := strings.TrimSpace(q.SanitizedTerm()); term != "" {
		p.Set("searchstr", term)
	}
	for _, cat := range s.caps.Categories.MapToTracker(q.Categories, false) {
		p.Set("filter_cat["+cat+"]", "1")
	}
	if s.settings.FreeloadOnly {
		p.Set("freetorrent", "4")
	}
	r := indexer.NewRequest(s.baseURL+"ajax.php?"+p.Encode(), indexer.AcceptJSON)
	r.Header.Set("Authorization", s.authorization(s.settings.APIKey))
	r.FollowRedirects = s.followRedirects
	c.AddRequests(r)
	return &c, nil
}

// ParseResponse implements indexer.Site.
func (s *APISite) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, statusError(r)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, contentTypeError(r)
	}
	var resp response[group]
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("gazelle: %w", err)
	}
	if resp.Status != "success" || resp.Response == nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, g := range resp.Response.Results {
		if g.Torrents == nil {
			rel, err := s.groupRelease(&g)
			if err != nil {
				return nil, err
			}
			if rel != nil {
				releases = append(releases, rel)
			}
			continue
		}
		for _, t := range g.Torrents {
			if !s.wanted(&t.torrentStats) {
				continue
			}
			rel, err := s.release(&g, &t.torrentStats, html.UnescapeString(torrentTitle(&g, &t)), t.Time.UTC(), t.Category)
			if err != nil {
				return nil, err
			}
			year, err := strconv.Atoi(string(g.GroupYear))
			if err != nil {
				return nil, fmt.Errorf("gazelle: year %q: %w", g.GroupYear, err)
			}
			rel.Artist = html.UnescapeString(string(g.Artist))
			rel.Album = html.UnescapeString(string(g.GroupName))
			rel.Year = year
			rel.Scene = bool(t.Scene)
			releases = append(releases, rel)
		}
	}
	SortByDate(releases)
	return releases, nil
}

// wanted reports whether a torrent is kept: with a token required, one
// that can be used; with freeload only, a freeload one.
func (s *APISite) wanted(t *torrentStats) bool {
	if s.settings.UseFreeleechToken == TokenRequired && !bool(t.CanUseToken) {
		return false
	}
	return !s.settings.FreeloadOnly || bool(t.IsFreeload)
}

// groupRelease returns the release of a group listed as a torrent; nil
// when it is not wanted.
func (s *APISite) groupRelease(g *group) (*indexer.Release, error) {
	if !s.wanted(&g.torrentStats) {
		return nil, nil
	}
	n, err := strconv.ParseInt(string(g.GroupTime), 10, 64)
	date := time.Unix(n, 0).UTC()
	if err != nil {
		date = time.Unix(0, 0).UTC()
		if s.fuzzyGroupTime {
			d, err := parse.Fuzzy(time.Now(), string(g.GroupTime), "")
			if err != nil {
				return nil, err
			}
			date = d.UTC()
		}
	}
	return s.release(g, &g.torrentStats, html.UnescapeString(string(g.GroupName)), date, g.Category)
}

// release returns the release of a torrent.
func (s *APISite) release(g *group, t *torrentStats, title string, date time.Time, category *jsonx.String) (*indexer.Release, error) {
	size, err := strconv.ParseInt(string(t.Size), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gazelle: size %q: %w", t.Size, err)
	}
	seeders, err := strconv.Atoi(string(t.Seeders))
	if err != nil {
		return nil, fmt.Errorf("gazelle: seeders %q: %w", t.Seeders, err)
	}
	leechers, err := strconv.Atoi(string(t.Leechers))
	if err != nil {
		return nil, fmt.Errorf("gazelle: leechers %q: %w", t.Leechers, err)
	}
	freeload := s.freeload && bool(t.IsFreeload)
	freeleech := bool(t.IsFreeLeech || t.IsNeutralLeech || t.IsPersonalFreeLeech) || freeload
	info := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "id", string(g.GroupID)), "torrentid", strconv.Itoa(int(t.TorrentID)))
	rel := &indexer.Release{
		GUID:                 info,
		InfoURL:              info,
		DownloadURL:          s.downloadURL(int(t.TorrentID), bool(t.CanUseToken) && !freeleech),
		Title:                title,
		Protocol:             indexer.ProtocolTorrent,
		Size:                 &size,
		Files:                indexer.Ptr(int(t.FileCount)),
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + leechers),
		PublishDate:          date,
		Categories:           categories(s.caps, category),
		DownloadVolumeFactor: indexer.Ptr(volumeFactor(freeleech)),
		UploadVolumeFactor:   indexer.Ptr(volumeFactor(bool(t.IsNeutralLeech) || freeload)),
	}
	if t.Snatches != nil {
		rel.Grabs = indexer.Ptr(int(*t.Snatches))
	}
	return rel, nil
}

// torrentTitle returns the title of a group's torrent: the artist, album,
// year, kind of release and edition, then its format, encoding, media,
// log and cue.
func torrentTitle(g *group, t *torrent) string {
	title := fmt.Sprintf("%s - %s (%s)", g.Artist, g.GroupName, g.GroupYear)
	if strings.TrimSpace(string(g.ReleaseType)) != "" && g.ReleaseType != "Unknown" {
		title += " [" + string(g.ReleaseType) + "]"
	}
	if strings.TrimSpace(string(t.RemasterTitle)) != "" {
		title += " [" + strings.TrimSpace(string(t.RemasterTitle)+" "+string(t.RemasterYear)) + "]"
	}
	flags := []string{string(t.Format) + " " + string(t.Encoding), string(t.Media)}
	if t.HasLog {
		flags = append(flags, fmt.Sprintf("Log (%d%%)", t.LogScore))
	}
	if t.HasCue {
		flags = append(flags, "Cue")
	}
	return title + " [" + strings.Join(flags, " / ") + "]"
}

// downloadURL returns the link downloading a torrent through the API,
// using a token only when the settings say so and one can be used.
func (s *APISite) downloadURL(id int, canUseToken bool) string {
	u := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/ajax.php"), "action", "download"), "id", strconv.Itoa(id))
	if s.settings.UseFreeleechToken != TokenNever && canUseToken {
		u = indexer.AddQueryParam(u, "usetoken", "1")
	}
	return u
}

// DownloadRequest implements indexer.DownloadSite: downloads carry the
// API key.
func (s *APISite) DownloadRequest(_ context.Context, _ *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, "")
	r.Header.Set("Authorization", s.authorization(s.settings.APIKey))
	return r, nil
}

// RetryDownload implements indexer.DownloadRetrier, as Site does.
func (s *APISite) RetryDownload(link string, data []byte) (string, bool) {
	return retryDownload(s.settings.UseFreeleechToken, link, data)
}

// CleanupReleases implements indexer.ReleaseCleaner: recent releases are
// limited to the 50 newest.
func (s *APISite) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	if q.IsRSS() && len(releases) > 50 {
		return releases[:50]
	}
	return releases
}

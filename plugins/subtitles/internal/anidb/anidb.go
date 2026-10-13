// Package anidb finds the AniDB and AniList IDs of anime the anime
// providers search by, as Bazarr's anidb and anilist refiners: the
// anime-lists mapping of TVDB series and seasons to AniDB anime, the
// AniDB HTTP API for episode IDs, under its daily request limit, and
// Fribb's anime-lists for AniList IDs. The lists are kept in the
// provider's data folder for a day.
package anidb

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// The sources, as Bazarr's refiners.
const (
	MappingsURL = "https://raw.githubusercontent.com/Anime-Lists/anime-lists/master/anime-list.xml"
	AniListURL  = "https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-mini.json"
	APIURL      = "http://api.anidb.net:9001/httpapi"
)

const (
	// listTTL is how long the lists are kept, as Bazarr's region cache
	// of a day.
	listTTL = 24 * time.Hour
	// episodesTTL is how long an anime's episodes are kept, as Bazarr's
	// REFINER_EXPIRATION_TIME.
	episodesTTL = 7 * 24 * time.Hour
	// quotaTTL is how long the API's request count and throttling last.
	quotaTTL = 24 * time.Hour
	// dailyLimit is Bazarr's soft limit of API requests a day.
	dailyLimit = 200
)

// Settings are the AniDB API client's settings, as Bazarr's anidb
// integration, for the providers that need episode IDs.
func Settings() []provider.Setting {
	return []provider.Setting{
		{
			Name: "anidb_api_client", Label: "AniDB API Client", Type: provider.Text,
			Help: "Register an API client on AniDB (https://anidb.net/software/add) to obtain the API Client name and version",
		},
		{Name: "anidb_api_client_ver", Label: "AniDB API Client Version", Type: provider.Number, Default: 1},
	}
}

// Config is the AniDB API client a provider is set up with.
type Config struct {
	APIClient    string `json:"anidb_api_client"`
	APIClientVer *int   `json:"anidb_api_client_ver"`
}

// Version returns the client's version, 1 when unset.
func (c Config) Version() int {
	if c.APIClientVer == nil {
		return 1
	}
	return *c.APIClientVer
}

// Validate checks the settings as Bazarr's validators: a version from 1
// to 9.
func (c Config) Validate() error {
	if v := c.Version(); v < 1 || v > 9 {
		return provider.Errorf(provider.ConfigurationError, "AniDB API client version %d is not from 1 to 9", v)
	}
	return nil
}

// Client finds AniDB and AniList IDs.
type Client struct {
	env  provider.Env
	cfg  Config
	sess *site.Session

	mappingsURL, aniListURL, apiURL string

	mu         sync.Mutex
	mappings   map[string][]anime
	mappingsAt time.Time
	aniList    []aniListEntry
	aniListAt  time.Time
	episodes   map[int]cachedEpisodes
	quota      *quota
}

type cachedEpisodes struct {
	data []byte
	at   time.Time
}

// New returns a client keeping its lists in env.DataDir.
func New(env provider.Env, cfg Config) *Client {
	return &Client{
		env: env, cfg: cfg, sess: site.New(env.HTTP),
		mappingsURL: MappingsURL, aniListURL: AniListURL, apiURL: APIURL,
		episodes: map[int]cachedEpisodes{},
	}
}

// SetURLs points the client at other sources, for tests.
func (c *Client) SetURLs(mappings, aniList, api string) {
	c.mappingsURL, c.aniListURL, c.apiURL = mappings, aniList, api
}

// IDs are an episode's AniDB IDs; zero when unknown.
type IDs struct {
	// Series is the anime's ID (aid).
	Series int
	// Episode is the episode's ID (eid).
	Episode int
	// EpisodeNumber is the episode's number in the anime.
	EpisodeNumber int
	// Offset is how many TVDB episodes of the season come before the
	// anime's first.
	Offset int
}

// Refine returns the AniDB IDs of an episode, as Bazarr's
// refine_anidb_ids: those the video carries, else the anime and episode
// number its TVDB series, season and episode map to, and, when
// episodeID is set, the episode's ID from the AniDB API.
func (c *Client) Refine(ctx context.Context, v *provider.Video, episodeID bool) (IDs, error) {
	var ids IDs
	if v == nil || !v.Episode {
		return ids, nil
	}
	ids.Series = number(v.SeriesExternalIDs["anidb"])
	ids.Episode = number(v.ExternalIDs["anidb"])
	ids.EpisodeNumber = v.Number
	if tvdb := v.SeriesExternalIDs["tvdb"]; tvdb != "" {
		series, no, offset, err := c.showInformation(ctx, tvdb, v.Season, v.Number)
		if err != nil {
			return IDs{}, err
		}
		switch {
		case series != 0 && (ids.Series == 0 || ids.Series == series):
			ids.Series, ids.EpisodeNumber, ids.Offset = series, no, offset
			c.env.Log.DebugContext(ctx, "AniDB refinement identified the series", "series", v.Series, "anidb", series)
		case ids.Series == 0:
			return IDs{}, nil
		}
	}
	if ids.Series == 0 || ids.Episode != 0 || !episodeID {
		return ids, nil
	}
	if c.cfg.APIClient == "" {
		c.env.Log.WarnContext(ctx, "AniDB API credentials are not fully set up; episode IDs cannot be found", "series", v.Series)
		return ids, nil
	}
	switch throttled, err := c.Throttled(); {
	case err != nil:
		return ids, err
	case throttled:
		c.env.Log.WarnContext(ctx, "AniDB API daily limit reached; skipping episode ID refinement", "series", v.Series)
		return ids, nil
	}
	eid, err := c.episodeID(ctx, ids.Series, ids.EpisodeNumber)
	switch {
	case provider.KindOf(err) == provider.TooManyRequests:
		c.env.Log.ErrorContext(ctx, "AniDB API daily limit reached", "series", v.Series, "error", err)
		if err := c.markThrottled(); err != nil {
			return ids, err
		}
		return ids, nil
	case err != nil:
		return ids, err
	}
	ids.Episode = eid
	return ids, nil
}

// number reads an ID; 0 when it is none.
func number(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// anime is an entry of the anime-lists mapping.
type anime struct {
	AniDBID       string    `xml:"anidbid,attr"`
	TVDBID        string    `xml:"tvdbid,attr"`
	DefaultSeason string    `xml:"defaulttvdbseason,attr"`
	EpisodeOffset string    `xml:"episodeoffset,attr"`
	Mappings      []mapping `xml:"mapping-list>mapping"`
}

type mapping struct {
	TVDBSeason string `xml:"tvdbseason,attr"`
	Offset     string `xml:"offset,attr"`
	Text       string `xml:",chardata"`
}

// showInformation maps a TVDB episode to its AniDB anime, episode
// number and the season's offset, as Bazarr's get_show_information; 0
// for none.
func (c *Client) showInformation(ctx context.Context, tvdb string, season, episode int) (series, no, offset int, err error) {
	mappings, err := c.loadMappings(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	entries := mappings[tvdb]
	s := strconv.Itoa(season)
	var animes []anime
	for _, a := range entries {
		if a.DefaultSeason == s {
			animes = append(animes, a)
		}
	}
	if len(animes) == 0 {
		// Some entries keep the TVDB seasons in their mappings, their
		// default season being "a".
		found := false
		for _, a := range entries {
			if a.DefaultSeason != "a" {
				continue
			}
			found = true
			series, offset = number(a.AniDBID), 0
			for _, m := range a.Mappings {
				if m.TVDBSeason == s {
					offset = number(m.Offset)
					break
				}
			}
		}
		if !found {
			return 0, 0, 0, nil
		}
		return series, episode - offset, offset, nil
	}
	slices.SortStableFunc(animes, func(a, b anime) int { return number(a.EpisodeOffset) - number(b.EpisodeOffset) })
	for _, a := range animes {
		// AniDB episodes mapped to TVDB ones, as ";1-1;2-3+4;".
		for _, m := range a.Mappings {
			if m.TVDBSeason != s {
				continue
			}
			for ref := range strings.SplitSeq(strings.TrimSpace(m.Text), ";") {
				anidbNo, tvdbNos, ok := strings.Cut(ref, "-")
				if !ok {
					continue
				}
				for n := range strings.SplitSeq(tvdbNos, "+") {
					if t, err := strconv.Atoi(n); err == nil && t == episode {
						return number(a.AniDBID), number(anidbNo), 0, nil
					}
				}
			}
		}
		// AniDB splits TVDB seasons in parts, each its own anime.
		if o := number(a.EpisodeOffset); episode > o {
			series, offset = number(a.AniDBID), o
		}
	}
	return series, episode - offset, offset, nil
}

// loadMappings returns the anime-lists mapping by TVDB ID.
func (c *Client) loadMappings(ctx context.Context) (map[string][]anime, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mappings != nil && c.env.Now().Sub(c.mappingsAt) < listTTL {
		return c.mappings, nil
	}
	data, at, err := c.cached(ctx, "anime-list.xml", listTTL, c.mappingsURL)
	if err != nil {
		return nil, err
	}
	var list struct {
		Anime []anime `xml:"anime"`
	}
	if err := xml.Unmarshal(data, &list); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "anime-lists mapping: %w", err)
	}
	byTVDB := map[string][]anime{}
	for _, a := range list.Anime {
		byTVDB[a.TVDBID] = append(byTVDB[a.TVDBID], a)
	}
	c.mappings, c.mappingsAt = byTVDB, at
	return byTVDB, nil
}

// aniListEntry is an entry of Fribb's anime-lists.
type aniListEntry struct {
	AniListID json.RawMessage `json:"anilist_id"`
	AniDBID   json.RawMessage `json:"anidb_id"`
	IMDbID    json.RawMessage `json:"imdb_id"`
}

// AniListID returns the AniList ID of a video, as Bazarr's
// refine_anilist_ids: the one it carries, else the one Fribb's
// anime-lists give its AniDB anime, for an episode, or its IMDb ID, for
// a movie; 0 for none.
func (c *Client) AniListID(ctx context.Context, v *provider.Video, anidbSeries int) (int, error) {
	if id := number(v.ExternalIDs["anilist"]); id != 0 && !v.Episode {
		return id, nil
	}
	if id := number(v.SeriesExternalIDs["anilist"]); id != 0 && v.Episode {
		return id, nil
	}
	key, value := "imdb_id", v.ExternalIDs["imdb"]
	if v.Episode {
		key, value = "anidb_id", ""
		if anidbSeries != 0 {
			value = strconv.Itoa(anidbSeries)
		}
	}
	if value == "" {
		c.env.Log.DebugContext(ctx, "no ID to find the AniList ID by", "id", key)
		return 0, nil
	}
	list, err := c.loadAniList(ctx)
	if err != nil {
		return 0, err
	}
	for _, e := range list {
		raw := e.IMDbID
		if key == "anidb_id" {
			raw = e.AniDBID
		}
		if strings.Trim(string(raw), `"`) != value {
			continue
		}
		id := number(strings.Trim(string(e.AniListID), `"`))
		if id == 0 {
			c.env.Log.ErrorContext(ctx, "the anime-lists entry has no AniList ID", key, value)
		}
		return id, nil
	}
	c.env.Log.DebugContext(ctx, "no AniList ID found", key, value)
	return 0, nil
}

func (c *Client) loadAniList(ctx context.Context) ([]aniListEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.aniList != nil && c.env.Now().Sub(c.aniListAt) < listTTL {
		return c.aniList, nil
	}
	data, at, err := c.cached(ctx, "anime-list-mini.json", listTTL, c.aniListURL)
	if err != nil {
		return nil, err
	}
	var list []aniListEntry
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "anime-lists: %w", err)
	}
	c.aniList, c.aniListAt = list, at
	return list, nil
}

// cached returns a list kept in the data folder for ttl, fetching it
// when it is older; a list that cannot be fetched again is used stale.
// It returns when the list was fetched.
func (c *Client) cached(ctx context.Context, name string, ttl time.Duration, u string) ([]byte, time.Time, error) {
	now := c.env.Now()
	path := ""
	var stale []byte
	var staleAt time.Time
	if c.env.DataDir != "" {
		path = filepath.Join(c.env.DataDir, name)
		if info, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("%s: %w", name, err)
			}
			if now.Sub(info.ModTime()) < ttl {
				return data, info.ModTime(), nil
			}
			stale, staleAt = data, info.ModTime()
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, time.Time{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	r, err := c.sess.Get(ctx, u, nil)
	if err != nil {
		if stale != nil && ctx.Err() == nil {
			c.env.Log.WarnContext(ctx, "using a stale list", "list", name, "error", err)
			return stale, staleAt, nil
		}
		return nil, time.Time{}, err
	}
	if path != "" {
		if err := writeFile(path, r.Body, now); err != nil {
			return nil, time.Time{}, err
		}
	}
	return r.Body, now, nil
}

// writeFile replaces a file of the data folder, dated at.
func writeFile(path string, data []byte, at time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("data folder: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("data folder: %w", err)
	}
	if err := os.Chtimes(tmp, at, at); err != nil {
		return fmt.Errorf("data folder: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("data folder: %w", err)
	}
	return nil
}

// episodesResponse is the AniDB API's answer about an anime.
type episodesResponse struct {
	XMLName  xml.Name
	Code     string `xml:"code,attr"`
	Episodes *struct {
		Episode []struct {
			ID   string `xml:"id,attr"`
			EpNo string `xml:"epno"`
		} `xml:"episode"`
	} `xml:"episodes"`
}

// episodeID returns the ID of an anime's episode, as Bazarr's
// get_episode_ids; 0 for none.
func (c *Client) episodeID(ctx context.Context, series, no int) (int, error) {
	data, err := c.episodesOf(ctx, series)
	if err != nil {
		return 0, err
	}
	var res episodesResponse
	if err := xml.Unmarshal(data, &res); err != nil {
		return 0, provider.Errorf(provider.ParseResponseError, "AniDB anime %d: %w", series, err)
	}
	if res.Episodes == nil {
		return 0, nil
	}
	want := strconv.Itoa(no)
	for _, e := range res.Episodes.Episode {
		if strings.TrimSpace(e.EpNo) == want {
			return number(e.ID), nil
		}
	}
	return 0, nil
}

// episodesOf returns the API's answer about an anime, kept a week, as
// Bazarr's get_episodes: within the daily limit, a ban being too many
// requests and an unknown client a configuration error.
func (c *Client) episodesOf(ctx context.Context, series int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.env.Now()
	if e, ok := c.episodes[series]; ok && now.Sub(e.at) < episodesTTL {
		return e.data, nil
	}
	name := filepath.Join("anidb-episodes", strconv.Itoa(series)+".xml")
	if c.env.DataDir != "" {
		path := filepath.Join(c.env.DataDir, name)
		if info, err := os.Stat(path); err == nil && now.Sub(info.ModTime()) < episodesTTL {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			c.episodes[series] = cachedEpisodes{data, info.ModTime()}
			return data, nil
		}
	}
	q, err := c.loadQuota()
	if err != nil {
		return nil, err
	}
	if q.Count >= dailyLimit {
		return nil, provider.Errorf(provider.TooManyRequests, "AniDB daily API request limit exceeded")
	}
	r, err := c.sess.Get(ctx, c.apiURL, url.Values{
		"request": {"anime"}, "client": {c.cfg.APIClient}, "clientver": {strconv.Itoa(c.cfg.Version())},
		"protover": {"1"}, "aid": {strconv.Itoa(series)},
	})
	if err != nil {
		return nil, err
	}
	var res episodesResponse
	if err := xml.NewDecoder(bytes.NewReader(r.Body)).Decode(&res); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "AniDB anime %d: %w", series, err)
	}
	switch res.Code {
	case "500":
		return nil, provider.Errorf(provider.TooManyRequests, "AniDB API abuse detected: banned")
	case "302":
		return nil, provider.Errorf(provider.ConfigurationError, "AniDB API client %q is disabled or does not exist", c.cfg.APIClient)
	}
	if err := c.saveQuota(quota{Count: q.Count + 1, Set: now}); err != nil {
		return nil, err
	}
	if res.Episodes == nil {
		return nil, provider.Errorf(provider.ProviderError, "AniDB anime %d: no episodes in the answer", series)
	}
	c.episodes[series] = cachedEpisodes{r.Body, now}
	if c.env.DataDir != "" {
		if err := writeFile(filepath.Join(c.env.DataDir, name), r.Body, now); err != nil {
			return nil, err
		}
	}
	return r.Body, nil
}

// quota is the API's daily request count, or that it is throttled, as
// Bazarr's anidb_refiner cache entry: it lasts a day from when it was
// set.
type quota struct {
	Count     int       `json:"daily_api_request_count,omitempty"`
	Throttled bool      `json:"is_throttled,omitempty"`
	Set       time.Time `json:"set"`
}

const quotaFile = "anidb-api.json"

// loadQuota returns the quota, from the data folder when it has one;
// the caller holds c.mu.
func (c *Client) loadQuota() (quota, error) {
	if c.quota == nil {
		c.quota = &quota{}
		if c.env.DataDir != "" {
			data, err := os.ReadFile(filepath.Join(c.env.DataDir, quotaFile))
			switch {
			case err == nil:
				if err := json.Unmarshal(data, c.quota); err != nil {
					c.quota = &quota{}
				}
			case !errors.Is(err, fs.ErrNotExist):
				return quota{}, fmt.Errorf("%s: %w", quotaFile, err)
			}
		}
	}
	if c.env.Now().Sub(c.quota.Set) >= quotaTTL {
		*c.quota = quota{}
	}
	return *c.quota, nil
}

// saveQuota sets the quota; the caller holds c.mu.
func (c *Client) saveQuota(q quota) error {
	c.quota = &q
	if c.env.DataDir == "" {
		return nil
	}
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(c.env.DataDir, quotaFile), data, c.env.Now())
}

// markThrottled stops API requests for a day, as Bazarr's
// mark_as_throttled.
func (c *Client) markThrottled() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveQuota(quota{Throttled: true, Set: c.env.Now()})
}

// Throttled reports whether API requests are stopped for the day.
func (c *Client) Throttled() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q, err := c.loadQuota()
	return q.Throttled, err
}

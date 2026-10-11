package gazellegames

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
)

var links = []string{"https://gazellegames.net/"}

// Definition is GazelleGames.
var Definition = indexer.Definition{
	ID:          "gazellegames",
	Name:        "GazelleGames",
	Description: "GazelleGames (GGn) is a Private Torrent Tracker for GAMES",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{
			Name: "api_key", Label: "API Key", Type: indexer.FieldText, Secret: true,
			HelpText: "API Key from the Site (Found in Settings => Access Settings), Must have User and Torrents permissions",
		},
		{
			Name: "search_group_names", Label: "Search Group Names", Type: indexer.FieldCheckbox,
			HelpText: "Search Group Names Only",
		},
		{
			Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, Advanced: true,
			HelpText: "Search freeleech releases only",
		},
	},
	New: newSite,
}

// Settings are GazelleGames' settings.
type Settings struct {
	indexer.BaseSettings
	APIKey string `json:"api_key,omitempty"`
	// SearchGroupNames searches the names of groups instead of all the
	// text of their torrents.
	SearchGroupNames bool `json:"search_group_names,omitempty"`
	FreeleechOnly    bool `json:"freeleech_only,omitempty"`
}

// Validate checks that the API key is set.
func (s Settings) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api_key: must not be empty")
	}
	return nil
}

// Site is GazelleGames.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities

	mu sync.Mutex
	// passkey is the user's passkey, which download links carry; it is
	// asked for on testing the site or on downloading without it.
	passkey string
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
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

// apiURL returns the address of the API, with its query.
func (s *Site) apiURL(query string) string {
	return strings.TrimRight(strings.TrimSpace(s.baseURL), "/") + "/api.php?" + query
}

func (s *Site) apiRequest(query string) *indexer.Request {
	r := indexer.NewRequest(s.apiURL(query), indexer.AcceptJSON)
	r.Header.Set("X-API-Key", s.settings.APIKey)
	return r
}

// SearchRequests implements indexer.Site: one search of the torrents of
// the platforms the categories map to, whatever the kind of search.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	parts := []string{"request=search", "search_type=torrents", "empty_groups=filled", "order_by=time", "order_way=desc"}
	if term := q.SanitizedTerm(); strings.TrimSpace(term) != "" {
		key := "searchstr"
		if s.settings.SearchGroupNames {
			key = "groupname"
		}
		parts = append(parts, key+"="+indexer.URLEncode(strings.ReplaceAll(term, ".", " ")))
	}
	var platforms []string
	for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
		if !slices.Contains(platforms, c) && strings.Trim(c, "0123456789") != "" {
			platforms = append(platforms, c)
		}
	}
	for _, p := range platforms {
		parts = append(parts, "artistcheck[]="+indexer.URLEncode(p))
	}
	if s.settings.FreeleechOnly {
		parts = append(parts, "freetorrent=1")
	}
	var c indexer.Chain
	c.AddRequests(s.apiRequest(strings.Join(parts, "&")))
	return &c, nil
}

type response struct {
	Status   jsonx.String    `json:"status"`
	Response json.RawMessage `json:"response"`
}

type group struct {
	Artists []struct {
		Name string `json:"name"`
	} `json:"Artists"`
	Torrents json.RawMessage `json:"Torrents"`
	Year     *jsonx.Int      `json:"Year"`
}

type torrent struct {
	CategoryID    jsonx.Int    `json:"CategoryID"`
	Format        jsonx.String `json:"Format"`
	Encoding      jsonx.String `json:"Encoding"`
	Language      jsonx.String `json:"Language"`
	Region        jsonx.String `json:"Region"`
	RemasterYear  jsonx.String `json:"RemasterYear"`
	RemasterTitle jsonx.String `json:"RemasterTitle"`
	ReleaseTitle  jsonx.String `json:"ReleaseTitle"`
	Miscellaneous jsonx.String `json:"Miscellaneous"`
	Scene         jsonx.Int    `json:"Scene"`
	Dupable       jsonx.Int    `json:"Dupable"`
	Time          jsonx.Time   `json:"Time"`
	TorrentType   jsonx.String `json:"TorrentType"`
	FileCount     jsonx.Int    `json:"FileCount"`
	Size          jsonx.String `json:"Size"`
	Snatched      *jsonx.Int   `json:"Snatched"`
	Seeders       jsonx.Int    `json:"Seeders"`
	Leechers      jsonx.Int    `json:"Leechers"`
	FreeTorrent   jsonx.String `json:"FreeTorrent"`
	LowSeedFL     jsonx.Bool   `json:"LowSeedFL"`
	GameDOXType   jsonx.String `json:"GameDOXType"`
}

// The kinds of freeleech of torrents.
const (
	freeLeech = 1
	neutral   = 2
)

// freeTorrent reads a torrent's kind of freeleech, by number or name; 0
// when it is neither.
func freeTorrent(s string) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "freeleech":
		return freeLeech
	case "neutral":
		return neutral
	case "either":
		return 3
	}
	return 0
}

// ParseResponse implements indexer.Site, as Prowlarr's GazelleGamesParser
// does.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("gazellegames: %w", err)
	}
	if resp.Status != "success" || !isObject(resp.Response) {
		return nil, nil
	}
	groups, err := inOrder[group](resp.Response)
	if err != nil {
		return nil, fmt.Errorf("gazellegames: %w", err)
	}
	passkey := s.currentPasskey()
	var releases []*indexer.Release
	for _, g := range groups {
		if !isObject(g.value.Torrents) {
			continue
		}
		torrents, err := inOrder[torrent](g.value.Torrents)
		if err != nil {
			return nil, fmt.Errorf("gazellegames: %w", err)
		}
		var cats []int
		for _, a := range g.value.Artists {
			for _, id := range indexer.IDs(s.caps.Categories.MapTrackerDesc(a.Name)) {
				if !slices.Contains(cats, id) {
					cats = append(cats, id)
				}
			}
		}
		for _, t := range torrents {
			if strings.ToUpper(string(t.value.TorrentType)) != "TORRENT" {
				continue
			}
			rel, err := s.release(g, t, passkey)
			if err != nil {
				return nil, err
			}
			if rel == nil {
				continue
			}
			// As Prowlarr does, a group without platforms takes the
			// category of its first torrent for all of them.
			if len(cats) == 0 {
				cats = indexer.IDs(s.caps.Categories.MapTracker(strconv.Itoa(int(t.value.CategoryID))))
			}
			rel.Categories = cats
			releases = append(releases, rel)
		}
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

// release returns the release of a group's torrent; nil when only
// freeleech releases are searched and it is not one.
func (s *Site) release(g keyed[group], t keyed[torrent], passkey string) (*indexer.Release, error) {
	free := freeTorrent(string(t.value.FreeTorrent))
	down := 1.0
	if free == freeLeech || free == neutral || bool(t.value.LowSeedFL) {
		down = 0
	}
	if s.settings.FreeleechOnly && down != 0 {
		return nil, nil
	}
	up := 1.0
	if free == neutral {
		up = 0
	}
	size, err := strconv.ParseInt(string(t.value.Size), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("gazellegames: size %q: %w", t.value.Size, err)
	}
	info := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "id", g.key), "torrentid", t.key)
	seeders := int(t.value.Seeders)
	rel := &indexer.Release{
		GUID:                 info,
		InfoURL:              info,
		DownloadURL:          s.downloadURL(t.key, passkey),
		Title:                title(&g.value, &t.value),
		Protocol:             indexer.ProtocolTorrent,
		Files:                indexer.Ptr(int(t.value.FileCount)),
		Size:                 &size,
		Seeders:              &seeders,
		Peers:                indexer.Ptr(seeders + int(t.value.Leechers)),
		PublishDate:          t.value.Time.UTC(),
		Scene:                t.value.Scene == 1,
		DownloadVolumeFactor: &down,
		UploadVolumeFactor:   &up,
		// At least 3 days and 8 hours, 80 hours in all.
		MinimumSeedTime: indexer.Ptr(int64(288000)),
	}
	if t.value.Snatched != nil {
		rel.Grabs = indexer.Ptr(int(*t.value.Snatched))
	}
	return rel, nil
}

var year = regexp.MustCompile(`\b(?:19|20|21)\d{2}\b`)

// title returns a torrent's title: its release title, the group's year
// when it has none, the edition, then its format, platforms, language,
// region and the like.
func title(g *group, t *torrent) string {
	title := html.UnescapeString(string(t.ReleaseTitle))
	if g.Year != nil && *g.Year > 0 && strings.TrimSpace(title) != "" && !year.MatchString(title) {
		title += fmt.Sprintf(" (%d)", *g.Year)
	}
	if strings.TrimSpace(string(t.RemasterTitle)) != "" {
		title += " [" + strings.TrimSpace(html.UnescapeString(string(t.RemasterTitle))+" "+string(t.RemasterYear)) + "]"
	}
	flags := []string{strings.TrimSpace(string(t.Format) + " " + string(t.Encoding))}
	if len(g.Artists) > 0 {
		names := make([]string, len(g.Artists))
		for i, a := range g.Artists {
			names[i] = a.Name
		}
		flags = append(flags, strings.Join(names, ", "))
	}
	flags = append(flags, string(t.Language), string(t.Region), string(t.Miscellaneous))
	if t.Dupable == 1 {
		flags = append(flags, "Trumpable")
	}
	flags = slices.DeleteFunc(flags, func(f string) bool { return strings.TrimSpace(f) == "" })
	if len(flags) > 0 {
		title += " [" + strings.Join(flags, " / ") + "]"
	}
	if dox := strings.TrimSpace(string(t.GameDOXType)); dox != "" {
		title += " [" + dox + "]"
	}
	return title
}

// downloadURL returns the link downloading a torrent. The site needs an
// authkey but does not check it, so a dummy one spares tracking the one
// it changes now and then.
func (s *Site) downloadURL(id, passkey string) string {
	u := indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "action", "download")
	u = indexer.AddQueryParam(indexer.AddQueryParam(u, "id", id), "authkey", "prowlarr")
	return indexer.AddQueryParam(u, "torrent_pass", passkey)
}

func (s *Site) currentPasskey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.passkey
}

// fetchPasskey asks the API for the user's passkey and keeps it.
func (s *Site) fetchPasskey(ctx context.Context, ix *indexer.Indexer) (string, error) {
	resp, err := ix.Do(ctx, s.apiRequest("request=quick_user"))
	if err != nil {
		return "", fmt.Errorf("gazellegames: passkey: %w", err)
	}
	var user struct {
		Status   jsonx.String `json:"status"`
		Response *struct {
			PassKey string `json:"passkey"`
		} `json:"response"`
	}
	if json.Unmarshal([]byte(resp.Content()), &user) != nil || user.Status != "success" || user.Response == nil || strings.TrimSpace(user.Response.PassKey) == "" {
		return "", fmt.Errorf("failed to authenticate with GazelleGames: %w", indexer.ErrAuth)
	}
	s.mu.Lock()
	s.passkey = user.Response.PassKey
	s.mu.Unlock()
	return user.Response.PassKey, nil
}

// TestSite implements indexer.SiteTester: it asks for the passkey, which
// downloads need.
func (s *Site) TestSite(ctx context.Context, ix *indexer.Indexer) error {
	_, err := s.fetchPasskey(ctx, ix)
	return err
}

// DownloadRequest implements indexer.DownloadSite: links found before
// the passkey was known get it, asked for when still unknown.
func (s *Site) DownloadRequest(ctx context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	if strings.Contains(link, "torrent_pass=&") || strings.HasSuffix(link, "torrent_pass=") {
		passkey := s.currentPasskey()
		if passkey == "" {
			var err error
			if passkey, err = s.fetchPasskey(ctx, ix); err != nil {
				return nil, err
			}
		}
		link = indexer.AddQueryParam(indexer.RemoveQueryParam(link, "torrent_pass"), "torrent_pass", passkey)
	}
	return indexer.NewRequest(link, ""), nil
}

// keyed is a value of a JSON object with its key.
type keyed[T any] struct {
	key   string
	value T
}

// inOrder reads the values of a JSON object, in its order.
func inOrder[T any](data json.RawMessage) ([]keyed[T], error) {
	d := json.NewDecoder(bytes.NewReader(data))
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	var out []keyed[T]
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return nil, err
		}
		var v T
		if err := d.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, keyed[T]{key: fmt.Sprint(k), value: v})
	}
	return out, nil
}

// isObject reports whether a JSON value is an object; the API answers
// empty arrays for empty objects.
func isObject(data json.RawMessage) bool {
	return len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '{'
}

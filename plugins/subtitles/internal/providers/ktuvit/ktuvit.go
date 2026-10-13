// Package ktuvit is the Ktuvit provider of Hebrew subtitles, ported from
// Bazarr's ktuvit.py: Ktuvit's services with an account, the titles found
// by IMDb ID through TMDB, and their subtitles read from Ktuvit's pages.
package ktuvit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

// Name is the provider's, as Bazarr names it.
const Name = "ktuvit"

const (
	userAgent            = "Mavio/0.1"
	serverURL            = "https://www.ktuvit.me/"
	tmdbURL              = "http://api.tmdb.org"
	signInPath           = "Services/MembershipService.svc/Login"
	searchPath           = "Services/ContentProvider.svc/SearchPage_search"
	movieInfoPath        = "MovieInfo.aspx?ID="
	episodeInfoPath      = "Services/GetModuleAjax.ashx?"
	requestDownloadPath  = "Services/ContentProvider.svc/RequestSubtitleDownload"
	downloadPath         = "Services/DownloadFile.ashx?DownloadIdentifier="
	noSubtitles          = "אין כתוביות"
	tmdbAPIKey           = "a51ee051bcd762543373903de296e0a3"
	timeout              = 10 * time.Second
	imdbCacheExpiration  = 3 * 7 * 24 * time.Hour
	minimumContentLength = 10
)

// Definition returns the provider's definition, with Bazarr's settings.
func Definition() provider.Definition {
	return provider.Definition{
		Name: Name, DisplayName: "Ktuvit",
		Settings: []provider.Setting{
			{Name: "email", Label: "Email", Help: "Hebrew Subtitles Provider", Type: provider.Text},
			{Name: "hashed_password", Label: "Hashed Password", Type: provider.Text},
		},
		Languages: []string{"he"},
		Movies:    true, Episodes: true,
		New: New,
	}
}

// imdbEntry is a cached IMDb ID.
type imdbEntry struct {
	id      string
	expires time.Time
}

// Provider searches Ktuvit.
type Provider struct {
	env provider.Env
	s   *site.Session
	// login is the session logging in, which does not follow redirects
	// as Bazarr's.
	login                 *site.Session
	email, hashedPassword string
	server, tmdb          string

	mu sync.Mutex
	// initialized and loggedIn are as Bazarr's initialize leaves them.
	initialized, loggedIn bool
	imdb                  map[string]imdbEntry
}

// New makes the provider from its settings: an email and a hashed
// password, both or neither, as Bazarr's.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s struct {
		Email          string `json:"email"`
		HashedPassword string `json:"hashed_password"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
	}
	if (s.Email == "") != (s.HashedPassword == "") {
		return nil, provider.Errorf(provider.ConfigurationError, "Email and Hashed Password must be specified")
	}
	hc := env.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	sess := site.New(hc)
	sess.Header.Set("User-Agent", userAgent)
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	login := site.New(&c)
	login.Header = sess.Header
	login.Check = func(*site.Response) error { return nil }
	return &Provider{
		env: env, s: sess, login: login, email: s.Email, hashedPassword: s.HashedPassword,
		server: serverURL, tmdb: tmdbURL, imdb: map[string]imdbEntry{},
	}, nil
}

// SetBaseURL points the provider, Ktuvit and TMDB alike, at another
// server, for tests.
func (p *Provider) SetBaseURL(u *url.URL) {
	base := strings.TrimRight(u.String(), "/")
	p.server, p.tmdb = base+"/", base
}

// do makes a request with Bazarr's timeout.
func (p *Provider) do(ctx context.Context, method, u string, body any) (*site.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if method == http.MethodPost {
		return p.s.PostJSON(ctx, u, body)
	}
	return p.s.Get(ctx, u, nil)
}

// initialize logs in once when an account is set, as Bazarr's initialize.
func (p *Provider) initialize(ctx context.Context) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized || p.email == "" {
		return p.loggedIn, nil
	}
	p.env.Log.InfoContext(ctx, "Logging in to Ktuvit")
	req := map[string]any{"request": map[string]string{"Email": p.email, "Password": p.hashedPassword}}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r, err := p.login.PostJSON(ctx, p.server+signInPath, req)
	if err != nil {
		return false, err
	}
	p.initialized = true
	if len(r.Body) == 0 {
		return false, nil
	}
	success, err := field(r, "IsSuccess", false)
	if err != nil || success != true {
		message, merr := field(r, "ErrorMessage", "[None]")
		if merr != nil {
			return false, provider.Errorf(provider.AuthenticationError, "Error Logging in to Ktuvit Provider: %s", r.Body)
		}
		return false, provider.Errorf(provider.AuthenticationError, "Error Logging in to Ktuvit Provider: %v", message)
	}
	cookie := strings.Split(strings.Join(r.Header.Values("Set-Cookie"), ", "), "Login=")
	if len(cookie) != 2 {
		return false, provider.Errorf(provider.AuthenticationError, "Login Failed, didn't receive valid cookie in response")
	}
	// The session keeps the login cookie.
	if u, err := url.Parse(p.server); err == nil {
		p.s.Jar().SetCookies(u, (&http.Response{Header: r.Header}).Cookies())
	}
	p.env.Log.DebugContext(ctx, "Logged in to Ktuvit")
	p.loggedIn = true
	return true, nil
}

// field returns a field of a "d" answer, as Bazarr's parse_d_response: a
// missing or empty field other than its default is a parse error.
func field(r *site.Response, name string, def any) (any, error) {
	var outer map[string]any
	if err := json.Unmarshal(r.Body, &outer); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "Unable to parse JSON returned while getting %s: %w", name, err)
	}
	d, ok := outer["d"].(string)
	if !ok {
		return nil, provider.Errorf(provider.ParseResponseError, "Incomplete JSON returned while getting %s", name)
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(d), &inner); err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "Unable to parse JSON returned while getting %s: %w", name, err)
	}
	v, ok := inner[name]
	if !ok {
		v = def
	}
	if !truthy(v) && fmt.Sprint(v) != fmt.Sprint(def) {
		return nil, provider.Errorf(provider.ParseResponseError, "Missing %s", name)
	}
	return v, nil
}

// truthy is Python's bool of a JSON value.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

// searchIMDbID returns the IMDb ID of a title through TMDB, as Bazarr's
// _search_imdb_id; "" when none is found.
func (p *Provider) searchIMDbID(ctx context.Context, title string, year int, movie bool) (string, error) {
	key := fmt.Sprintf("%s\x00%d\x00%t", title, year, movie)
	now := p.env.Now()
	p.mu.Lock()
	if e, ok := p.imdb[key]; ok && now.Before(e.expires) {
		p.mu.Unlock()
		return e.id, nil
	}
	p.mu.Unlock()
	p.env.Log.InfoContext(ctx, "Searching IMDB ID", "title", title, "year", year)
	category := "tv"
	if movie {
		category = "movie"
	}
	q := url.Values{"api_key": {tmdbAPIKey}, "query": {strings.ReplaceAll(title, "'", "")}, "language": {"en"}}
	if year != 0 {
		q.Set("year", fmt.Sprint(year))
	}
	r, err := p.do(ctx, http.MethodGet, p.tmdb+"/3/search/"+category+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	var search struct {
		Results []struct {
			ID any `json:"id"`
		} `json:"results"`
	}
	if err := r.JSON(&search); err != nil {
		return "", err
	}
	id := ""
	if len(search.Results) > 0 && truthy(search.Results[0].ID) {
		suffix := "/external_ids"
		if movie {
			suffix = ""
		}
		tmdbID := fmt.Sprint(search.Results[0].ID)
		if f, ok := search.Results[0].ID.(float64); ok {
			tmdbID = fmt.Sprint(int64(f))
		}
		r, err := p.do(ctx, http.MethodGet, fmt.Sprintf("%s/3/%s/%s%s?api_key=%s&language=en", p.tmdb, category, tmdbID, suffix, tmdbAPIKey), nil)
		if err != nil {
			return "", err
		}
		var ids struct {
			IMDbID any `json:"imdb_id"`
		}
		if err := r.JSON(&ids); err != nil {
			return "", err
		}
		if truthy(ids.IMDbID) {
			id = fmt.Sprint(ids.IMDbID)
		}
	}
	p.mu.Lock()
	p.imdb[key] = imdbEntry{id: id, expires: now.Add(imdbCacheExpiration)}
	p.mu.Unlock()
	return id, nil
}

// found is a subtitle of a film's page.
type found struct {
	filmID, subID, release string
}

// query searches a title, as Bazarr's query: the films with the IMDb ID,
// then their subtitles.
func (p *Provider) query(ctx context.Context, title string, season, episode, year int, imdbID string) ([]found, error) {
	movie := season == 0 || episode == 0
	if imdbID == "" {
		var err error
		if imdbID, err = p.searchIMDbID(ctx, title, year, movie); err != nil || imdbID == "" {
			return nil, err
		}
	}
	p.env.Log.DebugContext(ctx, "Using IMDB ID", "imdb", imdbID)
	q := map[string]any{
		"FilmName": title, "Actors": []any{}, "Studios": []any{}, "Directors": []any{}, "Genres": []any{},
		"Countries": []any{}, "Languages": []any{}, "Year": "", "Rating": []any{}, "Page": 1, "SearchType": "0",
		"WithSubsOnly": false,
	}
	if !movie {
		q["SearchType"] = "1"
	}
	if year != 0 {
		q["Year"] = year
	}
	films, err := p.searchFilms(ctx, q)
	if err != nil {
		return nil, err
	}
	// Ktuvit's entry may have no year, so a yearless search follows, as
	// Bazarr's.
	if len(films) == 0 && year != 0 {
		p.env.Log.DebugContext(ctx, "No films returned with Year filter; retrying search without Year")
		q["Year"] = ""
		if films, err = p.searchFilms(ctx, q); err != nil {
			return nil, err
		}
	}
	var out []found
	seen := map[string]int{}
	for _, f := range films {
		link, _ := f["IMDB_Link"].(string)
		link = strings.TrimSuffix(link, "/")
		if link[strings.LastIndex(link, "/")+1:] != imdbID {
			continue
		}
		id := fmt.Sprint(f["ID"])
		var subs []found
		if movie {
			subs, err = p.searchMovie(ctx, id)
		} else {
			subs, err = p.searchTVShow(ctx, id, season, episode)
		}
		if err != nil {
			return nil, err
		}
		p.env.Log.DebugContext(ctx, "Got subtitles from Ktuvit", "count", len(subs))
		for _, s := range subs {
			if i, ok := seen[s.subID]; ok {
				out[i] = s
				continue
			}
			seen[s.subID] = len(out)
			out = append(out, s)
		}
	}
	return out, nil
}

// searchFilms returns the films of a search, as Bazarr's _search_films.
func (p *Provider) searchFilms(ctx context.Context, q map[string]any) ([]map[string]any, error) {
	r, err := p.do(ctx, http.MethodPost, p.server+searchPath, map[string]any{"request": q})
	if err != nil || len(r.Body) == 0 {
		return nil, err
	}
	v, err := field(r, "Films", []any{})
	if err != nil {
		return nil, err
	}
	list, _ := v.([]any)
	var films []map[string]any
	for _, f := range list {
		if m, ok := f.(map[string]any); ok {
			films = append(films, m)
		}
	}
	return films, nil
}

// releaseOf is the first line of a cell's text, as Bazarr's.
func releaseOf(n site.Node) string {
	return strings.Split(strings.TrimSpace(n.Text()), "\n")[0]
}

// searchTVShow lists an episode's subtitles, as Bazarr's _search_tvshow.
func (p *Provider) searchTVShow(ctx context.Context, id string, season, episode int) ([]found, error) {
	u := fmt.Sprintf("%s%smoduleName=SubtitlesList&SeriesID=%s&Season=%d&Episode=%d", p.server, episodeInfoPath, id, season, episode)
	r, err := p.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) < minimumContentLength {
		return nil, nil
	}
	// The rows come without their table, which an HTML parser drops.
	doc, err := site.Parse(append(append([]byte("<table>"), r.Body...), "</table>"...), r.Header.Get("Content-Type"))
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%w", err)
	}
	rows := doc.All("tr")
	if len(rows) > 0 {
		if td := rows[0].First("td"); td.Valid() && td.Text() == noSubtitles {
			p.env.Log.DebugContext(ctx, "No Subtitles Found", "url", u)
			return nil, nil
		}
	}
	return subtitlesOf(rows, id, "input[data-sub-id]", "data-sub-id"), nil
}

// searchMovie lists a movie's subtitles, as Bazarr's _search_movie.
func (p *Provider) searchMovie(ctx context.Context, id string) ([]found, error) {
	r, err := p.do(ctx, http.MethodGet, p.server+movieInfoPath+id, nil)
	if err != nil {
		return nil, err
	}
	if len(r.Body) < minimumContentLength {
		return nil, nil
	}
	doc, err := r.HTML()
	if err != nil {
		return nil, provider.Errorf(provider.ParseResponseError, "%w", err)
	}
	return subtitlesOf(doc.All("table#subtitlesList tbody > tr"), id, "a[data-subtitle-id]", "data-subtitle-id"), nil
}

// subtitlesOf reads the rows of a list of subtitles: the release in the
// first cell, the ID in the sixth.
func subtitlesOf(rows []site.Node, id, selector, attr string) []found {
	var out []found
	for _, row := range rows {
		cells := directCells(row)
		if len(cells) < 6 {
			continue
		}
		link := cells[5].First(selector)
		if !link.Valid() {
			continue
		}
		out = append(out, found{filmID: id, subID: link.Attr(attr), release: releaseOf(cells[0])})
	}
	return out
}

// directCells are a row's cells, as BeautifulSoup's find_all("td").
func directCells(row site.Node) []site.Node { return row.All("td") }

// Search lists the Hebrew subtitles of a video, trying its titles in
// turn, as Bazarr's list_subtitles; nothing when not logged in.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	loggedIn, err := p.initialize(ctx)
	if err != nil {
		return nil, err
	}
	if !loggedIn {
		p.env.Log.InfoContext(ctx, "Not logged in to Ktuvit. Returning 0 results")
		return nil, nil
	}
	hebrew := provider.Language{Tag: "he"}
	if !slices.Contains(langs, hebrew) {
		return nil, nil
	}
	titles := append([]string{v.Title}, v.AlternativeTitles...)
	imdbID := v.IMDbID()
	season, episode := 0, 0
	if v.Episode {
		titles = append([]string{v.Series}, v.AlternativeSeries...)
		imdbID = v.SeriesExternalIDs["imdb"]
		season, episode = v.Season, v.Number
	}
	for _, title := range titles {
		subs, err := p.query(ctx, title, season, episode, v.Year, imdbID)
		if err != nil {
			return nil, err
		}
		if len(subs) == 0 {
			continue
		}
		out := make([]provider.Subtitle, 0, len(subs))
		for _, s := range subs {
			out = append(out, provider.Subtitle{
				ID: s.filmID + "/" + s.subID, Language: hebrew, ReleaseNames: []string{s.release},
				Matches: matches(v, imdbID), PageLink: p.server + movieInfoPath + s.filmID,
			})
		}
		return out, nil
	}
	return nil, nil
}

// matches are those of Bazarr's get_matches: the title searched is the
// video's, and so are the season and episode; the release is matched by
// the server.
func matches(v *provider.Video, imdbID string) []string {
	if !v.Episode {
		if v.Title == "" {
			return nil
		}
		return []string{"title"}
	}
	var m []string
	if v.Series != "" {
		m = append(m, "series")
	}
	if v.Season != 0 {
		m = append(m, "season")
	}
	if v.Number != 0 {
		m = append(m, "episode")
	}
	if id := v.SeriesExternalIDs["imdb"]; id != "" && id == imdbID {
		m = append(m, "series_imdb_id")
	}
	return m
}

// Download asks Ktuvit for a download identifier, then fetches the
// subtitle, as Bazarr's download_subtitle.
func (p *Provider) Download(ctx context.Context, _ *provider.Video, id string) (provider.File, error) {
	filmID, subID, ok := strings.Cut(id, "/")
	if !ok {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "bad subtitle ID %q", id)
	}
	if _, err := p.initialize(ctx); err != nil {
		return provider.File{}, err
	}
	req := map[string]any{"request": map[string]any{
		"FilmID": filmID, "SubtitleID": subID, "FontSize": 0, "FontColor": "", "PredefinedLayout": -1,
	}}
	r, err := p.do(ctx, http.MethodPost, p.server+requestDownloadPath, req)
	if err != nil {
		return provider.File{}, err
	}
	if len(r.Body) == 0 {
		return provider.File{}, provider.Errorf(provider.ParseResponseError, "no download identifier")
	}
	ident, err := field(r, "DownloadIdentifier", nil)
	if err != nil {
		return provider.File{}, err
	}
	if ident == nil {
		return provider.File{}, provider.Errorf(provider.ParseResponseError, "Missing DownloadIdentifier")
	}
	r, err = p.do(ctx, http.MethodGet, p.server+downloadPath+url.QueryEscape(fmt.Sprint(ident)), nil)
	if err != nil {
		return provider.File{}, err
	}
	if len(bytes.TrimSpace(r.Body)) == 0 {
		return provider.File{}, provider.Errorf(provider.BadSubtitle, "Unable to download subtitle. No data returned from provider")
	}
	data := archive.FixLineEndings(r.Body)
	return provider.File{Data: data, Format: archive.Detect(data)}, nil
}

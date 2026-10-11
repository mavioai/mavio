package newznab

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// params are a request's parameters in the order they are set; setting
// one again replaces it, as NameValueCollection.Set does.
type params struct {
	keys   []string
	values map[string]string
}

func (p *params) set(k, v string) {
	if p.values == nil {
		p.values = map[string]string{}
	}
	if _, ok := p.values[k]; !ok {
		p.keys = append(p.keys, k)
	}
	p.values[k] = v
}

func (p *params) encode() string {
	parts := make([]string, len(p.keys))
	for i, k := range p.keys {
		parts[i] = k + "=" + url.QueryEscape(p.values[k])
	}
	return strings.Join(parts, "&")
}

// SearchRequests returns the request of a search, as Prowlarr's
// NewznabRequestGenerator makes it: by the IDs the indexer takes, or by
// the term alone through a basic search when it takes none of them.
func SearchRequests(s Settings, caps *indexer.Capabilities, q *indexer.Query) *indexer.Chain {
	var p params
	has := func(param indexer.Param) bool { return caps.Has(q.Type, param) }
	t := ""
	switch q.Type {
	case indexer.SearchMovie:
		t = "movie"
		if q.TMDBID != 0 && has(indexer.ParamTMDBID) {
			p.set("tmdbid", strconv.Itoa(q.TMDBID))
		}
		if q.IMDbID != "" && has(indexer.ParamIMDbID) {
			p.set("imdbid", q.IMDbID)
		}
		if q.TraktID != 0 && has(indexer.ParamTraktID) {
			p.set("traktid", strconv.Itoa(q.TraktID))
		}
	case indexer.SearchMusic:
		t = "music"
		if q.Artist != "" && has(indexer.ParamArtist) {
			p.set("artist", q.Artist)
		}
		if q.Album != "" && has(indexer.ParamAlbum) {
			p.set("album", q.Album)
		}
	case indexer.SearchTV:
		t = "tvsearch"
		if q.TVDBID != 0 && has(indexer.ParamTVDBID) {
			p.set("tvdbid", strconv.Itoa(q.TVDBID))
		}
		if q.TMDBID != 0 && has(indexer.ParamTMDBID) {
			p.set("tmdbid", strconv.Itoa(q.TMDBID))
		}
		if q.IMDbID != "" && has(indexer.ParamIMDbID) {
			p.set("imdbid", q.IMDbID)
		}
		if q.TVMazeID != 0 && has(indexer.ParamTVMazeID) {
			p.set("tvmazeid", strconv.Itoa(q.TVMazeID))
		}
		if q.RID != 0 && has(indexer.ParamRID) {
			p.set("rid", strconv.Itoa(q.RID))
		}
		if q.Season != nil && has(indexer.ParamSeason) {
			// NNTMux takes season=0 for none; "00" works everywhere.
			season := strconv.Itoa(*q.Season)
			if *q.Season == 0 {
				season = "00"
			}
			p.set("season", season)
		}
		if strings.TrimSpace(q.Episode) != "" && has(indexer.ParamEp) {
			p.set("ep", q.Episode)
		}
		// Sonarr's AddTitlePageableRequests sends a series' title to the
		// indexers taking it.
		if q.Title != "" && has(indexer.ParamTitle) {
			p.set("title", q.Title)
		}
	case indexer.SearchBook:
		t = "book"
		if q.Author != "" && has(indexer.ParamAuthor) {
			p.set("author", q.Author)
		}
		if q.Title != "" && has(indexer.ParamTitle) {
			p.set("title", q.Title)
		}
	}
	term := strings.TrimSpace(q.Term) != ""
	if len(p.keys) == 0 {
		// Sphinx search returns garbage without IDs, so terms alone go
		// through a basic search.
		t = "search"
		if term && caps.Available(indexer.SearchBasic) {
			p.set("q", newznabifyTitle(q.Term))
		}
	} else if term && caps.Available(q.Type) {
		p.set("q", newznabifyTitle(q.Term))
	}

	u := s.apiURL() + "?t=" + t + "&extended=1"
	if cats := caps.Categories.MapToTracker(q.Categories, false); len(cats) > 0 {
		u += "&cat=" + strings.Join(cats, ",")
	}
	u += strings.TrimSpace(s.AdditionalParameters)
	if strings.TrimSpace(s.APIKey) != "" {
		u += "&apikey=" + s.APIKey
	}
	if q.Limit > 0 {
		p.set("limit", strconv.Itoa(q.Limit))
	}
	// Prowlarr sends the offset when the search has one; queries always
	// do.
	p.set("offset", strconv.Itoa(q.Offset))
	if q.MinAge != nil {
		p.set("minage", strconv.Itoa(*q.MinAge))
	}
	if q.MaxAge != nil {
		p.set("maxage", strconv.Itoa(*q.MaxAge))
	}
	if q.MinSize != nil {
		p.set("minsize", strconv.FormatInt(*q.MinSize, 10))
	}
	if q.MaxSize != nil {
		p.set("maxsize", strconv.FormatInt(*q.MaxSize, 10))
	}
	u += "&" + p.encode()

	r := indexer.NewRequest(u, indexer.AcceptRSS)
	r.FollowRedirects = true
	var c indexer.Chain
	c.AddRequests(r)
	return &c
}

// newznabifyTitle writes pluses as Prowlarr does, which indexers would
// otherwise take for spaces.
func newznabifyTitle(title string) string {
	return strings.ReplaceAll(title, "+", "%20")
}

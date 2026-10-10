package cardigann

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// searchState is what reading a search's response needs.
type searchState struct {
	vars Vars
	path *SearchPathBlock
}

// searchTypes are the names definitions know search types by.
var searchTypes = map[indexer.SearchType]string{
	indexer.SearchBasic: "search",
	indexer.SearchTV:    "tvsearch",
	indexer.SearchMovie: "movie",
	indexer.SearchMusic: "music",
	indexer.SearchBook:  "book",
}

// optional returns s, or nil when it is empty.
func optional(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// optionalInt returns n as a string, or nil when it is 0.
func optionalInt(n int) any {
	if n == 0 {
		return nil
	}
	return strconv.Itoa(n)
}

// queryVariables returns the variables of a search, as Prowlarr's
// GetSearchRequests overloads set them.
func (e *engine) queryVariables(q *indexer.Query) (Vars, error) {
	vars, err := e.baseVariables()
	if err != nil {
		return nil, err
	}
	vars[".Query.Type"] = searchTypes[q.Type]
	vars[".Query.Q"] = optional(q.Term)
	vars[".Query.Categories"] = slices.Clone(q.Categories)
	vars[".Query.Limit"] = optionalInt(q.Limit)
	vars[".Query.Offset"] = optionalInt(q.Offset)
	for _, name := range []string{
		"Extended", "APIKey", "Genre",
		"Movie", "Year", "IMDBID", "IMDBIDShort", "TMDBID",
		"Series", "Ep", "Season", "TVDBID", "TVRageID", "TVMazeID", "TraktID", "DoubanID", "Episode",
		"Album", "Artist", "Label", "Track",
		"Author", "Title", "Publisher",
	} {
		vars[".Query."+name] = nil
	}
	imdb := func() {
		vars[".Query.IMDBID"] = optional(q.FullIMDbID())
		vars[".Query.IMDBIDShort"] = optional(strings.TrimPrefix(q.IMDbID, "tt"))
	}
	switch q.Type {
	case indexer.SearchMovie:
		vars[".Query.Year"] = optionalInt(q.Year)
		vars[".Query.Genre"] = optional(q.Genre)
		imdb()
		vars[".Query.TMDBID"] = optionalInt(q.TMDBID)
		vars[".Query.TraktID"] = optionalInt(q.TraktID)
		vars[".Query.DoubanID"] = optionalInt(q.DoubanID)
	case indexer.SearchTV:
		vars[".Query.Ep"] = optional(q.Episode)
		if q.Season != nil {
			vars[".Query.Season"] = strconv.Itoa(*q.Season)
		}
		vars[".Query.Genre"] = optional(q.Genre)
		vars[".Query.Year"] = optionalInt(q.Year)
		imdb()
		vars[".Query.TVDBID"] = optionalInt(q.TVDBID)
		vars[".Query.TMDBID"] = optionalInt(q.TMDBID)
		vars[".Query.TVRageID"] = optionalInt(q.RID)
		vars[".Query.TVMazeID"] = optionalInt(q.TVMazeID)
		vars[".Query.TraktID"] = optionalInt(q.TraktID)
		vars[".Query.DoubanID"] = optionalInt(q.DoubanID)
		vars[".Query.Episode"] = q.EpisodeString()
	case indexer.SearchMusic:
		vars[".Query.Album"] = optional(q.Album)
		vars[".Query.Artist"] = optional(q.Artist)
		vars[".Query.Label"] = optional(q.Label)
		vars[".Query.Genre"] = optional(q.Genre)
		vars[".Query.Year"] = optionalInt(q.Year)
		vars[".Query.Track"] = optional(q.Track)
	case indexer.SearchBook:
		vars[".Query.Author"] = optional(q.Author)
		vars[".Query.Title"] = optional(q.Title)
		vars[".Query.Genre"] = optional(q.Genre)
		vars[".Query.Publisher"] = optional(q.Publisher)
		vars[".Query.Year"] = optionalInt(q.Year)
	}
	return vars, nil
}

// searchRequests returns the requests of a search, one per search path
// whose categories it searches, as Prowlarr's GetRequest does.
func (e *engine) searchRequests(q *indexer.Query) ([]*indexer.Request, error) {
	vars, err := e.queryVariables(q)
	if err != nil {
		return nil, err
	}
	search := e.def.Search
	mapped := e.categories.MapToTracker(q.Categories, false)
	if len(mapped) == 0 {
		mapped = slices.Clone(e.defaultCategories)
	}

	var keywords []string
	for _, key := range []string{"Q", "Series", "Movie", "Year"} {
		if s, _ := vars[".Query."+key].(string); !isBlank(s) {
			keywords = append(keywords, s)
		}
	}
	if s, _ := vars[".Query.Episode"].(string); !isBlank(s) {
		keywords = append(keywords, s)
	}
	vars[".Query.Keywords"] = strings.Join(keywords, " ")
	if vars[".Keywords"], err = e.applyFilters(strings.Join(keywords, " "), search.KeywordsFilters, vars); err != nil {
		return nil, err
	}

	var requests []*indexer.Request
	var urls []string
	for i := range search.Paths {
		path := &search.Paths[i]
		vars[".Categories"] = mapped
		// Paths for other categories are skipped; "!" first inverts them.
		if path.Categories != nil && len(mapped) > 0 {
			has := slices.ContainsFunc(mapped, func(c string) bool { return slices.Contains(path.Categories, c) })
			if path.Categories[0] == "!" {
				has = !has
			}
			if !has {
				continue
			}
			vars[".Categories"] = intersect(mapped, path.Categories)
		}

		p, err := e.applyTemplate(path.Path, vars, func(s string) string { return urlEncode(s, nil) })
		if err != nil {
			return nil, err
		}
		searchURL, err := e.resolvePath(strings.ReplaceAll(p, "+", "%20"), "")
		if err != nil {
			return nil, err
		}
		method := http.MethodGet
		if strings.EqualFold(path.Method, "post") {
			method = http.MethodPost
		}

		var args []Entry[string]
		inputs := []Ordered[string]{}
		if path.inheritsInputs() {
			inputs = append(inputs, search.Inputs)
		}
		inputs = append(inputs, path.Inputs)
		for _, in := range inputs {
			for _, input := range in {
				if input.Key == "$raw" {
					raw, err := e.applyTemplate(input.Value, vars, func(s string) string { return urlEncode(s, nil) })
					if err != nil {
						return nil, err
					}
					for part := range strings.SplitSeq(raw, "&") {
						k, v, _ := strings.Cut(part, "=")
						if k != "" {
							args = append(args, Entry[string]{k, v})
						}
					}
					continue
				}
				v, err := e.applyTemplate(input.Value, vars, nil)
				if err != nil {
					return nil, err
				}
				if !isBlank(v) || search.AllowEmptyInputs {
					args = append(args, Entry[string]{input.Key, v})
				}
			}
		}
		if method == http.MethodGet && len(args) > 0 {
			searchURL += "?" + queryString(args, e.encoding, "&")
		}
		if method == http.MethodGet && slices.Contains(urls, searchURL) {
			e.log.Debug("cardigann: skipping a repeated request", "definition", e.def.ID, "url", searchURL)
			continue
		}
		urls = append(urls, searchURL)

		r := indexer.NewRequest(searchURL, "")
		if method == http.MethodPost {
			e.setForm(r, args)
		}
		if search.Headers != nil {
			h, err := e.headers(search.Headers, vars)
			if err != nil {
				return nil, err
			}
			setHeaders(r, h)
		}
		r.FollowRedirects = path.FollowRedirect
		// The variables are those of this path; responses add theirs.
		r.State = &searchState{vars: vars.clone(), path: path}
		requests = append(requests, r)
	}
	return requests, nil
}

// intersect returns the items of a that b has, in a's order, once each.
func intersect(a, b []string) []string {
	var out []string
	for _, v := range a {
		if slices.Contains(b, v) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// setForm makes a request a POST of a form, its values encoded as the
// site's.
func (e *engine) setForm(r *indexer.Request, form []Entry[string]) {
	r.Method = http.MethodPost
	r.Body = []byte(queryString(form, e.encoding, "&"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
}

func setHeaders(r *indexer.Request, h map[string]string) {
	for k, v := range h {
		r.Header.Set(k, v)
	}
}

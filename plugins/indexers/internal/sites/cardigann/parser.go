package cardigann

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

// errLoginRedirect is returned for searches redirected to the login page.
var errLoginRedirect = errors.New("we are being redirected to the login page; most likely your session expired or was killed; recheck your cookie or credentials and try testing the indexer")

// parseResponse reads the releases of a search's response, as Prowlarr's
// CardigannParser does.
func (e *engine) parseResponse(r *indexer.Response) ([]*indexer.Release, error) {
	if r.IsRedirect() && r.RedirectURL() != "" {
		if strings.Contains(strings.ToLower(r.RedirectURL()), "/login.php") {
			return nil, indexer.Errorf("%s", errLoginRedirect.Error())
		}
		return nil, indexer.Errorf("Redirected to %s from indexer request", r.RedirectURL())
	}
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	state, ok := r.Request.State.(*searchState)
	if !ok {
		return nil, errors.New("not a response to a search")
	}
	vars := state.vars.clone()
	searchURL := r.Request.URL
	results := r.Content()

	var releases []*indexer.Release
	var err error
	if state.path.Response != nil && state.path.Response.Type == "json" {
		releases, err = e.parseJSONResults(results, state.path.Response, vars, searchURL)
	} else {
		xmlResults := state.path.Response != nil && state.path.Response.Type == "xml"
		releases, err = e.parseHTMLResults(results, xmlResults, vars, searchURL)
	}
	if err != nil {
		return nil, err
	}

	for _, rel := range releases {
		// Magnet links of info hashes, not for private sites.
		if rel.MagnetURL == "" && !isBlank(rel.InfoHash) && e.def.Type != "private" {
			m, err := indexer.PublicMagnetLink(rel.InfoHash, rel.Title)
			if err != nil {
				return nil, err
			}
			rel.MagnetURL = m
		}
		if rel.MagnetURL != "" && isBlank(rel.InfoHash) {
			rel.InfoHash = magnetHash(rel.MagnetURL)
		}
		if !isBlank(rel.Description) && strings.HasPrefix(rel.Description, "Internal") && !rel.HasFlag(indexer.FlagInternal) {
			rel.Flags = append(rel.Flags, indexer.FlagInternal)
		}
	}
	return releases, nil
}

// magnetHash returns the last part of a magnet link's xt argument.
func magnetHash(magnet string) string {
	xt := parse.QueryArg(magnet, "xt")
	return xt[strings.LastIndex(xt, ":")+1:]
}

// parseJSONResults reads the rows of a JSON response.
func (e *engine) parseJSONResults(results string, resp *ResponseBlock, vars Vars, searchURL string) ([]*indexer.Release, error) {
	if msg := resp.NoResultsMessage; msg != nil && ((!isBlank(*msg) && strings.Contains(results, *msg)) || (isBlank(*msg) && isBlank(results))) {
		return nil, nil
	}
	parsed, err := parseJSON(results, e.now().Location())
	if err != nil {
		e.log.Error("cardigann: unable to parse JSON response from indexer", "definition", e.def.ID, "error", err)
		return nil, indexer.Errorf("Error Parsing Json Response")
	}
	rows := &e.def.Search.Rows
	if rows.Count != nil {
		count, _, err := e.handleJSONSelector(rows.Count, parsed, vars, true)
		if err != nil {
			e.log.Debug("cardigann: failed to parse JSON rows count", "definition", e.def.ID, "error", err)
		} else if n, err := strconv.Atoi(count); err == nil && n < 1 {
			return nil, nil
		}
	}
	selector := ""
	if rows.Selector != nil {
		selector = *rows.Selector
	}
	array, ok := e.jsonRows(parsed, selector)
	if !ok {
		if rows.MissingAttributeEqualsNoResults {
			return nil, nil
		}
		return nil, indexer.Errorf("Error Parsing Rows Selector")
	}

	var releases []*indexer.Release
	for _, row := range array {
		sel := row
		if rows.Attribute != nil {
			v, found, err := selectToken(row, *rows.Attribute)
			if err != nil {
				return nil, err
			}
			if !found {
				if rows.MissingAttributeEqualsNoResults {
					continue
				}
				return nil, fmt.Errorf("rows attribute %q not found", *rows.Attribute)
			}
			sel = v
		}
		var objects []any
		if rows.Multiple {
			objects = children([]any{sel})
		} else {
			objects = []any{sel}
		}
		for _, obj := range objects {
			if _, ok := obj.(*jsonObject); !ok {
				return nil, fmt.Errorf("row is %s, not an object", jsonKind(obj))
			}
			rel := &indexer.Release{}
			err := e.parseFields(rel, vars, searchURL, func(f *SelectorBlock, required bool) (string, bool, error) {
				parent := obj
				if f.Selector != nil && strings.HasPrefix(*f.Selector, "..") {
					parent = row
				}
				return e.handleJSONSelector(f, parent, vars, required)
			}, true)
			if err != nil {
				return nil, err
			}
			e.rowFilters(row)
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// parseHTMLResults reads the rows of an HTML or XML response.
func (e *engine) parseHTMLResults(results string, isXML bool, vars Vars, searchURL string) ([]*indexer.Release, error) {
	search := e.def.Search
	parseDoc := parseHTML
	if isXML {
		parseDoc = parseXMLDocument
	}
	if search.PreprocessingFilters != nil {
		var err error
		if results, err = e.applyFilters(results, search.PreprocessingFilters, vars); err != nil {
			return nil, err
		}
	}
	doc, err := parseDoc(results)
	if err != nil {
		return nil, err
	}
	if search.Rows.Selector == nil {
		return nil, errors.New("no rows selector")
	}
	rowsSelector, err := e.applyTemplate(*search.Rows.Selector, vars, nil)
	if err != nil {
		return nil, err
	}
	rows, err := e.sel.queryAll(doc, rowsSelector)
	if err != nil {
		return nil, err
	}

	// Rows are merged with the given number of rows after them.
	if after := search.Rows.After; after > 0 {
		for i := 0; i < len(rows); i++ {
			for j := range after {
				k := i + j + 1
				if k >= len(rows) {
					return nil, fmt.Errorf("row %d has no %d rows after it", i, after)
				}
				for c := rows[k].FirstChild; c != nil; {
					next := c.NextSibling
					rows[k].RemoveChild(c)
					rows[i].AppendChild(c)
					c = next
				}
			}
			rows = slices.Delete(rows, i+1, i+1+after)
		}
	}

	var releases []*indexer.Release
	for _, row := range rows {
		rel, err := e.parseHTMLRow(row, vars, searchURL)
		if err != nil {
			e.log.Error("cardigann: error while parsing row", "definition", e.def.ID, "row", outerHTML(row), "error", err)
			continue
		}
		releases = append(releases, rel)
	}
	return releases, nil
}

func (e *engine) parseHTMLRow(row *html.Node, vars Vars, searchURL string) (*indexer.Release, error) {
	rel := &indexer.Release{}
	err := e.parseFields(rel, vars, searchURL, func(f *SelectorBlock, required bool) (string, bool, error) {
		return e.handleSelector(f, row, vars, required)
	}, false)
	if err != nil {
		return nil, err
	}
	e.rowFilters(row)

	// Without a date, the date is that of the header rows before.
	if dh := e.def.Search.Rows.DateHeaders; rel.PublishDate.IsZero() && dh != nil {
		prev := previousElement(row)
		if prev == nil {
			if p := parentElement(row); p != nil {
				prev = previousElement(p)
			}
		}
		var value string
		found := false
		for prev != nil {
			cur := prev
			if v, ok, err := e.handleSelector(dh, cur, nil, true); err == nil && ok {
				value, found = v, true
				break
			}
			prev = previousElement(cur)
			if prev == nil {
				if p := parentElement(cur); p != nil {
					prev = previousElement(p)
				}
			}
		}
		if !found && !dh.Optional {
			return nil, fmt.Errorf("no date header row found for %s", rel.Title)
		}
		if found {
			t, err := parse.Unknown(e.now(), value, "")
			if err != nil {
				return nil, err
			}
			rel.PublishDate = t
		}
	}
	return rel, nil
}

// rowFilters applies the rows' filters: andmatch filters releases by the
// query after the search, strdump logs the row.
func (e *engine) rowFilters(row any) {
	for _, f := range e.def.Search.Rows.Filters {
		switch f.Name {
		case "andmatch":
		case "strdump":
			s, ok := row.(*html.Node)
			if ok {
				e.log.Debug("cardigann: row strdump", "definition", e.def.ID, "row", outerHTML(s))
			} else {
				e.log.Debug("cardigann: row strdump", "definition", e.def.ID, "row", jsonText(row))
			}
		default:
			e.log.Error("cardigann: unsupported rows filter", "definition", e.def.ID, "filter", f.Name)
		}
	}
}

// matchesQuery reports whether the definition filters releases by the
// query's terms.
func (e *engine) matchesQuery() bool {
	return slices.ContainsFunc(e.def.Search.Rows.Filters, func(f Filter) bool { return f.Name == "andmatch" })
}

// parseFields reads a row's fields into a release, keeping their values
// as .Result.<name> variables. Failures of required fields fail JSON rows
// and are logged for HTML rows.
func (e *engine) parseFields(rel *indexer.Release, vars Vars, searchURL string, handle func(f *SelectorBlock, required bool) (string, bool, error), failRow bool) error {
	for i := range e.def.Search.Fields {
		field := &e.def.Search.Fields[i]
		parts := strings.Split(field.Key, "|")
		name, modifiers := parts[0], parts[1:]
		key := ".Result." + name
		optional := slices.Contains(optionalFields, field.Key) || slices.Contains(modifiers, "optional") || field.Value.Optional

		value, ok, err := handle(&field.Value, !optional)
		var result any
		if err == nil {
			if optional && (!ok || isBlank(value)) {
				var def string
				if def, err = e.applyTemplate(field.Value.Default, vars, nil); err == nil {
					if isBlank(def) {
						vars[key] = nil
						continue
					}
					value, ok = def, true
				}
			}
			if err == nil {
				var v *string
				if ok {
					v = &value
				}
				result, err = e.parseField(rel, name, v, modifiers, searchURL)
			}
		}
		if err == nil {
			vars[key] = result
			continue
		}
		if _, set := vars[key]; !set || optional {
			vars[key] = nil
		}
		if optional {
			continue
		}
		selector := ""
		if field.Value.Selector != nil {
			selector = *field.Value.Selector
		}
		if failRow {
			shown := "<null>"
			if ok {
				shown = value
			}
			return fmt.Errorf("error while parsing field=%s, selector=%s, value=%s: %w", field.Key, selector, shown, err)
		}
		e.log.Debug("cardigann: error while parsing field", "definition", e.def.ID, "field", field.Key, "selector", selector, "error", err)
	}
	return nil
}

// textFields are the fields taking text, which may be null.
var textFields = []string{"download", "title", "description", "category", "categorydesc", "infohash", "author", "booktitle", "publisher", "artist", "album", "label", "track", "poster", "imdb", "imdbid", "tmdbid", "rageid", "tvdbid", "tvmazeid", "traktid", "doubanid"}

// parseField sets a release's field from its value, nil for null, and
// returns the value for the field's variable, as Prowlarr's ParseFields
// does.
func (e *engine) parseField(rel *indexer.Release, name string, v *string, modifiers []string, searchURL string) (any, error) {
	if v == nil && !slices.Contains(textFields, name) {
		switch name {
		case "magnet", "details", "comments", "size", "leechers", "seeders", "date", "files", "grabs",
			"downloadvolumefactor", "uploadvolumefactor", "minimumratio", "minimumseedtime", "genre", "year":
			return nil, errors.New("value cannot be null")
		}
		return nil, nil
	}
	value := ""
	if v != nil {
		value = *v
	}
	switch name {
	case "download":
		if value == "" {
			rel.DownloadURL = ""
			return nil, nil
		}
		if strings.HasPrefix(value, "magnet:") {
			rel.MagnetURL = value
		} else {
			u, err := e.resolvePath(value, searchURL)
			if err != nil {
				return nil, err
			}
			rel.DownloadURL = u
			value = u
		}
		rel.GUID = value
	case "magnet":
		rel.MagnetURL = value
	case "infohash":
		rel.InfoHash = value
	case "details":
		u, err := e.resolvePath(value, searchURL)
		if err != nil {
			return nil, err
		}
		rel.InfoURL, value = u, u
	case "comments":
		u, err := e.resolvePath(value, searchURL)
		if err != nil {
			return nil, err
		}
		if rel.CommentsURL == "" {
			rel.CommentsURL = u
		}
		value = u
	case "title":
		if slices.Contains(modifiers, "append") {
			rel.Title += value
		} else {
			rel.Title = value
		}
		value = rel.Title
	case "description":
		if slices.Contains(modifiers, "append") {
			rel.Description += value
		} else {
			rel.Description = value
		}
		value = rel.Description
	case "category", "categorydesc":
		if slices.Contains(modifiers, "noappend") {
			e.log.Warn(`cardigann: the "noappend" modifier is deprecated, please switch to "default"`, "definition", e.def.ID)
		}
		mapped := e.categories.MapTracker(value)
		if name == "categorydesc" {
			mapped = e.categories.MapTrackerDesc(value)
		}
		if len(mapped) > 0 {
			if slices.Contains(modifiers, "noappend") {
				rel.Categories = nil
			}
			for _, c := range mapped {
				if !slices.Contains(rel.Categories, c.ID) {
					rel.Categories = append(rel.Categories, c.ID)
				}
			}
		}
		if !isBlank(value) && len(rel.Categories) == 0 {
			e.log.Warn("cardigann: invalid category", "definition", e.def.ID, "value", value)
		} else {
			// As .NET writes a list.
			value = "System.Collections.Generic.List`1[NzbDrone.Core.Indexers.IndexerCategory]"
		}
	case "size":
		n, err := parse.Bytes(value)
		if err != nil {
			return nil, err
		}
		rel.Size = &n
		value = strconv.FormatInt(n, 10)
	case "leechers":
		n, err := parse.Int(value)
		if err != nil {
			return nil, err
		}
		if n >= 5000000 {
			n = 0
		}
		peers := int(n)
		if rel.Peers != nil {
			peers += *rel.Peers
		}
		rel.Peers = &peers
		value = strconv.FormatInt(n, 10)
	case "seeders":
		n, err := coerceInt(value)
		if err != nil {
			return nil, err
		}
		if n >= 5000000 {
			n = 0
		}
		rel.Seeders = &n
		peers := n
		if rel.Peers != nil {
			peers += *rel.Peers
		}
		rel.Peers = &peers
		value = strconv.Itoa(n)
	case "date":
		t, err := parse.Unknown(e.now(), value, "")
		if err != nil {
			return nil, err
		}
		rel.PublishDate = t
		value = t.Format(dateLayout)
	case "files", "grabs":
		n, err := coerceInt(value)
		if err != nil {
			return nil, err
		}
		if name == "files" {
			rel.Files = &n
		} else {
			rel.Grabs = &n
		}
		value = strconv.Itoa(n)
	case "downloadvolumefactor", "uploadvolumefactor", "minimumratio":
		f, err := parse.Float(value)
		if err != nil {
			return nil, err
		}
		switch name {
		case "downloadvolumefactor":
			rel.DownloadVolumeFactor = &f
		case "uploadvolumefactor":
			rel.UploadVolumeFactor = &f
		default:
			rel.MinimumRatio = &f
		}
		value = strconv.FormatFloat(f, 'f', -1, 64)
	case "minimumseedtime":
		n, err := parse.Int(value)
		if err != nil {
			return nil, err
		}
		rel.MinimumSeedTime = &n
		value = strconv.FormatInt(n, 10)
	case "imdb", "imdbid", "tmdbid", "rageid", "tvdbid", "tvmazeid", "traktid", "doubanid":
		n, _ := parse.LeadingInt(value)
		id := int(int32(n))
		switch name {
		case "imdb", "imdbid":
			rel.IMDbID = id
		case "tmdbid":
			rel.TMDBID = id
		case "rageid":
			rel.TVRageID = id
		case "tvdbid":
			rel.TVDBID = id
		case "tvmazeid":
			rel.TVMazeID = id
		case "traktid":
			rel.TraktID = id
		case "doubanid":
			rel.DoubanID = id
		}
		value = strconv.Itoa(id)
	case "poster":
		if !isBlank(value) {
			u, err := e.resolvePath(value, searchURL)
			if err != nil {
				return nil, err
			}
			rel.PosterURL = u
		}
		value = rel.PosterURL
	case "genre":
		for _, g := range strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune(validateDelimiters, r) }) {
			if g = strings.TrimSpace(g); g != "" && !slices.Contains(rel.Genres, g) {
				rel.Genres = append(rel.Genres, g)
			}
		}
		for i, g := range rel.Genres {
			rel.Genres[i] = strings.ReplaceAll(g, "_", " ")
		}
		value = strings.Join(rel.Genres, ", ")
	case "year":
		n, err := coerceInt(value)
		if err != nil {
			return nil, err
		}
		rel.Year = n
		value = strconv.Itoa(n)
	case "author":
		rel.Author = value
	case "booktitle":
		rel.BookTitle = value
	case "publisher":
		rel.Publisher = value
	case "artist":
		rel.Artist = value
	case "album":
		rel.Album = value
	case "label":
		rel.Label = value
	case "track":
		rel.Track = value
	}
	if v == nil && value == "" {
		return nil, nil
	}
	return value, nil
}

// coerceInt reads a 32-bit integer as Prowlarr's CoerceInt does.
func coerceInt(s string) (int, error) {
	n, err := parse.Int(s)
	if err != nil {
		return 0, err
	}
	if n > math.MaxInt32 || n < math.MinInt32 {
		return 0, fmt.Errorf("value %q was either too large or too small for an Int32", s)
	}
	return int(n), nil
}

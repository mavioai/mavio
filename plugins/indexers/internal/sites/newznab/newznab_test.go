package newznab

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

// transport answers caps requests with caps and others with feed,
// counting the caps requests.
type transport struct {
	caps, feed []byte
	capsErr    error
	capsCalls  *atomic.Int32
	urls       *[]string
}

func (tr transport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := tr.feed
	if r.URL.Query().Get("t") == "caps" {
		if tr.capsCalls != nil {
			tr.capsCalls.Add(1)
		}
		if tr.capsErr != nil {
			return nil, tr.capsErr
		}
		body = tr.caps
	} else if tr.urls != nil {
		*tr.urls = append(*tr.urls, r.URL.String())
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/rss+xml"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    r,
	}, nil
}

func newIndexer(t *testing.T, site indexer.Site, tr http.RoundTripper) *indexer.Indexer {
	t.Helper()
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = tr
	s := c.Session(nil, nil)
	return &indexer.Indexer{ID: "5", Name: "Newznab", Site: site, Session: s, Statuses: st}
}

func newSite(t *testing.T, s Settings) *Site {
	t.Helper()
	site, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return site
}

// testCaps are the capabilities NewznabFixture gives its indexer.
func testCaps() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Categories.Add("2000", indexer.Movies, "Movies")
	c.Categories.Add("5000", indexer.TV, "TV")
	return c
}

// withCaps gives a site capabilities, as Prowlarr's tests mock them.
func withCaps(site *Site, caps *indexer.Capabilities) {
	site.caps.Set(caps, time.Now())
}

func TestNewznabCases(t *testing.T) {
	ported.Run(t, "newznab_tests/newznab.json", ported.Cases{Facts: map[string]string{
		"should_parse_recent_feed_from_newznab_nzb_su": "TestParseNzbSu",
		"should_use_pagesize_reported_by_caps":         "TestPageSizeFromCaps",
	}})
}

func TestParseNzbSu(t *testing.T) {
	site := newSite(t, Settings{BaseURL: "http://indexer.local/"})
	withCaps(site, testCaps())
	ix := newIndexer(t, site, transport{feed: ported.Response(t, "Newznab/newznab_nzb_su.xml")})
	releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie, Categories: []int{2000}, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 100 {
		t.Fatalf("got = %d releases, want = 100", len(releases))
	}
	r := releases[0]
	checks := []struct{ name, got, want string }{
		{"title", r.Title, "White.Collar.S03E05.720p.HDTV.X264-DIMENSION"},
		{"download", r.DownloadURL, "http://nzb.su/getnzb/24967ef4c2e26296c65d3bbfa97aa8fe.nzb&i=37292&r=xxx"},
		{"info", r.InfoURL, "http://nzb.su/details/24967ef4c2e26296c65d3bbfa97aa8fe"},
		{"comments", r.CommentsURL, "http://nzb.su/details/24967ef4c2e26296c65d3bbfa97aa8fe#comments"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got = %q, want = %q", c.name, c.got, c.want)
		}
	}
	if r.Protocol != indexer.ProtocolUsenet {
		t.Errorf("protocol: got = %v, want = usenet", r.Protocol)
	}
	if want := time.Date(2012, 2, 27, 16, 9, 39, 0, time.UTC); !r.PublishDate.Equal(want) {
		t.Errorf("publish date: got = %v, want = %v", r.PublishDate, want)
	}
	if r.Size == nil || *r.Size != 1183105773 {
		t.Errorf("size: got = %v, want = 1183105773", r.Size)
	}
}

func TestPageSizeFromCaps(t *testing.T) {
	site := newSite(t, Settings{BaseURL: "http://indexer.local/"})
	if got := site.Info().PageSize; got != 100 {
		t.Errorf("before caps: got = %d, want = 100", got)
	}
	caps := testCaps()
	caps.LimitsMax, caps.LimitsDefault = 30, 25
	withCaps(site, caps)
	if got := site.Info().PageSize; got != 25 {
		t.Errorf("got = %d, want = 25", got)
	}
}

func getCaps(t *testing.T, caps string) (*indexer.Capabilities, error) {
	t.Helper()
	site := newSite(t, Settings{BaseURL: "http://indxer.local"})
	ix := newIndexer(t, site, transport{caps: []byte(caps)})
	return site.Capabilities(context.Background(), ix)
}

func mustCaps(t *testing.T, caps string) *indexer.Capabilities {
	t.Helper()
	c, err := getCaps(t, caps)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCapabilitiesProviderCases(t *testing.T) {
	ported.Run(t, "newznab_tests/newznab_capabilities_provider.json", ported.Cases{
		Run: map[string]func(*testing.T, ported.Args){
			"should_find_sub_categories_as_main_categories": func(t *testing.T, a ported.Args) {
				var cat int
				a.Decode(t, "category", &cat)
				caps := mustCaps(t, string(ported.Response(t, "Torznab/torznab_animetosho_caps.xml")))
				got := caps.Categories.MapTracker(strconv.Itoa(cat))
				if len(got) != 2 || got[0].ID != cat {
					t.Errorf("got = %v, want = 2 categories, the first %d", ids(got), cat)
				}
			},
		},
		Facts: map[string]string{
			"should_not_request_same_caps_twice":      "TestCapsRequestedOnce",
			"should_report_pagesize":                  "TestCaps",
			"should_map_different_categories":         "TestCaps",
			"should_map_by_name_when_available":       "TestCaps",
			"should_use_default_pagesize_if_missing":  "TestCaps",
			"should_throw_if_failed_to_get":           "TestCapsErrors",
			"should_throw_if_xml_invalid":             "TestCapsErrors",
			"should_not_throw_on_xml_data_unexpected": "TestCapsErrors",
		},
	})
}

func ids(cs []*indexer.Category) []int {
	out := make([]int, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func TestCapsRequestedOnce(t *testing.T) {
	var calls atomic.Int32
	site := newSite(t, Settings{BaseURL: "http://indxer.local"})
	ix := newIndexer(t, site, transport{caps: ported.Response(t, "Newznab/newznab_caps.xml"), capsCalls: &calls})
	for range 2 {
		if _, err := site.Capabilities(context.Background(), ix); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("got = %d caps requests, want = 1", got)
	}
}

func TestCapsKeptAcrossRestarts(t *testing.T) {
	file := filepath.Join(t.TempDir(), "status.json")
	var calls atomic.Int32
	tr := transport{caps: ported.Response(t, "Newznab/newznab_caps.xml"), capsCalls: &calls}
	start := func(s Settings) (*Site, *indexer.Indexer) {
		t.Helper()
		st, err := indexer.NewStatuses(file, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		site := newSite(t, s)
		ix := newIndexer(t, site, tr)
		ix.Statuses = st
		return site, ix
	}
	settings := Settings{BaseURL: "http://indxer.local", APIKey: "abc"}
	for i, tt := range []struct {
		name     string
		settings Settings
		calls    int32
	}{
		{name: "first start", settings: settings, calls: 1},
		{name: "restart", settings: settings, calls: 1},
		{name: "other API key", settings: Settings{BaseURL: "http://indxer.local", APIKey: "def"}, calls: 2},
	} {
		site, ix := start(tt.settings)
		caps, err := site.Capabilities(context.Background(), ix)
		if err != nil {
			t.Fatalf("%d %s: %v", i, tt.name, err)
		}
		if caps.LimitsMax != 60 || len(caps.Params[indexer.SearchTV]) == 0 {
			t.Errorf("%s: caps = %+v, want = those of newznab_caps.xml", tt.name, caps)
		}
		if got := calls.Load(); got != tt.calls {
			t.Errorf("%s: caps requests = %d, want = %d", tt.name, got, tt.calls)
		}
	}
}

func TestCaps(t *testing.T) {
	xml := string(ported.Response(t, "Newznab/newznab_caps.xml"))
	caps := mustCaps(t, xml)
	if caps.LimitsDefault != 25 || caps.LimitsMax != 60 {
		t.Errorf("limits: got = %d/%d, want = 25/60", caps.LimitsDefault, caps.LimitsMax)
	}
	books := caps.Categories.MapToTracker([]int{indexer.Books.ID}, false)
	if len(books) != 2 || !slices.Contains(books, "8000") {
		t.Errorf("books: got = %v, want = 2 with 8000", books)
	}
	if got := caps.Categories.MapTracker("5999"); len(got) != 2 || got[0].ID != indexer.TVOther.ID {
		t.Errorf("5999: got = %v, want = 2, the first %d", ids(got), indexer.TVOther.ID)
	}
	for _, typ := range []indexer.SearchType{indexer.SearchBasic, indexer.SearchTV, indexer.SearchMovie, indexer.SearchMusic} {
		if !caps.Has(typ, indexer.ParamQ) {
			t.Errorf("%s search: got = %v, want = q", typ, caps.Params[typ])
		}
	}
	if caps.Available(indexer.SearchBook) {
		t.Errorf("book search: got = %v, want = none", caps.Params[indexer.SearchBook])
	}

	caps = mustCaps(t, strings.ReplaceAll(xml, "<limits", "<abclimits"))
	if caps.LimitsDefault != 100 || caps.LimitsMax != 100 {
		t.Errorf("default limits: got = %d/%d, want = 100/100", caps.LimitsDefault, caps.LimitsMax)
	}
}

func TestCapsErrors(t *testing.T) {
	xml := string(ported.Response(t, "Newznab/newznab_caps.xml"))
	site := newSite(t, Settings{BaseURL: "http://indxer.local"})
	ix := newIndexer(t, site, transport{capsErr: errors.New("boom")})
	if _, err := site.Capabilities(context.Background(), ix); err == nil {
		t.Error("failed to get: got = nil, want = error")
	}
	if _, err := getCaps(t, strings.ReplaceAll(xml, "<limits", "<>")); err == nil {
		t.Error("invalid XML: got = nil, want = error")
	}
	if c, err := getCaps(t, strings.ReplaceAll(xml, "5030", "asdf")); err != nil || c == nil {
		t.Errorf("unexpected data: got = %v, %v, want = capabilities", c, err)
	}
	if _, err := getCaps(t, string(ported.Response(t, "Newznab/unauthorized.xml"))); !errors.Is(err, indexer.ErrAuth) {
		t.Errorf("unauthorized: got = %v, want = %v", err, indexer.ErrAuth)
	}
}

func TestCapsSearchParams(t *testing.T) {
	caps := mustCaps(t, `<caps><searching>
<search available="yes" supportedParams="q,q,bogus" searchEngine="raw"/>
<tv-search available="yes" supportedParams="q, season,ep,tvdbId"/>
<movie-search available="no" supportedParams="q,imdbid"/>
<book-search available="yes"/>
</searching></caps>`)
	tests := []struct {
		typ  indexer.SearchType
		want []indexer.Param
	}{
		{indexer.SearchBasic, []indexer.Param{indexer.ParamQ}},
		{indexer.SearchTV, []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamTVDBID}},
		{indexer.SearchMovie, nil},
		{indexer.SearchMusic, nil},
		{indexer.SearchBook, []indexer.Param{indexer.ParamQ}},
	}
	for _, tt := range tests {
		if got := caps.Params[tt.typ]; !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %v, want = %v", tt.typ, got, tt.want)
		}
	}
	if !caps.RawSearch {
		t.Error("raw search: got = false, want = true")
	}
}

func generatorCaps() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Categories.Add("2000", indexer.Movies, "")
	c.Categories.Add("5000", indexer.TV, "")
	return c
}

func requestURL(t *testing.T, caps *indexer.Capabilities, q *indexer.Query) *url.URL {
	t.Helper()
	s := Settings{BaseURL: "http://127.0.0.1:1234/", APIKey: "abcd"}
	c := SearchRequests(s, caps, q)
	if len(c.Tiers()) != 1 || len(c.Tiers()[0]) != 1 {
		t.Fatalf("got = %d tiers, want = 1 of one search", len(c.Tiers()))
	}
	n := 0
	var first *indexer.Request
	for r := range c.Tiers()[0][0] {
		if first == nil {
			first = r
		}
		n++
	}
	if n != 1 {
		t.Errorf("pages: got = %d, want = 1", n)
	}
	u, err := url.Parse(first.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestRequestGeneratorCases(t *testing.T) {
	ported.Run(t, "newznab_tests/newznab_request_generator.json", ported.Cases{Facts: map[string]string{
		"should_return_subsequent_pages":                           "TestRequestGenerator",
		"should_not_get_unlimited_pages":                           "TestRequestGenerator",
		"should_not_search_by_imdbid_if_not_supported":             "TestRequestGenerator",
		"should_search_by_imdbid_if_supported":                     "TestRequestGenerator",
		"should_search_by_tmdbid_if_supported":                     "TestRequestGenerator",
		"should_prefer_search_by_tmdbid_if_rid_supported":          "TestRequestGenerator",
		"should_use_aggregrated_id_search_if_supported":            "TestRequestGenerator",
		"should_not_use_aggregrated_id_search_if_no_ids_supported": "TestRequestGenerator",
		"should_not_use_aggregrated_id_search_if_no_ids_are_known": "TestRequestGenerator",
		"should_fallback_to_q":                                     "TestRequestGenerator",
		"should_pad_seasons_for_tv_search":                         "TestRequestGenerator",
	}})
}

func TestRequestGenerator(t *testing.T) {
	q, imdb, tmdb := indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamTMDBID
	tests := []struct {
		name        string
		typ         indexer.SearchType
		params      []indexer.Param
		set         func(q *indexer.Query)
		has, hasNot []string
	}{
		{name: "pages", typ: indexer.SearchMovie, has: []string{"t=search", "offset=0", "cat=2000", "apikey=abcd", "extended=1", "q=Star+Wars"}},
		{name: "imdbid not supported", typ: indexer.SearchMovie, params: []indexer.Param{q}, set: func(q *indexer.Query) { q.IMDbID = "0076759" }, has: []string{"q=Star"}, hasNot: []string{"imdbid=0076759"}},
		{name: "imdbid", typ: indexer.SearchMovie, params: []indexer.Param{q, imdb}, set: func(q *indexer.Query) { q.IMDbID = "0076759" }, has: []string{"t=movie", "imdbid=0076759"}},
		{name: "tmdbid", typ: indexer.SearchMovie, params: []indexer.Param{q, tmdb}, set: func(q *indexer.Query) { q.TMDBID = 11 }, has: []string{"tmdbid=11"}},
		{name: "tmdbid preferred", typ: indexer.SearchMovie, params: []indexer.Param{q, imdb, tmdb}, set: func(q *indexer.Query) { q.TMDBID = 11 }, has: []string{"tmdbid=11"}, hasNot: []string{"imdbid="}},
		{name: "aggregated ids", typ: indexer.SearchMovie, params: []indexer.Param{q, imdb, tmdb}, set: func(q *indexer.Query) { q.IMDbID, q.TMDBID = "0076759", 11 }, has: []string{"tmdbid=11", "imdbid=0076759"}},
		{name: "no ids supported", typ: indexer.SearchMovie, params: []indexer.Param{q}, has: []string{"q="}},
		{name: "no ids known", typ: indexer.SearchMovie, params: []indexer.Param{q, imdb}, has: []string{"q="}},
		{name: "fallback to q", typ: indexer.SearchMovie, params: []indexer.Param{q, imdb, tmdb}, has: []string{"q=", "t=search"}, hasNot: []string{"tmdbid=11", "imdbid=0076759"}},
		{name: "season padded", typ: indexer.SearchTV, params: []indexer.Param{q, indexer.ParamSeason, indexer.ParamEp}, set: func(q *indexer.Query) { q.Season = indexer.Ptr(0) }, has: []string{"t=tvsearch", "season=00", "cat=5000"}},
		{name: "plus", typ: indexer.SearchBasic, set: func(q *indexer.Query) { q.Term = "C++" }, has: []string{"q=C%2520%2520"}},
		{
			name: "limits", typ: indexer.SearchBasic,
			set: func(q *indexer.Query) {
				q.MinAge, q.MaxAge, q.MinSize, q.MaxSize = indexer.Ptr(1), indexer.Ptr(30), indexer.Ptr[int64](1000), indexer.Ptr[int64](5000)
			},
			has: []string{"minage=1", "maxage=30", "minsize=1000", "maxsize=5000"},
		},
		{name: "no limits", typ: indexer.SearchBasic, hasNot: []string{"minage=", "maxage=", "minsize=", "maxsize="}},
		{
			name: "title", typ: indexer.SearchTV, params: []indexer.Param{q, indexer.ParamSeason, indexer.ParamEp, indexer.ParamTitle},
			set: func(q *indexer.Query) {
				q.Term, q.Title, q.Season, q.Episode = "", "Marvel's Agents", indexer.Ptr(1), "2"
			},
			has: []string{"t=tvsearch", "title=Marvel%27s+Agents", "season=1", "ep=2"}, hasNot: []string{"q="},
		},
		{
			name: "title not taken", typ: indexer.SearchTV, params: []indexer.Param{q, indexer.ParamSeason, indexer.ParamEp},
			set: func(q *indexer.Query) { q.Title, q.Season = "Marvel's Agents", indexer.Ptr(1) }, hasNot: []string{"title="},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := generatorCaps()
			if tt.params != nil {
				c.Params[tt.typ] = tt.params
			}
			query := &indexer.Query{Type: tt.typ, Term: "Star Wars", Categories: []int{2000}}
			if tt.typ == indexer.SearchTV {
				query.Categories = []int{5000}
			}
			if tt.set != nil {
				tt.set(query)
			}
			u := requestURL(t, c, query).String()
			for _, want := range tt.has {
				if !strings.Contains(u, want) {
					t.Errorf("got = %s, want = containing %s", u, want)
				}
			}
			for _, not := range tt.hasNot {
				if strings.Contains(u, not) {
					t.Errorf("got = %s, want = without %s", u, not)
				}
			}
		})
	}
}

func TestSettingCases(t *testing.T) {
	fieldsOf := func(err error) []string {
		var out []string
		var errs []error
		if j, ok := err.(interface{ Unwrap() []error }); ok {
			errs = j.Unwrap()
		}
		for _, e := range errs {
			var fe *FieldError
			if errors.As(e, &fe) {
				out = append(out, fe.Field)
			}
		}
		return out
	}
	validate := func(t *testing.T, a ported.Args) []string {
		u, _ := a.Str(t, "url")
		err := Settings{BaseURL: u}.Validate()
		if err == nil {
			return nil
		}
		return append(fieldsOf(err), "invalid")
	}
	ported.Run(t, "newznab_tests/newznab_setting.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"requires_apikey": func(t *testing.T, a ported.Args) {
			if got := validate(t, a); !slices.Contains(got, "api_key") {
				t.Errorf("got = %v, want = api_key", got)
			}
		},
		"invalid_url_should_not_apikey": func(t *testing.T, a ported.Args) {
			if got := validate(t, a); slices.Contains(got, "api_key") || !slices.Contains(got, "base_url") {
				t.Errorf("got = %v, want = base_url only", got)
			}
		},
		"doesnt_requires_apikey": func(t *testing.T, a ported.Args) {
			if got := validate(t, a); got != nil {
				t.Errorf("got = %v, want = valid", got)
			}
		},
	}})
}

func TestCheckError(t *testing.T) {
	tests := []struct {
		name, body, url string
		check           func(error) bool
	}{
		{"auth", `<error code="101" description="Account suspended"/>`, "http://x/api?apikey=1", func(err error) bool { return errors.Is(err, indexer.ErrAuth) }},
		{"needs key", `<error code="200" description="Missing parameter"/>`, "http://x/api", func(err error) bool { return errors.Is(err, indexer.ErrAuth) }},
		{"limit", `<error code="500" description="Request limit reached"/>`, "http://x/api?apikey=1", func(err error) bool {
			var rl *indexer.RateLimitError
			return errors.As(err, &rl)
		}},
		{"other", `<error code="300" description="No such function"/>`, "http://x/api?apikey=1", func(err error) bool {
			var e *indexer.Error
			return errors.As(err, &e) && strings.Contains(e.Message, "No such function")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := indexer.LoadXML(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			r := &indexer.Response{Request: indexer.NewRequest(tt.url, indexer.AcceptRSS)}
			if err := CheckError(doc, r); !tt.check(err) {
				t.Errorf("got = %v", err)
			}
		})
	}
}

func TestParseRejectsTorrents(t *testing.T) {
	body := `<rss><channel><item><title>A</title><pubDate>Mon, 27 Feb 2012 11:09:39 -0500</pubDate>
<enclosure url="http://x/a.torrent" length="1" type="application/x-bittorrent"/></item></channel></rss>`
	r := &indexer.Response{Request: indexer.NewRequest("http://x/api", indexer.AcceptRSS), URL: "http://x/api", StatusCode: http.StatusOK, Header: http.Header{}, Body: []byte(body)}
	releases, err := NewParser(testCaps()).Parse(context.Background(), r)
	if err != nil || len(releases) != 0 {
		t.Errorf("got = %d releases, %v, want = none", len(releases), err)
	}
}

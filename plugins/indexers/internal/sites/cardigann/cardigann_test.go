package cardigann

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

func mustParse(t *testing.T, yml string) *Definition {
	t.Helper()
	def, err := Parse([]byte(yml))
	if err != nil {
		t.Fatal(err)
	}
	return def
}

const templateDefinition = `
id: somesite
name: Some Site
encoding: UTF-8
links: ["https://somesite.com/"]
caps:
  modes:
    search: [q]
search:
  path: /
  rows: {selector: tr}
  fields: {}
`

func TestApplyGoTemplateText(t *testing.T) {
	e, err := newEngine(mustParse(t, templateDefinition), Settings{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	variables := func() Vars {
		return Vars{
			".Config.sitelink": "https://somesite.com/",
			".True":            "True",
			".False":           nil,
			".Today.Year":      strconv.Itoa(time.Now().Year()),
			".Categories":      []string{"tv", "movies"},
		}
	}
	check := func(t *testing.T, vars Vars, a ported.Args) {
		template, _ := a.Str(t, "template")
		want, _ := a.Str(t, "expected")
		got, err := e.applyTemplate(template, vars, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("applyTemplate(%q) = %q, want = %q", template, got, want)
		}
	}
	plain := func(t *testing.T, a ported.Args) { check(t, variables(), a) }
	ported.Run(t, "cardigann_tests/apply_go_template_text.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"should_handle_range_statements": plain,
		"should_handle_join_statements":  plain,
		"should_handle_if_statements":    plain,
		"should_handle_re_replace_statements": func(t *testing.T, a ported.Args) {
			vars := variables()
			vars[".Query.Keywords"] = strings.Join([]string{"abc", "def"}, " ")
			check(t, vars, a)
		},
		"should_handle_variables_statements": func(t *testing.T, a ported.Args) {
			template, _ := a.Str(t, "template")
			got, err := e.applyTemplate(template, variables(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := strconv.Itoa(time.Now().Year()); got != want {
				t.Errorf("applyTemplate(%q) = %q, want = %q", template, got, want)
			}
		},
	}})
}

func TestTemplateLogic(t *testing.T) {
	e, err := newEngine(mustParse(t, templateDefinition), Settings{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	vars := Vars{".A": "a", ".B": nil, ".C": "c", ".Empty": "", ".True": "True", ".False": nil}
	for _, tt := range []struct{ template, want string }{
		{`{{ if and .A .C }}yes{{ else }}no{{ end }}`, "yes"},
		{`{{ if and .A .B }}yes{{ else }}no{{ end }}`, "no"},
		{`{{ if or .B .C }}yes{{ else }}no{{ end }}`, "yes"},
		{`{{ if eq .A "a" }}yes{{ else }}no{{ end }}`, "yes"},
		{`{{ if ne .A .C }}yes{{ else }}no{{ end }}`, "yes"},
		{`{{ if .Empty }}yes{{ else }}no{{ end }}`, "no"},
		{`x{{ .A }}{{ .C }}x`, "xacx"},
	} {
		got, err := e.applyTemplate(tt.template, vars, nil)
		if err != nil {
			t.Fatalf("applyTemplate(%q): %v", tt.template, err)
		}
		if got != tt.want {
			t.Errorf("applyTemplate(%q) = %q, want = %q", tt.template, got, tt.want)
		}
	}
	if _, err := e.applyTemplate("{{ .Missing }}", vars, nil); err == nil {
		t.Errorf("applyTemplate of a missing variable: got = nil, want = an error")
	}
}

func TestFilters(t *testing.T) {
	e, err := newEngine(mustParse(t, templateDefinition), Settings{}, func() time.Time {
		return time.Date(2024, 5, 6, 12, 0, 0, 0, time.UTC)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := func(name string, args ...string) Filter { return Filter{Name: name, Args: args} }
	for _, tt := range []struct {
		data    string
		filters []Filter
		want    string
	}{
		{"details.php?id=123&x=1", []Filter{f("querystring", "id")}, "123"},
		{"Size: 1.5 GB", []Filter{f("regexp", `([\d\.]+ \w+)`)}, "1.5 GB"},
		{"a-b-c", []Filter{{Name: "replace", Args: []string{"-", "+"}, List: true}}, "a+b+c"},
		{"a|b|c", []Filter{{Name: "split", Args: []string{"|", "1"}, List: true}}, "b"},
		{"  x  ", []Filter{f("trim")}, "x"},
		{"x", []Filter{f("prepend", "<"), f("append", ">")}, "<x>"},
		{"MiXed", []Filter{f("tolower")}, "mixed"},
		{"MiXed", []Filter{f("toupper")}, "MIXED"},
		{"a%20b", []Filter{f("urldecode")}, "a b"},
		{"a b&c", []Filter{f("urlencode")}, "a+b%26c"},
		{"&lt;b&gt;", []Filter{f("htmldecode")}, "<b>"},
		{"Crème brûlée", []Filter{f("diacritics", "replace")}, "Creme brulee"},
		{"abc123", []Filter{{Name: "re_replace", Args: []string{`\d`, "#"}, List: true}}, "abc###"},
		{"2024-01-02", []Filter{f("dateparse", "yyyy-MM-dd")}, "Tue, 02 Jan 2024 00:00:00 +0000"},
		{"a b/c", []Filter{f("validfilename")}, "a b_c"},
		{"Hello World", []Filter{f("validate", "world, foo")}, "world"},
	} {
		got, err := e.applyFilters(tt.data, tt.filters, Vars{})
		if err != nil {
			t.Fatalf("applyFilters(%q, %v): %v", tt.data, tt.filters, err)
		}
		if got != tt.want {
			t.Errorf("applyFilters(%q, %v) = %q, want = %q", tt.data, tt.filters, got, tt.want)
		}
	}
}

func TestSelectToken(t *testing.T) {
	root, err := parseJSON(`{"data":{"items":[{"id":1,"name":"a","tags":["x","y"]},{"id":2.5,"name":null}]},"ok":true,"when":"2024-01-02T03:04:05Z"}`, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		path  string
		want  string
		found bool
	}{
		{"data.items[0].name", "a", true},
		{"data.items[1].id", "2.5", true},
		{"data.items[-1].id", "2.5", true},
		{"$.ok", "True", true},
		{"when", "01/02/2024 03:04:05", true},
		{"data.items[0].missing", "", false},
		{"data['items'][0].id", "1", true},
	} {
		v, found, err := selectToken(root, tt.path)
		if err != nil {
			t.Fatalf("selectToken(%q): %v", tt.path, err)
		}
		if found != tt.found {
			t.Fatalf("selectToken(%q) found = %v, want = %v", tt.path, found, tt.found)
		}
		if !found {
			continue
		}
		got, _, err := jsonString(v)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("selectToken(%q) = %q, want = %q", tt.path, got, tt.want)
		}
	}
	if _, _, err := selectToken(root, "data.items[*].id"); err == nil {
		t.Errorf("selectToken of several values: got = nil, want = an error")
	}
}

// siteServer answers requests with its handler, recording them.
type siteServer struct {
	handler  http.HandlerFunc
	mu       sync.Mutex
	requests []string
}

func (s *siteServer) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, r.Method+" "+r.URL.String())
	s.mu.Unlock()
	if r.Body != nil {
		defer r.Body.Close()
	}
	rec := httptest.NewRecorder()
	s.handler(rec, r)
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func newTestIndexer(t *testing.T, def *Definition, s Settings, h http.HandlerFunc) (*indexer.Indexer, *siteServer) {
	t.Helper()
	site, err := New(def, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	srv := &siteServer{handler: h}
	c := indexer.NewClient()
	c.Transport = srv
	return &indexer.Indexer{ID: "1", Name: def.Name, Site: site, Session: c.Session(nil, nil), Statuses: st}, srv
}

const htmlDefinition = `
id: testsite
name: Test Site
type: private
links: ["https://test.site/"]
caps:
  categorymappings:
    - {id: 1, cat: Movies, desc: "Movies"}
    - {id: 2, cat: TV/HD, desc: "TV HD"}
  modes:
    search: [q]
    tv-search: [q, season, ep]
settings:
  - {name: username, type: text, label: Username}
  - {name: password, type: password, label: Password}
  - {name: freeleech, type: checkbox, label: Freeleech, default: false}
login:
  path: login.php
  method: form
  form: form#login
  inputs:
    username: "{{ .Config.username }}"
    password: "{{ .Config.password }}"
  error:
    - selector: div.error
  test:
    path: index.php
    selector: a[href="logout.php"]
search:
  paths:
    - path: browse.php
  inputs:
    search: "{{ .Keywords }}"
    $raw: "{{ range .Categories }}c{{.}}=1&{{end}}"
    free: "{{ if .Config.freeleech }}1{{ else }}{{ end }}"
  rows:
    selector: table.torrents > tbody > tr:has(a.title)
  fields:
    category:
      selector: a[href*="cat="]
      attribute: href
      filters:
        - name: querystring
          args: cat
    title:
      selector: a.title
    details:
      selector: a.title
      attribute: href
    download:
      selector: a[href^="download.php"]
      attribute: href
    size:
      selector: td.size
    seeders:
      selector: td.seeders
    leechers:
      selector: td.leechers
    date:
      selector: td.date
      filters:
        - name: dateparse
          args: "yyyy-MM-dd HH:mm"
    downloadvolumefactor:
      case:
        img.free: 0
        "*": 1
    uploadvolumefactor:
      text: 1
    description:
      selector: span.tags
      optional: true
download:
  selectors:
    - selector: a.dl
      attribute: href
`

const browsePage = `<html><body><a href="logout.php">Logout</a>
<table class="torrents"><tbody>
<tr><th>Header</th></tr>
<tr>
 <td><a href="browse.php?cat=1">Movies</a></td>
 <td><a class="title" href="details.php?id=10">Movie.2024.1080p</a><img class="free"><span class="tags">Internal release</span></td>
 <td><a href="download.php?id=10">DL</a></td>
 <td class="size">1.5 GB</td><td class="seeders">10</td><td class="leechers">2</td>
 <td class="date">2024-01-02 03:04</td>
</tr>
<tr>
 <td><a href="browse.php?cat=2">TV</a></td>
 <td><a class="title" href="details.php?id=11">Show.S01E02.720p</a></td>
 <td><a href="download.php?id=11">DL</a></td>
 <td class="size">700 MB</td><td class="seeders">1,234</td><td class="leechers">0</td>
 <td class="date">2024-01-03 04:05</td>
</tr>
</tbody></table></body></html>`

func TestSearchHTML(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		loggedIn := false
		ix, srv := newTestIndexer(t, mustParse(t, htmlDefinition), Settings{Fields: map[string]any{"username": "user", "password": "pass", "freeleech": true}}, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login.php":
				if r.Method == http.MethodGet {
					http.SetCookie(w, &http.Cookie{Name: "session", Value: "landing"})
					_, _ = w.Write([]byte(`<html><body><form id="login" action="takelogin.php" method="post">
<input type="hidden" name="token" value="t0k">
<input type="checkbox" name="remember" value="1">
<input type="text" name="username"><input type="password" name="password">
<input type="submit" name="go" value="Go" disabled>
</form></body></html>`))
					return
				}
			case "/takelogin.php":
				_ = r.ParseForm()
				if r.PostForm.Get("token") != "t0k" || r.PostForm.Get("username") != "user" || r.PostForm.Get("password") != "pass" || r.PostForm.Has("remember") || r.PostForm.Has("go") {
					_, _ = w.Write([]byte(`<div class="error"> Bad form </div>`))
					return
				}
				if c, err := r.Cookie("session"); err != nil || c.Value != "landing" {
					_, _ = w.Write([]byte(`<div class="error">No session</div>`))
					return
				}
				loggedIn = true
				http.SetCookie(w, &http.Cookie{Name: "uid", Value: "42"})
				_, _ = w.Write([]byte(`<a href="logout.php">Logout</a>`))
				return
			case "/browse.php":
				if c, err := r.Cookie("uid"); !loggedIn || err != nil || c.Value != "42" {
					http.Redirect(w, r, "/login.php", http.StatusFound)
					return
				}
				_, _ = w.Write([]byte(browsePage))
				return
			}
			http.NotFound(w, r)
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchBasic, Term: "some movie", Categories: []int{2000}})
		if err != nil {
			t.Fatal(err)
		}
		wantRequests := []string{
			"GET https://test.site/browse.php?search=some+movie&c1=1&free=1",
			"GET https://test.site/login.php",
			"POST https://test.site/takelogin.php",
			"GET https://test.site/browse.php?search=some+movie&c1=1&free=1",
		}
		if !slices.Equal(srv.requests, wantRequests) {
			t.Errorf("requests = %q, want = %q", srv.requests, wantRequests)
		}
		if len(releases) != 2 {
			t.Fatalf("len(releases) = %d, want = 2", len(releases))
		}
		r := releases[0]
		for _, c := range []struct{ name, got, want string }{
			{"title", r.Title, "Movie.2024.1080p"},
			{"details", r.InfoURL, "https://test.site/details.php?id=10"},
			{"download", r.DownloadURL, "https://test.site/download.php?id=10"},
			{"guid", r.GUID, "https://test.site/download.php?id=10"},
			{"date", r.PublishDate.In(time.Local).Format(time.DateTime), "2024-01-02 03:04:00"},
			{"description", r.Description, "Internal release"},
		} {
			if c.got != c.want {
				t.Errorf("%s = %q, want = %q", c.name, c.got, c.want)
			}
		}
		if r.Size == nil || *r.Size != 1610612736 {
			t.Errorf("size = %v, want = 1610612736", r.Size)
		}
		if r.Seeders == nil || *r.Seeders != 10 || r.Peers == nil || *r.Peers != 12 {
			t.Errorf("seeders, peers = %v, %v, want = 10, 12", r.Seeders, r.Peers)
		}
		if r.DownloadVolumeFactor == nil || *r.DownloadVolumeFactor != 0 {
			t.Errorf("downloadvolumefactor = %v, want = 0", r.DownloadVolumeFactor)
		}
		if !slices.Contains(r.Categories, 2000) {
			t.Errorf("categories = %v, want = with 2000", r.Categories)
		}
		if !r.HasFlag(indexer.FlagInternal) {
			t.Errorf("flags = %v, want = with internal", r.Flags)
		}
		if s := releases[1].Seeders; s == nil || *s != 1234 {
			t.Errorf("seeders of the second = %v, want = 1234", s)
		}
	})
}

func TestLoginError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix, _ := newTestIndexer(t, mustParse(t, htmlDefinition), Settings{Fields: map[string]any{"username": "user", "password": "wrong"}}, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/login.php":
				_, _ = w.Write([]byte(`<form id="login" action="takelogin.php"><input name="username"><input name="password"></form>`))
			default:
				_, _ = w.Write([]byte(`<div class="error"> Wrong password </div>`))
			}
		})
		err := ix.Site.(*Site).Login(context.Background(), ix)
		if err == nil || err.Error() != "Error: Wrong password" {
			t.Errorf("Login() = %v, want = Error: Wrong password", err)
		}
	})
}

const jsonDefinition = `
id: jsonsite
name: JSON Site
type: public
links: ["https://json.site/"]
caps:
  categories:
    movies: Movies
  modes:
    search: [q]
    movie-search: [q, imdbid]
settings: []
search:
  paths:
    - path: "api/search/{{ .Keywords }}"
      response:
        type: json
  inputs:
    imdb: "{{ .Query.IMDBID }}"
  rows:
    selector: results
    count:
      selector: total
  fields:
    category:
      text: movies
    title:
      selector: name
    infohash:
      selector: hash
    details:
      selector: id
      filters:
        - name: prepend
          args: "torrent/"
    size:
      selector: size
    seeders:
      selector: stats.seeders
    leechers:
      selector: stats.leechers
    date:
      selector: added
    genre:
      selector: genres
      optional: true
    imdbid:
      selector: imdb
`

func TestSearchJSON(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix, srv := newTestIndexer(t, mustParse(t, jsonDefinition), Settings{}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"total": 1, "results": [{"id": 7, "name": "Movie 2024", "hash": "0123456789ABCDEF0123456789ABCDEF01234567", "size": 2048, "stats": {"seeders": 5, "leechers": 1}, "added": 1704164645, "genres": ["Action", "Sci_Fi"], "imdb": "tt0012345"}]}`))
		})
		releases, err := ix.Search(context.Background(), &indexer.Query{Type: indexer.SearchMovie, Term: "movie", IMDbID: "tt12345"})
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"GET https://json.site/api/search/movie?imdb=tt0012345"}; !slices.Equal(srv.requests, want) {
			t.Errorf("requests = %q, want = %q", srv.requests, want)
		}
		if len(releases) != 1 {
			t.Fatalf("len(releases) = %d, want = 1", len(releases))
		}
		r := releases[0]
		wantMagnet := "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=Movie+2024&tr=http%3a%2f%2ftracker.opentrackr.org%3a1337%2fannounce"
		for _, c := range []struct{ name, got, want string }{
			{"title", r.Title, "Movie 2024"},
			{"details", r.InfoURL, "https://json.site/api/search/torrent/7"},
			{"magnet", r.MagnetURL[:min(len(r.MagnetURL), len(wantMagnet))], wantMagnet},
			{"genres", strings.Join(r.Genres, "|"), "Action|Sci Fi"},
			{"date", r.PublishDate.UTC().Format(time.DateTime), "2024-01-02 03:04:05"},
		} {
			if c.got != c.want {
				t.Errorf("%s = %q, want = %q", c.name, c.got, c.want)
			}
		}
		if r.IMDbID != 12345 {
			t.Errorf("imdb = %d, want = 12345", r.IMDbID)
		}
		if r.Size == nil || *r.Size != 2048 {
			t.Errorf("size = %v, want = 2048", r.Size)
		}
	})
}

func TestDownloadSelectors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ix, _ := newTestIndexer(t, mustParse(t, htmlDefinition), Settings{}, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/details.php":
				_, _ = w.Write([]byte(`<a class="dl" href="dl/10.torrent">get</a>`))
			case "/dl/10.torrent":
				_, _ = w.Write([]byte("d8:announce0:e"))
			default:
				http.NotFound(w, r)
			}
		})
		r, err := ix.Site.(*Site).DownloadRequest(context.Background(), ix, "https://test.site/details.php?id=10")
		if err != nil {
			t.Fatal(err)
		}
		if want := "https://test.site/dl/10.torrent"; r.URL != want {
			t.Errorf("download URL = %q, want = %q", r.URL, want)
		}
	})
}

func TestCapabilities(t *testing.T) {
	site, err := New(mustParse(t, htmlDefinition), Settings{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	caps, _ := site.Capabilities(context.Background(), nil)
	if !caps.Has(indexer.SearchTV, indexer.ParamSeason) || caps.Available(indexer.SearchMovie) {
		t.Errorf("params = %v, want = tv with season, no movie search", caps.Params)
	}
	if info := site.Info(); info.Privacy != indexer.Private || info.PageSize != 1 {
		t.Errorf("info = %+v, want = private with page size 1", info)
	}
	bad := strings.Replace(htmlDefinition, "search: [q]", "search: [q, imdbid]", 1)
	if _, err := New(mustParse(t, bad), Settings{}, nil); err == nil {
		t.Errorf("New with search mode [q, imdbid]: got = nil, want = an error")
	}
}

func TestLoadDefinitions(t *testing.T) {
	def := func(id string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte(strings.NewReplacer("id: somesite", "id: "+id, "name: Some Site", "name: "+id).Replace(templateDefinition))}
	}
	fsys := fstest.MapFS{
		"a.yml":        def("a"),
		"hdbits.yml":   def("hdbits"),
		"broken.yml":   {Data: []byte("id: [")},
		"notes.txt":    {Data: []byte("x")},
		"Custom/a.yml": def("a"),
		"Custom/b.yml": def("b"),
	}
	defs, err := LoadDefinitions(fsys, nil)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, d := range defs {
		files = append(files, d.File)
	}
	if want := []string{"a", "b"}; !slices.Equal(files, want) {
		t.Errorf("files = %q, want = %q", files, want)
	}
}

func TestPublicMagnetLink(t *testing.T) {
	got, err := indexer.PublicMagnetLink("0123456789abcdef0123456789abcdef01234567", "A b&c")
	if err != nil {
		t.Fatal(err)
	}
	if want := "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=A+b%26c&tr="; !strings.HasPrefix(got, want) {
		t.Errorf("PublicMagnetLink() = %q, want = prefix %q", got, want)
	}
	if _, err := indexer.PublicMagnetLink("abc", "x"); err == nil {
		t.Errorf("PublicMagnetLink of a short hash: got = nil, want = an error")
	}
}

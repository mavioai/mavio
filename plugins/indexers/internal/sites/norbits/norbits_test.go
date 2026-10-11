package norbits

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

type handler func(r *http.Request) *http.Response

func (h handler) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := h(r)
	resp.Request = r
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp, nil
}

func newIndexer(t *testing.T, settings Settings, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	site, err := Definition.New(data)
	if err != nil {
		t.Fatal(err)
	}
	st, err := indexer.NewStatuses("", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	st.SetStarted(time.Now().Add(-time.Hour))
	c := indexer.NewClient()
	c.Transport = h
	return &indexer.Indexer{ID: "1", Name: "NorBits", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{UserPassSettings: indexer.UserPassSettings{Username: "user", Password: "pass"}}

func TestSearchRequests(t *testing.T) {
	full := testSettings
	full.UseFullSearch = true
	full.FreeleechOnly = true
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{
			"RSS", testSettings,
			indexer.Query{},
			"https://norbits.net/browse.php?search=&incldead=1&fullsearch=0&scenerelease=0",
		},
		{
			"term and categories", full,
			indexer.Query{Term: "Kongen's nei", Categories: []int{2000, 5000}},
			"https://norbits.net/browse.php?search=Kongen%27s+nei&incldead=1&fullsearch=1&scenerelease=0&FL=1&main_cat[]=1&main_cat[]=2",
		},
		{
			"TV episode", testSettings,
			indexer.Query{Type: indexer.SearchTV, Term: "Skam", Season: indexer.Ptr(1), Episode: "2"},
			"https://norbits.net/browse.php?search=Skam+S01E02&incldead=1&fullsearch=0&scenerelease=0",
		},
		{
			"movie by IMDb ID", testSettings,
			indexer.Query{Type: indexer.SearchMovie, Term: "Film", IMDbID: "76759", Categories: []int{2000}},
			"https://norbits.net/browse.php?imdbsearch=tt0076759&incldead=1&fullsearch=0&scenerelease=0&main_cat[]=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.First().URL; got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

// page is a search page as NorBits shows it.
const page = `<html><body><a href="/logout.php">Logg ut</a>
<table id="torrentTable">
<tr><td>Type</td><td>Navn</td><td>Filer</td><td>Komm.</td><td>Lagt til</td><td>TTL</td><td>Størrelse</td><td>Fullført</td><td>Seed</td><td>Leech</td></tr>
<tr>
<td><a href="/browse.php?main_cat[]=2&amp;sub2_cat[]=19"><img alt="TV"></a></td>
<td><a href="/details.php?id=1001&amp;hit=1" title=" Skam.S01E02.NORWEGIAN.1080p.WEB.h264-GRP ">Skam.S01E02...</a>
<a href="/download.php?id=1001&amp;passkey=x"><img alt="DL"></a>
<span class="genres">(Drama&nbsp;|&nbsp;Komedie)</span>
<a href="https://www.imdb.com/title/tt5174284/">IMDb</a>
<img title="Halfleech"></td>
<td><a href="/filelist.php?id=1001">3</a></td>
<td>0</td>
<td>2023-05-01<br>12:34:56</td>
<td>10<br>dager</td>
<td>1.37 GB</td>
<td>1,204<br>ganger</td>
<td>87</td>
<td>4</td>
</tr>
<tr>
<td><a href="/browse.php?main_cat[]=1"><img alt="Filmer"></a></td>
<td><a href="details.php?id=1002" title="Film.2020.NORWEGIAN.1080p.BluRay.x264-GRP">Film</a>
<a href="download.php?id=1002"><img alt="DL"></a>
<img title="100% freeleech"></td>
<td><a href="/filelist.php?id=1002">1</a></td>
<td>2</td>
<td>2023-04-30<br>01:02:03</td>
<td></td>
<td>8.5 GB</td>
<td>10<br>ganger</td>
<td>5</td>
<td>1</td>
</tr>
</table></body></html>`

func respond(body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(page))
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r, r2 := releases[0], releases[1]
	tv := indexer.IDs(testCaps.Categories.MapTracker("main_cat[]=2"))
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Skam.S01E02.NORWEGIAN.1080p.WEB.h264-GRP"},
		{"GUID", r.GUID, "https://norbits.net/details.php?id=1001&hit=1"},
		{"InfoURL", r.InfoURL, "https://norbits.net/details.php?id=1001&hit=1"},
		{"DownloadURL", r.DownloadURL, "https://norbits.net/download.php?id=1001&passkey=x"},
		{"Categories", r.Categories, tv},
		{"first category", r.Categories[0], 5000},
		{"Description", r.Description, "Drama,Komedie"},
		{"Genres", r.Genres, []string{"Drama", "Komedie"}},
		{"IMDbID", r.IMDbID, 5174284},
		{"Size", *r.Size, int64(1471026298)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 1204},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"PublishDate", r.PublishDate, time.Date(2023, 5, 1, 12, 34, 56, 0, time.Local)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.5},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
		{"second GUID", r2.GUID, "https://norbits.net/details.php?id=1002"},
		{"second first category", r2.Categories[0], 2000},
		{"second Description", r2.Description, ""},
		{"second Grabs", *r2.Grabs, 10},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 0.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

var testCaps = capabilities()

func equal(a, b any) bool {
	if x, ok := a.(time.Time); ok {
		return x.Equal(b.(time.Time))
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name    string
		cookies []string
		ok      bool
	}{
		{"success", []string{"uid=42; path=/", "pass=secret; path=/"}, true},
		{"failure", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { testLogin(t, tt.cookies, tt.ok) })
		})
	}
}

// testLogin searches with a login setting cookies, wanting success or an
// authentication failure.
func testLogin(t *testing.T, cookies []string, ok bool) {
	s := testSettings
	s.TwoFactorAuthCode = "123456"
	var steps []string
	var searched int
	ix := newIndexer(t, s, func(r *http.Request) *http.Response {
		steps = append(steps, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/":
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Set-Cookie": {"PHPSESSID=abc; path=/"}}, Body: io.NopCloser(strings.NewReader("home"))}
		case "/login.php":
			if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "abc" {
				t.Errorf("login page cookie PHPSESSID = %v, want = abc", c)
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("login"))}
		case "/takelogin.php":
			form, _ := io.ReadAll(r.Body)
			if got, want := string(form), "username=user&password=pass&code=123456&logout=no&returnto=%2F"; got != want {
				t.Errorf("login form = %q, want = %q", got, want)
			}
			if got, want := r.Header.Get("Referer"), "https://norbits.net/login.php"; got != want {
				t.Errorf("Referer = %q, want = %q", got, want)
			}
			if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "abc" {
				t.Errorf("takelogin cookie PHPSESSID = %v, want = abc", c)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Set-Cookie": cookies}, Body: io.NopCloser(strings.NewReader("ok"))}
		}
		searched++
		if searched == 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<html>Logg inn</html>"))}
		}
		for name, want := range map[string]string{"uid": "42", "PHPSESSID": "abc"} {
			if c, err := r.Cookie(name); err != nil || c.Value != want {
				t.Errorf("cookie %s = %v, want = %s", name, c, want)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if !ok {
		if err == nil || !strings.Contains(err.Error(), "Login failed") || !errors.Is(err, indexer.ErrAuth) {
			t.Errorf("err = %v, want = Login failed", err)
		}
		return
	}
	if err != nil || len(releases) != 2 {
		t.Errorf("Search() = %d releases, %v, want = 2 releases", len(releases), err)
	}
	want := "GET /browse.php,GET /,GET /login.php,POST /takelogin.php,GET /browse.php"
	if got := strings.Join(steps, ","); got != want {
		t.Errorf("requests = %s, want = %s", got, want)
	}
}

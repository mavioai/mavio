package bithdtv

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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
	return &indexer.Indexer{ID: "1", Name: "BitHDTV", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{Cookie: "h_sl=a; h_sp=b; h_su=c"}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want []string
	}{
		{
			"RSS",
			indexer.Query{},
			[]string{"https://www.bit-hdtv.com/torrents.php?cat=0&search=&options=0"},
		},
		{
			"term in one category",
			indexer.Query{Term: "Some Show", Categories: []int{6000}},
			[]string{"https://www.bit-hdtv.com/torrents.php?cat=11&search=Some+Show&options=0"},
		},
		{
			"subcategory mapped with its parent's",
			indexer.Query{Term: "x", Categories: []int{5060}},
			[]string{"https://www.bit-hdtv.com/torrents.php?cat=0&search=x&options=0"},
		},
		{
			"TV episode",
			indexer.Query{Type: indexer.SearchTV, Term: "The Show", Season: indexer.Ptr(1), Episode: "2"},
			[]string{"https://www.bit-hdtv.com/torrents.php?cat=0&search=The+Show+S01E02&options=0"},
		},
		{
			"movie by IMDb ID",
			indexer.Query{Type: indexer.SearchMovie, Term: "Film", IMDbID: "tt0076759", Categories: []int{100002}},
			[]string{
				"https://www.bit-hdtv.com/torrents.php?cat=2&search=tt0076759&options=4",
				"https://www.bit-hdtv.com/torrents.php?cat=2&search=tt0076759&options=1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, tier := range c.Tiers() {
				for _, pages := range tier {
					for r := range pages {
						got = append(got, r.URL)
						if v := r.Cookies["h_sp"]; v != "b" {
							t.Errorf("cookie h_sp = %q, want = %q", v, "b")
						}
					}
				}
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("URLs = %q, want = %q", got, tt.want)
			}
		})
	}
}

// page is a search page as BIT-HDTV shows it: the results' table follows
// the navigation's and a line break.
const page = `<html><body>
<table align=center><tr><td><a href="/torrents.php">Browse</a></td></tr></table>
<br>
<table width="100%">
<tr><td>Fav</td><td>Type</td><td>Name</td><td>Files</td><td>Comm.</td><td>Added</td><td>Size</td><td>Snatched</td><td>Seeders</td><td>Leechers</td></tr>
<tr bgcolor="#FFFF99">
<td></td>
<td><a href="torrents.php?cat=10"><img alt="TV"></a></td>
<td><a href="https://www.bit-hdtv.com/details.php?id=123" title="The.Show.S01E02.1080p.WEB.h264-GRP"><b>The.Show.S01E02...</b></a> <a href="download.php?id=123"><img alt="DL"></a><span id="needseed">Needs seeders</span></td>
<td>3</td>
<td>0</td>
<td>2023-05-01<br>12:34:56</td>
<td>1.5 GB</td>
<td>1,024</td>
<td>87</td>
<td>4</td>
</tr>
<tr bgcolor="#DDDDDD">
<td></td>
<td><a href="torrents.php?cat=7"><img alt="Movies"></a></td>
<td><a href="https://www.bit-hdtv.com/details.php?id=124" title="A.Film.2020.1080p.BluRay.x264-GRP">A.Film</a> <a href="download.php?id=124"><img alt="DL"></a></td>
<td>1</td>
<td>2</td>
<td>2023-04-30<br>01:02:03</td>
<td>8.5 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
<tr>
<td></td>
<td><a href="torrents.php?cat=7"><img alt="Movies"></a></td>
<td><a href="https://www.bit-hdtv.com/details.php?id=124" title="A.Film.2020.1080p.BluRay.x264-GRP">A.Film</a> <a href="download.php?id=124"><img alt="DL"></a></td>
<td>1</td>
<td>2</td>
<td>2023-04-30<br>01:02:03</td>
<td>8.5 GB</td>
<td>10</td>
<td>5</td>
<td>1</td>
</tr>
</table>
</body></html>`

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
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "The.Show.S01E02.1080p.WEB.h264-GRP"},
		{"GUID", r.GUID, "https://www.bit-hdtv.com/details.php?id=123"},
		{"InfoURL", r.InfoURL, "https://www.bit-hdtv.com/details.php?id=123"},
		{"DownloadURL", r.DownloadURL, "https://www.bit-hdtv.com/download.php?id=123"},
		{"Categories", r.Categories, []int{5000, 100010}},
		{"Size", *r.Size, int64(1610612736)},
		{"Files", *r.Files, 3},
		{"Grabs", *r.Grabs, 1024},
		{"Seeders", *r.Seeders, 87},
		{"Peers", *r.Peers, 91},
		{"PublishDate", r.PublishDate, time.Date(2023, 5, 1, 12, 34, 56, 0, time.Local)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(172800)},
		{"second Categories", r2.Categories, []int{2000, 100007}},
		{"second DownloadVolumeFactor", *r2.DownloadVolumeFactor, 1.0},
		{"second UploadVolumeFactor", *r2.UploadVolumeFactor, 2.0},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func equal(a, b any) bool {
	if x, ok := a.(time.Time); ok {
		return x.Equal(b.(time.Time))
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestParseNoResults(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(`<html><body><table align=center><tr><td></td></tr></table><p>Nothing found!</p></body></html>`))
	releases, err := ix.Search(context.Background(), &indexer.Query{Term: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 0 {
		t.Errorf("releases = %d, want = 0", len(releases))
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{}).Validate(); err == nil || !strings.Contains(err.Error(), "cookie") {
		t.Errorf("Validate() = %v, want = a cookie error", err)
	}
}

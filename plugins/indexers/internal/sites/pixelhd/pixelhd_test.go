package pixelhd

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
	return &indexer.Indexer{ID: "1", Name: "PiXELHD", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

var testSettings = Settings{Cookie: "session=abc", UserAgent: "Mozilla/5.0"}

func TestSearchRequests(t *testing.T) {
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{"RSS", indexer.Query{}, "https://pixelhd.me/torrents.php?order_by=time&order_way=desc"},
		{"term", indexer.Query{Term: "A Film (2020)"}, "https://pixelhd.me/torrents.php?order_by=time&order_way=desc&groupname=A+Film+(2020)"},
		{
			"movie by IMDb ID",
			indexer.Query{Type: indexer.SearchMovie, Term: "A Film", IMDbID: "tt0076759"},
			"https://pixelhd.me/torrents.php?order_by=time&order_way=desc&imdbid=tt0076759&groupname=A+Film",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, testSettings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			r := c.First()
			if r.URL != tt.want {
				t.Errorf("URL = %q, want = %q", r.URL, tt.want)
			}
			if got := r.Header.Get("User-Agent"); got != "Mozilla/5.0" {
				t.Errorf("User-Agent = %q, want = %q", got, "Mozilla/5.0")
			}
			if got := r.Cookies["session"]; got != "abc" {
				t.Errorf("cookie session = %q, want = %q", got, "abc")
			}
		})
	}
	for _, typ := range []indexer.SearchType{indexer.SearchTV, indexer.SearchMusic, indexer.SearchBook} {
		ix := newIndexer(t, testSettings, nil)
		c, err := ix.Site.SearchRequests(context.Background(), ix, &indexer.Query{Type: typ, Term: "x"})
		if err != nil || c.First() != nil {
			t.Errorf("%s SearchRequests() = %v, %v, want = no requests", typ, c.First(), err)
		}
	}
}

// page is a search page as PiXELHD shows it: posters of groups of
// torrents.
const page = `<html><body>
<div class="browsePoster">
<strong><a href="torrents.php?id=10" title="View Torrent">A Film</a> [2020 ]</strong>
<a href="https://www.imdb.com/title/tt0076759/">IMDb</a>
<table>
<tr class="group_torrent">
<td><a href="torrents.php?action=download&amp;id=101&amp;authkey=a&amp;torrent_pass=b">DL</a></td>
<td><a href="torrents.php?id=10&amp;torrentid=101"> 1080p / MP4 </a></td>
<td><span class="time" title="Jan 05 2024, 14:22">5 days ago</span></td>
<td>2.5 GB</td>
<td>0</td>
<td>42</td>
<td>12</td>
<td> 3 </td>
</tr>
<tr class="group_torrent">
<td><a href="torrents.php?action=download&amp;id=102">DL</a></td>
<td><a href="torrents.php?id=10&amp;torrentid=102">720p / MP4</a></td>
<td>2 hours ago</td>
<td>1 GB</td>
<td>0</td>
<td>7</td>
<td>1</td>
<td>0</td>
</tr>
</table>
</div>
</body></html>`

func respond(status int, body string) handler {
	return func(*http.Request) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}
	}
}

func TestParse(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(http.StatusOK, page))
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	ix.Now = func() time.Time { return now }
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
		{"Title", r.Title, "A Film [2020] 1080p / MP4"},
		{"GUID", r.GUID, "https://pixelhd.me/torrents.php?id=10&torrentid=101"},
		{"InfoURL", r.InfoURL, "https://pixelhd.me/torrents.php?id=10&torrentid=101"},
		{"DownloadURL", r.DownloadURL, "https://pixelhd.me/torrents.php?action=download&id=101&authkey=a&torrent_pass=b"},
		{"Categories", r.Categories, []int{2000, 2040}},
		{"IMDbID", r.IMDbID, 76759},
		{"Size", *r.Size, int64(2684354560)},
		{"Grabs", *r.Grabs, 42},
		{"Seeders", *r.Seeders, 12},
		{"Peers", *r.Peers, 15},
		{"PublishDate", r.PublishDate, time.Date(2024, 1, 5, 14, 22, 0, 0, time.Local)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
		{"MinimumRatio", *r.MinimumRatio, 1.0},
		{"MinimumSeedTime", *r.MinimumSeedTime, int64(259200)},
		{"second Title", r2.Title, "A Film [2020] 720p / MP4"},
		{"second PublishDate", r2.PublishDate, now.Add(-2 * time.Hour)},
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

func TestParseUnexpectedStatus(t *testing.T) {
	ix := newIndexer(t, testSettings, respond(http.StatusForbidden, "denied"))
	_, err := ix.Search(context.Background(), &indexer.Query{})
	if want := "Expected 200 OK but got Forbidden."; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want = containing %q", err, want)
	}
}

func TestValidate(t *testing.T) {
	if err := (Settings{Cookie: "a=b"}).Validate(); err == nil || !strings.Contains(err.Error(), "user_agent") {
		t.Errorf("Validate() = %v, want = a user_agent error", err)
	}
}

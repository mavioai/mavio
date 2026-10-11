package anidex

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
	return &indexer.Indexer{ID: "1", Name: "Anidex", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

func TestSearchRequests(t *testing.T) {
	filtered := Settings{AuthorisedOnly: true, LanguagesOnly: []int{1, 2}}
	const prefix = "https://anidex.info/?page=search&s=upload_timestamp&o=desc&group_id=0&q="
	tests := []struct {
		name     string
		settings Settings
		q        indexer.Query
		want     string
	}{
		{"rss", Settings{}, indexer.Query{}, prefix},
		{"all categories", Settings{}, indexer.Query{Term: "x", Categories: []int{5070, 7000, 3000, 4000, 6000, 8000}}, prefix + "x"},
		{
			"tv episode", filtered,
			indexer.Query{Type: indexer.SearchTV, Term: "One Piece", Season: indexer.Ptr(1), Episode: "5", Categories: []int{5070}},
			prefix + "One+Piece+S01E05&a=1&id=1,2,3,4,5&lang_id=1,2",
		},
		{"music", Settings{}, indexer.Query{Type: indexer.SearchMusic, Term: "OST (2020)"}, prefix + "OST+(2020)"},
		{"movie", Settings{}, indexer.Query{Type: indexer.SearchMovie, Term: "Akira"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, tt.settings, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			if r := c.First(); r != nil {
				got = r.URL
			}
			if got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

const page = `<!DOCTYPE html><html><body><div id="content"><table class="table">
<thead><tr><th>Category</th></tr></thead>
<tbody>
<tr>
<td><a href="/?id=1"><span>Anime - Sub</span></a> <img title="English" src="/images/flags/gb.png"></td>
<td></td>
<td><a href="/torrent/12345" class="torrent"><span title="[SubsPlease] One Piece - 1059 (1080p) [ABCD1234].mkv">[SubsPlease] One Piece - 1059...</span></a></td>
<td><a href="/dl/12345"><i></i></a></td>
<td><a href="magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&amp;dn=x"><i></i></a></td>
<td></td>
<td>1.37 GB</td>
<td title="2023-04-23 02:06:08 UTC">2 hours ago</td>
<td>1,204</td>
<td>87</td>
<td>4,321</td>
</tr>
</tbody></table></div></body></html>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, Settings{}, func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(page))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 1 {
		t.Fatalf("releases = %d, want = 1", len(releases))
	}
	r := releases[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "[SubsPlease] One Piece - 1059 (1080p) [ABCD1234].mkv [English]"},
		{"GUID", r.GUID, "https://anidex.info//torrent/12345"},
		{"InfoURL", r.InfoURL, "https://anidex.info//torrent/12345"},
		{"DownloadURL", r.DownloadURL, "https://anidex.info//dl/12345"},
		{"MagnetURL", r.MagnetURL, "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=x"},
		{"Categories", len(r.Categories), 2},
		{"Category", r.Categories[0], 5070},
		{"Seeders", *r.Seeders, 1204},
		{"Peers", *r.Peers, 1291},
		{"Grabs", *r.Grabs, 4321},
		{"Size", *r.Size, int64(1471026298)},
		{"PublishDate", r.PublishDate, time.Date(2023, 4, 23, 2, 6, 8, 0, time.UTC)},
		{"DownloadVolumeFactor", *r.DownloadVolumeFactor, 0.0},
		{"UploadVolumeFactor", *r.UploadVolumeFactor, 1.0},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestParseStatus(t *testing.T) {
	ix := newIndexer(t, Settings{}, func(*http.Request) *http.Response {
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(""))}
	})
	_, err := ix.Search(context.Background(), &indexer.Query{})
	if want := "Expected 200 OK but got 403"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Search() error = %v, want = %q", err, want)
	}
}

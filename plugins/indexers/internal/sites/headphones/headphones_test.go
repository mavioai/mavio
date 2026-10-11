package headphones

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
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

func newIndexer(t *testing.T, h handler) *indexer.Indexer {
	t.Helper()
	data, err := json.Marshal(indexer.UserPassSettings{Username: "user", Password: "pass"})
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
	return &indexer.Indexer{ID: "1", Name: "Headphones", Site: site, Session: c.Session(nil, nil), Statuses: st}
}

const auth = "Basic dXNlcjpwYXNz"

func TestSearchRequests(t *testing.T) {
	const base = "https://indexer.codeshy.com/api?t=search&extended=1"
	tests := []struct {
		name string
		q    indexer.Query
		want string
	}{
		{"rss", indexer.Query{}, base + "&apikey=964d601959918a578a670984bdee9357&offset=0"},
		{
			"basic",
			indexer.Query{Term: "Daft Punk C++", Limit: 100, Offset: 200},
			base + "&apikey=964d601959918a578a670984bdee9357&q=Daft+Punk+C%2520%2520&limit=100&offset=200",
		},
		{
			"music in categories",
			indexer.Query{Type: indexer.SearchMusic, Term: "Album", Artist: "Artist", Categories: []int{3040, 3010, 3040}},
			base + "&cat=3040,3010&apikey=964d601959918a578a670984bdee9357&q=Album&offset=0",
		},
		{"movie", indexer.Query{Type: indexer.SearchMovie, Term: "Film"}, ""},
		{"tv", indexer.Query{Type: indexer.SearchTV, Term: "Show"}, ""},
		{"book", indexer.Query{Type: indexer.SearchBook, Term: "Book"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, nil)
			c, err := ix.Site.SearchRequests(context.Background(), ix, &tt.q)
			if err != nil {
				t.Fatal(err)
			}
			// Prowlarr's IndexerRequests of URLs follow redirects.
			if r := c.First(); r != nil && !r.FollowRedirects {
				t.Errorf("FollowRedirects = false, want = true")
			}
			r := c.First()
			got := ""
			if r != nil {
				got = r.URL
				if a := r.Header.Get("Authorization"); a != auth {
					t.Errorf("Authorization = %q, want = %q", a, auth)
				}
			}
			if got != tt.want {
				t.Errorf("URL = %q, want = %q", got, tt.want)
			}
		})
	}
}

const feed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/">
<channel>
<item>
<title>Artist - Album (2020) [FLAC]</title>
<guid isPermaLink="true">https://indexer.codeshy.com/details/abc</guid>
<link>https://indexer.codeshy.com/getnzb/abc.nzb</link>
<comments>https://indexer.codeshy.com/details/abc#comments</comments>
<pubDate>Sat, 01 Feb 2020 10:00:00 +0000</pubDate>
<enclosure url="https://indexer.codeshy.com/getnzb/abc.nzb" length="123" type="application/x-nzb"/>
<newznab:attr name="category" value="3000"/>
<newznab:attr name="category" value="3040"/>
<newznab:attr name="size" value="456789"/>
<newznab:attr name="grabs" value="7"/>
<newznab:attr name="usenetdate" value="Fri, 31 Jan 2020 09:00:00 +0000"/>
<newznab:attr name="language" value="English, French"/>
</item>
<item>
<title>Other - Single</title>
<guid>def</guid>
<pubDate>Sun, 02 Feb 2020 10:00:00 +0000</pubDate>
<enclosure url="/getnzb/def.nzb" length="99" type="application/x-nzb"/>
<newznab:attr name="category" value="Audio"/>
<newznab:attr name="imdb" value="1234567"/>
<newznab:attr name="files" value="12"/>
</item>
</channel>
</rss>`

func TestParse(t *testing.T) {
	ix := newIndexer(t, func(r *http.Request) *http.Response {
		if got := r.Header.Get("Authorization"); got != auth {
			t.Errorf("Authorization = %q, want = %q", got, auth)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/rss+xml"}}, Body: io.NopCloser(strings.NewReader(feed))}
	})
	releases, err := ix.Search(context.Background(), &indexer.Query{Interactive: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 {
		t.Fatalf("releases = %d, want = 2", len(releases))
	}
	r, o := releases[0], releases[1]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Title", r.Title, "Artist - Album (2020) [FLAC]"},
		{"GUID", r.GUID, "https://indexer.codeshy.com/details/abc"},
		{"DownloadURL", r.DownloadURL, "https://indexer.codeshy.com/getnzb/abc.nzb"},
		{"InfoURL", r.InfoURL, "https://indexer.codeshy.com/details/abc"},
		{"CommentsURL", r.CommentsURL, "https://indexer.codeshy.com/details/abc#comments"},
		{"PublishDate", r.PublishDate, time.Date(2020, 1, 31, 9, 0, 0, 0, time.UTC)},
		{"Size", *r.Size, int64(456789)},
		{"Grabs", *r.Grabs, 7},
		{"Files", *r.Files, 0},
		{"Categories", slices.Equal(r.Categories, []int{3000, 3040}), true},
		{"Languages", strings.Join(r.Languages, "|"), "English, French"},
		{"Protocol", r.Protocol, indexer.ProtocolUsenet},
		{"other DownloadURL", o.DownloadURL, "https://indexer.codeshy.com/getnzb/def.nzb"},
		{"other Size", *o.Size, int64(99)},
		{"other Categories", len(o.Categories), 0},
		{"other IMDbID", o.IMDbID, 1234567},
		{"other Files", *o.Files, 12},
		{"other Grabs", *o.Grabs, 0},
	} {
		if c.got != c.want {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name, body string
		want       string
		auth       bool
	}{
		{"auth", `<?xml version="1.0"?><error code="100" description="Incorrect user credentials"/>`, "Incorrect user credentials", true},
		{"other", `<?xml version="1.0"?><error code="300" description="Something broke"/>`, "Something broke", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix := newIndexer(t, func(*http.Request) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/xml"}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			})
			_, err := ix.Search(context.Background(), &indexer.Query{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Search() error = %v, want = containing %q", err, tt.want)
			}
			if got := errors.Is(err, indexer.ErrAuth); got != tt.auth {
				t.Errorf("errors.Is(err, ErrAuth) = %v, want = %v", got, tt.auth)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	ix := newIndexer(t, func(r *http.Request) *http.Response {
		if got := r.Header.Get("Authorization"); got != auth {
			t.Errorf("Authorization = %q, want = %q", got, auth)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/x-nzb"}}, Body: io.NopCloser(strings.NewReader("<nzb><file/></nzb>"))}
	})
	data, err := ix.Download(context.Background(), "https://indexer.codeshy.com/getnzb/abc.nzb")
	if err != nil || string(data) != "<nzb><file/></nzb>" {
		t.Errorf("Download() = %q, %v, want = %q, nil", data, err, "<nzb><file/></nzb>")
	}
}

func TestValidate(t *testing.T) {
	if _, err := Definition.New([]byte(`{"username":"u"}`)); err == nil || !strings.Contains(err.Error(), "password") {
		t.Errorf("New() error = %v, want = a password error", err)
	}
}

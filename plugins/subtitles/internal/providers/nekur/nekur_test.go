package nekur

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const results = `<table><thead><tr><th>t</th></tr></thead><tbody>
<tr><td class="title"><a href="files/inception.zip">Inception <span class="year">(2010)</span></a></td>
<td class="fps">23.976</td><td class="notes">Inception.2010.720p.BluRay.x264-REFiNED</td>
<td><a href="https://www.imdb.com/title/tt1375666/">IMDb</a></td></tr>
<tr><td class="title"><a href="files/inception-dvd.srt">Inception <span class="year">(2010)</span></a></td>
<td class="fps">25</td><td class="notes">DVDRip</td>
<td><a href="https://www.imdb.com/title/tt1375666/">IMDb</a></td></tr>
<tr><td class="title">broken</td></tr>
</tbody></table>`

func zipOf(t *testing.T, name, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(name)
	_, _ = w.Write([]byte(data))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSearchAndDownload(t *testing.T) {
	var searches []string
	srt := "1\r\n00:00:01,000 --> 00:00:02,000\r\nSveiki\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/Subtitles.php":
			if err := r.ParseForm(); err != nil || r.Method != http.MethodPost || r.Form.Get("ajax") != "1" {
				t.Errorf("got = %s %v, want = a POST of the search form", r.Method, r.Form)
			}
			if r.Header.Get("Referer") == "" {
				t.Errorf("referer got = none, want = the site")
			}
			searches = append(searches, r.Form.Get("sSearch"))
			if r.Form.Get("sSearch") == "Inception" {
				_, _ = w.Write([]byte(results))
			}
		case "/files/inception.zip":
			_, _ = w.Write(zipOf(t, "Inception.2010.lv.srt", srt))
		case "/files/inception-dvd.srt":
			_, _ = w.Write([]byte(srt))
		case "/files/page.html":
			_, _ = w.Write([]byte("<html>not found</html>"))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	p, _ := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)

	v := &provider.Video{
		Title: "Inception", AlternativeTitles: []string{"Iesākums"}, Year: 2010, FrameRate: 25,
		ExternalIDs: map[string]string{"imdb": "tt1375666"},
	}
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}}); err != nil || subs != nil {
		t.Errorf("english got = %v, %v, want = none", subs, err)
	}
	if subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "X"}, nil); err != nil || subs != nil {
		t.Errorf("episode got = %v, %v, want = none", subs, err)
	}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "lv"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Inception", "Iesākums"}; !slices.Equal(searches, want) {
		t.Errorf("searches got = %v, want = %v", searches, want)
	}
	if len(subs) != 2 {
		t.Fatalf("got = %d subtitles, want = 2", len(subs))
	}
	s := subs[0]
	if s.ID != srv.URL+"/files/inception.zip" || !slices.Equal(s.ReleaseNames, []string{"Inception.2010.720p.BluRay.x264-REFiNED"}) ||
		!slices.Equal(s.Matches, []string{"title", "year", "imdb_id"}) || s.Language.Tag != "lv" {
		t.Errorf("got = %+v, want = the zip matching title, year and IMDb ID", s)
	}
	for _, sub := range subs {
		f, err := p.Download(t.Context(), v, sub.ID)
		if err != nil {
			t.Fatal(err)
		}
		if f.Format != "srt" || f.Language.Tag != "lv" || len(f.Data) == 0 {
			t.Errorf("%s: got = %q, %q, want = an srt in lv", sub.ID, f.Data, f.Format)
		}
	}
	for id, want := range map[string]provider.Kind{
		srv.URL + "/files/page.html": provider.ProviderError,
		srv.URL + "/files/gone.zip":  provider.ServiceUnavailable,
		"http://elsewhere/x.zip":     provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), v, id); provider.KindOf(err) != want {
			t.Errorf("%s: got = %v, want = %v", id, err, want)
		}
	}
}

func TestFramerateEqual(t *testing.T) {
	for _, c := range []struct {
		a, b float64
		want bool
	}{{25, 25, true}, {23.976, 24, true}, {23.98, 25, false}} {
		if got := framerateEqual(c.a, c.b); got != c.want {
			t.Errorf("framerateEqual(%v, %v) got = %v, want = %v", c.a, c.b, got, c.want)
		}
	}
}

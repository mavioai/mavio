package greeksubtitles

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

func page(rows, next string) string {
	return `<html><body><table>` + rows + `<tr><td>` + next + `</td></tr></table></body></html>`
}

func row(id, lang, version string) string {
	return `<tr><td class="latest_name"><img src="/images/flags/` + lang + `.gif"><a href="http://gr.greek-subtitles.com/` + id + `/x.html">` +
		version + `</a> <a href="/other">info</a></td></tr>`
}

func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		w, err := zw.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[i+1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newProvider(t *testing.T, h http.Handler) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	var names []string
	p := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		names = append(names, name)
		switch {
		case r.URL.Query().Get("page") == "2":
			_, _ = w.Write([]byte(page(row("103", "en", "Show.S01E02.WEB"), "")))
		case name == "Show S01E02":
			_, _ = w.Write([]byte(page(row("101", "el", "Show.S01E02.720p-GRP")+row("102", "fr", "French"),
				`<a href="search.php?name=Show+S01E02&page=2">Next &gt;</a>`)))
		case name == "Movie 2010":
			_, _ = w.Write([]byte(page(row("201", "gr", "Movie.2010"), "")))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	ep := &provider.Video{Episode: true, Series: "Show", AlternativeSeries: []string{"Alt"}, Season: 1, Number: 2, Year: 2020}
	subs, err := p.Search(t.Context(), ep, []provider.Language{{Tag: "el"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, s.Language.Tag+":"+s.ReleaseNames[0])
		if !slices.Equal(s.Matches, []string{"year"}) {
			t.Errorf("matches got = %v, want = [year]", s.Matches)
		}
	}
	if want := []string{"el:Show.S01E02.720p-GRP", "en:Show.S01E02.WEB"}; !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
	if want := []string{"Show S01E02", "Show S01E02", "Alt S01E02"}; !slices.Equal(names, want) {
		t.Errorf("searches got = %v, want = %v", names, want)
	}
	q, _ := url.ParseQuery(subs[0].ID)
	if q.Get("id") != "101" || subs[0].PageLink != "http://gr.greek-subtitles.com/101/x.html" {
		t.Errorf("got = %v, %s, want = 101", q, subs[0].PageLink)
	}

	movie := &provider.Video{Title: "Movie", Year: 2010}
	subs, err = p.Search(t.Context(), movie, nil)
	if err != nil || len(subs) != 0 {
		// "gr" is no language code: the row is skipped.
		t.Errorf("got = %v, %v, want = none", subs, err)
	}
}

func TestDownload(t *testing.T) {
	srt := "1\r\n00:00:01,000 --> 00:00:02,000\r\nΓεια\r\n"
	archives := map[string][]byte{
		"1": zipOf(t, "readme.nfo", "x", ".hidden.srt", "x", "Show.S01E02.srt", srt),
		"2": zipOf(t, "readme.nfo", "x"),
		"3": []byte("<html>nothing</html>"),
		"4": nil,
	}
	p := newProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getp.php" || r.Header.Get("Referer") != "http://gr.greek-subtitles.com/1/x.html" {
			t.Errorf("got = %s with referer %q, want = /getp.php with the page", r.URL.Path, r.Header.Get("Referer"))
		}
		data, ok := archives[r.URL.Query().Get("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	id := func(n string) string {
		return url.Values{"id": {n}, "page": {"http://gr.greek-subtitles.com/1/x.html"}, "lang": {"el"}}.Encode()
	}
	f, err := p.Download(t.Context(), nil, id("1"))
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "1\n00:00:01,000 --> 00:00:02,000\nΓεια\n" || f.Format != "srt" || f.Language.Tag != "el" {
		t.Errorf("got = %q, %q, %q, want = the srt", f.Data, f.Format, f.Language.Tag)
	}
	for n, want := range map[string]provider.Kind{"2": provider.BadSubtitle, "3": provider.BadSubtitle, "4": provider.ProviderError, "5": provider.ProviderError} {
		if _, err := p.Download(t.Context(), nil, id(n)); provider.KindOf(err) != want {
			t.Errorf("%s: got = %v, want = %v", n, err, want)
		}
	}
}

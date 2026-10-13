package yifysubtitles

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

func zipped(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		f, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(files[i+1]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const moviePage = `<html><body><table class="other-subs"><thead><tr><th>Rating</th></tr></thead><tbody>
	<tr><td>2</td><td>English</td><td><a href="/subtitles/up-english-yify-1">
subtitle Up.2009.720p.BluRay.x264-YIFY
Up.2009.1080p.BluRay</a></td><td></td><td>uploader1</td></tr>
	<tr><td>5</td><td>English</td><td><a href="/subtitles/up-english-yify-2">
subtitle Up.2009.WEB</a></td><td><span class="hi-subtitle"></span></td><td>uploader2</td></tr>
	<tr><td>9</td><td>Brazilian Portuguese</td><td><a href="/subtitles/up-pt-3">
subtitle Up.2009.BRRip</a></td><td></td><td>uploader3</td></tr>
	<tr><td>7</td><td>Klingon</td><td><a href="/subtitles/up-tlh-4">
subtitle Up</a></td><td></td><td>uploader4</td></tr>
	<tr><td>x</td><td>English</td><td><a href="/subtitles/up-en-5">
subtitle Up</a></td><td></td><td>uploader5</td></tr>
	<tr><td>8</td><td>English</td><td><a href="/subtitles/up-english-yify-6">
subtitle Up.2009.DVDRip</a></td><td></td><td>uploader6</td></tr>
</tbody></table></body></html>`

func newFake(t *testing.T) *Provider {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" || r.Header.Get("Accept-Language") != "en-US,en;q=0.5" {
			t.Errorf("%s: headers %v", r.URL.Path, r.Header)
		}
		switch r.URL.Path {
		case "/movie-imdb/tt1049413":
			_, _ = fmt.Fprint(w, moviePage)
		case "/movie-imdb/tt0000002":
			http.Redirect(w, r, "/elsewhere", http.StatusMovedPermanently)
		case "/movie-imdb/tt0000003":
			http.Error(w, "down", http.StatusServiceUnavailable)
		case "/subtitles/up-english-yify-1":
			_, _ = fmt.Fprint(w, `<html><body><a class="download-subtitle" href="/subtitle/up-1.zip">Download</a></body></html>`)
		case "/subtitles/up-english-yify-2":
			_, _ = fmt.Fprint(w, `<html><body><a class="download-subtitle" href="/subtitle/up-2.rar">Download</a></body></html>`)
		case "/subtitles/up-english-yify-6":
			_, _ = fmt.Fprint(w, `<html><body>gone</body></html>`)
		case "/subtitle/up-1.zip":
			_, _ = w.Write(zipped(t, "readme.txt", "x", "Up.empty.srt", "", "Up.2009.srt", srt))
		case "/subtitle/up-2.rar":
			_, _ = w.Write([]byte(srt))
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func movie(imdb string) *provider.Video {
	return &provider.Video{Title: "Up", Year: 2009, ExternalIDs: map[string]string{"imdb": imdb}}
}

func TestSearch(t *testing.T) {
	p := newFake(t)
	subs, err := p.Search(t.Context(), movie("tt1049413"), []provider.Language{{Tag: "en"}, {Tag: "en", HearingImpaired: true}, {Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.ID)
	}
	// The best rated first; unknown languages and unreadable rows are left
	// out.
	want := []string{"/subtitles/up-pt-3", "/subtitles/up-english-yify-6", "/subtitles/up-english-yify-2", "/subtitles/up-english-yify-1"}
	if !slices.Equal(ids, want) {
		t.Fatalf("subtitles = %v, want = %v", ids, want)
	}
	if s := subs[3]; !slices.Equal(s.ReleaseNames, []string{"Up.2009.720p.BluRay.x264-YIFY", "Up.2009.1080p.BluRay"}) ||
		!slices.Equal(s.Matches, []string{"imdb_id"}) || s.Uploader != "uploader1" || s.Language != (provider.Language{Tag: "en"}) {
		t.Errorf("subtitle = %+v", s)
	}
	if s := subs[2]; s.Language != (provider.Language{Tag: "en", HearingImpaired: true}) {
		t.Errorf("language = %+v, want = hearing impaired English", s.Language)
	}
	if s := subs[0]; s.Language.Tag != "pt-BR" {
		t.Errorf("language = %+v, want = pt-BR", s.Language)
	}

	// Hearing-impaired subtitles are those of hearing-impaired languages.
	subs, err = p.Search(t.Context(), movie("tt1049413"), []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 2 {
		t.Errorf("search in English = %+v, %v, want = 2 subtitles", subs, err)
	}
	for _, c := range []*provider.Video{movie("tt0000001"), movie("tt0000002"), movie(""), {Episode: true, Series: "Up"}} {
		if subs, err := p.Search(t.Context(), c, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
			t.Errorf("search(%+v) = %v, %v, want = none", c, subs, err)
		}
	}
	if _, err := p.Search(t.Context(), movie("tt0000003"), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("search: err = %v, want = ServiceUnavailable", err)
	}
}

func TestDownload(t *testing.T) {
	p := newFake(t)
	f, err := p.Download(t.Context(), movie("tt1049413"), "/subtitles/up-english-yify-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"; string(f.Data) != want || f.Format != "srt" {
		t.Errorf("download = %q %s, want = %q srt", f.Data, f.Format, want)
	}
	for id, kind := range map[string]provider.Kind{
		"/subtitles/up-english-yify-2": provider.BadSubtitle,
		"/subtitles/up-english-yify-6": provider.ProviderError,
		"/subtitles/missing":           provider.ProviderError,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != kind {
			t.Errorf("download %s: err = %v, want = %v", id, err, kind)
		}
	}
}

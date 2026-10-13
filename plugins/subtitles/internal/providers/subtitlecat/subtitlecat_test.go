package subtitlecat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

type fakeSite struct {
	srv *httptest.Server

	mu       sync.Mutex
	searches []string
	results  map[string]string
	status   int
}

// resultRow is a release of the search results.
func resultRow(href, release, source string) string {
	marker := ""
	if source != "" {
		marker = " (translated from " + source + ")"
	}
	return fmt.Sprintf(`<tr><td><a href="%s">%s</a>%s</td><td>10 downloads</td></tr>`, href, release, marker)
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{results: map[string]string{
		"The Matrix 1999": resultRow("subs/1/The.Matrix.1999.720p.html", "The.Matrix.1999.720p.BluRay", "English") +
			resultRow("subs/2/The.Matrix.Reloaded.2003.html", "The.Matrix.Reloaded.2003.1080p", "English") +
			resultRow("subs/3/The.Matrix.1999.French.html", "The.Matrix.1999.DVDRip", "French") +
			resultRow("other/4.html", "The.Matrix.1999.Other", "") +
			resultRow("subs/5/broken.html", "The.Matrix.1999.Broken", ""),
		"Show S01E02": resultRow("subs/6/Show.S01E03.html", "Show.S01E03.HDTV", "English"),
		"Serie S01E02": resultRow("subs/7/Serie.S01E02.html", "Serie.S01E02.WEB", "") +
			resultRow("subs/8/Serie.S01E01-02.html", "Serie.S01E01E02.WEB", ""),
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /index.php", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		q := r.URL.Query().Get("search")
		f.searches = append(f.searches, q)
		rows, status := f.results[q], f.status
		f.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		_, _ = fmt.Fprintf(w, `<table class="sub-table"><thead><tr><td>Release</td></tr></thead><tbody>%s</tbody></table>`, rows)
	})
	detail := func(id string) string {
		return fmt.Sprintf(`<div class="sub-single"><span>English</span><a id="download_en" href="/subs/%s/en.srt">Download</a></div>
			<div class="sub-single"><span>French</span><a id="download_fr" href="subs/%s/fr.srt">Download</a></div>
			<div class="sub-single"><span>Chinese</span><a id="download_zh-CN" href="https://cdn.example/%s/zh.srt">Download</a></div>
			<div class="sub-single"><span>Unknown</span><a id="download_xx" href="/subs/%s/xx.srt">Download</a></div>
			<div class="sub-single"><span>Pending</span><a id="download_de">Translate</a></div>`, id, id, id, id)
	}
	mux.HandleFunc("GET /subs/{id}/{page}", func(w http.ResponseWriter, r *http.Request) {
		switch page := r.PathValue("page"); {
		case r.PathValue("id") == "5":
			w.WriteHeader(http.StatusInternalServerError)
		case page == "en.srt":
			_, _ = w.Write([]byte(srt))
		case page == "fr.srt":
			// An empty subtitle.
		case strings.HasSuffix(page, ".html"):
			_, _ = w.Write([]byte(detail(r.PathValue("id"))))
		default:
			http.NotFound(w, r)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func describe(base string, subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, fmt.Sprintf("%s %s %v %q", strings.TrimPrefix(s.ID, base), s.Language.Tag, s.ReleaseNames, s.Comment))
	}
	return out
}

func TestSearch(t *testing.T) {
	matrix := &provider.Video{Title: "The Matrix", Year: 1999, AlternativeTitles: []string{"the matrix", "Matrix"}}
	for _, c := range []struct {
		name     string
		settings string
		video    *provider.Video
		langs    []string
		want     []string
		searches []string
	}{
		{
			// The other year's release and the French source are left out,
			// as the machine translations.
			"movie", "", matrix,
			[]string{"en", "fr"},
			[]string{"/subs/1/en.srt en [The.Matrix.1999.720p.BluRay] \"\"", "/subs/3/fr.srt fr [The.Matrix.1999.DVDRip] \"\""},
			[]string{"The Matrix 1999"},
		},
		{
			"machine translated", `{"include_machine_translated": true}`, matrix,
			[]string{"fr", "zh"},
			[]string{
				"/subs/1/fr.srt fr [The.Matrix.1999.720p.BluRay] \"machine translated\"",
				"https://cdn.example/1/zh.srt zh [The.Matrix.1999.720p.BluRay] \"machine translated\"",
				"/subs/3/fr.srt fr [The.Matrix.1999.DVDRip] \"\"",
				"https://cdn.example/3/zh.srt zh [The.Matrix.1999.DVDRip] \"machine translated\"",
			},
			[]string{"The Matrix 1999"},
		},
		{
			// No release of the episode: the alternative title is searched,
			// then the primary search's releases are taken.
			"fallback", "", &provider.Video{Episode: true, Series: "Show", AlternativeSeries: []string{"Other"}, Season: 1, Number: 2},
			[]string{"en"},
			[]string{"/subs/6/en.srt en [Show.S01E03.HDTV] \"\""},
			[]string{"Show S01E02", "Other S01E02"},
		},
		{
			// A release of two episodes is no candidate.
			"episode", "", &provider.Video{Episode: true, Series: "Serie", Season: 1, Number: 2},
			[]string{"en"},
			[]string{"/subs/7/en.srt en [Serie.S01E02.WEB] \"\""},
			[]string{"Serie S01E02"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			p := newProvider(t, f, c.settings)
			var langs []provider.Language
			for _, l := range c.langs {
				langs = append(langs, provider.Language{Tag: l})
			}
			subs, err := p.Search(t.Context(), c.video, langs)
			if got := describe(f.srv.URL, subs); err != nil || !slices.Equal(got, c.want) {
				t.Errorf("Search got = %v, %v, want = %v", got, err, c.want)
			}
			if !slices.Equal(f.searches, c.searches) {
				t.Errorf("searches got = %q, want = %q", f.searches, c.searches)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	for _, c := range []struct {
		status int
		kind   provider.Kind
	}{
		{http.StatusTooManyRequests, provider.TooManyRequests},
		{http.StatusForbidden, provider.ForbiddenError},
		{http.StatusServiceUnavailable, provider.ServiceUnavailable},
		{http.StatusNotFound, provider.ProviderError},
	} {
		f := newFake(t)
		f.status = c.status
		p := newProvider(t, f, "")
		if _, err := p.Search(t.Context(), &provider.Video{Title: "The Matrix"}, []provider.Language{{Tag: "en"}}); provider.KindOf(err) != c.kind {
			t.Errorf("Search of %d got = %v, want = %v", c.status, err, c.kind)
		}
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, "")
	file, err := p.Download(t.Context(), nil, f.srv.URL+"/subs/1/en.srt")
	if err != nil || string(file.Data) != strings.ReplaceAll(srt, "\r\n", "\n") || file.Format != "srt" {
		t.Errorf("Download got = %+v, %v, want = the subtitle", file, err)
	}
	for _, c := range []struct {
		link string
		kind provider.Kind
	}{
		{"/subs/1/fr.srt", provider.BadSubtitle},
		{"/subs/1/missing.srt", provider.BadSubtitle},
		{"/subs/5/en.srt", provider.ServiceUnavailable},
	} {
		if _, err := p.Download(t.Context(), nil, f.srv.URL+c.link); provider.KindOf(err) != c.kind {
			t.Errorf("Download of %s got = %v, want = %v", c.link, err, c.kind)
		}
	}
}

func TestCandidate(t *testing.T) {
	for _, c := range []struct {
		release string
		video   provider.Video
		want    bool
	}{
		{"The.Matrix.1999.720p", provider.Video{Title: "The Matrix", Year: 1999}, true},
		{"The.Matrix.2003.720p", provider.Video{Title: "The Matrix", Year: 1999}, false},
		{"Matrix.720p", provider.Video{Title: "The Matrix", Year: 1999}, false},
		{"Show.S01E02", provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}, true},
		{"Show.S02E02", provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}, false},
		{"Show.S01", provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}, true},
		{"Other.S01E02", provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}, false},
	} {
		if got := candidate(c.release, &c.video, ""); got != c.want {
			t.Errorf("candidate(%q) got = %v, want = %v", c.release, got, c.want)
		}
	}
}

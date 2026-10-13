package subs4series

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// solver answers captchas, failing the first ones.
type solver struct {
	fails  int
	solved atomic.Int32
	key    string
}

func (s *solver) ReCaptchaV2(_ context.Context, pageURL, siteKey string, invisible bool) (string, error) {
	s.key = siteKey
	if n := s.solved.Add(1); int(n) <= s.fails {
		return "", errors.New("unsolvable")
	}
	return "token", nil
}

func (s *solver) Image(context.Context, []byte) (string, error) { return "", errors.New("unsupported") }

// fakeSite is Subs4Series.
type fakeSite struct {
	srv       *httptest.Server
	searches  atomic.Int32
	antiBlock atomic.Int32
	captcha   bool
	expired   bool
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{captcha: true}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range []string{".hidden.srt", "show.srt", "readme.txt"} {
		fw, _ := w.Create(n)
		// "Γεια" in Windows-1253.
		_, _ = fw.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n\xc3\xe5\xe9\xe1 " + n + "\r\n"))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search_report.php", func(w http.ResponseWriter, r *http.Request) {
		f.searches.Add(1)
		if r.URL.Query().Get("search") != "the show" || r.URL.Query().Get("searchType") != "1" {
			return
		}
		_, _ = fmt.Fprintf(w, `<select name="Mov_sel"><option>pick</option>
			<option value="%[1]s/tv-series/the-show/12">The Show (2010)</option>
			<option value="%[1]s/tv-series/the-show-us/13">The Show US</option></select>`, f.srv.URL)
	})
	mux.HandleFunc("GET /tv-series/the-show/12/season-1/episode-2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<div id="dates_header_br"><table><tr><td><div><u>The Show</u><br>(2010) </div></td></tr></table></div>
			<table>
			<tr class="seeDark"><td><img src="/flags/el.gif"><b>The.Show.S01E02.720p.HDTV.x264-DIMENSION</b><a href="/getSub-1.html">get</a><b>nikos</b></td></tr>
			<tr class="seeMedium"><td><img src="/flags/en.gif"><b>The.Show.S01E02.WEB</b><a href="/getSub-2.html">get</a><b>john</b></td></tr>
			<tr class="seeDark"><td><img src="/flags/fr.gif"><b>French</b><a href="/getSub-3.html">get</a><b>jean</b></td></tr>
			</table>`)
	})
	mux.HandleFunc("GET /getSub-1.html", func(w http.ResponseWriter, r *http.Request) {
		if f.captcha {
			_, _ = fmt.Fprint(w, `<div class="g-recaptcha" data-sitekey="site-key"></div><form method="post" action="/dl.php?id=1"></form>`)
			return
		}
		_, _ = fmt.Fprint(w, `<a class="style55ws" href="/dl.php?id=1">download</a>`)
	})
	mux.HandleFunc("GET /includes/{page}", func(w http.ResponseWriter, r *http.Request) {
		if r.Referer() == f.srv.URL+"/getSub-1.html" {
			f.antiBlock.Add(1)
		}
	})
	mux.HandleFunc("POST /dl.php", func(w http.ResponseWriter, r *http.Request) {
		if f.expired || f.antiBlock.Load() < 2 || f.captcha && r.PostFormValue("g-recaptcha-response") != "token" ||
			r.PostFormValue("my_recaptcha_challenge_field") != "manual_challenge" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write(b.Bytes())
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite, captcha provider.CaptchaSolver) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler), Captcha: captcha}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func episode() *provider.Video {
	return &provider.Video{Episode: true, Series: "The Show", Year: 2010, Season: 1, Number: 2, ReleaseGroup: "LOL"}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, nil)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "el"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, fmt.Sprintf("%s %s %s %v %v", s.ID, s.Language.Tag, s.Uploader, s.ReleaseNames, s.Matches))
	}
	want := []string{
		f.srv.URL + "/getSub-1.html el nikos [The.Show.S01E02.720p.HDTV.x264-DIMENSION] [series year release_group]",
		f.srv.URL + "/getSub-2.html en john [The.Show.S01E02.WEB] [series year]",
	}
	if !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}
	if subs[0].PageLink != f.srv.URL+"/tv-series/the-show/12/season-1/episode-2" {
		t.Errorf("page link got = %s", subs[0].PageLink)
	}
	// The shows found are kept.
	if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "el"}}); err != nil || f.searches.Load() != 1 {
		t.Errorf("searches got = %d, %v, want = 1", f.searches.Load(), err)
	}
	for _, v := range []*provider.Video{
		{Episode: true, Series: "Another", Season: 1, Number: 2},
		{Title: "The Show"},
	} {
		if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "el"}}); err != nil || len(subs) != 0 {
			t.Errorf("%+v got = %v, %v, want none", v, subs, err)
		}
	}
}

func TestDownload(t *testing.T) {
	for _, c := range []struct {
		name    string
		captcha bool
		solver  *solver
		expired bool
		fails   bool
		want    provider.Kind
	}{
		{"captcha", true, &solver{fails: 2}, false, false, 0},
		{"no captcha", false, nil, false, false, 0},
		{"no solver", true, nil, false, true, provider.ConfigurationError},
		{"unsolvable", true, &solver{fails: 3}, false, true, provider.Other},
		{"expired", true, &solver{}, true, true, provider.ProviderError},
	} {
		f := newFake(t)
		f.captcha, f.expired = c.captcha, c.expired
		var cs provider.CaptchaSolver
		if c.solver != nil {
			cs = c.solver
		}
		p := newProvider(t, f, cs)
		file, err := p.Download(t.Context(), episode(), f.srv.URL+"/getSub-1.html")
		if c.fails {
			if err == nil || provider.KindOf(err) != c.want {
				t.Errorf("%s got = %v, want = %v", c.name, err, c.want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s got = %v", c.name, err)
			continue
		}
		if want := "1\n00:00:01,000 --> 00:00:02,000\nΓεια show.srt\n"; string(file.Data) != want || file.Format != "srt" {
			t.Errorf("%s got = %q, want = %q", c.name, file.Data, want)
		}
		if c.solver != nil && c.solver.key != "site-key" {
			t.Errorf("%s site key got = %q, want = site-key", c.name, c.solver.key)
		}
	}
}

func TestDecode(t *testing.T) {
	for in, want := range map[string]string{"\xc3\xe5\xe9\xe1": "Γεια", "Γεια": "Γεια", "plain": "plain"} {
		if got := string(sitekit.Decode([]byte(in), charmap.Windows1253)); got != want {
			t.Errorf("decode(%q) got = %q, want = %q", in, got, want)
		}
	}
	if !strings.Contains(Definition().Name, Name) {
		t.Errorf("name got = %s", Definition().Name)
	}
}

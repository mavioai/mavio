package pipocas

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fakeSite is Pipocas.tv with the account user/secret.
type fakeSite struct {
	srv      *httptest.Server
	searches []url.Values
	logins   atomic.Int32
	failOnce atomic.Bool
}

func zipped(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for i, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(f, "1\r\n00:00:01,000 --> 00:00:02,000\r\nLegenda %d\r\n", i+1)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// details are the subtitles' pages: release, hits, uploader and rating.
var details = map[string][4]string{
	"10": {"Show.S01E02.720p.HDTV-GRP", "250", "maria", "4 / 5"},
	"11": {"Show.S01E02.1080p.WEB", "900", "", "0 / 5"},
	"12": {"Show.S01.Complete", "30", "joao", "3 / 5"},
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	pack := zipped(t, "Show.S01E01.srt", "Show.S01E02.srt")
	mux := http.NewServeMux()
	loggedIn := func(r *http.Request) bool {
		c, err := r.Cookie("session")
		return err == nil && c.Value == "ok"
	}
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><head><meta name="csrf-token" content="tok"></head><body>Cria uma conta</body></html>`)
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if r.PostFormValue("_token") != "tok" || r.PostFormValue("username") != "user" || r.PostFormValue("password") != "secret" {
			_, _ = fmt.Fprint(w, "Cria uma conta")
			return
		}
		f.logins.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "ok", Path: "/"})
		_, _ = fmt.Fprint(w, "Bem-vindo")
	})
	mux.HandleFunc("GET /legendas", func(w http.ResponseWriter, r *http.Request) {
		if !loggedIn(r) {
			_, _ = fmt.Fprint(w, "Cria uma conta")
			return
		}
		f.searches = append(f.searches, r.URL.Query())
		if r.URL.Query().Get("l") != "portugues" { //nolint:misspell // The site's name.
			return
		}
		_, _ = fmt.Fprint(w, `<a class="text-dark no-decoration" href="/legendas/info/10">a</a>
			<a class="text-dark no-decoration" href="/legendas/info/11">b</a>
			<a class="text-dark no-decoration" href="/legendas/info/12">c</a>
			<a class="text-dark no-decoration" href="/legendas/info/13">no download</a>
			<a class="text-dark no-decoration" href="/outra">d</a>`)
	})
	mux.HandleFunc("GET /legendas/info/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, ok := details[r.PathValue("id")]
		if !ok {
			_, _ = fmt.Fprint(w, `<h3 class="title"><span class="font-normal">gone</span></h3>`)
			return
		}
		uploader := ""
		if d[2] != "" {
			uploader = fmt.Sprintf(`<span style="color: #FF0000">%s</span>`, d[2])
		}
		_, _ = fmt.Fprintf(w, `<h3 class="title">Show <span class="font-normal"> %s </span></h3>
			<span class="hits hits-pd"><div>%s</div></span>%s<h2 class="mt-3 text-center">%s</h2>
			<a href="/legendas/download/%s">Download</a>`, d[0], d[1], uploader, d[3], r.PathValue("id"))
	})
	mux.HandleFunc("GET /legendas/download/{id}", func(w http.ResponseWriter, r *http.Request) {
		if f.failOnce.CompareAndSwap(true, false) {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if !loggedIn(r) {
			_, _ = fmt.Fprint(w, "Cria uma conta")
			return
		}
		switch r.PathValue("id") {
		case "10":
			_, _ = w.Write(pack)
		case "11":
			_, _ = fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nplain\n")
		case "12":
			_, _ = w.Write(zipped(t, "readme.txt"))
		case "13":
			_, _ = fmt.Fprint(w, "<html>not a subtitle</html>")
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
	p.(*Provider).delay = 0
	return p.(*Provider)
}

const account = `{"username":"user","password":"secret"}`

func episode() *provider.Video {
	return &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, Year: 2020}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, account)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "pt"}, {Tag: "en"}, {Tag: "fr"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, fmt.Sprintf("%s %s %d %s %v %v", s.ID, s.Language.Tag, s.Downloads, s.Uploader, s.ReleaseNames, s.Matches))
	}
	// By stars, then hits: (4+2.5)/2 rounds to 3, (3+0.3)/2 to 2, and no
	// rating is no stars.
	want := []string{
		"pipocas_10 pt 250 maria [Show.S01E02.720p.HDTV-GRP] [series season episode]",
		"pipocas_12 pt 30 joao [Show.S01.Complete] [series season]",
		"pipocas_11 pt 900 pipocas-bot [Show.S01E02.1080p.WEB] [series season episode]",
	}
	if !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}
	if subs[0].PageLink != f.srv.URL+"/legendas/download/10" {
		t.Errorf("page link got = %s", subs[0].PageLink)
	}
	if len(f.searches) != 2 || f.searches[0].Get("s") != "Show S01E02" || f.searches[0].Get("t") != "rel" || f.searches[1].Get("l") != "ingles" {
		t.Errorf("searches got = %v", f.searches)
	}
}

func TestQuery(t *testing.T) {
	for _, c := range []struct {
		v    provider.Video
		want string
	}{
		{provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}, "Show S01E02"},
		{provider.Video{Episode: true, Series: "Show"}, "Show"},
		{provider.Video{Title: "Movie"}, "Movie"},
		{provider.Video{FileName: `C:\media\Movie.2010.mkv`}, "Movie.2010"},
		{provider.Video{}, ""},
	} {
		if got := query(&c.v); got != c.want {
			t.Errorf("query(%+v) got = %q, want = %q", c.v, got, c.want)
		}
	}
	if got := matches(&provider.Video{Title: "The Movie", Year: 2010}, "the.movie.2010"); !slices.Equal(got, []string{"year"}) {
		t.Errorf("movie matches got = %v, want = [year]", got)
	}
	if got := matches(&provider.Video{Title: "Movie", Year: 2010}, "MOVIE 2010"); !slices.Equal(got, []string{"title", "year"}) {
		t.Errorf("movie matches got = %v, want = [title year]", got)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, account)
	file, err := p.Download(t.Context(), episode(), "pipocas_10")
	if err != nil || !strings.Contains(string(file.Data), "Legenda 2") || file.Format != "srt" {
		t.Errorf("pack got = %q, %v, want the second episode", file.Data, err)
	}
	file, err = p.Download(t.Context(), episode(), "pipocas_11")
	if err != nil || !strings.Contains(string(file.Data), "plain") {
		t.Errorf("subtitle got = %q, %v", file.Data, err)
	}
	for id, want := range map[string]provider.Kind{"pipocas_12": provider.ServiceUnavailable, "pipocas_13": provider.ProviderError} {
		if _, err := p.Download(t.Context(), episode(), id); provider.KindOf(err) != want {
			t.Errorf("%s got = %v, want = %v", id, err, want)
		}
	}
	// A failure logs in again and tries once more.
	f.failOnce.Store(true)
	if _, err := p.Download(t.Context(), episode(), "pipocas_10"); err != nil || f.logins.Load() != 2 {
		t.Errorf("retry got = %v, %d logins, want = 2", err, f.logins.Load())
	}
}

func TestLogin(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"username":"user","password":"wrong"}`)
	if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "pt"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("wrong password got = %v, want = AuthenticationError", err)
	}
	for _, s := range []string{``, `{"username":"user"}`, `{"password":"x"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("%q got = %v, want = ConfigurationError", s, err)
		}
	}
}

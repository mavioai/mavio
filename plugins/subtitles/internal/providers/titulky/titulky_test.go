package titulky

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

func zipped(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for i, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(f, "1\r\n00:00:01,000 --> 00:00:02,000\r\nTitulek %d\r\n", i+1)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fakeSite is premium.titulky.com with the V.I.P. account user/secret and
// the account free/secret without V.I.P.
type fakeSite struct {
	srv     *httptest.Server
	logins  atomic.Int32
	details atomic.Int32
	expire  atomic.Bool
}

func row(class, id, release, flag, uploader string) string {
	return fmt.Sprintf(`<div class="row %s"><div><a href="./?action=detail&id=%s">%s</a></div><div><img src="img/%s.png"></div><div>1</div><div>2</div><div>3</div><div> %s </div></div>`,
		class, id, release, flag, uploader)
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{$}", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.PostFormValue("LoginName") == "user" && r.PostFormValue("LoginPassword") == "secret":
			f.logins.Add(1)
			f.expire.Store(false)
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "ok", Path: "/"})
			http.Redirect(w, r, "/?msg_type=i&msg="+url.QueryEscape("Přihlášení proběhlo úspěšně"), http.StatusFound)
		case r.PostFormValue("LoginName") == "free":
			http.Redirect(w, r, "/?msg_type=i&msg="+url.QueryEscape("Přístup je omezené"), http.StatusFound)
		default:
			http.Redirect(w, r, "/?msg_type=e&msg="+url.QueryEscape("Špatné heslo"), http.StatusFound)
		}
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if c, err := r.Cookie("PHPSESSID"); err != nil || c.Value != "ok" || f.expire.Load() {
			http.Redirect(w, r, "/?msg_type=e&msg="+url.QueryEscape("Prosím přihlašte se"), http.StatusFound)
			return
		}
		switch q.Get("action") {
		case "serial":
			switch q.Get("id") {
			case "0000009":
				// The site's own ID.
				http.Redirect(w, r, "/?action=serial&step="+q.Get("step")+"&id=55x", http.StatusFound)
			case "55x":
				_, _ = fmt.Fprint(w, `<form class="cloudForm"><div class="row"><h5>1.</h5></div>`,
					row("pbl1", "101", "Show.S01E01.WEB", "flag-CZ", "karel"),
					`<div class="row"><h5>2.</h5></div>`,
					row("pbl1", "102", "Show.S01E02.HDTV", "flag-CZ", "karel"),
					row("pbl0", "103", "???", "flag-SK", "jozef"),
					row("pbl0", "104", "Both", "flag-CZ flag-SK", "x"),
					`<div class="row"><a href="/about">about</a></div></form>`)
			case "0000001":
				_, _ = fmt.Fprint(w, `<form class="cloudForm"><div class="row"><h5></h5></div>`,
					row("pbl1", "201", "Movie.2010.1080p", "flag-CZ", "eva"), `</form>`)
			case "0000002":
				http.Redirect(w, r, "/?msg_type=e&msg=Nenalezeno", http.StatusFound)
			default:
				_, _ = fmt.Fprint(w, "<html>nic</html>")
			}
		case "detail":
			f.details.Add(1)
			fps := map[string]string{"102": "23,976 fps", "103": "25,000 fps"}[q.Get("id")]
			_, _ = fmt.Fprintf(w, `<div class="ulozil"><img src="img/ico/user.png"> karel</div><div class="ulozil"><img src="img/ico/Movieroll.png"> %s</div>`, fps)
		}
	})
	mux.HandleFunc("GET /download.php", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("id") {
		case "102":
			_, _ = w.Write(zipped(t, "Show.S01E01.srt", "Show.S01E02.srt"))
		case "103":
			_, _ = w.Write(zipped(t, "limit.txt"))
		case "104":
			_, _ = w.Write(zipped(t, "a.txt", "b.txt"))
		case "429":
			w.WriteHeader(http.StatusTooManyRequests)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite, dir, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler), DataDir: dir}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

const account = `{"username":"user","password":"secret"}`

func episode() *provider.Video {
	return &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0000009"}, FrameRate: 25}
}

func describe(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, fmt.Sprintf("%s %s %s %v %v", s.ID, s.Language.Tag, s.Uploader, s.ReleaseNames, s.Matches))
	}
	return out
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, t.TempDir(), account)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}, {Tag: "sk"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"102 cs karel [Show.S01E02.HDTV] [series_imdb_id series year season episode]",
		"103 sk jozef [] [series_imdb_id series year season episode]",
	}
	if got := describe(subs); !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}
	if subs[0].PageLink != f.srv.URL+"/?action=detail&id=102" {
		t.Errorf("page link got = %s", subs[0].PageLink)
	}

	movie := &provider.Video{Title: "Movie", ExternalIDs: map[string]string{"imdb": "tt0000001"}}
	subs, err = p.Search(t.Context(), movie, []provider.Language{{Tag: "cs"}})
	if got := describe(subs); err != nil || !slices.Equal(got, []string{"201 cs eva [Movie.2010.1080p] [imdb_id title year]"}) {
		t.Errorf("movie got = %q, %v", got, err)
	}

	for _, c := range []struct {
		name string
		v    *provider.Video
		want provider.Kind
	}{
		{"no IMDb ID", &provider.Video{Title: "Movie"}, -1},
		{"not found", &provider.Video{ExternalIDs: map[string]string{"imdb": "tt0000003"}}, -1},
		{"error page", &provider.Video{ExternalIDs: map[string]string{"imdb": "tt0000002"}}, provider.ProviderError},
	} {
		subs, err := p.Search(t.Context(), c.v, []provider.Language{{Tag: "cs"}})
		if c.want < 0 && (err != nil || len(subs) != 0) || c.want >= 0 && provider.KindOf(err) != c.want {
			t.Errorf("%s got = %v, %v, want = %v", c.name, subs, err, c.want)
		}
	}
}

func TestSettings(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, t.TempDir(), `{"username":"user","password":"secret","approved_only":true,"skip_wrong_fps":true}`)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}, {Tag: "sk"}})
	// Only the approved, of another frame rate: no matches.
	if got := describe(subs); err != nil || !slices.Equal(got, []string{"102 cs karel [Show.S01E02.HDTV] []"}) {
		t.Errorf("approved only got = %q, %v", got, err)
	}
	p = newProvider(t, f, t.TempDir(), `{"username":"user","password":"secret","skip_wrong_fps":true}`)
	subs, err = p.Search(t.Context(), episode(), []provider.Language{{Tag: "sk"}})
	if got := describe(subs); err != nil || !slices.Equal(got, []string{"103 sk jozef [] [series_imdb_id series year season episode]"}) {
		t.Errorf("same frame rate got = %q, %v", got, err)
	}
	// Frame rates are kept.
	if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "sk"}}); err != nil || f.details.Load() != 3 {
		t.Errorf("details got = %d, %v, want = 3", f.details.Load(), err)
	}
}

func TestLogin(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	if _, err := newProvider(t, f, dir, account).Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}}); err != nil {
		t.Fatal(err)
	}
	// The kept cookies are used after a restart, and expired ones log in
	// again.
	p := newProvider(t, f, dir, account)
	if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}}); err != nil || f.logins.Load() != 1 {
		t.Errorf("kept cookies got = %d logins, %v, want = 1", f.logins.Load(), err)
	}
	f.expire.Store(true)
	if subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}}); err != nil || len(subs) != 1 || f.logins.Load() != 2 {
		t.Errorf("expired cookies got = %d logins, %v, %v, want = 2", f.logins.Load(), subs, err)
	}
	for _, settings := range []string{`{"username":"user","password":"wrong"}`, `{"username":"free","password":"secret"}`} {
		if _, err := newProvider(t, f, "", settings).Search(t.Context(), episode(), []provider.Language{{Tag: "cs"}}); provider.KindOf(err) != provider.AuthenticationError {
			t.Errorf("%s got = %v, want = AuthenticationError", settings, err)
		}
	}
	for _, s := range []string{``, `{"username":"user"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("%q got = %v, want = ConfigurationError", s, err)
		}
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, "", account)
	file, err := p.Download(t.Context(), episode(), "102")
	if err != nil || !strings.Contains(string(file.Data), "Titulek 2") || file.Format != "srt" {
		t.Errorf("download got = %q, %v, want the second episode", file.Data, err)
	}
	for id, want := range map[string]provider.Kind{
		"103": provider.DownloadLimitExceeded, "104": provider.BadSubtitle, "105": provider.ProviderError, "429": provider.TooManyRequests,
	} {
		if _, err := p.Download(t.Context(), episode(), id); provider.KindOf(err) != want {
			t.Errorf("%s got = %v, want = %v", id, err, want)
		}
	}
	d := Definition()
	if got := provider.Throttle(&d, provider.DownloadLimitExceeded, time.Date(2024, 7, 1, 20, 0, 0, 0, time.UTC)); got != 2*time.Hour {
		t.Errorf("download limit throttle got = %v, want = 2h", got)
	}
}

func TestQuote(t *testing.T) {
	if got, want := quote("https://premium.titulky.com/?action=detail&id=1"), "https%3A//premium.titulky.com/%3Faction%3Ddetail%26id%3D1"; got != want {
		t.Errorf("quote got = %q, want = %q", got, want)
	}
}

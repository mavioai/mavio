package addic7ed

import (
	"context"
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

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "addic_7ed.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_episode": "TestSearchEpisode",
			"test_list_subtitles_movie":   "TestSearchMovie",
		},
	})
}

// fakeSite is Addic7ed with an account whose login asks for a reCAPTCHA.
type fakeSite struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	logins   int
	shows    int
	limitHit bool
	// status answers the next request of a path with a status.
	status map[string]int
}

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{t: t, status: map[string]int{}}
	mux := http.NewServeMux()
	loggedIn := func(r *http.Request) bool {
		c, err := r.Cookie("wikisubtitlesuser")
		return err == nil && c.Value == "u1"
	}
	mux.HandleFunc("GET /panel.php", func(w http.ResponseWriter, r *http.Request) {
		if !loggedIn(r) {
			http.Redirect(w, r, "/login.php", http.StatusFound)
			return
		}
		_, _ = fmt.Fprint(w, "<html>panel</html>")
	})
	mux.HandleFunc("GET /login.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<form><div class="g-recaptcha" data-sitekey="site-key"></div></form>`)
	})
	mux.HandleFunc("POST /dologin.php", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("g-recaptcha-response") != "token" || r.FormValue("remember") != "true" {
			_, _ = fmt.Fprint(w, "<html>captcha</html>")
			return
		}
		if r.FormValue("username") != "me" || r.FormValue("password") != "pw" {
			_, _ = fmt.Fprint(w, "<html>Wrong password</html>")
			return
		}
		f.mu.Lock()
		f.logins++
		f.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "wikisubtitlesuser", Value: "u1", Path: "/"})
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /shows.php", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.shows++
		f.mu.Unlock()
		_, _ = fmt.Fprint(w, `<html><body><table><tr>
			<td class="version"><h3><a href="/show/1">Breaking Bad</a></h3></td>
			<td class="vr"><h3><a href="/show/2">Mr. Robot (2015)</a></h3></td>
			<td class="version"><h3><a href="/show/x">Broken</a></h3></td>
			</tr></table></body></html>`)
	})
	mux.HandleFunc("GET /ajax_loadShow.php", func(w http.ResponseWriter, r *http.Request) {
		if f.answer(w, "season") {
			return
		}
		if !loggedIn(r) || r.Header.Get("X-Requested-With") != "XMLHttpRequest" || r.URL.Query().Get("langs") != "|" {
			f.t.Errorf("season request %v %v", r.URL, r.Header)
		}
		if r.URL.Query().Get("show") != "1" || r.URL.Query().Get("season") != "1" {
			return
		}
		row := func(ep int, lang, version, status, hi, dl string) string {
			return fmt.Sprintf(`<tr class="epeven completed"><td>1</td><td>%d</td><td><a href="/serie/Breaking_Bad/1/%d/Pilot">Pilot</a></td>
				<td>%s</td><td>%s</td><td>%s</td><td>%s</td><td></td><td></td><td><a href="%s">Download</a></td></tr>`, ep, ep, lang, version, status, hi, dl)
		}
		_, _ = fmt.Fprint(w, "<table><tbody>"+
			row(1, "English", "720p.BluRay+REWARD", "Completed", "", "/original/1/0")+
			row(1, "English", "DEMAND", "Completed", "✔", "/original/1/1")+
			row(1, "French", "DEMAND", "Completed", "", "/original/1/2")+
			row(1, "English", "DEMAND", "38.5% Completed", "", "/original/1/3")+
			row(1, "Klingon", "DEMAND", "Completed", "", "/original/1/4")+
			row(2, "English", "DEMAND", "Completed", "", "/original/1/5")+
			"</tbody></table>")
	})
	mux.HandleFunc("GET /search.php", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<table class="tabel">
			<tr><td><a href="movie/99">Dune (1984)</a></td></tr>
			<tr><td><a href="movie/100">Dune (2021)</a></td></tr>
			<tr><td><a href="show/5">Dune Show (2021)</a></td></tr></table>`)
	})
	mux.HandleFunc("GET /movie/100", func(w http.ResponseWriter, r *http.Request) {
		table := func(lang, hi, dl string) string {
			return fmt.Sprintf(`<table width="100%%" border="0" align="center" class="tabel95">
<tr>
<td class="NewsTitle"><img src="/images/folder_page.png">Version WEB-DL.NTb, 0.00 MBs</td><td>a<b>b</b>c<b>d</b>e<b>f</b>g<b>h</b><a href="/user/1"> uploader1 </a></td></tr>
<tr><td>spacer</td></tr><tr><td></td><td></td><td></td><td></td><td class="language">%s</td><td></td><td><b>Completed</b></td><td></td><td>x<b>y</b>z<a href="%s">Download</a></td></tr>
<tr><td></td><td>x<img src="/images/%s"></td></tr>
</table>`, lang, dl, hi)
		}
		_, _ = fmt.Fprint(w, table("English", "hi.jpg", "/original/9/0")+table("English", "none.jpg", "/original/9/1")+
			table("Greek", "none.jpg", "/original/9/2"))
	})
	mux.HandleFunc("GET /original/{id}/{n}", func(w http.ResponseWriter, r *http.Request) {
		if f.answer(w, "download") {
			return
		}
		f.mu.Lock()
		limit := f.limitHit
		f.mu.Unlock()
		if limit {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, "<html>Daily Download count exceeded</html>")
			return
		}
		if r.Header.Get("Referer") == "" {
			f.t.Errorf("download without a referer")
		}
		w.Header().Set("Content-Type", "text/srt")
		_, _ = fmt.Fprint(w, srt)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSite) answer(w http.ResponseWriter, path string) bool {
	f.mu.Lock()
	code, ok := f.status[path]
	delete(f.status, path)
	f.mu.Unlock()
	if ok {
		w.WriteHeader(code)
	}
	return ok
}

type solver struct{ calls int }

func (s *solver) ReCaptchaV2(_ context.Context, pageURL, siteKey string, invisible bool) (string, error) {
	s.calls++
	if !strings.HasSuffix(pageURL, "/login.php") || siteKey != "site-key" || !invisible {
		return "", fmt.Errorf("unexpected captcha %s %s %v", pageURL, siteKey, invisible)
	}
	return "token", nil
}

func (s *solver) Image(context.Context, []byte) (string, error) { return "", fmt.Errorf("no image") }

func newProvider(t *testing.T, f *fakeSite, settings string, captcha provider.CaptchaSolver, dir string) *Provider {
	t.Helper()
	env := provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler), DataDir: dir}
	if captcha != nil {
		env.Captcha = captcha
	}
	p, err := New(env, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).pause = 0
	return p.(*Provider)
}

func episode() *provider.Video {
	return &provider.Video{Episode: true, Series: "Breaking Bad", Title: "Pilot", Season: 1, Number: 1, Year: 2008}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	captcha := &solver{}
	p := newProvider(t, f, `{"username":"me","password":"pw"}`, captcha, dir)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	// Incomplete subtitles, other languages and episodes are left out.
	if got := releases(subs); !slices.Equal(got, []string{"720p.BluRay,REWARD", "DEMAND"}) {
		t.Fatalf("releases got = %v, want = [720p.BluRay,REWARD DEMAND]", got)
	}
	if got, want := subs[0].Matches, []string{"series", "season", "episode", "year", "title"}; !slices.Equal(got, want) {
		t.Errorf("matches got = %v, want = %v", got, want)
	}
	if subs[0].Language.HearingImpaired || !subs[1].Language.HearingImpaired {
		t.Errorf("hearing impaired got = %v, %v, want = false, true", subs[0].Language.HearingImpaired, subs[1].Language.HearingImpaired)
	}
	if want := f.srv.URL + "/serie/Breaking_Bad/1/1/Pilot"; subs[0].PageLink != want {
		t.Errorf("page link got = %s, want = %s", subs[0].PageLink, want)
	}
	if captcha.calls != 1 || f.logins != 1 {
		t.Errorf("captchas, logins got = %d, %d, want = 1, 1", captcha.calls, f.logins)
	}

	// A new provider re-uses the kept login.
	p2 := newProvider(t, f, `{"username":"me","password":"pw"}`, nil, dir)
	alt := episode()
	alt.Series, alt.AlternativeSeries, alt.Year = "Unknown", []string{"Breaking Bad"}, 0
	subs, err = p2.Search(t.Context(), alt, []provider.Language{{Tag: "en"}, {Tag: "fr"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 || f.logins != 1 {
		t.Errorf("subtitles, logins got = %d, %d, want = 3, 1", len(subs), f.logins)
	}
	if slices.Contains(subs[0].Matches, "year") {
		t.Errorf("matches got = %v, want no year", subs[0].Matches)
	}
	// The show IDs are fetched once.
	if f.shows != 2 {
		t.Errorf("show fetches got = %d, want = 2", f.shows)
	}

	file, err := p2.Download(t.Context(), episode(), subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.ReplaceAll(srt, "\r\n", "\n"); string(file.Data) != want || file.Format != "srt" {
		t.Errorf("download got = %q, %s, want = %q, srt", file.Data, file.Format, want)
	}
}

func TestShowID(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1; PHPSESSID=abc"}`, nil, "")
	s, err := p.session(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		series string
		year   int
		want   int
	}{
		{"Breaking Bad", 2008, 1},
		// Shows named with their year are found only with it, as Bazarr's.
		{"Mr. Robot", 0, 0},
		{"Mr Robot", 2015, 2},
		{"Broken", 0, 0},
	} {
		got, err := p.showID(t.Context(), s, c.series, c.year, false)
		if err != nil || got != c.want {
			t.Errorf("showID(%s, %d) got = %d, %v, want = %d", c.series, c.year, got, err, c.want)
		}
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1"}`, nil, "")
	v := &provider.Video{Title: "Dune", Year: 2021}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("subtitles got = %d, want = 2", len(subs))
	}
	s := subs[0]
	if !s.Language.HearingImpaired || subs[1].Language.HearingImpaired {
		t.Errorf("hearing impaired got = %v, %v, want = true, false", s.Language.HearingImpaired, subs[1].Language.HearingImpaired)
	}
	if !slices.Equal(s.ReleaseNames, []string{"WEB-DL.NTb"}) || s.Uploader != "uploader1" || !slices.Equal(s.Matches, []string{"year", "title"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	if q, _ := url.ParseQuery(s.ID); q.Get("dl") != "original/9/0" || q.Get("ref") != f.srv.URL+"/movie/100" {
		t.Errorf("id got = %s", s.ID)
	}
}

func TestDownloadLimit(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	p := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1"}`, nil, dir)
	now := time.Now()
	p.env.Now = func() time.Time { return now }
	id := url.Values{"dl": {"original/1/0"}, "ref": {f.srv.URL + "/x"}}.Encode()
	for i := range 40 {
		if _, err := p.Download(t.Context(), nil, id); err != nil {
			t.Fatalf("download %d: %v", i, err)
		}
	}
	if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != provider.DownloadLimitExceeded {
		t.Errorf("download 41 got = %v, want = DownloadLimitExceeded", err)
	}
	// The count is kept across restarts, and expires after a day.
	p2 := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1","vip":true}`, nil, dir)
	p2.env.Now = func() time.Time { return now }
	if _, err := p2.Download(t.Context(), nil, id); err != nil {
		t.Errorf("VIP download 41 got = %v, want = nil", err)
	}
	p3 := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1"}`, nil, dir)
	p3.env.Now = func() time.Time { return now.Add(25 * time.Hour) }
	if _, err := p3.Download(t.Context(), nil, id); err != nil {
		t.Errorf("download a day later got = %v, want = nil", err)
	}
	// The site answers a page once its own limit is reached.
	f.limitHit = true
	if _, err := p3.Download(t.Context(), nil, id); provider.KindOf(err) != provider.DownloadLimitExceeded {
		t.Errorf("download past the site's limit got = %v, want = DownloadLimitExceeded", err)
	}
}

func TestFailures(t *testing.T) {
	f := newFake(t)
	if _, err := newProvider(t, f, `{"cookies":"wikisubtitlesuser=expired"}`, nil, "").Search(t.Context(), episode(), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("expired cookies got = %v, want = AuthenticationError", err)
	}
	if _, err := newProvider(t, f, `{"username":"me","password":"pw"}`, nil, "").Search(t.Context(), episode(), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("login without a captcha solver got = %v, want = ConfigurationError", err)
	}
	if _, err := newProvider(t, f, `{"username":"me","password":"bad"}`, &solver{}, "").Search(t.Context(), episode(), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("wrong password got = %v, want = AuthenticationError", err)
	}
	for _, c := range []struct {
		code int
		kind provider.Kind
	}{
		{http.StatusNotModified, provider.TooManyRequests},
		{http.StatusTooManyRequests, provider.TooManyRequests},
	} {
		p := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1"}`, nil, "")
		f.status["season"] = c.code
		if _, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != c.kind {
			t.Errorf("season %d got = %v, want = %v", c.code, err, c.kind)
		}
	}
	// Failures reaching the site are retried once after logging in again.
	p := newProvider(t, f, `{"cookies":"wikisubtitlesuser=u1"}`, nil, "")
	f.status["season"] = http.StatusBadGateway
	if subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 2 {
		t.Errorf("search after a 502 got = %d, %v, want = 2, nil", len(subs), err)
	}
	if _, err := p.Download(t.Context(), nil, "%zz"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("malformed ID got = %v, want = BadSubtitle", err)
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{}`, `{"username":"me"}`, `{"password":"pw","cookies":"a=b"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = ConfigurationError", s, err)
		}
	}
	for tag, name := range map[string]string{"pt-BR": "Portuguese (Brazilian)", "sr-Cyrl": "Serbian (Cyrillic)", "en": "English", "fr-CA": "French (Canadian)", "ca": "Català", "": "Klingon"} {
		if got := languageTag(name); got != tag {
			t.Errorf("languageTag(%s) got = %q, want = %q", name, got, tag)
		}
	}
}

func releases(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ReleaseNames...)
	}
	return out
}

package karagarga

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "karagarga.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_search_movie": func(t *testing.T, args map[string]json.RawMessage) {
				f := newFake(t)
				p := newProvider(t, f, `{"username":"user","password":"secret"}`)
				subs, err := p.searchMovie(t.Context(), ported.Arg[string](t, args, "title"), ported.Arg[int](t, args, "year"), provider.Language{Tag: "en"})
				if want := ported.Arg[int](t, args, "expected_subs_len"); err != nil || len(subs) < want {
					t.Errorf("subtitles got = %d, %v, want = at least %d", len(subs), err, want)
				}
			},
		},
		Facts: map[string]string{
			"test_init_raises_configuration_error": "TestNew",
			"test_login":                           "TestLogin",
			"test_list_subtitles":                  "TestSearch",
			"test_download_subtitle":               "TestDownload",
			"test_subtitle_get_matches":            "TestSearch",
		},
	})
}

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

// fakeSite is the tracker and its forum, with the account user/secret,
// whose forum account is forum/forum-secret when forum is set.
type fakeSite struct {
	srv   *httptest.Server
	forum bool
	gets  atomic.Int32
}

// movies are the requests of the tracker: title, year, language, status
// and forum's thread.
var movies = []struct{ title, lang, status, thread string }{
	{"Paradise (2009)", "English", "approved", "1"},
	{"Paradise (2009) [Director's cut]", "English", "approved", "2"},
	{"Paradise (2013)", "English", "approved", "3"},
	{"Paradise (2009)", "French", "approved", "4"},
	{"Paradise (2009)", "English", "pending", "5"},
	{"Her Way (2021)", "English", "approved", "6"},
	{"Batrachian's Ballad (2016)", "English, Spanish", "approved", "7"},
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /takelogin.php", func(w http.ResponseWriter, r *http.Request) {
		if r.PostFormValue("username") == "user" && r.PostFormValue("password") == "secret" {
			http.SetCookie(w, &http.Cookie{Name: "pass", Value: "p", Path: "/"})
		}
	})
	mux.HandleFunc("POST /index.php", func(w http.ResponseWriter, r *http.Request) {
		user, pass := "user", "secret"
		if f.forum {
			user, pass = "forum", "forum-secret"
		}
		if r.URL.Query().Get("do") != "process" || r.PostFormValue("auth_key") != authKey ||
			r.PostFormValue("ips_username") != user || r.PostFormValue("ips_password") != pass {
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "s", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "pass_hash", Value: "h", Path: "/"})
	})
	mux.HandleFunc("GET /login.php", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /pots.php", func(w http.ResponseWriter, r *http.Request) {
		f.gets.Add(1)
		if _, err := r.Cookie("pass"); err != nil {
			http.Redirect(w, r, "/login.php", http.StatusFound)
			return
		}
		var rows strings.Builder
		rows.WriteString("<tr><td>header</td></tr>")
		for _, m := range movies {
			if r.URL.Query().Get("status") != "completed" || !strings.HasPrefix(m.title, r.URL.Query().Get("search")) {
				continue
			}
			fmt.Fprintf(&rows, `<tr><td>1</td><td>%s</td><td></td><td></td><td></td><td>%s</td><td></td><td></td><td></td>
				<td><a href="%s/index.php?showtopic=%s" title="forum.karagarga.in">%s</a></td><td></td></tr>`, m.title, m.lang, f.srv.URL, m.thread, m.status)
		}
		_, _ = fmt.Fprintf(w, `<html><body><table cellspacing="5">%s</table></body></html>`, rows.String())
	})
	mux.HandleFunc("GET /index.php", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("section") == "attach" {
			_, _ = fmt.Fprint(w, srt)
			return
		}
		topic := r.URL.Query().Get("showtopic")
		attach := func(id, name string, downloads int) string {
			return fmt.Sprintf(`<a href="%s/index.php?app=core&module=attach&section=attach&attach_id=%s"><strong>%s</strong></a> <span class="desc lighter">%d downloads</span>`,
				f.srv.URL, id, name, downloads)
		}
		_, _ = fmt.Fprintf(w, `<html><body>
			<div class="post entry-content"><p>%s</p><ul><li class="attachment">%s</li></ul><p>no attachment</p>
				<p><a href="x"><strong>bad</strong></a> <span class="desc lighter">many downloads</span></p></div>
			<div class="post entry-content"><p>%s</p></div>
			</body></html>`,
			attach(topic+"1", "Paradise.2009."+topic+".srt", 5), attach(topic+"2", "Paradise.2009."+topic+".DVD.srt", 12),
			attach(topic+"1", "Paradise.2009."+topic+".srt", 5))
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

func TestNew(t *testing.T) {
	for _, settings := range []string{``, `{"username":"","password":""}`, `{"username":"user"}`, `{"password":"secret"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(settings)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("%q got = %v, want = ConfigurationError", settings, err)
		}
	}
}

func TestLogin(t *testing.T) {
	for _, c := range []struct {
		name, settings string
		forum          bool
		want           provider.Kind
	}{
		{"tracker's account", `{"username":"user","password":"secret"}`, false, 0},
		{"forum's account", `{"username":"user","password":"secret","f_username":"forum","f_password":"forum-secret"}`, true, 0},
		{"wrong password", `{"username":"user","password":"wrong"}`, false, provider.AuthenticationError},
		{"wrong forum password", `{"username":"user","password":"secret"}`, true, provider.AuthenticationError},
	} {
		f := newFake(t)
		f.forum = c.forum
		p := newProvider(t, f, c.settings)
		_, err := p.session(t.Context())
		if got := provider.KindOf(err); (err == nil) != (c.want == 0) || (err != nil && got != c.want) {
			t.Errorf("%s got = %v, want = %v", c.name, err, c.want)
		}
	}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"username":"user","password":"secret"}`)
	v := &provider.Video{Title: "Paradise", Year: 2009}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	// Posts are counted apart, as Bazarr does. The most downloaded of the approved English requests of 2009.
	if len(subs) != 1 {
		t.Fatalf("subtitles got = %d, want = 1", len(subs))
	}
	s := subs[0]
	want := f.srv.URL + "/index.php?app=core&module=attach&section=attach&attach_id=12"
	if s.ID != want || s.PageLink != want || s.Downloads != 12 || s.Language.Tag != "en" ||
		s.ReleaseNames[0] != "Paradise.2009.1.DVD.srt" || strings.Join(s.Matches, ",") != "title,year" {
		t.Errorf("subtitle got = %+v, want = %s", s, want)
	}
	all, err := p.searchMovie(t.Context(), "Paradise", 2009, provider.Language{Tag: "en"})
	if err != nil || len(all) != 6 {
		t.Errorf("all subtitles got = %d, %v, want = 6", len(all), err)
	}
	// Pages are kept a week.
	if n := f.gets.Load(); n != 1 {
		t.Errorf("searches got = %d, want = 1", n)
	}

	for _, c := range []struct {
		name  string
		v     *provider.Video
		langs []provider.Language
	}{
		{"another year", &provider.Video{Title: "Paradise", Year: 2001}, []provider.Language{{Tag: "en"}}},
		{"unknown movie", &provider.Video{Title: "Nothing", Year: 2009}, []provider.Language{{Tag: "en"}}},
		{"episode", &provider.Video{Episode: true, Title: "Paradise", Year: 2009}, []provider.Language{{Tag: "en"}}},
		{"another language", v, []provider.Language{{Tag: "fr"}}},
	} {
		if subs, err := p.Search(t.Context(), c.v, c.langs); err != nil || len(subs) != 0 {
			t.Errorf("%s got = %v, %v, want none", c.name, subs, err)
		}
	}
}

func TestLoggedOut(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"username":"user","password":"secret"}`)
	if _, err := p.session(t.Context()); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.sess.Jar().SetCookies(u, []*http.Cookie{{Name: "pass", Value: "", Path: "/", MaxAge: -1}})
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Paradise", Year: 2009}, []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("logged out got = %v, want = AuthenticationError", err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"username":"user","password":"secret"}`)
	file, err := p.Download(t.Context(), &provider.Video{Title: "Paradise"}, f.srv.URL+"/index.php?app=core&module=attach&section=attach&attach_id=49324")
	if err != nil || file.Format != "srt" || !strings.Contains(string(file.Data), "Hello") {
		t.Errorf("download got = %+v, %v, want an srt", file, err)
	}
}

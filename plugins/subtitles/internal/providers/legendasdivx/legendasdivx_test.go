package legendasdivx

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

// fakeSite is LegendasDivx with the account user/secret.
type fakeSite struct {
	srv      *httptest.Server
	logins   atomic.Int32
	searches atomic.Int32
	queries  []url.Values
	// searchesDone is what the site says of the day's searches.
	searchesDone int
	blocked      bool
	limit        bool
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

func box(lid, flag, hits, fps, desc, uploader string) string {
	header := ""
	if uploader != "" {
		header = fmt.Sprintf(`<div class="sub_header"><a href="/u">%s</a></div>`, uploader)
	}
	return fmt.Sprintf(`<div class="sub_box">%s<table><tr><th>Idioma:</th><td><img src="/img/%s.gif"></td>
		<th>Hits:</th><td>%s</td><th>Frame Rate:</th><td> %s </td></tr>
		<tr><td class="td_desc brd_up">%s</td></tr></table>
		<div class="sub_footer"><a class="sub_download" href="?name=Downloads&d_op=getit&lid=%s">Download</a></div></div>`,
		header, flag, hits, fps, desc, lid)
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	pack := zipped(t, "Show.S01E01.srt", "Show.S01E02.srt", ".hidden.srt")
	mux := http.NewServeMux()
	valid := func(r *http.Request) bool {
		c, err := r.Cookie("PHPSESSID")
		return err == nil && c.Value == "ok"
	}
	mux.HandleFunc("GET /forum/ucp.php", func(w http.ResponseWriter, r *http.Request) {
		if f.blocked {
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, "O seu IP está Bloqueado")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "anonymous", Path: "/"})
		_, _ = fmt.Fprint(w, `<form><input name="sid" value="abc"><input name="redirect" value="index.php"><input name="login" value="Login"></form>`)
	})
	mux.HandleFunc("POST /forum/ucp.php", func(w http.ResponseWriter, r *http.Request) {
		if r.PostFormValue("sid") != "abc" || r.PostFormValue("username") != "user" || r.PostFormValue("password") != "secret" {
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "", Path: "/", MaxAge: -1})
			return
		}
		f.logins.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "ok", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "phpbb3_2z8zs_u", Value: "42", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "tracking", Value: "x", Path: "/"})
	})
	mux.HandleFunc("GET /modules.php", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("d_op") == "getit" {
			switch {
			case f.limit:
				_, _ = fmt.Fprint(w, "<html>Limite de downloads diário atingido</html>")
			case q.Get("lid") == "3":
				_, _ = fmt.Fprint(w, "1\n00:00:01,000 --> 00:00:02,000\nplain\n")
			default:
				_, _ = w.Write(pack)
			}
			return
		}
		if !valid(r) {
			http.Redirect(w, r, "/forum/ucp.php?mode=login", http.StatusFound)
			return
		}
		f.searches.Add(1)
		f.queries = append(f.queries, q)
		_, _ = fmt.Fprintf(w, "<html><body><!--pesquisas: %d-->", f.searchesDone)
		switch {
		case strings.Contains(q.Get("query"), "nothing"):
			_, _ = fmt.Fprint(w, "A legenda não foi encontrada")
		case q.Get("page") == "2":
			_, _ = fmt.Fprint(w, box("4", "portugal", "1", "25.000", "The.Movie.2010.720p.BluRay-OTHER", ""))
		default:
			_, _ = fmt.Fprint(w, `<div class="pager_bar">(12 encontradas)</div>`,
				box("1", "brazil", "120", "23.976", "The.Movie.2010.1080p.BluRay.x264-GRP", "ana"),
				box("2", "portugal", "7", "25.000", "The Movie (2010) PAL", "rui"),
				box("5", "spain", "7", "25.000", "Unknown flag", "rui"))
		}
		_, _ = fmt.Fprint(w, "</body></html>")
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
	p.(*Provider).pause = 0
	return p.(*Provider)
}

const account = `{"username":"user","password":"secret"}`

func movie() *provider.Video {
	return &provider.Video{
		Title: "The Movie", Year: 2010, ExternalIDs: map[string]string{"imdb": "tt0000001"},
		FileName: "/media/The.Movie.2010.1080p.BluRay.x264-GRP.mkv", ReleaseGroup: "GRP", FrameRate: 23.976,
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, t.TempDir(), account)
	subs, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, fmt.Sprintf("%s %s %d %s %v", s.ID, s.Language.Tag, s.Downloads, s.Uploader, s.Matches))
	}
	want := []string{
		"1 pt-BR 120 ana [title year release_group]",
		"2 pt 7 rui [title year]",
		"4 pt 1 anonymous [title year]",
	}
	if !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}
	if q := f.queries[0]; q.Get("query") != "tt0000001" || q.Get("form_cat") != "28" || q.Get("d_op") != "search" || q.Get("op") != "_jz00" {
		t.Errorf("query got = %v", q)
	}
	if subs[0].PageLink != f.srv.URL+"/modules.php?name=Downloads&d_op=getit&lid=1" || subs[0].ReleaseNames[0] != "The.Movie.2010.1080p.BluRay.x264-GRP" {
		t.Errorf("subtitle got = %+v", subs[0])
	}

	// Subtitles of another frame rate match nothing when skipped.
	p = newProvider(t, f, t.TempDir(), `{"username":"user","password":"secret","skip_wrong_fps":true}`)
	subs, err = p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt-BR"}})
	if err != nil || len(subs) != 3 || len(subs[0].Matches) != 3 || subs[1].Matches != nil {
		t.Errorf("skipping wrong frame rates got = %+v, %v", subs, err)
	}
	if q := f.queries[len(f.queries)-2]; q.Get("form_cat") != "29" {
		t.Errorf("Brazilian query got = %v", q)
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, t.TempDir(), account)
	v := &provider.Video{Episode: true, Series: "Nothing Show", Season: 1, Number: 2}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "pt"}, {Tag: "pt-BR"}})
	if err != nil || len(subs) != 0 {
		t.Errorf("no results got = %v, %v", subs, err)
	}
	// Quoted twice, and the filters add up, as Bazarr's.
	if got, want := f.queries[1].Get("query"), `%22nothing show%22%20s01e02`; got != want ||
		!slices.Equal(f.queries[1]["form_cat"], []string{"28", "29"}) || f.queries[1].Get("imdb") != "None" || f.queries[1].Get("episodio") != "2" {
		t.Errorf("query got = %q, %v, want = %q", got, f.queries[1], want)
	}

	v = &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0000009"}}
	subs, err = p.Search(t.Context(), v, []provider.Language{{Tag: "pt"}})
	if err != nil || len(subs) != 3 {
		t.Fatalf("by IMDb ID got = %v, %v", subs, err)
	}
	if !slices.Equal(subs[0].Matches, []string{"series", "series_imdb_id", "season", "episode"}) {
		t.Errorf("matches got = %v", subs[0].Matches)
	}
	if q := f.queries[2]; q.Get("faz") != "pesquisa_episodio" || q.Get("idioma") != "28" || q.Get("d_op") != "jz_00" || q.Get("imdb") != "0000009" || q.Get("temporada") != "1" {
		t.Errorf("query got = %v", q)
	}
}

func TestSession(t *testing.T) {
	f := newFake(t)
	dir := t.TempDir()
	p := newProvider(t, f, dir, account)
	if _, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}}); err != nil {
		t.Fatal(err)
	}
	// The kept session is used after a restart.
	p = newProvider(t, f, dir, account)
	if _, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}}); err != nil {
		t.Fatal(err)
	}
	if n := f.logins.Load(); n != 1 {
		t.Errorf("logins got = %d, want = 1", n)
	}
	// An expired session logs in again.
	p = newProvider(t, f, dir, account)
	p.sess, _ = p.session(t.Context())
	u, _ := url.Parse(f.srv.URL)
	p.sess.Jar().SetCookies(u, []*http.Cookie{{Name: "PHPSESSID", Value: "expired", Path: "/"}})
	if _, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}}); err != nil {
		t.Fatal(err)
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("logins after expiry got = %d, want = 2", n)
	}
	// Another account doesn't use the kept session.
	if _, err := newProvider(t, f, dir, `{"username":"other","password":"secret"}`).Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("another account got = %v, want = AuthenticationError", err)
	}
}

func TestFailures(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(*fakeSite)
		want provider.Kind
	}{
		{"search limit", func(f *fakeSite) { f.searchesDone = 145 }, provider.SearchLimitReached},
		{"blocked", func(f *fakeSite) { f.blocked = true }, provider.IPAddressBlocked},
	} {
		f := newFake(t)
		c.set(f)
		p := newProvider(t, f, t.TempDir(), account)
		if _, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "pt"}}); provider.KindOf(err) != c.want {
			t.Errorf("%s got = %v, want = %v", c.name, err, c.want)
		}
	}
	if subs, err := newProvider(t, newFake(t), "", account).Search(t.Context(), movie(), []provider.Language{{Tag: "en"}}); err != nil || subs != nil {
		t.Errorf("English got = %v, %v, want none", subs, err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, t.TempDir(), account)
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}
	file, err := p.Download(t.Context(), v, "1")
	if err != nil || !strings.Contains(string(file.Data), "Legenda 2") || file.Format != "srt" {
		t.Errorf("download got = %q, %v, want the second episode", file.Data, err)
	}
	if _, err := p.Download(t.Context(), v, "3"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("not an archive got = %v, want = BadSubtitle", err)
	}
	f.limit = true
	if _, err := p.Download(t.Context(), v, "1"); provider.KindOf(err) != provider.DownloadLimitExceeded {
		t.Errorf("daily limit got = %v, want = DownloadLimitExceeded", err)
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{``, `{"username":"user"}`, `{"password":"secret"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("%q got = %v, want = ConfigurationError", s, err)
		}
	}
	d := Definition()
	now := time.Date(2024, 1, 1, 22, 30, 0, 0, time.UTC)
	if got := provider.Throttle(&d, provider.SearchLimitReached, now); got != 2*time.Hour+30*time.Minute {
		t.Errorf("search limit throttle got = %v, want = 2h30m", got)
	}
}

func TestQuote(t *testing.T) {
	for _, c := range []struct{ in, quote, requote string }{
		{"%22a b%22", "%2522a%20b%2522", "%22a%20b%22"},
		{"Amélie & co/x", "Am%C3%A9lie%20%26%20co/x", "Am%C3%A9lie%20&%20co/x"},
	} {
		if got := quote(c.in); got != c.quote {
			t.Errorf("quote(%q) got = %q, want = %q", c.in, got, c.quote)
		}
		if got := requote(c.in); got != c.requote {
			t.Errorf("requote(%q) got = %q, want = %q", c.in, got, c.requote)
		}
	}
}

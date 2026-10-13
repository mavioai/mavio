package turkcealtyaziorg

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
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func item(class, link, flag, season, episode, uploader, rips string) string {
	alcd := ""
	if season != "" {
		alcd = fmt.Sprintf(`<div class="alcd"><b>%s</b> <b>%s</b></div>`, season, episode)
	}
	return fmt.Sprintf(`<div class="%s"><div class="alisim"><div class="fl"><a href="%s">x</a></div></div>
		<div class="aldil"><span class="%s"></span></div>%s<div class="alcevirmen">%s</div><div class="alfps">25</div>
		<div class="alindirme">10</div><div class="ta-container"><div class="ripdiv">%s</div><div class="datediv">3 gün önce</div></div></div>`,
		class, link, flag, alcd, uploader, rips)
}

// fakeSite is Turkcealtyazi.org, which wants the cf_clearance cookie.
type fakeSite struct {
	srv   *httptest.Server
	posts []url.Values
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, n := range []string{"Show.S01E01.srt", "Show.S01E02.srt"} {
		fw, _ := w.Create(n)
		_, _ = fmt.Fprintf(fw, "1\n00:00:01,000 --> 00:00:02,000\n%s\n", n)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	cleared := func(r *http.Request) bool {
		c, err := r.Cookie("cf_clearance")
		return err == nil && c.Value == "ok" && r.UserAgent() == "Browser/1.0"
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if !cleared(r) {
			w.WriteHeader(http.StatusForbidden)
		}
	})
	mux.HandleFunc("GET /find.php", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("find") {
		case "tt0000009":
			_, _ = fmt.Fprint(w, `<html><head><meta name="description" content="Show"></head><body><div class="altyazi-list-wrapper"><div>`,
				item("altsonsez1 sezon_1", "/sub/1/show-s01e02", "flagtr", "1", "2", " ayse ", `<span class="rps r8"></span> NTb / AMZN <img src="/images/isitme.png">`),
				item("altsonsez1 sezon_1", "/sub/2/show-s01", "flagen", "1", "Paket", `<span class="rip9"></span>`, `<span class="rip9"></span>`),
				item("altsonsez1 sezon_1", "/sub/3/show-s01e03", "flagtr", "1", "3", "x", ""),
				item("altsonsez1 sezon_1", "/sub/4/show-s01e02", "flagfr", "1", "2", "x", ""),
				item("altsonsez1 sezon_2", "/sub/5/show-s02e02", "flagtr", "2", "2", "x", ""),
				item("altsonsez1 sezon_1", "/sub/6/broken", "flagtr", "", "", "x", ""),
				item("altsonsez1 sezon_1", "/sub/7/after", "flagtr", "1", "2", "x", ""),
				`</div></div></body></html>`)
		case "tt0000001":
			_, _ = fmt.Fprint(w, `<html><head><meta name="description" content="Movie"></head><body><div class="altyazi-list-wrapper"><div>`,
				item("altsonsez2", "/sub/8/movie", "flagtr", "", "", "mehmet", `<span class="rps r12"></span> FLEET`),
				`</div></div></body></html>`)
		case "tt0000404":
			w.WriteHeader(http.StatusNotFound)
		default:
			_, _ = fmt.Fprint(w, `<html><head><meta name="description" content="404 Error"></head></html>`)
		}
	})
	mux.HandleFunc("GET /sub/{id}/{name}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<form><input name="idid" value="%s"><input name="altid" value="a"><input name="sidid" value="s"></form>`, r.PathValue("id"))
	})
	mux.HandleFunc("POST /ind", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.posts = append(f.posts, r.PostForm)
		if !strings.HasSuffix(r.Referer(), "/sub/2/show-s01") || r.PostFormValue("idid") != "2" {
			return
		}
		_, _ = w.Write(b.Bytes())
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

const browser = `{"cookies":"cf_clearance=ok; other=1","user_agent":"Browser/1.0"}`

func episode() *provider.Video {
	return &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0000009"}}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, browser)
	subs, err := p.Search(t.Context(), episode(), []provider.Language{{Tag: "tr"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, fmt.Sprintf("%s %s %v %s %q %v", strings.TrimPrefix(s.ID, f.srv.URL), s.Language.Tag, s.Language.HearingImpaired, s.Uploader, s.ReleaseNames, s.Matches))
	}
	// The broken entry ends the list, as Bazarr's.
	want := []string{
		`/sub/1/show-s01e02S01E02 tr true ayse ["WEB-DL" "NTb" "AMZN"] [series season episode series_imdb_id]`,
		`/sub/2/show-s01S01E02 en false WEB-DL ["WEB-DL"] [series season episode series_imdb_id]`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("subtitles got = %q, want = %q", got, want)
	}

	movie := &provider.Video{Title: "Movie", ReleaseGroup: "ASAP", ExternalIDs: map[string]string{"imdb": "tt0000001"}}
	subs, err = p.Search(t.Context(), movie, []provider.Language{{Tag: "tr"}})
	if err != nil || len(subs) != 1 || subs[0].ID != f.srv.URL+"/sub/8/movie" || !slices.Equal(subs[0].Matches, []string{"imdb_id", "release_group"}) {
		t.Errorf("movie got = %+v, %v", subs, err)
	}
	for _, imdb := range []string{"tt0000404", "tt0000002", ""} {
		v := &provider.Video{ExternalIDs: map[string]string{"imdb": imdb}}
		if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "tr"}}); err != nil || len(subs) != 0 {
			t.Errorf("%q got = %v, %v, want none", imdb, subs, err)
		}
	}
}

func TestCookies(t *testing.T) {
	f := newFake(t)
	if _, err := newProvider(t, f, `{"cookies":"cf_clearance=old"}`).Search(t.Context(), episode(), []provider.Language{{Tag: "tr"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("expired cookies got = %v, want = AuthenticationError", err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, browser)
	file, err := p.Download(t.Context(), episode(), f.srv.URL+"/sub/2/show-s01S01E02")
	if err != nil || !strings.Contains(string(file.Data), "Show.S01E02.srt") {
		t.Errorf("pack got = %q, %v, want the episode's", file.Data, err)
	}
	if got := f.posts[0]; got.Get("altid") != "a" || got.Get("sidid") != "s" {
		t.Errorf("form got = %v", got)
	}
	if _, err := p.Download(t.Context(), episode(), f.srv.URL+"/sub/3/show-s01e03S01E03"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("no data got = %v, want = BadSubtitle", err)
	}
}

package avistaz

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nEpisode %d\r\n"

func pack(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for i, n := range names {
		f, err := w.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(f, srt, i+1)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type fakeTracker struct {
	srv       *httptest.Server
	downloads atomic.Int32
}

func newFake(t *testing.T) *fakeTracker {
	f := &fakeTracker{}
	zipped := pack(t, "Show/Show.S01E02.srt", "Show/Show.S01E01.srt", "Show/readme.txt")
	mux := http.NewServeMux()
	valid := func(r *http.Request) bool {
		c, err := r.Cookie("remember_web")
		return err == nil && c.Value == "ok"
	}
	mux.HandleFunc("GET /rules", func(w http.ResponseWriter, r *http.Request) {
		if !valid(r) {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
		}
	})
	mux.HandleFunc("GET /auth/login", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /torrent/1-show", func(w http.ResponseWriter, r *http.Request) {
		row := func(lang, link, uploader, downloads, likes, dislikes string) string {
			return fmt.Sprintf(`<tr><td>%s</td><td><a href="%s">dl</a></td><td>%s</td><td>%s</td>
				<td><span class="subtitle-likes"><a data-like="like"><span class="count">%s</span></a><a data-like="dislike"><span class="count">%s</span></a></span></td></tr>`,
				lang, link, uploader, downloads, likes, dislikes)
		}
		_, _ = fmt.Fprintf(w, `<html><body><div id="content-area"><div class="block"><div class="table-responsive"><table><tbody>
			<tr><td>Title</td><td> Show.S01.1080p.WEB-DL-GRP </td></tr>
			<tr><td>Subtitles</td><td><table><thead><tr><th>Language</th><th>Download</th><th>Uploader</th><th>Downloads</th><th></th></tr></thead>
			<tbody>%s%s%s%s%s</tbody></table></td></tr>
			</tbody></table></div></div></div></body></html>`,
			row("English", f.srv.URL+"/subtitle/a.srt", "u1", "1,200", "0", "0"),
			row("English", f.srv.URL+"/subtitle/b.srt", "u2", "10", "5", "1"),
			row("English", f.srv.URL+"/subtitle/pack.zip", "", "3", "0", "0"),
			row("French", f.srv.URL+"/subtitle/c.srt", "u3", "3", "0", "0"),
			row("Klingon", f.srv.URL+"/subtitle/d.srt", "u3", "3", "0", "0"))
	})
	mux.HandleFunc("GET /torrent/2-gone", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("GET /torrent/3-layout", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>maintenance</body></html>")
	})
	mux.HandleFunc("GET /subtitle/{name}", func(w http.ResponseWriter, r *http.Request) {
		f.downloads.Add(1)
		if r.PathValue("name") == "pack.zip" {
			_, _ = w.Write(zipped)
			return
		}
		_, _ = fmt.Fprintf(w, srt, 7)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeTracker, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).pause = 0
	return p.(*Provider)
}

func video(f *fakeTracker, page string) *provider.Video {
	return &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 1, ExternalIDs: map[string]string{InfoURLKey: f.srv.URL + page}}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"cookies":"remember_web=ok; XSRF-TOKEN=\"x\""}`)
	subs, err := p.Search(t.Context(), video(f, "/torrent/1-show"), []provider.Language{{Tag: "en"}, {Tag: "fr"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, s.ReleaseNames[0])
	}
	// Packs first, then by rating and downloads.
	want := []string{
		"Show.S01.1080p.WEB-DL-GRP [unknown] ZIP 3↓",
		"Show.S01.1080p.WEB-DL-GRP [u2] SRT 5\U0001f44d 1\U0001f44e",
		"Show.S01.1080p.WEB-DL-GRP [u1] SRT 1200↓",
		"Show.S01.1080p.WEB-DL-GRP [u3] SRT 3↓",
	}
	if !slices.Equal(got, want) {
		t.Errorf("releases got = %q, want = %q", got, want)
	}
	s := subs[2]
	if s.ID != f.srv.URL+"/subtitle/a.srt" || s.Language.Tag != "en" || s.Downloads != 1200 || s.Uploader != "u1" ||
		!slices.Equal(s.Matches, []string{"hash"}) || s.PageLink != f.srv.URL+"/torrent/1-show" {
		t.Errorf("subtitle got = %+v", s)
	}

	for _, c := range []struct {
		name string
		v    *provider.Video
	}{
		{"removed release", video(f, "/torrent/2-gone")},
		{"another tracker", &provider.Video{ExternalIDs: map[string]string{InfoURLKey: "https://other.example/torrent/1"}}},
		{"no info URL", &provider.Video{}},
	} {
		if subs, err := p.Search(t.Context(), c.v, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
			t.Errorf("%s got = %v, %v, want none", c.name, subs, err)
		}
	}
	if _, err := p.Search(t.Context(), video(f, "/torrent/3-layout"), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("unknown layout got = %v, want = ProviderError", err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"cookies":"remember_web=ok"}`)
	for ep := 1; ep <= 2; ep++ {
		v := video(f, "/torrent/1-show")
		v.Number = ep
		file, err := p.Download(t.Context(), v, f.srv.URL+"/subtitle/pack.zip")
		if err != nil {
			t.Fatal(err)
		}
		// The files are named in reverse order of their content.
		if want := fmt.Sprintf("1\n00:00:01,000 --> 00:00:02,000\nEpisode %d\n", 3-ep); string(file.Data) != want || file.Format != "srt" {
			t.Errorf("episode %d got = %q, want = %q", ep, file.Data, want)
		}
	}
	// The pack is downloaded once.
	if n := f.downloads.Load(); n != 1 {
		t.Errorf("downloads got = %d, want = 1", n)
	}
	file, err := p.Download(t.Context(), video(f, "/torrent/1-show"), f.srv.URL+"/subtitle/a.srt")
	if err != nil || file.Format != "srt" {
		t.Errorf("subtitle file got = %v, %v, want an srt", file, err)
	}
	v := video(f, "/torrent/1-show")
	v.Number = 9
	if _, err := p.Download(t.Context(), v, f.srv.URL+"/subtitle/pack.zip"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("missing episode got = %v, want = BadSubtitle", err)
	}
}

func TestCookies(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, `{"cookies":"remember_web=expired"}`)
	if _, err := p.Search(t.Context(), video(f, "/torrent/1-show"), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("expired cookies got = %v, want = AuthenticationError", err)
	}
}

func TestDefinitions(t *testing.T) {
	for _, d := range []provider.Definition{Definition(), CinemaZDefinition()} {
		for _, tag := range []string{"en", "pt-BR", "nb", "yue", "he", "id"} {
			if !slices.Contains(d.Languages, tag) {
				t.Errorf("%s languages got no %s", d.Name, tag)
			}
		}
	}
	p, err := NewCinemaZ(provider.Env{}, nil)
	if err != nil || p.(*Provider).server.String() != "https://cinemaz.to/" {
		t.Errorf("CinemaZ got = %v, %v", p, err)
	}
	if _, err := New(provider.Env{}, json.RawMessage(`[`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("bad settings got = %v, want = ConfigurationError", err)
	}
}

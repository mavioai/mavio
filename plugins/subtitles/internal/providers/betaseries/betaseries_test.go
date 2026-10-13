package betaseries

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nBonjour\r\n"

type fakeAPI struct {
	srv     *httptest.Server
	queries []string
	status  int
	body    string
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{}
	mux := http.NewServeMux()
	answer := func(w http.ResponseWriter, r *http.Request) {
		f.queries = append(f.queries, r.URL.Path+"?"+r.URL.RawQuery)
		if f.status != 0 {
			w.WriteHeader(f.status)
		}
		_, _ = w.Write([]byte(strings.ReplaceAll(f.body, "%[1]s", f.srv.URL)))
	}
	mux.HandleFunc("GET /episodes/display", answer)
	mux.HandleFunc("GET /shows/episodes", answer)
	mux.HandleFunc("GET /srt/1", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(srt)) })
	mux.HandleFunc("GET /srt/2", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, n := range []string{".hidden.srt", "readme.nfo", "Show.S01E02.LOL.srt", "Show.S01E02.KILLERS.srt"} {
			fw, _ := zw.Create(n)
			_, _ = fw.Write([]byte(srt + n))
		}
		_ = zw.Close()
		_, _ = w.Write(buf.Bytes())
	})
	mux.HandleFunc("GET /srt/3", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	mux.HandleFunc("GET /srt/4", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>")) })
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeAPI) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"token":"tok"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

const subtitles = `{"subtitles":[
	{"id":1,"language":"VF","source":"addic7ed","file":"Show.S01E02.720p.HDTV.x264-KILLERS.fr.srt","url":"%[1]s/srt/1"},
	{"id":2,"language":"VO","source":"tvsubtitles","file":"Show.S01E02.zip","url":"%[1]s/srt/2"},
	{"id":3,"language":"VF","source":"seriessub","file":"dead.srt","url":"%[1]s/srt/3"},
	{"id":4,"language":"VOVF","source":"addic7ed","file":"both.srt","url":"%[1]s/srt/4"}]}`

func TestSearch(t *testing.T) {
	for _, c := range []struct {
		name, body, query, match string
		v                        *provider.Video
	}{
		{
			"by episode", `{"errors":[],"episode":` + subtitles + `}`,
			"/episodes/display?key=tok&subtitles=1&thetvdb_id=77&v=3.0", "tvdb_id",
			&provider.Video{Episode: true, Season: 1, Number: 2, ExternalIDs: map[string]string{"tvdb": "77"}, SeriesExternalIDs: map[string]string{"tvdb": "5"}},
		},
		{
			"by series", `{"errors":[],"episodes":[` + subtitles + `]}`,
			"/shows/episodes?episode=2&key=tok&season=1&subtitles=1&thetvdb_id=5&v=3.0", "series_tvdb_id",
			&provider.Video{Episode: true, Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"tvdb": "5"}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.body = c.body
			p := newProvider(t, f)
			subs, err := p.Search(t.Context(), c.v, []provider.Language{{Tag: "fr"}, {Tag: "en"}})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(f.queries, []string{c.query}) {
				t.Errorf("queries got = %v, want = [%s]", f.queries, c.query)
			}
			want := []string{f.srv.URL + "/srt/1", f.srv.URL + "/srt/2"}
			if got := ids(subs); !slices.Equal(got, want) {
				t.Fatalf("subtitles got = %v, want = %v", got, want)
			}
			s := subs[0]
			if s.Language.Tag != "fr" || subs[1].Language.Tag != "en" || !slices.Equal(s.Matches, []string{c.match}) ||
				!slices.Equal(s.ReleaseNames, []string{"Show.S01E02.720p.HDTV.x264-KILLERS.fr.srt"}) {
				t.Errorf("subtitle got = %+v", s)
			}
			subs, err = p.Search(t.Context(), c.v, []provider.Language{{Tag: "en"}})
			if err != nil || len(subs) != 1 || subs[0].Language.Tag != "en" {
				t.Errorf("English subtitles got = %+v, %v", subs, err)
			}
		})
	}
}

func TestSearchNothing(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	ep := &provider.Video{Episode: true, Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"tvdb": "5"}}
	if subs, err := p.Search(t.Context(), &provider.Video{Episode: true}, nil); err != nil || subs != nil || len(f.queries) != 0 {
		t.Errorf("search without TVDB IDs got = %v, %v, %v, want none", subs, err, f.queries)
	}
	for _, c := range []struct {
		name, body string
		status     int
		kind       provider.Kind
	}{
		{"no series", `{"errors":[{"code":4001,"text":"no show"}]}`, 400, -1},
		{"errors", `{"errors":[{"code":2001}],"episodes":[` + subtitles + `]}`, 200, -1},
		{"no episodes", `{"errors":[],"episodes":[]}`, 200, -1},
		{"bad token", `{"errors":[{"code":1001}]}`, 400, provider.AuthenticationError},
		{"other error", `{"errors":[{"code":2005}]}`, 400, provider.ProviderError},
		{"server error", `oops`, 503, provider.ProviderError},
		{"not JSON", `oops`, 200, provider.ParseResponseError},
	} {
		t.Run(c.name, func(t *testing.T) {
			f.body, f.status = c.body, c.status
			subs, err := p.Search(t.Context(), ep, []provider.Language{{Tag: "fr"}})
			if c.kind < 0 {
				if err != nil || len(subs) != 0 {
					t.Errorf("search got = %v, %v, want none", subs, err)
				}
				return
			}
			if got := provider.KindOf(err); got != c.kind {
				t.Errorf("kind got = %v, want = %v (%v)", got, c.kind, err)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	const sub = "1\n00:00:01,000 --> 00:00:02,000\nBonjour\n"
	for _, c := range []struct {
		name, id, group, want string
		kind                  provider.Kind
	}{
		{"subtitle", "/srt/1", "", sub, -1},
		{"archive by release group", "/srt/2", "KILLERS", sub + "Show.S01E02.KILLERS.srt", -1},
		{"archive's first", "/srt/2", "DIMENSION", sub + "Show.S01E02.LOL.srt", -1},
		{"not found", "/srt/3", "", "", provider.BadSubtitle},
		{"no subtitle", "/srt/4", "", "", provider.BadSubtitle},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := p.Download(t.Context(), &provider.Video{Episode: true, ReleaseGroup: c.group}, f.srv.URL+c.id)
			if c.kind >= 0 {
				if provider.KindOf(err) != c.kind {
					t.Errorf("err got = %v, want = %v", err, c.kind)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Data) != c.want || got.Format != "srt" {
				t.Errorf("download got = %q %s, want = %q", got.Data, got.Format, c.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	if _, err := New(provider.Env{}, json.RawMessage(`{"token":""}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New without a token got = %v, want = ConfigurationError", err)
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}

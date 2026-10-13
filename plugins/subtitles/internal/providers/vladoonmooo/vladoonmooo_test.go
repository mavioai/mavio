package vladoonmooo

import (
	"archive/zip"
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "vladoonmooo.json", ported.Cases{Facts: map[string]string{
		"test_list_subtitles_episode_accepts_matching_season_pack": "TestSearchEpisode",
		"test_list_subtitles_movie_prefers_imdb_match":             "TestSearchMovie",
		"test_download_subtitle_extracts_archive_content":          "TestDownload",
	}})
}

// Videos of Bazarr's tests.
var (
	dune = &provider.Video{
		Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"},
		ReleaseName: "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX", ReleaseGroup: "SHITBOX",
	}
	breakingBad = &provider.Video{
		Episode: true, Series: "Breaking Bad", Season: 1, Number: 1, SeriesExternalIDs: map[string]string{"imdb": "tt0903747"},
		ReleaseName: "Breaking.Bad.S01E01.720p.BluRay.X264-REWARD", ReleaseGroup: "REWARD",
	}
)

func makeZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n" + name + "\r\n"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type fakeSite struct {
	srv     *httptest.Server
	queries []url.Values
}

func newFake(t *testing.T, status int, search string, files map[string][]byte) *fakeSite {
	f := &fakeSite{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != f.srv.URL+"/subs/" {
			t.Errorf("Referer got = %q, want = %q", r.Header.Get("Referer"), f.srv.URL+"/subs/")
		}
		if r.URL.Path == "/subs/search-subtitles" {
			f.queries = append(f.queries, r.URL.Query())
			w.WriteHeader(status)
			_, _ = w.Write([]byte(search))
			return
		}
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSite) provider(t *testing.T) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t, http.StatusOK, `{"results":[
		{"id":20554,"original_title":"Breaking Bad","title":"Breaking Bad","type":"tv","season":1,"episode":null,
		 "episode_type":"season","imdb_id":"tt0903747",
		 "release_names":["breaking.bad.s01e01.720p.bluray.x264-reward","breaking.bad.s01e02.720p.bluray.x264-reward"],
		 "uploaded_by":"Vlad00n"},
		{"id":20555,"title":"Breaking Bad","type":"tv","season":1,"episode":1,"imdb_id":"tt0903747","release_names":"bb.e01"},
		{"id":20556,"title":"Breaking Bad","type":"tv","season":2,"episode_type":"season","imdb_id":"tt0903747"},
		{"id":20557,"title":"Breaking Bad","type":"tv","season":1,"episode":1,"imdb_id":"tt9999999"},
		{"id":20558,"title":"Breaking Bad","type":"movie","season":1,"episode":1},
		{"id":20559,"title":"Breaking Bad","season":1,"episode":1}
	]}`, nil)
	got, err := f.provider(t).Search(t.Context(), breakingBad, []provider.Language{{Tag: "bg"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"20554", "20555", "20559"}; !slices.Equal(ids(got), want) {
		t.Fatalf("IDs got = %v, want = %v", ids(got), want)
	}
	if want := (url.Values{"q": {"Breaking Bad S01E01"}}); len(f.queries) != 1 || f.queries[0].Encode() != want.Encode() {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	s := got[0]
	if s.PageLink != f.srv.URL+"/subs/download/20554" || s.Uploader != "Vlad00n" || !slices.Equal(s.Matches, []string{"imdb_id"}) ||
		!slices.Contains(s.ReleaseNames, "breaking.bad.s01e01.720p.bluray.x264-reward") || !slices.Contains(s.ReleaseNames, "Breaking Bad") {
		t.Errorf("season pack got = %+v", s)
	}
	if !slices.Equal(got[1].ReleaseNames, []string{"bb.e01", "Breaking Bad"}) || len(got[2].Matches) != 0 {
		t.Errorf("subtitles got = %+v", got[1:])
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t, http.StatusOK, `{"results":[
		{"id":15800,"original_title":"Dune","title":"Dune","type":"movie","imdb_id":"tt1160419","release_names":["Dune.2021.1080p.WEBRip.x264-SHITBOX"]},
		{"id":15801,"original_title":"Dune: Part Two","title":"Dune: Part Two","type":"movie","imdb_id":"tt15239678","release_names":["Dune.Part.Two.2024.720p.WEB.h264-EDITH"]},
		{"id":15802,"title":"Dune","type":"tv"},
		{"title":"Dune"}
	]}`, nil)
	got, err := f.provider(t).Search(t.Context(), dune, []provider.Language{{Tag: "bg"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"15800"}; !slices.Equal(ids(got), want) {
		t.Fatalf("IDs got = %v, want = %v", ids(got), want)
	}
	if want := (url.Values{"q": {"Dune"}}); f.queries[0].Encode() != want.Encode() {
		t.Errorf("query got = %v, want = %v", f.queries[0], want)
	}
	if !slices.Equal(got[0].Matches, []string{"imdb_id"}) || !slices.Contains(got[0].ReleaseNames, "Dune.2021.1080p.WEBRip.x264-SHITBOX") {
		t.Errorf("subtitle got = %+v", got[0])
	}
}

func TestSearchFailure(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   provider.Kind
	}{
		{http.StatusTooManyRequests, "", provider.TooManyRequests},
		{http.StatusInternalServerError, "", provider.ServiceUnavailable},
		{http.StatusNotFound, "", provider.ProviderError},
		{http.StatusOK, "<html>", provider.ParseResponseError},
	} {
		f := newFake(t, c.status, c.body, nil)
		if _, err := f.provider(t).Search(t.Context(), dune, []provider.Language{{Tag: "bg"}}); provider.KindOf(err) != c.want {
			t.Errorf("Search(%d) got = %v, want = %v", c.status, err, c.want)
		}
	}
	f := newFake(t, http.StatusOK, "null", nil)
	if got, err := f.provider(t).Search(t.Context(), dune, []provider.Language{{Tag: "bg"}}); err != nil || len(got) != 0 {
		t.Errorf("Search(null) got = %v, %v, want = none", got, err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t, http.StatusOK, "", map[string][]byte{
		"/subs/download/20554": makeZip(t, "breaking.bad.s01e02.720p.bluray.x264-reward.srt", "breaking.bad.s01e01.720p.bluray.x264-reward.srt"),
		"/subs/download/15800": makeZip(t, "Dune.srt", "Dune.Part.Two.srt"),
		"/subs/download/1":     makeZip(t, "readme.txt"),
	})
	p := f.provider(t)
	for _, c := range []struct {
		id   string
		v    *provider.Video
		want string
	}{
		{"20554", breakingBad, "breaking.bad.s01e01.720p.bluray.x264-reward.srt"},
		{"15800", dune, "Dune.srt"},
	} {
		got, err := p.Download(t.Context(), c.v, c.id)
		if err != nil {
			t.Fatalf("Download(%s): %v", c.id, err)
		}
		if bytes.Contains(got.Data, []byte("\r\n")) || !strings.Contains(string(got.Data), c.want) || got.Format != "srt" || got.Language.Tag != "bg" {
			t.Errorf("Download(%s) got = %q, want = %s with Unix line endings", c.id, got.Data, c.want)
		}
	}
	if _, err := p.Download(t.Context(), dune, "1"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Download without subtitle got = %v, want = %v", err, provider.BadSubtitle)
	}
}

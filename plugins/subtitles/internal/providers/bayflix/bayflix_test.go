package bayflix

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
	ported.Run(t, "bayflix.json", ported.Cases{Facts: map[string]string{
		"test_list_subtitles_movie_filters_by_year_and_builds_download_link": "TestSearchMovie",
		"test_list_subtitles_episode_filters_to_requested_episode":           "TestSearchEpisode",
		"test_download_subtitle_extracts_archive_content":                    "TestDownload",
	}})
}

// Videos of Bazarr's tests.
var (
	dune = &provider.Video{
		Title: "Dune", Year: 2021, ExternalIDs: map[string]string{"imdb": "tt1160419"},
		ReleaseName: "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX", ReleaseGroup: "SHITBOX", Resolution: "1080p", Source: "Web",
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

// fakeSite answers searches with a JSON body and downloads with archives.
type fakeSite struct {
	srv     *httptest.Server
	queries []url.Values
}

func newFake(t *testing.T, search string, files map[string][]byte) *fakeSite {
	f := &fakeSite{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != f.srv.URL+"/" {
			t.Errorf("Referer got = %q, want = %q", r.Header.Get("Referer"), f.srv.URL+"/")
		}
		if r.URL.Path == "/api/subtitles/search" {
			f.queries = append(f.queries, r.URL.Query())
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

func TestSearchMovie(t *testing.T) {
	f := newFake(t, `[
		{"_id":"dune-2021","title":"Dune","description":"Dune.2021.1080p.WEBRip.x264-SHITBOX\n\nFPS: 23.976 | CDs: 1",
		 "release_name":["Dune.2021.1080p.WEBRip.x264-SHITBOX"],"release_date":"2021-09-15","media_type":"movie"},
		{"_id":"dune-1964","title":"Woman in the Dunes","description":"Suna No Onna\n\nFPS: 23.976 | CDs: 2",
		 "release_name":["Suna.No.Onna.1964"],"release_date":"1964-02-15","media_type":"movie"},
		{"_id":"dune-2020","title":"Dune","release_name":"Dune.2020.Fan.Edit","release_date":"2020","subtitle_link":"https://cdn.example/dune.zip"},
		{"title":"No ID","release_date":"2021"}
	]`, nil)
	got, err := f.provider(t).Search(t.Context(), dune, []provider.Language{{Tag: "bg"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := (url.Values{"title": {"Dune"}}); len(f.queries) != 1 || f.queries[0].Encode() != want.Encode() {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	if len(got) != 2 {
		t.Fatalf("got = %d subtitles, want = 2", len(got))
	}
	if got[0].ID != "dune-2021" || got[0].PageLink != f.srv.URL+"/api/subtitles/download/dune-2021" {
		t.Errorf("ID, page link got = %s, %s, want = dune-2021 and its download link", got[0].ID, got[0].PageLink)
	}
	if want := []string{"Dune.2021.1080p.WEBRip.x264-SHITBOX", "Dune"}; !slices.Equal(got[0].ReleaseNames, want) {
		t.Errorf("release names got = %v, want = %v", got[0].ReleaseNames, want)
	}
	if !slices.Equal(got[0].Matches, []string{"year"}) || got[0].Language.Tag != "bg" {
		t.Errorf("matches, language got = %v, %v, want = [year], bg", got[0].Matches, got[0].Language)
	}
	if got[1].ID != "https://cdn.example/dune.zip" || got[1].PageLink != got[1].ID || len(got[1].Matches) != 0 ||
		!slices.Equal(got[1].ReleaseNames, []string{"Dune.2020.Fan.Edit", "Dune"}) {
		t.Errorf("subtitle with a link got = %+v", got[1])
	}
}

func TestSearchEpisode(t *testing.T) {
	f := newFake(t, `[
		{"_id":"bb-s01e01","title":"Breaking Bad","description":"Breaking.Bad.S01E01.720p.BluRay.X264-REWARD\n\nFPS: 23.976 | CDs: 1",
		 "release_name":["Breaking.Bad.S01E01.720p.BluRay.X264-REWARD"],"media_type":"tv"},
		{"_id":"bb-s01e02","title":"Breaking Bad","description":"Breaking.Bad.S01E02.720p.BluRay.X264-REWARD\n\nFPS: 23.976 | CDs: 1",
		 "release_name":["Breaking.Bad.S01E02.720p.BluRay.X264-REWARD"],"media_type":"tv"},
		{"_id":"bb-1x01","title":"Breaking Bad","description":"Breaking Bad 1x01 Pilot","media_type":"tv"},
		{"_id":"bb-pack","title":"Breaking Bad","description":"Season 1","media_type":"tv"}
	]`, nil)
	got, err := f.provider(t).Search(t.Context(), breakingBad, []provider.Language{{Tag: "bg"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if want := []string{"bb-s01e01", "bb-1x01"}; !slices.Equal(ids, want) {
		t.Errorf("IDs got = %v, want = %v", ids, want)
	}
	if want := (url.Values{"title": {"Breaking Bad"}}); len(f.queries) != 1 || f.queries[0].Encode() != want.Encode() {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	if len(got) > 0 && !slices.Contains(got[0].ReleaseNames, "Breaking.Bad.S01E01.720p.BluRay.X264-REWARD") {
		t.Errorf("release names got = %v, want = the release", got[0].ReleaseNames)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t, `[]`, map[string][]byte{
		"/api/subtitles/download/dune-2021": makeZip(t, "Dune.2021.1080p.WEBRip.x264-SHITBOX.srt"),
		"/api/subtitles/download/bb-pack":   makeZip(t, "Breaking.Bad.S01E02.srt", "Breaking.Bad.S01E01.srt"),
		"/api/subtitles/download/junk":      []byte("<html>no</html>"),
		"/link.zip":                         makeZip(t, "a.srt", "b.srt"),
	})
	p := f.provider(t)
	for _, c := range []struct {
		id   string
		v    *provider.Video
		want string
	}{
		{"dune-2021", dune, "Dune.2021.1080p.WEBRip.x264-SHITBOX.srt"},
		{"bb-pack", breakingBad, "Breaking.Bad.S01E01.srt"},
		{f.srv.URL + "/link.zip", dune, "a.srt"},
	} {
		got, err := p.Download(t.Context(), c.v, c.id)
		if err != nil {
			t.Fatalf("Download(%s): %v", c.id, err)
		}
		if bytes.Contains(got.Data, []byte("\r\n")) || !strings.Contains(string(got.Data), c.want) || got.Format != "srt" || got.Language.Tag != "bg" {
			t.Errorf("Download(%s) got = %q, want = %s with Unix line endings", c.id, got.Data, c.want)
		}
	}
	for _, c := range []struct {
		id   string
		want provider.Kind
	}{
		{"junk", provider.BadSubtitle},
		{"missing", provider.ProviderError},
	} {
		if _, err := p.Download(t.Context(), dune, c.id); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.id, err, c.want)
		}
	}
}

func TestSearchFailure(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   provider.Kind
	}{
		{http.StatusTooManyRequests, "", provider.TooManyRequests},
		{http.StatusBadGateway, "", provider.ServiceUnavailable},
		{http.StatusOK, "<html>", provider.ParseResponseError},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(c.body))
		}))
		f := &fakeSite{srv: srv}
		_, err := f.provider(t).Search(t.Context(), dune, []provider.Language{{Tag: "bg"}})
		srv.Close()
		if provider.KindOf(err) != c.want {
			t.Errorf("Search(%d) got = %v, want = %v", c.status, err, c.want)
		}
	}
}

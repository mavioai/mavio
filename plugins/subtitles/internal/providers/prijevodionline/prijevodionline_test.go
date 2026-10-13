package prijevodionline

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "prijevodi.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_hr":               "TestListCroatian",
			"test_list_subtitles_sr":               "TestListSerbian",
			"test_list_subtitles_series_not_found": "TestSeriesNotFound",
			"test_get_matches_reward_release":      "TestRewardRelease",
			"test_download_subtitle_zip":           "TestDownloadZip",
		},
	})
}

const subPath = "/preuzmi-prijevod/epizoda/37142/game-of-thrones-01x06-a-golden-crown-720pbluray-sr"

// fakeSite serves the recorded pages of Game of Thrones' S01E06 that Bazarr's
// tests mock.
func fakeSite(t *testing.T) *Provider {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(pattern, name string) {
		data := ported.Response(t, name)
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) })
	}
	serve("GET /serije/index/g", "prijevodi_index_g.html")
	serve("GET /serije/view/935/game-of-thrones", "prijevodi_got.html")
	mux.HandleFunc("POST /prijevod/get/34272", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("key") != "ca7a167e13db896fe2324b2cbf10311f" ||
			r.Header.Get("X-Requested-With") != "XMLHttpRequest" || !strings.HasSuffix(r.Header.Get("Referer"), "/serije/view/935/game-of-thrones") {
			t.Errorf("got = %v with %v, want = the page's key", r.Form, r.Header)
		}
		_, _ = w.Write(ported.Response(t, "prijevodi_ep34272.html"))
	})
	serve("GET "+subPath, "prijevodi_sub_37143.zip")
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func got() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Game of Thrones", Season: 1, Number: 6, ReleaseGroup: "REWARD", Resolution: "720p", VideoCodec: "H.264",
		FileName: "Game.of.Thrones.S01E06.720p.BluRay.X264-REWARD.mkv",
	}
}

func search(t *testing.T, tag string) []provider.Subtitle {
	t.Helper()
	subs, err := fakeSite(t).Search(t.Context(), got(), []provider.Language{{Tag: tag}})
	if err != nil {
		t.Fatal(err)
	}
	return subs
}

func check(t *testing.T, subs []provider.Subtitle, tag string, n int) {
	t.Helper()
	if len(subs) != n {
		t.Fatalf("got = %d subtitles, want = %d", len(subs), n)
	}
	for _, s := range subs {
		if s.Language.Tag != tag || !slices.Equal(s.Matches, []string{"series", "season", "episode"}) {
			t.Errorf("got = %+v, want = %s matching series, season and episode", s, tag)
		}
	}
}

func TestListCroatian(t *testing.T) { check(t, search(t, "hr"), "hr", 5) }

func TestListSerbian(t *testing.T) { check(t, search(t, "sr"), "sr", 4) }

func TestListSerboCroatian(t *testing.T) { check(t, search(t, "sh"), "sh", 9) }

func TestSeriesNotFound(t *testing.T) {
	v := &provider.Video{Episode: true, Series: "Galaxy Nonsense Show", Season: 1, Number: 1}
	subs, err := fakeSite(t).Search(t.Context(), v, []provider.Language{{Tag: "hr"}})
	if err != nil || subs != nil {
		t.Errorf("got = %v, %v, want = none", subs, err)
	}
}

func reward(t *testing.T, subs []provider.Subtitle) provider.Subtitle {
	t.Helper()
	for _, s := range subs {
		if strings.Contains(s.PageLink, "/37142/") {
			return s
		}
	}
	t.Fatalf("got = %v, want = subtitle 37142", subs)
	return provider.Subtitle{}
}

// TestRewardRelease ports test_get_matches_reward_release: the release
// group is matched by the server from the release names.
func TestRewardRelease(t *testing.T) {
	s := reward(t, search(t, "sr"))
	if want := []string{"720p.BluRay.X264-REWARD", "Preveo dragan4e"}; !slices.Equal(s.ReleaseNames, want) {
		t.Errorf("got = %v, want = %v", s.ReleaseNames, want)
	}
}

func TestDownloadZip(t *testing.T) {
	p := fakeSite(t)
	subs, err := p.Search(t.Context(), got(), []provider.Language{{Tag: "sr"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := p.Download(t.Context(), got(), reward(t, subs).ID)
	if err != nil {
		t.Fatal(err)
	}
	data := ported.Response(t, "prijevodi_sub_37143.zip")
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var want []byte
	for _, zf := range zr.File {
		if slices.Contains(extensions, strings.ToLower(path.Ext(zf.Name))) {
			rc, _ := zf.Open()
			want, _ = io.ReadAll(rc)
			_ = rc.Close()
			break
		}
	}
	if !bytes.Equal(f.Data, archive.FixLineEndings(want)) || f.Language.Tag != "sr" {
		t.Errorf("got = %d bytes in %q, want = %d bytes in sr", len(f.Data), f.Language.Tag, len(want))
	}
	for id, kind := range map[string]provider.Kind{
		url.Values{"u": {"/serije/index/g"}}.Encode(): provider.ProviderError,
		url.Values{"u": {"/nowhere"}}.Encode():        provider.ProviderError,
		"u=http://elsewhere":                          provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), got(), id); provider.KindOf(err) != kind {
			t.Errorf("%s: got = %v, want = %v", id, err, kind)
		}
	}
}

package greeksubs

import (
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
	ported.Run(t, "greeksubs.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_parses_current_table_markup":     "TestListMovie",
			"test_download_subtitle_refreshes_expired_token":      "TestDownloadRefreshesToken",
			"test_download_subtitle_submits_legacy_download_form": "TestDownloadForm",
		},
	})
}

const (
	moviePage = `<input type="hidden" value="fresh-token" id="secCode">` +
		`<table id="elSub"><tbody><tr>` +
		`<td>1</td><td><img src="/resources/lang/greek.jpg" alt="el"></td><span>` +
		`<td></td><td></td><td><button onclick="downloadMe('subtitle-id')">Download</button></td>` +
		`<td>64</td><td>Season-of-the-Witch-2011-1080p-bluray-x264-blow</td>` +
		`<td><img class="userProfileImgN"><div class="userNameBox">WhiteAngel</div></td>` +
		`</span></tr><tr>` +
		`<td>2</td><td><img src="/resources/lang/english.jpg" alt="en"></td>` +
		`<td><button onclick="downloadMe('english-id')">Download</button></td><td>1</td><td>English</td>` +
		`<td><div class="userNameBox">Someone</div></td></tr></tbody></table>`
	seriesPage = `<div class="col-lg-offset-2 col-md-8 text-center top30 bottom10">` +
		`<a href="/en/view/tt0944947/s1e1">Season 1 Episode 1</a><a href="/en/view/tt0944947/s1e2">Season 1 Episode 2</a></div>`
	episodePage = `<input id="secCode" value="ep-token"><table id="elSub"><tbody><tr>` +
		`<td><img src="/resources/lang/gr.png" alt="el"></td><td><button onclick="downloadMe('ep-id')">D</button></td>` +
		`<td>3</td><td>Game-of-Thrones-S01E02-720p</td><td><div class="userNameBox">Uploader</div></td></tr></tbody></table>`
	srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"
)

type fake struct {
	srv *httptest.Server
	mux *http.ServeMux
}

func newFake(t *testing.T) *fake {
	f := &fake{mux: http.NewServeMux()}
	f.srv = httptest.NewServer(f.mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) page(pattern, body string) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	})
}

func (f *fake) provider(t *testing.T) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestListMovie(t *testing.T) {
	f := newFake(t)
	f.page("GET /en/view/tt0479997", moviePage)
	p := f.provider(t)
	movie := &provider.Video{Title: "Season of the Witch", Year: 2011, ExternalIDs: map[string]string{"imdb": "tt0479997"}}
	subs, err := p.Search(t.Context(), movie, []provider.Language{{Tag: "el"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("got = %d subtitles, want = 1", len(subs))
	}
	s := subs[0]
	q, _ := url.ParseQuery(s.ID)
	if q.Get("id") != "subtitle-id" || s.PageLink != f.srv.URL+"/dll/subtitle-id/0/fresh-token" || s.Uploader != "WhiteAngel" ||
		!slices.Equal(s.ReleaseNames, []string{"Season.of.the.Witch.2011.1080p.bluray.x264.blow"}) || len(s.Matches) != 0 {
		t.Errorf("got = %+v, want = subtitle-id by WhiteAngel", s)
	}
	if subs, err := p.Search(t.Context(), &provider.Video{Title: "X"}, nil); err != nil || subs != nil {
		t.Errorf("without IMDb ID got = %v, %v, want = none", subs, err)
	}
	unknown := &provider.Video{ExternalIDs: map[string]string{"imdb": "tt1"}}
	if subs, err := p.Search(t.Context(), unknown, nil); err != nil || subs != nil {
		t.Errorf("unknown IMDb ID got = %v, %v, want = none", subs, err)
	}
}

func TestListEpisode(t *testing.T) {
	f := newFake(t)
	f.page("GET /en/view/tt0944947", seriesPage)
	f.page("GET /en/view/tt0944947/s1e2", episodePage)
	p := f.provider(t)
	ep := &provider.Video{Episode: true, Series: "Game of Thrones", Season: 1, Number: 2, SeriesExternalIDs: map[string]string{"imdb": "tt0944947"}}
	subs, err := p.Search(t.Context(), ep, []provider.Language{{Tag: "el"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"year"}) || subs[0].ReleaseNames[0] != "Game.of.Thrones.S01E02.720p" {
		t.Fatalf("got = %+v, want = 1 subtitle matching the year", subs)
	}
	q, _ := url.ParseQuery(subs[0].ID)
	if q.Get("ref") != f.srv.URL+"/en/view/tt0944947/s1e2" || q.Get("sec") != "ep-token" {
		t.Errorf("got = %v, want = the episode's page and token", q)
	}

	f.mux.HandleFunc("GET /en/view/tt5", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	ep.SeriesExternalIDs["imdb"] = "tt5"
	if _, err := p.Search(t.Context(), ep, nil); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("got = %v, want = %v", err, provider.ServiceUnavailable)
	}
}

func id(f *fake, token, referer string) string {
	return url.Values{"id": {"subtitle-id"}, "sec": {token}, "ref": {f.srv.URL + referer}, "lang": {"el"}}.Encode()
}

func TestDownloadRefreshesToken(t *testing.T) {
	f := newFake(t)
	f.mux.HandleFunc("GET /dll/subtitle-id/0/stale-token", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})
	f.page("GET /en/view/tt0479997", `<input id="secCode" value="fresh-token">`)
	f.mux.HandleFunc("GET /dll/subtitle-id/0/fresh-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != f.srv.URL+"/en/view/tt0479997" {
			t.Errorf("referer got = %q, want = the subtitle's page", r.Header.Get("Referer"))
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="subtitle.srt"`)
		_, _ = w.Write([]byte(srt))
	})
	p := f.provider(t)
	file, err := p.Download(t.Context(), nil, id(f, "stale-token", "/en/view/tt0479997"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Format != "srt" || strings.Contains(string(file.Data), "\r\n") || file.Language.Tag != "el" {
		t.Errorf("got = %q, %q, want = an srt without CRLF", file.Data, file.Format)
	}

	f.page("GET /en/view/tt2", `<p>no token</p>`)
	if _, err := p.Download(t.Context(), nil, id(f, "stale-token", "/en/view/tt2")); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("got = %v, want = %v", err, provider.ProviderError)
	}
	f.page("GET /en/view/tt3", `<input id="secCode" value="stale-token">`)
	if _, err := p.Download(t.Context(), nil, id(f, "stale-token", "/en/view/tt3")); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("got = %v, want = %v", err, provider.ProviderError)
	}
}

func TestDownloadForm(t *testing.T) {
	f := newFake(t)
	f.page("GET /dll/subtitle-id/0/fresh-token", `<input name="langcode" value="el">`+
		`<input name="uid" value="user-id"><input name="output" value="subtitle.srt"><input name="dll" value="download-id">`)
	f.mux.HandleFunc("POST /dll/subtitle-id/0/fresh-token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("uid") != "user-id" || r.Form.Get("dll") != "download-id" || r.Form.Get("langcode") != "el" {
			t.Errorf("form got = %v, want = the page's", r.Form)
		}
		_, _ = w.Write([]byte(srt))
	})
	p := f.provider(t)
	file, err := p.Download(t.Context(), nil, id(f, "fresh-token", "/en/view/tt0479997"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Format != "srt" || strings.Contains(string(file.Data), "\r\n") {
		t.Errorf("got = %q, %q, want = an srt without CRLF", file.Data, file.Format)
	}
	if _, err := p.Download(t.Context(), nil, "id=x"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("got = %v, want = %v", err, provider.BadSubtitle)
	}
}

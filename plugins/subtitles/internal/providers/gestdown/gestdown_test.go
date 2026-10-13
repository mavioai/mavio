package gestdown

import (
	"encoding/json"
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

const (
	breakingBad = "cd880e2e-ef44-47cd-9f3d-a03b343ba2d0"
	saul        = "a1b2c3d4-0000-0000-0000-000000000006"
)

// fake is the Gestdown API with Breaking Bad and Better Call Saul.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	locked   int
	requests []string
}

func newFake(t *testing.T) *fake {
	f := &fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shows/external/tvdb/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		switch r.PathValue("id") {
		case "81189":
			_, _ = w.Write([]byte(`{"shows":[{"id":"` + breakingBad + `","name":"Breaking Bad","nbSeasons":5,"seasons":[1,2,3,4,5],"tvDbId":81189}]}`))
		case "273181":
			_, _ = w.Write([]byte(`{"shows":[{"id":"` + saul + `","name":"Better Call Saul","nbSeasons":6,"seasons":[1,2,3,4,5,6],"tvDbId":273181}]}`))
		case "1":
			_, _ = w.Write([]byte(`not json`))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /subtitles/get/{show}/{season}/{episode}/{lang}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if r.UserAgent() != userAgent {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		f.mu.Lock()
		locked := f.locked > 0
		f.locked--
		f.mu.Unlock()
		if locked {
			w.WriteHeader(http.StatusLocked)
			return
		}
		key := r.PathValue("show") + "/" + r.PathValue("season") + "/" + r.PathValue("episode") + "/" + r.PathValue("lang")
		switch key {
		case breakingBad + "/1/1/English":
			_, _ = w.Write([]byte(`{"matchingSubtitles":[
				{"subtitleId":"bb1","version":"BluRayREWARD","completed":true,"hearingImpaired":false,"downloadUri":"/subtitles/download/bb1","qualities":["720p","1080p"]},
				{"subtitleId":"bb2","version":"HDTV, WEB","completed":true,"hearingImpaired":true,"downloadUri":"/subtitles/download/bb2","qualities":[]},
				{"subtitleId":"bb3","version":"Incomplete","completed":false,"hearingImpaired":false,"downloadUri":"/subtitles/download/bb3"}]}`))
		case saul + "/6/4/French":
			_, _ = w.Write([]byte(`{"matchingSubtitles":[
				{"subtitleId":"d28b4d5b-7dcc-47b3-8232-fb02f081d135","version":"AMZN-NTb","completed":true,"hearingImpaired":false,"downloadUri":"/subtitles/download/d28b4d5b-7dcc-47b3-8232-fb02f081d135","qualities":null}]}`))
		case saul + "/6/4/English":
			_, _ = w.Write([]byte(`{"matchingSubtitles":[]}`))
		case breakingBad + "/1/1/Portuguese (Brazilian)":
			_, _ = w.Write([]byte(`{"matchingSubtitles":[{"subtitleId":"pt","version":"WEB","completed":true,"downloadUri":"/subtitles/download/pt"}]}`))
		case breakingBad + "/1/1/French":
			_, _ = w.Write([]byte(`{"unexpected":[]}`))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /subtitles/download/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("id") {
		case "bb1":
			_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n"))
		case "empty":
		default:
			http.NotFound(w, r)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) record(r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.URL.Path)
	f.mu.Unlock()
}

func newProvider(t *testing.T, f *fake) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).wait = 0
	return p.(*Provider)
}

// episodes are the Python tests' episodes.
var episodes = map[string]*provider.Video{
	"breaking_bad_s01e01": {
		Episode: true, Series: "Breaking Bad", Season: 1, Number: 1, Source: "Blu-ray", ReleaseGroup: "REWARD",
		Resolution: "720p", VideoCodec: "H.264", SeriesExternalIDs: map[string]string{"tvdb": "81189", "imdb": "tt0903747"},
	},
	"better_call_saul_s06e04": {
		Episode: true, Series: "Better Call Saul", Season: 6, Number: 4, Source: "Web",
		Resolution: "720p", VideoCodec: "H.264", SeriesExternalIDs: map[string]string{"tvdb": "273181"},
	},
	"inexistent": {Episode: true, Series: "121361asdfgh", Season: 1, Number: 1, Source: "Blu-ray"},
}

func TestPorted(t *testing.T) {
	ported.Run(t, "gestdown.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_list_subtitles": func(t *testing.T, args map[string]json.RawMessage) {
				v := episodes[ported.Arg[string](t, args, "episode_key")]
				expr := ported.Arg[map[string]string](t, args, "language")["$expr"]
				tag := strings.TrimSuffix(strings.TrimPrefix(expr, `Language.fromietf("`), `")`)
				want := ported.Arg[string](t, args, "expected_any_release_info")
				subs, err := newProvider(t, newFake(t)).Search(t.Context(), v, []provider.Language{{Tag: tag}})
				if err != nil {
					t.Fatal(err)
				}
				if !slices.ContainsFunc(subs, func(s provider.Subtitle) bool { return slices.Equal(s.ReleaseNames, []string{want}) }) {
					t.Errorf("subtitles got = %+v, want = one of %s", subs, want)
				}
			},
		},
		Skip: map[string]string{
			"test_subtitle_get_matches_release_group": "the server matches release groups from the subtitle's release names",
		},
		Facts: map[string]string{
			"test_language_list_is_convertible":                   "TestLanguages",
			"test_list_subtitles_hearing_impaired":                "TestSearch",
			"test_list_subtitles_inexistent":                      "TestSearchInexistent",
			"test_subtitle":                                       "TestSearch",
			"test_subtitle_get_matches":                           "TestSearch",
			"test_subtitle_multi_release_version":                 "TestSearch",
			"test_subtitle_get_matches_resolution_from_qualities": "TestSearch",
			"test_subtitle_download":                              "TestDownload",
			"test_list_subtitles_423":                             "TestLocked",
		},
	})
}

func TestLanguages(t *testing.T) {
	for _, tag := range Definition().Languages {
		if siteName(tag) == "" {
			t.Errorf("siteName(%s) got = \"\", want = a name", tag)
		}
	}
	for tag, want := range map[string]string{
		"pt-BR": "Portuguese (Brazilian)", "pt": "Portuguese", "ca": "Català", "zh": "Chinese (Simplified)",
		"sr-Cyrl": "Serbian (Cyrillic)", "sr-Latn": "Serbian", "el": "Greek", "fr-CA": "French", "xx": "",
	} {
		if got := siteName(tag); got != want {
			t.Errorf("siteName(%s) got = %q, want = %q", tag, got, want)
		}
	}
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	// English is searched once for the normal and hearing-impaired
	// variants.
	subs, err := p.Search(t.Context(), episodes["breaking_bad_s01e01"], []provider.Language{{Tag: "en"}, {Tag: "en", HearingImpaired: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("subtitles got = %+v, want = 2", subs)
	}
	s := subs[0]
	if s.ID != "/subtitles/download/bb1" || s.PageLink != f.srv.URL+"/subtitles/download/bb1" || s.Language != (provider.Language{Tag: "en"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	// The TVDB ID matches, as does a resolution of the qualities.
	if want := []string{"title", "series", "season", "episode", "tvdb_id", "resolution", "year"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	s = subs[1]
	if !s.Language.HearingImpaired || !slices.Equal(s.ReleaseNames, []string{"HDTV", "WEB"}) {
		t.Errorf("subtitle got = %+v, want = hearing-impaired of HDTV and WEB", s)
	}
	if slices.Contains(s.Matches, "resolution") {
		t.Errorf("matches got = %v, want = no resolution", s.Matches)
	}
	if n := strings.Count(strings.Join(f.requests, " "), "/English"); n != 1 {
		t.Errorf("English searches got = %d, want = 1", n)
	}

	subs, err = p.Search(t.Context(), episodes["breaking_bad_s01e01"], []provider.Language{{Tag: "pt-BR"}, {Tag: "fr"}})
	if err != nil || len(subs) != 1 || subs[0].Language != (provider.Language{Tag: "pt-BR"}) {
		t.Errorf("subtitles got = %+v, %v, want = one in pt-BR", subs, err)
	}
}

func TestSearchInexistent(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := *episodes["inexistent"]
	if subs, err := p.Search(t.Context(), &v, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 || len(f.requests) != 0 {
		t.Errorf("without a TVDB ID got = %v, %v, %v, want = none", subs, err, f.requests)
	}
	for _, tvdb := range []string{"999", "1"} {
		v.SeriesExternalIDs = map[string]string{"tvdb": tvdb}
		if subs, err := p.Search(t.Context(), &v, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
			t.Errorf("show %s got = %v, %v, want = none", tvdb, subs, err)
		}
	}
	// An episode the API does not know empties the search.
	saul := *episodes["better_call_saul_s06e04"]
	saul.Number = 5
	if subs, err := p.Search(t.Context(), &saul, []provider.Language{{Tag: "fr"}}); err != nil || len(subs) != 0 {
		t.Errorf("unknown episode got = %v, %v, want = none", subs, err)
	}
}

func TestLocked(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	f.locked = 3
	if subs, err := p.Search(t.Context(), episodes["breaking_bad_s01e01"], []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Errorf("locked got = %v, %v, want = none", subs, err)
	}
	f.locked = 2
	if subs, err := p.Search(t.Context(), episodes["breaking_bad_s01e01"], []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 2 {
		t.Errorf("locked twice got = %v, %v, want = 2 subtitles", subs, err)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	got, err := p.Download(t.Context(), nil, "/subtitles/download/bb1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "srt" || !strings.Contains(string(got.Data), "Hi") {
		t.Errorf("download got = %q, %s", got.Data, got.Format)
	}
	for id, want := range map[string]provider.Kind{
		"/subtitles/download/none":  provider.ProviderError,
		"/subtitles/download/empty": provider.ProviderError,
		"http://elsewhere/x":        provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != want {
			t.Errorf("download of %s got = %v, want = %v", id, err, want)
		}
	}
}

func TestStatus(t *testing.T) {
	for code, want := range map[int]provider.Kind{
		http.StatusTooManyRequests: provider.TooManyRequests, http.StatusServiceUnavailable: provider.ServiceUnavailable,
		http.StatusBadRequest: provider.ProviderError,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		p, _ := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
		u, _ := url.Parse(srv.URL)
		p.(*Provider).SetBaseURL(u)
		if _, err := p.Search(t.Context(), episodes["breaking_bad_s01e01"], []provider.Language{{Tag: "en"}}); provider.KindOf(err) != want {
			t.Errorf("status %d got = %v, want = %v", code, err, want)
		}
		srv.Close()
	}
}

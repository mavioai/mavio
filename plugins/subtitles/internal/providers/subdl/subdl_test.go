package subdl

import (
	"archive/zip"
	"bytes"
	"context"
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

func TestPorted(t *testing.T) {
	live := "needs the live SubDL API and an API key"
	ported.Run(t, "subdl.json", ported.Cases{
		Skip: map[string]string{
			"test_list_subtitles_movie": live,
			"test_download_subtitle":    live,
		},
		Facts: map[string]string{
			"test_ai_candidate_synthesized":                              "TestTranslationCandidates",
			"test_ai_candidate_requires_setting":                         "TestTranslationCandidates",
			"test_no_candidate_when_not_entitled":                        "TestTranslationCandidates",
			"test_no_candidate_when_quota_exhausted":                     "TestTranslationCandidates",
			"test_no_candidate_when_language_already_found":              "TestTranslationCandidates",
			"test_graceful_without_translation_block":                    "TestTranslationCandidates",
			"test_hi_source_does_not_silence_non_hi_candidate":           "TestTranslationCandidates",
			"test_remote_policy_disables_ai_candidates":                  "TestTranslationCandidates",
			"test_other_episodes_row_does_not_suppress_ai_candidate":     "TestTranslationCandidatesOfEpisodes",
			"test_same_episode_row_still_suppresses_ai_candidate":        "TestTranslationCandidatesOfEpisodes",
			"test_include_ai_translated_filter":                          "TestTranslatedRows",
			"test_ai_translate_implies_including_ai_rows":                "TestTranslatedRows",
			"test_download_ai_translation":                               "TestDownloadTranslation",
			"test_download_ai_translation_reused_job":                    "TestDownloadTranslation",
			"test_download_ai_translation_quota_exhausted_never_raises":  "TestDownloadTranslation",
			"test_download_ai_translation_job_failure":                   "TestDownloadTranslation",
			"test_quota_exhausted_pauses_candidates":                     "TestQuotaPausesCandidates",
			"test_search_402_raises_provider_error":                      "TestStatusCodes",
			"test_search_404_is_service_unavailable":                     "TestStatusCodes",
			"test_is_hi_marker_in_comment":                               "TestHearingImpaired",
			"test_is_hi_marker_in_release_name":                          "TestHearingImpaired",
			"test_is_hi_negative":                                        "TestHearingImpaired",
			"test_select_unpack_entry_rejects_contradicted_claim":        "TestSelectUnpack",
			"test_select_unpack_entry_uses_name_when_server_has_nothing": "TestSelectUnpack",
			"test_select_unpack_entry_prefers_matching_hi_class":         "TestSelectUnpack",
			"test_select_unpack_entry_none_when_nothing_matches":         "TestSelectUnpack",
			"test_traditional_chinese_reaches_subdl":                     "TestTraditionalChinese",
			"test_hant_alias_is_not_advertised_without_matching_results": "TestTraditionalChinese",
			"test_filename_selected_unpack_file_keeps_episode_match":     "TestUnpackedFiles",
			"test_unpack_selection_uses_effective_hi_class":              "TestUnpackedFiles",
			"test_remote_policy_limits_pages_and_disables_fallbacks":     "TestRemotePolicy",
			"test_remote_policy_disables_provider":                       "TestRemotePolicy",
		},
	})
}

const (
	matchingRelease = "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX"
	otherRelease    = "Dune.2021.720p.BluRay.x264-OTHER"
)

func dune() *provider.Video {
	return &provider.Video{
		Title: "Dune", Year: 2021, Resolution: "1080p", Source: "Web", ExternalIDs: map[string]string{"imdb": "tt1160419"},
		AudioCodec: "Dolby Digital", VideoCodec: "H.264", ReleaseGroup: "SHITBOX", FileName: "Dune.2021.1080p.WEBRip.DD5.1.x264-SHITBOX",
	}
}

func got() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Game of Thrones", Season: 3, Number: 10, Title: "Mhysa", Source: "Web", ReleaseGroup: "NTb",
		Resolution: "720p", SeriesExternalIDs: map[string]string{"imdb": "tt0944947"},
		FileName: "Game.of.Thrones.S03E10.Mhysa.720p.WEB-DL.DD5.1.H.264-NTb.mkv",
	}
}

// fakeAPI is SubDL's API answering searches with a body, and its
// translation jobs.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	search   func(q url.Values) (int, any)
	searches []url.Values
	routes   map[string]func(w http.ResponseWriter, r *http.Request)
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		route, ok := f.routes[r.Method+" "+r.URL.Path]
		search := f.search
		f.mu.Unlock()
		if ok {
			route(w, r)
			return
		}
		if r.URL.Path != "/api/v1/subtitles" || search == nil {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("api_key") != "fake-key" {
			t.Errorf("api_key = %q, want = fake-key", r.URL.Query().Get("api_key"))
		}
		f.mu.Lock()
		f.searches = append(f.searches, r.URL.Query())
		f.mu.Unlock()
		code, body := search(r.URL.Query())
		reply(w, code, body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func reply(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if s, ok := body.(string); ok {
		_, _ = w.Write([]byte(s))
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (f *fakeAPI) answer(body any) {
	f.search = func(url.Values) (int, any) { return http.StatusOK, body }
}

func (f *fakeAPI) route(pattern string, h func(w http.ResponseWriter, r *http.Request)) {
	f.mu.Lock()
	f.routes[pattern] = h
	f.mu.Unlock()
}

func newProvider(t *testing.T, f *fakeAPI, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	sp := p.(*Provider)
	sp.SetBaseURL(u)
	sp.sleep = func(context.Context, time.Duration) error { return nil }
	sp.poll = 0
	return sp
}

const (
	plain = `{"api_key":"fake-key"}`
	ai    = `{"api_key":"fake-key","ai_translate":true}`
)

type m = map[string]any

func searchResponse(translation any, subtitles ...m) m {
	body := m{
		"status":    true,
		"results":   []m{{"imdb_id": "tt1160419", "tmdb_id": 438631, "type": "movie", "name": "Dune", "sd_id": 123456, "year": 2021}},
		"subtitles": subtitles,
	}
	if subtitles == nil {
		body["subtitles"] = []m{}
	}
	if translation != nil {
		body["translation"] = translation
	}
	return body
}

func entitled(sources []m, missing ...string) m {
	if sources == nil {
		sources = []m{
			{"n_id": "src2", "language": "EN", "hi": false, "releases": []string{otherRelease}},
			{"n_id": "src1", "language": "EN", "hi": false, "releases": []string{matchingRelease}},
		}
	}
	if missing == nil {
		missing = []string{"FA"}
	}
	return m{"entitled": true, "quota_remaining": 42, "quota_reset_at": "2026-08-01T00:00:00.000Z", "missing_languages": missing, "sources": sources}
}

var farsi = []provider.Language{{Tag: "fa"}}

func TestTranslationCandidates(t *testing.T) {
	existing := m{"url": "/subtitle/1-1.zip", "name": "dune-fa.zip", "language": "FA", "subtitlePage": "/s/info/abc", "releases": []string{matchingRelease}, "author": "someone"}
	withPolicy := searchResponse(entitled(nil))
	withPolicy["bazarr_policy"] = m{"revision": 4, "enabled": true, "ai_translation_enabled": false, "max_pages": 2}
	for _, c := range []struct {
		name     string
		settings string
		body     any
		want     []string
	}{
		// The source whose releases match the video wins, not the first.
		{"synthesized", ai, searchResponse(entitled(nil)), []string{"ai:src1:FA"}},
		{"requires the setting", plain, searchResponse(entitled(nil)), nil},
		{"not entitled", ai, searchResponse(m{"entitled": false, "quota_remaining": 0, "missing_languages": []string{"FA"}, "upgrade_url": "https://subdl.com/pro?ref=bazarr"}), nil},
		{"quota exhausted", ai, searchResponse(m{"entitled": true, "quota_remaining": 0, "missing_languages": []string{"FA"}}), nil},
		{"language already found", ai, searchResponse(entitled(nil), existing), []string{"/subtitle/1-1.zip"}},
		{"no translation block", ai, searchResponse(nil), nil},
		// An HI source matching best does not silence the non-HI candidate.
		{"per HI class", ai, searchResponse(entitled([]m{
			{"n_id": "srcHI", "language": "EN", "hi": true, "releases": []string{matchingRelease}},
			{"n_id": "srcNonHI", "language": "EN", "hi": false, "releases": []string{otherRelease}},
		})), []string{"ai:srcNonHI:FA"}},
		{"disabled by the remote policy", ai, withPolicy, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.answer(c.body)
			subs, err := newProvider(t, f, c.settings).Search(t.Context(), dune(), farsi)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(subs); !slices.Equal(got, c.want) {
				t.Fatalf("subtitles = %v, want = %v", got, c.want)
			}
			if len(subs) == 1 && strings.HasPrefix(subs[0].ID, "ai:") {
				s := subs[0]
				if s.Language != (provider.Language{Tag: "fa"}) || s.Uploader != "SubDL AI" || !strings.HasPrefix(s.Comment, "AI translation (SubDL) from EN") ||
					!slices.Contains(s.Matches, "title") {
					t.Errorf("candidate = %+v", s)
				}
			}
		})
	}
}

func TestTranslationCandidatesOfEpisodes(t *testing.T) {
	row := func(episode int) m {
		return m{
			"url": "/subtitle/9-9.zip", "name": "Game.of.Thrones.S03E01.720p.WEB-DL.fa.zip", "language": "FA", "subtitlePage": "/s/info/x",
			"releases": []string{"Game.of.Thrones.S03E01.720p.WEB-DL.DD5.1.H.264-NTb"}, "author": "someone", "season": 3, "episode": episode,
		}
	}
	sources := []m{{"n_id": "srcE10", "language": "EN", "hi": false, "releases": []string{"Game.of.Thrones.S03E10.Mhysa.720p.WEB-DL.DD5.1.H.264-NTb"}}}
	for _, c := range []struct {
		name    string
		episode int
		want    []string
	}{
		// A row of another episode does not count as the language found.
		{"other episode", 1, []string{"ai:srcE10:FA"}},
		{"same episode", 10, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.answer(searchResponse(entitled(sources), row(c.episode)))
			subs, err := newProvider(t, f, ai).Search(t.Context(), got(), farsi)
			if err != nil {
				t.Fatal(err)
			}
			var candidates []string
			for _, s := range subs {
				if strings.HasPrefix(s.ID, "ai:") {
					candidates = append(candidates, s.ID)
				}
			}
			if !slices.Equal(candidates, c.want) {
				t.Errorf("candidates = %v, want = %v", candidates, c.want)
			}
		})
	}
}

func TestTranslatedRows(t *testing.T) {
	item := m{
		"url": "/subtitle/1-1.zip", "name": "dune-fa-ai.zip", "language": "FA", "subtitlePage": "/s/info/abc",
		"releases": []string{matchingRelease}, "author": "someone", "ai_translated": true,
	}
	for _, c := range []struct {
		settings string
		want     int
	}{
		{`{"api_key":"fake-key","include_ai_translated":false}`, 0},
		{`{"api_key":"fake-key","include_ai_translated":true}`, 1},
		// Asking SubDL for translations accepts them.
		{`{"api_key":"fake-key","ai_translate":true,"include_ai_translated":false}`, 1},
	} {
		f := newFake(t)
		f.answer(searchResponse(nil, item))
		subs, err := newProvider(t, f, c.settings).Search(t.Context(), dune(), farsi)
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != c.want {
			t.Fatalf("%s: subtitles = %d, want = %d", c.settings, len(subs), c.want)
		}
		if len(subs) == 1 && subs[0].Uploader != "someone (AI translated)" {
			t.Errorf("uploader = %q, want = someone (AI translated)", subs[0].Uploader)
		}
	}
}

const translated = "1\n00:00:01,000 --> 00:00:02,000\nsalam\n"

func TestDownloadTranslation(t *testing.T) {
	queued := m{"status": true, "request_id": "job1", "job": m{"status": "queued", "download_ready": false}, "estimated_duration_ms": 30000}
	for _, c := range []struct {
		name   string
		status int
		submit m
		job    m
		ok     bool
	}{
		{"polled", http.StatusAccepted, queued, m{"job": m{"status": "translated", "download_ready": true}}, true},
		{"reused job", http.StatusOK, m{"status": true, "request_id": "job1", "reused": true, "job": m{"status": "published", "download_ready": true}}, nil, true},
		// The quota fails this candidate only, never throttling downloads.
		{"quota exhausted", http.StatusTooManyRequests, m{"status": false, "error": "translation_quota_exhausted", "message": "Monthly translation quota exhausted"}, nil, false},
		{"job failure", http.StatusAccepted, queued, m{"job": m{"status": "failed", "error": "boom"}}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.route("POST /api/v1/pro/translate/subtitles", func(w http.ResponseWriter, r *http.Request) {
				var body m
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["n_id"] != "src1" || body["target_language"] != "FA" || body["season"] != nil {
					t.Errorf("translation request = %v", body)
				}
				reply(w, c.status, c.submit)
			})
			f.route("GET /api/v1/pro/translate/jobs/job1", func(w http.ResponseWriter, r *http.Request) { reply(w, http.StatusOK, c.job) })
			f.route("GET /api/v1/pro/translate/jobs/job1/download", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(translated))
			})
			file, err := newProvider(t, f, ai).Download(t.Context(), dune(), "ai:src1:FA")
			if !c.ok {
				if k := provider.KindOf(err); k != provider.BadSubtitle {
					t.Errorf("kind = %v, want = BadSubtitle (%v)", k, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(file.Data) != translated || file.Format != "srt" || file.Language.Tag != "fa" {
				t.Errorf("file = %q %s %+v", file.Data, file.Format, file.Language)
			}
		})
	}
}

func TestQuotaPausesCandidates(t *testing.T) {
	f := newFake(t)
	f.answer(searchResponse(entitled(nil)))
	f.route("POST /api/v1/pro/translate/subtitles", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusTooManyRequests, m{"status": false, "error": "translation_quota_exhausted"})
	})
	p := newProvider(t, f, ai)
	first, err := p.Search(t.Context(), dune(), farsi)
	if err != nil || len(first) != 1 {
		t.Fatalf("first search = %v, %v, want one candidate", ids(first), err)
	}
	if _, err := p.Download(t.Context(), dune(), first[0].ID); err == nil {
		t.Fatal("download succeeded, want a failure")
	}
	if subs, err := p.Search(t.Context(), dune(), farsi); err != nil || len(subs) != 0 {
		t.Errorf("search after the quota = %v, %v, want none", ids(subs), err)
	}
}

func TestStatusCodes(t *testing.T) {
	for _, c := range []struct {
		code int
		body any
		kind provider.Kind
	}{
		{http.StatusPaymentRequired, m{"status": false, "error": "paid_api_required", "message": "Active paid API subscription required"}, provider.ProviderError},
		// A bare 404 means the search is not served at all.
		{http.StatusNotFound, "Not found", provider.ServiceUnavailable},
		{http.StatusForbidden, m{"error": "Invalid api key"}, provider.AuthenticationError},
		{http.StatusForbidden, "<html>challenge</html>", provider.APIThrottled},
		{http.StatusTooManyRequests, m{"error": "daily_limit"}, provider.DownloadLimitExceeded},
		{http.StatusTooManyRequests, m{"error": "rate_limit"}, provider.APIThrottled},
		{http.StatusTooManyRequests, m{"error": "service_busy"}, provider.ProviderError},
		{http.StatusTooManyRequests, "", provider.APIThrottled},
		{http.StatusBadGateway, "", provider.ServiceUnavailable},
	} {
		f := newFake(t)
		f.search = func(url.Values) (int, any) { return c.code, c.body }
		p := newProvider(t, f, plain)
		_, err := p.Search(t.Context(), dune(), []provider.Language{{Tag: "en"}})
		if k := provider.KindOf(err); k != c.kind {
			t.Errorf("%d %v: kind = %v, want = %v (%v)", c.code, c.body, k, c.kind, err)
		}
		if c.code == http.StatusPaymentRequired && (err == nil || !strings.Contains(err.Error(), "Active paid API subscription required")) {
			t.Errorf("402: err = %v", err)
		}
	}
	// Rate limits are waited out six times, then throttle.
	f := newFake(t)
	f.search = func(url.Values) (int, any) { return http.StatusTooManyRequests, m{"error": "rate_limit"} }
	_, _ = newProvider(t, f, plain).Search(t.Context(), dune(), []provider.Language{{Tag: "en"}})
	if len(f.searches) != 7 {
		t.Errorf("tries = %d, want = 7", len(f.searches))
	}
	// Any other client error costs that search alone.
	f = newFake(t)
	f.search = func(url.Values) (int, any) {
		return http.StatusBadRequest, m{"error": "Film name contains potentially unsafe characters"}
	}
	if subs, err := newProvider(t, f, plain).Search(t.Context(), dune(), []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Errorf("rejected search = %v, %v, want none", subs, err)
	}
}

func TestHearingImpaired(t *testing.T) {
	for _, c := range []struct {
		it   item
		want bool
	}{
		{item{Comment: "SDH version", Name: "dune-en.zip", Releases: strs{matchingRelease}}, true},
		{item{Name: "dune-en.zip", Releases: strs{"Dune.2021.1080p.WEBRip.DD5.1.x264.SDH-SHITBOX"}}, true},
		{item{Comment: "great subtitle", Name: "dune-en.zip", Releases: strs{matchingRelease, otherRelease}}, false},
		{item{Name: "Show.Hive-CM8.Hi10P.Hindi.zip"}, false},
		{item{Comment: "removed SDH", HI: true}, false},
	} {
		if got := isHI(&c.it) || bool(c.it.HI) && hearingImpaired(&c.it); got != c.want {
			t.Errorf("hearing impaired %+v = %v, want = %v", c.it, got, c.want)
		}
	}
	if !isForced(&item{Releases: strs{"Movie.Foreign.Parts.Only"}}) || isForced(&item{Comment: "a foreign film"}) {
		t.Error("forced markers misread")
	}
}

func TestSelectUnpack(t *testing.T) {
	file := func(name string, episode int, hi bool) unpackFile {
		return unpackFile{Name: name, Episode: num{episode, true}, HI: flag(hi)}
	}
	for _, c := range []struct {
		files    []unpackFile
		target   int
		preferHI bool
		want     string
	}{
		// The name contradicting the server's claim wins.
		{[]unpackFile{file("03 ...And the Bag's in the River.en.srt", 1, false), file("01 Pilot.en.srt", 0, false)}, 1, false, "01 Pilot.en.srt"},
		{[]unpackFile{file("05 Gray Matter.en.srt", 0, false)}, 5, false, "05 Gray Matter.en.srt"},
		{[]unpackFile{file("Show.S01E05.SDH.srt", 5, true), file("Show.S01E05.srt", 5, false)}, 5, false, "Show.S01E05.srt"},
		{[]unpackFile{file("Show.S01E05.SDH.srt", 5, true), file("Show.S01E05.srt", 5, false)}, 5, true, "Show.S01E05.SDH.srt"},
		{[]unpackFile{file("02 Other.en.srt", 2, false)}, 7, false, ""},
	} {
		got := ""
		if f := selectUnpack(c.files, c.target, 0, c.preferHI); f != nil {
			got = f.Name
		}
		if got != c.want {
			t.Errorf("selected = %q, want = %q", got, c.want)
		}
	}
}

func TestTraditionalChinese(t *testing.T) {
	f := newFake(t)
	f.answer(searchResponse(nil, m{"url": "/subtitle/1-1.zip", "name": "dune-zh-bg.zip", "language": "ZH_BG", "subtitlePage": "/s/info/zht", "releases": []string{matchingRelease}, "author": "someone"}))
	subs, err := newProvider(t, f, plain).Search(t.Context(), dune(), []provider.Language{{Tag: "zh-TW"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].Language.Tag != "zh-TW" {
		t.Errorf("subtitles = %+v, want one in zh-TW", subs)
	}
	if got := f.searches[0].Get("languages"); got != "ZH_BG" {
		t.Errorf("languages = %s, want = ZH_BG", got)
	}
	if slices.Contains(Definition().Languages, "zh-Hant") {
		t.Error("zh-Hant is advertised")
	}
}

func showVideo() *provider.Video {
	return &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 5, SeriesExternalIDs: map[string]string{"imdb": "tt1234567"}, FileName: "Show.S01E05.mkv"}
}

func TestUnpackedFiles(t *testing.T) {
	f := newFake(t)
	f.answer(m{"status": true, "totalPages": 1, "subtitles": []m{{
		"url": "/subtitle/show-season-1.zip", "name": "show-season-1.zip", "language": "EN", "subtitlePage": "/s/info/season",
		"releases": []string{"Show.S01.Complete"}, "author": "someone", "season": 1, "episode": 0, "full_season": true,
		"unpack_files": []m{{"url": "/subtitle/show-s01e05.srt", "file_n_id": "file5", "name": "05 Episode Five.en.srt", "season": 1, "episode": 0, "hi": false}},
	}}})
	p := newProvider(t, f, plain)
	subs, err := p.Search(t.Context(), showVideo(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].ID != "file:/subtitle/show-s01e05.srt" || !slices.Contains(subs[0].Matches, "episode") {
		t.Fatalf("subtitles = %+v", subs)
	}
	if want := []string{"series", "series_imdb_id", "episode", "season"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("matches = %v, want = %v", subs[0].Matches, want)
	}
	if q := f.searches[0]; q.Get("imdb_id") != "tt1234567" || q.Get("type") != "tv" || q.Get("episode_number") != "5" || q.Get("season_number") != "1" ||
		q.Get("unpack") != "1" || q.Get("subs_per_page") != "30" || q.Get("bazarr") != "1" {
		t.Errorf("query = %v", q)
	}

	// The pack's HI class is that of its comment, not of the API's flag.
	f = newFake(t)
	f.answer(m{"status": true, "totalPages": 1, "subtitles": []m{{
		"url": "/subtitle/show-season-1.zip", "name": "show-season-1.zip", "language": "EN", "subtitlePage": "/s/info/season-hi",
		"releases": []string{"Show.S01.Complete"}, "comment": "SDH version", "author": "someone", "season": 1, "episode": 0, "hi": false, "full_season": true,
		"unpack_files": []m{
			{"url": "/subtitle/show-s01e05.srt", "file_n_id": "normal5", "name": "Show.S01E05.srt", "season": 1, "episode": 5, "hi": false},
			{"url": "/subtitle/show-s01e05-sdh.srt", "file_n_id": "sdh5", "name": "Show.S01E05.SDH.srt", "season": 1, "episode": 5, "hi": true},
		},
	}}})
	subs, err = newProvider(t, f, plain).Search(t.Context(), showVideo(), []provider.Language{{Tag: "en", HearingImpaired: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || subs[0].ID != "file:/subtitle/show-s01e05-sdh.srt" || !subs[0].Language.HearingImpaired {
		t.Errorf("subtitles = %+v", subs)
	}
}

func TestRemotePolicy(t *testing.T) {
	f := newFake(t)
	f.answer(m{"status": true, "subtitles": []m{}, "totalPages": 4, "bazarr_policy": m{
		"revision": 2, "enabled": true, "ai_translation_enabled": true, "max_pages": 1,
		"season_fallback_enabled": false, "title_fallback_enabled": false, "unpack_enabled": false, "message": nil,
	}})
	p := newProvider(t, f, plain)
	if subs, err := p.Search(t.Context(), got(), []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Fatalf("search = %v, %v, want none", subs, err)
	}
	if len(f.searches) != 1 {
		t.Errorf("searches = %d, want = 1", len(f.searches))
	}
	// Later searches leave out unpacking.
	if _, err := p.Search(t.Context(), got(), []provider.Language{{Tag: "en"}}); err != nil {
		t.Fatal(err)
	}
	if f.searches[1].Has("unpack") {
		t.Errorf("query = %v, want no unpack", f.searches[1])
	}

	f = newFake(t)
	f.answer(m{"status": false, "error": "provider_disabled", "bazarr_policy": m{
		"revision": 3, "enabled": false, "ai_translation_enabled": false, "max_pages": 1,
		"season_fallback_enabled": false, "title_fallback_enabled": false, "unpack_enabled": false, "message": "Maintenance",
	}})
	if subs, err := newProvider(t, f, plain).Search(t.Context(), dune(), []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Fatalf("search = %v, %v, want none", subs, err)
	}
	if len(f.searches) != 1 {
		t.Errorf("searches = %d, want = 1", len(f.searches))
	}
}

func TestSearchFallbacks(t *testing.T) {
	f := newFake(t)
	f.search = func(q url.Values) (int, any) {
		if q.Get("tmdb_id") == "438631" {
			return http.StatusOK, searchResponse(nil, m{"url": "/subtitle/2-2.zip", "name": "dune-en.zip", "language": "EN", "subtitlePage": "/s/info/en", "releases": []string{matchingRelease}})
		}
		return http.StatusOK, searchResponse(nil)
	}
	v := dune()
	v.ExternalIDs = map[string]string{"tmdb": "438631"}
	v.Title = "Grey's Anatomy"
	subs, err := newProvider(t, f, plain).Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.searches[0].Get("film_name"); got != "Grey s Anatomy" {
		t.Errorf("film_name = %q, want = Grey s Anatomy", got)
	}
	if f.searches[1].Has("film_name") || f.searches[1].Get("tmdb_id") != "438631" {
		t.Errorf("TMDB query = %v", f.searches[1])
	}
	// The TMDB ID found them, so they match it.
	if len(subs) != 1 || !slices.Equal(subs[0].Matches, []string{"tmdb_id", "title", "year"}) || subs[0].PageLink != "https://subdl.com/s/info/en" {
		t.Errorf("subtitles = %+v", subs)
	}

	// An episode is searched by its episode, its absolute number, its
	// season, and its title last.
	f = newFake(t)
	f.answer(searchResponse(nil))
	v = got()
	v.AbsoluteNumber = 30
	if _, err := newProvider(t, f, plain).Search(t.Context(), v, []provider.Language{{Tag: "en"}, {Tag: "pt-BR"}}); err != nil {
		t.Fatal(err)
	}
	var searched []string
	for _, q := range f.searches {
		searched = append(searched, q.Get("episode_number")+"/"+q.Get("season_number"))
	}
	if want := []string{"10/3", "30/", "/3", "/"}; !slices.Equal(searched, want) {
		t.Errorf("searches = %v, want = %v", searched, want)
	}
	if got := f.searches[0].Get("languages"); got != "BR_PT,EN" {
		t.Errorf("languages = %s, want = BR_PT,EN", got)
	}
}

func TestPacks(t *testing.T) {
	f := newFake(t)
	f.answer(searchResponse(nil,
		m{"url": "/subtitle/pack.zip", "name": "a", "language": "EN", "subtitlePage": "/a", "releases": []string{"Game.of.Thrones.S03E01-E05"}, "season": 3},
		m{"url": "/subtitle/pack2.zip", "name": "b", "language": "EN", "subtitlePage": "/b", "releases": []string{"Game.of.Thrones.S03.Complete"}, "season": 3, "episode_from": 8, "episode_end": 12},
		m{"url": "/subtitle/one.zip", "name": "c", "language": "EN", "subtitlePage": "/c", "releases": []string{"x"}, "season": 3, "episode": 10, "episode_from": 10, "episode_end": 10},
	))
	subs, err := newProvider(t, f, plain).Search(t.Context(), got(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	// The pack of episodes 1 to 5 is left out.
	if want := []string{"pack:/subtitle/pack2.zip", "/subtitle/one.zip"}; !slices.Equal(ids(subs), want) {
		t.Fatalf("subtitles = %v, want = %v", ids(subs), want)
	}
	if want := []string{"series", "series_imdb_id", "episode", "season"}; !slices.Equal(subs[0].Matches, want) {
		t.Errorf("pack matches = %v, want = %v", subs[0].Matches, want)
	}
}

func zipOf(t *testing.T, files map[string]string, order ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range order {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

func TestDownload(t *testing.T) {
	f := newFake(t)
	files := map[string]string{
		"__MACOSX/._a.srt":           "junk",
		"Game.of.Thrones.S03E09.srt": "1\n00:00:01,000 --> 00:00:02,000\nnine\n",
		"Game.of.Thrones.S03E10.srt": "1\n00:00:01,000 --> 00:00:02,000\nten\n",
	}
	single := zipOf(t, files, "__MACOSX/._a.srt", "Game.of.Thrones.S03E09.srt")
	pack := zipOf(t, files, "Game.of.Thrones.S03E09.srt", "Game.of.Thrones.S03E10.srt")
	f.route("GET /subtitle/one.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(single) })
	f.route("GET /subtitle/pack.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(pack) })
	f.route("GET /subtitle/e10.srt", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(srt)) })
	f.route("GET /subtitle/junk.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("not a subtitle")) })
	p := newProvider(t, f, plain)
	for _, c := range []struct {
		id, want string
		kind     provider.Kind
	}{
		{"/subtitle/one.zip", "nine", 0},
		{"pack:/subtitle/pack.zip", "ten", 0},
		{"file:/subtitle/e10.srt", "Hello", 0},
		{"/subtitle/junk.zip", "", provider.BadSubtitle},
		// A subtitle that no longer exists fails alone.
		{"/subtitle/gone.zip", "", provider.BadSubtitle},
	} {
		file, err := p.Download(t.Context(), got(), c.id)
		if c.kind != 0 {
			if k := provider.KindOf(err); k != c.kind {
				t.Errorf("%s: kind = %v, want = %v (%v)", c.id, k, c.kind, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.id, err)
			continue
		}
		if !strings.Contains(string(file.Data), c.want) || strings.Contains(string(file.Data), "\r") || file.Format != "srt" {
			t.Errorf("%s: file = %q %s, want %s", c.id, file.Data, file.Format, c.want)
		}
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{}`, `{"ai_translate":true}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s): err = %v, want = ConfigurationError", s, err)
		}
	}
}

func ids(subs []provider.Subtitle) []string {
	var out []string
	for _, s := range subs {
		out = append(out, s.ID)
	}
	return out
}

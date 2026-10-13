package assrt

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\n你好\r\n"

// fakeAPI is Assrt's API with a quota of 20 requests a minute.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	search  string
	detail  string
	queries []string
	quota   string
	status  int
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, quota: `{"user":{"quota":20}}`}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/user/quota", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		_, _ = w.Write([]byte(f.quota))
	})
	mux.HandleFunc("GET /v1/sub/search", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		f.mu.Lock()
		f.queries = append(f.queries, r.URL.Query().Get("q")+"|"+r.URL.Query().Get("is_file"))
		f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
		}
		_, _ = w.Write([]byte(f.search))
	})
	mux.HandleFunc("GET /v1/sub/detail", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		_, _ = w.Write([]byte(f.detail))
	})
	mux.HandleFunc("GET /file/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s%s", srt, r.URL.Path)
	})
	mux.HandleFunc("GET /dl/sub.srt", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(srt)) })
	mux.HandleFunc("GET /dl/pack.zip", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for _, n := range []string{"Show.S01E01.chs.srt", "Show.S01E02.chs.srt"} {
			fw, _ := zw.Create(n)
			_, _ = fw.Write([]byte(srt + n))
		}
		_ = zw.Close()
		_, _ = w.Write(buf.Bytes())
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) check(r *http.Request) {
	if r.URL.Query().Get("token") != "tok" {
		f.t.Errorf("%s: token = %q, want = tok", r.URL.Path, r.URL.Query().Get("token"))
	}
}

func newProvider(t *testing.T, f *fakeAPI) (*Provider, *[]time.Duration) {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{"token":"tok"}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	ap := p.(*Provider)
	ap.SetBaseURL(u)
	var slept []time.Duration
	ap.sleep = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return ap, &slept
}

const results = `{"status":0,"sub":{"subs":[
	{"id":1,"videoname":"瑞克和莫蒂.Rick.and.Morty.S07E10.1080p.WEB-DL","lang":{"langlist":{"langchs":true,"langcht":true,"langeng":true}}},
	{"id":2,"videoname":"不知道","native_name":["Rick.and.Morty.S07.1080p.BluRay-STORiES"],"lang":{"langlist":{"langcht":true}}},
	{"id":3,"videoname":"Rick.and.Morty.S06.1080p","lang":{"langlist":{"langdou":true}}},
	{"id":4,"videoname":"No languages"}]}}`

func TestSearchEpisode(t *testing.T) {
	f := newFake(t)
	f.search = results
	p, slept := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Rick and Morty", Season: 7, Number: 10}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "zh"}, {Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Rick and Morty S07E10|1"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
	// Each language asked for is a subtitle: Chinese of both scripts is one.
	if got, want := ids(subs), []string{"1/zh", "1/en", "2/zh"}; !slices.Equal(got, want) {
		t.Fatalf("subtitles got = %v, want = %v", got, want)
	}
	if got, want := subs[0].ReleaseNames, []string{"瑞克和莫蒂.Rick.and.Morty.S07E10.1080p.WEB-DL", "Rick.and.Morty.S07E10.1080p.WEB-DL"}; !slices.Equal(got, want) {
		t.Errorf("release names got = %v, want = %v", got, want)
	}
	if len(subs[0].Matches) != 0 {
		t.Errorf("matches got = %v, want none", subs[0].Matches)
	}
	// A season's pack matches the episode.
	if s := subs[2]; !slices.Equal(s.Matches, []string{"episode"}) || s.ReleaseNames[0] != "Rick.and.Morty.S07.1080p.BluRay-STORiES" {
		t.Errorf("pack got = %+v", s)
	}
	// Requests are paced by the quota: 60s / 20.
	if want := []time.Duration{3 * time.Second}; !slices.Equal(*slept, want) {
		t.Errorf("waits got = %v, want = %v", *slept, want)
	}

	subs, err = p.Search(t.Context(), v, []provider.Language{{Tag: "zh-Hant"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ids(subs), []string{"1/zh-Hant", "2/zh-Hant"}; !slices.Equal(got, want) {
		t.Errorf("subtitles got = %v, want = %v", got, want)
	}
}

func TestSearchMovie(t *testing.T) {
	f := newFake(t)
	f.search = `{"sub":{"subs":[]}}`
	p, _ := newProvider(t, f)
	if _, err := p.Search(t.Context(), &provider.Video{Title: "Up", Year: 2009}, []provider.Language{{Tag: "zh"}}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"Up 2009|1"}; !slices.Equal(f.queries, want) {
		t.Errorf("queries got = %v, want = %v", f.queries, want)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p, _ := newProvider(t, f)
	v := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2}
	const sub = "1\n00:00:01,000 --> 00:00:02,000\n你好\n"
	for _, c := range []struct {
		name, detail, id, want string
	}{
		{"single file", `{"sub":{"subs":[{"url":"%s/dl/sub.srt","filelist":[]}]}}`, "9/zh", sub},
		{"episode and language", `{"sub":{"subs":[{"filelist":[
			{"f":"Show.S01E01.cht.srt","url":"%[1]s/file/1cht"},
			{"f":"Show.S01E02.eng.srt","url":"%[1]s/file/2eng"},
			{"f":"Show.S01E02.cht.srt","url":"%[1]s/file/2cht"}]}]}}`, "9/zh-Hant", sub + "/file/2cht"},
		{"first file", `{"sub":{"subs":[{"filelist":[
			{"f":"Show.S01E02.srt","url":"%[1]s/file/a"},{"f":"Show.S01E02.ass","url":"%[1]s/file/b"}]}]}}`, "9/en", sub + "/file/a"},
		{"archive", `{"sub":{"subs":[{"url":"%s/dl/pack.zip","filelist":[]}]}}`, "9/zh", sub + "Show.S01E02.chs.srt"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f.detail = fmt.Sprintf(c.detail, f.srv.URL)
			got, err := p.Download(t.Context(), v, c.id)
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Data) != c.want || got.Format != "srt" {
				t.Errorf("download got = %q, want = %q", got.Data, c.want)
			}
		})
	}
	f.detail = `{"sub":{"subs":[]}}`
	if _, err := p.Download(t.Context(), v, "9/zh"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("download without details got = %v, want = BadSubtitle", err)
	}
	if _, err := p.Download(t.Context(), v, "9"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("download of a malformed ID got = %v, want = BadSubtitle", err)
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct {
		name, quota, search string
		status              int
		kind                provider.Kind
	}{
		{"no quota", `{"user":{}}`, "", 0, provider.ProviderError},
		{"zero quota", `{"user":{"quota":0}}`, "", 0, provider.ProviderError},
		{"API error", "", `{"status":101,"errmsg":"bad token"}`, 0, provider.ProviderError},
		{"bad status", "", `oops`, 500, provider.ProviderError},
		{"not JSON", "", `oops`, 0, provider.ParseResponseError},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			if c.quota != "" {
				f.quota = c.quota
			}
			f.search, f.status = c.search, c.status
			p, _ := newProvider(t, f)
			_, err := p.Search(t.Context(), &provider.Video{Title: "Up"}, []provider.Language{{Tag: "zh"}})
			if got := provider.KindOf(err); got != c.kind {
				t.Errorf("kind got = %v, want = %v (%v)", got, c.kind, err)
			}
		})
	}
	if _, err := New(provider.Env{}, json.RawMessage(`{}`)); provider.KindOf(err) != provider.ConfigurationError {
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

package subtis

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

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fakeAPI is subt.is answering the searches of answers, 404 for others.
type fakeAPI struct {
	srv     *httptest.Server
	answers map[string]string
	mu      sync.Mutex
	paths   []string
}

func newFake(t *testing.T, answers map[string]string) *fakeAPI {
	f := &fakeAPI{answers: answers}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Bazarr/Subtis/"+version {
			t.Errorf("User-Agent got = %q, want = Bazarr/Subtis/%s", r.Header.Get("User-Agent"), version)
		}
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.EscapedPath())
		f.mu.Unlock()
		if r.URL.Path == "/files/sub.srt" {
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHola\n"))
			return
		}
		if r.URL.Path == "/files/empty.srt" {
			return
		}
		a, ok := f.answers[r.URL.EscapedPath()]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(strings.ReplaceAll(a, "%s", f.srv.URL)))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeAPI) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	const found = `{"subtitle":{"subtitle_link":"%s/files/sub.srt"},"title":{"title_name":" The Matrix "}}`
	movie := &provider.Video{Title: "The Matrix", Year: 1999, FileName: "/movies/The Matrix (1999).mkv", FileSize: 1234, FileHash: "8e245d9679d31e12"}
	cases := []struct {
		name      string
		v         *provider.Video
		answers   map[string]string
		langs     []provider.Language
		wantPaths []string
		want      []provider.Subtitle
	}{
		{
			name:      "hash",
			v:         movie,
			answers:   map[string]string{"/v1/subtitle/find/file/hash/8e245d9679d31e12": found},
			langs:     []provider.Language{{Tag: "es"}},
			wantPaths: []string{"/v1/subtitle/find/file/hash/8e245d9679d31e12"},
			want: []provider.Subtitle{{
				ID: "/files/sub.srt", Language: provider.Language{Tag: "es"}, ReleaseNames: []string{"The Matrix"},
				Matches: []string{"hash"}, PageLink: "/v1/subtitle/find/file/hash/8e245d9679d31e12", Format: "srt",
			}},
		},
		{
			name:    "alternative",
			v:       movie,
			answers: map[string]string{"/v1/subtitle/file/alternative/The%20Matrix%20%281999%29.mkv": found},
			langs:   []provider.Language{{Tag: "es"}},
			wantPaths: []string{
				"/v1/subtitle/find/file/hash/8e245d9679d31e12", "/v1/subtitle/find/file/bytes/1234",
				"/v1/subtitle/find/file/name/The%20Matrix%20%281999%29.mkv", "/v1/subtitle/file/alternative/The%20Matrix%20%281999%29.mkv",
			},
			want: []provider.Subtitle{{
				ID: "/files/sub.srt", Language: provider.Language{Tag: "es"}, ReleaseNames: []string{"The Matrix"},
				PageLink: "/v1/subtitle/file/alternative/The%20Matrix%20%281999%29.mkv", Format: "srt", Comment: "fuzzy match",
			}},
		},
		{
			name:      "no title and no hash",
			v:         &provider.Video{Title: "Up", FileName: "Up.mkv"},
			answers:   map[string]string{"/v1/subtitle/find/file/name/Up.mkv": `{"subtitle":{"subtitle_link":"%s/files/sub.srt"}}`},
			langs:     []provider.Language{{Tag: "es"}},
			wantPaths: []string{"/v1/subtitle/find/file/name/Up.mkv"},
			want: []provider.Subtitle{{
				ID: "/files/sub.srt", Language: provider.Language{Tag: "es"}, ReleaseNames: []string{"Unknown"},
				PageLink: "/v1/subtitle/find/file/name/Up.mkv", Format: "srt",
			}},
		},
		{
			name:  "not found",
			v:     &provider.Video{Title: "Up", FileName: "Up.mkv"},
			langs: []provider.Language{{Tag: "es"}},
			wantPaths: []string{
				"/v1/subtitle/find/file/name/Up.mkv", "/v1/subtitle/file/alternative/Up.mkv",
			},
		},
		{
			name:    "malformed answer",
			v:       &provider.Video{Title: "Up", FileName: "Up.mkv"},
			answers: map[string]string{"/v1/subtitle/find/file/name/Up.mkv": `{"subtitle":{"subtitle_link":""}}`, "/v1/subtitle/file/alternative/Up.mkv": `not json %s`},
			langs:   []provider.Language{{Tag: "es"}},
			wantPaths: []string{
				"/v1/subtitle/find/file/name/Up.mkv", "/v1/subtitle/file/alternative/Up.mkv",
			},
		},
		{name: "episode", v: &provider.Video{Episode: true, FileName: "a.mkv"}, langs: []provider.Language{{Tag: "es"}}},
		{name: "no name", v: &provider.Video{Title: "Up"}, langs: []provider.Language{{Tag: "es"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t, c.answers)
			p := newProvider(t, f)
			got, err := p.Search(t.Context(), c.v, c.langs)
			if err != nil {
				t.Fatal(err)
			}
			for i := range got {
				got[i].ID = got[i].ID[len(f.srv.URL):]
				u, _ := url.Parse(got[i].PageLink)
				got[i].PageLink = u.EscapedPath()
			}
			if !equal(got, c.want) {
				t.Errorf("Search got = %+v, want = %+v", got, c.want)
			}
			if !slices.Equal(f.paths, c.wantPaths) {
				t.Errorf("paths got = %q, want = %q", f.paths, c.wantPaths)
			}
		})
	}
}

func equal(a, b []provider.Subtitle) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestDownload(t *testing.T) {
	f := newFake(t, nil)
	p := newProvider(t, f)
	got, err := p.Download(t.Context(), nil, f.srv.URL+"/files/sub.srt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != "srt" || string(got.Data) != "1\n00:00:01,000 --> 00:00:02,000\nHola\n" || got.Language.Tag != "es" {
		t.Errorf("Download got = %+v, want = the Spanish srt", got)
	}
	for _, c := range []struct {
		id   string
		want provider.Kind
	}{
		{f.srv.URL + "/files/empty.srt", provider.BadSubtitle},
		{f.srv.URL + "/missing", provider.ProviderError},
		{"", provider.BadSubtitle},
	} {
		if _, err := p.Download(t.Context(), nil, c.id); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.id, err, c.want)
		}
	}
}

package napisy24

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func makeZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte(content))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// newServer answers CheckSub by the hash asked for.
func newServer(t *testing.T, user, pass string) *httptest.Server {
	t.Helper()
	answers := map[string][]byte{
		"found":    append([]byte("OK-2|imdb:133093|napisId:12345||"), makeZip(t, "The.Matrix.srt", "1\r\n00:00:01,000 --> 00:00:02,000\r\nCześć\r\n")...),
		"none":     []byte("OK-0"),
		"info":     []byte("OK-1|imdb:133093"),
		"other":    []byte("OK-3|imdb:133093"),
		"unknown":  []byte("what"),
		"notzip":   []byte("OK-2|imdb:1||nope"),
		"badlogin": []byte("login error"),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/run/CheckSubAgent.php" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		f := r.PostForm
		if f.Get("postAction") != "CheckSub" || f.Get("ua") != user || f.Get("ap") != pass || f.Get("fs") != "734003200" ||
			f.Get("fn") != "The.Matrix.1999.mkv" || f.Get("n24pref") != "1" {
			t.Errorf("form got = %v", f)
		}
		if f.Get("fh") == "throttled" {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(answers[f.Get("fh")])
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(t *testing.T, srv *httptest.Server, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func video(hash string) *provider.Video {
	return &provider.Video{
		Title: "The Matrix", ExternalIDs: map[string]string{"imdb": "tt0133093"},
		FileName: "/movies/The Matrix/The.Matrix.1999.mkv", FileSize: 734003200, FileHash: hash,
	}
}

func TestSearch(t *testing.T) {
	srv := newServer(t, defaultUsername, defaultPassword)
	p := newProvider(t, srv, `{"username":"me"}`)
	pl := []provider.Language{{Tag: "pl"}}
	for _, c := range []struct {
		hash     string
		langs    []provider.Language
		want     []string
		wantKind provider.Kind
	}{
		{hash: "found", langs: pl, want: []string{"hash", "imdb_id"}},
		{hash: "none", langs: pl},
		{hash: "info", langs: pl},
		{hash: "other", langs: pl},
		{hash: "unknown", langs: pl},
		{hash: "found", langs: []provider.Language{{Tag: "en"}}},
		{hash: "", langs: pl},
		{hash: "badlogin", langs: pl, wantKind: provider.AuthenticationError},
		{hash: "throttled", langs: pl, wantKind: provider.TooManyRequests},
	} {
		got, err := p.Search(t.Context(), video(c.hash), c.langs)
		if provider.KindOf(err) != c.wantKind || c.wantKind == provider.Other && err != nil {
			t.Errorf("Search(%s) got = %v, want = %v", c.hash, err, c.wantKind)
			continue
		}
		if c.want == nil {
			if len(got) != 0 {
				t.Errorf("Search(%s) got = %+v, want = none", c.hash, got)
			}
			continue
		}
		if len(got) != 1 || got[0].ID != c.hash || got[0].Language.Tag != "pl" || !slices.Equal(got[0].Matches, c.want) {
			t.Errorf("Search(%s) got = %+v, want = one with matches %v", c.hash, got, c.want)
		}
	}
}

func TestAccount(t *testing.T) {
	srv := newServer(t, "me", "pw")
	p := newProvider(t, srv, `{"username":"me","password":"pw"}`)
	if got, err := p.Search(t.Context(), video("found"), []provider.Language{{Tag: "pl"}}); err != nil || len(got) != 1 {
		t.Errorf("Search got = %v, %v, want = one subtitle", got, err)
	}
	if got := p.String(); got != "napisy24(me)" {
		t.Errorf("String got = %q, want = napisy24(me)", got)
	}
}

func TestDownload(t *testing.T) {
	srv := newServer(t, defaultUsername, defaultPassword)
	p := newProvider(t, srv, "")
	f, err := p.Download(t.Context(), video("other-hash"), "found")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nCześć\n"; string(f.Data) != want || f.Format != "srt" || f.Language.Tag != "pl" {
		t.Errorf("Download got = %q %q, want = %q srt", f.Data, f.Format, want)
	}
	for _, c := range []struct {
		v    *provider.Video
		id   string
		want provider.Kind
	}{
		{video("none"), "none", provider.BadSubtitle},
		{video("notzip"), "notzip", provider.BadSubtitle},
		{nil, "found", provider.BadSubtitle},
		{video("badlogin"), "badlogin", provider.AuthenticationError},
	} {
		if _, err := p.Download(t.Context(), c.v, c.id); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.id, err, c.want)
		}
	}
}

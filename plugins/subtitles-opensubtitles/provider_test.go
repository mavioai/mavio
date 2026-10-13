package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// fakeAPI is OpenSubtitles.com with an account whose first session
// expires after one request.
type fakeAPI struct {
	t       *testing.T
	srv     *httptest.Server
	logins  atomic.Int32
	queries []string
}

func newFake(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "me" || body["password"] != "pw" {
			http.Error(w, `{"message":"bad"}`, http.StatusUnauthorized)
			return
		}
		n := f.logins.Add(1)
		_, _ = fmt.Fprintf(w, `{"token":"t%d"}`, n)
	})
	mux.HandleFunc("GET /api/v1/subtitles", func(w http.ResponseWriter, r *http.Request) {
		f.check(r)
		f.queries = append(f.queries, r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"data":[
			{"id":"1","attributes":{"language":"zh-cn","download_count":10,"ratings":0,"release":"Other.Release","files":[{"file_id":11,"file_name":"a.ass"}]}},
			{"id":"2","attributes":{"language":"zh-cn","download_count":5,"moviehash_match":true,"release":"Up.2009","files":[{"file_id":22,"file_name":"b.srt"}]}},
			{"id":"3","attributes":{"language":"zh-cn","files":[]}}]}`))
	})
	mux.HandleFunc("POST /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer t1" {
			http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
			return
		}
		f.check(r)
		var body map[string]int64
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"link":"` + f.srv.URL + `/file/` + "22" + `","file_name":"b.srt"}`))
	})
	mux.HandleFunc("GET /file/22", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\n你好\n"))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) check(r *http.Request) {
	if r.Header.Get("Api-Key") != "key" || r.Header.Get("User-Agent") != userAgent || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer t") {
		f.t.Errorf("%s: headers %v", r.URL.Path, r.Header)
	}
}

func TestSearchAndDownload(t *testing.T) {
	f := newFake(t)
	p := &plugin{baseURL: f.srv.URL + "/api/v1", cfg: config{APIKey: "key", Username: "me", Password: "pw"}}
	resp, err := p.SearchSubtitles(t.Context(), pluginv1.SearchSubtitlesRequest_builder{Video: pluginv1.SubtitleVideo_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_EPISODE.Enum(), Title: proto.String("Pilot"), Series: proto.String("Show"),
		SeriesExternalIds: map[string]string{keyIMDb: "tt0012345"}, Season: proto.Int32(1), Episode: proto.Int32(2),
		FileHash: proto.String("0123456789abcdef"),
	}.Build(), Languages: []*pluginv1.SubtitleLanguage{lang("zh-Hans", false, false)}}.Build())
	if err != nil {
		t.Fatal(err)
	}
	want := "episode_number=2&foreign_parts_only=exclude&hearing_impaired=exclude&languages=zh-cn&moviehash=0123456789abcdef&parent_imdb_id=12345&season_number=1"
	if len(f.queries) != 1 || f.queries[0] != want {
		t.Errorf("query = %v, want = %s", f.queries, want)
	}
	subs := resp.GetSubtitles()
	// The server ranks subtitles; the plugin keeps the site's order.
	if len(subs) != 2 || !slices.Equal(subs[1].GetReleaseNames(), []string{"Up.2009"}) ||
		!slices.Equal(subs[1].GetMatches(), []string{"series_imdb_id", "hash"}) || subs[1].GetLanguage().GetLanguage() != "zh-Hans" ||
		subs[1].GetProvider() != providerName || subs[0].GetFormat() != "ass" {
		t.Fatalf("subtitles = %v", subs)
	}
	// The first session has expired by the download, which logs in again.
	got, err := p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Provider: proto.String(providerName), Id: proto.String(subs[1].GetId())}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetFormat() != "srt" || got.GetLanguage().GetLanguage() != "zh-Hans" || !strings.Contains(string(got.GetData()), "你好") {
		t.Errorf("download = %v", got)
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("logins = %d, want 2", n)
	}
	if _, err := p.DownloadSubtitle(t.Context(), pluginv1.DownloadSubtitleRequest_builder{Id: proto.String("x")}.Build()); err == nil {
		t.Error("download of a malformed ID: no error")
	}
}

func TestMovieQuery(t *testing.T) {
	f := newFake(t)
	p := &plugin{baseURL: f.srv.URL + "/api/v1", cfg: config{APIKey: "key", Username: "me", Password: "pw"}}
	if _, err := p.SearchSubtitles(t.Context(), pluginv1.SearchSubtitlesRequest_builder{Video: pluginv1.SubtitleVideo_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), Title: proto.String("Up"), Year: proto.Int32(2009),
	}.Build(), Languages: []*pluginv1.SubtitleLanguage{lang("fr", true, true)}}.Build()); err != nil {
		t.Fatal(err)
	}
	if want := "foreign_parts_only=only&languages=fr&query=Up&year=2009"; f.queries[0] != want {
		t.Errorf("query = %s, want = %s", f.queries[0], want)
	}
	unconfigured := &plugin{baseURL: f.srv.URL}
	if _, err := unconfigured.SearchSubtitles(t.Context(), pluginv1.SearchSubtitlesRequest_builder{Video: pluginv1.SubtitleVideo_builder{
		Title: proto.String("Up"),
	}.Build(), Languages: []*pluginv1.SubtitleLanguage{lang("fr", false, false)}}.Build()); err == nil {
		t.Error("search without an API key: no error")
	}
}

func lang(tag string, hearingImpaired, forced bool) *pluginv1.SubtitleLanguage {
	return subtitleLanguage(tag, hearingImpaired, forced)
}

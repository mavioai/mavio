package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// responses are canned MusicBrainz answers by path and query.
var responses = map[string]string{
	`/ws/2/release?query=release:"OK Computer" AND artist:"Radiohead"`: `{"releases":[
		{"id":"r1","score":100,"title":"OK Computer","date":"1997-05-21","country":"GB",
		 "artist-credit":[{"name":"Radiohead","artist":{"id":"a1","name":"Radiohead"}}],
		 "release-group":{"id":"g1","primary-type":"Album"}}]}`,
	`/ws/2/release/r1`: `{"id":"r1","title":"OK Computer","date":"1997-05-21",
		"artist-credit":[{"name":"Radiohead","artist":{"id":"a1","name":"Radiohead","sort-name":"Radiohead"}}],
		"release-group":{"id":"g1"},"label-info":[{"label":{"name":"Parlophone"}},{"label":{"name":"Parlophone"}}],
		"genres":[{"name":"rock","count":3}],"cover-art-archive":{"front":true}}`,
	`/ws/2/release-group/g1`: `{"id":"g1","title":"OK Computer","first-release-date":"1997-05",
		"genres":[{"name":"rock","count":5},{"name":"alternative rock","count":9}],"tags":[{"name":"90s","count":1}],
		"releases":[{"id":"r1","title":"OK Computer"},{"id":"r2","title":"OK Computer"}]}`,
	`/ws/2/artist/a1`: `{"id":"a1","name":"Radiohead","sort-name":"Radiohead","type":"Group",
		"life-span":{"begin":"1991","ended":false},"genres":[{"name":"rock","count":1}]}`,
	`/ws/2/artist?query=artist:"AC\/DC"`: `{"artists":[{"id":"a2","score":98,"name":"AC/DC","disambiguation":"Australian band",
		"life-span":{"begin":"1973-11-05"}}]}`,
	`/ws/2/recording?query=recording:"Airbag" AND artist:"Radiohead"`: `{"recordings":[{"id":"t1","score":90,"title":"Airbag"}]}`,
	`/ws/2/recording/t1`: `{"id":"t1","title":"Airbag","length":284000,"first-release-date":"1997-05-21",
		"artist-credit":[{"name":"Radiohead","artist":{"id":"a1","name":"Radiohead"}}],"releases":[{"id":"r1","title":"OK Computer"}]}`,
}

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent || r.URL.Query().Get("fmt") != "json" {
			t.Errorf("request %s: user agent %q", r.URL, r.Header.Get("User-Agent"))
		}
		key := r.URL.Path
		if q := r.URL.Query().Get("query"); q != "" {
			key += "?query=" + q
		}
		body, ok := responses[key]
		if !ok {
			http.Error(w, `{"error":"Not Found"}`, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	p := newPlugin(srv.URL, "https://covers.example")
	p.limiter.setInterval(0)
	return p
}

func lookup(kind pluginv1.MediaKind, name string, ids map[string]string, artists ...string) *pluginv1.Lookup {
	return pluginv1.Lookup_builder{Kind: kind.Enum(), Name: proto.String(name), ExternalIds: ids, Artists: artists}.Build()
}

func metadata(t *testing.T, p *plugin, l *pluginv1.Lookup) *pluginv1.Metadata {
	t.Helper()
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() {
		return nil
	}
	return resp.GetMetadata()
}

func TestAlbumMetadata(t *testing.T) {
	p := newTestPlugin(t)
	md := metadata(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM, "OK Computer", nil, "Radiohead"))
	if md == nil {
		t.Fatal("album not found")
	}
	if md.GetName() != "OK Computer" || md.GetPremiereDate() != "1997-05-01" || md.GetProductionYear() != 1997 {
		t.Errorf("album: %v", md)
	}
	if got, want := md.GetGenres(), []string{"alternative rock", "rock"}; !slices.Equal(got, want) {
		t.Errorf("genres = %v, want = %v", got, want)
	}
	if got := md.GetStudios(); !slices.Equal(got, []string{"Parlophone"}) {
		t.Errorf("studios = %v", got)
	}
	want := map[string]string{keyAlbum: "r1", keyReleaseGroup: "g1", keyAlbumArtist: "a1"}
	for k, v := range want {
		if md.GetExternalIds()[k] != v {
			t.Errorf("external IDs = %v, want = %v", md.GetExternalIds(), want)
		}
	}
	if len(md.GetImages()) != 1 || md.GetImages()[0].GetUrl() != "https://covers.example/release/r1/front" {
		t.Errorf("images = %v", md.GetImages())
	}
	if !slices.Equal(md.GetAlbumArtists(), []string{"Radiohead"}) {
		t.Errorf("album artists = %v", md.GetAlbumArtists())
	}
	// A release group's releases are its search results.
	resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{
		Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM, "", map[string]string{keyReleaseGroup: "g1"}),
	}.Build())
	if err != nil || len(resp.GetResults()) != 2 || resp.GetResults()[1].GetExternalIds()[keyAlbum] != "r2" ||
		resp.GetResults()[0].GetYear() != 1997 {
		t.Errorf("Search by release group = %v, %v", resp, err)
	}
}

func TestArtist(t *testing.T) {
	p := newTestPlugin(t)
	md := metadata(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST, "Radiohead", map[string]string{keyArtist: "a1"}))
	if md.GetName() != "Radiohead" || md.GetPremiereDate() != "1991-01-01" || md.GetExternalIds()[keyArtist] != "a1" || md.HasEndDate() {
		t.Errorf("artist: %v", md)
	}
	resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{
		Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST, "AC/DC", nil),
	}.Build())
	if err != nil || len(resp.GetResults()) != 1 {
		t.Fatalf("Search = %v, %v", resp, err)
	}
	if r := resp.GetResults()[0]; r.GetName() != "AC/DC (Australian band)" || r.GetScore() != 0.98 || r.GetYear() != 1973 {
		t.Errorf("result: %v", r)
	}
	// Unknown IDs are nothing found.
	if md := metadata(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST, "", map[string]string{keyArtist: "nobody"})); md != nil {
		t.Errorf("unknown artist: %v", md)
	}
}

func TestTrack(t *testing.T) {
	p := newTestPlugin(t)
	md := metadata(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_TRACK, "Airbag", nil, "Radiohead"))
	if md.GetName() != "Airbag" || md.GetRuntime().AsDuration() != 284*time.Second || !slices.Equal(md.GetArtists(), []string{"Radiohead"}) ||
		md.GetExternalIds()[keyTrack] != "t1" || md.GetExternalIds()[keyArtist] != "a1" {
		t.Errorf("track: %v", md)
	}
	// Other kinds are not MusicBrainz's.
	if md := metadata(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Airbag", nil)); md != nil {
		t.Errorf("movie: %v", md)
	}
}

func TestPhrase(t *testing.T) {
	if got, want := phrase(`AC/DC "live"`), `"AC\/DC \"live\""`; got != want {
		t.Errorf("phrase = %s, want = %s", got, want)
	}
}

func TestDate(t *testing.T) {
	for in, want := range map[string]string{"1997": "1997-01-01", "1997-05": "1997-05-01", "1997-05-21": "1997-05-21", "": "", "97": "", "1997-13": ""} {
		if got, _, ok := date(in); got != want || ok != (want != "") {
			t.Errorf("date(%q) = %q, %v, want = %q", in, got, ok, want)
		}
	}
}

func TestLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := &limiter{interval: time.Second}
		start := time.Now()
		for range 3 {
			if err := l.wait(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if got := time.Since(start); got != 2*time.Second {
			t.Errorf("three requests took %v, want = 2s", got)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := l.wait(ctx); err == nil {
			t.Error("wait with a cancelled context: no error")
		}
	})
}

func TestManifest(t *testing.T) {
	if shipped.GetId() != "org.mavio.scraper-musicbrainz" || !strings.Contains(shipped.GetConfigSchema(), "requests_per_second") {
		t.Errorf("manifest: %v", shipped)
	}
}

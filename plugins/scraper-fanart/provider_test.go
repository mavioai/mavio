package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var responses = map[string]string{
	"/movies/603": `{"name":"The Matrix","tmdb_id":"603",
		"movieposter":[{"id":"1","url":"https://a/en.jpg","lang":"en","likes":"9"},{"id":"2","url":"https://a/de.jpg","lang":"de","likes":"1"},
			{"id":"3","url":"https://a/none.jpg","lang":"00","likes":"0"},{"id":"4","url":"https://a/en2.jpg","lang":"en","likes":"20"}],
		"hdmovielogo":[{"id":"5","url":"https://a/hdlogo.png","lang":"en","likes":"1"}],
		"movielogo":[{"id":"6","url":"https://a/logo.png","lang":"en","likes":"5"}],
		"moviebackground":[{"id":"7","url":"https://a/bg.jpg","lang":"","likes":"2"}]}`,
	"/tv/81189": `{"seasonposter":[{"url":"https://a/s1.jpg","lang":"en","season":"1"},{"url":"https://a/s2.jpg","lang":"en","season":"2"}],
		"tvposter":[{"url":"https://a/show.jpg","lang":"en"}]}`,
	"/music/albums/g1": `{"name":"Radiohead","albums":{"g1":{"albumcover":[{"url":"https://a/cover.jpg","likes":"3"}],"cdart":[{"url":"https://a/cd.png","disc":"1"}]}}}`,
}

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "key" || r.URL.Query().Get("client_key") != "mine" {
			t.Errorf("request %s without the keys", r.URL)
		}
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	p := &plugin{baseURL: srv.URL}
	p.cfg = config{APIKey: "key", ClientKey: "mine"}
	return p
}

func urls(t *testing.T, p *plugin, l *pluginv1.Lookup) []string {
	t.Helper()
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, img := range resp.GetMetadata().GetImages() {
		out = append(out, img.GetKind().String()[len("IMAGE_KIND_"):]+" "+img.GetUrl())
	}
	return out
}

func TestImages(t *testing.T) {
	p := newTestPlugin(t)
	tests := []struct {
		name string
		l    *pluginv1.Lookup
		want []string
	}{
		{"movie in German", pluginv1.Lookup_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), ExternalIds: map[string]string{keyTMDB: "603"}, Language: proto.String("de-DE"),
		}.Build(), []string{
			"PRIMARY https://a/de.jpg", "PRIMARY https://a/none.jpg", "PRIMARY https://a/en2.jpg", "PRIMARY https://a/en.jpg",
			"BACKDROP https://a/bg.jpg", "LOGO https://a/hdlogo.png", "LOGO https://a/logo.png",
		}},
		{"season 2", pluginv1.Lookup_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_SEASON.Enum(), SeriesExternalIds: map[string]string{keyTVDB: "81189"}, SeasonNumber: proto.Int32(2),
		}.Build(), []string{"PRIMARY https://a/s2.jpg"}},
		{"album", pluginv1.Lookup_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM.Enum(), ExternalIds: map[string]string{keyMBGroup: "g1"},
		}.Build(), []string{"PRIMARY https://a/cover.jpg", "DISC https://a/cd.png"}},
		{"unknown movie", pluginv1.Lookup_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), ExternalIds: map[string]string{keyIMDb: "tt0"},
		}.Build(), nil},
		{"movie without IDs", pluginv1.Lookup_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), Name: proto.String("x")}.Build(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := urls(t, p, tt.l)
			if len(got) != len(tt.want) {
				t.Fatalf("got = %v, want = %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got = %v, want = %v", got, tt.want)
					break
				}
			}
		})
	}
}

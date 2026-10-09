package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// responses are canned answers by path and query.
var responses = map[string]string{
	"/key/artist-mb.php?i=mb-a1": `{"artists":[{"idArtist":"111","strArtist":"Radiohead","strMusicBrainzID":"mb-a1",
		"strGenre":"Alternative Rock","strStyle":"Rock/Pop","strMood":"Sad","intFormedYear":"1991","strDisbanded":null,
		"strBiographyEN":"English band.","strBiographyDE":"Englische Band.","strBiographyCN":null,
		"strArtistThumb":"https://img/thumb.jpg","strArtistLogo":"https://img/logo.png","strArtistFanart":"https://img/f1.jpg",
		"strArtistFanart2":"https://img/f2.jpg","strArtistFanart3":"","strArtistFanart4":null}]}`,
	"/key/search.php?s=Nobody":                       `{"artists":null}`,
	"/key/album-mb.php?i=mb-g1":                      `{"album":null}`,
	"/key/searchalbum.php?a=OK+Computer&s=Radiohead": `{"album":[{"idAlbum":"222","idArtist":"111","strAlbum":"OK Computer","strArtist":"Radiohead","intYearReleased":"1997","intScore":"8.7","strDescriptionEN":"Third album.","strAlbumThumb":"https://img/album.jpg","strAlbumCDart":"https://img/cd.png","strMusicBrainzID":"mb-g1","strMusicBrainzArtistID":"mb-a1"}]}`,
}

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path+"?"+r.URL.RawQuery]
		if !ok {
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	p := &plugin{baseURL: srv.URL}
	p.cfg.APIKey = "key"
	return p
}

func get(t *testing.T, p *plugin, l *pluginv1.Lookup) *pluginv1.Metadata {
	t.Helper()
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
	if err != nil {
		t.Fatal(err)
	}
	return resp.GetMetadata()
}

func TestArtist(t *testing.T) {
	p := newTestPlugin(t)
	md := get(t, p, pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST.Enum(), ExternalIds: map[string]string{keyMBArtist: "mb-a1"}, Language: proto.String("de"),
	}.Build())
	if md.GetName() != "Radiohead" || md.GetOverview() != "Englische Band." || md.GetProductionYear() != 1991 || md.HasEndDate() {
		t.Errorf("artist: %v", md)
	}
	if got, want := md.GetGenres(), []string{"Alternative Rock", "Rock", "Pop"}; !slices.Equal(got, want) {
		t.Errorf("genres = %v, want = %v", got, want)
	}
	var kinds []pluginv1.ImageKind
	for _, img := range md.GetImages() {
		kinds = append(kinds, img.GetKind())
	}
	want := []pluginv1.ImageKind{
		pluginv1.ImageKind_IMAGE_KIND_PRIMARY, pluginv1.ImageKind_IMAGE_KIND_LOGO,
		pluginv1.ImageKind_IMAGE_KIND_BACKDROP, pluginv1.ImageKind_IMAGE_KIND_BACKDROP,
	}
	if !slices.Equal(kinds, want) {
		t.Errorf("image kinds = %v, want = %v", kinds, want)
	}
	// A language without a biography falls back to English.
	md = get(t, p, pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST.Enum(), ExternalIds: map[string]string{keyMBArtist: "mb-a1"}, Language: proto.String("zh"),
	}.Build())
	if md.GetOverview() != "English band." || md.GetExternalIds()[keyArtist] != "111" {
		t.Errorf("artist in Chinese: %v", md)
	}
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ARTIST.Enum(), Name: proto.String("Nobody"),
	}.Build()}.Build())
	if err != nil || resp.GetFound() {
		t.Errorf("unknown artist = %v, %v", resp, err)
	}
}

func TestAlbum(t *testing.T) {
	p := newTestPlugin(t)
	// Unknown to TheAudioDB by its release group, it is searched for.
	md := get(t, p, pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM.Enum(), Name: proto.String("OK Computer"), Artists: []string{"Radiohead"},
		ExternalIds: map[string]string{keyMBGroup: "mb-g1"},
	}.Build())
	if md.GetName() != "OK Computer" || md.GetProductionYear() != 1997 || md.GetCommunityRating() != 8.7 || md.GetOverview() != "Third album." ||
		md.GetExternalIds()[keyAlbum] != "222" || md.GetExternalIds()[keyMBAlbumArtist] != "mb-a1" || len(md.GetImages()) != 2 {
		t.Errorf("album: %v", md)
	}
	resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{Lookup: pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM.Enum(), Name: proto.String("OK Computer"), Artists: []string{"Radiohead"},
	}.Build()}.Build())
	if err != nil || len(resp.GetResults()) != 1 || resp.GetResults()[0].GetName() != "Radiohead – OK Computer" {
		t.Errorf("Search = %v, %v", resp, err)
	}
}

func TestUnconfigured(t *testing.T) {
	p := &plugin{baseURL: "http://unused"}
	if _, err := p.Search(t.Context(), &pluginv1.SearchRequest{}); err == nil {
		t.Error("Search without an API key: no error")
	}
	if h, _ := p.Health(t.Context(), nil); h.GetHealthy() {
		t.Error("healthy without an API key")
	}
}

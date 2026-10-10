package providers

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// fakeClient knows one film by its TMDB ID and records the calls.
type fakeClient struct {
	searches, gets []*pluginv1.Lookup
	results        []*pluginv1.SearchResult
	metadata       *pluginv1.Metadata
}

func (c *fakeClient) Search(_ context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	c.searches = append(c.searches, proto.CloneOf(req.GetLookup()))
	return pluginv1.SearchResponse_builder{Results: c.results}.Build(), nil
}

func (c *fakeClient) GetMetadata(_ context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	l := req.GetLookup()
	c.gets = append(c.gets, proto.CloneOf(l))
	found := l.GetExternalIds()["tmdb"] == "10997" || l.GetSeriesExternalIds()["tmdb"] == "10997"
	resp := pluginv1.GetMetadataResponse_builder{Found: proto.Bool(found)}.Build()
	if found {
		resp.SetMetadata(c.metadata)
	}
	return resp, nil
}

func newFake() *fakeClient {
	return &fakeClient{
		results: []*pluginv1.SearchResult{pluginv1.SearchResult_builder{ExternalIds: map[string]string{"tmdb": "10997"}}.Build()},
		metadata: pluginv1.Metadata_builder{
			Name:            proto.String("Farewell My Concubine"),
			OriginalTitle:   proto.String("霸王别姬"),
			ProductionYear:  proto.Int32(1993),
			PremiereDate:    proto.String("1993-01-01"),
			Runtime:         durationpb.New(171 * time.Minute),
			CommunityRating: proto.Float64(11), // out of range
			CriticRating:    proto.Float64(91),
			Genres:          []string{"Drama"},
			ExternalIds:     map[string]string{"tmdb": "10997", "imdb": "tt0106332", "tvdb": ""},
			SeriesStatus:    pluginv1.SeriesStatus_SERIES_STATUS_ENDED.Enum(),
			People: []*pluginv1.PersonCredit{
				pluginv1.PersonCredit_builder{Name: proto.String("张国荣"), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(), Role: proto.String("Cheng Dieyi"), Order: proto.Int32(0)}.Build(),
				pluginv1.PersonCredit_builder{Name: proto.String("陈凯歌"), Kind: pluginv1.CreditKind_CREDIT_KIND_DIRECTOR.Enum()}.Build(),
				pluginv1.PersonCredit_builder{Name: proto.String("Someone"), Kind: pluginv1.CreditKind_CREDIT_KIND_UNSPECIFIED.Enum()}.Build(),
			},
			Images: []*pluginv1.RemoteImage{
				pluginv1.RemoteImage_builder{Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String("https://image.example/p.jpg")}.Build(),
				pluginv1.RemoteImage_builder{Kind: pluginv1.ImageKind_IMAGE_KIND_UNSPECIFIED.Enum(), Url: proto.String("https://image.example/x.jpg")}.Build(),
			},
		}.Build(),
	}
}

func TestPluginSearchesUnidentifiedItems(t *testing.T) {
	c := newFake()
	p := &Plugin{ID: "org.mavio.test", Client: c}
	res, err := p.Metadata(t.Context(), library.Lookup{Kind: core.KindMovie, Name: "Farewell My Concubine", Year: 1993, Language: "zh", Country: "CN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.searches) != 1 || c.searches[0].GetName() != "Farewell My Concubine" || c.searches[0].GetYear() != 1993 || c.searches[0].GetLanguage() != "zh" {
		t.Errorf("searches = %v, want = one for the film", c.searches)
	}
	if len(c.gets) != 1 || c.gets[0].GetExternalIds()["tmdb"] != "10997" {
		t.Errorf("gets = %v, want = one by the found ID", c.gets)
	}
	if res == nil {
		t.Fatal("Metadata() = nil, want = the film")
	}
	it := res.Item
	if it.Name != "Farewell My Concubine" || it.OriginalTitle != "霸王别姬" || it.ProductionYear != 1993 ||
		it.PremiereDate == nil || it.PremiereDate.Year() != 1993 || it.Runtime != 171*time.Minute ||
		it.CommunityRating != 0 || it.CriticRating != 91 || it.SeriesStatus != core.SeriesEnded {
		t.Errorf("item = %+v", it)
	}
	if want := map[core.Provider]string{core.ProviderTMDB: "10997", core.ProviderIMDb: "tt0106332"}; !maps.Equal(it.ExternalIDs, want) {
		t.Errorf("external IDs = %v, want = %v", it.ExternalIDs, want)
	}
	var people []string
	for _, p := range res.People {
		people = append(people, string(p.Kind)+":"+p.Name)
	}
	if want := []string{"actor:张国荣", "director:陈凯歌", "other:Someone"}; !slices.Equal(people, want) {
		t.Errorf("people = %v, want = %v", people, want)
	}
	if o := res.People[0].Order; o == nil || *o != 0 {
		t.Errorf("first order = %v, want = 0", o)
	}
	if res.People[1].Order != nil {
		t.Errorf("second order = %v, want = unknown", *res.People[1].Order)
	}
	if len(res.RemoteImages) != 1 || res.RemoteImages[0].Kind != core.ImagePrimary {
		t.Errorf("images = %+v, want = the primary image", res.RemoteImages)
	}
}

func TestPluginUsesKnownIDs(t *testing.T) {
	c := newFake()
	p := &Plugin{Client: c}
	res, err := p.Metadata(t.Context(), library.Lookup{Kind: core.KindMovie, Name: "x", ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "10997"}})
	if err != nil || res == nil {
		t.Fatalf("Metadata() = %v, %v, want = the film", res, err)
	}
	if len(c.searches) != 0 {
		t.Errorf("searches = %v, want = none", c.searches)
	}
}

func TestPluginEpisodes(t *testing.T) {
	c := newFake()
	p := &Plugin{Client: c}
	season, episode := 1, 2
	res, err := p.Metadata(t.Context(), library.Lookup{
		Kind: core.KindEpisode, Name: "Pilot", SeasonNumber: &season, EpisodeNumber: &episode,
		SeriesExternalIDs: map[core.Provider]string{core.ProviderTMDB: "10997"},
	})
	if err != nil || res == nil {
		t.Fatalf("Metadata() = %v, %v, want = metadata", res, err)
	}
	// Episodes are not searched for; their series identifies them.
	if len(c.searches) != 0 {
		t.Errorf("searches = %v, want = none", c.searches)
	}
	g := c.gets[0]
	if g.GetKind() != pluginv1.MediaKind_MEDIA_KIND_EPISODE || g.GetSeasonNumber() != 1 || g.GetEpisodeNumber() != 2 {
		t.Errorf("lookup = %v, want = S01E02", g)
	}
}

func TestPluginKnowsNothing(t *testing.T) {
	for _, tt := range []struct {
		name string
		l    library.Lookup
		c    *fakeClient
	}{
		{"unsupported kind", library.Lookup{Kind: core.KindFolder, Name: "x"}, newFake()},
		{"no name", library.Lookup{Kind: core.KindMovie}, newFake()},
		{"no search result", library.Lookup{Kind: core.KindMovie, Name: "x"}, &fakeClient{}},
		{"not found", library.Lookup{Kind: core.KindMovie, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "1"}}, newFake()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := (&Plugin{Client: tt.c}).Metadata(t.Context(), tt.l)
			if res != nil || err != nil {
				t.Errorf("Metadata() = %+v, %v, want = nil, nil", res, err)
			}
		})
	}
}

type fakeImages struct{ got *pluginv1.Lookup }

func (f *fakeImages) GetImages(_ context.Context, req *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	f.got = req.GetLookup()
	img := func(kind pluginv1.ImageKind, url string) *pluginv1.RemoteImage {
		return pluginv1.RemoteImage_builder{Kind: kind.Enum(), Url: proto.String(url), Width: proto.Int32(1000), Score: proto.Float64(0.5)}.Build()
	}
	return pluginv1.GetImagesResponse_builder{Images: []*pluginv1.RemoteImage{
		img(pluginv1.ImageKind_IMAGE_KIND_LOGO, "https://fanart.example/logo.png"),
		img(pluginv1.ImageKind_IMAGE_KIND_UNSPECIFIED, "https://fanart.example/what.png"),
		img(pluginv1.ImageKind_IMAGE_KIND_BANNER, ""),
	}}.Build(), nil
}

func TestImagePlugin(t *testing.T) {
	f := &fakeImages{}
	p := &ImagePlugin{ID: "fanart", Client: f}
	got, err := p.Images(t.Context(), library.Lookup{Kind: core.KindMovie, Name: "Heat", ExternalIDs: map[core.Provider]string{core.ProviderTMDB: "949"}})
	want := []metadata.RemoteImage{{Kind: core.ImageLogo, URL: "https://fanart.example/logo.png", Width: 1000, Score: 0.5}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Images() = %+v, %v, want %+v", got, err, want)
	}
	if f.got.GetExternalIds()["tmdb"] != "949" || f.got.GetKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE {
		t.Errorf("lookup = %v", f.got)
	}
	// Items of kinds the contract lacks are not asked about.
	f.got = nil
	if got, err := p.Images(t.Context(), library.Lookup{Kind: core.KindFolder}); got != nil || err != nil || f.got != nil {
		t.Errorf("Images(folder) = %v, %v; asked %v", got, err, f.got)
	}
}

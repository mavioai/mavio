// Command smokeplugin is the metadata provider used by the smoke tests. It
// builds for both runtimes: as plugin.wasm (GOOS=wasip1, -buildmode=c-shared)
// and as a native executable. It knows two films, "Farewell My Concubine"
// and "The Concubine", and searches carelessly: shorter names first,
// whatever the year. With "image_base" configured, it offers two posters
// and a backdrop of each film under that URL.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID and Version must match the smoke test manifest.
const (
	ID      = "org.mavio.smoke"
	Version = "0.1.0"
)

// TMDBID is the external ID of "Farewell My Concubine"; OtherTMDBID that
// of "The Concubine".
const (
	TMDBID      = "10997"
	OtherTMDBID = "117974"
)

type film struct {
	id, name, original, overview, collection, collectionID string
	year                                                   int32
	genres                                                 []string
}

var films = []film{
	{
		id: TMDBID, name: "Farewell My Concubine", original: "霸王别姬", year: 1993,
		overview: "Two boys meet at an opera training school in Peking in 1924.", genres: []string{"Drama", "Romance"},
		collection: "Chen Kaige Classics", collectionID: "4242",
	},
	{id: OtherTMDBID, name: "The Concubine", original: "후궁: 제왕의 첩", year: 2012, overview: "A palace intrigue.", genres: []string{"History"}},
}

// config is the plugin's configuration.
var config struct {
	sync.Mutex
	ImageBase string `json:"image_base"`
}

func init() {
	guest.Handle(pluginv1connect.NewPluginServiceHandler(lifecycle{}))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(provider{}))
}

type lifecycle struct{}

func (lifecycle) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(Version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

func (lifecycle) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	config.Lock()
	defer config.Unlock()
	config.ImageBase = ""
	_ = json.Unmarshal([]byte(req.GetConfigJson()), &config)
	return &pluginv1.ConfigureResponse{}, nil
}

func (lifecycle) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (lifecycle) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

type provider struct{}

func (provider) Search(_ context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	name := strings.ToLower(req.GetLookup().GetName())
	var found []film
	for _, f := range films {
		if name != "" && strings.Contains(strings.ToLower(f.name), name) {
			found = append(found, f)
		}
	}
	slices.SortStableFunc(found, func(a, b film) int { return cmp.Compare(len(a.name), len(b.name)) })
	resp := &pluginv1.SearchResponse{}
	for _, f := range found {
		resp.SetResults(append(resp.GetResults(), pluginv1.SearchResult_builder{
			Name:        proto.String(f.name),
			Year:        proto.Int32(f.year),
			ExternalIds: map[string]string{"tmdb": f.id},
			Score:       proto.Float64(1),
		}.Build()))
	}
	return resp, nil
}

func (provider) GetMetadata(_ context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	i := slices.IndexFunc(films, func(f film) bool { return f.id == req.GetLookup().GetExternalIds()["tmdb"] })
	if i < 0 {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	f := films[i]
	ids := map[string]string{"tmdb": f.id}
	if f.id == TMDBID {
		ids["imdb"] = "tt0106332"
	}
	if f.collectionID != "" {
		ids["tmdb_collection"] = f.collectionID
	}
	md := pluginv1.Metadata_builder{
		Name:           proto.String(f.name),
		OriginalTitle:  proto.String(f.original),
		Overview:       proto.String(f.overview),
		ProductionYear: proto.Int32(f.year),
		Genres:         f.genres,
		ExternalIds:    ids,
		CollectionName: proto.String(f.collection),
	}.Build()
	if f.id == TMDBID {
		md.SetPeople([]*pluginv1.PersonCredit{
			pluginv1.PersonCredit_builder{Name: proto.String("张国荣"), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(), Role: proto.String("Cheng Dieyi")}.Build(),
			pluginv1.PersonCredit_builder{Name: proto.String("陈凯歌"), Kind: pluginv1.CreditKind_CREDIT_KIND_DIRECTOR.Enum()}.Build(),
		})
	}
	config.Lock()
	base := config.ImageBase
	config.Unlock()
	if base != "" {
		image := func(kind pluginv1.ImageKind, name string, score float64) *pluginv1.RemoteImage {
			return pluginv1.RemoteImage_builder{
				Kind: kind.Enum(), Url: proto.String(base + "/" + f.id + "/" + name), Width: proto.Int32(1000),
				Height: proto.Int32(1500), Score: proto.Float64(score),
			}.Build()
		}
		md.SetImages([]*pluginv1.RemoteImage{
			image(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, "poster.jpg", 2),
			image(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, "poster-alt.jpg", 1),
			image(pluginv1.ImageKind_IMAGE_KIND_BACKDROP, "fanart.jpg", 1),
		})
	}
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

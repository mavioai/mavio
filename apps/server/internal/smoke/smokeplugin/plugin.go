// Command smokeplugin is the metadata provider used by the smoke tests. It
// builds for both runtimes: as plugin.wasm (GOOS=wasip1, -buildmode=c-shared)
// and as a native executable. It knows a single film, "Farewell My
// Concubine".
package main

import (
	"context"
	"strings"

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

// TMDBID is the external ID of the film the plugin knows.
const TMDBID = "10997"

func init() {
	guest.Handle(pluginv1connect.NewPluginServiceHandler(lifecycle{}))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(provider{}))
}

type lifecycle struct{}

func (lifecycle) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(Version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

func (lifecycle) Configure(context.Context, *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
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
	resp := &pluginv1.SearchResponse{}
	if strings.Contains(strings.ToLower(req.GetLookup().GetName()), "concubine") {
		resp.SetResults([]*pluginv1.SearchResult{pluginv1.SearchResult_builder{
			Name:        proto.String("Farewell My Concubine"),
			Year:        proto.Int32(1993),
			ExternalIds: map[string]string{"tmdb": TMDBID},
			Score:       proto.Float64(1),
		}.Build()})
	}
	return resp, nil
}

func (provider) GetMetadata(_ context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	if req.GetLookup().GetExternalIds()["tmdb"] != TMDBID {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	md := pluginv1.Metadata_builder{
		Name:           proto.String("Farewell My Concubine"),
		OriginalTitle:  proto.String("霸王别姬"),
		Overview:       proto.String("Two boys meet at an opera training school in Peking in 1924."),
		ProductionYear: proto.Int32(1993),
		Genres:         []string{"Drama", "Romance"},
		ExternalIds:    map[string]string{"tmdb": TMDBID, "imdb": "tt0106332"},
		People: []*pluginv1.PersonCredit{
			pluginv1.PersonCredit_builder{Name: proto.String("张国荣"), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(), Role: proto.String("Cheng Dieyi")}.Build(),
			pluginv1.PersonCredit_builder{Name: proto.String("陈凯歌"), Kind: pluginv1.CreditKind_CREDIT_KIND_DIRECTOR.Enum()}.Build(),
		},
	}.Build()
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

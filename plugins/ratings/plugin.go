// Command ratings is the ratings provider of the scores Seerr shows beside
// TMDB's: Rotten Tomatoes' critics' and audience scores of movies and
// series, found by name and year with the search Rotten Tomatoes' site
// uses, and IMDb's ratings of movies, by their IMDb IDs, from Radarr's
// metadata service. Neither needs a key. It builds as plugin.wasm
// (GOOS=wasip1, -buildmode=c-shared) and, for the process runtime, as a
// native executable.
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

//go:embed manifest.json
var manifestJSON []byte

// shipped is the manifest shipped next to the plugin, which Describe
// returns as is.
var shipped = func() *pluginv1.Manifest {
	m := &pluginv1.Manifest{}
	if err := protojson.Unmarshal(manifestJSON, m); err != nil {
		panic(fmt.Sprintf("manifest.json: %v", err))
	}
	return m
}()

// The services' base URLs.
const (
	algoliaURL        = "https://79frdp12pn-dsn.algolia.net/1/indexes/*"
	radarrMetadataURL = "https://api.radarr.video/v1"
)

func init() {
	p := &plugin{http: guest.HTTPClient(), algolia: algoliaURL, radarr: radarrMetadataURL}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewRatingsProviderServiceHandler(p))
}

// plugin implements the lifecycle and ratings provider services.
type plugin struct {
	http            *http.Client
	algolia, radarr string
}

func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return pluginv1.DescribeResponse_builder{Manifest: shipped}.Build(), nil
}

func (p *plugin) Configure(context.Context, *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	return &pluginv1.ConfigureResponse{}, nil
}

func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// GetRatings asks Rotten Tomatoes and, for a movie with an IMDb ID,
// Radarr's metadata service at once, and tells what they know; it fails
// only when every service asked fails.
func (p *plugin) GetRatings(ctx context.Context, req *pluginv1.GetRatingsRequest) (*pluginv1.GetRatingsResponse, error) {
	series := req.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES
	var asks []func() ([]*pluginv1.Rating, error)
	if name := req.GetName(); name != "" {
		asks = append(asks, func() ([]*pluginv1.Rating, error) { return p.rottenTomatoes(ctx, name, int(req.GetYear()), series) })
	}
	if id := req.GetExternalIds()["imdb"]; id != "" && !series {
		asks = append(asks, func() ([]*pluginv1.Rating, error) { return p.imdb(ctx, id) })
	}
	type result struct {
		ratings []*pluginv1.Rating
		err     error
	}
	results := make([]result, len(asks))
	var wg sync.WaitGroup
	for i, ask := range asks {
		wg.Go(func() { results[i].ratings, results[i].err = ask() })
	}
	wg.Wait()
	var (
		out  []*pluginv1.Rating
		errs []error
	)
	for _, r := range results {
		out = append(out, r.ratings...)
		if r.err != nil {
			errs = append(errs, r.err)
		}
	}
	if len(asks) > 0 && len(errs) == len(asks) {
		return nil, errors.Join(errs...)
	}
	return pluginv1.GetRatingsResponse_builder{Ratings: out}.Build(), nil
}

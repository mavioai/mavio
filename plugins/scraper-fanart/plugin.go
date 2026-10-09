// Command scraper-fanart is the artwork provider for fanart.tv: posters,
// backdrops, logos, clear art, banners, thumbs and disc art of movies
// (found by their TMDB or IMDb IDs), series and seasons (by TVDB IDs), and
// music artists and albums (by MusicBrainz IDs). It provides images only,
// which other providers' IDs lead to. It builds as plugin.wasm
// (GOOS=wasip1, -buildmode=c-shared) and, for the process runtime, as a
// native executable. Requests need a project API key, which
// administrators configure, and may add their personal key.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
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

// defaultBaseURL is fanart.tv's API v3.
const defaultBaseURL = "https://webservice.fanart.tv/v3"

func init() {
	p := &plugin{baseURL: defaultBaseURL}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(p))
}

// config is the plugin's configuration, as described by the manifest's
// schema.
type config struct {
	APIKey    string `json:"api_key"`
	ClientKey string `json:"client_key"`
}

var errUnconfigured = connect.NewError(connect.CodeFailedPrecondition, errors.New("no fanart.tv API key is configured"))

// plugin implements the lifecycle and metadata provider services.
type plugin struct {
	baseURL string

	mu  sync.RWMutex
	cfg config
}

func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return pluginv1.DescribeResponse_builder{Manifest: shipped}.Build(), nil
}

func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var cfg config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("configuration: %w", err))
	}
	p.mu.Lock()
	p.cfg = cfg
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	resp := pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build()
	if _, err := p.client(); err != nil {
		resp.SetHealthy(false)
		resp.SetMessage(errUnconfigured.Message())
	}
	return resp, nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (p *plugin) client() (*client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cfg.APIKey == "" {
		return nil, errUnconfigured
	}
	return &client{http: guest.HTTPClient(), baseURL: p.baseURL, key: p.cfg.APIKey, clientKey: p.cfg.ClientKey}, nil
}

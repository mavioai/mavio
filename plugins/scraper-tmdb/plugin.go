// Command scraper-tmdb is the metadata provider for The Movie Database
// (TMDB): movies, series, seasons and episodes, with their cast, crew and
// images; it also lists the episodes of series in their display orders.
// It builds as plugin.wasm (GOOS=wasip1, -buildmode=c-shared) and, for the
// process runtime, as a native executable. Requests use an
// administrator's TMDB API key when one is configured, else Radarr's API
// read access token, and Mavio's own key once TMDB rejects that token.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

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

func init() {
	p := &plugin{baseURL: defaultBaseURL, radarrToken: radarrToken, mavioKey: mavioKey}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(p))
	guest.Handle(pluginv1connect.NewEpisodeListProviderServiceHandler(p))
}

// config is the plugin's configuration, as described by the manifest's
// schema.
type config struct {
	APIKey                 string `json:"api_key"`
	MaxCastMembers         *int   `json:"max_cast_members"`
	HideMissingCastMembers bool   `json:"hide_missing_cast_members"`
	IncludeAdult           bool   `json:"include_adult"`
}

// defaultMaxCast is how many actors are credited unless configured.
const defaultMaxCast = 15

// radarrToken is Radarr's TMDB API read access token, from its
// RadarrCloudRequestBuilder, used unless an administrator configures a
// key.
const radarrToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJhdWQiOiIxYTczNzMzMDE5NjFkMDNmOTdmODUzYTg3NmRkMTIxMiIsInN1YiI6IjU4NjRmNTkyYzNhMzY4MGFiNjAxNzUzNCIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.gh1BwogCCKOda6xj9FRMgAAj_RYKMMPC3oNlcBtlmwk"

// mavioKey is Mavio's own TMDB API key, used once TMDB rejects Radarr's
// token; release builds set it with -ldflags "-X main.mavioKey=…".
var mavioKey string

var errUnconfigured = connect.NewError(connect.CodeFailedPrecondition,
	errors.New("TMDB rejected Radarr's token and this build has no key of its own; configure a TMDB API key"))

// plugin implements the lifecycle, metadata provider and episode list
// provider services.
type plugin struct {
	baseURL string
	// radarrToken and mavioKey are the keys used without a configured
	// one; rejected records that TMDB rejected radarrToken.
	radarrToken, mavioKey string
	rejected              atomic.Bool

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
	if _, err := p.session(); err != nil {
		resp.SetHealthy(false)
		resp.SetMessage(errUnconfigured.Message())
	}
	return resp, nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (p *plugin) config() config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg
}

// session is one request's view of the configuration.
type session struct {
	*client
	cast  castConfig
	adult bool
}

func (p *plugin) session() (session, error) {
	cfg := p.config()
	c := &client{http: guest.HTTPClient(), baseURL: p.baseURL, key: cfg.APIKey}
	if c.key == "" {
		c.key, c.fallback, c.rejected = p.radarrToken, p.mavioKey, &p.rejected
		if p.rejected.Load() {
			c.key, c.fallback = p.mavioKey, ""
		}
		if c.key == "" {
			return session{}, errUnconfigured
		}
	}
	cast := castConfig{maxCast: defaultMaxCast, hideMissing: cfg.HideMissingCastMembers}
	if cfg.MaxCastMembers != nil {
		cast.maxCast = *cfg.MaxCastMembers
	}
	return session{
		client: c,
		cast:   cast,
		adult:  cfg.IncludeAdult,
	}, nil
}

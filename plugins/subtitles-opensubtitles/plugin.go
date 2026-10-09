// Command subtitles-opensubtitles is the subtitle provider for
// OpenSubtitles.com: it searches subtitles of movies and episodes by the
// video's file hash, IDs and name, and downloads them. It builds as
// plugin.wasm (GOOS=wasip1, -buildmode=c-shared) and, for the process
// runtime, as a native executable. Requests need an API consumer key,
// which administrators configure; an account raises the download quota.
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

// defaultBaseURL is the OpenSubtitles.com REST API.
const defaultBaseURL = "https://api.opensubtitles.com/api/v1"

func init() {
	p := &plugin{baseURL: defaultBaseURL}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewSubtitleProviderServiceHandler(p))
}

// config is the plugin's configuration, as described by the manifest's
// schema.
type config struct {
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	Password string `json:"password"`
}

var errUnconfigured = connect.NewError(connect.CodeFailedPrecondition, errors.New("no OpenSubtitles API key is configured"))

// plugin implements the lifecycle and subtitle provider services.
type plugin struct {
	baseURL string

	mu  sync.Mutex
	cfg config
	// token is the account's session, once logged in; apiURL the server
	// the login assigned.
	token, apiURL string
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
	p.cfg, p.token, p.apiURL = cfg, "", ""
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	resp := pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfg.APIKey == "" {
		resp.SetHealthy(false)
		resp.SetMessage(errUnconfigured.Message())
	}
	return resp, nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// client returns a client, logged in when an account is configured.
func (p *plugin) client(ctx context.Context) (*client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cfg.APIKey == "" {
		return nil, errUnconfigured
	}
	c := &client{http: guest.HTTPClient(), baseURL: p.baseURL, key: p.cfg.APIKey}
	if p.cfg.Username == "" {
		return c, nil
	}
	if p.token == "" {
		token, host, err := c.login(ctx, p.cfg.Username, p.cfg.Password)
		if err != nil {
			return nil, err
		}
		p.token = token
		if host != "" {
			p.apiURL = "https://" + host + "/api/v1"
		}
	}
	c.token = p.token
	if p.apiURL != "" {
		c.baseURL = p.apiURL
	}
	return c, nil
}

// loggedOut forgets an expired session.
func (p *plugin) loggedOut() {
	p.mu.Lock()
	p.token, p.apiURL = "", ""
	p.mu.Unlock()
}

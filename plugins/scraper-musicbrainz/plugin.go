// Command scraper-musicbrainz is the metadata provider for MusicBrainz:
// music artists, albums (releases and their release groups) and tracks
// (recordings), with album covers from the Cover Art Archive. It builds as
// plugin.wasm (GOOS=wasip1, -buildmode=c-shared) and, for the process
// runtime, as a native executable. It needs no configuration; a mirror
// may be configured, and requests are spaced as MusicBrainz asks.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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

// Defaults of the configuration.
const (
	defaultBaseURL     = "https://musicbrainz.org"
	defaultCoverArtURL = "https://coverartarchive.org"
	defaultRate        = 1.0
)

func init() {
	p := newPlugin(defaultBaseURL, defaultCoverArtURL)
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(p))
}

// config is the plugin's configuration, as described by the manifest's
// schema.
type config struct {
	BaseURL           string  `json:"base_url"`
	RequestsPerSecond float64 `json:"requests_per_second"`
}

// plugin implements the lifecycle and metadata provider services.
type plugin struct {
	coverArtURL string
	limiter     *limiter

	mu      sync.RWMutex
	baseURL string
}

func newPlugin(baseURL, coverArtURL string) *plugin {
	return &plugin{baseURL: baseURL, coverArtURL: coverArtURL, limiter: &limiter{interval: time.Second}}
}

func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return pluginv1.DescribeResponse_builder{Manifest: shipped}.Build(), nil
}

func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var cfg config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("configuration: %w", err))
	}
	rate := cfg.RequestsPerSecond
	if rate <= 0 {
		rate = defaultRate
	}
	p.limiter.setInterval(time.Duration(float64(time.Second) / rate))
	p.mu.Lock()
	p.baseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	if p.baseURL == "" {
		p.baseURL = defaultBaseURL
	}
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (p *plugin) client() *client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return &client{http: guest.HTTPClient(), baseURL: p.baseURL, coverArtURL: p.coverArtURL, limiter: p.limiter}
}

// limiter spaces requests by an interval.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (l *limiter) setInterval(d time.Duration) {
	l.mu.Lock()
	l.interval = d
	l.mu.Unlock()
}

// wait blocks until the next request may be sent.
func (l *limiter) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	at := l.next
	if at.Before(now) {
		at = now
	}
	l.next = at.Add(l.interval)
	l.mu.Unlock()
	if d := time.Until(at); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	return nil
}

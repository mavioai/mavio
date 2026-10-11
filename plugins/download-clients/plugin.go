package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// config is the plugin's configuration.
type config struct {
	Clients []clientConfig `json:"clients"`
}

// clientConfig is a configured download client.
type clientConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	// Enabled, RemoveCompletedDownloads and RemoveFailedDownloads are
	// true when unset, as Radarr's defaults.
	Enabled                  *bool `json:"enabled"`
	RemoveCompletedDownloads *bool `json:"remove_completed_downloads"`
	RemoveFailedDownloads    *bool `json:"remove_failed_downloads"`
	// Priority orders the clients of a protocol, lower first; 1 when 0.
	Priority int `json:"priority"`
	// Settings are the definition's settings.
	Settings json.RawMessage `json:"settings"`
}

func orTrue(b *bool) bool { return b == nil || *b }

func (c *clientConfig) priority() int {
	if c.Priority == 0 {
		return 1
	}
	return c.Priority
}

func (c *clientConfig) options() client.Options {
	return client.Options{
		RemoveCompletedDownloads: orTrue(c.RemoveCompletedDownloads),
		RemoveFailedDownloads:    orTrue(c.RemoveFailedDownloads),
	}
}

// configured is a client of the configuration.
type configured struct {
	cfg    clientConfig
	def    client.Definition
	client client.Client
}

func (c *configured) name() string {
	if c.cfg.Name != "" {
		return c.cfg.Name
	}
	return c.def.Name
}

// plugin implements the plugin's services.
type plugin struct {
	log      *slog.Logger
	version  string
	http     *http.Client
	statuses *client.Statuses
	defs     []client.Definition
	byID     map[string]client.Definition
	// now is the clock; time.Now when nil.
	now func() time.Time

	mu      sync.RWMutex
	clients []*configured
}

// newPlugin returns the plugin keeping the clients' statuses in dataDir
// and reaching them with hc.
func newPlugin(log *slog.Logger, version, dataDir string, hc *http.Client) (*plugin, error) {
	file := ""
	if dataDir != "" {
		file = filepath.Join(dataDir, "status.json")
	}
	statuses, err := client.NewStatuses(file, time.Now)
	if err != nil {
		return nil, err
	}
	p := &plugin{log: log, version: version, http: hc, statuses: statuses, byID: map[string]client.Definition{}}
	p.defs = definitions()
	for _, d := range p.defs {
		p.byID[d.ID] = d
	}
	return p, nil
}

func (p *plugin) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *plugin) env() client.Env {
	return client.Env{HTTP: p.http, Now: p.clock, Log: p.log}
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Configure takes a configuration, whose clients replace those
// configured.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := c.validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	clients := p.build(c)
	keep := map[string]bool{}
	for _, cc := range c.Clients {
		keep[cc.ID] = true
	}
	p.statuses.Remove(func(id string) bool { return keep[id] })
	p.mu.Lock()
	p.clients = clients
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

// validate checks the configuration's clients.
func (c *config) validate() error {
	seen := map[string]bool{}
	for i, cc := range c.Clients {
		switch {
		case strings.TrimSpace(cc.ID) == "":
			return fmt.Errorf("clients[%d]: id is required", i)
		case seen[cc.ID]:
			return fmt.Errorf("clients[%d]: id %q is repeated", i, cc.ID)
		case strings.TrimSpace(cc.Definition) == "":
			return fmt.Errorf("client %s: definition is required", cc.ID)
		case cc.Priority < 0 || cc.Priority > 50:
			return fmt.Errorf("client %s: priority must be from 1 to 50", cc.ID)
		}
		seen[cc.ID] = true
	}
	return nil
}

// build makes the configured clients. Those whose definition is unknown
// or whose settings are invalid are logged and left out.
func (p *plugin) build(c config) []*configured {
	var out []*configured
	for _, cc := range c.Clients {
		cl, def, err := p.newClient(cc.Definition, cc.Settings)
		if err != nil {
			p.log.Error("configure download client", "client", cc.ID, "err", err)
			continue
		}
		out = append(out, &configured{cfg: cc, def: def, client: cl})
	}
	return out
}

// newClient makes a client of a definition.
func (p *plugin) newClient(definition string, settings []byte) (client.Client, client.Definition, error) {
	def, ok := p.byID[definition]
	if !ok {
		return nil, def, fmt.Errorf("unknown definition %q", definition)
	}
	if len(settings) == 0 {
		settings = []byte("{}")
	}
	cl, err := def.New(settings, p.env())
	if err != nil {
		return nil, def, fmt.Errorf("settings of %s: %w", def.Name, err)
	}
	return cl, def, nil
}

// Health implements PluginService.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

// Shutdown implements PluginService.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// client returns a configured client by ID.
func (p *plugin) client(id string) (*configured, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, c := range p.clients {
		if c.cfg.ID == id {
			return c, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown download client "+id))
}

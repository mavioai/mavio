package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/throttle"
)

// retryWait is how long providers wait before retrying a request, as
// Bazarr's ProviderRetryMixin.
const retryWait = 10 * time.Second

// config is the plugin's configuration.
type config struct {
	Providers []providerConfig `json:"providers"`
}

// providerConfig is a provider's configuration.
type providerConfig struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	// Settings are the provider's settings.
	Settings json.RawMessage `json:"settings"`
}

// configured is an enabled provider.
type configured struct {
	def      *provider.Definition
	settings string
	provider provider.Provider
	// err is why the provider could not be made from its settings.
	err error
}

// plugin implements the plugin's services.
type plugin struct {
	log      *slog.Logger
	version  string
	http     *http.Client
	dataDir  string
	throttle *throttle.Throttle
	defs     []provider.Definition
	byName   map[string]*provider.Definition
	// now is the clock; time.Now when nil.
	now func() time.Time
	// retryWait is how long providers wait before retrying.
	retryWait time.Duration

	mu      sync.RWMutex
	enabled map[string]*configured
}

// newPlugin returns the plugin keeping the throttled providers and the
// providers' data in dataDir, and reaching the sites with hc.
func newPlugin(log *slog.Logger, version, dataDir string, hc *http.Client) (*plugin, error) {
	p := &plugin{
		log: log, version: version, http: hc, dataDir: dataDir, defs: definitions(),
		byName: map[string]*provider.Definition{}, enabled: map[string]*configured{}, retryWait: retryWait,
	}
	file := ""
	if dataDir != "" {
		file = filepath.Join(dataDir, "throttled.json")
	}
	t, err := throttle.New(file, p.clock, log)
	if err != nil {
		return nil, err
	}
	p.throttle = t
	for i := range p.defs {
		p.byName[p.defs[i].Name] = &p.defs[i]
	}
	return p, nil
}

func (p *plugin) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// env is what a provider is made with.
func (p *plugin) env(name string) provider.Env {
	env := provider.Env{HTTP: p.http, Now: p.clock, Log: p.log.With("provider", name), RetryWait: p.retryWait}
	if p.dataDir != "" {
		env.DataDir = filepath.Join(p.dataDir, "providers", name)
	}
	return env
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Configure takes a configuration, whose providers replace those
// enabled. As Bazarr's, changing a provider's settings forgets its
// throttling for authentication and configuration failures, and the
// throttling of providers no longer enabled is forgotten.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	enabled, seen := map[string]*configured{}, map[string]bool{}
	for i, pc := range c.Providers {
		def, ok := p.byName[pc.Name]
		switch {
		case !ok:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("providers[%d]: unknown provider %q", i, pc.Name))
		case seen[pc.Name]:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("providers[%d]: provider %q is repeated", i, pc.Name))
		}
		seen[pc.Name] = true
		if !pc.Enabled {
			continue
		}
		cp := &configured{def: def, settings: string(pc.Settings)}
		cp.provider, cp.err = def.New(p.env(def.Name), pc.Settings)
		if cp.err != nil {
			p.log.Warn("configure subtitle provider", "provider", def.Name, "err", cp.err)
		}
		enabled[pc.Name] = cp
	}
	p.mu.Lock()
	old := p.enabled
	p.enabled = enabled
	p.mu.Unlock()
	for name, cp := range enabled {
		if prev := old[name]; prev == nil || prev.settings != cp.settings {
			p.resetAuth(name)
		}
	}
	p.throttle.Retain(func(name string) bool { return enabled[name] != nil })
	return &pluginv1.ConfigureResponse{}, nil
}

// resetAuth forgets a provider's throttling for failures its settings
// may have caused.
func (p *plugin) resetAuth(name string) {
	p.throttle.Reset(func(n, reason string) bool {
		return n == name && (reason == provider.AuthenticationError.String() || reason == provider.ConfigurationError.String())
	})
}

// Health implements PluginService.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

// Shutdown implements PluginService.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// settingFields describes a provider's settings.
func settingFields(def *provider.Definition) []*pluginv1.SettingField {
	var out []*pluginv1.SettingField
	for _, s := range def.Settings {
		f := pluginv1.SettingField_builder{
			Name: proto.String(s.Name), Label: proto.String(s.Label), HelpText: proto.String(s.Help),
			Advanced: proto.Bool(s.Advanced), Secret: proto.Bool(s.Type == provider.Password),
		}.Build()
		switch s.Type {
		case provider.Text, provider.Password:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT)
		case provider.Number:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER)
		case provider.Checkbox:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_CHECKBOX)
		case provider.Select:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT)
		}
		if s.Default != nil {
			if data, err := json.Marshal(s.Default); err == nil {
				f.SetDefaultJson(string(data))
			}
		}
		for _, o := range s.Options {
			data, err := json.Marshal(o)
			if err != nil {
				continue
			}
			f.SetOptions(append(f.GetOptions(), pluginv1.SettingFieldOption_builder{
				ValueJson: proto.String(string(data)), Label: proto.String(o),
			}.Build()))
		}
		out = append(out, f)
	}
	return out
}

// errNotEnabled is a provider asked for that is not enabled.
var errNotEnabled = errors.New("provider not enabled")

// lookup returns an enabled provider.
func (p *plugin) lookup(name string) (*configured, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cp := p.enabled[name]
	if cp == nil {
		return nil, fmt.Errorf("%s: %w", name, errNotEnabled)
	}
	return cp, nil
}

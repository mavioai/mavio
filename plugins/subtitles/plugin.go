package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/captcha"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/throttle"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
	"github.com/mavioai/mavio/plugins/subtitles/internal/web"
)

// retryWait is how long providers wait before retrying a request, as
// Bazarr's ProviderRetryMixin.
const retryWait = 10 * time.Second

// config is the plugin's configuration.
type config struct {
	Providers []providerConfig `json:"providers"`
	// Translators are the subtitle translators and their settings.
	Translators []providerConfig `json:"translators"`
	// Proxy is the proxy the sites are reached through; nil for none.
	Proxy *web.Proxy `json:"proxy,omitempty"`
	// FlareSolverr solves Cloudflare's challenges; nil for none.
	FlareSolverr *web.FlareSolverr `json:"flaresolverr,omitempty"`
	// Captcha is the captcha solving service's account; nil for none.
	Captcha *captcha.Config `json:"captcha,omitempty"`
}

// validate checks the settings other than the providers'.
func (c *config) validate() error {
	if c.Proxy != nil {
		if err := c.Proxy.Validate(); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	if c.FlareSolverr != nil {
		if u, err := url.Parse(c.FlareSolverr.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("flaresolverr: invalid URL %q", c.FlareSolverr.URL)
		}
	}
	return nil
}

// providerConfig is a provider's or translator's configuration.
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

// configuredTranslator is an enabled translator.
type configuredTranslator struct {
	def        *translator.Definition
	translator translator.Translator
	// err is why the translator could not be made from its settings.
	err error
}

// plugin implements the plugin's services.
type plugin struct {
	log     *slog.Logger
	version string
	// client returns the client the sites are reached with.
	client func(web.Config) *http.Client
	// host returns the client reaching the server's host API, which the
	// runtime sets once serving; nil for none.
	host     func() *http.Client
	dataDir  string
	throttle *throttle.Throttle
	defs     []provider.Definition
	byName   map[string]*provider.Definition
	// translatorDefs are the translators, by name in translatorByName.
	translatorDefs   []translator.Definition
	translatorByName map[string]*translator.Definition
	// now is the clock; time.Now when nil.
	now func() time.Time
	// retryWait is how long providers wait before retrying.
	retryWait time.Duration

	mu          sync.RWMutex
	enabled     map[string]*configured
	translators map[string]*configuredTranslator
}

// clientTimeout is how long requests to the sites take at most, as
// Bazarr's providers'.
const clientTimeout = 30 * time.Second

// newPlugin returns the plugin keeping the throttled providers and the
// providers' data in dataDir, reaching the sites with the clients client
// returns, web.NewClient's when nil, and the host API with the client host
// returns.
func newPlugin(log *slog.Logger, version, dataDir string, client func(web.Config) *http.Client, host func() *http.Client) (*plugin, error) {
	if client == nil {
		client = func(c web.Config) *http.Client { return web.NewClient(c, clientTimeout) }
	}
	p := &plugin{
		log: log, version: version, client: client, host: host, dataDir: dataDir, defs: definitions(),
		byName: map[string]*provider.Definition{}, enabled: map[string]*configured{}, retryWait: retryWait,
		translatorDefs: translatorDefinitions(), translatorByName: map[string]*translator.Definition{},
		translators: map[string]*configuredTranslator{},
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
	for i := range p.translatorDefs {
		p.translatorByName[p.translatorDefs[i].Name] = &p.translatorDefs[i]
	}
	return p, nil
}

func (p *plugin) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *plugin) hostClient() *http.Client {
	if p.host == nil {
		return nil
	}
	return p.host()
}

// env is what a provider is made with.
func (p *plugin) env(name string, hc *http.Client, solver provider.CaptchaSolver) provider.Env {
	env := provider.Env{HTTP: hc, Now: p.clock, Log: p.log.With("provider", name), Captcha: solver, RetryWait: p.retryWait, Host: p.hostClient()}
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

// Configure takes a configuration, whose providers and translators
// replace those enabled, reaching the sites through its proxy and
// FlareSolverr and solving captchas with its service; translators are
// reached through the proxy only. As Bazarr's, changing a provider's
// settings forgets its throttling for authentication and configuration
// failures, and the throttling of providers no longer enabled is
// forgotten.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := c.validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	hc := p.client(web.Config{Proxy: c.Proxy, FlareSolverr: c.FlareSolverr})
	var solver provider.CaptchaSolver
	if c.Captcha != nil {
		// The services are reached through the proxy, as Bazarr's
		// pitchers, but never through FlareSolverr.
		var err error
		if solver, err = captcha.New(*c.Captcha, p.client(web.Config{Proxy: c.Proxy}), p.log.With("captcha", c.Captcha.Service)); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("captcha: %w", err))
		}
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
		cp.provider, cp.err = def.New(p.env(def.Name, hc, solver), pc.Settings)
		if cp.err != nil {
			p.log.Warn("configure subtitle provider", "provider", def.Name, "err", cp.err)
		}
		enabled[pc.Name] = cp
	}
	translators, err := p.configureTranslators(c)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	old := p.enabled
	p.enabled, p.translators = enabled, translators
	p.mu.Unlock()
	for name, cp := range enabled {
		if prev := old[name]; prev == nil || prev.settings != cp.settings {
			p.resetAuth(name)
		}
	}
	p.throttle.Retain(func(name string) bool { return enabled[name] != nil })
	return &pluginv1.ConfigureResponse{}, nil
}

// configureTranslators makes the enabled translators of a configuration.
// Translations take minutes, bounded by the translators themselves
// rather than the sites' timeout.
func (p *plugin) configureTranslators(c config) (map[string]*configuredTranslator, error) {
	hc := *p.client(web.Config{Proxy: c.Proxy})
	hc.Timeout = 0
	out, seen := map[string]*configuredTranslator{}, map[string]bool{}
	for i, tc := range c.Translators {
		def, ok := p.translatorByName[tc.Name]
		switch {
		case !ok:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translators[%d]: unknown translator %q", i, tc.Name))
		case seen[tc.Name]:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("translators[%d]: translator %q is repeated", i, tc.Name))
		}
		seen[tc.Name] = true
		if !tc.Enabled {
			continue
		}
		ct := &configuredTranslator{def: def}
		env := provider.Env{HTTP: &hc, Now: p.clock, Log: p.log.With("translator", def.Name), RetryWait: p.retryWait, Host: p.hostClient()}
		ct.translator, ct.err = def.New(env, tc.Settings)
		if ct.err != nil {
			p.log.Warn("configure subtitle translator", "translator", def.Name, "err", ct.err)
		}
		out[tc.Name] = ct
	}
	return out, nil
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

// settingFields describes a provider's or translator's settings.
func settingFields(settings []provider.Setting) []*pluginv1.SettingField {
	var out []*pluginv1.SettingField
	for _, s := range settings {
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

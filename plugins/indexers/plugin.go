package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/challenge"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// config is the plugin's configuration.
type config struct {
	Indexers    []indexerConfig   `json:"indexers"`
	Definitions definitionsConfig `json:"definitions"`
}

// definitionsConfig is how the Cardigann definitions are updated.
type definitionsConfig struct {
	// Update fetches the definitions of indexers.prowlarr.com at start
	// and daily; true when unset.
	Update *bool `json:"update"`
	// URL replaces that of indexers.prowlarr.com, for tests and mirrors.
	URL string `json:"url"`
}

func (c definitionsConfig) update() bool { return c.Update == nil || *c.Update }

// indexerConfig is a configured indexer.
type indexerConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	// Enabled is true when unset.
	Enabled *bool `json:"enabled"`
	// Priority orders indexers, lower first; 25 when 0.
	Priority int `json:"priority"`
	// Tags limit the indexer to items sharing one of them, as the
	// IndexerTag specifications of Radarr and Sonarr do.
	Tags []string `json:"tags"`
	// Proxy is the ID of the server's proxy the indexer is reached
	// through: the system proxy when unset, none when empty.
	Proxy *string `json:"proxy"`
	// QueryLimit and GrabLimit are the most queries and grabs per
	// LimitsUnit, "day" (the default) or "hour"; the definition's when 0.
	QueryLimit int    `json:"query_limit"`
	GrabLimit  int    `json:"grab_limit"`
	LimitsUnit string `json:"limits_unit"`
	// Settings are the definition's settings.
	Settings json.RawMessage `json:"settings"`
	// RequiredFlags are the flags a torrent release must carry one of, as
	// Radarr's and Sonarr's RequiredFlags indexer setting.
	RequiredFlags []string `json:"required_flags"`
	// MinimumSeeders is the fewest seeders a torrent release may have; 1
	// when unset, as Radarr's and Sonarr's IndexerDefaults.MINIMUM_SEEDERS.
	MinimumSeeders *int `json:"minimum_seeders"`
	// MultiLanguages are the languages of the indexer's MULTi releases, by
	// English name, as Radarr's and Sonarr's MultiLanguages setting.
	MultiLanguages []string `json:"multi_languages"`
	// SeedRatio, SeedTime and SeasonPackSeedTime are how long torrents
	// seed, the times in minutes, as Sonarr's SeedCriteriaSettings; 0
	// for the download client's defaults.
	SeedRatio          float64 `json:"seed_ratio"`
	SeedTime           int     `json:"seed_time"`
	SeasonPackSeedTime int     `json:"season_pack_seed_time"`
	// RejectBlocklistedTorrentHashes refuses, when grabbing a torrent
	// whose hash the release did not tell, a torrent whose hash is
	// blocklisted, as Radarr's and Sonarr's
	// RejectBlocklistedTorrentHashesWhileGrabbing.
	RejectBlocklistedTorrentHashes bool `json:"reject_blocklisted_torrent_hashes"`
	// AnimeStandardFormatSearch searches anime by season and episode too,
	// as Sonarr's AnimeStandardFormatSearch indexer setting.
	AnimeStandardFormatSearch bool `json:"anime_standard_format_search"`
	// DownloadClientID is the download client the indexer's releases are
	// always sent to, "<plugin>/<client>" or a client's ID alone, as
	// Radarr's and Sonarr's IndexerDefinition.DownloadClientId; empty for
	// none.
	DownloadClientID string `json:"download_client_id"`
}

// minimumSeeders returns the fewest seeders a torrent release may have.
func (c *indexerConfig) minimumSeeders() int {
	if c.MinimumSeeders == nil {
		return 1
	}
	return *c.MinimumSeeders
}

// knownFlag reports whether indexers' required flags may name f.
func knownFlag(f string) bool {
	switch indexer.Flag(f) {
	case indexer.FlagInternal, indexer.FlagExclusive, indexer.FlagFreeleech, indexer.FlagNeutralLeech,
		indexer.FlagHalfLeech, indexer.FlagScene, indexer.FlagDoubleUpload:
		return true
	}
	return false
}

// proxyID returns the ID of the server's proxy the indexer is reached
// through, "" for none.
func (c *indexerConfig) proxyID() string {
	if c.Proxy == nil {
		return proxy.SystemID
	}
	return *c.Proxy
}

func (c *indexerConfig) enabled() bool { return c.Enabled == nil || *c.Enabled }

func (c *indexerConfig) priority() int {
	if c.Priority == 0 {
		return 25
	}
	return c.Priority
}

// configured is an indexer of the configuration.
type configured struct {
	cfg indexerConfig
	def indexer.Definition
	ix  *indexer.Indexer
}

// hostService is the server's host API the plugin calls: it lists the
// server's proxies and solves challenges.
type hostService interface {
	proxy.Lister
	challenge.Solver
}

// plugin implements the plugin's services.
type plugin struct {
	// host returns the server's host API, nil before it is reached.
	host     func() hostService
	log      *slog.Logger
	version  string
	client   *indexer.Client
	statuses *indexer.Statuses
	store    *definitionStore
	// now is the clock; time.Now when nil.
	now func() time.Time

	mu       sync.RWMutex
	cfg      config
	defs     []indexer.Definition
	byID     map[string]indexer.Definition
	indexers []*configured
	// proxies are the server's proxies the configuration names, by ID;
	// nil without the host API, reaching indexers directly.
	proxies map[string]*pluginv1.Proxy
	// solver solves challenges; nil leaves them unsolved.
	solver challenge.Solver
	// stop ends the definitions' updates.
	stop context.CancelFunc
	done chan struct{}
}

// newPlugin returns the plugin keeping its state in dataDir: the
// indexers' statuses and the Cardigann definitions. host returns the
// server's host API; it may be nil.
func newPlugin(log *slog.Logger, version, dataDir, hostOS string, host func() hostService) (*plugin, error) {
	file := ""
	if dataDir != "" {
		file = filepath.Join(dataDir, "status.json")
	}
	statuses, err := indexer.NewStatuses(file, time.Now)
	if err != nil {
		return nil, err
	}
	statuses.SetStarted(time.Now())
	p := &plugin{
		host: host, log: log, version: version,
		client:   indexer.NewClient(),
		statuses: statuses,
		store: &definitionStore{
			dir:       filepath.Join(dataDir, "Definitions"),
			url:       definitionsURL,
			userAgent: strings.TrimSpace(fmt.Sprintf("Prowlarr/%s (%s)", prowlarrVersion, hostOS)),
			client:    newHTTPClient(),
			log:       log,
		},
	}
	p.loadDefinitions()
	return p, nil
}

// loadDefinitions reads the definitions anew, those of the store's
// directory after those in code, and rebuilds the configured indexers.
func (p *plugin) loadDefinitions() {
	cards, err := p.store.Load()
	if err != nil {
		p.log.Error("load indexer definitions", "err", err)
	}
	defs, byID := definitionsByID(codeDefinitions(), cards, p.log)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.defs, p.byID = defs, byID
	p.indexers = p.build(p.cfg)
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Configure takes a configuration: its indexers replace those configured,
// and the definitions are updated at once and then daily unless updates
// are turned off. The server configures the plugin again when its
// proxies change.
func (p *plugin) Configure(ctx context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := c.validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var ids []string
	for _, ic := range c.Indexers {
		ids = append(ids, ic.proxyID())
	}
	h := p.hostService()
	var proxies map[string]*pluginv1.Proxy
	if h != nil {
		var err error
		if proxies, err = proxy.List(ctx, h, ids...); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
	}
	p.halt()
	p.mu.Lock()
	p.cfg, p.proxies = c, proxies
	if h != nil {
		p.solver = h
	}
	p.client.Forget()
	p.store.url = definitionsURL
	if c.Definitions.URL != "" {
		p.store.url = c.Definitions.URL
	}
	p.indexers = p.build(c)
	keep := map[string]bool{}
	for _, ic := range c.Indexers {
		keep[ic.ID] = true
	}
	p.statuses.Remove(func(id string) bool { return keep[id] })
	if c.Definitions.update() {
		ctx, cancel := context.WithCancel(context.Background())
		p.stop, p.done = cancel, make(chan struct{})
		go p.updateDefinitions(ctx, p.done)
	}
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

// hostService returns the server's host API, nil when unreached.
func (p *plugin) hostService() hostService {
	if p.host == nil {
		return nil
	}
	return p.host()
}

// validate checks the configuration's indexers.
func (c *config) validate() error {
	seen, names := map[string]bool{}, map[string]bool{}
	for i, ic := range c.Indexers {
		switch {
		case strings.TrimSpace(ic.ID) == "":
			return fmt.Errorf("indexers[%d]: id is required", i)
		case seen[ic.ID]:
			return fmt.Errorf("indexers[%d]: id %q is repeated", i, ic.ID)
		case strings.TrimSpace(ic.Name) == "":
			return fmt.Errorf("indexer %s: name is required", ic.ID)
		case names[strings.ToLower(ic.Name)]:
			return fmt.Errorf("indexer %s: name %q should be unique", ic.ID, ic.Name)
		case strings.TrimSpace(ic.Definition) == "":
			return fmt.Errorf("indexer %s: definition is required", ic.ID)
		case ic.LimitsUnit != "" && ic.LimitsUnit != "day" && ic.LimitsUnit != "hour":
			return fmt.Errorf("indexer %s: limits_unit must be day or hour", ic.ID)
		case ic.MinimumSeeders != nil && *ic.MinimumSeeders < 0:
			return fmt.Errorf("indexer %s: minimum_seeders must not be negative", ic.ID)
		case ic.SeedRatio < 0 || ic.SeedTime < 0 || ic.SeasonPackSeedTime < 0:
			return fmt.Errorf("indexer %s: seed_ratio, seed_time and season_pack_seed_time must not be negative", ic.ID)
		}
		for _, f := range ic.RequiredFlags {
			if !knownFlag(f) {
				return fmt.Errorf("indexer %s: unknown required flag %q", ic.ID, f)
			}
		}
		seen[ic.ID], names[strings.ToLower(ic.Name)] = true, true
	}
	return nil
}

// build makes the configured indexers. Those whose definition is unknown
// or whose settings are invalid are logged and left out until a
// configuration or the definitions change. p.mu is held.
func (p *plugin) build(c config) []*configured {
	var out []*configured
	for _, ic := range c.Indexers {
		ix, def, err := p.newIndexer(ic, p.proxies)
		if err != nil {
			p.log.Error("configure indexer", "indexer", ic.ID, "err", err)
			continue
		}
		out = append(out, &configured{cfg: ic, def: def, ix: ix})
	}
	return out
}

// newIndexer makes an indexer of a definition, reached through its proxy
// among proxies, directly when proxies is nil. p.mu is held.
func (p *plugin) newIndexer(ic indexerConfig, proxies map[string]*pluginv1.Proxy) (*indexer.Indexer, indexer.Definition, error) {
	def, ok := p.byID[ic.Definition]
	if !ok {
		return nil, def, fmt.Errorf("unknown definition %q", ic.Definition)
	}
	site, err := def.New(ic.Settings)
	if err != nil {
		return nil, def, err
	}
	var px *pluginv1.Proxy
	if id := ic.proxyID(); id != "" && proxies != nil {
		if px = proxies[id]; px == nil {
			return nil, def, fmt.Errorf("proxy %s: %w", id, proxy.ErrNotFound)
		}
	}
	limits := def.Limits
	if ic.QueryLimit > 0 {
		limits.Queries = ic.QueryLimit
	}
	if ic.GrabLimit > 0 {
		limits.Grabs = ic.GrabLimit
	}
	switch ic.LimitsUnit {
	case "hour":
		limits.Per = time.Hour
	case "day":
		limits.Per = 24 * time.Hour
	}
	return &indexer.Indexer{
		ID: ic.ID, Name: ic.Name, Priority: ic.priority(),
		Site: site, Session: p.client.Session(px, p.solver), Statuses: p.statuses,
		Limits: limits, Now: p.now,
	}, def, nil
}

// updateDefinitions updates the definitions now and then daily, reloading
// them after each update.
func (p *plugin) updateDefinitions(ctx context.Context, done chan struct{}) {
	defer close(done)
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if err := p.store.Update(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			p.log.ErrorContext(ctx, "definition update failed", "err", err)
		} else {
			p.loadDefinitions()
		}
		t.Reset(definitionsInterval)
	}
}

// Health implements PluginService.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

// Shutdown stops the definitions' updates.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	p.halt()
	return &pluginv1.ShutdownResponse{}, nil
}

// halt stops the definitions' updates and waits for them.
func (p *plugin) halt() {
	p.mu.Lock()
	stop, done := p.stop, p.done
	p.stop, p.done = nil, nil
	p.mu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

// indexer returns a configured indexer by ID.
func (p *plugin) indexer(id string) (*configured, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, c := range p.indexers {
		if c.cfg.ID == id {
			return c, nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown indexer "+id))
}

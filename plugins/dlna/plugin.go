package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/plugins/dlna/internal/profile"
	"github.com/mavioai/mavio/plugins/dlna/internal/ssdp"
	"github.com/mavioai/mavio/plugins/dlna/internal/upnp"
)

// config is the plugin's configuration.
type config struct {
	// User is the user whose libraries the media server serves.
	User string `json:"user"`
	// ServerName is the name devices show; empty uses the server's.
	ServerName  string `json:"server_name"`
	MediaServer *bool  `json:"media_server"`
	PlayTo      *bool  `json:"play_to"`
	// Interfaces names the network interfaces to use; empty uses every
	// interface that is up, takes multicast and has an IPv4 address.
	Interfaces []string `json:"interfaces"`
	// AnnounceMinutes is how often the media server announces itself.
	AnnounceMinutes int `json:"announce_interval_minutes"`
	// SSDPPort replaces SSDP's port 1900, for tests.
	SSDPPort int `json:"ssdp_port"`
	// Renderers lists the description URLs of renderers to control
	// besides those found, such as renderers on other subnets.
	Renderers []string          `json:"renderers"`
	Profiles  []profile.Profile `json:"profiles"`
}

func (c *config) mediaServer() bool { return c.MediaServer == nil || *c.MediaServer }
func (c *config) playTo() bool      { return c.PlayTo == nil || *c.PlayTo }

func (c *config) interval() time.Duration {
	if c.AnnounceMinutes <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(c.AnnounceMinutes) * time.Minute
}

// serverInfo is how the server is reached.
type serverInfo struct {
	name, version string
	httpPort      int
	// base is the server's base URL path, "" or "/path".
	base string
}

// plugin is the DLNA plugin's state.
type plugin struct {
	ctx     context.Context
	log     *slog.Logger
	udn     string
	version string
	// events keeps the subscriptions to the media server's services.
	events *upnp.Publisher
	// updateID is ContentDirectory's SystemUpdateID.
	updateID atomic.Uint32
	// changed wakes the announcer of content changes.
	changed chan struct{}
	media   *mediaCache

	mu        sync.Mutex
	cfg       config
	profiles  *profile.Set
	info      serverInfo
	ready     bool
	stop      context.CancelFunc
	done      chan struct{}
	renderers *renderers
}

// newPlugin returns the plugin, which runs until ctx ends, with its UDN
// kept in the data folder.
func newPlugin(ctx context.Context, log *slog.Logger, version string) (*plugin, error) {
	udn, err := loadUDN(guest.DataDir())
	if err != nil {
		return nil, err
	}
	set, err := profile.NewSet(nil)
	if err != nil {
		return nil, err
	}
	p := &plugin{
		ctx: ctx, log: log, udn: udn, version: version,
		events:   &upnp.Publisher{Logger: log},
		changed:  make(chan struct{}, 1),
		profiles: set,
	}
	p.media = newMediaCache(p)
	p.updateID.Store(1)
	go p.announceChanges(ctx)
	return p, nil
}

// loadUDN returns the media server's UDN, made once and kept in dir.
func loadUDN(dir string) (string, error) {
	file := filepath.Join(dir, "udn")
	if dir != "" {
		if b, err := os.ReadFile(file); err == nil && strings.HasPrefix(string(b), "uuid:") {
			return strings.TrimSpace(string(b)), nil
		}
	}
	var u [16]byte
	_, _ = rand.Read(u[:])
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	udn := fmt.Sprintf("uuid:%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
	if dir == "" {
		return udn, nil
	}
	if err := os.WriteFile(file, []byte(udn+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("keep the UDN: %w", err)
	}
	return udn, nil
}

// state returns the configuration, profiles and server info in use.
func (p *plugin) state() (config, *profile.Set, serverInfo, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg, p.profiles, p.info, p.ready
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Configure takes a configuration and restarts the media server and Play
// To with it.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if strings.TrimSpace(c.User) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name the user whose libraries are served"))
	}
	set, err := profile.NewSet(c.Profiles)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if _, err := ssdp.Interfaces(c.Interfaces); err != nil && len(c.Interfaces) > 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	p.restart(c, set)
	return &pluginv1.ConfigureResponse{}, nil
}

// Health implements PluginService.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

// Shutdown stops the media server and Play To, saying goodbye.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	p.halt()
	return &pluginv1.ShutdownResponse{}, nil
}

// halt stops what runs and waits for it.
func (p *plugin) halt() {
	p.mu.Lock()
	stop, done := p.stop, p.done
	p.stop, p.done, p.ready = nil, nil, false
	p.mu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
}

// restart stops what runs and starts anew with a configuration.
func (p *plugin) restart(c config, set *profile.Set) {
	p.halt()
	ctx, cancel := context.WithCancel(p.ctx)
	done := make(chan struct{})
	r := newRenderers(p, c, set)
	p.mu.Lock()
	p.cfg, p.profiles, p.stop, p.done, p.renderers = c, set, cancel, done, r
	p.mu.Unlock()
	go func() {
		defer close(done)
		p.run(ctx, c, r)
	}()
}

// run serves with a configuration until ctx ends.
func (p *plugin) run(ctx context.Context, c config, r *renderers) {
	info, err := p.serverInfo(ctx)
	if err != nil {
		return
	}
	p.mu.Lock()
	p.info, p.ready = info, true
	p.mu.Unlock()
	ifaces, err := ssdp.Interfaces(c.Interfaces)
	if err != nil {
		p.log.WarnContext(ctx, "network interfaces", "err", err)
	}
	if len(ifaces) == 0 {
		p.log.WarnContext(ctx, "no network interface to announce or search on; set interfaces")
	}
	srv := &ssdp.Server{
		Product:    "Mavio/" + p.version + " UPnP/1.0 DLNADOC/1.50",
		Interfaces: ifaces, Interval: c.interval(), Port: c.SSDPPort, Logger: p.log,
		Location: func(local net.IP) string { return p.root(local.String(), info) + "/description.xml" },
	}
	switch {
	case c.mediaServer() && info.httpPort == 0:
		p.log.WarnContext(ctx, "the server has no HTTP port for DLNA devices; the media server is off")
	case c.mediaServer():
		srv.Adverts = ssdp.Adverts(p.udn, deviceType, contentDirectory, connectionManager, receiverRegistrar)
	}
	if c.playTo() {
		srv.OnNotify = r.notified
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	if len(srv.Adverts) > 0 || c.playTo() {
		wg.Go(func() {
			if err := srv.Run(ctx); err != nil && ctx.Err() == nil {
				p.log.ErrorContext(ctx, "SSDP", "err", err)
			}
		})
	}
	if c.playTo() {
		wg.Go(func() { r.run(ctx, ifaces) })
	}
	<-ctx.Done()
}

// serverInfo asks the host how the server is reached, retrying until it
// answers or ctx ends.
func (p *plugin) serverInfo(ctx context.Context) (serverInfo, error) {
	host := pluginv1connect.NewHostServiceClient(guest.HostClient(), guest.HostURL)
	for wait := time.Second; ; wait = min(2*wait, time.Minute) {
		resp, err := host.GetServerInfo(ctx, &pluginv1.GetServerInfoRequest{})
		if err == nil {
			base := strings.TrimRight(resp.GetBaseUrl(), "/")
			if base != "" && !strings.HasPrefix(base, "/") {
				base = "/" + base
			}
			return serverInfo{name: resp.GetServerName(), version: resp.GetVersion(), httpPort: int(resp.GetHttpPort()), base: base}, nil
		}
		p.log.WarnContext(ctx, "server info", "err", err)
		select {
		case <-ctx.Done():
			return serverInfo{}, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// root returns the URL of the plugin's routes at a host of this server:
// an address, with the HTTP port added, or a host and port.
func (p *plugin) root(host string, info serverInfo) string {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, strconv.Itoa(info.httpPort))
	}
	return "http://" + host + info.base + "/plugins/" + ID
}

// friendlyName is the name devices show.
func (p *plugin) friendlyName(c config, info serverInfo) string {
	switch {
	case c.ServerName != "":
		return c.ServerName
	case info.name != "":
		return info.name
	}
	return "Mavio"
}

// contentChanged notes a change of the libraries' content.
func (p *plugin) contentChanged() {
	p.updateID.Add(1)
	select {
	case p.changed <- struct{}{}:
	default:
	}
}

// announceChanges sends SystemUpdateID to subscribers when content
// changes, at most every two seconds as ContentDirectory moderates it.
func (p *plugin) announceChanges(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.changed:
		}
		p.events.Notify(ctx, "cds", []upnp.Arg{{Name: "SystemUpdateID", Value: strconv.FormatUint(uint64(p.updateID.Load()), 10)}})
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// Consume notes library changes, which change the content directory.
func (p *plugin) Consume(_ context.Context, req *pluginv1.ConsumeRequest) (*pluginv1.ConsumeResponse, error) {
	for _, e := range req.GetEvents() {
		if strings.HasPrefix(e.GetType(), "item.") {
			p.contentChanged()
			break
		}
	}
	return &pluginv1.ConsumeResponse{}, nil
}

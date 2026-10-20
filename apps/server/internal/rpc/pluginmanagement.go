package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"

	"connectrpc.com/connect"
	"golang.org/x/text/language"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/dependencies"
	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	"github.com/mavioai/mavio/libs/plugin/proxy"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// PluginManagementService implements
// mavio.system.v1.PluginManagementService for administrators.
type PluginManagementService struct {
	// Plugins runs the server's plugins; nil means none.
	Plugins PluginManager
	// Hub tells administrators of plugin changes; nil tells no one.
	Hub *events.Hub
	// Activity records administrators' changes; nil records none.
	Activity *activity.Log
	// DownloadClients returns the download client providers, whose
	// clients are the options of manifest.OptionsDownloadClients; nil has
	// none.
	DownloadClients func() []downloads.Provider
	// Containers keeps plugins' container dependencies; nil means none.
	Containers ContainerDependencies
	// Proxies keeps the server's proxies, the options of
	// manifest.OptionsProxies; nil keeps none.
	Proxies *proxies.Service
	// Settings keep the proxies plugins reach sites through; nil keeps
	// none.
	Settings *settings.Manager
	// ProxyChanged is told of a plugin whose proxy changed; nil tells
	// nobody.
	ProxyChanged func(ctx context.Context, pluginID string)
}

// ContainerDependencies keeps plugins' container dependencies;
// *dependencies.Service implements it.
type ContainerDependencies interface {
	Runtime(ctx context.Context, detect bool) (containers.Runtime, error)
	List(ctx context.Context, pluginID string) ([]dependencies.Dependency, error)
	Set(ctx context.Context, pluginID, name string, mode core.ContainerMode, address string, useGPU bool, proxy *string) (dependencies.Dependency, error)
	// JobProgress returns the fraction done of a running image pull job.
	JobProgress(id core.ID) (float64, bool)
}

// PluginManager runs the server's plugins; *plugins.Manager implements it.
type PluginManager interface {
	Plugins() []plugins.Info
	Config(ctx context.Context, id string) (core.PluginConfig, error)
	SetConfig(ctx context.Context, id, configJSON string) (plugins.Info, string, error)
	Definitions(ctx context.Context, id, source string) ([]plugins.ConfigOption, error)
	Catalog(ctx context.Context) ([]plugins.CatalogPlugin, error)
	Install(ctx context.Context, id, version string) (plugins.Info, error)
	Uninstall(ctx context.Context, id string) error
	SetEnabled(ctx context.Context, id string, enabled bool) (plugins.Info, error)
	Restart(ctx context.Context, id string) error
	// AIGateway returns the ready AI gateway plugin.
	AIGateway() (string, pluginv1connect.AiGatewayServiceClient, bool)
	// AgentSessions returns the ready plugin keeping conversations with models.
	AgentSessions() (pluginv1connect.AgentSessionServiceClient, bool)
	Tasks() []plugins.Task
}

var _ systemv1connect.PluginManagementServiceHandler = (*PluginManagementService)(nil)

// pluginChanged tells administrators of a plugin change and records it,
// and returns the plugin in the caller's language.
func (s *PluginManagementService) pluginChanged(ctx context.Context, info plugins.Info, kind, verb string) *systemv1.Plugin {
	if s.Hub != nil {
		// The event's readers read different languages: it carries the
		// plugin's strings untranslated.
		plugin := s.toProto(info, nil)
		s.Hub.ToAdmins(sessionv1.Event_builder{PluginChanged: sessionv1.PluginChanged_builder{Plugin: plugin}.Build()}.Build())
	}
	p, _ := principal(ctx)
	id, version := info.Manifest.GetId(), info.Manifest.GetVersion()
	s.Activity.Record(ctx, core.Activity{
		Type: kind, UserID: p.User.ID, Title: fmt.Sprintf("%s %s plugin %s", p.User.Name, verb, id),
		Attributes: map[string]string{"plugin": id, "version": version},
	})
	return s.toProto(info, wantedLanguages(ctx))
}

// ListCatalogPlugins lists the catalogs' plugins.
func (s *PluginManagementService) ListCatalogPlugins(ctx context.Context, _ *systemv1.ListCatalogPluginsRequest) (*systemv1.ListCatalogPluginsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return &systemv1.ListCatalogPluginsResponse{}, nil
	}
	list, err := s.Plugins.Catalog(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	wanted := wantedLanguages(ctx)
	out := make([]*systemv1.CatalogPlugin, len(list))
	for i, p := range list {
		name, description := p.Localized(wanted)
		versions := make([]*systemv1.CatalogVersion, len(p.Versions))
		for j, v := range p.Versions {
			versions[j] = systemv1.CatalogVersion_builder{Version: &v.Version, Changelog: &v.Changelog}.Build()
			if !v.ReleaseTime.IsZero() {
				versions[j].SetReleaseTime(timestamppb.New(v.ReleaseTime))
			}
		}
		out[i] = systemv1.CatalogPlugin_builder{
			Id: &p.ID, Name: &name, Description: &description, Author: &p.Author, Homepage: &p.Homepage,
			CatalogUrl: &p.CatalogURL, Versions: versions, InstalledVersion: &p.Installed,
			Category: pluginv1.Category(pluginv1.Category_value[p.Category]).Enum(),
		}.Build()
	}
	return systemv1.ListCatalogPluginsResponse_builder{Plugins: out}.Build(), nil
}

// InstallPlugin installs or changes the version of a catalog plugin.
func (s *PluginManagementService) InstallPlugin(ctx context.Context, req *systemv1.InstallPluginRequest) (*systemv1.InstallPluginResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the server runs no plugins"))
	}
	installed := slices.ContainsFunc(s.Plugins.Plugins(), func(i plugins.Info) bool { return i.Manifest.GetId() == req.GetPluginId() })
	info, err := s.Plugins.Install(ctx, req.GetPluginId(), req.GetVersion())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	kind, verb := "plugin.installed", "installed"
	if installed {
		kind, verb = "plugin.updated", "updated"
	}
	plugin := s.pluginChanged(ctx, info, kind, verb)
	return systemv1.InstallPluginResponse_builder{Plugin: plugin}.Build(), nil
}

// UninstallPlugin removes a plugin.
func (s *PluginManagementService) UninstallPlugin(ctx context.Context, req *systemv1.UninstallPluginRequest) (*systemv1.UninstallPluginResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	if err := s.Plugins.Uninstall(ctx, req.GetPluginId()); err != nil {
		return nil, connectError(ctx, err)
	}
	if s.Settings != nil {
		if _, err := s.Settings.Change(ctx, func(cur *core.ServerSettings) error {
			delete(cur.PluginProxies, req.GetPluginId())
			return nil
		}); err != nil {
			slog.WarnContext(ctx, "forget an uninstalled plugin's proxy", "plugin", req.GetPluginId(), "err", err)
		}
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "plugin.uninstalled", UserID: p.User.ID, Title: fmt.Sprintf("%s uninstalled plugin %s", p.User.Name, req.GetPluginId()),
		Attributes: map[string]string{"plugin": req.GetPluginId()},
	})
	return &systemv1.UninstallPluginResponse{}, nil
}

// SetPluginEnabled disables a plugin or enables it again.
func (s *PluginManagementService) SetPluginEnabled(ctx context.Context, req *systemv1.SetPluginEnabledRequest) (*systemv1.SetPluginEnabledResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	info, err := s.Plugins.SetEnabled(ctx, req.GetPluginId(), req.GetEnabled())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	kind, verb := "plugin.disabled", "disabled"
	if req.GetEnabled() {
		kind, verb = "plugin.enabled", "enabled"
	}
	plugin := s.pluginChanged(ctx, info, kind, verb)
	return systemv1.SetPluginEnabledResponse_builder{Plugin: plugin}.Build(), nil
}

// RestartPlugin stops a plugin and starts it again.
func (s *PluginManagementService) RestartPlugin(ctx context.Context, req *systemv1.RestartPluginRequest) (*systemv1.RestartPluginResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id := req.GetPluginId()
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", id, core.ErrNotFound))
	}
	// A plugin that fails to start again is told as failed.
	if err := s.Plugins.Restart(ctx, id); errors.Is(err, core.ErrNotFound) {
		return nil, connectError(ctx, err)
	}
	i := slices.IndexFunc(s.Plugins.Plugins(), func(p plugins.Info) bool { return p.Manifest.GetId() == id })
	if i < 0 {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", id, core.ErrNotFound))
	}
	plugin := s.pluginChanged(ctx, s.Plugins.Plugins()[i], "plugin.restarted", "restarted")
	return systemv1.RestartPluginResponse_builder{Plugin: plugin}.Build(), nil
}

// ListPlugins lists the plugins of the plugin folder.
func (s *PluginManagementService) ListPlugins(ctx context.Context, _ *systemv1.ListPluginsRequest) (*systemv1.ListPluginsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var out []*systemv1.Plugin
	if s.Plugins != nil {
		wanted := wantedLanguages(ctx)
		for _, p := range s.Plugins.Plugins() {
			out = append(out, s.toProto(p, wanted))
		}
	}
	return systemv1.ListPluginsResponse_builder{Plugins: out}.Build(), nil
}

// GetPluginConfig returns a plugin's stored configuration.
func (s *PluginManagementService) GetPluginConfig(ctx context.Context, req *systemv1.GetPluginConfigRequest) (*systemv1.GetPluginConfigResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	c, err := s.Plugins.Config(ctx, req.GetPluginId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	resp := systemv1.GetPluginConfigResponse_builder{ConfigJson: &c.JSON}.Build()
	if !c.UpdatedAt.IsZero() {
		resp.SetUpdateTime(timestamppb.New(c.UpdatedAt))
	}
	return resp, nil
}

// SetPluginConfig configures a plugin.
func (s *PluginManagementService) SetPluginConfig(ctx context.Context, req *systemv1.SetPluginConfigRequest) (*systemv1.SetPluginConfigResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	info, config, err := s.Plugins.SetConfig(ctx, req.GetPluginId(), req.GetConfigJson())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	plugin := s.pluginChanged(ctx, info, "plugin.configured", "configured")
	return systemv1.SetPluginConfigResponse_builder{Plugin: plugin, ConfigJson: &config}.Build(), nil
}

// ListPluginConfigOptions lists the options of a source for a plugin's
// configuration: its providers' definitions, or the download clients.
func (s *PluginManagementService) ListPluginConfigOptions(ctx context.Context, req *systemv1.ListPluginConfigOptionsRequest) (*systemv1.ListPluginConfigOptionsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Plugins == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	var options []plugins.ConfigOption
	switch req.GetSource() {
	case manifest.OptionsDownloadClients, manifest.OptionsProxies:
		if !slices.ContainsFunc(s.Plugins.Plugins(), func(p plugins.Info) bool { return p.Manifest.GetId() == req.GetPluginId() }) {
			return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
		}
		if req.GetSource() == manifest.OptionsDownloadClients {
			options = s.downloadClients(ctx)
		} else {
			var err error
			if options, err = s.proxyOptions(ctx); err != nil {
				return nil, connectError(ctx, err)
			}
		}
	default:
		var err error
		if options, err = s.Plugins.Definitions(ctx, req.GetPluginId(), req.GetSource()); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	// The plugin's own options, with the settings they take, are in its
	// language files.
	tr := s.translator(ctx, req.GetPluginId())
	if req.GetSource() == manifest.OptionsDownloadClients || req.GetSource() == manifest.OptionsProxies {
		tr = nil
	}
	out := make([]*systemv1.PluginConfigOption, 0, len(options))
	for _, o := range options {
		if tr != nil {
			o.Name, o.Description = tr(o.Name), tr(o.Description)
			if o.SettingsSchema != "" {
				o.SettingsSchema = manifest.LocalizeSchema(o.SettingsSchema, tr)
			}
		}
		out = append(out, systemv1.PluginConfigOption_builder{
			Value: &o.Value, Name: &o.Name, Description: &o.Description, Tags: o.Tags, SettingsSchema: &o.SettingsSchema,
		}.Build())
	}
	return systemv1.ListPluginConfigOptionsResponse_builder{Options: out}.Build(), nil
}

// proxyOptions returns the server's proxies as options: their IDs, named
// and described by their kinds and addresses, the system proxy first,
// tagged "system" and undescribed when the system sets none.
func (s *PluginManagementService) proxyOptions(ctx context.Context) ([]plugins.ConfigOption, error) {
	if s.Proxies == nil {
		return nil, nil
	}
	list, err := s.Proxies.List(ctx)
	if err != nil {
		return nil, err
	}
	sys := s.Proxies.System(ctx)
	out := make([]plugins.ConfigOption, 0, len(list)+1)
	system := plugins.ConfigOption{Value: sys.GetId(), Name: sys.GetName(), Tags: []string{proxies.SystemID}}
	if u := proxy.URL(sys); u != nil {
		system.Description = u.Scheme + "://" + u.Host
		system.Tags = append(system.Tags, u.Scheme)
	}
	out = append(out, system)
	for _, p := range list {
		out = append(out, plugins.ConfigOption{
			Value: p.ID.String(), Name: p.Name, Description: string(p.Kind) + "://" + p.Address(), Tags: []string{string(p.Kind)},
		})
	}
	return out, nil
}

// downloadClients returns the clients of every download client provider
// as options, "<plugin>/<client>"; a provider failing to list them is
// left out.
func (s *PluginManagementService) downloadClients(ctx context.Context) []plugins.ConfigOption {
	if s.DownloadClients == nil {
		return nil
	}
	var out []plugins.ConfigOption
	for _, p := range s.DownloadClients() {
		resp, err := p.Client.ListDownloadClients(ctx, &pluginv1.ListDownloadClientsRequest{})
		if err != nil {
			slog.WarnContext(ctx, "list download clients", "plugin", p.ID, "err", err)
			continue
		}
		for _, c := range resp.GetClients() {
			o := plugins.ConfigOption{Value: p.ID + "/" + c.GetId(), Name: c.GetName()}
			if t := plugins.ProtocolTag(c.GetProtocol()); t != "" {
				o.Tags = []string{t}
			}
			out = append(out, o)
		}
	}
	return out
}

var pluginStates = map[plugins.State]systemv1.PluginState{
	plugins.Ready:        systemv1.PluginState_PLUGIN_STATE_READY,
	plugins.Unconfigured: systemv1.PluginState_PLUGIN_STATE_UNCONFIGURED,
	plugins.Failed:       systemv1.PluginState_PLUGIN_STATE_FAILED,
	plugins.Disabled:     systemv1.PluginState_PLUGIN_STATE_DISABLED,
}

// toProto returns a plugin as clients see it, with its proxy.
func (s *PluginManagementService) toProto(p plugins.Info, wanted []language.Tag) *systemv1.Plugin {
	out := pluginToProto(p, wanted)
	if s.Settings != nil && proxied(p.Manifest) {
		out.SetProxy(PluginProxy(s.Settings.Get(), p.Manifest.GetId()))
	}
	return out
}

// proxied reports whether a plugin's runtime lets the server choose its
// proxy: WASM plugins' requests the server performs, and process
// plugins' go to its proxy gateway.
func proxied(m *pluginv1.Manifest) bool {
	rt := m.GetRuntime()
	return rt == pluginv1.Runtime_RUNTIME_WASM || rt == pluginv1.Runtime_RUNTIME_PROCESS
}

// PluginProxy returns the ID of the proxy a plugin reaches sites through,
// the system proxy unless set.
func PluginProxy(set core.ServerSettings, pluginID string) string {
	if id, ok := set.PluginProxies[pluginID]; ok {
		return id
	}
	return core.SystemProxyID
}

// SetPluginProxy sets the proxy a plugin reaches sites through.
func (s *PluginManagementService) SetPluginProxy(ctx context.Context, req *systemv1.SetPluginProxyRequest) (*systemv1.SetPluginProxyResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id := req.GetPluginId()
	var info plugins.Info
	if s.Plugins != nil {
		i := slices.IndexFunc(s.Plugins.Plugins(), func(p plugins.Info) bool { return p.Manifest.GetId() == id })
		if i >= 0 {
			info = s.Plugins.Plugins()[i]
		}
	}
	switch {
	case info.Manifest == nil || s.Settings == nil:
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", id, core.ErrNotFound))
	case !proxied(info.Manifest):
		return nil, connectError(ctx, fmt.Errorf("%w: plugin %s has no runtime", core.ErrInvalid, id))
	}
	if _, err := s.Settings.Change(ctx, func(cur *core.ServerSettings) error {
		if cur.PluginProxies == nil {
			cur.PluginProxies = map[string]string{}
		}
		cur.PluginProxies[id] = req.GetProxy()
		return nil
	}); err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{
		Type: "plugin.proxy_changed", UserID: p.User.ID, Title: fmt.Sprintf("%s changed the proxy of plugin %s", p.User.Name, id),
		Attributes: map[string]string{"plugin": id},
	})
	if s.ProxyChanged != nil {
		s.ProxyChanged(ctx, id)
	}
	out := s.toProto(info, wantedLanguages(ctx))
	if s.Hub != nil {
		s.Hub.ToAdmins(sessionv1.Event_builder{PluginChanged: sessionv1.PluginChanged_builder{Plugin: s.toProto(info, nil)}.Build()}.Build())
	}
	return systemv1.SetPluginProxyResponse_builder{Plugin: out}.Build(), nil
}

func pluginToProto(p plugins.Info, wanted []language.Tag) *systemv1.Plugin {
	man := p.Manifest
	if len(wanted) > 0 && len(p.Locales) > 0 {
		man = manifest.Localize(man, p.Locales.Translator(wanted))
	}
	out := systemv1.Plugin_builder{Manifest: man, State: new(pluginStates[p.State])}.Build()
	if p.Err != nil {
		out.SetError(p.Err.Error())
	}
	if p.DevFolder != "" {
		out.SetDevFolder(filepath.ToSlash(p.DevFolder))
	}
	return out
}

// GetContainerRuntime tells whether a container engine is available.
func (s *PluginManagementService) GetContainerRuntime(ctx context.Context, req *systemv1.GetContainerRuntimeRequest) (*systemv1.GetContainerRuntimeResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := &systemv1.ContainerRuntime{}
	if s.Containers == nil {
		out.SetReason("the server runs no container dependencies")
		return systemv1.GetContainerRuntimeResponse_builder{Runtime: out}.Build(), nil
	}
	rt, err := s.Containers.Runtime(ctx, req.GetDetect())
	out.SetInContainer(rt.InContainer)
	if err != nil {
		if ctx.Err() != nil {
			return nil, connectError(ctx, err)
		}
		out.SetReason(err.Error())
		return systemv1.GetContainerRuntimeResponse_builder{Runtime: out}.Build(), nil
	}
	out.SetAvailable(true)
	out.SetEndpoint(rt.Client.Endpoint().String())
	out.SetEngine(rt.Version.Engine)
	out.SetVersion(rt.Version.Version)
	out.SetApiVersion(rt.APIVersion)
	out.SetRootless(rt.Rootless)
	out.SetOnHost(rt.OnHost())
	out.SetGpu(rt.GPU != containers.NoGPU)
	return systemv1.GetContainerRuntimeResponse_builder{Runtime: out}.Build(), nil
}

// ListPluginContainers lists a plugin's container dependencies.
func (s *PluginManagementService) ListPluginContainers(ctx context.Context, req *systemv1.ListPluginContainersRequest) (*systemv1.ListPluginContainersResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Containers == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	deps, err := s.Containers.List(ctx, req.GetPluginId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	tr := s.translator(ctx, req.GetPluginId())
	out := make([]*systemv1.PluginContainer, len(deps))
	for i, d := range deps {
		out[i] = pluginContainerToProto(d, tr)
	}
	return systemv1.ListPluginContainersResponse_builder{Containers: out}.Build(), nil
}

// ListContainers lists every plugin's container dependencies the server
// runs or was given an address of.
func (s *PluginManagementService) ListContainers(ctx context.Context, _ *systemv1.ListContainersRequest) (*systemv1.ListContainersResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var out []*systemv1.PluginContainer
	if s.Containers == nil || s.Plugins == nil {
		return systemv1.ListContainersResponse_builder{}.Build(), nil
	}
	for _, p := range s.Plugins.Plugins() {
		id := p.Manifest.GetId()
		if len(p.Manifest.GetContainers()) == 0 {
			continue
		}
		deps, err := s.Containers.List(ctx, id)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, connectError(ctx, err)
		}
		tr := s.translator(ctx, id)
		for _, d := range deps {
			if d.Kept.Mode != core.ContainerUnset {
				out = append(out, pluginContainerToProto(d, tr))
			}
		}
	}
	return systemv1.ListContainersResponse_builder{Containers: out}.Build(), nil
}

var containerModes = map[pluginv1.ContainerMode]core.ContainerMode{
	pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED: core.ContainerUnset,
	pluginv1.ContainerMode_CONTAINER_MODE_MANAGED:     core.ContainerManaged,
	pluginv1.ContainerMode_CONTAINER_MODE_EXTERNAL:    core.ContainerExternal,
}

// SetPluginContainer sets how a container dependency is reached.
func (s *PluginManagementService) SetPluginContainer(ctx context.Context, req *systemv1.SetPluginContainerRequest) (*systemv1.SetPluginContainerResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Containers == nil {
		return nil, connectError(ctx, fmt.Errorf("plugin %s: %w", req.GetPluginId(), core.ErrNotFound))
	}
	mode, ok := containerModes[req.GetMode()]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown container mode %v", req.GetMode()))
	}
	var proxy *string
	if req.HasProxy() {
		proxy = new(req.GetProxy())
	}
	d, err := s.Containers.Set(ctx, req.GetPluginId(), req.GetName(), mode, req.GetAddress(), req.GetUseGpu(), proxy)
	switch {
	case errors.Is(err, dependencies.ErrNoEngine):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, errors.ErrUnsupported):
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	case err != nil:
		return nil, connectError(ctx, err)
	}
	p, _ := principal(ctx)
	s.Activity.Record(ctx, core.Activity{
		Type: "plugin.container", UserID: p.User.ID,
		Title:      fmt.Sprintf("%s set container %s of plugin %s to %s", p.User.Name, req.GetName(), req.GetPluginId(), mode),
		Attributes: map[string]string{"plugin": req.GetPluginId(), "container": req.GetName(), "mode": string(mode)},
	})
	return systemv1.SetPluginContainerResponse_builder{Container: pluginContainerToProto(d, s.translator(ctx, req.GetPluginId()))}.Build(), nil
}

// translator returns what translates a plugin's strings into the caller's
// language, nil when they stay as they are.
func (s *PluginManagementService) translator(ctx context.Context, pluginID string) func(string) string {
	wanted := wantedLanguages(ctx)
	if len(wanted) == 0 || s.Plugins == nil {
		return nil
	}
	for _, p := range s.Plugins.Plugins() {
		if p.Manifest.GetId() == pluginID && len(p.Locales) > 0 {
			return p.Locales.Translator(wanted)
		}
	}
	return nil
}

// pluginContainerToProto returns a dependency with its description
// translated by tr, when not nil.
func pluginContainerToProto(d dependencies.Dependency, tr func(string) string) *systemv1.PluginContainer {
	spec := d.Spec
	if tr != nil && spec.HasDescription() {
		spec = proto.CloneOf(spec)
		spec.SetDescription(tr(spec.GetDescription()))
	}
	return systemv1.PluginContainer_builder{
		Dependency: spec, Status: d.Status, UseGpu: new(d.Kept.UseGPU),
		Commands: containerCommandsToProto(d.Commands), GpuCommands: containerCommandsToProto(d.GPUCommands),
		PluginId: new(d.Kept.PluginID), Proxy: new(d.Kept.Proxy), Command: new(d.Command),
	}.Build()
}

func containerCommandsToProto(c *containers.Commands) *systemv1.ContainerCommands {
	if c == nil {
		return nil
	}
	return systemv1.ContainerCommands_builder{DockerRun: new(c.DockerRun), Compose: new(c.Compose), Address: new(c.Address)}.Build()
}

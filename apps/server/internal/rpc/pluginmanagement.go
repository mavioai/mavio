package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
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
}

// PluginManager runs the server's plugins; *plugins.Manager implements it.
type PluginManager interface {
	Plugins() []plugins.Info
	Config(ctx context.Context, id string) (core.PluginConfig, error)
	SetConfig(ctx context.Context, id, configJSON string) (plugins.Info, error)
	Catalog(ctx context.Context) ([]plugins.CatalogPlugin, error)
	Install(ctx context.Context, id, version string) (plugins.Info, error)
	Uninstall(ctx context.Context, id string) error
	SetEnabled(ctx context.Context, id string, enabled bool) (plugins.Info, error)
	// AIGateway returns the ready AI gateway plugin.
	AIGateway() (string, pluginv1connect.AiGatewayServiceClient, bool)
	// AgentSessions returns the ready plugin keeping conversations with models.
	AgentSessions() (pluginv1connect.AgentSessionServiceClient, bool)
	Tasks() []plugins.Task
}

var _ systemv1connect.PluginManagementServiceHandler = (*PluginManagementService)(nil)

// pluginChanged tells administrators of a plugin change and records it.
func (s *PluginManagementService) pluginChanged(ctx context.Context, info plugins.Info, kind, verb string) *systemv1.Plugin {
	plugin := pluginToProto(info)
	if s.Hub != nil {
		s.Hub.ToAdmins(sessionv1.Event_builder{PluginChanged: sessionv1.PluginChanged_builder{Plugin: plugin}.Build()}.Build())
	}
	p, _ := principal(ctx)
	id, version := info.Manifest.GetId(), info.Manifest.GetVersion()
	s.Activity.Record(ctx, core.Activity{
		Type: kind, UserID: p.User.ID, Title: fmt.Sprintf("%s %s plugin %s", p.User.Name, verb, id),
		Attributes: map[string]string{"plugin": id, "version": version},
	})
	return plugin
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
	out := make([]*systemv1.CatalogPlugin, len(list))
	for i, p := range list {
		versions := make([]*systemv1.CatalogVersion, len(p.Versions))
		for j, v := range p.Versions {
			versions[j] = systemv1.CatalogVersion_builder{Version: &v.Version, Changelog: &v.Changelog}.Build()
			if !v.ReleaseTime.IsZero() {
				versions[j].SetReleaseTime(timestamppb.New(v.ReleaseTime))
			}
		}
		out[i] = systemv1.CatalogPlugin_builder{
			Id: &p.ID, Name: &p.Name, Description: &p.Description, Author: &p.Author, Homepage: &p.Homepage,
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

// ListPlugins lists the plugins of the plugin folder.
func (s *PluginManagementService) ListPlugins(ctx context.Context, _ *systemv1.ListPluginsRequest) (*systemv1.ListPluginsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var out []*systemv1.Plugin
	if s.Plugins != nil {
		for _, p := range s.Plugins.Plugins() {
			out = append(out, pluginToProto(p))
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
	info, err := s.Plugins.SetConfig(ctx, req.GetPluginId(), req.GetConfigJson())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	plugin := s.pluginChanged(ctx, info, "plugin.configured", "configured")
	return systemv1.SetPluginConfigResponse_builder{Plugin: plugin}.Build(), nil
}

var pluginStates = map[plugins.State]systemv1.PluginState{
	plugins.Ready:        systemv1.PluginState_PLUGIN_STATE_READY,
	plugins.Unconfigured: systemv1.PluginState_PLUGIN_STATE_UNCONFIGURED,
	plugins.Failed:       systemv1.PluginState_PLUGIN_STATE_FAILED,
	plugins.Disabled:     systemv1.PluginState_PLUGIN_STATE_DISABLED,
}

func pluginToProto(p plugins.Info) *systemv1.Plugin {
	out := systemv1.Plugin_builder{Manifest: p.Manifest, State: new(pluginStates[p.State])}.Build()
	if p.Err != nil {
		out.SetError(p.Err.Error())
	}
	return out
}

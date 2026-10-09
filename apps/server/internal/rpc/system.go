// Package rpc implements the Connect services defined in libs/proto.
package rpc

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/logs"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// SystemService implements mavio.system.v1.SystemService.
type SystemService struct {
	Version   string
	StartTime time.Time
	// Database is "sqlite" or "postgres"; empty until storage is wired.
	Database string
	// FFmpegVersion is the first line of `ffmpeg -version`, if available.
	FFmpegVersion string
	// Plugins runs the server's plugins; nil means none.
	Plugins PluginManager
	// Hub tells administrators of plugin changes; nil tells no one.
	Hub *events.Hub
	// Settings keeps the server settings; nil serves the defaults and
	// changes none.
	Settings *settings.Manager
	// Accelerations lists the hardware accelerations available; nil means
	// auto and none.
	Accelerations func() []core.HardwareAcceleration
	// Logs keeps the recent log records; nil keeps none.
	Logs *logs.Ring
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
}

var _ systemv1connect.SystemServiceHandler = (*SystemService)(nil)

// GetHealth reports the server as serving.
func (s *SystemService) GetHealth(context.Context, *systemv1.GetHealthRequest) (*systemv1.GetHealthResponse, error) {
	resp := &systemv1.GetHealthResponse{}
	resp.SetStatus(systemv1.GetHealthResponse_STATUS_SERVING)
	resp.SetVersion(s.Version)
	return resp, nil
}

// GetSystemInfo describes the running server.
func (s *SystemService) GetSystemInfo(context.Context, *systemv1.GetSystemInfoRequest) (*systemv1.GetSystemInfoResponse, error) {
	return systemv1.GetSystemInfoResponse_builder{
		Version:       &s.Version,
		Os:            new(runtime.GOOS),
		Arch:          new(runtime.GOARCH),
		Database:      &s.Database,
		FfmpegVersion: &s.FFmpegVersion,
		StartTime:     timestamppb.New(s.StartTime),
	}.Build(), nil
}

// ListPlugins lists the plugins of the plugin folder.
func (s *SystemService) ListPlugins(ctx context.Context, _ *systemv1.ListPluginsRequest) (*systemv1.ListPluginsResponse, error) {
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
func (s *SystemService) GetPluginConfig(ctx context.Context, req *systemv1.GetPluginConfigRequest) (*systemv1.GetPluginConfigResponse, error) {
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
func (s *SystemService) SetPluginConfig(ctx context.Context, req *systemv1.SetPluginConfigRequest) (*systemv1.SetPluginConfigResponse, error) {
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
}

func pluginToProto(p plugins.Info) *systemv1.Plugin {
	out := systemv1.Plugin_builder{Manifest: p.Manifest, State: new(pluginStates[p.State])}.Build()
	if p.Err != nil {
		out.SetError(p.Err.Error())
	}
	return out
}

package rpc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

// pluginChanged tells administrators of a plugin change and records it.
func (s *SystemService) pluginChanged(ctx context.Context, info plugins.Info, kind, verb string) *systemv1.Plugin {
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
func (s *SystemService) ListCatalogPlugins(ctx context.Context, _ *systemv1.ListCatalogPluginsRequest) (*systemv1.ListCatalogPluginsResponse, error) {
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
		}.Build()
	}
	return systemv1.ListCatalogPluginsResponse_builder{Plugins: out}.Build(), nil
}

// InstallPlugin installs or changes the version of a catalog plugin.
func (s *SystemService) InstallPlugin(ctx context.Context, req *systemv1.InstallPluginRequest) (*systemv1.InstallPluginResponse, error) {
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
func (s *SystemService) UninstallPlugin(ctx context.Context, req *systemv1.UninstallPluginRequest) (*systemv1.UninstallPluginResponse, error) {
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

var hardwareAccelerations = map[core.HardwareAcceleration]systemv1.HardwareAcceleration{
	core.HardwareAuto:         systemv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO,
	core.HardwareNone:         systemv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE,
	core.HardwareVideoToolbox: systemv1.HardwareAcceleration_HARDWARE_ACCELERATION_VIDEOTOOLBOX,
}

func settingsToProto(s *core.ServerSettings) *systemv1.ServerSettings {
	t, n := &s.Transcoding, &s.Network
	out := systemv1.ServerSettings_builder{
		Transcoding: systemv1.TranscodingSettings_builder{
			HardwareAcceleration: new(hardwareAccelerations[t.HardwareAcceleration]), HardwareEncoding: &t.HardwareEncoding,
			EncoderPreset: &t.EncoderPreset, H264Crf: new(int32(t.H264CRF)), H265Crf: new(int32(t.H265CRF)), Threads: new(int32(t.Threads)),
			TonemapAlgorithm: &t.TonemapAlgorithm, TonemapRange: &t.TonemapRange, TonemapDesat: &t.TonemapDesat, TonemapPeak: &t.TonemapPeak,
			DeinterlaceMethod: &t.DeinterlaceMethod, DeinterlaceDoubleRate: &t.DeinterlaceDoubleRate, DownmixBoost: &t.DownmixBoost,
			CropBlackBorders: &t.CropBlackBorders, TranscodeDir: &t.TranscodeDir,
		}.Build(),
		Network: systemv1.NetworkSettings_builder{
			ServerName: &n.ServerName, BaseUrl: &n.BaseURL, HttpsPort: new(int32(n.HTTPSPort)), CertificatePath: &n.CertificatePath,
			KeyPath: &n.KeyPath, LocalDiscovery: &n.LocalDiscovery,
		}.Build(),
		PluginCatalogs: s.PluginCatalogs, PasswordResetPlugin: &s.PasswordResetPlugin,
	}.Build()
	if !s.UpdatedAt.IsZero() {
		out.SetUpdateTime(timestamppb.New(s.UpdatedAt))
	}
	return out
}

func settingsFromProto(p *systemv1.ServerSettings) core.ServerSettings {
	t, n := p.GetTranscoding(), p.GetNetwork()
	var hw core.HardwareAcceleration
	for k, v := range hardwareAccelerations {
		if v == t.GetHardwareAcceleration() {
			hw = k
		}
	}
	return core.ServerSettings{
		Transcoding: core.TranscodingSettings{
			HardwareAcceleration: hw, HardwareEncoding: t.GetHardwareEncoding(), EncoderPreset: t.GetEncoderPreset(),
			H264CRF: int(t.GetH264Crf()), H265CRF: int(t.GetH265Crf()), Threads: int(t.GetThreads()),
			TonemapAlgorithm: t.GetTonemapAlgorithm(), TonemapRange: t.GetTonemapRange(), TonemapDesat: t.GetTonemapDesat(),
			TonemapPeak: t.GetTonemapPeak(), DeinterlaceMethod: t.GetDeinterlaceMethod(), DeinterlaceDoubleRate: t.GetDeinterlaceDoubleRate(),
			DownmixBoost: t.GetDownmixBoost(), CropBlackBorders: t.GetCropBlackBorders(), TranscodeDir: t.GetTranscodeDir(),
		},
		Network: core.NetworkSettings{
			ServerName: n.GetServerName(), BaseURL: n.GetBaseUrl(), HTTPSPort: int(n.GetHttpsPort()),
			CertificatePath: n.GetCertificatePath(), KeyPath: n.GetKeyPath(), LocalDiscovery: n.GetLocalDiscovery(),
		},
		PluginCatalogs: p.GetPluginCatalogs(), PasswordResetPlugin: p.GetPasswordResetPlugin(),
	}
}

func (s *SystemService) currentSettings() core.ServerSettings {
	if s.Settings == nil {
		return core.DefaultServerSettings()
	}
	return s.Settings.Get()
}

// GetServerSettings returns the server settings.
func (s *SystemService) GetServerSettings(ctx context.Context, _ *systemv1.GetServerSettingsRequest) (*systemv1.GetServerSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	set := s.currentSettings()
	available := []core.HardwareAcceleration{core.HardwareAuto, core.HardwareNone}
	if s.Accelerations != nil {
		available = s.Accelerations()
	}
	out := make([]systemv1.HardwareAcceleration, len(available))
	for i, a := range available {
		out[i] = hardwareAccelerations[a]
	}
	return systemv1.GetServerSettingsResponse_builder{Settings: settingsToProto(&set), AvailableHardwareAccelerations: out}.Build(), nil
}

// UpdateServerSettings applies and stores new settings.
func (s *SystemService) UpdateServerSettings(ctx context.Context, req *systemv1.UpdateServerSettingsRequest) (*systemv1.UpdateServerSettingsResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if s.Settings == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("settings cannot be changed"))
	}
	set, err := s.Settings.Update(ctx, settingsFromProto(req.GetSettings()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.Activity.Record(ctx, core.Activity{Type: "settings.updated", UserID: p.User.ID, Title: p.User.Name + " changed the server settings"})
	return systemv1.UpdateServerSettingsResponse_builder{Settings: settingsToProto(&set)}.Build(), nil
}

// ListDirectory lists a folder of the server, folders first.
func (s *SystemService) ListDirectory(ctx context.Context, req *systemv1.ListDirectoryRequest) (*systemv1.ListDirectoryResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	dir := req.GetPath()
	if dir == "" {
		var entries []*systemv1.DirectoryEntry
		for _, r := range roots() {
			entries = append(entries, systemv1.DirectoryEntry_builder{Name: new(r), Path: new(r), IsDir: new(true)}.Build())
		}
		return systemv1.ListDirectoryResponse_builder{Entries: entries}.Build(), nil
	}
	if !filepath.IsAbs(dir) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path %q is not absolute", dir))
	}
	dir = filepath.Clean(dir)
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("folder %s: %w", dir, err))
	}
	var entries []*systemv1.DirectoryEntry
	for _, e := range list {
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(dir, e.Name())); err == nil {
				isDir = info.IsDir()
			}
		}
		if strings.HasPrefix(e.Name(), ".") || !isDir && !req.GetIncludeFiles() {
			continue
		}
		entries = append(entries, systemv1.DirectoryEntry_builder{
			Name: new(e.Name()), Path: new(filepath.Join(dir, e.Name())), IsDir: new(isDir),
		}.Build())
	}
	slices.SortStableFunc(entries, func(a, b *systemv1.DirectoryEntry) int {
		if a.GetIsDir() != b.GetIsDir() {
			if a.GetIsDir() {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
	})
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}
	return systemv1.ListDirectoryResponse_builder{Path: &dir, Parent: &parent, Entries: entries}.Build(), nil
}

// roots returns the file system roots: "/", or the drives on Windows.
func roots() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	var out []string
	for d := 'A'; d <= 'Z'; d++ {
		r := string(d) + `:\`
		if _, err := os.Stat(r); err == nil {
			out = append(out, r)
		}
	}
	return out
}

var logLevels = map[systemv1.LogLevel]slog.Level{
	systemv1.LogLevel_LOG_LEVEL_DEBUG: slog.LevelDebug, systemv1.LogLevel_LOG_LEVEL_INFO: slog.LevelInfo,
	systemv1.LogLevel_LOG_LEVEL_WARN: slog.LevelWarn, systemv1.LogLevel_LOG_LEVEL_ERROR: slog.LevelError,
}

func levelToProto(l slog.Level) systemv1.LogLevel {
	switch {
	case l >= slog.LevelError:
		return systemv1.LogLevel_LOG_LEVEL_ERROR
	case l >= slog.LevelWarn:
		return systemv1.LogLevel_LOG_LEVEL_WARN
	case l >= slog.LevelInfo:
		return systemv1.LogLevel_LOG_LEVEL_INFO
	}
	return systemv1.LogLevel_LOG_LEVEL_DEBUG
}

// ListLogs lists the recent log records.
func (s *SystemService) ListLogs(ctx context.Context, req *systemv1.ListLogsRequest) (*systemv1.ListLogsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Logs == nil {
		return &systemv1.ListLogsResponse{}, nil
	}
	min, ok := logLevels[req.GetMinLevel()]
	if !ok {
		min = slog.LevelInfo
	}
	var since time.Time
	if req.HasMaxAge() {
		since = time.Now().Add(-req.GetMaxAge().AsDuration())
	}
	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 200
	}
	var out []*systemv1.LogRecord
	for _, r := range s.Logs.Records(min, since, limit) {
		out = append(out, systemv1.LogRecord_builder{
			Time: timestamppb.New(r.Time), Level: new(levelToProto(r.Level)), Message: &r.Message, Attributes: r.Attributes,
		}.Build())
	}
	return systemv1.ListLogsResponse_builder{Records: out}.Build(), nil
}

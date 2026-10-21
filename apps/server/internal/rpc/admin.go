package rpc

import (
	"cmp"
	"context"
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

	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

func generalToProto(s *core.ServerSettings) *systemv1.GeneralSettings {
	n := &s.Network
	return systemv1.GeneralSettings_builder{
		Network: systemv1.NetworkSettings_builder{
			ServerName: &n.ServerName, BaseUrl: &n.BaseURL, HttpsPort: new(int32(n.HTTPSPort)), CertificatePath: &n.CertificatePath,
			KeyPath: &n.KeyPath, LocalDiscovery: &n.LocalDiscovery, Relay: &n.Relay,
		}.Build(),
		Metadata: systemv1.MetadataSettings_builder{
			PreferredLanguage: &s.Metadata.PreferredLanguage, Country: &s.Metadata.Country,
		}.Build(),
		SessionIdleDays:     new(int32(s.SessionIdleDays)),
		PasswordResetPlugin: &s.PasswordResetPlugin,
	}.Build()
}

// setGeneral sets the general settings of p in s, keeping the outbound
// proxies, which ProxyService keeps.
func setGeneral(s *core.ServerSettings, p *systemv1.GeneralSettings) {
	n := p.GetNetwork()
	s.Network = core.NetworkSettings{
		ServerName: n.GetServerName(), BaseURL: n.GetBaseUrl(), HTTPSPort: int(n.GetHttpsPort()),
		CertificatePath: n.GetCertificatePath(), KeyPath: n.GetKeyPath(), LocalDiscovery: n.GetLocalDiscovery(), Relay: n.GetRelay(),
		Proxies: s.Network.Proxies,
	}
	s.Metadata = core.MetadataSettings{PreferredLanguage: p.GetMetadata().GetPreferredLanguage(), Country: p.GetMetadata().GetCountry()}
	s.SessionIdleDays = int(p.GetSessionIdleDays())
	s.PasswordResetPlugin = p.GetPasswordResetPlugin()
}

// GetGeneralSettings returns the server's general settings.
func (s *SystemService) GetGeneralSettings(ctx context.Context, _ *systemv1.GetGeneralSettingsRequest) (*systemv1.GetGeneralSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	set := currentSettings(s.Settings)
	return systemv1.GetGeneralSettingsResponse_builder{Settings: generalToProto(&set)}.Build(), nil
}

// UpdateGeneralSettings applies and stores the general settings.
func (s *SystemService) UpdateGeneralSettings(ctx context.Context, req *systemv1.UpdateGeneralSettingsRequest) (*systemv1.UpdateGeneralSettingsResponse, error) {
	set, err := changeSettings(ctx, s.Settings, s.Activity, "the general settings", func(cur *core.ServerSettings) error {
		setGeneral(cur, req.GetSettings())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return systemv1.UpdateGeneralSettingsResponse_builder{Settings: generalToProto(&set)}.Build(), nil
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
	for _, r := range s.Logs.Records(min, since, req.GetPluginId(), limit) {
		out = append(out, systemv1.LogRecord_builder{
			Time: timestamppb.New(r.Time), Level: new(levelToProto(r.Level)), Message: &r.Message, Attributes: r.Attributes,
		}.Build())
	}
	return systemv1.ListLogsResponse_builder{Records: out}.Build(), nil
}

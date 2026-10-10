// Package host loads plugins and exposes them through one interface,
// whatever runtime they use.
package host

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mavioai/mavio/libs/plugin/host/process"
	"github.com/mavioai/mavio/libs/plugin/host/wasm"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Plugin is a running plugin. Capability clients are nil when the manifest
// does not declare the capability.
type Plugin interface {
	Manifest() *pluginv1.Manifest
	Lifecycle() pluginv1connect.PluginServiceClient
	Metadata() pluginv1connect.MetadataProviderServiceClient
	Auth() pluginv1connect.AuthProviderServiceClient
	Notifier() pluginv1connect.NotifierServiceClient
	Subtitles() pluginv1connect.SubtitleProviderServiceClient
	Segments() pluginv1connect.MediaSegmentProviderServiceClient
	Tasks() pluginv1connect.TaskRunnerServiceClient
	Events() pluginv1connect.EventConsumerServiceClient
	Devices() pluginv1connect.DeviceControllerServiceClient
	Images() pluginv1connect.ImageProviderServiceClient
	LocalMetadata() pluginv1connect.LocalMetadataServiceClient
	Saver() pluginv1connect.MetadataSaverServiceClient
	Processor() pluginv1connect.MetadataProcessorServiceClient
	Lyrics() pluginv1connect.LyricsProviderServiceClient
	Resolver() pluginv1connect.ResolverServiceClient
	// HTTP returns the handler of the plugin's HTTP routes, or nil without
	// CAPABILITY_HTTP_HANDLER; it expects paths relative to the routes' prefix.
	HTTP() http.Handler
	// Close stops the plugin and releases its resources.
	Close(ctx context.Context) error
}

var (
	_ Plugin = (*wasm.Plugin)(nil)
	_ Plugin = (*process.Plugin)(nil)
)

// Options configures both runtimes.
type Options struct {
	WASM    wasm.Options
	Process process.Options
	// HostAPI returns the handler serving a plugin's host API requests; it
	// is called once per plugin, and nil serves none.
	HostAPI func(m *pluginv1.Manifest) http.Handler
	// DataDir holds a writable data folder per plugin, named by its ID and
	// created when the plugin starts; empty gives plugins none.
	DataDir string
}

// WithCallTimeout returns ctx bounding the calls made with it by d instead
// of the WASM runtime's call timeout; process plugins have no call timeout
// besides ctx's.
func WithCallTimeout(ctx context.Context, d time.Duration) context.Context {
	return wasm.WithCallTimeout(ctx, d)
}

// DataDir returns the data folder of plugin id in base.
func DataDir(base, id string) string { return filepath.Join(base, id) }

// Open starts the plugin in dir and checks that it describes itself as its
// manifest says.
func Open(ctx context.Context, dir string, opts Options) (Plugin, error) {
	m, err := manifest.Load(dir)
	if err != nil {
		return nil, err
	}
	exe := manifest.Executable(dir, m)
	if _, err := os.Stat(exe); err != nil {
		return nil, fmt.Errorf("plugin %s: %w", m.GetId(), err)
	}

	if opts.HostAPI != nil {
		h := opts.HostAPI(m)
		opts.WASM.HostAPI, opts.Process.HostAPI = h, h
	}
	if opts.DataDir != "" {
		dir := DataDir(opts.DataDir, m.GetId())
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("plugin %s: data folder: %w", m.GetId(), err)
		}
		opts.WASM.DataDir, opts.Process.DataDir = dir, dir
	}
	var p Plugin
	switch m.GetRuntime() {
	case pluginv1.Runtime_RUNTIME_WASM:
		p, err = wasm.Open(ctx, exe, m, opts.WASM)
	case pluginv1.Runtime_RUNTIME_PROCESS:
		p, err = process.Start(ctx, exe, m, opts.Process)
	default:
		err = fmt.Errorf("plugin %s: unsupported runtime %v", m.GetId(), m.GetRuntime())
	}
	if err != nil {
		return nil, err
	}

	desc, err := p.Lifecycle().Describe(ctx, &pluginv1.DescribeRequest{})
	if err == nil && (desc.GetManifest().GetId() != m.GetId() || desc.GetManifest().GetVersion() != m.GetVersion()) {
		err = fmt.Errorf("plugin describes itself as %s %s, manifest says %s %s",
			desc.GetManifest().GetId(), desc.GetManifest().GetVersion(), m.GetId(), m.GetVersion())
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("plugin %s: %w", m.GetId(), err), p.Close(ctx))
	}
	return p, nil
}

// Configure validates configJSON against the plugin's schema and delivers
// it.
func Configure(ctx context.Context, p Plugin, configJSON string) error {
	if err := manifest.ValidateConfig(p.Manifest(), configJSON); err != nil {
		return fmt.Errorf("plugin %s: %w", p.Manifest().GetId(), err)
	}
	req := &pluginv1.ConfigureRequest{}
	req.SetConfigJson(configJSON)
	_, err := p.Lifecycle().Configure(ctx, req)
	return err
}

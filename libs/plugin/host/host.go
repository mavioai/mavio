// Package host loads plugins and exposes them through one interface,
// whatever runtime they use.
package host

import (
	"context"
	"errors"
	"fmt"
	"os"

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
}

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

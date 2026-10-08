// Package clients builds the Connect clients of a plugin's services. Both
// runtimes use it, differing only in the HTTP client that carries calls.
package clients

import (
	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Set holds a plugin's manifest and clients. Capability clients are nil when
// the manifest does not declare the capability.
type Set struct {
	manifest  *pluginv1.Manifest
	lifecycle pluginv1connect.PluginServiceClient
	metadata  pluginv1connect.MetadataProviderServiceClient
	auth      pluginv1connect.AuthProviderServiceClient
	notifier  pluginv1connect.NotifierServiceClient
}

// New builds the clients for m over c.
func New(c connect.HTTPClient, baseURL string, m *pluginv1.Manifest, opts ...connect.ClientOption) Set {
	s := Set{manifest: m, lifecycle: pluginv1connect.NewPluginServiceClient(c, baseURL, opts...)}
	if manifest.HasCapability(m, pluginv1.Capability_CAPABILITY_METADATA_PROVIDER) {
		s.metadata = pluginv1connect.NewMetadataProviderServiceClient(c, baseURL, opts...)
	}
	if manifest.HasCapability(m, pluginv1.Capability_CAPABILITY_AUTH_PROVIDER) {
		s.auth = pluginv1connect.NewAuthProviderServiceClient(c, baseURL, opts...)
	}
	if manifest.HasCapability(m, pluginv1.Capability_CAPABILITY_NOTIFIER) {
		s.notifier = pluginv1connect.NewNotifierServiceClient(c, baseURL, opts...)
	}
	return s
}

// Manifest returns the plugin's manifest.
func (s Set) Manifest() *pluginv1.Manifest { return s.manifest }

// Lifecycle returns the PluginService client.
func (s Set) Lifecycle() pluginv1connect.PluginServiceClient { return s.lifecycle }

// Metadata returns the MetadataProviderService client, or nil.
func (s Set) Metadata() pluginv1connect.MetadataProviderServiceClient { return s.metadata }

// Auth returns the AuthProviderService client, or nil.
func (s Set) Auth() pluginv1connect.AuthProviderServiceClient { return s.auth }

// Notifier returns the NotifierService client, or nil.
func (s Set) Notifier() pluginv1connect.NotifierServiceClient { return s.notifier }

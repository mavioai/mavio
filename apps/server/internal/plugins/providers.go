package plugins

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// The capabilities of plugins are looked up when they are used, so that a
// plugin installed, upgraded or configured later takes part at once; a
// plugin that is not ready knows nothing.

// MetadataProviders returns the metadata providers among the started
// plugins.
func (m *Manager) MetadataProviders() []library.Provider {
	var out []library.Provider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_METADATA_PROVIDER) {
		out = append(out, &provider{m: m, id: id})
	}
	return out
}

type provider struct {
	m  *Manager
	id string
}

func (p *provider) Name() string { return p.id }

func (p *provider) plugin() (*providers.Plugin, bool) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Metadata() == nil {
		return nil, false
	}
	return &providers.Plugin{ID: p.id, Client: pl.Metadata()}, true
}

func (p *provider) Metadata(ctx context.Context, l library.Lookup) (*metadata.Result, error) {
	pl, ok := p.plugin()
	if !ok {
		return nil, nil
	}
	return pl.Metadata(ctx, l)
}

func (p *provider) Search(ctx context.Context, l library.Lookup, limit int) ([]library.SearchResult, error) {
	pl, ok := p.plugin()
	if !ok {
		return nil, nil
	}
	return pl.Search(ctx, l, limit)
}

// SubtitleProviders returns the subtitle providers among the started
// plugins.
func (m *Manager) SubtitleProviders() []library.SubtitleProvider {
	var out []library.SubtitleProvider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_SUBTITLE_PROVIDER) {
		out = append(out, &subtitleProvider{m: m, id: id})
	}
	return out
}

type subtitleProvider struct {
	m  *Manager
	id string
}

func (p *subtitleProvider) Name() string { return p.id }

func (p *subtitleProvider) plugin() (*providers.SubtitlePlugin, bool) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Subtitles() == nil {
		return nil, false
	}
	return &providers.SubtitlePlugin{ID: p.id, Client: pl.Subtitles()}, true
}

func (p *subtitleProvider) SearchSubtitles(ctx context.Context, q library.SubtitleQuery) ([]library.RemoteSubtitle, error) {
	pl, ok := p.plugin()
	if !ok {
		return nil, nil
	}
	return pl.SearchSubtitles(ctx, q)
}

func (p *subtitleProvider) DownloadSubtitle(ctx context.Context, id string) (library.DownloadedSubtitle, error) {
	pl, ok := p.plugin()
	if !ok {
		return library.DownloadedSubtitle{}, fmt.Errorf("plugin %s is not ready: %w", p.id, core.ErrNotFound)
	}
	return pl.DownloadSubtitle(ctx, id)
}

// Notifiers returns the notification plugins.
func (m *Manager) Notifiers() []activity.Notifier {
	var out []activity.Notifier
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_NOTIFIER) {
		out = append(out, &notifier{m: m, id: id})
	}
	return out
}

type notifier struct {
	m  *Manager
	id string
}

func (n *notifier) Name() string { return n.id }

// Notify sends an activity as an event; a plugin that is not ready is
// skipped.
func (n *notifier) Notify(ctx context.Context, a core.Activity) error {
	pl, ok := n.m.running(n.id)
	if !ok || pl.Notifier() == nil {
		return nil
	}
	ev := pluginv1.Event_builder{
		Id: new(a.ID.String()), Type: &a.Type, Time: timestamppb.New(a.Time), Title: &a.Title, Message: &a.Message,
		Attributes: a.Attributes,
	}.Build()
	if !a.UserID.IsZero() {
		ev.SetUserId(a.UserID.String())
	}
	if !a.ItemID.IsZero() {
		ev.SetItemId(a.ItemID.String())
	}
	_, err := pl.Notifier().Notify(ctx, pluginv1.NotifyRequest_builder{Event: ev}.Build())
	return err
}

// AuthResult is what an authentication plugin says of a sign-in.
type AuthResult struct {
	Authenticated bool
	DisplayName   string
	Admin         bool
	// Message is why it was rejected, for logs.
	Message string
}

// Authenticate checks a user's credentials with an authentication plugin.
// It fails with core.ErrNotFound when the plugin is not ready.
func (m *Manager) Authenticate(ctx context.Context, pluginID, username, password string) (AuthResult, error) {
	pl, ok := m.running(pluginID)
	if !ok || pl.Auth() == nil {
		return AuthResult{}, fmt.Errorf("authentication plugin %s: %w", pluginID, core.ErrNotFound)
	}
	resp, err := pl.Auth().Authenticate(ctx, pluginv1.AuthenticateRequest_builder{Username: &username, Password: &password}.Build())
	if err != nil {
		return AuthResult{}, fmt.Errorf("authentication plugin %s: %w", pluginID, err)
	}
	return AuthResult{
		Authenticated: resp.GetAuthenticated(), DisplayName: resp.GetDisplayName(), Admin: resp.GetAdmin(), Message: resp.GetMessage(),
	}, nil
}

package plugins

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	"github.com/mavioai/mavio/libs/plugin/manifest"
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

// ExternalIDKinds returns the built-in kinds of external IDs followed by
// those the started plugins declare.
func (m *Manager) ExternalIDKinds() []metadata.ExternalIDKind {
	m.mu.Lock()
	var declared []metadata.ExternalIDKind
	for _, e := range m.plugins {
		if e.plugin == nil {
			continue
		}
		for _, k := range e.manifest.GetExternalIdKinds() {
			kind := metadata.ExternalIDKind{Provider: core.Provider(k.GetKey()), Name: k.GetName(), Plugin: e.manifest.GetId()}
			for _, mk := range k.GetMediaKinds() {
				if mk == pluginv1.MediaKind_MEDIA_KIND_PERSON {
					kind.Persons, kind.PersonURL = true, k.GetUrlTemplate()
				} else if ik, ok := providers.ItemKind(mk); ok {
					if kind.Items == nil {
						kind.Items = map[core.ItemKind]string{}
					}
					kind.Items[ik] = k.GetUrlTemplate()
				}
			}
			declared = append(declared, kind)
		}
	}
	m.mu.Unlock()
	return metadata.MergeExternalIDKinds(metadata.BuiltinExternalIDKinds(), declared...)
}

// LocalReaders returns the local metadata readers among the started
// plugins.
func (m *Manager) LocalReaders() []library.LocalReader {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []library.LocalReader
	for _, e := range m.plugins {
		if e.plugin != nil && manifest.HasCapability(e.manifest, pluginv1.Capability_CAPABILITY_LOCAL_METADATA) {
			out = append(out, &localReader{m: m, id: e.manifest.GetId(), files: e.manifest.GetLocalMetadataFiles()})
		}
	}
	return out
}

type localReader struct {
	m     *Manager
	id    string
	files []string
}

func (p *localReader) Name() string { return p.id }

func (p *localReader) Patterns() []string { return p.files }

func (p *localReader) ReadLocal(ctx context.Context, l library.Lookup, media string, files []library.LocalFile) (*metadata.Result, error) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.LocalMetadata() == nil {
		return nil, nil
	}
	return (&providers.LocalPlugin{ID: p.id, Files: p.files, Client: pl.LocalMetadata()}).ReadLocal(ctx, l, media, files)
}

// Savers returns the metadata savers among the started plugins.
func (m *Manager) Savers() []library.Saver {
	var out []library.Saver
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_METADATA_SAVER) {
		out = append(out, &saver{m: m, id: id})
	}
	return out
}

type saver struct {
	m  *Manager
	id string
}

func (p *saver) Name() string { return p.id }

func (p *saver) Save(ctx context.Context, media string, res *metadata.Result) ([]library.LocalFile, error) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Saver() == nil {
		return nil, nil
	}
	return (&providers.SaverPlugin{ID: p.id, Client: pl.Saver()}).Save(ctx, media, res)
}

// Processors returns the metadata processors among the started plugins.
func (m *Manager) Processors() []library.Processor {
	var out []library.Processor
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_METADATA_PROCESSOR) {
		out = append(out, &processor{m: m, id: id})
	}
	return out
}

type processor struct {
	m  *Manager
	id string
}

func (p *processor) Name() string { return p.id }

func (p *processor) Process(ctx context.Context, res *metadata.Result) (*metadata.Result, error) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Processor() == nil {
		return nil, nil
	}
	return (&providers.ProcessorPlugin{ID: p.id, Client: pl.Processor()}).Process(ctx, res)
}

// LyricsProviders returns the lyrics providers among the started plugins.
func (m *Manager) LyricsProviders() []library.LyricsProvider {
	var out []library.LyricsProvider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_LYRICS_PROVIDER) {
		out = append(out, &lyricsProvider{m: m, id: id})
	}
	return out
}

type lyricsProvider struct {
	m  *Manager
	id string
}

func (p *lyricsProvider) Name() string { return p.id }

func (p *lyricsProvider) plugin() (*providers.LyricsPlugin, bool) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Lyrics() == nil {
		return nil, false
	}
	return &providers.LyricsPlugin{ID: p.id, Client: pl.Lyrics()}, true
}

func (p *lyricsProvider) SearchLyrics(ctx context.Context, q library.LyricsQuery) ([]library.RemoteLyrics, error) {
	pl, ok := p.plugin()
	if !ok {
		return nil, nil
	}
	return pl.SearchLyrics(ctx, q)
}

func (p *lyricsProvider) DownloadLyrics(ctx context.Context, id string) (string, bool, error) {
	pl, ok := p.plugin()
	if !ok {
		return "", false, fmt.Errorf("plugin %s is not ready", p.id)
	}
	return pl.DownloadLyrics(ctx, id)
}

// ImageProviders returns the image providers among the started plugins.
func (m *Manager) ImageProviders() []library.ImageProvider {
	var out []library.ImageProvider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_IMAGE_PROVIDER) {
		out = append(out, &imageProvider{m: m, id: id})
	}
	return out
}

type imageProvider struct {
	m  *Manager
	id string
}

func (p *imageProvider) Name() string { return p.id }

func (p *imageProvider) Images(ctx context.Context, l library.Lookup) ([]metadata.RemoteImage, error) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Images() == nil {
		return nil, nil
	}
	return (&providers.ImagePlugin{ID: p.id, Client: pl.Images()}).Images(ctx, l)
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

// SegmentProviders returns the media segment providers among the started
// plugins.
func (m *Manager) SegmentProviders() []library.SegmentProvider {
	var out []library.SegmentProvider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_SEGMENT_PROVIDER) {
		out = append(out, &segmentProvider{m: m, id: id})
	}
	return out
}

type segmentProvider struct {
	m  *Manager
	id string
}

func (p *segmentProvider) Name() string { return p.id }

func (p *segmentProvider) Segments(ctx context.Context, q library.SegmentQuery) ([]core.MediaSegment, error) {
	pl, ok := p.m.running(p.id)
	if !ok || pl.Segments() == nil {
		return nil, nil
	}
	return (&providers.SegmentPlugin{ID: p.id, Client: pl.Segments()}).Segments(ctx, q)
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
	_, err := pl.Notifier().Notify(ctx, pluginv1.NotifyRequest_builder{Event: eventToProto(&a)}.Build())
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

// Resolvers returns the resolvers among the started plugins.
func (m *Manager) Resolvers() []library.FolderResolver {
	var out []library.FolderResolver
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_RESOLVER) {
		if pl, ok := m.running(id); ok && pl.Resolver() != nil {
			out = append(out, &providers.ResolverPlugin{ID: id, Client: pl.Resolver()})
		}
	}
	return out
}

// IntroProviders returns the intro providers among the started plugins.
func (m *Manager) IntroProviders() []library.IntroProvider {
	var out []library.IntroProvider
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_INTRO_PROVIDER) {
		if pl, ok := m.running(id); ok && pl.Intros() != nil {
			out = append(out, &providers.IntroPlugin{ID: id, Client: pl.Intros()})
		}
	}
	return out
}

// ImageGenerators returns the image generators among the started plugins.
func (m *Manager) ImageGenerators() []library.ImageGenerator {
	var out []library.ImageGenerator
	for _, id := range m.withCapability(pluginv1.Capability_CAPABILITY_IMAGE_GENERATOR) {
		if pl, ok := m.running(id); ok && pl.ImageGenerator() != nil {
			out = append(out, &providers.GeneratorPlugin{ID: id, Client: pl.ImageGenerator()})
		}
	}
	return out
}

// StartPasswordReset has a password reset plugin deliver a PIN to a user.
func (m *Manager) StartPasswordReset(ctx context.Context, pluginID string, user core.User, pin string, expires time.Time) error {
	pl, ok := m.running(pluginID)
	if !ok || pl.PasswordReset() == nil {
		return fmt.Errorf("password reset plugin %s: %w", pluginID, core.ErrNotFound)
	}
	_, err := pl.PasswordReset().StartReset(ctx, pluginv1.StartResetRequest_builder{
		UserId: proto.String(user.ID.String()), UserName: proto.String(user.Name), Pin: proto.String(pin), Expires: timestamppb.New(expires),
	}.Build())
	if err != nil {
		return fmt.Errorf("password reset plugin %s: %w", pluginID, err)
	}
	return nil
}

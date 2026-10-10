package providers

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// MediaSourcePlugin is a media source provider plugin as a
// library.MediaSourceProvider.
type MediaSourcePlugin struct {
	ID     string
	Client pluginv1connect.MediaSourceProviderServiceClient
}

// Name returns the plugin's ID.
func (p *MediaSourcePlugin) Name() string { return p.ID }

// MediaSources asks the plugin for an item's sources read over HTTP.
func (p *MediaSourcePlugin) MediaSources(ctx context.Context, it core.Item) ([]library.RemoteSource, error) {
	req := pluginv1.GetMediaSourcesRequest_builder{
		ItemId: proto.String(it.ID.String()), Name: set(it.Name), ExternalIds: ids(it.ExternalIDs),
	}
	if kind, ok := mediaKinds[it.Kind]; ok {
		req.Kind = kind.Enum()
	}
	resp, err := p.Client.GetMediaSources(ctx, req.Build())
	if err != nil {
		return nil, fmt.Errorf("get media sources: %w", err)
	}
	out := make([]library.RemoteSource, 0, len(resp.GetSources()))
	for _, s := range resp.GetSources() {
		out = append(out, library.RemoteSource{ID: s.GetId(), Name: s.GetName(), URL: s.GetUrl()})
	}
	return out, nil
}

var _ library.MediaSourceProvider = (*MediaSourcePlugin)(nil)

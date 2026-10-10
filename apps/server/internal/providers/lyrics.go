package providers

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// LyricsPlugin is a lyrics provider plugin as a library.LyricsProvider.
type LyricsPlugin struct {
	ID     string
	Client pluginv1connect.LyricsProviderServiceClient
}

// Name returns the plugin's ID.
func (p *LyricsPlugin) Name() string { return p.ID }

// SearchLyrics asks the plugin for lyrics of a track.
func (p *LyricsPlugin) SearchLyrics(ctx context.Context, q library.LyricsQuery) ([]library.RemoteLyrics, error) {
	req := pluginv1.SearchLyricsRequest_builder{
		Name: set(q.Name), Artists: q.Artists, Album: set(q.Album), ExternalIds: ids(q.ExternalIDs),
	}
	if q.Duration > 0 {
		req.Duration = durationpb.New(q.Duration)
	}
	resp, err := p.Client.SearchLyrics(ctx, req.Build())
	if err != nil {
		return nil, fmt.Errorf("search lyrics: %w", err)
	}
	var out []library.RemoteLyrics
	for _, l := range resp.GetLyrics() {
		if l.GetId() == "" {
			continue
		}
		out = append(out, library.RemoteLyrics{
			ID: l.GetId(), Name: l.GetName(), Artists: l.GetArtists(), Album: l.GetAlbum(),
			Duration: l.GetDuration().AsDuration(), Synced: l.GetSynced(), Score: l.GetScore(),
		})
	}
	return out, nil
}

// DownloadLyrics asks the plugin for lyrics it found.
func (p *LyricsPlugin) DownloadLyrics(ctx context.Context, id string) (string, bool, error) {
	resp, err := p.Client.DownloadLyrics(ctx, pluginv1.DownloadLyricsRequest_builder{Id: &id}.Build())
	if err != nil {
		return "", false, fmt.Errorf("download lyrics: %w", err)
	}
	return resp.GetContent(), resp.GetSynced(), nil
}

var _ library.LyricsProvider = (*LyricsPlugin)(nil)

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

// SubtitlePlugin is a subtitle provider plugin as a
// library.SubtitleProvider.
type SubtitlePlugin struct {
	// ID names the plugin, such as "org.mavio.subtitles-opensubtitles".
	ID     string
	Client pluginv1connect.SubtitleProviderServiceClient
}

// Name returns the plugin's ID.
func (p *SubtitlePlugin) Name() string { return p.ID }

// subtitleKinds are the kinds of items subtitles are searched for.
var subtitleKinds = map[core.ItemKind]pluginv1.MediaKind{
	core.KindMovie:      pluginv1.MediaKind_MEDIA_KIND_MOVIE,
	core.KindEpisode:    pluginv1.MediaKind_MEDIA_KIND_EPISODE,
	core.KindMusicVideo: pluginv1.MediaKind_MEDIA_KIND_MUSIC_VIDEO,
	core.KindVideo:      pluginv1.MediaKind_MEDIA_KIND_MOVIE,
}

// SearchSubtitles asks the plugin for subtitles of a video.
func (p *SubtitlePlugin) SearchSubtitles(ctx context.Context, q library.SubtitleQuery) ([]library.RemoteSubtitle, error) {
	kind, ok := subtitleKinds[q.Kind]
	if !ok {
		return nil, nil
	}
	query := pluginv1.SubtitleQuery_builder{
		Kind: kind.Enum(), Name: proto.String(q.Name), Year: proto.Int32(int32(q.Year)),
		ExternalIds: ids(q.ExternalIDs), SeriesName: proto.String(q.SeriesName), SeriesExternalIds: ids(q.SeriesExternalIDs),
		Language: proto.String(q.Language), FileName: proto.String(q.FileName), FileSize: proto.Int64(q.FileSize),
		FileHash: proto.String(q.FileHash), HearingImpaired: proto.Bool(q.HearingImpaired), Forced: proto.Bool(q.Forced),
	}.Build()
	if q.SeasonNumber != nil {
		query.SetSeasonNumber(int32(*q.SeasonNumber))
	}
	if q.EpisodeNumber != nil {
		query.SetEpisodeNumber(int32(*q.EpisodeNumber))
	}
	resp, err := p.Client.SearchSubtitles(ctx, pluginv1.SearchSubtitlesRequest_builder{Query: query, Limit: proto.Int32(50)}.Build())
	if err != nil {
		return nil, fmt.Errorf("search subtitles: %w", err)
	}
	out := make([]library.RemoteSubtitle, 0, len(resp.GetSubtitles()))
	for _, s := range resp.GetSubtitles() {
		out = append(out, library.RemoteSubtitle{
			Provider: p.ID, ID: s.GetId(), Name: s.GetName(), Language: s.GetLanguage(), Format: s.GetFormat(),
			HearingImpaired: s.GetHearingImpaired(), Forced: s.GetForced(), Score: s.GetScore(),
			DownloadCount: int(s.GetDownloadCount()), HashMatch: s.GetHashMatch(),
		})
	}
	return out, nil
}

// DownloadSubtitle downloads a subtitle the plugin found.
func (p *SubtitlePlugin) DownloadSubtitle(ctx context.Context, id string) (library.DownloadedSubtitle, error) {
	resp, err := p.Client.DownloadSubtitle(ctx, pluginv1.DownloadSubtitleRequest_builder{Id: proto.String(id)}.Build())
	if err != nil {
		return library.DownloadedSubtitle{}, fmt.Errorf("download subtitle: %w", err)
	}
	return library.DownloadedSubtitle{
		Data: resp.GetData(), Format: resp.GetFormat(), Language: resp.GetLanguage(),
		HearingImpaired: resp.GetHearingImpaired(), Forced: resp.GetForced(),
	}, nil
}

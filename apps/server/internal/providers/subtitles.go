package providers

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// SubtitlePlugin is a subtitle provider plugin as a
// library.SubtitleProvider. The IDs of its subtitles are
// "<provider>/<subtitle>", naming the plugin's provider.
type SubtitlePlugin struct {
	// ID names the plugin, such as "org.mavio.subtitles".
	ID     string
	Client pluginv1connect.SubtitleProviderServiceClient
}

// Name returns the plugin's ID.
func (p *SubtitlePlugin) Name() string { return p.ID }

// subtitleKinds are the kinds of items subtitles are searched for.
var subtitleKinds = map[core.ItemKind]pluginv1.MediaKind{
	core.KindMovie:      pluginv1.MediaKind_MEDIA_KIND_MOVIE,
	core.KindEpisode:    pluginv1.MediaKind_MEDIA_KIND_EPISODE,
	core.KindMusicVideo: pluginv1.MediaKind_MEDIA_KIND_MOVIE,
	core.KindVideo:      pluginv1.MediaKind_MEDIA_KIND_MOVIE,
}

// SearchSubtitles asks the plugin's providers for subtitles of a video,
// best first by Bazarr's scores.
func (p *SubtitlePlugin) SearchSubtitles(ctx context.Context, q library.SubtitleQuery) ([]library.RemoteSubtitle, error) {
	kind, ok := subtitleKinds[q.Kind]
	if !ok {
		return nil, nil
	}
	v := &subtitles.Video{Episode: kind == pluginv1.MediaKind_MEDIA_KIND_EPISODE, Year: q.Year}
	video := pluginv1.SubtitleVideo_builder{
		Kind: kind.Enum(), Title: proto.String(q.Name), Year: proto.Int32(int32(q.Year)), ExternalIds: ids(q.ExternalIDs),
		FileName: proto.String(q.FileName), FileSize: proto.Int64(q.FileSize), FileHash: proto.String(q.FileHash),
	}.Build()
	if v.Episode {
		video.SetSeries(q.SeriesName)
		video.SetSeriesExternalIds(ids(q.SeriesExternalIDs))
		v.Series, v.Title = q.SeriesName, q.Name
		if q.SeasonNumber != nil {
			video.SetSeason(int32(*q.SeasonNumber))
			v.Season = *q.SeasonNumber
		}
		if q.EpisodeNumber != nil {
			video.SetEpisode(int32(*q.EpisodeNumber))
			v.EpisodeNumber = *q.EpisodeNumber
		}
	} else {
		v.Title = q.Name
	}
	lang := pluginv1.SubtitleLanguage_builder{
		Language: proto.String(q.Language), Forced: proto.Bool(q.Forced), HearingImpaired: proto.Bool(q.HearingImpaired),
	}.Build()
	resp, err := p.Client.SearchSubtitles(ctx, pluginv1.SearchSubtitlesRequest_builder{
		Video: video, Languages: []*pluginv1.SubtitleLanguage{lang},
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("search subtitles: %w", err)
	}
	maxScore := float64(subtitles.MaxScore(v.Episode))
	out := make([]library.RemoteSubtitle, 0, len(resp.GetSubtitles()))
	for _, s := range resp.GetSubtitles() {
		l := s.GetLanguage()
		m := subtitles.CandidateMatches(v, subtitles.NewMatches(s.GetMatches()...), s.GetReleaseNames(), false)
		hi := q.HearingImpaired
		score, _, _ := subtitles.Score(m, subtitles.ScoreSubject{
			HashVerifiable: s.GetHashVerifiable(), HearingImpaired: l.GetHearingImpaired(),
		}, v, &hi)
		out = append(out, library.RemoteSubtitle{
			Provider: p.ID, ID: s.GetProvider() + "/" + s.GetId(), Name: strings.Join(s.GetReleaseNames(), "\n"),
			Language: l.GetLanguage(), Format: s.GetFormat(), HearingImpaired: l.GetHearingImpaired(), Forced: l.GetForced(),
			Score: min(float64(score)/maxScore, 1), DownloadCount: int(s.GetDownloadCount()), HashMatch: m[subtitles.MatchHash],
		})
	}
	slices.SortStableFunc(out, func(a, b library.RemoteSubtitle) int { return cmp.Compare(b.Score, a.Score) })
	return out, nil
}

// DownloadSubtitle downloads a subtitle the plugin found.
func (p *SubtitlePlugin) DownloadSubtitle(ctx context.Context, id string) (library.DownloadedSubtitle, error) {
	provider, subID, ok := strings.Cut(id, "/")
	if !ok {
		return library.DownloadedSubtitle{}, fmt.Errorf("%w: subtitle ID %q names no provider", core.ErrInvalid, id)
	}
	resp, err := p.Client.DownloadSubtitle(ctx, pluginv1.DownloadSubtitleRequest_builder{
		Provider: proto.String(provider), Id: proto.String(subID),
	}.Build())
	if err != nil {
		return library.DownloadedSubtitle{}, fmt.Errorf("download subtitle: %w", err)
	}
	if f := resp.GetFailure(); f != nil {
		return library.DownloadedSubtitle{}, fmt.Errorf("download subtitle from %s: %s", provider, f.GetMessage())
	}
	l := resp.GetLanguage()
	return library.DownloadedSubtitle{
		Data: resp.GetData(), Format: cmp.Or(resp.GetFormat(), "srt"), Language: l.GetLanguage(),
		HearingImpaired: l.GetHearingImpaired(), Forced: l.GetForced(),
	}, nil
}

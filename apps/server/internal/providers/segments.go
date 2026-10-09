package providers

import (
	"context"
	"fmt"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// SegmentPlugin is a segment provider plugin as a library.SegmentProvider.
type SegmentPlugin struct {
	ID     string
	Client pluginv1connect.MediaSegmentProviderServiceClient
}

// Name returns the plugin's ID.
func (p *SegmentPlugin) Name() string { return p.ID }

var segmentKinds = map[pluginv1.SegmentKind]core.SegmentKind{
	pluginv1.SegmentKind_SEGMENT_KIND_INTRO:      core.SegmentIntro,
	pluginv1.SegmentKind_SEGMENT_KIND_OUTRO:      core.SegmentOutro,
	pluginv1.SegmentKind_SEGMENT_KIND_RECAP:      core.SegmentRecap,
	pluginv1.SegmentKind_SEGMENT_KIND_PREVIEW:    core.SegmentPreview,
	pluginv1.SegmentKind_SEGMENT_KIND_COMMERCIAL: core.SegmentCommercial,
}

// Segments asks the plugin for a video's segments; those of unknown kinds
// are left out.
func (p *SegmentPlugin) Segments(ctx context.Context, q library.SegmentQuery) ([]core.MediaSegment, error) {
	kind, ok := mediaKinds[q.Kind]
	if !ok {
		return nil, nil
	}
	req := pluginv1.GetSegmentsRequest_builder{
		Kind: kind.Enum(), Name: proto.String(q.Name), ExternalIds: ids(q.ExternalIDs), SeriesExternalIds: ids(q.SeriesExternalIDs),
		FileName: proto.String(filepath.Base(q.Path)), Duration: durationpb.New(q.Duration),
	}.Build()
	if q.SeasonNumber != nil {
		req.SetSeasonNumber(int32(*q.SeasonNumber))
	}
	if q.EpisodeNumber != nil {
		req.SetEpisodeNumber(int32(*q.EpisodeNumber))
	}
	for _, c := range q.Chapters {
		req.SetChapters(append(req.GetChapters(), pluginv1.SegmentChapter_builder{Start: durationpb.New(c.Start), Title: proto.String(c.Title)}.Build()))
	}
	resp, err := p.Client.GetSegments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get segments: %w", err)
	}
	var out []core.MediaSegment
	for _, s := range resp.GetSegments() {
		if k, ok := segmentKinds[s.GetKind()]; ok {
			out = append(out, core.MediaSegment{Kind: k, Start: s.GetStart().AsDuration(), End: s.GetEnd().AsDuration()})
		}
	}
	return out, nil
}

package rpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/decision"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
)

var (
	playMethods = map[decision.Method]playbackv1.PlayMethod{
		decision.Transcode:    playbackv1.PlayMethod_PLAY_METHOD_TRANSCODE,
		decision.DirectStream: playbackv1.PlayMethod_PLAY_METHOD_DIRECT_STREAM,
		decision.DirectPlay:   playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY,
	}
	subtitleMethodsToProto = map[decision.SubtitleMethod]playbackv1.SubtitleMethod{
		decision.SubtitleEncode:   playbackv1.SubtitleMethod_SUBTITLE_METHOD_ENCODE,
		decision.SubtitleEmbed:    playbackv1.SubtitleMethod_SUBTITLE_METHOD_EMBED,
		decision.SubtitleExternal: playbackv1.SubtitleMethod_SUBTITLE_METHOD_EXTERNAL,
		decision.SubtitleHLS:      playbackv1.SubtitleMethod_SUBTITLE_METHOD_HLS,
		decision.SubtitleDrop:     playbackv1.SubtitleMethod_SUBTITLE_METHOD_DROP,
	}
)

// PlaybackService implements mavio.playback.v1.PlaybackService.
type PlaybackService struct {
	playbacks *playback.Manager
	store     core.Store
	// Intros finds the items played before items; nil finds none.
	Intros *library.Intros
}

var _ playbackv1connect.PlaybackServiceHandler = (*PlaybackService)(nil)

// NewPlaybackService returns a PlaybackService running playbacks with m
// on the items of store.
func NewPlaybackService(m *playback.Manager, store core.Store) *PlaybackService {
	return &PlaybackService{playbacks: m, store: store}
}

// ListIntros returns the items to play before an item.
func (s *PlaybackService) ListIntros(ctx context.Context, req *playbackv1.ListIntrosRequest) (*playbackv1.ListIntrosResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	in := s.Intros
	if in == nil {
		in = &library.Intros{Store: s.store}
	}
	items, err := in.List(ctx, &p.User, core.MustParseID(req.GetItemId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, items, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return playbackv1.ListIntrosResponse_builder{Items: out}.Build(), nil
}

// StartPlayback decides how the client plays the item and starts the
// playback.
func (s *PlaybackService) StartPlayback(ctx context.Context, req *playbackv1.StartPlaybackRequest) (*playbackv1.StartPlaybackResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	r := playback.Request{
		User: p.User, SessionID: p.Session.ID,
		Client:     capabilitiesFromProto(req.GetCapabilities()),
		MaxBitrate: req.GetMaxBitrate(), Download: req.GetDownload(),
	}
	// The request was validated, so the IDs parse.
	r.ItemID = core.MustParseID(req.GetItemId())
	if req.HasMediaSourceId() {
		r.SourceID = core.MustParseID(req.GetMediaSourceId())
	}
	if req.HasAudioStreamIndex() {
		r.AudioStream = new(int(req.GetAudioStreamIndex()))
	}
	if req.HasSubtitleStreamIndex() {
		r.SubtitleStream = new(int(req.GetSubtitleStreamIndex()))
	}
	if req.HasStartPosition() {
		r.Start = req.GetStartPosition().AsDuration()
	}
	pb, err := s.playbacks.Start(ctx, r)
	if err != nil {
		return nil, playbackError(ctx, err)
	}

	subs := make([]*playbackv1.SubtitleTrack, len(pb.Subtitles))
	for i := range pb.Subtitles {
		sub := &pb.Subtitles[i]
		st := sub.Stream
		subs[i] = playbackv1.SubtitleTrack_builder{
			StreamIndex: new(int32(st.Index)),
			Language:    &st.Language,
			Title:       &st.Title,
			Default:     &st.Default,
			Forced:      &st.Forced,
			Method:      new(subtitleMethodsToProto[sub.Method]),
			Format:      &sub.Format,
			Url:         new(pb.SubtitleURL(sub)),
		}.Build()
	}
	var attachments []*playbackv1.Attachment
	for _, a := range pb.Attachments() {
		attachments = append(attachments, playbackv1.Attachment_builder{
			Index: new(int32(a.Index)), FileName: &a.Title, MimeType: &a.MimeType, Url: new(pb.AttachmentURL(a.Index)),
		}.Build())
	}
	reasons := reasonsToProto(pb.Decision.Reasons)
	var start *durationpb.Duration
	if pb.StartPosition > 0 {
		start = durationpb.New(pb.StartPosition)
	}
	return playbackv1.StartPlaybackResponse_builder{
		PlaybackId:          &pb.ID,
		MediaSourceId:       new(pb.Source().ID.String()),
		Method:              new(playMethods[pb.Method]),
		TranscodeReasons:    reasons,
		Url:                 new(pb.URL()),
		AudioStreamIndex:    new(int32(pb.AudioStream)),
		SubtitleStreamIndex: new(int32(pb.SubtitleStream)),
		Subtitles:           subs,
		Attachments:         attachments,
		StartPosition:       start,
	}.Build(), nil
}

// ReportProgress records a playback's position.
func (s *PlaybackService) ReportProgress(ctx context.Context, req *playbackv1.ReportProgressRequest) (*playbackv1.ReportProgressResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.playbacks.Progress(ctx, p.User.ID, req.GetPlaybackId(), req.GetPosition().AsDuration(), req.GetPaused()); err != nil {
		return nil, playbackError(ctx, err)
	}
	return &playbackv1.ReportProgressResponse{}, nil
}

// StopPlayback ends a playback.
func (s *PlaybackService) StopPlayback(ctx context.Context, req *playbackv1.StopPlaybackRequest) (*playbackv1.StopPlaybackResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var pos *time.Duration
	if req.HasPosition() {
		pos = new(req.GetPosition().AsDuration())
	}
	if err := s.playbacks.Stop(ctx, p.User.ID, req.GetPlaybackId(), pos); err != nil {
		return nil, playbackError(ctx, err)
	}
	return &playbackv1.StopPlaybackResponse{}, nil
}

func playbackError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, playback.ErrNotPlayable), errors.Is(err, playback.ErrNoSource):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, playback.ErrNotAllowed):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, playback.ErrUnsupported):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, playback.ErrTooManyPlaybacks):
		return connect.NewError(connect.CodeResourceExhausted, err)
	}
	return connectError(ctx, err)
}

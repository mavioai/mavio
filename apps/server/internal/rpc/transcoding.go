package rpc

import (
	"context"

	"github.com/mavioai/mavio/libs/core"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
)

var hardwareAccelerations = map[core.HardwareAcceleration]playbackv1.HardwareAcceleration{
	core.HardwareAuto:         playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_AUTO,
	core.HardwareNone:         playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_NONE,
	core.HardwareVideoToolbox: playbackv1.HardwareAcceleration_HARDWARE_ACCELERATION_VIDEOTOOLBOX,
}

func transcodingToProto(t *core.TranscodingSettings) *playbackv1.TranscodingSettings {
	return playbackv1.TranscodingSettings_builder{
		HardwareAcceleration: new(hardwareAccelerations[t.HardwareAcceleration]), HardwareEncoding: &t.HardwareEncoding,
		EncoderPreset: &t.EncoderPreset, H264Crf: new(int32(t.H264CRF)), H265Crf: new(int32(t.H265CRF)), Threads: new(int32(t.Threads)),
		TonemapAlgorithm: &t.TonemapAlgorithm, TonemapRange: &t.TonemapRange, TonemapDesat: &t.TonemapDesat, TonemapPeak: &t.TonemapPeak,
		DeinterlaceMethod: &t.DeinterlaceMethod, DeinterlaceDoubleRate: &t.DeinterlaceDoubleRate, DownmixBoost: &t.DownmixBoost,
		CropBlackBorders: &t.CropBlackBorders, TranscodeDir: &t.TranscodeDir,
	}.Build()
}

func transcodingFromProto(t *playbackv1.TranscodingSettings) core.TranscodingSettings {
	return core.TranscodingSettings{
		HardwareAcceleration: keyOf(hardwareAccelerations, t.GetHardwareAcceleration()), HardwareEncoding: t.GetHardwareEncoding(),
		EncoderPreset: t.GetEncoderPreset(), H264CRF: int(t.GetH264Crf()), H265CRF: int(t.GetH265Crf()), Threads: int(t.GetThreads()),
		TonemapAlgorithm: t.GetTonemapAlgorithm(), TonemapRange: t.GetTonemapRange(), TonemapDesat: t.GetTonemapDesat(),
		TonemapPeak: t.GetTonemapPeak(), DeinterlaceMethod: t.GetDeinterlaceMethod(), DeinterlaceDoubleRate: t.GetDeinterlaceDoubleRate(),
		DownmixBoost: t.GetDownmixBoost(), CropBlackBorders: t.GetCropBlackBorders(), TranscodeDir: t.GetTranscodeDir(),
	}
}

// GetTranscodingSettings returns the transcoding settings and the
// accelerations available.
func (s *PlaybackService) GetTranscodingSettings(ctx context.Context, _ *playbackv1.GetTranscodingSettingsRequest) (*playbackv1.GetTranscodingSettingsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	set := currentSettings(s.Settings)
	available := []core.HardwareAcceleration{core.HardwareAuto, core.HardwareNone}
	if s.Accelerations != nil {
		available = s.Accelerations()
	}
	out := make([]playbackv1.HardwareAcceleration, len(available))
	for i, a := range available {
		out[i] = hardwareAccelerations[a]
	}
	return playbackv1.GetTranscodingSettingsResponse_builder{
		Settings: transcodingToProto(&set.Transcoding), AvailableHardwareAccelerations: out,
	}.Build(), nil
}

// UpdateTranscodingSettings applies and stores the transcoding settings.
func (s *PlaybackService) UpdateTranscodingSettings(ctx context.Context, req *playbackv1.UpdateTranscodingSettingsRequest) (*playbackv1.UpdateTranscodingSettingsResponse, error) {
	set, err := changeSettings(ctx, s.Settings, s.Activity, "the transcoding settings", func(cur *core.ServerSettings) error {
		cur.Transcoding = transcodingFromProto(req.GetSettings())
		return nil
	})
	if err != nil {
		return nil, err
	}
	return playbackv1.UpdateTranscodingSettingsResponse_builder{Settings: transcodingToProto(&set.Transcoding)}.Build(), nil
}

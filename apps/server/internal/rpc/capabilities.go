package rpc

import (
	"math/bits"

	"github.com/mavioai/mavio/libs/media/decision"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
)

var (
	mediaKinds = map[playbackv1.MediaKind]decision.MediaKind{
		playbackv1.MediaKind_MEDIA_KIND_AUDIO: decision.Audio,
		playbackv1.MediaKind_MEDIA_KIND_VIDEO: decision.Video,
		playbackv1.MediaKind_MEDIA_KIND_PHOTO: decision.Photo,
	}
	contexts = map[playbackv1.Context]decision.Context{
		playbackv1.Context_CONTEXT_UNSPECIFIED: decision.Streaming,
		playbackv1.Context_CONTEXT_STREAMING:   decision.Streaming,
		playbackv1.Context_CONTEXT_STATIC:      decision.Static,
	}
	protocols = map[playbackv1.Protocol]decision.Protocol{
		playbackv1.Protocol_PROTOCOL_UNSPECIFIED: decision.HTTP,
		playbackv1.Protocol_PROTOCOL_HTTP:        decision.HTTP,
		playbackv1.Protocol_PROTOCOL_HLS:         decision.HLS,
	}
	seekInfos = map[playbackv1.SeekInfo]decision.SeekInfo{
		playbackv1.SeekInfo_SEEK_INFO_UNSPECIFIED: decision.SeekAuto,
		playbackv1.SeekInfo_SEEK_INFO_AUTO:        decision.SeekAuto,
		playbackv1.SeekInfo_SEEK_INFO_BYTES:       decision.SeekBytes,
	}
	subtitleMethods = map[playbackv1.SubtitleMethod]decision.SubtitleMethod{
		playbackv1.SubtitleMethod_SUBTITLE_METHOD_ENCODE:   decision.SubtitleEncode,
		playbackv1.SubtitleMethod_SUBTITLE_METHOD_EMBED:    decision.SubtitleEmbed,
		playbackv1.SubtitleMethod_SUBTITLE_METHOD_EXTERNAL: decision.SubtitleExternal,
		playbackv1.SubtitleMethod_SUBTITLE_METHOD_HLS:      decision.SubtitleHLS,
		playbackv1.SubtitleMethod_SUBTITLE_METHOD_DROP:     decision.SubtitleDrop,
	}
	codecKinds = map[playbackv1.CodecKind]decision.CodecKind{
		playbackv1.CodecKind_CODEC_KIND_VIDEO:       decision.CodecVideo,
		playbackv1.CodecKind_CODEC_KIND_VIDEO_AUDIO: decision.CodecVideoAudio,
		playbackv1.CodecKind_CODEC_KIND_AUDIO:       decision.CodecAudio,
	}
	properties = map[playbackv1.Property]decision.Property{
		playbackv1.Property_PROPERTY_AUDIO_CHANNELS:     decision.AudioChannels,
		playbackv1.Property_PROPERTY_AUDIO_BITRATE:      decision.AudioBitrate,
		playbackv1.Property_PROPERTY_AUDIO_PROFILE:      decision.AudioProfile,
		playbackv1.Property_PROPERTY_AUDIO_SAMPLE_RATE:  decision.AudioSampleRate,
		playbackv1.Property_PROPERTY_AUDIO_BIT_DEPTH:    decision.AudioBitDepth,
		playbackv1.Property_PROPERTY_IS_SECONDARY_AUDIO: decision.IsSecondaryAudio,
		playbackv1.Property_PROPERTY_WIDTH:              decision.Width,
		playbackv1.Property_PROPERTY_HEIGHT:             decision.Height,
		playbackv1.Property_PROPERTY_VIDEO_BIT_DEPTH:    decision.VideoBitDepth,
		playbackv1.Property_PROPERTY_VIDEO_BITRATE:      decision.VideoBitrate,
		playbackv1.Property_PROPERTY_VIDEO_FRAMERATE:    decision.VideoFramerate,
		playbackv1.Property_PROPERTY_VIDEO_LEVEL:        decision.VideoLevel,
		playbackv1.Property_PROPERTY_VIDEO_PROFILE:      decision.VideoProfile,
		playbackv1.Property_PROPERTY_VIDEO_RANGE_TYPE:   decision.VideoRangeType,
		playbackv1.Property_PROPERTY_VIDEO_CODEC_TAG:    decision.VideoCodecTag,
		playbackv1.Property_PROPERTY_VIDEO_ROTATION:     decision.VideoRotation,
		playbackv1.Property_PROPERTY_IS_ANAMORPHIC:      decision.IsAnamorphic,
		playbackv1.Property_PROPERTY_IS_INTERLACED:      decision.IsInterlaced,
		playbackv1.Property_PROPERTY_IS_AVC:             decision.IsAVC,
		playbackv1.Property_PROPERTY_REF_FRAMES:         decision.RefFrames,
		playbackv1.Property_PROPERTY_NUM_STREAMS:        decision.NumStreams,
		playbackv1.Property_PROPERTY_NUM_AUDIO_STREAMS:  decision.NumAudioStreams,
		playbackv1.Property_PROPERTY_NUM_VIDEO_STREAMS:  decision.NumVideoStreams,
	}
	ops = map[playbackv1.Op]decision.Op{
		playbackv1.Op_OP_EQUALS:             decision.Equals,
		playbackv1.Op_OP_NOT_EQUALS:         decision.NotEquals,
		playbackv1.Op_OP_LESS_THAN_EQUAL:    decision.LessThanEqual,
		playbackv1.Op_OP_GREATER_THAN_EQUAL: decision.GreaterThanEqual,
		playbackv1.Op_OP_EQUALS_ANY:         decision.EqualsAny,
	}
)

// capabilitiesFromProto converts client capabilities. The request has been
// validated, so every enum is defined.
func capabilitiesFromProto(c *playbackv1.ClientCapabilities) *decision.ClientCapabilities {
	out := &decision.ClientCapabilities{
		Name:                             c.GetName(),
		MaxStreamingBitrate:              c.GetMaxStreamingBitrate(),
		MaxStaticBitrate:                 c.GetMaxStaticBitrate(),
		MusicStreamingTranscodingBitrate: c.GetMusicStreamingTranscodingBitrate(),
		MaxStaticMusicBitrate:            c.GetMaxStaticMusicBitrate(),
	}
	for _, p := range c.GetDirectPlay() {
		out.DirectPlay = append(out.DirectPlay, decision.DirectPlayProfile{
			Kind:       mediaKinds[p.GetKind()],
			Container:  decision.Names(p.GetContainer()),
			VideoCodec: decision.Names(p.GetVideoCodec()),
			AudioCodec: decision.Names(p.GetAudioCodec()),
		})
	}
	for _, p := range c.GetTranscoding() {
		out.Transcoding = append(out.Transcoding, decision.TranscodingProfile{
			Kind:                      mediaKinds[p.GetKind()],
			Context:                   contexts[p.GetContext()],
			Protocol:                  protocols[p.GetProtocol()],
			Container:                 p.GetContainer(),
			VideoCodec:                decision.Names(p.GetVideoCodec()),
			AudioCodec:                decision.Names(p.GetAudioCodec()),
			MaxAudioChannels:          int(p.GetMaxAudioChannels()),
			EstimateContentLength:     p.GetEstimateContentLength(),
			EnableMpegtsM2TsMode:      p.GetEnableMpegtsM2TsMode(),
			SeekInfo:                  seekInfos[p.GetSeekInfo()],
			CopyTimestamps:            p.GetCopyTimestamps(),
			EnableSubtitlesInManifest: p.GetEnableSubtitlesInManifest(),
			MinSegments:               int(p.GetMinSegments()),
			SegmentLength:             int(p.GetSegmentLengthSeconds()),
			Conditions:                conditionsFromProto(p.GetConditions()),
			DisableAudioVBR:           p.GetDisableAudioVbr(),
		})
	}
	for _, p := range c.GetContainers() {
		out.Containers = append(out.Containers, decision.ContainerProfile{
			Kind:         mediaKinds[p.GetKind()],
			Container:    decision.Names(p.GetContainer()),
			SubContainer: decision.Names(p.GetSubContainer()),
			Conditions:   conditionsFromProto(p.GetConditions()),
		})
	}
	for _, p := range c.GetCodecs() {
		out.Codecs = append(out.Codecs, decision.CodecProfile{
			Kind:            codecKinds[p.GetKind()],
			Codec:           decision.Names(p.GetCodec()),
			Container:       decision.Names(p.GetContainer()),
			SubContainer:    decision.Names(p.GetSubContainer()),
			Conditions:      conditionsFromProto(p.GetConditions()),
			ApplyConditions: conditionsFromProto(p.GetApplyConditions()),
		})
	}
	for _, p := range c.GetSubtitles() {
		out.Subtitles = append(out.Subtitles, decision.SubtitleProfile{
			Format:    p.GetFormat(),
			Method:    subtitleMethods[p.GetMethod()],
			Language:  decision.Names(p.GetLanguage()),
			Container: decision.Names(p.GetContainer()),
		})
	}
	return out
}

func conditionsFromProto(cs []*playbackv1.Condition) []decision.Condition {
	if len(cs) == 0 {
		return nil
	}
	out := make([]decision.Condition, len(cs))
	for i, c := range cs {
		out[i] = decision.Condition{
			Property: properties[c.GetProperty()],
			Op:       ops[c.GetOp()],
			Value:    c.GetValue(),
			Optional: c.GetOptional(),
		}
	}
	return out
}

// reasonsToProto lists transcode reasons; each enum value is the reason's
// bit index plus one.
func reasonsToProto(r decision.Reasons) []playbackv1.TranscodeReason {
	var out []playbackv1.TranscodeReason
	for v := uint32(r); v != 0; v &= v - 1 {
		out = append(out, playbackv1.TranscodeReason(bits.TrailingZeros32(v)+1))
	}
	return out
}

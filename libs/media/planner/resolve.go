package planner

import (
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
)

// transcoderChannelLimit caps the channels an audio encoder writes;
// unlisted encoders take 8.
var transcoderChannelLimit = map[string]int{
	"libmp3lame": 2, "libfdk_aac": 6, "ac3": 6, "eac3": 6, "dca": 6, "mlp": 6, "truehd": 6,
}

// Job builds the job carrying out a decision from start, copying streams
// the client takes as they are.
func (p *Planner) Job(d *decision.Decision, start time.Duration) *Job {
	j := &Job{
		Source:               d.Source,
		Video:                d.TargetVideoStream(),
		Audio:                d.TargetAudioStream(),
		SubtitleMethod:       d.SubtitleMethod,
		Request:              d,
		AudioBitrate:         d.AudioBitrate,
		AudioSampleRate:      d.AudioSampleRate,
		VideoBitrate:         d.VideoBitrate,
		MaxWidth:             d.MaxWidth,
		MaxHeight:            d.MaxHeight,
		MaxFramerate:         d.MaxFramerate,
		Container:            d.Container,
		Start:                start,
		CopyTimestamps:       d.CopyTimestamps,
		AllowVideoCopy:       true,
		AllowAudioCopy:       true,
		RequireAVC:           d.RequireAVC,
		RequireNonAnamorphic: d.RequireNonAnamorphic,
		AlwaysBurnIn:         d.AlwaysBurnInSubtitleWhenTranscoding,
		DisableAudioVBR:      d.DisableAudioVBR,
		Delivery:             Progressive,
	}
	if d.Protocol == decision.HLS {
		j.Delivery = HLS
		j.SegmentContainer = d.Container
		j.SegmentLength = time.Duration(d.SegmentLength) * time.Second
	}
	if d.SubtitleStreamIndex != nil && *d.SubtitleStreamIndex >= 0 && d.Source != nil {
		for i := range d.Source.Streams {
			if st := &d.Source.Streams[i]; st.Kind == core.StreamSubtitle && st.Index == *d.SubtitleStreamIndex {
				j.Subtitle = st
			}
		}
	}
	if j.Video != nil && d.Kind == decision.Video {
		j.VideoCodec = firstOr(d.VideoCodecs, j.Video.Codec)
		if p.CanCopyVideo(j) {
			j.VideoCodec = Copy
		}
	}
	if j.Audio != nil {
		j.AudioCodec = firstOr(d.AudioCodecs, InferAudioCodec(d.Container))
		if p.canCopyAudio(j, d) {
			j.AudioCodec = Copy
		} else {
			j.AudioChannels = p.audioChannels(j, d)
		}
	}
	return j
}

func firstOr(list []string, def string) string {
	if len(list) > 0 && list[0] != "" {
		return list[0]
	}
	return def
}

// canCopyAudio reports whether the audio is copied: the client takes its
// codec and the source stays within the channel, bitrate and sample rate
// targets.
func (p *Planner) canCopyAudio(j *Job, d *decision.Decision) bool {
	a := j.Audio
	if !j.AllowAudioCopy || a.ExternalPath != "" || !slices.ContainsFunc(d.AudioCodecs, func(c string) bool { return strings.EqualFold(c, a.Codec) }) {
		return false
	}
	if ch := d.TargetAudioChannels(); ch > 0 && a.Channels > ch {
		return false
	}
	lossless := slices.Contains(losslessAudioCodecs, strings.ToLower(a.Codec))
	if !lossless && j.AudioBitrate > 0 && a.Bitrate > int64(j.AudioBitrate) {
		return false
	}
	if j.AudioSampleRate > 0 && a.SampleRate > j.AudioSampleRate {
		return false
	}
	return true
}

// audioChannels returns the output channel count: the source's, capped by
// the client, the encoder and, for HLS, rounded to a layout Apple devices
// play (1, 2, 6 or 8 channels).
func (p *Planner) audioChannels(j *Job, d *decision.Decision) int {
	n := j.Audio.Channels
	if req := d.TargetAudioChannels(); req > 0 && (n == 0 || req < n) {
		n = req
	}
	limit, ok := transcoderChannelLimit[p.AudioEncoder(j)]
	if !ok {
		limit = 8
	}
	if n == 0 || n > limit {
		n = limit
	}
	if t := d.TranscodingMaxAudioChannels; t > 0 && t < n {
		n = t
	}
	if j.Delivery != Progressive {
		switch {
		case n == 5:
			n = 6
		case n == 7:
			n = 8
		case n > 2 && n < 6:
			n = 2
		}
	}
	return n
}

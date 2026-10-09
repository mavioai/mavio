package planner

import (
	"math"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Output describes the streams a job produces, as an HLS master playlist
// announces them.
type Output struct {
	// VideoCodec is the output video codec, such as "h264", "hevc" or
	// "av1"; empty without video.
	VideoCodec   string
	VideoProfile string
	// VideoLevel is the level as ffprobe reports it: 41 for H.264 level
	// 4.1, 120 for HEVC level 4; zero when unknown.
	VideoLevel    int
	BitDepth      int
	Width, Height int
	FrameRate     float64
	// Range is the output's dynamic range: SDR, or HDR for copied HDR
	// video; RangeType tells PQ from HLG.
	Range     core.VideoRange
	RangeType core.VideoRangeType
	// DolbyVision is the configuration of copied video that keeps its
	// Dolby Vision metadata; HDR10Plus says copied video keeps HDR10+.
	DolbyVision *core.DolbyVision
	HDR10Plus   bool
	// AudioCodec is the output audio codec, such as "aac"; empty without
	// audio.
	AudioCodec, AudioProfile string
	// VideoBitrate and AudioBitrate are the targets, or the source's for
	// copied streams; zero when unknown.
	VideoBitrate, AudioBitrate int
	// Copied streams pass through unchanged.
	VideoCopied, AudioCopied bool
}

// Default transcoding levels when the client asks for none.
const (
	defaultH264Level = 41
	defaultHEVCLevel = 120
	defaultAV1Level  = 19
)

// Output describes what the job produces.
func (p *Planner) Output(j *Job) Output {
	var o Output
	if v := j.Video; v != nil && j.isVideo() {
		p.videoOutput(j, v, &o)
	}
	if a := j.Audio; a != nil && j.AudioCodec != "" {
		o.AudioCopied = j.AudioCodec == Copy
		o.AudioCodec, o.AudioProfile, o.AudioBitrate = j.AudioCodec, "LC", j.AudioBitrate
		if o.AudioCopied {
			o.AudioCodec, o.AudioProfile, o.AudioBitrate = strings.ToLower(a.Codec), a.Profile, int(a.Bitrate)
		}
	}
	return o
}

func (p *Planner) videoOutput(j *Job, v *core.MediaStream, o *Output) {
	encoder := p.VideoEncoder(j)
	o.VideoCopied = encoder == Copy
	if fr, ok := referenceFrameRate(v); ok {
		if j.MaxFramerate > 0 && fr > j.MaxFramerate && !o.VideoCopied {
			fr = j.MaxFramerate
		}
		o.FrameRate = math.Round(float64(fr)*1000) / 1000
	}
	if o.VideoCopied {
		o.VideoCodec, o.VideoProfile, o.VideoLevel = normalizeCodec(v.Codec), v.Profile, v.Level
		o.BitDepth, o.Width, o.Height = colorBitDepth(v), v.Width, v.Height
		o.Range, o.RangeType = v.VideoRange(), v.VideoRangeType()
		o.VideoBitrate = int(v.Bitrate)
		if IsDOVI(v) && !p.DOVIRemoved(j) && v.DolbyVision != nil {
			dv := *v.DolbyVision
			o.DolbyVision = &dv
		}
		o.HDR10Plus = IsHDR10Plus(v) && !p.HDR10PlusRemoved(j)
		return
	}
	// Encoded video is SDR: HDR sources are tone mapped.
	o.VideoCodec = normalizeCodec(j.VideoCodec)
	o.Range, o.RangeType, o.BitDepth = core.RangeSDR, core.RangeTypeSDR, 8
	o.VideoBitrate = j.VideoBitrate
	if w, h, ok := outputSize(j); ok {
		o.Width, o.Height = w, h
	}
	switch o.VideoCodec {
	case "h264":
		o.VideoProfile = "high"
		if req := splitList(j.option("h264", "profile")); len(req) > 0 {
			o.VideoProfile = strings.ToLower(strings.ReplaceAll(req[0], " ", ""))
		}
		if o.VideoProfile == "high10" && encoder == "libx264" {
			o.BitDepth = 10
		} else if o.VideoProfile == "high10" {
			o.VideoProfile = "high"
		}
		o.VideoLevel = requestedLevel(j, "h264", defaultH264Level)
	case "hevc":
		o.VideoProfile = "main"
		o.VideoLevel = requestedLevel(j, "hevc", defaultHEVCLevel)
	case "av1":
		o.VideoProfile = "main"
		o.VideoLevel = requestedLevel(j, "av1", defaultAV1Level)
	}
}

// requestedLevel is the highest level the client accepts for codec.
func requestedLevel(j *Job, codec string, def int) int {
	l := j.option(codec, "level")
	if l == "" && codec == "hevc" {
		l = j.option("h265", "level")
	}
	if n, err := strconv.ParseFloat(l, 64); err == nil && n > 0 {
		return int(n)
	}
	return def
}

func normalizeCodec(c string) string {
	switch c = strings.ToLower(c); c {
	case "h265", "hevc":
		return "hevc"
	case "avc", "h264":
		return "h264"
	}
	return c
}

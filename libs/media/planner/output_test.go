package planner

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
)

func TestOutput(t *testing.T) {
	video := core.MediaStream{
		Kind: core.StreamVideo, Index: 0, Codec: "hevc", Profile: "Main 10", Level: 150, Width: 3840, Height: 2160, BitDepth: 10,
		PixelFormat: "yuv420p10le", ColorTransfer: "smpte2084", ColorPrimaries: "bt2020", ColorSpace: "bt2020nc",
		RealFrameRate: core.Rational{Num: 24000, Den: 1001}, Bitrate: 40_000_000,
	}
	audio := core.MediaStream{Kind: core.StreamAudio, Index: 1, Codec: "aac", Profile: "HE-AAC", Channels: 2, Bitrate: 128_000}
	job := func(videoCodec, audioCodec string) *Job {
		src := &decision.Source{MediaSource: &core.MediaSource{Container: "mkv", Streams: []core.MediaStream{video, audio}}}
		return &Job{
			Delivery: HLS, Source: src, Video: &src.Streams[0], Audio: &src.Streams[1], Request: &decision.Decision{},
			VideoCodec: videoCodec, AudioCodec: audioCodec, AllowVideoCopy: true, AllowAudioCopy: true,
		}
	}
	p := &Planner{Options: DefaultOptions(), Caps: doviPlanner(true).Caps}

	t.Run("copied", func(t *testing.T) {
		o := p.Output(job(Copy, Copy))
		want := Output{
			VideoCodec: "hevc", VideoProfile: "Main 10", VideoLevel: 150, BitDepth: 10, Width: 3840, Height: 2160, FrameRate: 23.976,
			Range: core.RangeHDR, RangeType: core.RangeTypeHDR10, AudioCodec: "aac", AudioProfile: "HE-AAC",
			VideoBitrate: 40_000_000, AudioBitrate: 128_000, VideoCopied: true, AudioCopied: true,
		}
		if o != want {
			t.Errorf("Output() = %+v, want = %+v", o, want)
		}
	})
	t.Run("transcoded", func(t *testing.T) {
		j := job("h264", "aac")
		j.MaxWidth, j.MaxHeight, j.MaxFramerate, j.VideoBitrate, j.AudioBitrate = 1920, 1080, 30, 8_000_000, 192_000
		j.Request.SetOption("h264", "level", "40")
		o := p.Output(j)
		want := Output{
			VideoCodec: "h264", VideoProfile: "high", VideoLevel: 40, BitDepth: 8, Width: 1920, Height: 1080, FrameRate: 23.976,
			Range: core.RangeSDR, RangeType: core.RangeTypeSDR, AudioCodec: "aac", AudioProfile: "LC",
			VideoBitrate: 8_000_000, AudioBitrate: 192_000,
		}
		if o != want {
			t.Errorf("Output() = %+v, want = %+v", o, want)
		}
	})
	t.Run("Dolby Vision kept", func(t *testing.T) {
		j := doviJob("hevc", "smpte2084")
		j.Video.ColorSpace, j.Video.ColorPrimaries = "bt2020nc", "bt2020"
		j.Video.DolbyVision.Profile, j.Video.DolbyVision.BLCompatibilityID, j.Video.DolbyVision.Level = 8, 1, 6
		requestRanges(j, "DOVI,HDR10")
		o := doviPlanner(true).Output(j)
		if o.DolbyVision == nil || o.DolbyVision.Profile != 8 || o.DolbyVision.Level != 6 || o.Range != core.RangeHDR {
			t.Errorf("Output() = %+v, want = Dolby Vision 8.6 kept", o)
		}
		// An enhancement layer the client cannot take is stripped.
		j.Video.DolbyVision.Profile, j.Video.DolbyVision.BLCompatibilityID = 7, 6
		if o := doviPlanner(true).Output(j); o.DolbyVision != nil {
			t.Errorf("Output() = %+v, want = Dolby Vision removed", o)
		}
	})
}

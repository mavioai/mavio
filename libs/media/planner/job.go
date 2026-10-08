package planner

import (
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
	"github.com/mavioai/mavio/libs/media/hwaccel"
)

// Delivery is how the output reaches the client.
type Delivery string

// Deliveries.
const (
	// Progressive writes one file or byte stream.
	Progressive Delivery = "progressive"
	// HLS writes playlist segments.
	HLS Delivery = "hls"
)

// Copy is the codec name for copying a stream unchanged.
const Copy = "copy"

// Job describes one ffmpeg run: what is read, what is produced and the
// targets the playback decision set.
type Job struct {
	Delivery Delivery
	Source   *decision.Source
	// Video, Audio and Subtitle are the selected streams of Source; nil
	// when there are none.
	Video, Audio, Subtitle *core.MediaStream
	SubtitleMethod         decision.SubtitleMethod
	// Request carries the client's per-codec limits, such as accepted
	// profiles, levels and range types; nil means none.
	Request *decision.Decision

	// VideoCodec and AudioCodec are the output codecs, such as "h264" or
	// [Copy]; empty VideoCodec means no video output.
	VideoCodec, AudioCodec string
	// AudioChannels, AudioBitrate and AudioSampleRate target the output
	// audio; 0 keeps the source's.
	AudioChannels, AudioBitrate, AudioSampleRate int
	// VideoBitrate targets the output video; 0 uses the encoder's quality
	// setting.
	VideoBitrate int
	// MaxWidth, MaxHeight and MaxFramerate bound the output video.
	MaxWidth, MaxHeight int
	MaxFramerate        float32

	// Container is the progressive output container; SegmentContainer the
	// HLS segment container, "mp4" (fMP4) or "ts".
	Container        string
	SegmentContainer string
	SegmentLength    time.Duration

	// Start is the position to start at.
	Start time.Duration
	// CopyTimestamps keeps the source timestamps.
	CopyTimestamps bool

	AllowVideoCopy, AllowAudioCopy   bool
	RequireAVC, RequireNonAnamorphic bool
	// Deinterlace asks for deinterlacing interlaced video.
	Deinterlace bool
	// AlwaysBurnIn burns the subtitle in whenever the video is encoded.
	AlwaysBurnIn bool
	// DisableAudioVBR asks for constant bitrate audio.
	DisableAudioVBR bool
}

// isVideo reports whether the job produces video.
func (j *Job) isVideo() bool { return j.VideoCodec != "" }

// option returns a requested option for codec.
func (j *Job) option(codec, name string) string {
	if j.Request == nil {
		return ""
	}
	return j.Request.Option(codec, name)
}

// Options are the server's encoding settings.
type Options struct {
	// Threads is the encoder thread count; 0 lets ffmpeg decide.
	Threads int
	// Hardware is the acceleration to use: "" or "videotoolbox".
	Hardware string
	// HardwareEncoding allows hardware encoders.
	HardwareEncoding bool
	// AudioVBR enables variable bitrate audio where the encoder has it.
	AudioVBR bool
	// Downmix is the stereo downmix algorithm.
	Downmix Downmix
	// DownmixBoost amplifies downmixed audio; 1 leaves it.
	DownmixBoost float64
	// Tonemapping settings for HDR to SDR conversion.
	TonemapAlgorithm string
	TonemapRange     string // "auto", "tv" or "pc"
	TonemapDesat     float64
	TonemapPeak      float64
	TonemapParam     float64
	// VideoToolboxTonemapping enables tone mapping on Apple hardware.
	VideoToolboxTonemapping bool
	// DeinterlaceMethod is "yadif" or "bwdif"; DeinterlaceDoubleRate keeps
	// every field as a frame.
	DeinterlaceMethod     string
	DeinterlaceDoubleRate bool
	// H264CRF and H265CRF are the software encoders' quality.
	H264CRF, H265CRF int
	// Preset is the software encoders' speed preset; "" picks one.
	Preset string
}

// DefaultOptions returns the settings a new server starts with.
func DefaultOptions() Options {
	return Options{
		HardwareEncoding:  true,
		DownmixBoost:      2,
		TonemapAlgorithm:  "bt2390",
		TonemapRange:      "auto",
		TonemapPeak:       100,
		DeinterlaceMethod: "yadif",
		H264CRF:           23,
		H265CRF:           28,
	}
}

// Planner turns jobs into ffmpeg arguments.
type Planner struct {
	Options Options
	// Caps are the capabilities of the ffmpeg in use.
	Caps *hwaccel.Capabilities
	// FileExists reports whether a file exists, for companion files such
	// as VobSub indexes; nil assumes none do.
	FileExists func(path string) bool
}

func (p *Planner) caps() *hwaccel.Capabilities {
	if p.Caps == nil {
		return &hwaccel.Capabilities{}
	}
	return p.Caps
}

func (p *Planner) exists(path string) bool {
	return p.FileExists != nil && p.FileExists(path)
}

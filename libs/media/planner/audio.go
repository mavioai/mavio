package planner

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/hwaccel"
)

// Downmix is a stereo downmix algorithm.
type Downmix string

// Downmix algorithms; DownmixNone leaves it to ffmpeg's -ac.
const (
	DownmixNone              Downmix = ""
	DownmixDave750           Downmix = "dave750"
	DownmixNightmodeDialogue Downmix = "nightmode_dialogue"
	DownmixRFC7845           Downmix = "rfc7845"
	DownmixAC4               Downmix = "ac4"
)

const (
	pan51To71Side = "pan=5.1(side)|c0=c0|c1=c1|c2=c2|c3=c3|c4=0.707*c4+0.707*c6|c5=0.707*c5+0.707*c7"
	dave750       = "pan=stereo|c0=0.5*c2+0.707*c0+0.707*c4+0.5*c3|c1=0.5*c2+0.707*c1+0.707*c5+0.5*c3"
	nightmode     = "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5"
	ac4From51     = "pan=stereo|c0=c0+0.707*c2+0.707*c4|c1=c1+0.707*c2+0.707*c5"
)

// downmixFilters are the pan filters per algorithm and source layout; 7.1
// is first folded to 5.1 as AC-4 does.
var downmixFilters = map[Downmix]map[string]string{
	DownmixDave750: {
		"5.1": dave750,
		"7.1": pan51To71Side + "," + dave750,
	},
	DownmixNightmodeDialogue: {
		"5.1": nightmode,
		"7.1": pan51To71Side + "," + nightmode,
	},
	DownmixRFC7845: {
		"3.0":  "pan=stereo|c0=0.414214*c2+0.585786*c0|c1=0.414214*c2+0.585786*c1",
		"quad": "pan=stereo|c0=0.422650*c0+0.366025*c2+0.211325*c3|c1=0.422650*c1+0.366025*c3+0.211325*c2",
		"5.0":  "pan=stereo|c0=0.460186*c2+0.650802*c0+0.563611*c3+0.325401*c4|c1=0.460186*c2+0.650802*c1+0.563611*c4+0.325401*c3",
		"5.1":  "pan=stereo|c0=0.374107*c2+0.529067*c0+0.458186*c4+0.264534*c5+0.374107*c3|c1=0.374107*c2+0.529067*c1+0.458186*c5+0.264534*c4+0.374107*c3",
		"6.1":  "pan=stereo|c0=0.321953*c2+0.455310*c0+0.394310*c5+0.227655*c6+0.278819*c4+0.321953*c3|c1=0.321953*c2+0.455310*c1+0.394310*c6+0.227655*c5+0.278819*c4+0.321953*c3",
		"7.1":  "pan=stereo|c0=0.274804*c2+0.388631*c0+0.336565*c6+0.194316*c7+0.336565*c4+0.194316*c5+0.274804*c3|c1=0.274804*c2+0.388631*c1+0.336565*c7+0.194316*c6+0.336565*c5+0.194316*c4+0.274804*c3",
	},
	DownmixAC4: {
		"3.0": "pan=stereo|c0=c0+0.707*c2|c1=c1+0.707*c2",
		"5.0": "pan=stereo|c0=c0+0.707*c2+0.707*c3|c1=c1+0.707*c2+0.707*c4",
		"5.1": ac4From51,
		"7.0": "pan=5.0(side)|c0=c0|c1=c1|c2=c2|c3=0.707*c3+0.707*c5|c4=0.707*c4+0.707*c6,pan=stereo|c0=c0+0.707*c2+0.707*c3|c1=c1+0.707*c2+0.707*c4",
		"7.1": pan51To71Side + "," + ac4From51,
	},
}

// DownmixFilter returns the pan filter of an algorithm for a source layout.
func DownmixFilter(algo Downmix, layout string) (string, bool) {
	f, ok := downmixFilters[algo][layout]
	return f, ok
}

// channelLayout returns the stream's layout, or the usual one for its
// channel count.
func channelLayout(a *core.MediaStream) string {
	if strings.TrimSpace(a.ChannelLayout) != "" {
		return a.ChannelLayout
	}
	switch a.Channels {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	case 3:
		return "2.1"
	case 4:
		return "4.0"
	case 5:
		return "5.0"
	case 6:
		return "5.1"
	case 7:
		return "6.1"
	case 8:
		return "7.1"
	}
	return ""
}

// InferAudioCodec returns the audio codec a container usually carries,
// falling back to AAC; manifests and other non-codec names never become
// encoder names.
func InferAudioCodec(container string) string {
	c := strings.ToLower(strings.TrimSpace(container))
	switch c {
	case "ogg", "oga", "ogv", "webm", "webma":
		return "opus"
	case "m4a", "m4b", "mp4", "mov", "mkv", "mka":
		return "aac"
	case "ts", "avi", "flv", "f4v", "swf":
		return "mp3"
	case "aac", "ac3", "alac", "dts", "eac3", "flac", "mp2", "mp3", "opus", "truehd", "vorbis":
		return c
	}
	return "aac"
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9\-._,|]{0,40}$`)

var losslessAudioCodecs = []string{"alac", "ape", "flac", "mlp", "truehd", "wavpack"}

var mp4Containers = []string{"mp4", "m4a", "m4p", "m4b", "m4r", "m4v"}

// AudioEncoder returns the ffmpeg encoder for the job's audio codec,
// preferring AudioToolbox and then FDK for AAC.
func (p *Planner) AudioEncoder(j *Job) string {
	return audioEncoder(p.caps(), j.AudioCodec)
}

func audioEncoder(caps *hwaccel.Capabilities, codec string) string {
	if !validName.MatchString(codec) {
		codec = "aac"
	}
	switch strings.ToLower(codec) {
	case "aac":
		switch {
		case caps.SupportsEncoder("aac_at"):
			return "aac_at"
		case caps.SupportsEncoder("libfdk_aac"):
			return "libfdk_aac"
		}
		return "aac"
	case "mp3":
		return "libmp3lame"
	case "vorbis":
		return "libvorbis"
	case "opus":
		return "libopus"
	case "dts":
		return "dca"
	}
	// ALAC uses ffmpeg's own encoder; AudioToolbox's broke in ffmpeg 6.1.
	return strings.ToLower(codec)
}

// audioVBR returns variable bitrate options for encoders that have them.
func audioVBR(encoder string, bitrate, channels int) []string {
	perChannel := bitrate / max(channels, 1)
	pick := func(limits []int, values []string) string {
		for i, l := range limits {
			if perChannel < l {
				return values[i]
			}
		}
		return values[len(values)-1]
	}
	switch strings.ToLower(encoder) {
	case "libfdk_aac":
		return []string{"-vbr:a", pick([]int{32000, 48000, 64000, 96000}, []string{"1", "2", "3", "4", "5"})}
	case "libmp3lame":
		// LAME's VBR is only good in a middle range; ABR elsewhere.
		if perChannel > 48000 && perChannel < 122500 {
			return []string{"-qscale:a", pick([]int{64000, 88000, 112000}, []string{"6", "4", "2", "0"})}
		}
		return []string{"-abr:a", "1", "-b:a", strconv.Itoa(bitrate)}
	case "aac_at":
		// AudioToolbox's constrained VBR.
		return []string{"-aac_at_mode:a", "2", "-b:a", strconv.Itoa(bitrate)}
	case "libvorbis":
		return []string{"-qscale:a", pick([]int{40000, 56000, 80000, 112000}, []string{"0", "2", "4", "6", "8"})}
	}
	return nil
}

// usesDownmixFilter reports whether stereo output from a multichannel
// source goes through the downmix filter instead of -ac.
func (p *Planner) usesDownmixFilter(j *Job) bool {
	if j.AudioChannels != 2 || j.Audio == nil || j.Audio.Channels <= 2 {
		return false
	}
	_, ok := DownmixFilter(p.Options.Downmix, channelLayout(j.Audio))
	return ok
}

// AudioFilters returns the audio filters: the stereo downmix with its
// boost, and for text subtitles burned into a progressive stream that
// restarts its timestamps, the matching audio shift.
func (p *Planner) AudioFilters(j *Job) []string {
	var filters []string
	if j.AudioChannels == 2 && j.Audio != nil && j.Audio.Channels > 2 {
		if f, ok := DownmixFilter(p.Options.Downmix, channelLayout(j.Audio)); ok {
			filters = append(filters, f)
		}
		if p.Options.DownmixBoost != 1 {
			filters = append(filters, "volume="+strconv.FormatFloat(p.Options.DownmixBoost, 'f', -1, 64))
		}
	}
	copyingTimestamps := j.CopyTimestamps || j.Delivery != Progressive
	if j.Subtitle != nil && j.Subtitle.IsTextSubtitle() && p.burnsInSubtitle(j) && !copyingTimestamps {
		filters = append(filters, fmt.Sprintf("asetpts=PTS-%d/TB", int64(math.RoundToEven(j.Start.Seconds()))))
	}
	return filters
}

// audioArgs returns the options encoding the audio: codec, channels,
// bitrate, sample rate and filters.
func (p *Planner) audioArgs(j *Job, encoder string) []string {
	args := []string{"-codec:a:0", encoder}
	if encoder == Copy {
		return args
	}
	if j.AudioChannels > 0 && !p.usesDownmixFilter(j) {
		args = append(args, "-ac", strconv.Itoa(j.AudioChannels))
	}
	if j.AudioBitrate > 0 && !slices.Contains(losslessAudioCodecs, strings.ToLower(encoder)) {
		channels := j.AudioChannels
		if channels == 0 {
			channels = 2
		}
		if vbr := audioVBR(encoder, j.AudioBitrate, channels); p.Options.AudioVBR && !j.DisableAudioVBR && vbr != nil {
			args = append(args, vbr...)
		} else {
			args = append(args, "-ab", strconv.Itoa(j.AudioBitrate))
		}
	}
	if j.AudioSampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(j.AudioSampleRate))
	}
	if f := p.AudioFilters(j); len(f) > 0 {
		args = append(args, "-af", strings.Join(f, ","))
	}
	return args
}

// opusSampleRate snaps a sample rate to one libopus supports.
func opusSampleRate(rate int) int {
	switch {
	case rate <= 8000:
		return 8000
	case rate <= 12000:
		return 12000
	case rate <= 16000:
		return 16000
	case rate <= 24000:
		return 24000
	}
	return 48000
}

// ProgressiveAudioArgs returns the ffmpeg arguments transcoding an audio
// file into a progressive stream written to output.
func (p *Planner) ProgressiveAudioArgs(j *Job, output string) []string {
	args := p.inputArgs(j)
	args = append(args, "-threads", strconv.Itoa(p.Options.Threads), "-vn")
	encoder := p.AudioEncoder(j)
	if j.AudioBitrate > 0 && !slices.Contains(losslessAudioCodecs, strings.ToLower(j.AudioCodec)) {
		channels := j.AudioChannels
		if channels == 0 {
			channels = 2
		}
		if vbr := audioVBR(encoder, j.AudioBitrate, channels); p.Options.AudioVBR && !j.DisableAudioVBR && vbr != nil {
			args = append(args, vbr...)
		} else {
			args = append(args, "-ab", strconv.Itoa(j.AudioBitrate))
		}
	}
	if j.AudioChannels > 0 {
		args = append(args, "-ac", strconv.Itoa(j.AudioChannels))
	}
	if j.AudioCodec != "" {
		args = append(args, "-acodec", encoder)
	}
	// Raw PCM has no header of its own; only a client asking for the raw
	// "pcm" container gets the raw muxer, a WAV still gets its RIFF header.
	if pcm, ok := strings.CutPrefix(encoder, "pcm_"); ok && strings.EqualFold(j.Container, "pcm") {
		args = append(args, "-f", pcm)
	}
	if rate := j.AudioSampleRate; rate > 0 {
		if strings.EqualFold(j.AudioCodec, "opus") {
			rate = opusSampleRate(rate)
		}
		args = append(args, "-ar", strconv.Itoa(rate))
	}
	// Without the downmix filter, -ac 2 drops the LFE channel.
	if f := p.AudioFilters(j); len(f) > 0 {
		args = append(args, "-af", strings.Join(f, ","))
	}
	if slices.Contains(mp4Containers, strings.ToLower(j.Container)) {
		args = append(args, "-movflags", "empty_moov+delay_moov")
	}
	return append(args, "-map_metadata", "-1", "-id3v2_version", "3", "-write_id3v1", "1", "-y", output)
}

// audioBitstreamArgs returns the bitstream filters for copied audio:
// dropping packets before the start of a seeked HLS transcode, whose video
// is trimmed exactly, and converting ADTS AAC for MP4 segments.
func (p *Planner) audioBitstreamArgs(j *Job, segmentContainer, sourceContainer string) []string {
	var filters []string
	if f := p.copiedAudioTrim(j); f != "" {
		filters = append(filters, f)
	}
	format := strings.TrimPrefix(segmentExtension(segmentContainer), ".")
	if strings.EqualFold(format, "mp4") && isADTSContainer(sourceContainer) && j.Audio != nil && isAAC(j.Audio) {
		filters = append(filters, "aac_adtstoasc")
	}
	if len(filters) == 0 {
		return nil
	}
	return []string{"-bsf:a", strings.Join(filters, ",")}
}

func isADTSContainer(c string) bool {
	for _, n := range []string{"ts", "aac", "hls"} {
		if strings.EqualFold(c, n) {
			return true
		}
	}
	return false
}

// AudioBitstreamArgs is audioBitstreamArgs joined as one option string with
// a leading space, the form Jellyfin's command lines use.
func (p *Planner) AudioBitstreamArgs(j *Job, segmentContainer, sourceContainer string) string {
	args := p.audioBitstreamArgs(j, segmentContainer, sourceContainer)
	if len(args) == 0 {
		return ""
	}
	return " " + strings.Join(args, " ")
}

// copiedAudioTrim drops copied audio before the start of a seeked HLS
// video transcode: the video decoder trims to the exact position, copied
// audio would start at the previous keyframe. The noise filter's drop
// option needs ffmpeg 5.0; WTV seeking breaks with it.
func (p *Planner) copiedAudioTrim(j *Job) string {
	if j.Delivery != HLS || !j.isVideo() || j.VideoCodec == Copy || j.AudioCodec != Copy ||
		strings.EqualFold(j.inputContainer(), "wtv") || p.caps().Version.Compare(hwaccel.NewVersion(5, 0)) < 0 {
		return ""
	}
	if j.Start <= 0 {
		return ""
	}
	return fmt.Sprintf(`noise=drop='lt(pts*tb\,%.3f)'`, j.Start.Seconds())
}

func (j *Job) inputContainer() string {
	if j.Source == nil || j.Source.MediaSource == nil {
		return ""
	}
	return j.Source.Container
}

func segmentExtension(container string) string {
	if strings.TrimSpace(container) != "" {
		return "." + container
	}
	return ".ts"
}

func isAAC(st *core.MediaStream) bool { return strings.Contains(strings.ToLower(st.Codec), "aac") }

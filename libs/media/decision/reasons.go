package decision

import (
	"math/bits"
	"strings"
)

// Reasons is a set of reasons why a stream cannot be played directly.
type Reasons uint32

// Transcode reasons. The values are stable and may be persisted.
const (
	ContainerNotSupported Reasons = 1 << iota
	VideoCodecNotSupported
	AudioCodecNotSupported
	SubtitleCodecNotSupported
	AudioIsExternal
	SecondaryAudioNotSupported
	VideoProfileNotSupported
	VideoLevelNotSupported
	VideoResolutionNotSupported
	VideoBitDepthNotSupported
	VideoFramerateNotSupported
	RefFramesNotSupported
	AnamorphicVideoNotSupported
	InterlacedVideoNotSupported
	AudioChannelsNotSupported
	AudioProfileNotSupported
	AudioSampleRateNotSupported
	AudioBitDepthNotSupported
	ContainerBitrateExceedsLimit
	VideoBitrateNotSupported
	AudioBitrateNotSupported
	UnknownVideoStreamInfo
	UnknownAudioStreamInfo
	DirectPlayError
	VideoRangeTypeNotSupported
	VideoCodecTagNotSupported
	StreamCountExceedsLimit
	VideoRotationNotSupported
)

// Reason groups.
const (
	containerReasons  = ContainerNotSupported | ContainerBitrateExceedsLimit
	audioCodecReasons = AudioBitrateNotSupported | AudioChannelsNotSupported | AudioProfileNotSupported |
		AudioSampleRateNotSupported | SecondaryAudioNotSupported | AudioBitDepthNotSupported | AudioIsExternal
	audioReasons      = AudioCodecNotSupported | audioCodecReasons
	videoCodecReasons = VideoResolutionNotSupported | AnamorphicVideoNotSupported | InterlacedVideoNotSupported |
		VideoBitDepthNotSupported | VideoBitrateNotSupported | VideoFramerateNotSupported | VideoLevelNotSupported |
		RefFramesNotSupported | VideoRangeTypeNotSupported | VideoProfileNotSupported | VideoRotationNotSupported
	videoReasons        = VideoCodecNotSupported | videoCodecReasons
	directStreamReasons = audioReasons | ContainerNotSupported | VideoCodecTagNotSupported
)

var reasonNames = [...]string{
	"ContainerNotSupported", "VideoCodecNotSupported", "AudioCodecNotSupported", "SubtitleCodecNotSupported",
	"AudioIsExternal", "SecondaryAudioNotSupported", "VideoProfileNotSupported", "VideoLevelNotSupported",
	"VideoResolutionNotSupported", "VideoBitDepthNotSupported", "VideoFramerateNotSupported", "RefFramesNotSupported",
	"AnamorphicVideoNotSupported", "InterlacedVideoNotSupported", "AudioChannelsNotSupported", "AudioProfileNotSupported",
	"AudioSampleRateNotSupported", "AudioBitDepthNotSupported", "ContainerBitrateExceedsLimit", "VideoBitrateNotSupported",
	"AudioBitrateNotSupported", "UnknownVideoStreamInfo", "UnknownAudioStreamInfo", "DirectPlayError",
	"VideoRangeTypeNotSupported", "VideoCodecTagNotSupported", "StreamCountExceedsLimit", "VideoRotationNotSupported",
}

// ParseReason returns the reason with the given name.
func ParseReason(name string) (Reasons, bool) {
	for i, n := range reasonNames {
		if strings.EqualFold(n, name) {
			return 1 << i, true
		}
	}
	return 0, false
}

// String lists the reasons as "A|B".
func (r Reasons) String() string {
	var names []string
	for v := uint32(r); v != 0; v &= v - 1 {
		i := bits.TrailingZeros32(v)
		if i < len(reasonNames) {
			names = append(names, reasonNames[i])
		}
	}
	return strings.Join(names, "|")
}

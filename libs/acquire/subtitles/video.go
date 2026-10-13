package subtitles

import (
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/release"
)

// Video describes a movie or an episode file for matching and scoring
// subtitles, as subliminal's Movie and Episode.
type Video struct {
	// Episode is true for an episode, false for a movie.
	Episode bool
	// Title is the movie's title or the episode's; AlternativeTitles are
	// the movie's other titles.
	Title             string
	AlternativeTitles []string
	// Series and AlternativeSeries are the episode's series' names.
	Series            string
	AlternativeSeries []string
	// OriginalSeries is true when the series' name needs no year to tell
	// it from another series, so a release name without one matches it.
	OriginalSeries bool
	Season         int
	EpisodeNumber  int
	// Special is true for an episode of the specials season, whose
	// numbers subtitle sites tend to get wrong.
	Special bool
	Year    int
	// ReleaseGroup, Source, Resolution, VideoCodec, AudioCodec, Edition
	// and StreamingService describe the file as Guess does a release name.
	ReleaseGroup     string
	Source           string
	Resolution       string
	VideoCodec       string
	AudioCodec       string
	Edition          string
	StreamingService string
}

// The sources of videos and release names, as Bazarr merges guessit's:
// subtitles for one source suit a video from another of the same group.
const (
	SourceTV     = "tv"
	SourceAir    = "air"
	SourceDiskHD = "disk-hd"
	SourceDiskSD = "disk-sd"
	SourceWeb    = "web"
)

// SourceOf returns the merged source of a quality, or "" for sources
// Bazarr does not merge (cameras, telesyncs, telecines, workprints), which
// match nothing.
func SourceOf(q release.Quality) string {
	switch q {
	case release.SDTV, release.HDTV720p, release.HDTV1080p, release.HDTV2160p:
		return SourceTV
	case release.DVD, release.DVDR, release.DVDSCR, release.Regional:
		return SourceDiskSD
	}
	switch q.Source() {
	case release.SourceWebDL, release.SourceWebRip:
		return SourceWeb
	case release.SourceBluray:
		return SourceDiskHD
	}
	return ""
}

// ResolutionOf returns the resolution of a quality as guessit names it
// ("1080p"), or "" when it is unknown.
func ResolutionOf(q release.ParsedQuality) string {
	r := q.Resolution
	if r == 0 {
		r = q.Quality.Resolution()
	}
	return ResolutionName(r)
}

// ResolutionName returns a resolution in lines as guessit names it
// ("720p"), or "" for 0.
func ResolutionName(lines int) string {
	if lines <= 0 {
		return ""
	}
	return strconv.Itoa(lines) + "p"
}

// VideoCodecOf returns the guessit name of a video stream's codec, as
// ffprobe names it ("h264" is "H.264"), or "" for codecs guessit does not
// name.
func VideoCodecOf(codec string) string {
	switch strings.ToLower(codec) {
	case "h264":
		return "H.264"
	case "hevc":
		return "H.265"
	case "h263":
		return "H.263"
	case "mpeg2video":
		return "MPEG-2"
	case "vc1":
		return "VC-1"
	case "vp7":
		return "VP7"
	case "vp8":
		return "VP8"
	case "vp9":
		return "VP9"
	}
	return ""
}

// AudioCodecOf returns the guessit name of an audio stream's codec and
// profile, as ffprobe names them ("eac3" is "Dolby Digital Plus", "dts"
// of profile "DTS-HD MA" is "DTS-HD"), or "" for codecs guessit does not
// name.
func AudioCodecOf(codec, profile string) string {
	switch c := strings.ToLower(codec); {
	case c == "ac3":
		return "Dolby Digital"
	case c == "eac3":
		return "Dolby Digital Plus"
	case c == "truehd":
		return "Dolby TrueHD"
	case c == "dts" && strings.Contains(profile, "DTS:X"):
		return "DTS:X"
	case c == "dts" && strings.HasPrefix(profile, "DTS-HD"):
		return "DTS-HD"
	case c == "dts":
		return "DTS"
	case c == "aac":
		return "AAC"
	case c == "flac":
		return "FLAC"
	case c == "mp3":
		return "MP3"
	case c == "mp2":
		return "MP2"
	case c == "opus":
		return "Opus"
	case c == "vorbis":
		return "Vorbis"
	case strings.HasPrefix(c, "pcm_"):
		return "PCM"
	}
	return ""
}

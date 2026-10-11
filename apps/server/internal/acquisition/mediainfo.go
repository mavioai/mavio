package acquisition

import (
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/release"
)

// importQuality returns the quality of a movie file being imported,
// aggregated, as Radarr's AggregateQuality, from its file name, the name
// of the download's folder, its media info and the download's release
// name, so that the video's resolution corrects the names.
func importQuality(d *core.Download, f importFile, probed library.ProbeResult) acquire.QualityModel {
	augments := []*acquire.QualityAugment{acquire.AugmentParsedQuality(release.ParseQuality(filepath.Base(f.path), release.Movie))}
	if f.inFolder {
		if folder := strings.TrimSuffix(f.path, f.rel); folder != f.path {
			augments = append(augments, acquire.AugmentParsedQuality(release.ParseQuality(filepath.Base(filepath.Clean(folder)), release.Movie)))
		}
	}
	if v := videoStream(probed.Source); v != nil {
		augments = append(augments, acquire.AugmentMediaInfoQuality(v.Width, v.Height, probed.Tags.Name))
	}
	grabbed := release.ParseQuality(d.Title, release.Movie)
	if q := qualityModel(d.Quality); grabbed.Quality == release.Unknown && q.Quality != release.Unknown {
		// The quality the release was grabbed as, when its title says none.
		grabbed = release.ParsedQuality{
			Quality: q.Quality, Revision: q.Revision,
			SourceDetection: release.DetectedName, ResolutionDetection: release.DetectedName, ModifierDetection: release.DetectedName,
		}
	}
	augments = append(augments, acquire.AugmentParsedQuality(grabbed))
	return acquire.ModelOf(acquire.AggregateQuality(augments...))
}

// videoStream returns the main video stream of a file, or nil.
func videoStream(src core.MediaSource) *core.MediaStream {
	for i := range src.Streams {
		if s := &src.Streams[i]; s.Kind == core.StreamVideo && !s.IsDolbyVisionEnhancement() {
			return s
		}
	}
	return nil
}

// mediaInfoOf returns the media info of a probed file for the MediaInfo
// tokens of file names: the codec, bit depth and dynamic range of its
// main video stream, the codec and channels of its first audio stream,
// the languages of its audio and subtitle streams, and whether its tags
// say it is 3D. sceneName, the release title, chooses among the spellings
// of video codecs.
func mediaInfoOf(probed library.ProbeResult, sceneName string) *organize.MediaInfo {
	src := probed.Source
	mi := &organize.MediaInfo{Is3D: probed.Tags.Video3DFormat != ""}
	known := false
	if v := videoStream(src); v != nil {
		known = true
		mi.VideoCodec = organize.FormatVideoCodec(v.Codec, v.CodecTag, sceneName)
		mi.VideoBitDepth = v.BitDepth
		mi.VideoDynamicRangeType = dynamicRangeType(v.VideoRangeType())
	}
	audio := false
	for _, s := range src.Streams {
		switch s.Kind {
		case core.StreamAudio:
			known = true
			if !audio {
				audio = true
				mi.AudioCodec = organize.FormatAudioCodec(s.Codec, s.CodecTag, s.Profile)
				mi.AudioChannels = organize.FormatAudioChannels(s.ChannelLayout, float64(s.Channels))
			}
			if s.Language != "" {
				mi.AudioLanguages = append(mi.AudioLanguages, s.Language)
			}
		case core.StreamSubtitle:
			if s.Language != "" && s.ExternalPath == "" {
				mi.SubtitleLanguages = append(mi.SubtitleLanguages, s.Language)
			}
		}
	}
	if !known {
		return nil
	}
	return mi
}

// dynamicRangeType returns the name the MediaInfo VideoDynamicRangeType
// token writes of a video range type, as Radarr's
// MediaInfoFormatter.FormatVideoDynamicRangeType; "" for SDR.
func dynamicRangeType(t core.VideoRangeType) string {
	switch t {
	case core.RangeTypeHDR10:
		return "HDR10"
	case core.RangeTypeHDR10Plus:
		return "HDR10Plus"
	case core.RangeTypeHLG:
		return "HLG"
	case core.RangeTypeDOVI, core.RangeTypeDOVIWithEL:
		return "DV"
	case core.RangeTypeDOVIWithHDR10:
		return "DV HDR10"
	case core.RangeTypeDOVIWithHDR10Plus, core.RangeTypeDOVIWithELHDR10Plus:
		return "DV HDR10Plus"
	case core.RangeTypeDOVIWithHLG:
		return "DV HLG"
	case core.RangeTypeDOVIWithSDR:
		return "DV SDR"
	}
	return ""
}

package planner

import (
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// HLSOutput names the files of an HLS transcode.
type HLSOutput struct {
	// Playlist is the media playlist path ffmpeg writes.
	Playlist string
	// Segments is the segment path pattern, e.g. "/tmp/x/seg%d.mp4".
	Segments string
	// Init is the fMP4 initialization segment's file name, relative to
	// the playlist.
	Init string
	// StartNumber is the number of the first segment written, for
	// transcodes that start at a seek position.
	StartNumber int
	// KeyframeChunks cuts copied video at every keyframe instead of after
	// each segment length, so that each file holds one group of pictures
	// and is numbered by its keyframe whatever position the transcode
	// started at; segments are then made of consecutive files.
	KeyframeChunks bool
}

func (p *Planner) commonArgs() []string {
	return []string{"-hide_banner", "-nostdin", "-loglevel", "error"}
}

// videoArgs returns the video codec, filter and bitstream options.
func (p *Planner) videoArgs(j *Job, keyframes bool) []string {
	if j.Video == nil || !j.isVideo() {
		return nil
	}
	encoder := p.VideoEncoder(j)
	if encoder == Copy {
		args := []string{"-codec:v:0", Copy}
		args = append(args, p.BitstreamArgs(j, core.StreamVideo)...)
		return append(args, p.copyTagArgs(j)...)
	}
	args := []string{"-codec:v:0", encoder}
	args = append(args, p.videoQualityArgs(j, encoder)...)
	if keyframes {
		args = append(args, p.keyframeArgs(j, encoder)...)
	}
	g := p.VideoFilters(j, encoder)
	subInput, subIndex := 0, 0
	if j.Subtitle != nil {
		subIndex = inFileIndex(j.Source, j.Subtitle)
		if p.externalSubtitleInput(j) {
			subInput = 1
		}
	}
	return append(args, g.Args(inFileIndex(j.Source, j.Video), subInput, subIndex)...)
}

// copyTagArgs tags copied video: Dolby Vision sample entries when the
// client takes Dolby Vision and the stream keeps it, which needs ffmpeg's
// experimental mode, and hvc1 rather than hev1 for other HEVC.
func (p *Planner) copyTagArgs(j *Job) []string {
	v := j.Video
	dv := IsDOVI(v) && containsFold(j.requestedRangeTypes(v.Codec), string(core.RangeTypeDOVI)) && !p.DOVIRemoved(j)
	switch {
	case isH265(v) && dv:
		return []string{"-tag:v:0", DolbyVisionHEVCTag(v), "-strict", "-2"}
	case isAV1(v) && dv:
		return []string{"-tag:v:0", "dav1", "-strict", "-2"}
	case isH265(v):
		return []string{"-tag:v:0", "hvc1"}
	}
	return nil
}

// DolbyVisionHEVCTag returns the sample entry tag of copied Dolby Vision
// HEVC: hvc1 for profile 8, whose base layer plays without Dolby Vision,
// dvh1 for the others.
func DolbyVisionHEVCTag(st *core.MediaStream) string {
	if st.DolbyVision != nil && st.DolbyVision.Profile == 8 {
		return "hvc1"
	}
	return "dvh1"
}

// streamArgs returns the maps and stream options shared by all outputs.
func (p *Planner) streamArgs(j *Job, keyframes bool, segmentContainer string) []string {
	var args []string
	g := VideoGraph{}
	if j.Video != nil && j.isVideo() && p.VideoEncoder(j) != Copy {
		g = p.VideoFilters(j, p.VideoEncoder(j))
	}
	maps := p.MapArgs(j, j.Video != nil)
	if len(g.Overlay) > 0 {
		// The complex graph feeds the video; the plain map would add it
		// again.
		maps = dropVideoMap(maps)
	}
	args = append(args, maps...)
	args = append(args, "-map_metadata", "-1", "-map_chapters", "-1")
	if p.Options.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(p.Options.Threads))
	}
	if len(p.hwDecodeArgs(j)) > 0 {
		// Hardware frames must not meet an automatically inserted software
		// scaler when the resolution changes.
		args = append(args, "-noautoscale")
	}
	args = append(args, p.videoArgs(j, keyframes)...)
	if j.Audio == nil {
		return args
	}
	encoder := p.AudioEncoder(j)
	if j.AudioCodec == Copy {
		encoder = Copy
	}
	args = append(args, p.audioArgs(j, encoder)...)
	if encoder == Copy {
		args = append(args, p.audioBitstreamArgs(j, segmentContainer, j.inputContainer())...)
	}
	return args
}

func dropVideoMap(maps []string) []string {
	if len(maps) >= 2 && maps[0] == "-map" {
		return maps[2:]
	}
	return maps
}

// HLSArgs returns the ffmpeg arguments writing the job as HLS segments.
// Segments are fMP4 (CMAF) unless the job asks for MPEG-TS; their
// timestamps continue from the start position so that seeking transcodes
// line up with the playlist.
func (p *Planner) HLSArgs(j *Job, out HLSOutput) []string {
	seg := j.SegmentContainer
	if seg == "" {
		seg = "mp4"
	}
	length := j.SegmentLength
	if length <= 0 {
		length = 6 * time.Second
	}
	args := p.commonArgs()
	args = append(args, p.inputArgs(j)...)
	args = append(args, p.streamArgs(j, true, seg)...)
	if j.Start > 0 {
		args = append(args, "-output_ts_offset", formatSeconds(j.Start))
	}
	hlsTime := formatSeconds(length)
	if out.KeyframeChunks {
		// Shorter than any group of pictures: every keyframe cuts.
		hlsTime = "0.001"
	}
	// Segments appear under their names once complete, and replace older
	// copies without truncating them for readers.
	args = append(args, "-max_muxing_queue_size", "2048", "-f", "hls", "-max_delay", "5000000",
		"-hls_time", hlsTime, "-hls_flags", "temp_file", "-hls_list_size", "0", "-hls_playlist_type", "vod",
		"-start_number", strconv.Itoa(out.StartNumber))
	if seg == "mp4" {
		init := out.Init
		if init == "" {
			init = "init.mp4"
		}
		args = append(args, "-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", init)
	} else {
		args = append(args, "-hls_segment_type", "mpegts")
	}
	return append(args, "-hls_segment_filename", out.Segments, "-y", out.Playlist)
}

// ProgressiveArgs returns the ffmpeg arguments writing a video job as one
// progressive file or stream; MP4 output is fragmented so that it can be
// streamed while written.
func (p *Planner) ProgressiveArgs(j *Job, output string) []string {
	if !j.isVideo() {
		return append(p.commonArgs(), p.ProgressiveAudioArgs(j, output)...)
	}
	args := p.commonArgs()
	args = append(args, p.inputArgs(j)...)
	args = append(args, p.streamArgs(j, false, "")...)
	switch j.Container {
	case "mp4", "m4v", "mov":
		args = append(args, "-movflags", "frag_keyframe+empty_moov+delay_moov", "-f", "mp4")
	case "mkv":
		args = append(args, "-f", "matroska")
	case "ts":
		args = append(args, "-f", "mpegts")
	case "":
	default:
		args = append(args, "-f", j.Container)
	}
	return append(args, "-y", output)
}

func formatSeconds(d time.Duration) string { return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) }

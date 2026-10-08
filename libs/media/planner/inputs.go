package planner

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/decision"
)

// burnsInSubtitle reports whether the subtitle is rendered into the video.
func (p *Planner) burnsInSubtitle(j *Job) bool {
	return j.SubtitleMethod == decision.SubtitleEncode || (j.AlwaysBurnIn && j.VideoCodec != Copy)
}

// externalSubtitleInput reports whether the subtitle file is an ffmpeg
// input: embedded into the output, or a graphical subtitle burned in.
func (p *Planner) externalSubtitleInput(j *Job) bool {
	s := j.Subtitle
	return s != nil && s.ExternalPath != "" &&
		(j.SubtitleMethod == decision.SubtitleEmbed || (p.burnsInSubtitle(j) && !s.IsTextSubtitle()))
}

// inFileIndex returns the index of a stream among the streams of the same
// file: the source for embedded streams, the sidecar file for external
// ones.
func inFileIndex(src *decision.Source, st *core.MediaStream) int {
	if src == nil || src.MediaSource == nil {
		return st.Index
	}
	n := 0
	for i := range src.Streams {
		cur := &src.Streams[i]
		if cur.Index == st.Index && cur.ExternalPath == st.ExternalPath && cur.Kind == st.Kind {
			return n
		}
		if cur.ExternalPath == st.ExternalPath {
			n++
		}
	}
	return -1
}

// MapArgs returns the -map options selecting the video, audio and subtitle
// streams from the inputs.
func (p *Planner) MapArgs(j *Job, inputIsVideo bool) []string {
	if j.Video == nil && j.Audio == nil {
		if inputIsVideo {
			return []string{"-sn"}
		}
		return nil
	}
	var args []string
	if j.Video != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(inFileIndex(j.Source, j.Video)))
	} else {
		args = append(args, "-vn")
	}
	if j.Audio != nil {
		input := 0
		if j.Audio.ExternalPath != "" {
			input = 1
			if p.externalSubtitleInput(j) {
				input = 2
			}
		}
		args = append(args, "-map", strconv.Itoa(input)+":"+strconv.Itoa(inFileIndex(j.Source, j.Audio)))
	} else {
		args = append(args, "-map", "-0:a")
	}
	s := j.Subtitle
	switch {
	case s == nil || j.SubtitleMethod == decision.SubtitleHLS:
		args = append(args, "-map", "-0:s")
	case j.SubtitleMethod == decision.SubtitleEmbed:
		input := "0"
		if s.ExternalPath != "" {
			// The selected sidecar file is input 1.
			input = "1"
		}
		args = append(args, "-map", input+":"+strconv.Itoa(inFileIndex(j.Source, s)))
	case s.ExternalPath != "" && !s.IsTextSubtitle():
		args = append(args, "-map", "1:"+strconv.Itoa(inFileIndex(j.Source, s)), "-sn")
	}
	return args
}

// subtitleInputPath returns the file ffmpeg reads a sidecar subtitle from:
// a VobSub .sub is read through its .idx when there is one.
func (p *Planner) subtitleInputPath(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".sub") {
		if idx := strings.TrimSuffix(path, filepath.Ext(path)) + ".idx"; p.exists(idx) {
			return idx
		}
	}
	return path
}

// seekArgs returns -ss for the start position.
func (j *Job) seekArgs() []string {
	if j.Start <= 0 {
		return nil
	}
	return []string{"-ss", formatTime(j.Start)}
}

// formatTime formats a duration as ffmpeg's HH:MM:SS.mmm.
func formatTime(d time.Duration) string {
	ms := d.Milliseconds()
	h, ms := ms/3_600_000, ms%3_600_000
	m, ms := ms/60_000, ms%60_000
	s, ms := ms/1000, ms%1000
	return pad2(h) + ":" + pad2(m) + ":" + pad2(s) + "." + strconv.FormatInt(1000+ms, 10)[1:]
}

func pad2(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// inputArgs returns the input options: hardware decoding, the source,
// and sidecar subtitle and audio files, each seeked to the start.
func (p *Planner) inputArgs(j *Job) []string {
	var args []string
	args = append(args, p.hwDecodeArgs(j)...)
	args = append(args, j.seekArgs()...)
	path := ""
	if j.Source != nil && j.Source.MediaSource != nil {
		path = j.Source.Path
	}
	args = append(args, "-i", inputURL(path, j.Source))
	if p.externalSubtitleInput(j) {
		args = append(args, j.seekArgs()...)
		args = append(args, "-i", "file:"+p.subtitleInputPath(j.Subtitle.ExternalPath))
	}
	if j.Audio != nil && j.Audio.ExternalPath != "" {
		args = append(args, j.seekArgs()...)
		args = append(args, "-i", "file:"+j.Audio.ExternalPath)
	}
	return args
}

// inputURL prefixes local paths with "file:" so that names with colons are
// not taken for protocols.
func inputURL(path string, src *decision.Source) string {
	if src != nil && src.Remote {
		return path
	}
	return "file:" + path
}

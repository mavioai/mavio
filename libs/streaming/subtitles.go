package streaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/mavioai/mavio/libs/core"
)

// emptyVTT is the WebVTT of a run that has written no cue yet.
const emptyVTT = "WEBVTT\n"

// subtitleFile is the WebVTT file of a subtitle stream written by a run.
func (s *Stream) subtitleFile(stream, run int) string {
	return filepath.Join(s.cfg.Dir, "t"+strconv.Itoa(stream)+"-"+strconv.Itoa(run)+".vtt")
}

// Subtitle returns the WebVTT of a subtitle stream that a run wrote while
// reading segment index, with the segments' timestamps: the source is read
// once, by the run producing the video, and never for its subtitles alone.
// It waits for the run to have read past the segment, starting or
// restarting ffmpeg as [Stream.Segment] does. The WebVTT holds the cues
// the run read, from where it started, and complete ones only.
func (s *Stream) Subtitle(ctx context.Context, stream, index int) ([]byte, error) {
	l := s.cfg.Layout
	if index < 0 || index >= len(l.Segments) ||
		!slices.ContainsFunc(s.cfg.Subtitles, func(st *core.MediaStream) bool { return st.Index == stream }) {
		return nil, ErrSegmentRange
	}
	if err := s.lock(ctx); err != nil {
		return nil, err
	}
	defer s.unlock()
	if s.closed {
		return nil, errors.New("streaming: stream closed")
	}
	// The newest run that read the segment.
	if s.job != nil && s.covers(s.job, index) {
		return s.readSubtitle(s.job, stream)
	}
	for _, j := range slices.Backward(s.past) {
		if s.covers(j, index) {
			return s.readSubtitle(j, stream)
		}
	}
	if s.restart(index) {
		if err := s.startAt(ctx, index); err != nil {
			return nil, err
		}
	} else {
		s.job.session.Touch()
	}
	j := s.job
	if err := s.wait(ctx, func() bool { return s.covers(j, index) }); err != nil {
		return nil, fmt.Errorf("subtitles of segment %d: %w", index, err)
	}
	return s.readSubtitle(j, stream)
}

// covers reports whether a run has read all of segment index's subtitles:
// it started at or before the segment and wrote the file after it, or it
// ended after the segment, or the segment is the last.
func (s *Stream) covers(j *job, index int) bool {
	l := s.cfg.Layout
	first, last := l.files(index)
	if j.first > first {
		return false
	}
	next := j.next
	if !j.stopped {
		next = s.ranTo(j)
	}
	ended := !j.stopped && s.job == j && s.ended() && j.session.Err() == nil
	return next > last+1 || (next > last && (ended || index == len(l.Segments)-1))
}

// ranTo returns the first file after those a run wrote in a row from its
// first: files written since it started, as an earlier run may have
// written the ones further on.
func (s *Stream) ranTo(j *job) int {
	n := j.first
	for {
		fi, err := os.Stat(s.file(n))
		if err != nil || fi.ModTime().Before(j.started) {
			return n
		}
		n++
	}
}

// readSubtitle reads the WebVTT a run wrote of a stream, up to its last
// complete cue.
func (s *Stream) readSubtitle(j *job, stream int) ([]byte, error) {
	data, err := os.ReadFile(s.subtitleFile(stream, j.run))
	if errors.Is(err, fs.ErrNotExist) {
		return []byte(emptyVTT), nil
	} else if err != nil {
		return nil, err
	}
	// Each cue ends with a line break once written whole.
	if !bytes.HasSuffix(data, []byte("\n")) {
		data = data[:bytes.LastIndex(data, []byte("\n\n"))+1]
	}
	if len(data) == 0 {
		return []byte(emptyVTT), nil
	}
	return data, nil
}

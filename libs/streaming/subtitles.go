package streaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// emptyVTT is the WebVTT of a run that has written no cue yet.
const emptyVTT = "WEBVTT\n"

// subtitleLag is how long after the video files a run's subtitles are
// taken to have caught up when no later cue shows it.
const subtitleLag = time.Second

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
	if s.job != nil && s.covers(s.job, stream, index) {
		return s.readSubtitle(s.job, stream)
	}
	for _, j := range slices.Backward(s.past) {
		if s.covers(j, stream, index) {
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
	if err := s.wait(ctx, func() bool { return s.covers(j, stream, index) }); err != nil {
		return nil, fmt.Errorf("subtitles of segment %d: %w", index, err)
	}
	return s.readSubtitle(j, stream)
}

// covers reports whether a run has written all of segment index's
// subtitles of a stream: it started at or before the segment and ended
// after it, or wrote two more video files after it and either a cue
// starting after the segment or those files a while ago. ffmpeg muxes
// each output in a thread of its own, so the subtitles may lag behind the
// video files, by far more than their duration when the video is copied.
func (s *Stream) covers(j *job, stream, index int) bool {
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
	if next > last && ended {
		return true
	}
	if next <= last+2 {
		return false
	}
	if end, ok := s.lastCue(j, stream); ok && end >= l.End(index) {
		return true
	}
	fi, err := os.Stat(s.file(last + 2))
	return err == nil && time.Since(fi.ModTime()) >= subtitleLag
}

// lastCue returns the start of the last cue a run wrote of a stream.
func (s *Stream) lastCue(j *job, stream int) (time.Duration, bool) {
	f, err := os.Open(s.subtitleFile(stream, j.run))
	if err != nil {
		return 0, false
	}
	defer f.Close()
	// The last cue is near the end.
	const tail = 4096
	if fi, err := f.Stat(); err == nil && fi.Size() > tail {
		if _, err := f.Seek(fi.Size()-tail, io.SeekStart); err != nil {
			return 0, false
		}
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return 0, false
	}
	i := strings.LastIndex(string(data), " --> ")
	if i < 0 {
		return 0, false
	}
	line := string(data[:i])
	return vttTime(line[strings.LastIndexByte(line, '\n')+1:])
}

// vttTime parses a WebVTT timestamp, [hh:]mm:ss.ttt.
func vttTime(v string) (time.Duration, bool) {
	clock, frac, ok := strings.Cut(v, ".")
	parts := strings.Split(clock, ":")
	if !ok || len(frac) != 3 || len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	ms, err := strconv.Atoi(frac)
	if err != nil {
		return 0, false
	}
	d := time.Duration(ms) * time.Millisecond
	unit := time.Second
	for _, p := range slices.Backward(parts) {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, false
		}
		d += time.Duration(n) * unit
		unit *= 60
	}
	return d, true
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

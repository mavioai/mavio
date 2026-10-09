package streaming

import (
	"errors"
	"path"
	"strings"
	"time"
)

// ErrNoSegments is returned for a zero segment length or duration.
var ErrNoSegments = errors.New("streaming: no segments")

// KeyframeSegments divides a stream at keyframes, the first keyframe at or
// after each multiple of the desired length, for copied video, which can
// only be cut there. A duration shorter than the last keyframe, which
// probes report for some files, is taken to end at that keyframe.
func KeyframeSegments(keyframes []time.Duration, duration, desired time.Duration) []time.Duration {
	if n := len(keyframes); n > 0 && duration < keyframes[n-1] {
		duration = keyframes[n-1]
	}
	var (
		segments []time.Duration
		last     time.Duration
		cut      = desired
	)
	for _, k := range keyframes {
		if k >= cut {
			segments = append(segments, k-last)
			last = k
			cut += desired
		}
	}
	if rest := duration - last; rest > 0 {
		segments = append(segments, rest)
	}
	return segments
}

// EqualSegments divides a duration into segments of the desired length and
// a shorter last one, for encoded video, whose keyframes are placed at the
// segment boundaries.
func EqualSegments(desired, duration time.Duration) ([]time.Duration, error) {
	if desired <= 0 || duration <= 0 {
		return nil, ErrNoSegments
	}
	n := int(duration / desired)
	segments := make([]time.Duration, n, n+1)
	for i := range segments {
		segments[i] = desired
	}
	if rest := duration % desired; rest != 0 {
		segments = append(segments, rest)
	}
	return segments, nil
}

// KeyframeExtractionAllowed reports whether keyframes may be read on
// demand from a file with this name: its extension, with or without the
// dot, is one of the allowed ones.
func KeyframeExtractionAllowed(file string, extensions []string) bool {
	ext := path.Ext(strings.ReplaceAll(file, `\`, "/"))
	if ext == "" {
		return false
	}
	for _, e := range extensions {
		if strings.EqualFold(ext[1:], strings.TrimPrefix(e, ".")) {
			return true
		}
	}
	return false
}

// Starts returns the start times of segments.
func Starts(segments []time.Duration) []time.Duration {
	starts := make([]time.Duration, len(segments))
	var t time.Duration
	for i, s := range segments {
		starts[i] = t
		t += s
	}
	return starts
}

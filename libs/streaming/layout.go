package streaming

import "time"

// Layout is how a media source divides into segments, and into the files
// ffmpeg writes them in.
type Layout struct {
	// Segments are the segment lengths.
	Segments []time.Duration
	starts   []time.Duration
	duration time.Duration
	// For copied video cut at keyframes, chunks start at the keyframes
	// (and at zero) and segment i is made of the chunks first[i] up to
	// first[i+1]; otherwise each segment is one file.
	chunks []time.Duration
	first  []int
}

// EncodedLayout divides encoded video or audio into segments of the
// desired length, each written as one file; the encoder places keyframes
// at the boundaries.
func EncodedLayout(desired, duration time.Duration) (Layout, error) {
	segments, err := EqualSegments(desired, duration)
	if err != nil {
		return Layout{}, err
	}
	return Layout{Segments: segments, starts: Starts(segments), duration: duration}, nil
}

// CopiedLayout divides copied video at its keyframes, as
// [KeyframeSegments] does. Each group of pictures is written as a file
// of its own, so that transcodes starting at any segment write the same
// files.
func CopiedLayout(keyframes []time.Duration, duration, desired time.Duration) Layout {
	segments := KeyframeSegments(keyframes, duration, desired)
	l := Layout{Segments: segments, starts: Starts(segments)}
	for _, s := range segments {
		l.duration += s
	}
	if len(keyframes) == 0 {
		return l
	}
	l.chunks = keyframes
	if keyframes[0] > 0 {
		// What precedes the first keyframe is a chunk of its own.
		l.chunks = append([]time.Duration{0}, keyframes...)
	}
	// Segment boundaries are keyframes; find their chunks.
	l.first = make([]int, len(segments)+1)
	c := 0
	for i, start := range l.starts {
		for c < len(l.chunks) && l.chunks[c] < start {
			c++
		}
		l.first[i] = c
	}
	l.first[len(segments)] = len(l.chunks)
	return l
}

// Chunked reports whether segments are made of keyframe chunks.
func (l Layout) Chunked() bool { return l.chunks != nil }

// Start returns the start time of segment i.
func (l Layout) Start(i int) time.Duration { return l.starts[i] }

// End returns the end time of segment i.
func (l Layout) End(i int) time.Duration { return l.starts[i] + l.Segments[i] }

// files returns the numbers of the files segment i is made of.
func (l Layout) files(i int) (first, last int) {
	if !l.Chunked() {
		return i, i
	}
	return l.first[i], l.first[i+1] - 1
}

// fileEnd returns the end time of file n.
func (l Layout) fileEnd(n int) time.Duration {
	if !l.Chunked() {
		return l.End(n)
	}
	if n+1 < len(l.chunks) {
		return l.chunks[n+1]
	}
	return l.duration
}

// seek returns where a transcode producing segment i starts reading, and
// the number of its first file. Copied video is read from within the
// segment's first group of pictures: ffmpeg seeks to the keyframe before
// the position, after moving it back by up to a few frames for streams
// with reordered frames, so the middle of the group is safe.
func (l Layout) seek(i int) (time.Duration, int) {
	if !l.Chunked() {
		return l.starts[i], i
	}
	c := l.first[i]
	if c == 0 {
		return 0, 0
	}
	return l.chunks[c] + (l.fileEnd(c)-l.chunks[c])/2, c
}

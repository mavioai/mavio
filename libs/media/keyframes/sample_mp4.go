package keyframes

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"time"
)

// Matroska elements of the stream an MP4 file's samples are written as.
const (
	mkvEBMLVersion        = 0x4286
	mkvEBMLReadVersion    = 0x42F7
	mkvEBMLMaxIDLength    = 0x42F2
	mkvEBMLMaxSizeLength  = 0x42F3
	mkvDocType            = 0x4282
	mkvDocTypeVersion     = 0x4287
	mkvDocTypeReadVersion = 0x4285
	mkvTrackUID           = 0x73C5
	mkvCodecID            = 0x86
	mkvCodecPrivate       = 0x63A2
	mkvVideo              = 0xE0
	mkvPixelWidth         = 0xB0
	mkvPixelHeight        = 0xBA
)

// mp4Codec returns the Matroska codec ID of the video of an MP4 sample
// entry, and the entry's box whose contents are its codec private data,
// empty for none; false for a codec not carried.
func mp4Codec(entry string) (id, private string, ok bool) {
	switch entry {
	case "avc1", "avc3":
		return "V_MPEG4/ISO/AVC", "avcC", true
	case "hvc1", "hev1", "dvh1", "dvhe":
		return "V_MPEGH/ISO/HEVC", "hvcC", true
	case "av01":
		return "V_AV1", "av1C", true
	case "vp09":
		return "V_VP9", "", true
	}
	return "", "", false
}

// MP4 is the index of the video track of an MP4 file read for sampling
// its keyframes: their times, and where each one's sample is.
type MP4 struct {
	// head is the start of the Matroska stream the samples are written
	// as: its EBML header, segment, info and track.
	head           []byte
	times          []time.Duration
	offsets, sizes []int64
}

// ReadMP4 reads the index of the first video track of an MP4 or QuickTime
// file from the sample tables of its moov, for [MP4.WriteSamples]. A
// fragmented file, whose fragments its index points to rather than its
// samples, and a codec Matroska does not carry its samples of the same
// way, give ErrNoIndex.
func ReadMP4(r io.ReaderAt, size int64) (*MP4, error) {
	t, d, err := readMP4(r, size)
	if err != nil {
		return nil, err
	}
	if t.sizes == nil || len(t.sizes) != len(d.Keyframes) {
		return nil, fmt.Errorf("mp4: keyframes without their samples: %w", ErrNoIndex)
	}
	track, err := t.trackEntry()
	if err != nil {
		return nil, err
	}
	m := &MP4{times: d.Keyframes, offsets: d.Offsets, sizes: t.sizes}
	sort.Sort(byTime{m})
	m.head = mp4Head(track)
	return m, nil
}

// trackEntry returns the contents of a Matroska TrackEntry of the track,
// from its first sample entry.
func (t *mp4Track) trackEntry() ([]byte, error) {
	stsd := first(boxes(t.stbl), "stsd")
	// The full box header and entry count precede the first entry.
	if len(stsd) < 16 {
		return nil, fmt.Errorf("mp4: no sample description: %w", ErrNoIndex)
	}
	entry := stsd[8:]
	size, kind := int(binary.BigEndian.Uint32(entry)), string(entry[4:8])
	codec, box, ok := mp4Codec(kind)
	if !ok {
		return nil, fmt.Errorf("mp4: %q video: %w", kind, ErrNoIndex)
	}
	// A visual sample entry holds 78 bytes of fields before its boxes,
	// its width and height 24 bytes in.
	if size < 8+78 || size > len(entry) {
		return nil, fmt.Errorf("mp4: bad %q sample entry: %w", kind, ErrNoIndex)
	}
	body := entry[8:size]
	width, height := binary.BigEndian.Uint16(body[24:]), binary.BigEndian.Uint16(body[26:])
	var private []byte
	if box != "" {
		if private = first(boxes(body[78:]), box); private == nil {
			return nil, fmt.Errorf("mp4: %q video without %s: %w", kind, box, ErrNoIndex)
		}
	}
	b := appendUint(nil, mkvTrackNumber, 1)
	b = appendUint(b, mkvTrackUID, 1)
	b = appendUint(b, mkvTrackType, mkvTrackTypeVideo)
	b = appendElement(b, mkvCodecID, []byte(codec))
	if private != nil {
		b = appendElement(b, mkvCodecPrivate, private)
	}
	video := appendUint(nil, mkvPixelWidth, uint64(width))
	video = appendUint(video, mkvPixelHeight, uint64(height))
	return appendElement(b, mkvVideo, video), nil
}

// mp4Head returns the start of a Matroska stream of a track, in
// millisecond ticks.
func mp4Head(track []byte) []byte {
	ebml := appendUint(nil, mkvEBMLVersion, 1)
	ebml = appendUint(ebml, mkvEBMLReadVersion, 1)
	ebml = appendUint(ebml, mkvEBMLMaxIDLength, 4)
	ebml = appendUint(ebml, mkvEBMLMaxSizeLength, 8)
	ebml = appendElement(ebml, mkvDocType, []byte("matroska"))
	ebml = appendUint(ebml, mkvDocTypeVersion, 4)
	ebml = appendUint(ebml, mkvDocTypeReadVersion, 2)
	b := appendElement(nil, mkvEBML, ebml)
	// The segment's size is unknown, as in a live stream.
	b = appendID(b, mkvSegment)
	b = append(b, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	b = appendElement(b, mkvInfo, appendUint(nil, mkvTimestampScale, mkvDefaultTSScale))
	return appendElement(b, mkvTracks, appendElement(nil, mkvTrackEntry, track))
}

// Keyframes returns the times of the sync samples.
func (m *MP4) Keyframes() []time.Duration { return m.times }

// WriteSamples writes to w a Matroska stream of the video track alone that
// holds, for each time in at, which must increase, the sync sample at or
// before it, presented at that time: r is the file of size bytes the index
// was read from.
func (m *MP4) WriteSamples(ctx context.Context, w io.Writer, r io.ReaderAt, size int64, at []time.Duration) error {
	return writeSamples(ctx, w, m.head, mkvDefaultTSScale, m.times, at, func(i int) ([]byte, error) {
		n := m.sizes[i]
		if n <= 0 || n > maxMatroskaElement || m.offsets[i]+n > size {
			return nil, fmt.Errorf("mp4: bad sample of %d bytes at %d", n, m.offsets[i])
		}
		// Track 1, at the cluster's time, a keyframe.
		b := make([]byte, 4+n)
		b[0], b[3] = 0x81, 0x80
		if _, err := r.ReadAt(b[4:], m.offsets[i]); err != nil {
			return nil, err
		}
		return b, nil
	})
}

// byTime sorts the samples of an MP4 index by time, as presentation times
// of keyframes in decode order need not increase.
type byTime struct{ m *MP4 }

func (s byTime) Len() int           { return len(s.m.times) }
func (s byTime) Less(i, j int) bool { return s.m.times[i] < s.m.times[j] }
func (s byTime) Swap(i, j int) {
	m := s.m
	m.times[i], m.times[j] = m.times[j], m.times[i]
	m.offsets[i], m.offsets[j] = m.offsets[j], m.offsets[i]
	m.sizes[i], m.sizes[j] = m.sizes[j], m.sizes[i]
}

// appendUint appends an unsigned integer element.
func appendUint(b []byte, id, v uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], v)
	return appendElement(b, id, data[:])
}

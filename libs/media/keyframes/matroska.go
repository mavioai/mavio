package keyframes

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

// Matroska element IDs, as encoded with their length marker.
const (
	mkvEBML            = 0x1A45DFA3
	mkvSegment         = 0x18538067
	mkvSeekHead        = 0x114D9B74
	mkvSeek            = 0x4DBB
	mkvSeekID          = 0x53AB
	mkvSeekPosition    = 0x53AC
	mkvInfo            = 0x1549A966
	mkvTimestampScale  = 0x2AD7B1
	mkvDuration        = 0x4489
	mkvTracks          = 0x1654AE6B
	mkvTrackEntry      = 0xAE
	mkvTrackNumber     = 0xD7
	mkvTrackType       = 0x83
	mkvCues            = 0x1C53BB6B
	mkvCuePoint        = 0xBB
	mkvCueTime         = 0xB3
	mkvCueTrackPos     = 0xB7
	mkvCueTrack        = 0xF7
	mkvCueClusterPos   = 0xF1
	mkvCueRelativePos  = 0xF0
	mkvCluster         = 0x1F43B675
	mkvTimestamp       = 0xE7
	mkvSimpleBlock     = 0xA3
	mkvBlockGroup      = 0xA0
	mkvBlock           = 0xA1
	mkvReferenceBlock  = 0xFB
	mkvTrackTypeVideo  = 1
	mkvDefaultTSScale  = 1000000
	maxMatroskaElement = 64 << 20
)

// element is an EBML element: its ID, and where its data starts and how
// long it is; size is -1 when unknown.
type element struct {
	id         uint64
	data, size int64
}

func (e element) end(limit int64) int64 {
	if e.size < 0 {
		return limit
	}
	return e.data + e.size
}

// readHeader reads the header of the element at off.
func readHeader(r io.ReaderAt, off int64) (element, error) {
	var b [12]byte
	n, err := r.ReadAt(b[:], off)
	if n == 0 {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return element{}, err
	}
	id, il := vint(b[:n], true)
	if il == 0 || il > 4 {
		return element{}, fmt.Errorf("bad EBML element ID at %d", off)
	}
	size, sl := vint(b[il:n], false)
	if sl == 0 {
		return element{}, fmt.Errorf("bad EBML element size at %d", off)
	}
	return element{id: uint64(id), data: off + int64(il+sl), size: size}, nil
}

// vint decodes an EBML variable-length integer and returns it with its
// length, 0 when malformed: an ID keeps its length marker; a size of all
// ones is unknown, -1.
func vint(b []byte, keepMarker bool) (int64, int) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0
	}
	l := 1
	for b[0]&(0x80>>(l-1)) == 0 {
		l++
	}
	if l > 8 || l > len(b) {
		return 0, 0
	}
	v := uint64(b[0])
	if !keepMarker {
		v &= uint64(0xFF >> l)
	}
	allOnes := v == uint64(0xFF>>l)
	for _, c := range b[1:l] {
		v = v<<8 | uint64(c)
		allOnes = allOnes && c == 0xFF
	}
	if !keepMarker && allOnes {
		return -1, l
	}
	return int64(v), l
}

// children returns the elements in data, an element's contents.
func children(data []byte) []elementData {
	var out []elementData
	for off := 0; off < len(data); {
		id, il := vint(data[off:], true)
		if il == 0 {
			break
		}
		size, sl := vint(data[off+il:], false)
		if sl == 0 || size < 0 || size > int64(len(data)-off-il-sl) {
			break
		}
		start := off + il + sl
		out = append(out, elementData{id: uint64(id), b: data[start : start+int(size)]})
		off = start + int(size)
	}
	return out
}

type elementData struct {
	id uint64
	b  []byte
}

func (e elementData) uint() uint64 {
	var v uint64
	for _, c := range e.b {
		v = v<<8 | uint64(c)
	}
	return v
}

func (e elementData) float() float64 {
	switch len(e.b) {
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(e.b)))
	case 8:
		return math.Float64frombits(binary.BigEndian.Uint64(e.b))
	}
	return 0
}

// readElement reads the contents of an element of known size.
func readElement(r io.ReaderAt, e element) ([]byte, error) {
	if e.size < 0 || e.size > maxMatroskaElement {
		return nil, fmt.Errorf("EBML element %#x of size %d", e.id, e.size)
	}
	b := make([]byte, e.size)
	if e.size == 0 {
		return b, nil
	}
	if _, err := r.ReadAt(b, e.data); err != nil {
		return nil, err
	}
	return b, nil
}

// Matroska is the index of a Matroska or WebM file: the elements a decoder
// of its first video track needs, and the cue points of that track.
type Matroska struct {
	// header holds the EBML header element; info the Info element's
	// contents and track the video TrackEntry's.
	header, info, track []byte
	// number is the video track's number; scale the nanoseconds of a
	// timestamp tick.
	number, scale uint64
	duration      time.Duration
	// Cues are the cue points of the video track, which point at its
	// keyframes.
	Cues []Cue
}

// Cue is a cue point of a Matroska video track.
type Cue struct {
	// Time is the keyframe's presentation time.
	Time time.Duration
	// cluster is the file offset of the cluster holding the keyframe;
	// block the offset of its block within the cluster's data, -1 when
	// the cue does not tell.
	cluster, block int64
}

// ReadMatroska reads the index of the first video track of a Matroska or
// WebM file from its Cues, found through the SeekHead, as Jellyfin's
// MatroskaKeyframeExtractor does; a SeekHead pointing to a second one is
// followed. It reads the file's headers and its index, not its clusters.
func ReadMatroska(r io.ReaderAt, size int64) (*Matroska, error) {
	ebml, err := readHeader(r, 0)
	if err != nil || ebml.id != mkvEBML || ebml.size < 0 {
		return nil, errNotMatroska
	}
	m := &Matroska{scale: mkvDefaultTSScale}
	if m.header, err = readRange(r, 0, ebml.end(size)); err != nil {
		return nil, err
	}
	seg, err := readHeader(r, ebml.end(size))
	if err != nil || seg.id != mkvSegment {
		return nil, fmt.Errorf("matroska: no segment: %w", ErrNoIndex)
	}
	segEnd := seg.end(size)
	// Seek positions are relative to the start of the segment's data.
	pos := map[uint64]int64{}
	var readSeekHead func(off int64, depth int) error
	readSeekHead = func(off int64, depth int) error {
		e, err := readHeader(r, off)
		if err != nil || e.id != mkvSeekHead {
			return fmt.Errorf("matroska: no SeekHead: %w", ErrNoIndex)
		}
		b, err := readElement(r, e)
		if err != nil {
			return err
		}
		var more []int64
		for _, s := range children(b) {
			if s.id != mkvSeek {
				continue
			}
			var id uint64
			p := int64(-1)
			for _, c := range children(s.b) {
				switch c.id {
				case mkvSeekID:
					id = c.uint()
				case mkvSeekPosition:
					p = int64(c.uint())
				}
			}
			switch {
			case p < 0:
			case id == mkvSeekHead && depth == 0 && seg.data+p != off:
				more = append(more, seg.data+p)
			case pos[id] == 0:
				pos[id] = seg.data + p
			}
		}
		for _, m := range more {
			if err := readSeekHead(m, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	// The SeekHead is the segment's first element, or follows a Void.
	head, err := readHeader(r, seg.data)
	if err != nil {
		return nil, err
	}
	at := seg.data
	if head.id != mkvSeekHead && head.size >= 0 {
		at = head.end(segEnd)
	}
	if err := readSeekHead(at, 0); err != nil {
		return nil, err
	}
	if pos[mkvInfo] == 0 || pos[mkvTracks] == 0 || pos[mkvCues] == 0 {
		return nil, fmt.Errorf("matroska: SeekHead lacks Info, Tracks or Cues: %w", ErrNoIndex)
	}
	read := func(id uint64) ([]byte, error) {
		e, err := readHeader(r, pos[id])
		if err != nil {
			return nil, err
		}
		if e.id != id {
			return nil, fmt.Errorf("matroska: element %#x at %d is not %#x: %w", e.id, pos[id], id, ErrNoIndex)
		}
		return readElement(r, e)
	}

	if m.info, err = read(mkvInfo); err != nil {
		return nil, err
	}
	duration := 0.0
	for _, c := range children(m.info) {
		switch c.id {
		case mkvTimestampScale:
			m.scale = c.uint()
		case mkvDuration:
			duration = c.float()
		}
	}
	m.duration = ticks(duration * float64(m.scale))
	tracks, err := read(mkvTracks)
	if err != nil {
		return nil, err
	}
	for _, t := range children(tracks) {
		if t.id != mkvTrackEntry {
			continue
		}
		var number, kind uint64
		for _, c := range children(t.b) {
			switch c.id {
			case mkvTrackNumber:
				number = c.uint()
			case mkvTrackType:
				kind = c.uint()
			}
		}
		if kind == mkvTrackTypeVideo {
			m.number, m.track = number, t.b
			break
		}
	}
	if m.number == 0 {
		return nil, fmt.Errorf("matroska: no video track: %w", ErrNoIndex)
	}
	cues, err := read(mkvCues)
	if err != nil {
		return nil, err
	}
	for _, cp := range children(cues) {
		if cp.id != mkvCuePoint {
			continue
		}
		cue := Cue{cluster: -1, block: -1}
		found := false
		for _, c := range children(cp.b) {
			switch c.id {
			case mkvCueTime:
				cue.Time = ticks(float64(c.uint()) * float64(m.scale))
			case mkvCueTrackPos:
				var track uint64
				cluster, block := int64(-1), int64(-1)
				for _, p := range children(c.b) {
					switch p.id {
					case mkvCueTrack:
						track = p.uint()
					case mkvCueClusterPos:
						cluster = seg.data + int64(p.uint())
					case mkvCueRelativePos:
						block = int64(p.uint())
					}
				}
				if track == m.number && !found {
					found, cue.cluster, cue.block = true, cluster, block
				}
			}
		}
		if found {
			m.Cues = append(m.Cues, cue)
		}
	}
	if len(m.Cues) == 0 {
		return nil, fmt.Errorf("matroska: no cues of the video track: %w", ErrNoIndex)
	}
	return m, nil
}

// readRange reads the bytes from off to end.
func readRange(r io.ReaderAt, off, end int64) ([]byte, error) {
	return readElement(r, element{data: off, size: end - off})
}

// matroska reads the keyframes of a Matroska or WebM file from its index.
func matroska(r io.ReaderAt, size int64) (Data, error) {
	m, err := ReadMatroska(r, size)
	if err != nil {
		return Data{}, err
	}
	d := Data{Duration: m.duration, Keyframes: make([]time.Duration, len(m.Cues))}
	for i, c := range m.Cues {
		d.Keyframes[i] = c.Time
	}
	return d, nil
}

// ticks rounds nanoseconds to microseconds, the precision ffprobe writes
// times with, so that index and packet times agree.
func ticks(ns float64) time.Duration {
	return time.Duration(math.RoundToEven(ns/float64(time.Microsecond))) * time.Microsecond
}

package keyframes

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"time"
)

// maxScannedBlocks bounds the elements of a cluster read to find a
// keyframe whose cue does not give its position.
const maxScannedBlocks = 256

// WriteSamples writes to w a Matroska stream of the video track alone that
// holds, for each time in at, which must increase, the keyframe at or
// before it, presented at that time. A decoder of the stream sees one
// picture a time, so that thumbnails at intervals take a read of a
// keyframe each instead of the whole file: r is the file of size bytes
// the index was read from.
func (m *Matroska) WriteSamples(ctx context.Context, w io.Writer, r io.ReaderAt, size int64, at []time.Duration) error {
	b := append([]byte(nil), m.header...)
	// The segment's size is unknown, as in a live stream.
	b = appendID(b, mkvSegment)
	b = append(b, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	b = appendElement(b, mkvInfo, m.info)
	b = appendElement(b, mkvTracks, appendElement(nil, mkvTrackEntry, m.track))
	if _, err := w.Write(b); err != nil {
		return err
	}
	last, block := -1, []byte(nil)
	for _, t := range at {
		if err := ctx.Err(); err != nil {
			return err
		}
		// The last cue at or before the time, else the first.
		i := max(0, sort.Search(len(m.Cues), func(i int) bool { return m.Cues[i].Time > t })-1)
		if i != last {
			var err error
			if block, err = m.keyframe(r, size, m.Cues[i]); err != nil {
				return fmt.Errorf("keyframe at %v: %w", m.Cues[i].Time, err)
			}
			last = i
		}
		ts := make([]byte, 8)
		binary.BigEndian.PutUint64(ts, uint64(t.Nanoseconds())/m.scale)
		cluster := appendElement(nil, mkvTimestamp, ts)
		cluster = appendElement(cluster, mkvSimpleBlock, block)
		if _, err := w.Write(appendElement(nil, mkvCluster, cluster)); err != nil {
			return err
		}
	}
	return nil
}

// keyframe reads the block of a cue's keyframe and returns it as the
// contents of a SimpleBlock at the start of its cluster.
func (m *Matroska) keyframe(r io.ReaderAt, size int64, c Cue) ([]byte, error) {
	if c.cluster < 0 {
		return nil, fmt.Errorf("matroska: cue without a cluster: %w", ErrNoIndex)
	}
	cl, err := readHeader(r, c.cluster)
	if err != nil {
		return nil, err
	}
	if cl.id != mkvCluster {
		return nil, fmt.Errorf("matroska: element %#x at %d is not a cluster: %w", cl.id, c.cluster, ErrNoIndex)
	}
	end := cl.end(size)
	if c.block >= 0 {
		if b, err := m.blockAt(r, cl.data+c.block, end); err == nil && b != nil {
			return b, nil
		}
	}
	off := cl.data
	for range maxScannedBlocks {
		if off >= end {
			break
		}
		e, err := readHeader(r, off)
		if err != nil {
			return nil, err
		}
		if e.size < 0 || e.id == mkvCluster {
			break
		}
		if b, err := m.blockAt(r, off, end); err != nil || b != nil {
			return b, err
		}
		off = e.end(end)
	}
	return nil, fmt.Errorf("matroska: no keyframe in the cluster at %d: %w", c.cluster, ErrNoIndex)
}

// blockAt reads the element at off when it is a keyframe block of the
// video track, nil otherwise.
func (m *Matroska) blockAt(r io.ReaderAt, off, end int64) ([]byte, error) {
	e, err := readHeader(r, off)
	if err != nil {
		return nil, err
	}
	if e.size < 0 || e.end(end) > end {
		return nil, nil
	}
	switch e.id {
	case mkvSimpleBlock:
		// The track number and flags precede the frame; a block of
		// another track is not read.
		var peek [12]byte
		n, _ := r.ReadAt(peek[:min(int64(len(peek)), e.size)], e.data)
		if track, l := vint(peek[:n], false); l == 0 || uint64(track) != m.number || n < l+3 || peek[l+2]&0x80 == 0 {
			return nil, nil
		}
		b, err := readElement(r, e)
		if err != nil {
			return nil, err
		}
		return m.simpleBlock(b, true), nil
	case mkvBlockGroup:
		b, err := readElement(r, e)
		if err != nil {
			return nil, err
		}
		var block []byte
		for _, c := range children(b) {
			switch c.id {
			case mkvBlock:
				block = c.b
			case mkvReferenceBlock:
				// A block referring to others is not a keyframe.
				return nil, nil
			}
		}
		return m.simpleBlock(block, false), nil
	}
	return nil, nil
}

// simpleBlock returns a copy of a block of the video track at the start of
// its cluster, flagged a keyframe; nil for a block of another track.
func (m *Matroska) simpleBlock(b []byte, simple bool) []byte {
	track, l := vint(b, false)
	if l == 0 || uint64(track) != m.number || len(b) < l+3 {
		return nil
	}
	out := append([]byte(nil), b...)
	out[l], out[l+1] = 0, 0
	flags := out[l+2]
	if !simple {
		// A Block's flags hold only the invisible and lacing bits.
		flags &= 0x0E
	}
	out[l+2] = flags | 0x80
	return out
}

// appendID appends an element ID, whose bytes carry their length.
func appendID(b []byte, id uint64) []byte {
	for i := (bits.Len64(id)+7)/8 - 1; i >= 0; i-- {
		b = append(b, byte(id>>(8*i)))
	}
	return b
}

// appendElement appends an element, its size written in eight bytes.
func appendElement(b []byte, id uint64, data []byte) []byte {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(data)))
	size[0] = 0x01
	b = append(appendID(b, id), size[:]...)
	return append(b, data...)
}

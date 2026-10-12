package keyframes

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

// maxMP4Box bounds the boxes read whole: moov, mfra and sidx.
const maxMP4Box = 256 << 20

// box is an MP4 box: its type, and where its data starts and how long it
// is.
type box struct {
	kind       string
	data, size int64
}

// readBoxHeader reads the header of the box at off, of a parent ending at
// end.
func readBoxHeader(r io.ReaderAt, off, end int64) (box, error) {
	var b [16]byte
	if _, err := r.ReadAt(b[:8], off); err != nil {
		return box{}, err
	}
	size, kind, hdr := int64(binary.BigEndian.Uint32(b[:4])), string(b[4:8]), int64(8)
	switch size {
	case 0:
		size = end - off
	case 1:
		if _, err := r.ReadAt(b[8:16], off+8); err != nil {
			return box{}, err
		}
		size, hdr = int64(binary.BigEndian.Uint64(b[8:16])), 16
	}
	if size < hdr || off+size > end {
		return box{}, fmt.Errorf("mp4: bad %q box at %d", kind, off)
	}
	return box{kind: kind, data: off + hdr, size: size - hdr}, nil
}

func readBox(r io.ReaderAt, b box) ([]byte, error) {
	if b.size > maxMP4Box {
		return nil, fmt.Errorf("mp4: %q box of %d bytes", b.kind, b.size)
	}
	buf := make([]byte, b.size)
	if _, err := r.ReadAt(buf, b.data); err != nil {
		return nil, err
	}
	return buf, nil
}

// boxes splits the contents of a box into its children; a malformed child
// ends the list.
func boxes(b []byte) map[string][][]byte {
	out := map[string][][]byte{}
	for len(b) >= 8 {
		size, hdr := uint64(binary.BigEndian.Uint32(b)), uint64(8)
		kind := string(b[4:8])
		switch size {
		case 0:
			size = uint64(len(b))
		case 1:
			if len(b) < 16 {
				return out
			}
			size, hdr = binary.BigEndian.Uint64(b[8:]), 16
		}
		if size < hdr || size > uint64(len(b)) {
			return out
		}
		out[kind] = append(out[kind], b[hdr:size])
		b = b[size:]
	}
	return out
}

// first returns the first child of a kind, nil when there is none.
func first(children map[string][][]byte, kind string) []byte {
	if c := children[kind]; len(c) > 0 {
		return c[0]
	}
	return nil
}

// path follows a path of first children.
func path(b []byte, kinds ...string) []byte {
	for _, k := range kinds {
		b = first(boxes(b), k)
		if b == nil {
			return nil
		}
	}
	return b
}

// fields reads big-endian unsigned integers off a full box's contents.
type fields struct {
	b   []byte
	err bool
}

func (f *fields) take(n int) []byte {
	if f.err || len(f.b) < n {
		f.err = true
		return make([]byte, n)
	}
	v := f.b[:n]
	f.b = f.b[n:]
	return v
}

func (f *fields) u32() uint64 { return uint64(binary.BigEndian.Uint32(f.take(4))) }
func (f *fields) u64() uint64 { return binary.BigEndian.Uint64(f.take(8)) }

// uv reads 64 bits in version 1 boxes and 32 otherwise.
func (f *fields) uv(version uint64) uint64 {
	if version == 1 {
		return f.u64()
	}
	return f.u32()
}

// n reads an unsigned integer of n bytes, 1 to 8.
func (f *fields) n(n int) uint64 {
	var v uint64
	for _, c := range f.take(n) {
		v = v<<8 | uint64(c)
	}
	return v
}

// header reads a full box's version and flags.
func (f *fields) header() (version, flags uint64) {
	v := f.u32()
	return v >> 24, v & 0xFFFFFF
}

// mp4Track is the video track of a movie.
type mp4Track struct {
	id, timescale, duration uint64
	stbl                    []byte
	elst                    []byte
}

// mp4 reads the keyframes of the first video track of an MP4 or QuickTime
// file: from the sync sample table of its moov, or for a fragmented file
// from its mfra index or a sidx. It reads the top-level box headers and
// those boxes, not the media data.
func mp4(r io.ReaderAt, size int64) (Data, error) {
	var (
		moov []byte
		sidx []box
	)
	for off := int64(0); off < size; {
		b, err := readBoxHeader(r, off, size)
		if err != nil {
			if off == 0 {
				return Data{}, ErrNoIndex
			}
			break
		}
		if off == 0 && b.kind != "ftyp" && b.kind != "moov" && b.kind != "free" && b.kind != "skip" && b.kind != "wide" {
			return Data{}, ErrNoIndex
		}
		switch b.kind {
		case "moov":
			if moov, err = readBox(r, b); err != nil {
				return Data{}, err
			}
		case "sidx":
			sidx = append(sidx, b)
		case "moof":
			// A fragmented file's index precedes its first fragment, or is
			// the mfra at its end; the fragments are not walked.
			if moov != nil {
				off = size
				continue
			}
		}
		off = b.data + b.size
	}
	if moov == nil {
		return Data{}, fmt.Errorf("mp4: no moov: %w", ErrNoIndex)
	}
	movie := boxes(moov)
	movieScale := uint64(0)
	if f := (fields{b: first(movie, "mvhd")}); f.b != nil {
		v, _ := f.header()
		f.uv(v)
		f.uv(v)
		movieScale = f.u32()
	}
	var t *mp4Track
	for _, trak := range movie["trak"] {
		children := boxes(trak)
		hdlr := path(trak, "mdia", "hdlr")
		if len(hdlr) < 12 || string(hdlr[8:12]) != "vide" {
			continue
		}
		t = &mp4Track{stbl: path(trak, "mdia", "minf", "stbl"), elst: path(trak, "edts", "elst")}
		f := fields{b: first(children, "tkhd")}
		v, _ := f.header()
		f.uv(v)
		f.uv(v)
		t.id = f.u32()
		f = fields{b: path(trak, "mdia", "mdhd")}
		v, _ = f.header()
		f.uv(v)
		f.uv(v)
		t.timescale = f.u32()
		t.duration = f.uv(v)
		if f.err || t.timescale == 0 {
			return Data{}, fmt.Errorf("mp4: bad video track: %w", ErrNoIndex)
		}
		break
	}
	if t == nil {
		return Data{}, fmt.Errorf("mp4: no video track: %w", ErrNoIndex)
	}
	if f := (fields{b: path(moov, "mvex", "mehd")}); t.duration == 0 && f.b != nil && movieScale > 0 {
		// A fragmented file may give its duration here, in the movie's
		// timescale.
		v, _ := f.header()
		t.duration = f.uv(v) * t.timescale / movieScale
	}
	if d, ok := t.samples(movieScale); ok {
		return d, nil
	}
	if d, ok, err := t.mfra(r, size); err != nil || ok {
		return d, err
	}
	for _, b := range sidx {
		buf, err := readBox(r, b)
		if err != nil {
			return Data{}, err
		}
		if d, ok := t.sidx(buf); ok {
			return d, nil
		}
	}
	return Data{}, fmt.Errorf("mp4: no index of the video track: %w", ErrNoIndex)
}

// at converts a time in the track's timescale to 100 ns ticks.
func (t *mp4Track) at(v int64) time.Duration {
	return ticks(float64(v) * float64(time.Second) / float64(t.timescale))
}

// shift returns the offset an edit list adds to the media's times: the
// empty edits before the media, less the media time it starts at.
func (t *mp4Track) shift(movieScale uint64) int64 {
	f := fields{b: t.elst}
	if f.b == nil {
		return 0
	}
	v, _ := f.header()
	var empty float64
	for range f.u32() {
		duration := f.uv(v)
		start := int64(f.uv(v))
		if v != 1 {
			start = int64(int32(start))
		}
		f.u32() // media rate
		if f.err {
			return 0
		}
		if start == -1 {
			if movieScale > 0 {
				empty += float64(duration) / float64(movieScale)
			}
			continue
		}
		return int64(math.Round(empty*float64(t.timescale))) - start
	}
	return 0
}

// samples reads the keyframes from the sample tables: the decode times of
// stts, offset by ctts and the edit list, of the samples stss lists, or of
// all samples without stss.
func (t *mp4Track) samples(movieScale uint64) (Data, bool) {
	tables := boxes(t.stbl)
	f := fields{b: first(tables, "stts")}
	f.header()
	n := f.u32()
	type run struct{ count, delta uint64 }
	var stts []run
	total := uint64(0)
	for range n {
		r := run{f.u32(), f.u32()}
		if f.err {
			return Data{}, false
		}
		stts = append(stts, r)
		total += r.count
	}
	if total == 0 {
		return Data{}, false
	}
	var sync []uint64
	if b := first(tables, "stss"); b != nil {
		f = fields{b: b}
		f.header()
		for range f.u32() {
			s := f.u32()
			if f.err {
				return Data{}, false
			}
			sync = append(sync, s)
		}
	}
	var ctts []run
	if b := first(tables, "ctts"); b != nil {
		f = fields{b: b}
		f.header()
		for range f.u32() {
			r := run{f.u32(), f.u32()}
			if f.err {
				return Data{}, false
			}
			ctts = append(ctts, r)
		}
	}
	shift := t.shift(movieScale)
	d := Data{Duration: t.at(int64(t.duration))}
	// Walks the samples once, in step with the sync samples.
	var (
		sample, dts  uint64 = 1, 0
		si, ci, next int
		sttsLeft     = stts[0].count
		cttsLeft     uint64
	)
	if len(ctts) > 0 {
		cttsLeft = ctts[0].count
	}
	for sample <= total {
		if sync == nil || (next < len(sync) && sync[next] == sample) {
			pts := int64(dts) + shift
			if ci < len(ctts) {
				// Version 0 offsets are read as signed, as ffmpeg does.
				pts += int64(int32(uint32(ctts[ci].delta)))
			}
			d.Keyframes = append(d.Keyframes, t.at(pts))
			next++
			if sync != nil && next == len(sync) {
				break
			}
		}
		dts += stts[si].delta
		if sttsLeft--; sttsLeft == 0 && si+1 < len(stts) {
			si++
			sttsLeft = stts[si].count
		}
		if ci < len(ctts) {
			if cttsLeft--; cttsLeft == 0 {
				if ci++; ci < len(ctts) {
					cttsLeft = ctts[ci].count
				}
			}
		}
		sample++
	}
	return d, len(d.Keyframes) > 0
}

// mfra reads the keyframes from the track's tfra in the movie fragment
// random access box at the end of the file, found through its mfro.
func (t *mp4Track) mfra(r io.ReaderAt, size int64) (Data, bool, error) {
	if size < 16 {
		return Data{}, false, nil
	}
	var b [16]byte
	if _, err := r.ReadAt(b[:], size-16); err != nil {
		return Data{}, false, err
	}
	if string(b[4:8]) != "mfro" {
		return Data{}, false, nil
	}
	start := size - int64(binary.BigEndian.Uint32(b[12:16]))
	if start < 0 {
		return Data{}, false, nil
	}
	hdr, err := readBoxHeader(r, start, size)
	if err != nil || hdr.kind != "mfra" {
		return Data{}, false, nil
	}
	buf, err := readBox(r, hdr)
	if err != nil {
		return Data{}, false, err
	}
	for _, tfra := range boxes(buf)["tfra"] {
		f := fields{b: tfra}
		v, _ := f.header()
		if f.u32() != t.id {
			continue
		}
		lengths := f.u32()
		sizes := []int{int(lengths>>4&3) + 1, int(lengths>>2&3) + 1, int(lengths&3) + 1}
		d := Data{Duration: t.at(int64(t.duration))}
		for range f.u32() {
			at := int64(f.uv(v))
			f.uv(v) // moof offset
			for _, s := range sizes {
				f.n(s)
			}
			if f.err {
				return Data{}, false, nil
			}
			d.Keyframes = append(d.Keyframes, t.at(at))
		}
		return d, len(d.Keyframes) > 0, nil
	}
	return Data{}, false, nil
}

// sidx reads the keyframes from a segment index of the track whose
// subsegments start with a stream access point, the keyframe.
func (t *mp4Track) sidx(b []byte) (Data, bool) {
	f := fields{b: b}
	v, _ := f.header()
	if f.u32() != t.id {
		return Data{}, false
	}
	scale := f.u32()
	at := f.uv(v)
	f.uv(v) // first offset
	f.take(2)
	n := f.n(2)
	if f.err || scale == 0 {
		return Data{}, false
	}
	ts := mp4Track{timescale: scale}
	d := Data{}
	for range n {
		ref := f.u32()
		duration := f.u32()
		sap := f.u32()
		if f.err || ref>>31 != 0 {
			// A hierarchical index is not followed.
			return Data{}, false
		}
		if sap>>31 == 1 {
			d.Keyframes = append(d.Keyframes, ts.at(int64(at+sap&0x0FFFFFFF)))
		}
		at += duration
	}
	d.Duration = ts.at(int64(at))
	return d, len(d.Keyframes) > 0
}

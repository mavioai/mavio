package rclone

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"sync"

	"github.com/mavioai/mavio/libs/core"
)

// Read-ahead of the reads through the block cache.
const (
	// playbackAhead is how many seconds of media playback reads ahead,
	// at most maxPlaybackAhead bytes; unknownPlaybackAhead bytes when
	// the bitrate is unknown.
	playbackAhead        = 30
	maxPlaybackAhead     = 256 << 20
	unknownPlaybackAhead = 32 << 20
	// anyAhead is how far reads nothing is known of read ahead.
	anyAhead = 16 << 20
	// probeHead and probeTail are what a probe of a media file is
	// fetched ahead of it: ffprobe reads the head, and the tail where
	// the index of many files is.
	probeHead = 1 << 20
	probeTail = 256 << 10
)

// aheadOf returns how far sequential reads made for intent read ahead.
func aheadOf(intent core.ReadIntent) int64 {
	switch intent.Kind {
	case core.ReadPlayback:
		if intent.Bitrate > 0 {
			return min(intent.Bitrate/8*playbackAhead, maxPlaybackAhead)
		}
		return unknownPlaybackAhead
	case core.ReadProbe, core.ReadBackground:
		return 0
	}
	return anyAhead
}

// itemReader reads an item for one user, as io.ReaderAt.
type itemReader struct {
	ctx   context.Context
	it    *blockItem
	cur   *cursor
	ahead int64
}

func (r itemReader) ReadAt(p []byte, off int64) (int, error) {
	return r.it.ReadAt(r.ctx, r.cur, p, off, r.ahead)
}

// cachedFile is an object of a remote read through the block cache:
// Read reads ahead as the intent of its Open asks, ReadAt fetches what it
// reads, rounded up.
type cachedFile struct {
	ctx   context.Context
	it    *blockItem
	cur   *cursor
	info  fs.FileInfo
	ahead int64

	mu     sync.Mutex
	off    int64
	closed bool
}

func (f *cachedFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *cachedFile) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, &fs.PathError{Op: "readat", Path: f.info.Name(), Err: fs.ErrInvalid}
	}
	n, err := f.it.ReadAt(f.ctx, f.cur, p, off, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		err = &fs.PathError{Op: "read", Path: f.info.Name(), Err: err}
	}
	return n, err
}

func (f *cachedFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, fs.ErrClosed
	}
	n, err := f.it.ReadAt(f.ctx, f.cur, p, f.off, f.ahead)
	f.off += int64(n)
	if errors.Is(err, io.EOF) && n > 0 {
		err = nil
	}
	if err != nil && !errors.Is(err, io.EOF) {
		err = &fs.PathError{Op: "read", Path: f.info.Name(), Err: err}
	}
	return n, err
}

func (f *cachedFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.off
	case io.SeekEnd:
		offset += f.info.Size()
	default:
		return 0, fs.ErrInvalid
	}
	if offset < 0 {
		return 0, &fs.PathError{Op: "seek", Path: f.info.Name(), Err: fs.ErrInvalid}
	}
	f.off = offset
	return offset, nil
}

// Prefetch implements core.Prefetcher.
func (f *cachedFile) Prefetch(off, end int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.it.prefetch(f.cur, off, end)
	}
}

func (f *cachedFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		f.it.detach(f.cur)
		f.it.release()
	}
	return nil
}

var (
	_ core.File       = (*cachedFile)(nil)
	_ core.Prefetcher = (*cachedFile)(nil)
)

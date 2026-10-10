package storage

import (
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"sync"
	"time"
	"unsafe"
)

// DefaultHeartbeatInterval is how often a volume kept awake is read, as
// khuaplayer's keep-alive does: well within the minutes after which disks
// spin down and network shares disconnect.
const DefaultHeartbeatInterval = 2500 * time.Millisecond

// heartbeatBlock is the size and alignment of a heartbeat read, a page,
// which unbuffered reads on every platform accept.
const heartbeatBlock = 4096

// VolumeHeartbeat reads 4 KB of the file at path, at a random aligned
// offset and past the page cache, so that the read reaches the disk or
// the share and keeps it awake.
func VolumeHeartbeat(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("heartbeat open %s: %w", path, err)
	}
	fi, err := f.Stat()
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("heartbeat stat %s: %w", path, err)
	}
	size := fi.Size()
	if size <= 0 || !fi.Mode().IsRegular() {
		return nil
	}
	var offset int64
	if blocks := size / heartbeatBlock; blocks > 1 {
		offset = rand.Int64N(blocks-1) * heartbeatBlock
	}
	if err := uncachedRead(path, offset, alignedBlock()); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("heartbeat read %s: %w", path, err)
	}
	return nil
}

// alignedBlock returns a heartbeatBlock-sized buffer aligned to its size,
// as unbuffered reads require.
func alignedBlock() []byte {
	b := make([]byte, 2*heartbeatBlock)
	off := int(uintptr(unsafe.Pointer(&b[0])) & (heartbeatBlock - 1))
	if off != 0 {
		off = heartbeatBlock - off
	}
	return b[off : off+heartbeatBlock]
}

// Heartbeats spaces the heartbeats of each volume: however many
// playbacks or clients want a volume awake, it is read once an interval.
type Heartbeats struct {
	// Interval defaults to DefaultHeartbeatInterval.
	Interval time.Duration

	mu   sync.Mutex
	last map[string]time.Time
}

// Claim reports whether volume is due a heartbeat at now, recording it
// when it is.
func (h *Heartbeats) Claim(volume string, now time.Time) bool {
	interval := h.Interval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if last, ok := h.last[volume]; ok && now.Sub(last) < interval {
		return false
	}
	if h.last == nil {
		h.last = map[string]time.Time{}
	}
	// Forget volumes not read for a while, so the map stays small.
	for k, t := range h.last {
		if now.Sub(t) > time.Hour {
			delete(h.last, k)
		}
	}
	h.last[volume] = now
	return true
}

package subtitle

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// hashChunk is the size of the head and tail FileHash reads.
const hashChunk = 64 << 10

// FileHash returns the OpenSubtitles hash of a video file of the given
// size: the size plus the 64-bit little-endian words of its first and
// last 64 KiB, wrapping around, in 16 lower-case hexadecimal digits.
// Files smaller than 64 KiB have none.
func FileHash(r io.ReaderAt, size int64) (string, error) {
	if size < hashChunk {
		return "", errors.New("subtitle: file too small to hash")
	}
	sum := uint64(size)
	buf := make([]byte, hashChunk)
	for _, off := range []int64{0, size - hashChunk} {
		if _, err := r.ReadAt(buf, off); err != nil {
			return "", fmt.Errorf("subtitle: hash: %w", err)
		}
		for i := 0; i < hashChunk; i += 8 {
			sum += binary.LittleEndian.Uint64(buf[i:])
		}
	}
	return fmt.Sprintf("%016x", sum), nil
}

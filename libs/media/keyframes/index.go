package keyframes

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNoIndex reports a file whose keyframes are not in an index of its
// container, which leaves reading every packet, [Extractor.Extract].
var ErrNoIndex = errors.New("no keyframe index")

// errNotMatroska reports a file without an EBML header.
var errNotMatroska = fmt.Errorf("not matroska: %w", ErrNoIndex)

// FromIndex reads the keyframes of the first video stream of a Matroska,
// WebM, MP4 or QuickTime file of size bytes from its container's index:
// Matroska Cues, or the sync samples, mfra or sidx of MP4. It reads a few
// elements or boxes, a small part of the file, which suits storage where
// whole-file reads are costly; other files give ErrNoIndex. The duration
// is zero when the index does not give it, as in a fragmented MP4 file
// without mehd.
func FromIndex(r io.ReaderAt, size int64) (Data, error) {
	d, err := matroska(r, size)
	if errors.Is(err, errNotMatroska) {
		return mp4(r, size)
	}
	return d, err
}

// FromFile reads the keyframes of the file at path with [FromIndex].
func FromFile(path string) (Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return Data{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return Data{}, err
	}
	d, err := FromIndex(f, fi.Size())
	if err != nil {
		return Data{}, fmt.Errorf("keyframe index of %s: %w", path, err)
	}
	return d, nil
}

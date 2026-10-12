package providers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Keyframes reads keyframes from container indexes, and with ffprobe from
// files without one, as a library.KeyframeExtractor.
type Keyframes struct {
	Extractor *keyframes.Extractor
}

// Keyframes returns the keyframe times of the file's first video stream:
// from its index, or when whole is set, with ffprobe.
func (k Keyframes) Keyframes(ctx context.Context, f library.MediaFile, whole bool) ([]time.Duration, bool, error) {
	d, err := index(ctx, f)
	if err == nil {
		return d.Keyframes, true, nil
	}
	if !whole {
		if errors.Is(err, keyframes.ErrNoIndex) {
			return nil, false, nil
		}
		return nil, false, err
	}
	// A malformed index is no index: ffprobe reads the packets.
	d, err = k.Extractor.Extract(ctx, f.URL)
	if err != nil {
		return nil, false, err
	}
	return d.Keyframes, true, nil
}

// index reads the keyframes of a file's container index.
func index(ctx context.Context, f library.MediaFile) (keyframes.Data, error) {
	if f.Local != "" {
		return keyframes.FromFile(f.Local)
	}
	file, err := f.Open(ctx)
	if err != nil {
		return keyframes.Data{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return keyframes.Data{}, err
	}
	d, err := keyframes.FromIndex(file, info.Size())
	if err != nil {
		return keyframes.Data{}, fmt.Errorf("keyframe index of %s: %w", f.Path, err)
	}
	return d, nil
}

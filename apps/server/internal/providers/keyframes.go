package providers

import (
	"context"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Keyframes reads keyframes from container indexes, and with ffprobe from
// files without one, as a library.KeyframeExtractor.
type Keyframes struct {
	Extractor *keyframes.Extractor
}

// Keyframes returns the keyframe times of the file's first video stream:
// from its index, or when whole is set, with ffprobe.
func (k Keyframes) Keyframes(ctx context.Context, path string, whole bool) ([]time.Duration, bool, error) {
	d, err := keyframes.FromFile(path)
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
	d, err = k.Extractor.Extract(ctx, path)
	if err != nil {
		return nil, false, err
	}
	return d.Keyframes, true, nil
}

package providers

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Keyframes extracts keyframes with ffprobe as a
// library.KeyframeExtractor.
type Keyframes struct {
	Extractor *keyframes.Extractor
}

// Keyframes returns the keyframe times of the file's first video stream.
func (k Keyframes) Keyframes(ctx context.Context, path string) ([]time.Duration, error) {
	d, err := k.Extractor.Extract(ctx, path)
	if err != nil {
		return nil, err
	}
	return d.Keyframes, nil
}

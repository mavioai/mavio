package providers

import (
	"context"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/borders"
	"github.com/mavioai/mavio/libs/media/keyframes"
)

// Borders finds black borders with ffmpeg, as a library.BorderDetector.
type Borders struct {
	Detector *borders.Detector
}

// Borders returns the black borders of a video file's frames: keyframes
// read through its index, or when seek is set and it has none, frames
// seeked to.
func (b Borders) Borders(ctx context.Context, f library.MediaFile, duration time.Duration, width, height int, seek bool) (core.Crop, bool, error) {
	in := borders.Input{URL: f.URL, Open: func() (keyframes.File, error) { return f.Open(ctx) }}
	crop, err := b.Detector.Detect(ctx, in, duration, width, height, seek)
	if !seek && errors.Is(err, keyframes.ErrNoIndex) {
		return core.Crop{}, false, nil
	}
	return crop, err == nil, err
}

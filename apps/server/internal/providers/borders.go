package providers

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/borders"
)

// Borders finds black borders with ffmpeg, as a library.BorderDetector.
type Borders struct {
	Detector *borders.Detector
}

// Borders returns the black borders of a video file's frames.
func (b Borders) Borders(ctx context.Context, f library.MediaFile, duration time.Duration, width, height int) (core.Crop, error) {
	return b.Detector.Detect(ctx, f.URL, duration, width, height)
}

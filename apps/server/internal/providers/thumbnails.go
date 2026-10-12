package providers

import (
	"context"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/thumbnails"
)

// ChapterImageWidth is the width of chapter images.
const ChapterImageWidth = 640

// Thumbnails makes trickplay sheets and chapter images with ffmpeg as a
// library.Thumbnailer.
type Thumbnails struct {
	Maker   *thumbnails.Maker
	Options thumbnails.Options
}

func video(v library.VideoFrame) thumbnails.Video {
	return thumbnails.Video{Duration: v.Duration, Width: v.Width, Height: v.Height, Crop: v.Crop}
}

// Trickplay writes a video's sheets into dir: through its index, or when
// whole is set and it has none, from the whole file.
func (t Thumbnails) Trickplay(ctx context.Context, path string, v library.VideoFrame, dir string, whole bool) (core.Trickplay, bool, error) {
	tp, err := t.Maker.Trickplay(ctx, path, video(v), t.Options, dir, whole)
	if !whole && errors.Is(err, keyframes.ErrNoIndex) {
		return core.Trickplay{}, false, nil
	}
	return tp, err == nil, err
}

// ChapterImage writes the frame at a time into file.
func (t Thumbnails) ChapterImage(ctx context.Context, path string, at time.Duration, v library.VideoFrame, file string) error {
	return t.Maker.ChapterImage(ctx, path, at, video(v), ChapterImageWidth, file)
}

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
func (t Thumbnails) Trickplay(ctx context.Context, f library.MediaFile, v library.VideoFrame, dir string, whole bool) (core.Trickplay, bool, error) {
	in := thumbnails.Input{URL: f.URL, Open: func() (thumbnails.File, error) { return f.Open(ctx) }}
	tp, err := t.Maker.Trickplay(ctx, in, video(v), t.Options, dir, whole)
	if !whole && errors.Is(err, keyframes.ErrNoIndex) {
		return core.Trickplay{}, false, nil
	}
	return tp, err == nil, err
}

// ChapterImage writes the frame at a time into file.
func (t Thumbnails) ChapterImage(ctx context.Context, f library.MediaFile, at time.Duration, v library.VideoFrame, file string) error {
	return t.Maker.ChapterImage(ctx, f.URL, at, video(v), ChapterImageWidth, file)
}

// Loudness measures the integrated loudness of tracks with ffmpeg, as a
// library.LoudnessMeter.
type Loudness struct {
	Maker *thumbnails.Maker
}

// Loudness measures a track's first audio stream, in LUFS.
func (l Loudness) Loudness(ctx context.Context, f library.MediaFile) (float64, error) {
	return l.Maker.Loudness(ctx, f.URL)
}

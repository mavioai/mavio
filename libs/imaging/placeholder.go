package imaging

import (
	"bytes"
	"fmt"
	"image"
	"math"

	"github.com/bbrks/go-blurhash"
	"go.n16f.net/thumbhash"
)

// Placeholder describes an image and the compact previews clients show
// while it loads.
type Placeholder struct {
	Width, Height int
	Blurhash      string
	Thumbhash     []byte
}

// Placeholders decodes an image (as Process does) and computes its
// blurhash, on a thumbnail of at most 32 pixels, and its thumbhash, on one
// of at most 100 pixels.
func Placeholders(data []byte) (Placeholder, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Placeholder{}, fmt.Errorf("imaging: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return Placeholder{}, fmt.Errorf("%w: %d×%d", ErrTooLarge, cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Placeholder{}, fmt.Errorf("imaging: %w", err)
	}
	b := img.Bounds()
	p := Placeholder{Width: b.Dx(), Height: b.Dy()}
	x, y := BlurhashComponents(p.Width, p.Height)
	small := thumbnail(img, 32)
	if p.Blurhash, err = blurhash.Encode(x, y, small); err != nil {
		return Placeholder{}, fmt.Errorf("imaging: blurhash: %w", err)
	}
	p.Thumbhash = thumbhash.EncodeImage(thumbnail(img, 100))
	return p, nil
}

// BlurhashComponents picks the blurhash components as Jellyfin does: tiles
// as close to square as possible, mostly under 16 of them, at most 9 per
// axis.
func BlurhashComponents(width, height int) (x, y int) {
	xf := float32(math.Sqrt(float64(16 * float32(width) / float32(height))))
	yf := xf * float32(height) / float32(width)
	return min(int(xf)+1, 9), min(int(yf)+1, 9)
}

// thumbnail scales img to fit in a square of the given side.
func thumbnail(img image.Image, side int) image.Image {
	b := img.Bounds()
	s := ScaleDownToFit(Size{b.Dx(), b.Dy()}, Size{side, side})
	if s.Width < 1 || s.Height < 1 {
		return img
	}
	return ResizeImage(img, s.Width, s.Height)
}

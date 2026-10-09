package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // registers the GIF decoder
	"image/jpeg"
	"image/png"

	_ "golang.org/x/image/bmp"  // registers the BMP decoder
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

// MaxPixels bounds the images Process decodes, against decompression
// bombs.
const MaxPixels = 100_000_000

// ErrTooLarge is returned for images of more than MaxPixels.
var ErrTooLarge = errors.New("imaging: image too large")

// Options are how Process renders an image.
type Options struct {
	SizeOptions
	// Quality is the JPEG quality, 1–100; 0 means 90.
	Quality int
}

// Process decodes a JPEG, PNG, GIF, BMP or WebP image, resizes it as the
// options ask (see NewSize) and encodes it as JPEG, or as PNG when it has
// transparency. GIFs keep only their first frame.
func Process(data []byte, o Options) ([]byte, Format, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("imaging: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, "", fmt.Errorf("%w: %d×%d", ErrTooLarge, cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("imaging: %w", err)
	}
	b := img.Bounds()
	size := NewSize(o.SizeOptions, Size{Width: b.Dx(), Height: b.Dy()})
	if size.Width > 0 && size.Height > 0 {
		img = ResizeImage(img, size.Width, size.Height)
	}
	var out bytes.Buffer
	if !opaque(img) {
		if err := png.Encode(&out, img); err != nil {
			return nil, "", fmt.Errorf("imaging: %w", err)
		}
		return out.Bytes(), PNG, nil
	}
	quality := o.Quality
	if quality <= 0 || quality > 100 {
		quality = 90
	}
	if err := jpeg.Encode(&out, flatten(img), &jpeg.Options{Quality: quality}); err != nil {
		return nil, "", fmt.Errorf("imaging: %w", err)
	}
	return out.Bytes(), JPEG, nil
}

// opaque reports whether an image has no transparent pixels.
func opaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

// flatten returns an opaque image as one the JPEG encoder takes without
// conversion.
func flatten(img image.Image) image.Image {
	switch img.(type) {
	case *image.YCbCr, *image.Gray, *image.RGBA:
		return img
	}
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
	return dst
}

package imaging

import (
	"image"

	"golang.org/x/image/draw"
)

// The light 3×3 sharpening kernel applied after downscaling, in float32
// as in Jellyfin.
const (
	sharpenCenter   float32 = 1.4
	sharpenNeighbor float32 = -0.1
)

// ResizeImage scales img to width × height. An image of that size is
// returned as is; upscaling uses Catmull–Rom interpolation; downscaling
// uses bilinear interpolation followed by a light sharpening, as in
// Jellyfin.
func ResizeImage(img image.Image, width, height int) image.Image {
	b := img.Bounds()
	if b.Dx() == width && b.Dy() == height {
		return img
	}
	downscale := b.Dx() > width || b.Dy() > height
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	scaler := draw.Interpolator(draw.CatmullRom)
	if downscale {
		scaler = draw.BiLinear
	}
	scaler.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	if downscale {
		Sharpen(dst)
	}
	return dst
}

// Sharpen applies the light sharpening kernel to img in place: each channel
// becomes 1.4 × itself − 0.1 × its four neighbors, with edge pixels
// standing in for neighbors outside the image. Images other than RGBA and
// NRGBA are left unchanged.
func Sharpen(img image.Image) {
	var pix []uint8
	var stride int
	var r image.Rectangle
	switch m := img.(type) {
	case *image.RGBA:
		pix, stride, r = m.Pix, m.Stride, m.Rect
	case *image.NRGBA:
		pix, stride, r = m.Pix, m.Stride, m.Rect
	default:
		return
	}
	w, h := r.Dx(), r.Dy()
	if w == 0 || h == 0 {
		return
	}
	src := make([]uint8, len(pix))
	copy(src, pix)
	for y := range h {
		row := y * stride
		up, down := row-stride, row+stride
		if y == 0 {
			up = row
		}
		if y == h-1 {
			down = row
		}
		for x := range w {
			col := x * 4
			left, right := col-4, col+4
			if x == 0 {
				left = col
			}
			if x == w-1 {
				right = col
			}
			for c := range 4 {
				v := sharpenCenter*float32(src[row+col+c]) + sharpenNeighbor*(float32(src[up+col+c])+
					float32(src[down+col+c])+float32(src[row+left+c])+float32(src[row+right+c]))
				pix[row+col+c] = uint8(min(max(int(v+0.5), 0), 255))
			}
		}
	}
}

package imaging

import (
	"errors"
	"image"
	"image/draw"
	"math"
)

// CollageImages is the most images a collage shows.
const CollageImages = 4

// ErrNoImages is returned for a collage without a decodable image.
var ErrNoImages = errors.New("imaging: no images for a collage")

// Collage composes up to CollageImages images into one of width × height,
// as Jellyfin's dynamic images of libraries, collections and playlists:
// side by side when the collage is at least 1.4 times as wide as high,
// else in a 2 × 2 grid. Each image covers its cell, scaled and cropped
// around its centre rather than stretched; fewer images repeat to fill
// the cells. Images that do not decode are skipped.
func Collage(sources [][]byte, width, height int) (image.Image, error) {
	var images []image.Image
	for _, data := range sources {
		if len(images) == CollageImages {
			break
		}
		if img, err := decode(data); err == nil {
			images = append(images, img)
		}
	}
	if len(images) == 0 || width <= 0 || height <= 0 {
		return nil, ErrNoImages
	}
	columns, rows := 2, 2
	if float64(width)/float64(height) >= 1.4 {
		columns, rows = CollageImages, 1
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := range columns * rows {
		col, row := i%columns, i/columns
		cell := image.Rect(col*width/columns, row*height/rows, (col+1)*width/columns, (row+1)*height/rows)
		cover(out, cell, images[i%len(images)])
	}
	return out, nil
}

// cover draws img into cell, scaled to cover it and centred.
func cover(dst draw.Image, cell image.Rectangle, img image.Image) {
	b := img.Bounds()
	scale := math.Max(float64(cell.Dx())/float64(b.Dx()), float64(cell.Dy())/float64(b.Dy()))
	w := max(cell.Dx(), int(math.Ceil(float64(b.Dx())*scale)))
	h := max(cell.Dy(), int(math.Ceil(float64(b.Dy())*scale)))
	scaled := ResizeImage(img, w, h)
	offset := scaled.Bounds().Min.Add(image.Pt((w-cell.Dx())/2, (h-cell.Dy())/2))
	draw.Draw(dst, cell, scaled, offset, draw.Src)
}

package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// stripes encodes a PNG of equal vertical stripes, one per color.
func stripes(t *testing.T, width, height int, colors ...color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.SetRGBA(x, y, colors[x*len(colors)/width])
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCollage(t *testing.T) {
	red, green, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}
	near := func(got color.Color, want color.RGBA) bool {
		r, g, b, _ := got.RGBA()
		d := func(a uint32, b uint8) bool { return int(a>>8)-int(b) < 24 && int(b)-int(a>>8) < 24 }
		return d(r, want.R) && d(g, want.G) && d(b, want.B)
	}
	images := [][]byte{stripes(t, 200, 300, red), []byte("not an image"), stripes(t, 100, 100, green), stripes(t, 300, 200, blue)}

	// Wide: side by side, the first image repeated in the fourth cell.
	wide, err := Collage(images, 400, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []color.RGBA{red, green, blue, red} {
		if got := wide.At(i*100+50, 50); !near(got, want) {
			t.Errorf("wide cell %d = %v, want %v", i, got, want)
		}
	}
	// Square: a 2 × 2 grid, row by row.
	square, err := Collage(images, 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range []image.Point{{50, 50}, {150, 50}, {50, 150}, {150, 150}} {
		if got, want := square.At(p.X, p.Y), []color.RGBA{red, green, blue, red}[i]; !near(got, want) {
			t.Errorf("square cell %d = %v, want %v", i, got, want)
		}
	}
	// A wide image covers a square cell with its middle, not squeezed.
	striped, err := Collage([][]byte{stripes(t, 300, 100, red, green, blue)}, 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []int{5, 50, 95} {
		if got := striped.At(x, 50); !near(got, green) {
			t.Errorf("cropped cell at x=%d = %v, want green", x, got)
		}
	}
	if b := striped.Bounds(); b.Dx() != 200 || b.Dy() != 200 {
		t.Errorf("bounds = %v", b)
	}

	if _, err := Collage([][]byte{[]byte("x")}, 100, 100); !errors.Is(err, ErrNoImages) {
		t.Errorf("no decodable images: %v, want ErrNoImages", err)
	}
}

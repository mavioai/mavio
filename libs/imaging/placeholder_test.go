package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/bbrks/go-blurhash"
	"go.n16f.net/thumbhash"
)

func TestBlurhashComponents(t *testing.T) {
	tests := []struct{ w, h, x, y int }{
		{400, 600, 4, 5},  // poster
		{500, 500, 5, 5},  // square
		{1000, 100, 9, 2}, // wide banner, capped at 9
		{100, 1000, 2, 9},
	}
	for _, tt := range tests {
		if x, y := BlurhashComponents(tt.w, tt.h); x != tt.x || y != tt.y {
			t.Errorf("BlurhashComponents(%d, %d) = %d, %d, want = %d, %d", tt.w, tt.h, x, y, tt.x, tt.y)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	// An orange poster.
	img := image.NewRGBA(image.Rect(0, 0, 400, 600))
	for y := range 600 {
		for x := range 400 {
			img.Set(x, y, color.RGBA{R: 240, G: 120, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p, err := Placeholders(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 400 || p.Height != 600 || p.Blurhash == "" || len(p.Thumbhash) == 0 {
		t.Fatalf("Placeholders = %+v", p)
	}
	x, y, err := blurhash.Components(p.Blurhash)
	if err != nil || x != 4 || y != 5 {
		t.Errorf("blurhash components = %d, %d, %v; want = 4, 5", x, y, err)
	}
	near := func(what string, c color.Color) {
		r, g, b, _ := c.RGBA()
		if d := absDiff(r>>8, 240) + absDiff(g>>8, 120) + absDiff(b>>8, 20); d > 30 {
			t.Errorf("%s color = %d,%d,%d, want about 240,120,20", what, r>>8, g>>8, b>>8)
		}
	}
	decoded, err := blurhash.Decode(p.Blurhash, 8, 12, 1)
	if err != nil {
		t.Fatal(err)
	}
	near("blurhash", decoded.At(4, 6))
	thumb, err := thumbhash.DecodeImage(p.Thumbhash)
	if err != nil {
		t.Fatal(err)
	}
	b := thumb.Bounds()
	if b.Dx() >= b.Dy() {
		t.Errorf("thumbhash image %v, want portrait", b)
	}
	near("thumbhash", thumb.At(b.Dx()/2, b.Dy()/2))

	if _, err := Placeholders([]byte("nope")); err == nil {
		t.Error("Placeholders(garbage) = nil error")
	}
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

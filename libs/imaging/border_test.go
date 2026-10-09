package imaging

import (
	"image"
	"image/color"
	"testing"
)

func TestDetectBlackBorders_Letterbox(t *testing.T) {
	// 400x200 image with 30px top and 30px bottom black bars (content from y=30 to 169)
	w, h := 400, 200
	topBar, botBar := 30, 30

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if y >= topBar && y < h-botBar {
				img.SetRGBA(x, y, color.RGBA{R: 200, G: 180, B: 160, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 5, G: 5, B: 5, A: 255})
			}
		}
	}

	borders, ok := DetectBlackBorders(img, 20)
	if !ok {
		t.Fatalf("expected black borders to be detected")
	}
	if borders.Top != topBar {
		t.Errorf("Top: got = %d, want = %d", borders.Top, topBar)
	}
	if borders.Bottom != botBar {
		t.Errorf("Bottom: got = %d, want = %d", borders.Bottom, botBar)
	}
	if borders.Left != 0 || borders.Right != 0 {
		t.Errorf("Left/Right: got = (%d, %d), want = (0, 0)", borders.Left, borders.Right)
	}

	cropped, cropBorders, croppedOk := CropBlackBorders(img, 20)
	if !croppedOk || cropBorders != borders {
		t.Fatalf("CropBlackBorders: got ok=%v borders=%+v", croppedOk, cropBorders)
	}
	cb := cropped.Bounds()
	if cb.Dy() != h-topBar-botBar || cb.Dx() != w {
		t.Errorf("cropped dimensions: got = %dx%d, want = %dx%d", cb.Dx(), cb.Dy(), w, h-topBar-botBar)
	}
}

func TestDetectBlackBorders_Pillarbox(t *testing.T) {
	// 400x200 image with 40px left and 40px right black bars
	w, h := 400, 200
	leftBar, rightBar := 40, 40

	gray := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= leftBar && x < w-rightBar {
				gray.SetGray(x, y, color.Gray{Y: 220})
			} else {
				gray.SetGray(x, y, color.Gray{Y: 10})
			}
		}
	}

	borders, ok := DetectBlackBorders(gray, 20)
	if !ok {
		t.Fatalf("expected pillarbox black borders to be detected")
	}
	if borders.Left != leftBar {
		t.Errorf("Left: got = %d, want = %d", borders.Left, leftBar)
	}
	if borders.Right != rightBar {
		t.Errorf("Right: got = %d, want = %d", borders.Right, rightBar)
	}
	if borders.Top != 0 || borders.Bottom != 0 {
		t.Errorf("Top/Bottom: got = (%d, %d), want = (0, 0)", borders.Top, borders.Bottom)
	}
}

func TestDetectBlackBorders_YCbCr(t *testing.T) {
	w, h := 320, 240
	topBar, botBar := 25, 25

	ycbcr := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := ycbcr.YOffset(x, y)
			if y >= topBar && y < h-botBar {
				ycbcr.Y[off] = 180
			} else {
				ycbcr.Y[off] = 16 // standard studio black
			}
		}
	}

	borders, ok := DetectBlackBorders(ycbcr, 20)
	if !ok {
		t.Fatalf("expected YCbCr black borders to be detected")
	}
	if borders.Top != topBar || borders.Bottom != botBar {
		t.Errorf("Top/Bottom: got = (%d, %d), want = (%d, %d)", borders.Top, borders.Bottom, topBar, botBar)
	}
}

func TestDetectBlackBorders_AllBlack(t *testing.T) {
	// Fade to black or all black image should NOT be cropped
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	borders, ok := DetectBlackBorders(img, 20)
	if ok || !borders.IsZero() {
		t.Errorf("all black image should not report borders: got = %+v, ok = %v", borders, ok)
	}
}

func TestDetectBlackBorders_NoBorders(t *testing.T) {
	// Full bright image
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}
	borders, ok := DetectBlackBorders(img, 20)
	if ok || !borders.IsZero() {
		t.Errorf("solid bright image should not report borders: got = %+v, ok = %v", borders, ok)
	}
}

func TestDetectBlackBordersP010(t *testing.T) {
	w, h := 200, 100
	topBar := 15
	stride := w * 2
	base := make([]byte, h*stride)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := y*stride + x*2
			var val uint16
			if y >= topBar {
				val = 700 << 6 // 10-bit bright
			} else {
				val = 16 << 6 // 10-bit black
			}
			base[pos] = byte(val & 0xff)
			base[pos+1] = byte(val >> 8)
		}
	}

	borders, ok := DetectBlackBordersP010(base, w, h, stride, 20)
	if !ok {
		t.Fatalf("expected P010 black borders to be detected")
	}
	if borders.Top != topBar {
		t.Errorf("P010 Top: got = %d, want = %d", borders.Top, topBar)
	}
}

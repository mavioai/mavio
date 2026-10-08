package imaging

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"
	"testing/iotest"

	xdraw "golang.org/x/image/draw"
)

func TestPortedDrawingUtils(t *testing.T) {
	portedCases(t, "drawing/drawing_utils.json", ported{run: map[string]func(*testing.T, args){
		"ScaleDownToFit_Bounds_WithoutUpscaling": func(t *testing.T, a args) {
			n := func(name string) int {
				var v int
				if _, err := fmt.Sscan(string(a[name]), &v); err != nil {
					t.Fatalf("argument %s: %v", name, err)
				}
				return v
			}
			got := ScaleDownToFit(Size{n("width"), n("height")}, Size{n("boxWidth"), n("boxHeight")})
			if want := (Size{n("expectedWidth"), n("expectedHeight")}); got != want {
				t.Errorf("got = %v, want = %v", got, want)
			}
		},
	}})
}

func TestPortedImageHelper(t *testing.T) {
	portedCases(t, "drawing/image_helper.json", ported{facts: map[string]string{
		"GetNewImageSize_ExplicitSizeLargerThanSource_ClampsToSource": "TestNewSize",
		"GetNewImageSize_WidthLargerThanSource_ClampsToSource":        "TestNewSize",
		"GetNewImageSize_FillLargerThanSource_ClampsToSource":         "TestNewSize",
		"GetNewImageSize_SmallerThanSource_StillDownscales":           "TestNewSize",
		"GetNewImageSize_NoSizeRequested_ReturnsSource":               "TestNewSize",
	}})
}

func TestNewSize(t *testing.T) {
	source := Size{600, 336}
	for _, tt := range []struct {
		o    SizeOptions
		want Size
	}{
		// An explicit size larger than the source is clamped to it
		// (jellyfin/jellyfin#17056).
		{SizeOptions{Width: 23100, Height: 23100}, Size{336, 336}},
		{SizeOptions{Width: 10000}, Size{600, 336}},
		{SizeOptions{FillWidth: 23100, FillHeight: 23100}, Size{600, 336}},
		{SizeOptions{MaxWidth: 300}, Size{300, 168}},
		{SizeOptions{}, Size{600, 336}},
		// Fill covers the requested box.
		{SizeOptions{FillWidth: 300, FillHeight: 300}, Size{536, 300}},
	} {
		if got := NewSize(tt.o, source); got != tt.want {
			t.Errorf("NewSize(%+v): got = %v, want = %v", tt.o, got, tt.want)
		}
	}
}

func TestPortedImageFormatExtensions(t *testing.T) {
	invalid := func(t *testing.T, a args) {
		f := Format(a["format"])
		if _, ok := f.MimeType(); ok {
			t.Errorf("MimeType(%q): got = ok", f)
		}
		if _, ok := f.Extension(); ok {
			t.Errorf("Extension(%q): got = ok", f)
		}
	}
	portedCases(t, "drawing/image_format_extensions.json", ported{
		run: map[string]func(*testing.T, args){
			"GetMimeType_Valid_ThrowsInvalidEnumArgumentException":  invalid,
			"GetExtension_Valid_ThrowsInvalidEnumArgumentException": invalid,
		},
		facts: map[string]string{
			"GetMimeType_Valid_Valid":  "TestFormats",
			"GetExtension_Valid_Valid": "TestFormats",
		},
	})
}

func TestFormats(t *testing.T) {
	for _, f := range Formats {
		mime, ok1 := f.MimeType()
		ext, ok2 := f.Extension()
		if !ok1 || !ok2 || !strings.HasPrefix(mime, "image/") || ext != "."+string(f) {
			t.Errorf("%s: got = %q %v, %q %v", f, mime, ok1, ext, ok2)
		}
	}
}

func filled(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	return img
}

func TestPortedSharpen(t *testing.T) {
	portedCases(t, "skia_encoder_sharpen.json", ported{facts: map[string]string{
		"SharpenInPlace_UniformImage_IsUnchanged":                 "TestSharpen",
		"SharpenInPlace_BrightPixelOnDarkBackground_SharpensEdge": "TestSharpen",
		"SharpenInPlace_EdgePixels_ClampOutOfBoundsTaps":          "TestSharpen",
		"SharpenInPlace_UnsupportedColorType_IsLeftUntouched":     "TestSharpen",
	}})
}

func checkPixel(t *testing.T, img *image.RGBA, x, y int, want color.RGBA) {
	t.Helper()
	if got := img.RGBAAt(x, y); got != want {
		t.Errorf("pixel (%d,%d): got = %v, want = %v", x, y, got, want)
	}
}

func TestSharpen(t *testing.T) {
	// 1.4v − 4 × 0.1v = v for a uniform image.
	img := filled(8, 8, color.RGBA{100, 150, 200, 255})
	Sharpen(img)
	for y := range 8 {
		for x := range 8 {
			checkPixel(t, img, x, y, color.RGBA{100, 150, 200, 255})
		}
	}

	img = filled(5, 5, color.RGBA{50, 50, 50, 255})
	img.SetRGBA(2, 2, color.RGBA{250, 250, 250, 255})
	Sharpen(img)
	checkPixel(t, img, 2, 2, color.RGBA{255, 255, 255, 255}) // 1.4 × 250 − 0.1 × 4 × 50 = 330, clamped
	checkPixel(t, img, 1, 2, color.RGBA{30, 30, 30, 255})    // 1.4 × 50 − 0.1 × (250 + 3 × 50)
	checkPixel(t, img, 0, 0, color.RGBA{50, 50, 50, 255})    // only background around

	// A corner pixel stands in for its two neighbors outside the image.
	img = filled(3, 3, color.RGBA{100, 100, 100, 255})
	img.SetRGBA(0, 0, color.RGBA{200, 200, 200, 255})
	Sharpen(img)
	checkPixel(t, img, 0, 0, color.RGBA{220, 220, 220, 255}) // 1.4 × 200 − 0.1 × (200 + 200 + 100 + 100)

	gray := image.NewGray(image.Rect(0, 0, 4, 4))
	draw.Draw(gray, gray.Bounds(), &image.Uniform{color.Gray{80}}, image.Point{}, draw.Src)
	Sharpen(gray)
	if got := gray.GrayAt(1, 1).Y; got != 80 {
		t.Errorf("gray: got = %d, want = 80", got)
	}
}

func TestPortedResize(t *testing.T) {
	portedCases(t, "skia_encoder_resize.json", ported{facts: map[string]string{
		"ResizeImage_MatchingDimensions_ReturnsTheImageUntouched": "TestResizeImage",
		"ResizeImage_Upscale_DoesNotSharpen":                      "TestResizeImage",
		"ResizeImage_Downscale_StillSharpens":                     "TestResizeImage",
	}})
}

// edgeImage has a vertical edge in the middle.
func edgeImage(w, h int) *image.RGBA {
	img := filled(w, h, color.RGBA{40, 60, 80, 255})
	draw.Draw(img, image.Rect(0, 0, w/2, h), &image.Uniform{color.RGBA{220, 210, 200, 255}}, image.Point{}, draw.Src)
	return img
}

// TestResizeImage ports Jellyfin's resize tests: Skia's resampling cannot be
// reproduced, so expected images are built with the same interpolation.
func TestResizeImage(t *testing.T) {
	src := edgeImage(16, 16)
	if got := ResizeImage(src, 16, 16); got != image.Image(src) {
		t.Error("same size: got = a new image, want = the source")
	}

	// Upscaling interpolates without sharpening.
	src = edgeImage(8, 8)
	up := ResizeImage(src, 24, 24).(*image.RGBA)
	want := image.NewRGBA(image.Rect(0, 0, 24, 24))
	xdrawCatmullRom(want, src)
	if !equalPixels(up, want) {
		t.Error("upscale: got = sharpened or differently scaled pixels")
	}

	// Downscaling sharpens.
	src = edgeImage(32, 32)
	down := ResizeImage(src, 16, 16).(*image.RGBA)
	plain := image.NewRGBA(image.Rect(0, 0, 16, 16))
	xdrawBiLinear(plain, src)
	sharpened := image.NewRGBA(plain.Rect)
	copy(sharpened.Pix, plain.Pix)
	Sharpen(sharpened)
	if !equalPixels(down, sharpened) {
		t.Error("downscale: got = unsharpened pixels")
	}
	if plain.RGBAAt(8, 8) == sharpened.RGBAAt(8, 8) {
		t.Error("the edge must be something sharpening changes")
	}
}

func equalPixels(a, b *image.RGBA) bool { return a.Rect == b.Rect && string(a.Pix) == string(b.Pix) }

func TestPortedSvgSecurityValidator(t *testing.T) {
	portedCases(t, "svg_security_validator.json", ported{facts: map[string]string{
		"IsSafe_MissingFile_ReturnsFalse":        "TestCheckSVG",
		"IsSafe_ExternalReference_ReturnsFalse":  "TestCheckSVG",
		"IsSafe_NoExternalReference_ReturnsTrue": "TestCheckSVG",
	}})
}

func TestCheckSVG(t *testing.T) {
	for i, svg := range unsafeSVGs {
		if err := CheckSVG(strings.NewReader(svg)); !errors.Is(err, ErrUnsafeSVG) {
			t.Errorf("unsafe %d: got = %v, want = ErrUnsafeSVG\n%s", i, err, svg)
		}
	}
	for i, svg := range safeSVGs {
		if err := CheckSVG(strings.NewReader(svg)); err != nil {
			t.Errorf("safe %d: got = %v\n%s", i, err, svg)
		}
	}
	// A file that cannot be read is not safe.
	if err := CheckSVG(iotest.ErrReader(errors.New("no such file"))); !errors.Is(err, ErrUnsafeSVG) {
		t.Errorf("unreadable: got = %v, want = ErrUnsafeSVG", err)
	}
}

func TestPortedTrickplayManager(t *testing.T) {
	portedCases(t, "trickplay/trickplay_manager.json", ported{skip: map[string]string{
		"DeleteTrickplayDataAsync_DisposesDbContext": "tests the disposal of an Entity Framework context when deleting trickplay rows; storage is libs/store's and has no such context",
	}})
}

func xdrawCatmullRom(dst *image.RGBA, src image.Image) {
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
}

func xdrawBiLinear(dst *image.RGBA, src image.Image) {
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
}

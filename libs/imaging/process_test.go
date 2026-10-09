package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encoded(t *testing.T, w, h int, alpha uint8, format Format) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 128, A: alpha})
		}
	}
	var buf bytes.Buffer
	var err error
	if format == PNG {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		o          Options
		wantFormat Format
		wantSize   Size
	}{
		{"poster to max width", encoded(t, 400, 600, 255, JPEG), Options{SizeOptions: SizeOptions{MaxWidth: 200}}, JPEG, Size{200, 300}},
		{"opaque PNG becomes JPEG", encoded(t, 100, 50, 255, PNG), Options{}, JPEG, Size{100, 50}},
		{"transparent PNG stays PNG", encoded(t, 100, 50, 10, PNG), Options{SizeOptions: SizeOptions{Height: 25}}, PNG, Size{50, 25}},
		{"never enlarged", encoded(t, 100, 50, 255, JPEG), Options{SizeOptions: SizeOptions{Width: 400}}, JPEG, Size{100, 50}},
	}
	for _, tt := range tests {
		out, format, err := Process(tt.data, tt.o)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		cfg, name, err := image.DecodeConfig(bytes.NewReader(out))
		if err != nil || format != tt.wantFormat || (Size{cfg.Width, cfg.Height}) != tt.wantSize || (name == "png") != (format == PNG) {
			t.Errorf("%s: got = %s %d×%d (%s, %v), want = %s %v", tt.name, format, cfg.Width, cfg.Height, name, err, tt.wantFormat, tt.wantSize)
		}
	}

	if _, _, err := Process([]byte("not an image"), Options{}); err == nil {
		t.Error("Process(garbage) = nil error")
	}
	// The size is checked before decoding: a PNG header claiming 200
	// megapixels is refused.
	if _, _, err := Process(pngHeader(20000, 10000), Options{}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Process(200 megapixels) = %v, want ErrTooLarge", err)
	}
}

// pngHeader returns the start of a PNG file of the given size: its
// signature and IHDR chunk.
func pngHeader(w, h uint32) []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 0, 0, 0, 0) // 8-bit grayscale
	chunk := append([]byte("IHDR"), ihdr...)
	out := append([]byte("\x89PNG\r\n\x1a\n"), binary.BigEndian.AppendUint32(nil, uint32(len(ihdr)))...)
	out = append(out, chunk...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

package zimuku

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/bits"
)

// maxPixels bounds the captcha images decoded.
const maxPixels = 1 << 22

var errBMP = errors.New("bmp: unsupported or malformed image")

// bmpToPNG converts the site's BMP captcha image to an RGB PNG, as
// Bazarr's bmp_to_image with Pillow.
func bmpToPNG(data []byte) ([]byte, error) {
	img, err := decodeBMP(data)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, fmt.Errorf("png: %w", err)
	}
	return b.Bytes(), nil
}

// decodeBMP decodes an uncompressed BMP of 1, 4, 8, 16, 24 or 32 bits a
// pixel, with bit fields or not, as an opaque image.
func decodeBMP(data []byte) (*image.RGBA, error) {
	if len(data) < 26 || data[0] != 'B' || data[1] != 'M' {
		return nil, errBMP
	}
	le := binary.LittleEndian
	offset := int(le.Uint32(data[10:]))
	dib := int(le.Uint32(data[14:]))
	var (
		width, height int
		bpp           int
		compression   uint32
		colors        int
		entry         = 4
	)
	switch {
	case dib == 12:
		width, height = int(int16(le.Uint16(data[18:]))), int(int16(le.Uint16(data[20:])))
		bpp = int(le.Uint16(data[24:]))
		entry = 3
	case dib >= 40 && len(data) >= 54:
		width, height = int(int32(le.Uint32(data[18:]))), int(int32(le.Uint32(data[22:])))
		bpp = int(le.Uint16(data[28:]))
		compression = le.Uint32(data[30:])
		colors = int(le.Uint32(data[46:]))
	default:
		return nil, errBMP
	}
	topDown := height < 0
	if topDown {
		height = -height
	}
	if width <= 0 || height <= 0 || width*height > maxPixels {
		return nil, errBMP
	}

	// The palette, or the masks of bit fields.
	var palette []color.RGBA
	masks := [3]uint32{0x7c00, 0x3e0, 0x1f}
	if bpp == 24 || bpp == 32 {
		masks = [3]uint32{0xff0000, 0xff00, 0xff}
	}
	switch {
	case bpp <= 8 && compression == 0:
		if colors == 0 || colors > 1<<bpp {
			colors = 1 << bpp
		}
		start := 14 + dib
		for i := range colors {
			at := start + i*entry
			if at+3 > len(data) {
				break
			}
			palette = append(palette, color.RGBA{R: data[at+2], G: data[at+1], B: data[at], A: 0xff})
		}
	case (bpp == 16 || bpp == 32) && compression == 3:
		if len(data) < 66 {
			return nil, errBMP
		}
		for i := range masks {
			masks[i] = le.Uint32(data[54+i*4:])
		}
	case (bpp == 16 || bpp == 24 || bpp == 32) && compression == 0:
	default:
		return nil, errBMP
	}

	stride := (width*bpp + 31) / 32 * 4
	if offset < 0 || offset+stride*height > len(data) {
		return nil, errBMP
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		row := data[offset+stride*y:]
		dy := height - 1 - y
		if topDown {
			dy = y
		}
		for x := range width {
			var c color.RGBA
			switch bpp {
			case 1, 2, 4, 8:
				bit := x * bpp
				i := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1)
				if i < len(palette) {
					c = palette[i]
				}
			case 16:
				c = fields(uint32(le.Uint16(row[x*2:])), masks)
			case 24:
				c = color.RGBA{R: row[x*3+2], G: row[x*3+1], B: row[x*3]}
			case 32:
				c = fields(le.Uint32(row[x*4:]), masks)
			}
			c.A = 0xff
			img.SetRGBA(x, dy, c)
		}
	}
	return img, nil
}

// fields reads a pixel's colors by their bit masks.
func fields(v uint32, masks [3]uint32) color.RGBA {
	var out [3]uint8
	for i, m := range masks {
		if m == 0 {
			continue
		}
		shift := bits.TrailingZeros32(m)
		width := bits.OnesCount32(m)
		c := uint64((v & m) >> shift)
		top := uint64(1)<<width - 1
		out[i] = uint8(c * 255 / top)
	}
	return color.RGBA{R: out[0], G: out[1], B: out[2]}
}

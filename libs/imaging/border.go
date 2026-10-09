package imaging

import (
	"image"
	"image/color"
	"image/draw"
)

// DefaultBlackThreshold is the default luminance threshold (0–255) below which
// a pixel or sampled line is considered black. In narrow-range video, studio black
// is 16; setting 20 tolerates minor compression artifacts in black bars.
const DefaultBlackThreshold uint8 = 20

// BlackBorders holds detected border widths in pixels along each edge.
type BlackBorders struct {
	Top    int
	Bottom int
	Left   int
	Right  int
}

// IsZero reports whether no black borders were detected.
func (b BlackBorders) IsZero() bool {
	return b.Top == 0 && b.Bottom == 0 && b.Left == 0 && b.Right == 0
}

// Total returns the sum of all border depths.
func (b BlackBorders) Total() int {
	return b.Top + b.Bottom + b.Left + b.Right
}

// CropRect returns the sub-rectangle within bounds after cropping detected borders.
// If the cropped area is empty or invalid, bounds is returned unchanged.
func (b BlackBorders) CropRect(bounds image.Rectangle) image.Rectangle {
	r := image.Rect(
		bounds.Min.X+b.Left,
		bounds.Min.Y+b.Top,
		bounds.Max.X-b.Right,
		bounds.Max.Y-b.Bottom,
	)
	if r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y {
		return bounds
	}
	return r
}

// DetectBlackBorders detects letterbox (top/bottom) or pillarbox (left/right)
// black bars in img using pure-CPU 1/4 quarter-sampling with early exit.
// If threshold is 0, DefaultBlackThreshold (20) is used.
// It returns the detected borders and whether any valid borders were found.
func DetectBlackBorders(img image.Image, threshold uint8) (BlackBorders, bool) {
	if threshold == 0 {
		threshold = DefaultBlackThreshold
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 16 || h < 16 {
		return BlackBorders{}, false
	}

	switch im := img.(type) {
	case *image.YCbCr:
		return detectYCbCr(im, threshold)
	case *image.Gray:
		return detectGray(im, threshold)
	case *image.RGBA:
		return detectRGBA(im, threshold)
	default:
		return detectGeneric(img, threshold)
	}
}

// DetectBlackBordersY detects black borders from an 8-bit Y (luma) plane
// of size width x height with the given stride in bytes.
func DetectBlackBordersY(base []byte, width, height, stride int, threshold uint8) (BlackBorders, bool) {
	if threshold == 0 {
		threshold = DefaultBlackThreshold
	}
	if width < 16 || height < 16 || len(base) < height*stride {
		return BlackBorders{}, false
	}

	rowCount := (width + 3) / 4
	colCount := (height + 3) / 4

	rowBlack := func(y int) bool {
		return sampledLineIsBlack8Bit(base, y*stride, 4, rowCount, threshold)
	}
	colBlack := func(x int) bool {
		return sampledLineIsBlack8Bit(base, x, 4*stride, colCount, threshold)
	}

	return scanBorders(width, height, rowBlack, colBlack)
}

// DetectBlackBordersP010 detects black borders from a 10-bit P010 semiplanar
// Y plane (2 bytes per sample, little-endian, upper 10 bits used).
func DetectBlackBordersP010(base []byte, width, height, stride int, threshold uint8) (BlackBorders, bool) {
	if threshold == 0 {
		threshold = DefaultBlackThreshold
	}
	if width < 16 || height < 16 || len(base) < height*stride {
		return BlackBorders{}, false
	}

	rowCount := (width + 3) / 4
	colCount := (height + 3) / 4

	rowBlack := func(y int) bool {
		return sampledLineIsBlack10Bit(base, y*stride, 4*2, rowCount, threshold)
	}
	colBlack := func(x int) bool {
		return sampledLineIsBlack10Bit(base, x*2, 4*stride, colCount, threshold)
	}

	return scanBorders(width, height, rowBlack, colBlack)
}

// CropBlackBorders crops black borders detected in img. If no borders are detected,
// img is returned unchanged with hasBorders = false.
func CropBlackBorders(img image.Image, threshold uint8) (image.Image, BlackBorders, bool) {
	borders, ok := DetectBlackBorders(img, threshold)
	if !ok || borders.IsZero() {
		return img, BlackBorders{}, false
	}

	cropRect := borders.CropRect(img.Bounds())
	if cropRect == img.Bounds() {
		return img, BlackBorders{}, false
	}

	if sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return sub.SubImage(cropRect), borders, true
	}

	dst := image.NewRGBA(image.Rect(0, 0, cropRect.Dx(), cropRect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, cropRect.Min, draw.Src)
	return dst, borders, true
}

func scanBorders(width, height int, rowBlack, colBlack func(int) bool) (BlackBorders, bool) {
	var b BlackBorders
	maxTop := height / 4
	maxBottom := height / 4
	maxLeft := width / 4
	maxRight := width / 4

	for b.Top < maxTop && rowBlack(b.Top) {
		b.Top++
	}
	for b.Bottom < maxBottom && rowBlack(height-1-b.Bottom) {
		b.Bottom++
	}
	for b.Left < maxLeft && colBlack(b.Left) {
		b.Left++
	}
	for b.Right < maxRight && colBlack(width-1-b.Right) {
		b.Right++
	}

	if b.Total() == 0 {
		return BlackBorders{}, false
	}

	// Fade to black or all-black frame guard: do not crop when all borders max out.
	if b.Top == maxTop && b.Bottom == maxBottom && b.Left == maxLeft && b.Right == maxRight {
		return BlackBorders{}, false
	}

	// Guard against over-cropping on dark scenes:
	if width-b.Left-b.Right < width/2 || height-b.Top-b.Bottom < height/2 {
		return BlackBorders{}, false
	}

	return b, true
}

func sampledLineIsBlack8Bit(base []byte, offset, step, count int, threshold uint8) bool {
	limit := uint64(threshold) * uint64(count)
	var sum uint64
	prefix := count
	if prefix > 16 {
		prefix = 16
	}
	i := 0
	for ; i < prefix; i++ {
		sum += uint64(base[offset+i*step])
	}
	if sum > limit {
		return false
	}
	for ; i < count; i++ {
		sum += uint64(base[offset+i*step])
	}
	return sum <= limit
}

func sampledLineIsBlack10Bit(base []byte, offset, step, count int, threshold uint8) bool {
	limit := uint64(threshold) * uint64(count)
	var sum uint64
	prefix := count
	if prefix > 16 {
		prefix = 16
	}
	i := 0
	for ; i < prefix; i++ {
		pos := offset + i*step
		val := (uint16(base[pos]) | (uint16(base[pos+1]) << 8)) >> 8
		sum += uint64(val)
	}
	if sum > limit {
		return false
	}
	for ; i < count; i++ {
		pos := offset + i*step
		val := (uint16(base[pos]) | (uint16(base[pos+1]) << 8)) >> 8
		sum += uint64(val)
	}
	return sum <= limit
}

func detectYCbCr(img *image.YCbCr, threshold uint8) (BlackBorders, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	minX, minY := b.Min.X, b.Min.Y

	rowCount := (w + 3) / 4
	colCount := (h + 3) / 4

	rowBlack := func(y int) bool {
		offset := img.YOffset(minX, minY+y)
		return sampledLineIsBlack8Bit(img.Y, offset, 4, rowCount, threshold)
	}
	colBlack := func(x int) bool {
		offset := img.YOffset(minX+x, minY)
		return sampledLineIsBlack8Bit(img.Y, offset, 4*img.YStride, colCount, threshold)
	}

	return scanBorders(w, h, rowBlack, colBlack)
}

func detectGray(img *image.Gray, threshold uint8) (BlackBorders, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	minX, minY := b.Min.X, b.Min.Y

	rowCount := (w + 3) / 4
	colCount := (h + 3) / 4

	rowBlack := func(y int) bool {
		offset := img.PixOffset(minX, minY+y)
		return sampledLineIsBlack8Bit(img.Pix, offset, 4, rowCount, threshold)
	}
	colBlack := func(x int) bool {
		offset := img.PixOffset(minX+x, minY)
		return sampledLineIsBlack8Bit(img.Pix, offset, 4*img.Stride, colCount, threshold)
	}

	return scanBorders(w, h, rowBlack, colBlack)
}

func detectRGBA(img *image.RGBA, threshold uint8) (BlackBorders, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	minX, minY := b.Min.X, b.Min.Y

	rowCount := (w + 3) / 4
	colCount := (h + 3) / 4

	rowBlack := func(y int) bool {
		limit := uint64(threshold) * uint64(rowCount)
		var sum uint64
		prefix := rowCount
		if prefix > 16 {
			prefix = 16
		}
		rowOff := img.PixOffset(minX, minY+y)
		i := 0
		for ; i < prefix; i++ {
			off := rowOff + i*16 // 4 pixels * 4 bytes
			r := uint32(img.Pix[off])
			g := uint32(img.Pix[off+1])
			b := uint32(img.Pix[off+2])
			sum += uint64((77*r + 150*g + 29*b) >> 8)
		}
		if sum > limit {
			return false
		}
		for ; i < rowCount; i++ {
			off := rowOff + i*16
			r := uint32(img.Pix[off])
			g := uint32(img.Pix[off+1])
			b := uint32(img.Pix[off+2])
			sum += uint64((77*r + 150*g + 29*b) >> 8)
		}
		return sum <= limit
	}

	colBlack := func(x int) bool {
		limit := uint64(threshold) * uint64(colCount)
		var sum uint64
		prefix := colCount
		if prefix > 16 {
			prefix = 16
		}
		colOff := img.PixOffset(minX+x, minY)
		step := 4 * img.Stride
		i := 0
		for ; i < prefix; i++ {
			off := colOff + i*step
			r := uint32(img.Pix[off])
			g := uint32(img.Pix[off+1])
			b := uint32(img.Pix[off+2])
			sum += uint64((77*r + 150*g + 29*b) >> 8)
		}
		if sum > limit {
			return false
		}
		for ; i < colCount; i++ {
			off := colOff + i*step
			r := uint32(img.Pix[off])
			g := uint32(img.Pix[off+1])
			b := uint32(img.Pix[off+2])
			sum += uint64((77*r + 150*g + 29*b) >> 8)
		}
		return sum <= limit
	}

	return scanBorders(w, h, rowBlack, colBlack)
}

func detectGeneric(img image.Image, threshold uint8) (BlackBorders, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	minX, minY := b.Min.X, b.Min.Y

	rowCount := (w + 3) / 4
	colCount := (h + 3) / 4

	rowBlack := func(y int) bool {
		limit := uint64(threshold) * uint64(rowCount)
		var sum uint64
		prefix := rowCount
		if prefix > 16 {
			prefix = 16
		}
		i := 0
		for ; i < prefix; i++ {
			sum += lumaAt(img, minX+i*4, minY+y)
		}
		if sum > limit {
			return false
		}
		for ; i < rowCount; i++ {
			sum += lumaAt(img, minX+i*4, minY+y)
		}
		return sum <= limit
	}

	colBlack := func(x int) bool {
		limit := uint64(threshold) * uint64(colCount)
		var sum uint64
		prefix := colCount
		if prefix > 16 {
			prefix = 16
		}
		i := 0
		for ; i < prefix; i++ {
			sum += lumaAt(img, minX+x, minY+i*4)
		}
		if sum > limit {
			return false
		}
		for ; i < colCount; i++ {
			sum += lumaAt(img, minX+x, minY+i*4)
		}
		return sum <= limit
	}

	return scanBorders(w, h, rowBlack, colBlack)
}

func lumaAt(img image.Image, x, y int) uint64 {
	c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	return uint64((77*uint32(c.R) + 150*uint32(c.G) + 29*uint32(c.B)) >> 8)
}

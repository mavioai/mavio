package imaging

import "math"

// Size is an image's dimensions in pixels.
type Size struct {
	Width, Height int
}

// SizeOptions are the requested dimensions of an image; zero means
// unset.
type SizeOptions struct {
	// Width and Height are exact dimensions; with only one set, the other
	// follows the aspect ratio.
	Width, Height int
	// MaxWidth and MaxHeight bound the size, keeping the aspect ratio.
	MaxWidth, MaxHeight int
	// FillWidth and FillHeight request the smallest size that covers them,
	// keeping the aspect ratio.
	FillWidth, FillHeight int
}

// NewSize returns the size to encode an image of the source size at; it is
// never larger than the source.
func NewSize(o SizeOptions, source Size) Size {
	s := Resize(source, o.Width, o.Height, o.MaxWidth, o.MaxHeight)
	s = ResizeFill(s, o.FillWidth, o.FillHeight)
	return ScaleDownToFit(s, source)
}

// Resize applies exact and maximum dimensions to a size.
func Resize(s Size, width, height, maxWidth, maxHeight int) Size {
	w, h := s.Width, s.Height
	switch {
	case width > 0 && height > 0:
		w, h = width, height
	case height > 0:
		w, h = scaled(w, h, height), height
	case width > 0:
		w, h = width, scaled(h, w, width)
	}
	if maxHeight > 0 && maxHeight < h {
		w, h = scaled(w, h, maxHeight), maxHeight
	}
	if maxWidth > 0 && maxWidth < w {
		w, h = maxWidth, scaled(h, w, maxWidth)
	}
	return Size{w, h}
}

// scaled returns other scaled by newSide/side, rounded like .NET's
// Convert.ToInt32 (half to even).
func scaled(other, side, newSide int) int {
	return int(math.RoundToEven(float64(newSide) / float64(side) * float64(other)))
}

// ResizeFill returns the smallest size with the aspect ratio of s that
// covers the fill dimensions, without enlarging s. A zero fill dimension
// is ignored.
func ResizeFill(s Size, fillWidth, fillHeight int) Size {
	if fillWidth == 0 && fillHeight == 0 {
		return s
	}
	fillWidth, fillHeight = max(fillWidth, 1), max(fillHeight, 1)
	ratio := min(float64(s.Width)/float64(fillWidth), float64(s.Height)/float64(fillHeight))
	if ratio < 1 {
		return s
	}
	return Size{int(math.Ceil(float64(s.Width) / ratio)), int(math.Ceil(float64(s.Height) / ratio))}
}

// ScaleDownToFit shrinks s, keeping its aspect ratio, to fit in box; each
// side is at least one pixel. Degenerate sizes are returned unchanged.
func ScaleDownToFit(s, box Size) Size {
	if s.Width <= 0 || s.Height <= 0 || box.Width <= 0 || box.Height <= 0 {
		return s
	}
	ratio := max(float64(s.Width)/float64(box.Width), float64(s.Height)/float64(box.Height))
	if ratio <= 1 {
		return s
	}
	return Size{
		min(max(int(math.RoundToEven(float64(s.Width)/ratio)), 1), box.Width),
		min(max(int(math.RoundToEven(float64(s.Height)/ratio)), 1), box.Height),
	}
}

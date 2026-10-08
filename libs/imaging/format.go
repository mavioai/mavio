package imaging

// Format is an image file format.
type Format string

// Image formats.
const (
	BMP  Format = "bmp"
	GIF  Format = "gif"
	JPEG Format = "jpg"
	PNG  Format = "png"
	WebP Format = "webp"
	SVG  Format = "svg"
)

// Formats lists every format.
var Formats = []Format{BMP, GIF, JPEG, PNG, WebP, SVG}

// MimeType returns the media type of the format; ok is false for an
// unknown format.
func (f Format) MimeType() (mime string, ok bool) {
	switch f {
	case BMP:
		return "image/bmp", true
	case GIF:
		return "image/gif", true
	case JPEG:
		return "image/jpeg", true
	case PNG:
		return "image/png", true
	case WebP:
		return "image/webp", true
	case SVG:
		return "image/svg+xml", true
	}
	return "", false
}

// Extension returns the file extension of the format, with the dot; ok is
// false for an unknown format.
func (f Format) Extension() (ext string, ok bool) {
	if _, ok := f.MimeType(); !ok {
		return "", false
	}
	return "." + string(f), true
}

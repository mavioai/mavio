package core

import (
	"fmt"
	"slices"
)

// ImageKind is the role of an image attached to an item or person.
type ImageKind string

// Image kinds.
const (
	ImagePrimary    ImageKind = "primary" // poster, cover, portrait
	ImageBackdrop   ImageKind = "backdrop"
	ImageLogo       ImageKind = "logo"
	ImageThumb      ImageKind = "thumb" // landscape thumbnail
	ImageBanner     ImageKind = "banner"
	ImageArt        ImageKind = "art" // clear art
	ImageDisc       ImageKind = "disc"
	ImageScreenshot ImageKind = "screenshot"
)

// ImageKinds lists every image kind.
var ImageKinds = []ImageKind{
	ImagePrimary, ImageBackdrop, ImageLogo, ImageThumb,
	ImageBanner, ImageArt, ImageDisc, ImageScreenshot,
}

// Valid reports whether k is a known image kind.
func (k ImageKind) Valid() bool { return slices.Contains(ImageKinds, k) }

// Image is an artwork file owned by an item or a person. Several images of
// the same kind are ordered by Index (e.g. multiple backdrops).
type Image struct {
	ID      ID
	OwnerID ID // item or person
	Kind    ImageKind
	Index   int
	// Path is the local file; RemoteURL is where it was downloaded from.
	Path      string
	RemoteURL string
	Width     int
	Height    int
	// Blurhash and Thumbhash are compact placeholders shown while loading.
	Blurhash  string
	Thumbhash []byte
}

// Validate checks the image's invariants.
func (img *Image) Validate() error {
	switch {
	case img.ID.IsZero() || img.OwnerID.IsZero():
		return fmt.Errorf("%w: image requires ID and owner ID", ErrInvalid)
	case !img.Kind.Valid():
		return fmt.Errorf("%w: unknown image kind %q", ErrInvalid, img.Kind)
	case img.Index < 0:
		return fmt.Errorf("%w: negative image index", ErrInvalid)
	case img.Path == "" && img.RemoteURL == "":
		return fmt.Errorf("%w: image %s has neither path nor remote URL", ErrInvalid, img.ID)
	}
	return nil
}

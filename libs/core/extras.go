package core

import (
	"fmt"
	"slices"
	"time"
)

// Trickplay describes the thumbnail sheets of an item at one width: tiles
// of thumbnails taken every Interval, row by row, TileWidth across and
// TileHeight down a sheet, ThumbnailCount in all.
type Trickplay struct {
	ItemID ID
	// Width and Height are a thumbnail's size in pixels.
	Width, Height         int
	TileWidth, TileHeight int
	ThumbnailCount        int
	Interval              time.Duration
	// Bandwidth is the bits per second the sheets take while playing, for
	// HLS image playlists.
	Bandwidth int
}

// Sheets returns the number of sheets.
func (t *Trickplay) Sheets() int {
	per := t.TileWidth * t.TileHeight
	if per <= 0 {
		return 0
	}
	return (t.ThumbnailCount + per - 1) / per
}

// Validate checks the trickplay's invariants.
func (t *Trickplay) Validate() error {
	switch {
	case t.ItemID.IsZero():
		return fmt.Errorf("%w: trickplay requires an item", ErrInvalid)
	case t.Width <= 0 || t.Height <= 0 || t.TileWidth <= 0 || t.TileHeight <= 0 || t.ThumbnailCount <= 0:
		return fmt.Errorf("%w: trickplay of item %s has an empty size or no thumbnails", ErrInvalid, t.ItemID)
	case t.Interval <= 0:
		return fmt.Errorf("%w: trickplay of item %s has no interval", ErrInvalid, t.ItemID)
	}
	return nil
}

// SegmentKind is what a media segment is.
type SegmentKind string

// Segment kinds, as Jellyfin's media segments.
const (
	SegmentIntro      SegmentKind = "intro"
	SegmentOutro      SegmentKind = "outro"
	SegmentRecap      SegmentKind = "recap"
	SegmentPreview    SegmentKind = "preview"
	SegmentCommercial SegmentKind = "commercial"
)

// SegmentKinds lists the segment kinds.
var SegmentKinds = []SegmentKind{SegmentIntro, SegmentOutro, SegmentRecap, SegmentPreview, SegmentCommercial}

// Valid reports whether k is a known segment kind.
func (k SegmentKind) Valid() bool { return slices.Contains(SegmentKinds, k) }

// MediaSegment is a stretch of an item, such as its intro, that clients
// may offer to skip.
type MediaSegment struct {
	ID         ID
	ItemID     ID
	Kind       SegmentKind
	Start, End time.Duration
	// Provider names the segment provider plugin that found it.
	Provider string
}

// Validate checks the segment's invariants.
func (s *MediaSegment) Validate() error {
	switch {
	case s.ID.IsZero() || s.ItemID.IsZero():
		return fmt.Errorf("%w: media segment requires ID and item", ErrInvalid)
	case !s.Kind.Valid():
		return fmt.Errorf("%w: unknown media segment kind %q", ErrInvalid, s.Kind)
	case s.Start < 0 || s.End <= s.Start:
		return fmt.Errorf("%w: media segment %s ends before it starts", ErrInvalid, s.ID)
	}
	return nil
}

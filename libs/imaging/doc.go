// Package imaging decodes, resizes and encodes artwork without cgo, and
// checks SVG files for references to external resources. Sizes follow
// Jellyfin's rules (NewSize): requested dimensions keep the aspect ratio,
// fill dimensions crop-fill, and an image is never enlarged beyond its
// source. Downscaled images are lightly sharpened.
package imaging

package indexer

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidNZB is returned for a download that is not a valid NZB.
var ErrInvalidNZB = errors.New("invalid NZB")

// ValidateNZB checks that data is an NZB with files, as Prowlarr's
// NzbValidationService does: an XML document whose root is nzb and has
// file elements, failing with the indexer's error when the root is one.
func ValidateNZB(data []byte) error {
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	// Only the structure matters, whatever the declared encoding.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	depth := 0
	var root xml.StartElement
	files := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: unable to parse: %w", ErrInvalidNZB, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch depth {
			case 1:
				root = t.Copy()
			case 2:
				if t.Name.Local == "file" && t.Name.Space == root.Name.Space {
					files++
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if root.Name.Local == "" {
		return fmt.Errorf("%w: no root element", ErrInvalidNZB)
	}
	if root.Name.Local == "error" {
		code, hasCode := attr(root, "code")
		desc, hasDesc := attr(root, "description")
		if hasCode && hasDesc {
			return fmt.Errorf("%w: contains indexer error: %s - %s", ErrInvalidNZB, code, desc)
		}
	}
	if root.Name.Local != "nzb" {
		return fmt.Errorf("%w: unexpected root element, expected 'nzb' found '%s'", ErrInvalidNZB, root.Name.Local)
	}
	if files == 0 {
		return fmt.Errorf("%w: no files", ErrInvalidNZB)
	}
	return nil
}

func attr(e xml.StartElement, name string) (string, bool) {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

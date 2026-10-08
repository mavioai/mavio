package metadata

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// node is an XML element: NFO files are small, so they are read into a
// tree and then interpreted.
type node struct {
	name     string
	attrs    []xml.Attr
	text     strings.Builder // character data directly inside the element
	children []*node
}

// attr returns an attribute value.
func (n *node) attr(name string) string {
	for _, a := range n.attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// value returns the element's text content, trimmed.
func (n *node) value() string { return strings.TrimSpace(n.text.String()) }

// child returns the first child element with the name, or nil.
func (n *node) child(name string) *node {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// errNoRoot is returned for data without an element.
var errNoRoot = errors.New("no XML element")

// parseXML reads the first element of data and its descendants. Parsing is
// lenient, as NFO files are often hand-written: unknown entities, HTML
// entities, an encoding declaration other than UTF-8 and malformed content
// after the root are tolerated, and an error inside the root keeps what was
// read before it.
func parseXML(data []byte) (*node, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }

	var root *node
	var stack []*node
	for {
		tok, err := d.Token()
		if err != nil {
			if root == nil {
				return nil, errNoRoot
			}
			return root, nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{name: t.Name.Local, attrs: t.Attr}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return root, nil
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
}

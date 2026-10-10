package indexer

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Node is an element of an XML document, as LINQ to XML's XElement is to
// Prowlarr's parsers.
type Node struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []*Node
	// Text and children in document order, for Value.
	parts []any
}

// Document is a parsed XML document.
type Document struct {
	Root *Node
	// Directives are the document's <!...> declarations, such as DOCTYPE.
	Directives []string
}

// LoadXML parses XML leniently: HTML's named entities are decoded and
// characters XML forbids are dropped, as Prowlarr's XmlCleaner does.
func LoadXML(content string) (*Document, error) {
	d := xml.NewDecoder(strings.NewReader(cleanXML(content)))
	d.Entity = xml.HTMLEntity
	// The content is decoded already, whatever the declaration says.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	doc := &Document{}
	var stack []*Node
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &Node{Name: t.Name, Attrs: t.Copy().Attr}
			if len(stack) == 0 {
				if doc.Root != nil {
					return nil, errors.New("parse XML: more than one root element")
				}
				doc.Root = n
			} else {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
				p.parts = append(p.parts, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].parts = append(stack[len(stack)-1].parts, string(t))
			}
		case xml.Directive:
			doc.Directives = append(doc.Directives, string(t))
		}
	}
	if doc.Root == nil {
		return nil, errors.New("parse XML: no root element")
	}
	return doc, nil
}

// cleanXML drops the characters XML forbids.
func cleanXML(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r',
			r >= 0x20 && r <= 0xD7FF,
			r >= 0xE000 && r <= 0xFFFD:
			if r == utf8.RuneError {
				return -1
			}
			return r
		}
		return -1
	}, s)
}

// Value returns the element's text and its descendants', concatenated.
func (n *Node) Value() string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	n.writeValue(&b)
	return b.String()
}

func (n *Node) writeValue(b *strings.Builder) {
	for _, p := range n.parts {
		switch p := p.(type) {
		case string:
			b.WriteString(p)
		case *Node:
			p.writeValue(b)
		}
	}
}

// Element returns the first child named local without a namespace, nil
// for none.
func (n *Node) Element(local string) *Node {
	return n.ElementNS("", local)
}

// ElementNS returns the first child of the namespace named local.
func (n *Node) ElementNS(space, local string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name.Space == space && c.Name.Local == local {
			return c
		}
	}
	return nil
}

// Elements returns the children named local without a namespace.
func (n *Node) Elements(local string) []*Node {
	return n.ElementsNS("", local)
}

// ElementsNS returns the children of the namespace named local.
func (n *Node) ElementsNS(space, local string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, c := range n.Children {
		if c.Name.Space == space && c.Name.Local == local {
			out = append(out, c)
		}
	}
	return out
}

// Descendants returns the descendants whose local name is local, in any
// case and namespace, as Prowlarr's FindDecendants does; all for "".
func (n *Node) Descendants(local string) []*Node {
	var out []*Node
	var walk func(*Node)
	walk = func(p *Node) {
		for _, c := range p.Children {
			if local == "" || strings.EqualFold(c.Name.Local, local) {
				out = append(out, c)
			}
			walk(c)
		}
	}
	if n != nil {
		walk(n)
	}
	return out
}

// Attr returns the value of the attribute named local without a namespace.
func (n *Node) Attr(local string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, a := range n.Attrs {
		if a.Name.Space == "" && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// ChildValue returns the value of the first child named local, def when
// there is none, as Prowlarr's TryGetValue does.
func (n *Node) ChildValue(local, def string) string {
	if c := n.Element(local); c != nil {
		return c.Value()
	}
	return def
}

// NamespaceOfPrefix returns the namespace the element declares for a
// prefix.
func (n *Node) NamespaceOfPrefix(prefix string) string {
	for _, a := range n.Attrs {
		if a.Name.Space == "xmlns" && a.Name.Local == prefix {
			return a.Value
		}
	}
	return ""
}

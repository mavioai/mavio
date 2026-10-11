// Package dom reads HTML pages for the sites Prowlarr scrapes in code,
// as AngleSharp does for them: parsing, text and attributes of elements.
package dom

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// Parse parses an HTML document.
func Parse(s string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return doc, nil
}

// Text returns the text of a node and its descendants, as AngleSharp's
// TextContent does; "" for nil.
func Text(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				b.WriteString(c.Data)
			case html.ElementNode:
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// Attr returns an element's attribute by name, in any case; "" when it
// has none or n is nil.
func Attr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Namespace == "" && strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

// Children returns an element's child elements.
func Children(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

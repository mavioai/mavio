package site

import (
	"bytes"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// Node is an element of a page.
type Node struct{ *html.Node }

// Parse parses a page in the charset its Content-Type or content
// declares, as BeautifulSoup does. An empty body is an empty page.
func Parse(body []byte, contentType string) (Node, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		n, err := html.Parse(strings.NewReader(""))
		return Node{n}, err
	}
	r, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return Node{}, err
	}
	n, err := html.Parse(r)
	if err != nil {
		return Node{}, err
	}
	return Node{n}, nil
}

// HTML parses an answer's page.
func (r *Response) HTML() (Node, error) { return Parse(r.Body, r.Header.Get("Content-Type")) }

// All returns the elements a CSS selector matches; an invalid selector
// matches none.
func (n Node) All(selector string) []Node {
	if n.Node == nil {
		return nil
	}
	sel, err := cascadia.ParseGroup(selector)
	if err != nil {
		return nil
	}
	var out []Node
	for _, m := range cascadia.QueryAll(n.Node, sel) {
		out = append(out, Node{m})
	}
	return out
}

// First returns the first element a CSS selector matches; Valid reports
// whether one did.
func (n Node) First(selector string) Node {
	if all := n.All(selector); len(all) > 0 {
		return all[0]
	}
	return Node{}
}

// Valid reports whether the node is an element.
func (n Node) Valid() bool { return n.Node != nil }

// Attr returns an attribute's value; "" when it has none.
func (n Node) Attr(name string) string {
	if n.Node == nil {
		return ""
	}
	for _, a := range n.Node.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// Text returns the text of the element and its descendants, as
// BeautifulSoup's get_text().
func (n Node) Text() string {
	if n.Node == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(m *html.Node) {
		if m.Type == html.TextNode {
			b.WriteString(m.Data)
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n.Node)
	return b.String()
}

// TrimmedText returns the text with its spaces trimmed, as get_text(strip=True)
// of a single string.
func (n Node) TrimmedText() string { return strings.TrimSpace(n.Text()) }

// Children returns the element children.
func (n Node) Children() []Node {
	if n.Node == nil {
		return nil
	}
	var out []Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			out = append(out, Node{c})
		}
	}
	return out
}

// Parent returns the parent element.
func (n Node) Parent() Node {
	if n.Node == nil || n.Node.Parent == nil {
		return Node{}
	}
	return Node{n.Node.Parent}
}

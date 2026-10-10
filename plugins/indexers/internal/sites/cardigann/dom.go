package cardigann

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// parseHTML parses an HTML document.
func parseHTML(s string) (*html.Node, error) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return doc, nil
}

// parseXMLDocument parses an XML document into a tree selectors work on,
// with element and attribute names in lower case.
func parseXMLDocument(s string) (*html.Node, error) {
	d := xml.NewDecoder(strings.NewReader(s))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	doc := &html.Node{Type: html.DocumentNode}
	cur := doc
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
			n := &html.Node{Type: html.ElementNode, Data: strings.ToLower(t.Name.Local)}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					continue
				}
				n.Attr = append(n.Attr, html.Attribute{Key: strings.ToLower(a.Name.Local), Val: a.Value})
			}
			cur.AppendChild(n)
			cur = n
		case xml.EndElement:
			if cur.Parent != nil {
				cur = cur.Parent
			}
		case xml.CharData:
			if cur != doc {
				cur.AppendChild(&html.Node{Type: html.TextNode, Data: string(t)})
			}
		}
	}
	return doc, nil
}

// documentElement returns a document's root element.
func documentElement(doc *html.Node) *html.Node {
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return doc
}

// selectors compiles CSS selectors, keeping them.
type selectors struct {
	mu    sync.Mutex
	cache map[string]cascadia.SelectorGroup
}

// compile compiles a selector as AngleSharp reads it: :contains matches
// text in its case.
func (s *selectors) compile(sel string) (cascadia.SelectorGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.cache[sel]; ok {
		return g, nil
	}
	g, err := cascadia.ParseGroup(translateSelector(sel))
	if err != nil {
		return nil, fmt.Errorf("invalid selector %q: %w", sel, err)
	}
	if s.cache == nil {
		s.cache = map[string]cascadia.SelectorGroup{}
	}
	s.cache[sel] = g
	return g, nil
}

// translateSelector rewrites :contains(text), which cascadia matches in
// any case, into a :matches of the text.
func translateSelector(sel string) string {
	var b strings.Builder
	for i := 0; i < len(sel); {
		switch c := sel[i]; {
		case c == '"' || c == '\'':
			end := quotedEnd(sel, i)
			b.WriteString(sel[i:end])
			i = end
			continue
		case c == '\\' && i+1 < len(sel):
			b.WriteString(sel[i : i+2])
			i += 2
			continue
		case strings.HasPrefix(strings.ToLower(sel[i:]), ":contains("):
			j := i + len(":contains(")
			for j < len(sel) && sel[j] == ' ' {
				j++
			}
			var text string
			if j < len(sel) && (sel[j] == '"' || sel[j] == '\'') {
				end := quotedEnd(sel, j)
				text = unquote(sel[j:end])
				j = end
			} else {
				end := strings.IndexByte(sel[j:], ')')
				if end < 0 {
					b.WriteString(sel[i:])
					return b.String()
				}
				text = strings.TrimSpace(sel[j : j+end])
				j += end
			}
			for j < len(sel) && sel[j] == ' ' {
				j++
			}
			if j >= len(sel) || sel[j] != ')' {
				b.WriteString(sel[i:])
				return b.String()
			}
			b.WriteString(":matches(" + regexpLiteral(text) + ")")
			i = j + 1
			continue
		}
		b.WriteByte(sel[i])
		i++
	}
	return b.String()
}

// quotedEnd returns the index after the quoted string starting at i.
func quotedEnd(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1
		}
	}
	return len(s)
}

func unquote(s string) string {
	if len(s) >= 2 && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	} else if len(s) > 0 {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// regexpLiteral returns a regular expression matching text, without the
// brackets and parentheses cascadia counts.
func regexpLiteral(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, `\x{%X}`, r)
	}
	return b.String()
}

// matches reports whether an element matches a selector.
func (s *selectors) matches(n *html.Node, sel string) (bool, error) {
	g, err := s.compile(sel)
	if err != nil {
		return false, err
	}
	return n.Type == html.ElementNode && g.Match(n), nil
}

// query returns the first descendant of n matching a selector; one
// starting with :root is matched in the whole document, as Prowlarr
// does for AngleSharp.
func (s *selectors) query(n *html.Node, sel string) (*html.Node, error) {
	if rest, ok := strings.CutPrefix(sel, ":root"); ok {
		sel = rest
		for n.Parent != nil && n.Parent.Type == html.ElementNode {
			n = n.Parent
		}
	}
	g, err := s.compile(sel)
	if err != nil {
		return nil, err
	}
	return cascadia.Query(n, g), nil
}

// queryAll returns the descendants of n matching a selector.
func (s *selectors) queryAll(n *html.Node, sel string) ([]*html.Node, error) {
	g, err := s.compile(sel)
	if err != nil {
		return nil, err
	}
	return cascadia.QueryAll(n, g), nil
}

// textContent returns the text of a node and its descendants.
func textContent(n *html.Node) string {
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

// attribute returns an element's attribute, by name in any case.
func attribute(n *html.Node, name string) (string, bool) {
	name = strings.ToLower(name)
	for _, a := range n.Attr {
		if a.Namespace == "" && strings.ToLower(a.Key) == name {
			return a.Val, true
		}
	}
	return "", false
}

// outerHTML returns a node as HTML, for errors.
func outerHTML(n *html.Node) string {
	if n == nil {
		return "<NULL>"
	}
	var b bytes.Buffer
	if err := html.Render(&b, n); err != nil {
		return "<" + n.Data + ">"
	}
	s := b.String()
	if len(s) > 5*1024 {
		s = s[:5*1024] + "..."
	}
	return s
}

// previousElement returns the element before n among its siblings.
func previousElement(n *html.Node) *html.Node {
	for p := n.PrevSibling; p != nil; p = p.PrevSibling {
		if p.Type == html.ElementNode {
			return p
		}
	}
	return nil
}

// parentElement returns n's parent if it is an element.
func parentElement(n *html.Node) *html.Node {
	if n.Parent != nil && n.Parent.Type == html.ElementNode {
		return n.Parent
	}
	return nil
}

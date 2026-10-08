package imaging

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// Limits of CheckSVG.
const (
	// maxDataURIDepth bounds chains of nested data:image/svg+xml payloads.
	maxDataURIDepth = 4
	// maxDecompressed bounds a gzip-compressed (svgz) payload.
	maxDecompressed = 16 << 20
	// maxEntityChars bounds the expansion of internal entities, against
	// "billion laughs" attacks.
	maxEntityChars = 1 << 20
)

// ErrUnsafeSVG is returned by CheckSVG for an SVG that cannot be served
// safely.
var ErrUnsafeSVG = errors.New("unsafe SVG")

// CheckSVG reports whether an SVG document is self-contained. It returns
// an error wrapping ErrUnsafeSVG when the document references an external
// resource (through href, a CSS url() or @import, also inside nested
// data:image/svg+xml payloads), declares external entities, expands
// entities beyond a limit, or cannot be parsed. Same-document fragments
// ("#id") and raster data URIs are allowed.
func CheckSVG(r io.Reader) error {
	if reason := checkSVG(r, 0); reason != "" {
		return fmt.Errorf("%w: %s", ErrUnsafeSVG, reason)
	}
	return nil
}

func checkSVG(r io.Reader, depth int) string {
	d := xml.NewDecoder(r)
	d.Strict = true
	d.Entity = map[string]string{}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return ""
		}
		if err != nil {
			return "the document could not be safely parsed: " + err.Error()
		}
		switch t := tok.(type) {
		case xml.Directive:
			if reason := checkDoctype(string(t), d.Entity); reason != "" {
				return reason
			}
		case xml.StartElement:
			for _, a := range t.Attr {
				var reason string
				if strings.EqualFold(a.Name.Local, "href") {
					reason = checkReference(a.Value, depth, "href")
				} else {
					reason = checkCSS(a.Value, depth)
				}
				if reason != "" {
					return reason
				}
			}
		case xml.CharData:
			if reason := checkCSS(string(t), depth); reason != "" {
				return reason
			}
		}
	}
}

var entityDecl = regexp.MustCompile(`<!ENTITY\s+(%\s+)?([^\s]+)\s+(?:"([^"]*)"|'([^']*)'|(.*?))\s*>`)

// checkDoctype rejects external entities in the internal subset of a
// DOCTYPE and records its internal general entities, fully expanded, in
// entities.
func checkDoctype(directive string, entities map[string]string) string {
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(directive)), "DOCTYPE") {
		return ""
	}
	open, end := strings.IndexByte(directive, '['), strings.LastIndexByte(directive, ']')
	if open < 0 || end < open {
		return "" // an external DTD declaration alone is not loaded
	}
	subset := directive[open+1 : end]
	upper := strings.ToUpper(subset)
	if strings.Contains(upper, "SYSTEM") || strings.Contains(upper, "PUBLIC") {
		return "the document declares an external DTD entity"
	}
	raw := map[string]string{}
	for _, m := range entityDecl.FindAllStringSubmatch(subset, -1) {
		if m[1] != "" {
			continue // parameter entity
		}
		raw[m[2]] = m[3] + m[4] + m[5]
	}
	budget := maxEntityChars
	var expand func(name string, seen map[string]bool) (string, bool)
	expand = func(name string, seen map[string]bool) (string, bool) {
		value, ok := raw[name]
		if !ok || seen[name] {
			return "", false
		}
		seen[name] = true
		defer delete(seen, name)
		var b strings.Builder
		for {
			i := strings.IndexByte(value, '&')
			if i < 0 {
				break
			}
			j := strings.IndexByte(value[i:], ';')
			if j < 0 {
				break
			}
			b.WriteString(value[:i])
			ref := value[i+1 : i+j]
			if v, ok := expand(ref, seen); ok {
				b.WriteString(v)
			} else {
				b.WriteString(value[i : i+j+1])
			}
			if budget -= b.Len(); budget < 0 {
				return "", false
			}
			value = value[i+j+1:]
		}
		b.WriteString(value)
		return b.String(), true
	}
	for name := range raw {
		v, ok := expand(name, map[string]bool{})
		if !ok {
			return "entity expansion exceeds the allowed size"
		}
		entities[name] = v
	}
	return ""
}

// checkReference allows same-document fragments and data URIs whose
// nested SVG is itself safe.
func checkReference(value string, depth int, context string) string {
	v := strings.TrimSpace(value)
	switch {
	case v == "" || v[0] == '#':
		return ""
	case len(v) >= 5 && strings.EqualFold(v[:5], "data:"):
		return checkDataURI(v, depth, context)
	}
	return "an external resource is referenced via " + context
}

// checkDataURI re-checks data:image/svg+xml payloads, which the renderer
// parses as SVG; other data URIs are raster data.
func checkDataURI(uri string, depth int, context string) string {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return "a malformed data URI is referenced via " + context
	}
	header := uri[5:comma]
	mediaType, _, _ := strings.Cut(header, ";")
	if !strings.EqualFold(strings.TrimSpace(mediaType), "image/svg+xml") {
		return ""
	}
	if depth >= maxDataURIDepth {
		return "nested data URIs exceed the allowed depth"
	}
	payload := strings.TrimSpace(uri[comma+1:])
	var data []byte
	if i := strings.LastIndexByte(header, ';'); i >= 0 && strings.EqualFold(strings.TrimSpace(header[i+1:]), "base64") {
		var err error
		if data, err = base64.StdEncoding.DecodeString(payload); err != nil {
			return "an undecodable data URI is referenced via " + context
		}
	} else {
		unescaped, err := url.PathUnescape(payload)
		if err != nil {
			return "an undecodable data URI is referenced via " + context
		}
		data = []byte(unescaped)
	}
	if len(data) > 2 && data[0] == 0x1F && data[1] == 0x8B {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "an invalid compressed data URI is referenced via " + context
		}
		if data, err = io.ReadAll(io.LimitReader(zr, maxDecompressed+1)); err != nil {
			return "an invalid compressed data URI is referenced via " + context
		}
		if len(data) > maxDecompressed {
			return "a compressed data URI exceeds the allowed size"
		}
	}
	return checkSVG(bytes.NewReader(data), depth+1)
}

// checkCSS checks the url() references and @import rules in a style.
func checkCSS(value string, depth int) string {
	lower := strings.ToLower(value)
	for i := 0; ; {
		found := strings.Index(lower[i:], "url(")
		if found < 0 {
			break
		}
		start := i + found + 4
		end := strings.IndexByte(value[start:], ')')
		if end < 0 {
			break
		}
		target := strings.TrimSpace(strings.Trim(strings.Trim(strings.TrimSpace(value[start:start+end]), "'"), `"`))
		if reason := checkReference(target, depth, "url()"); reason != "" {
			return reason
		}
		i = start + end + 1
	}
	for i := 0; ; {
		found := strings.Index(lower[i:], "@import")
		if found < 0 {
			break
		}
		rest := value[i+found+7:]
		if q := strings.IndexAny(rest, `'"`); q >= 0 {
			if end := strings.IndexAny(rest[q+1:], `'"`); end >= 0 {
				if reason := checkReference(rest[q+1:q+1+end], depth, "@import"); reason != "" {
					return reason
				}
			}
		}
		i += found + 7
	}
	return ""
}

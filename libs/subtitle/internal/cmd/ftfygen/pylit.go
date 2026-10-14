package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/runenames"
)

// runeName is the Unicode name of r.
func runeName(r rune) string { return runenames.Name(r) }

// pyParser reads the subset of Python literals ftfy's and CPython's tables
// are written in: dicts of strings, and parenthesized concatenations of
// string literals with comments in between.
type pyParser struct {
	src   string
	pos   int
	names map[string]rune // for \N{...}
}

func (p *pyParser) skip() {
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		case '#':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		default:
			return
		}
	}
}

func (p *pyParser) peek() byte {
	p.skip()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *pyParser) errorf(format string, args ...any) error {
	line := strings.Count(p.src[:p.pos], "\n") + 1
	return fmt.Errorf("line %d: %w: %s", line, errSyntax, fmt.Sprintf(format, args...))
}

// value parses a string (string) or a dict of strings (map[string]string).
func (p *pyParser) value() (any, error) {
	switch c := p.peek(); c {
	case '(':
		p.pos++
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		if p.peek() != ')' {
			return nil, p.errorf("want )")
		}
		p.pos++
		return v, nil
	case '{':
		p.pos++
		return p.dict()
	case '\'', '"':
		return p.strings()
	default:
		return nil, p.errorf("unexpected %q", c)
	}
}

func (p *pyParser) dict() (map[string]string, error) {
	d := map[string]string{}
	for {
		if p.peek() == '}' {
			p.pos++
			return d, nil
		}
		k, err := p.value()
		if err != nil {
			return nil, err
		}
		if p.peek() != ':' {
			return nil, p.errorf("want :")
		}
		p.pos++
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		ks, ok1 := k.(string)
		vs, ok2 := v.(string)
		if !ok1 || !ok2 {
			return nil, p.errorf("dict member is not a pair of strings")
		}
		d[ks] = vs
		switch p.peek() {
		case ',':
			p.pos++
		case '}':
		default:
			return nil, p.errorf("want , or }")
		}
	}
}

// strings reads adjacent string literals, which Python concatenates.
func (p *pyParser) strings() (string, error) {
	var b strings.Builder
	for c := p.peek(); c == '\'' || c == '"'; c = p.peek() {
		if err := p.literal(&b); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// literal reads one single-quoted or double-quoted string literal.
func (p *pyParser) literal(b *strings.Builder) error {
	quote := p.src[p.pos]
	p.pos++
	for {
		if p.pos >= len(p.src) || p.src[p.pos] == '\n' {
			return p.errorf("unterminated string")
		}
		c := p.src[p.pos]
		if c == quote {
			p.pos++
			return nil
		}
		if c != '\\' {
			r, size := utf8.DecodeRuneInString(p.src[p.pos:])
			b.WriteRune(r)
			p.pos += size
			continue
		}
		if p.pos+1 >= len(p.src) {
			return p.errorf("unterminated escape")
		}
		e := p.src[p.pos+1]
		p.pos += 2
		switch e {
		case '\\', '\'', '"':
			b.WriteByte(e)
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'x', 'u', 'U':
			n := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
			if p.pos+n > len(p.src) {
				return p.errorf("short escape")
			}
			v, err := strconv.ParseUint(p.src[p.pos:p.pos+n], 16, 32)
			if err != nil {
				return p.errorf("escape: %v", err)
			}
			b.WriteRune(rune(v))
			p.pos += n
		case 'N':
			end := strings.IndexByte(p.src[p.pos:], '}')
			if !strings.HasPrefix(p.src[p.pos:], "{") || end < 0 {
				return p.errorf("bad \\N escape")
			}
			name := p.src[p.pos+1 : p.pos+end]
			r, ok := p.names[name]
			if !ok {
				return p.errorf("unknown character name %q", name)
			}
			b.WriteRune(r)
			p.pos += end + 1
		default:
			return p.errorf("escape \\%c", e)
		}
	}
}

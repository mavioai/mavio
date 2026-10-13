package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// pyValue is a parsed Python literal: a string, a number, an identifier
// such as None, a dict (ordered pairs), or a list or tuple. A call such as
// OrderedDict([...]) is its only argument, or an empty dict without one.
type pyValue struct {
	kind  pyKind
	str   string // string, number or identifier
	pairs []pyPair
	items []pyValue
}

type pyKind int

const (
	pyString pyKind = iota
	pyAtom
	pyDict
	pyList
)

type pyPair struct {
	key, value pyValue
}

// pyParser reads the literal subset of Python that pprint writes.
type pyParser struct {
	src string
	pos int
}

// parseAssignment parses the literal assigned to name in a Python source,
// as in "name = {...}".
func parseAssignment(src, name string) (pyValue, error) {
	for i := 0; ; {
		j := strings.Index(src[i:], name)
		if j < 0 {
			return pyValue{}, fmt.Errorf("no assignment to %s", name)
		}
		i += j
		lineStart := i == 0 || src[i-1] == '\n'
		rest := strings.TrimLeft(src[i+len(name):], " \t")
		if lineStart && strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") {
			p := &pyParser{src: src, pos: len(src) - len(rest) + 1}
			return p.value()
		}
		i += len(name)
	}
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

func (p *pyParser) expect(c byte) error {
	if p.peek() != c {
		return fmt.Errorf("offset %d: want %q", p.pos, c)
	}
	p.pos++
	return nil
}

func (p *pyParser) value() (pyValue, error) {
	switch c := p.peek(); {
	case c == '{':
		p.pos++
		return p.dict()
	case c == '[':
		p.pos++
		items, err := p.sequence(']')
		return pyValue{kind: pyList, items: items}, err
	case c == '(':
		p.pos++
		items, err := p.sequence(')')
		if err != nil {
			return pyValue{}, err
		}
		if len(items) == 1 && p.src[p.pos-2] != ',' {
			return items[0], nil // parenthesized expression
		}
		return pyValue{kind: pyList, items: items}, nil
	case c == '\'' || c == '"' || isPrefixedString(p.src[p.pos:]):
		return p.strings()
	case c == '-' || c >= '0' && c <= '9':
		start := p.pos
		p.pos++
		for p.pos < len(p.src) && strings.IndexByte("0123456789.xXabcdefABCDEF_", p.src[p.pos]) >= 0 {
			p.pos++
		}
		return pyValue{kind: pyAtom, str: p.src[start:p.pos]}, nil
	case isIdent(c):
		start := p.pos
		for p.pos < len(p.src) && (isIdent(p.src[p.pos]) || p.src[p.pos] >= '0' && p.src[p.pos] <= '9' || p.src[p.pos] == '.') {
			p.pos++
		}
		name := p.src[start:p.pos]
		if p.peek() != '(' {
			return pyValue{kind: pyAtom, str: name}, nil
		}
		p.pos++
		args, err := p.sequence(')')
		switch {
		case err != nil:
			return pyValue{}, err
		case len(args) == 0:
			return pyValue{kind: pyDict}, nil
		case len(args) > 1:
			return pyValue{}, fmt.Errorf("%s(...) has %d arguments", name, len(args))
		}
		arg := args[0]
		if name == "OrderedDict" && arg.kind == pyList {
			d := pyValue{kind: pyDict}
			for _, it := range arg.items {
				if it.kind != pyList || len(it.items) != 2 {
					return pyValue{}, errors.New("OrderedDict item is not a pair")
				}
				d.pairs = append(d.pairs, pyPair{it.items[0], it.items[1]})
			}
			return d, nil
		}
		return arg, nil
	}
	return pyValue{}, fmt.Errorf("offset %d: unexpected %q", p.pos, p.peek())
}

func isIdent(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isPrefixedString(s string) bool {
	for i := 0; i < len(s) && i < 3; i++ {
		switch s[i] {
		case 'u', 'U', 'r', 'R', 'b', 'B':
			continue
		case '\'', '"':
			return i > 0
		}
		return false
	}
	return false
}

func (p *pyParser) sequence(end byte) ([]pyValue, error) {
	var items []pyValue
	for {
		if p.peek() == end {
			p.pos++
			return items, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
		if p.peek() == ',' {
			p.pos++
			continue
		}
		if err := p.expect(end); err != nil {
			return nil, err
		}
		return items, nil
	}
}

func (p *pyParser) dict() (pyValue, error) {
	d := pyValue{kind: pyDict}
	for {
		if p.peek() == '}' {
			p.pos++
			return d, nil
		}
		k, err := p.value()
		if err != nil {
			return d, err
		}
		if err := p.expect(':'); err != nil {
			return d, err
		}
		v, err := p.value()
		if err != nil {
			return d, err
		}
		d.pairs = append(d.pairs, pyPair{k, v})
		if p.peek() == ',' {
			p.pos++
			continue
		}
		if err := p.expect('}'); err != nil {
			return d, err
		}
		return d, nil
	}
}

// strings reads adjacent string literals, which Python concatenates.
func (p *pyParser) strings() (pyValue, error) {
	var b strings.Builder
	for {
		c := p.peek()
		if c != '\'' && c != '"' && !isPrefixedString(p.src[p.pos:]) {
			return pyValue{kind: pyString, str: b.String()}, nil
		}
		raw := false
		for p.src[p.pos] != '\'' && p.src[p.pos] != '"' {
			if p.src[p.pos] == 'r' || p.src[p.pos] == 'R' {
				raw = true
			}
			p.pos++
		}
		s, err := p.stringLiteral(raw)
		if err != nil {
			return pyValue{}, err
		}
		b.WriteString(s)
	}
}

func (p *pyParser) stringLiteral(raw bool) (string, error) {
	quote := p.src[p.pos : p.pos+1]
	if strings.HasPrefix(p.src[p.pos:], strings.Repeat(quote, 3)) {
		quote = strings.Repeat(quote, 3)
	}
	p.pos += len(quote)
	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			return "", errors.New("unterminated string")
		}
		if strings.HasPrefix(p.src[p.pos:], quote) {
			p.pos += len(quote)
			return b.String(), nil
		}
		c := p.src[p.pos]
		if c != '\\' || raw {
			if raw && c == '\\' && p.pos+1 < len(p.src) {
				b.WriteString(p.src[p.pos : p.pos+2])
				p.pos += 2
				continue
			}
			r, size := utf8.DecodeRuneInString(p.src[p.pos:])
			b.WriteRune(r)
			p.pos += size
			continue
		}
		if p.pos+1 >= len(p.src) {
			return "", errors.New("unterminated escape")
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
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\n':
		case 'x', 'u', 'U':
			n := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
			if p.pos+n > len(p.src) {
				return "", errors.New("short escape")
			}
			v, err := strconv.ParseUint(p.src[p.pos:p.pos+n], 16, 32)
			if err != nil {
				return "", fmt.Errorf("escape: %w", err)
			}
			b.WriteRune(rune(v))
			p.pos += n
		default:
			if e >= '0' && e <= '7' {
				end := p.pos - 1
				for end < len(p.src) && end < p.pos+2 && p.src[end] >= '0' && p.src[end] <= '7' {
					end++
				}
				v, _ := strconv.ParseUint(p.src[p.pos-1:end], 8, 32)
				b.WriteRune(rune(v))
				p.pos = end
				continue
			}
			// Unknown escapes keep their backslash.
			b.WriteByte('\\')
			b.WriteByte(e)
		}
	}
}

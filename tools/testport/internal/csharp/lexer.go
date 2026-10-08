// Package csharp tokenizes C# source and parses the subset of declarations and
// expressions that appear in parameterized xUnit tests.
package csharp

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind classifies a token.
type Kind int

// Token kinds.
const (
	Ident Kind = iota
	String
	Char
	Number
	Punct
	// Interpolated is an interpolated string ($"…"); it is not a constant, so
	// Value is empty and parsers treat it as an opaque expression.
	Interpolated
)

// Token is a lexical token. For String and Char tokens Value holds the decoded
// text; for other kinds it equals the source text.
type Token struct {
	Kind  Kind
	Text  string // source text
	Value string
	Line  int
	Pos   int // byte offset of the token start
	End   int // byte offset just past the token
}

// Lex tokenizes src, dropping whitespace, comments and preprocessor lines.
func Lex(src string) ([]Token, error) {
	l := &lexer{src: src, line: 1}
	for {
		l.skipTrivia()
		if l.pos >= len(l.src) {
			return l.toks, nil
		}
		if err := l.next(); err != nil {
			return nil, fmt.Errorf("line %d: %w", l.line, err)
		}
	}
}

type lexer struct {
	src  string
	pos  int
	line int
	toks []Token
}

func (l *lexer) peek(off int) byte {
	if l.pos+off < len(l.src) {
		return l.src[l.pos+off]
	}
	return 0
}

func (l *lexer) advance(n int) {
	l.line += strings.Count(l.src[l.pos:l.pos+n], "\n")
	l.pos += n
}

func (l *lexer) skipTrivia() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n' || c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			l.advance(1)
		case c == '/' && l.peek(1) == '/':
			end := strings.IndexByte(l.src[l.pos:], '\n')
			if end < 0 {
				end = len(l.src) - l.pos
			}
			l.advance(end)
		case c == '/' && l.peek(1) == '*':
			end := strings.Index(l.src[l.pos+2:], "*/")
			if end < 0 {
				end = len(l.src) - l.pos - 4
			}
			l.advance(end + 4)
		case c == '#' && l.atLineStart():
			end := strings.IndexByte(l.src[l.pos:], '\n')
			if end < 0 {
				end = len(l.src) - l.pos
			}
			l.advance(end)
		case c == 0xEF && strings.HasPrefix(l.src[l.pos:], "\uFEFF"):
			l.advance(3)
		default:
			return
		}
	}
}

func (l *lexer) atLineStart() bool {
	for i := l.pos - 1; i >= 0; i-- {
		switch l.src[i] {
		case ' ', '\t':
			continue
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

func (l *lexer) emit(kind Kind, start, startLine int, value string) {
	l.toks = append(l.toks, Token{Kind: kind, Text: l.src[start:l.pos], Value: value, Line: startLine, Pos: start, End: l.pos})
}

func (l *lexer) next() error {
	start, line := l.pos, l.line
	c := l.src[l.pos]
	switch {
	case c == '"' && strings.HasPrefix(l.src[l.pos:], `"""`):
		v, err := l.rawString()
		if err != nil {
			return err
		}
		l.emit(String, start, line, v)
	case c == '"':
		v, err := l.regularString()
		if err != nil {
			return err
		}
		l.emit(String, start, line, v)
	case c == '@' && l.peek(1) == '"', c == '$' && l.peek(1) == '@' && l.peek(2) == '"', c == '@' && l.peek(1) == '$' && l.peek(2) == '"':
		for l.src[l.pos] != '"' {
			l.advance(1)
		}
		v, err := l.verbatimString()
		if err != nil {
			return err
		}
		l.emit(String, start, line, v)
	case c == '$' && l.peek(1) == '"':
		l.advance(1)
		if _, err := l.regularString(); err != nil {
			return err
		}
		l.emit(Interpolated, start, line, "")
	case c == '\'':
		v, err := l.charLiteral()
		if err != nil {
			return err
		}
		l.emit(Char, start, line, v)
	case c >= '0' && c <= '9' || c == '.' && l.peek(1) >= '0' && l.peek(1) <= '9':
		l.number()
		l.emit(Number, start, line, l.src[start:l.pos])
	case c == '@' || c == '_' || c >= 0x80 || unicode.IsLetter(rune(c)):
		l.advance(1)
		for l.pos < len(l.src) {
			r, size := utf8.DecodeRuneInString(l.src[l.pos:])
			if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			l.advance(size)
		}
		l.emit(Ident, start, line, l.src[start:l.pos])
	default:
		n := 1
		for _, op := range []string{"=>", "??", "?.", "::", "==", "!=", "<=", ">=", "&&", "||", "++", "--"} {
			if strings.HasPrefix(l.src[l.pos:], op) {
				n = 2
				break
			}
		}
		l.advance(n)
		l.emit(Punct, start, line, l.src[start:l.pos])
	}
	return nil
}

func (l *lexer) regularString() (string, error) {
	l.advance(1) // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '"':
			l.advance(1)
			return b.String(), nil
		case '\\':
			r, n, err := decodeEscape(l.src[l.pos:])
			if err != nil {
				return "", err
			}
			b.WriteString(r)
			l.advance(n)
		case '\n':
			return "", fmt.Errorf("newline in string literal")
		default:
			b.WriteByte(c)
			l.advance(1)
		}
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (l *lexer) verbatimString() (string, error) {
	l.advance(1) // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		if l.src[l.pos] == '"' {
			if l.peek(1) == '"' {
				b.WriteByte('"')
				l.advance(2)
				continue
			}
			l.advance(1)
			return b.String(), nil
		}
		b.WriteByte(l.src[l.pos])
		l.advance(1)
	}
	return "", fmt.Errorf("unterminated verbatim string literal")
}

// rawString decodes a C# 11 raw string literal ("""…""").
func (l *lexer) rawString() (string, error) {
	n := 0
	for l.peek(n) == '"' {
		n++
	}
	quotes := strings.Repeat(`"`, n)
	l.advance(n)
	end := strings.Index(l.src[l.pos:], quotes)
	if end < 0 {
		return "", fmt.Errorf("unterminated raw string literal")
	}
	body := l.src[l.pos : l.pos+end]
	l.advance(end + n)
	if !strings.Contains(body, "\n") {
		return body, nil
	}
	// Multi-line: drop the first and last lines and remove the closing
	// line's indentation from every line.
	lines := strings.Split(body, "\n")
	indent := lines[len(lines)-1]
	lines = lines[1 : len(lines)-1]
	for i, ln := range lines {
		lines[i] = strings.TrimSuffix(strings.TrimPrefix(ln, indent), "\r")
	}
	return strings.Join(lines, "\n"), nil
}

func (l *lexer) charLiteral() (string, error) {
	l.advance(1)
	var v string
	if l.peek(0) == '\\' {
		r, n, err := decodeEscape(l.src[l.pos:])
		if err != nil {
			return "", err
		}
		v = r
		l.advance(n)
	} else {
		_, size := utf8.DecodeRuneInString(l.src[l.pos:])
		v = l.src[l.pos : l.pos+size]
		l.advance(size)
	}
	if l.peek(0) != '\'' {
		return "", fmt.Errorf("unterminated character literal")
	}
	l.advance(1)
	return v, nil
}

func (l *lexer) number() {
	if l.peek(0) == '0' && (l.peek(1) == 'x' || l.peek(1) == 'X' || l.peek(1) == 'b' || l.peek(1) == 'B') {
		l.advance(2)
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		isExpSign := (c == '+' || c == '-') && l.pos > 0 && (l.src[l.pos-1] == 'e' || l.src[l.pos-1] == 'E') &&
			!strings.HasPrefix(strings.ToLower(l.src[:l.pos]), "0x")
		if c == '.' && !isDigit(l.peek(1)) {
			return
		}
		partOfNumber := c == '.' || c == '_' || isExpSign || isDigit(c) || unicode.IsLetter(rune(c))
		if !partOfNumber {
			return
		}
		l.advance(1)
	}
}

// decodeEscape decodes the escape sequence at the start of s, returning the
// decoded text and the number of bytes consumed.
func decodeEscape(s string) (string, int, error) {
	if len(s) < 2 {
		return "", 0, fmt.Errorf("truncated escape sequence")
	}
	simple := map[byte]string{
		'\'': "'", '"': `"`, '\\': `\`, '0': "\x00", 'a': "\a", 'b': "\b",
		'f': "\f", 'n': "\n", 'r': "\r", 't': "\t", 'v': "\v",
	}
	if v, ok := simple[s[1]]; ok {
		return v, 2, nil
	}
	var digits int
	switch s[1] {
	case 'u':
		digits = 4
	case 'U':
		digits = 8
	case 'x':
		digits = 1
		for digits < 4 && 2+digits < len(s) && isHex(s[2+digits]) {
			digits++
		}
	default:
		return "", 0, fmt.Errorf("unknown escape sequence \\%c", s[1])
	}
	if len(s) < 2+digits {
		return "", 0, fmt.Errorf("truncated escape sequence")
	}
	code, err := strconv.ParseUint(s[2:2+digits], 16, 32)
	if err != nil {
		return "", 0, fmt.Errorf("invalid escape sequence %q", s[:2+digits])
	}
	return string(rune(code)), 2 + digits, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

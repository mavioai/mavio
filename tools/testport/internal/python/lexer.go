// Package python tokenizes Python source and reads the test functions of
// pytest modules: their decorators, parameters and line numbers, and the
// literal values of @pytest.mark.parametrize.
package python

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
	Name Kind = iota
	Number
	String
	Op
)

// Token is one token of a logical line.
type Token struct {
	Kind Kind
	// Text is the token as written; Value a string's decoded contents.
	Text  string
	Value string
	// Prefix is a string's prefix, lower case ("", "r", "b", "f", "rb", …).
	Prefix string
	// Pos and End are byte offsets in the source; Line is 1-based.
	Pos, End, Line int
}

// Line is a logical line: its indentation and tokens.
type Line struct {
	Indent int
	Tokens []Token
}

// Lines splits src into logical lines, joining lines inside brackets and
// after backslashes, and dropping comments and blank lines.
func Lines(src string) ([]Line, error) {
	var (
		out    []Line
		cur    []Token
		depth  int
		indent = -1
		line   = 1
		bol    = true // at the beginning of a physical line
		col    int
	)
	flush := func() {
		if len(cur) > 0 {
			out = append(out, Line{Indent: indent, Tokens: cur})
		}
		cur, indent = nil, -1
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
			if depth == 0 {
				flush()
			}
			bol, col = true, 0
			continue
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			if bol {
				switch c {
				case '\t':
					col = (col/8 + 1) * 8
				case ' ':
					col++
				}
			}
			i++
			continue
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '\\' && i+1 < len(src) && (src[i+1] == '\n' || src[i+1] == '\r'):
			i++
			if src[i] == '\r' {
				i++
			}
			if i < len(src) && src[i] == '\n' {
				i++
			}
			line++
			bol = false
			continue
		}
		if bol && depth == 0 && len(cur) == 0 {
			indent = col
		}
		bol = false
		tok, err := next(src, i, line)
		if err != nil {
			return nil, err
		}
		line += strings.Count(src[tok.Pos:tok.End], "\n")
		if tok.Kind == Op {
			switch tok.Text {
			case "(", "[", "{":
				depth++
			case ")", "]", "}":
				if depth > 0 {
					depth--
				}
			}
		}
		cur = append(cur, tok)
		i = tok.End
	}
	flush()
	return out, nil
}

var ops = []string{
	"**=", "//=", ">>=", "<<=", "...", "->", ":=", "**", "//", "==", "!=", "<=", ">=", "<<", ">>",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "@=",
}

func next(src string, i, line int) (Token, error) {
	c := src[i]
	r, size := utf8.DecodeRuneInString(src[i:])
	switch {
	case isNameStart(r):
		j := i + size
		for j < len(src) {
			r, n := utf8.DecodeRuneInString(src[j:])
			if !isNameStart(r) && !unicode.IsDigit(r) {
				break
			}
			j += n
		}
		if j < len(src) && (src[j] == '\'' || src[j] == '"') && isStringPrefix(src[i:j]) {
			return stringToken(src, i, j, line)
		}
		return Token{Kind: Name, Text: src[i:j], Pos: i, End: j, Line: line}, nil
	case c == '\'' || c == '"':
		return stringToken(src, i, i, line)
	case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
		j := i
		for j < len(src) {
			d := src[j]
			if d >= '0' && d <= '9' || d >= 'a' && d <= 'z' || d >= 'A' && d <= 'Z' || d == '_' || d == '.' {
				j++
			} else if (d == '+' || d == '-') && (src[j-1] == 'e' || src[j-1] == 'E') && !strings.HasPrefix(strings.ToLower(src[i:j]), "0x") {
				j++
			} else {
				break
			}
		}
		return Token{Kind: Number, Text: src[i:j], Pos: i, End: j, Line: line}, nil
	}
	for _, op := range ops {
		if strings.HasPrefix(src[i:], op) {
			return Token{Kind: Op, Text: op, Pos: i, End: i + len(op), Line: line}, nil
		}
	}
	if strings.ContainsRune("()[]{}:,;.@=+-*/%&|^~<>!", r) {
		return Token{Kind: Op, Text: string(r), Pos: i, End: i + size, Line: line}, nil
	}
	return Token{}, fmt.Errorf("line %d: unexpected %q", line, r)
}

func isNameStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }

func isStringPrefix(p string) bool {
	switch strings.ToLower(p) {
	case "r", "u", "b", "f", "br", "rb", "fr", "rf":
		return true
	}
	return false
}

// stringToken reads the string literal whose prefix starts at i and whose
// quote starts at q.
func stringToken(src string, i, q, line int) (Token, error) {
	prefix := strings.ToLower(src[i:q])
	quote := src[q : q+1]
	if strings.HasPrefix(src[q:], strings.Repeat(quote, 3)) {
		quote = strings.Repeat(quote, 3)
	}
	start := q + len(quote)
	j := start
	for {
		if j >= len(src) {
			return Token{}, fmt.Errorf("line %d: unterminated string", line)
		}
		if src[j] == '\\' {
			j += 2
			continue
		}
		if len(quote) == 1 && src[j] == '\n' {
			return Token{}, fmt.Errorf("line %d: unterminated string", line)
		}
		if strings.HasPrefix(src[j:], quote) {
			break
		}
		j++
	}
	body := src[start:j]
	value := body
	if !strings.Contains(prefix, "r") {
		value = unescape(body)
	}
	return Token{Kind: String, Text: src[i : j+len(quote)], Value: value, Prefix: prefix, Pos: i, End: j + len(quote), Line: line}, nil
}

// unescape decodes the escape sequences of a non-raw string.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case '\n':
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'x', 'u', 'U':
			n := map[byte]int{'x': 2, 'u': 4, 'U': 8}[c]
			if i+n < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+1+n], 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += n
					continue
				}
			}
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\\', '\'', '"':
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}

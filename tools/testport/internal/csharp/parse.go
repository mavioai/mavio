package csharp

import (
	"fmt"
	"strings"
)

// File is a parsed C# source file.
type File struct {
	Src     string
	Classes []*Class
}

// Class is a class, struct or record declaration.
type Class struct {
	Name string
	Line int
	// BodyStart is the byte offset just past the opening brace.
	BodyStart int
	Members   []*Member
	Nested    []*Class
	Outer     *Class
}

// Member is a method, property or field declaration.
type Member struct {
	Name       string
	Line       int
	Attributes []Attribute
	IsMethod   bool
	Params     []Param
	// Header holds the tokens between the attributes and the body: modifiers,
	// type, name and parameter list.
	Header []Token
	// Body holds the tokens of the block body or expression body, if any.
	Body []Token
	// Start and End are the byte offsets of the member, including its
	// attributes.
	Start, End int
}

// Attribute is one attribute applied to a member, e.g. InlineData("a", 1).
type Attribute struct {
	Name string // last name segment without the "Attribute" suffix
	Line int
	Args []Arg
}

// Arg is an argument of an attribute, constructor or method call.
type Arg struct {
	Name string // set for named arguments ("Skip = …" or "path: …")
	Expr []Token
}

// Param is a method parameter.
type Param struct {
	Name    string
	Type    string
	Params  bool // declared with the params modifier
	Default []Token
}

// Find returns the member with the given name in c or its outer classes.
func (c *Class) Find(name string) *Member {
	for cls := c; cls != nil; cls = cls.Outer {
		for _, m := range cls.Members {
			if m.Name == name {
				return m
			}
		}
	}
	return nil
}

// Parse parses src.
func Parse(src string) (*File, error) {
	toks, err := Lex(src)
	if err != nil {
		return nil, err
	}
	f := &File{Src: src}
	p := &parser{toks: toks}
	classes, err := p.scope(0, len(toks), nil)
	if err != nil {
		return nil, err
	}
	f.Classes = classes
	return f, nil
}

type parser struct {
	toks []Token
}

func (p *parser) is(i int, text string) bool {
	return i < len(p.toks) && p.toks[i].Kind != String && p.toks[i].Kind != Char && p.toks[i].Text == text
}

var typeKeywords = map[string]bool{"class": true, "struct": true, "record": true}

// scope finds the type declarations in toks[start:end].
func (p *parser) scope(start, end int, outer *Class) ([]*Class, error) {
	var classes []*Class
	for i := start; i < end; i++ {
		if p.toks[i].Kind != Ident || !typeKeywords[p.toks[i].Text] || i > 0 && p.is(i-1, ".") {
			continue
		}
		cls, next, err := p.class(i, end, outer)
		if err != nil {
			return nil, err
		}
		if cls != nil {
			classes = append(classes, cls)
		}
		i = next
	}
	return classes, nil
}

// class parses the type declaration whose keyword is at i and returns the
// index of its closing brace.
func (p *parser) class(i, end int, outer *Class) (*Class, int, error) {
	if i+1 >= end || p.toks[i+1].Kind != Ident {
		return nil, i, nil
	}
	cls := &Class{Name: p.toks[i+1].Text, Line: p.toks[i].Line, Outer: outer}
	open := -1
	for j := i + 2; j < end; j++ {
		if p.is(j, ";") { // record with a positional declaration only
			return cls, j, nil
		}
		if p.is(j, "{") {
			open = j
			break
		}
	}
	if open < 0 {
		return nil, end, fmt.Errorf("line %d: class %s has no body", cls.Line, cls.Name)
	}
	closing, err := p.match(open)
	if err != nil {
		return nil, end, err
	}
	cls.BodyStart = p.toks[open].End
	if err := p.members(cls, open+1, closing); err != nil {
		return nil, end, err
	}
	return cls, closing, nil
}

// match returns the index of the bracket closing the one at i.
func (p *parser) match(i int) (int, error) {
	pairs := map[string]string{"(": ")", "[": "]", "{": "}"}
	stack := []string{pairs[p.toks[i].Text]}
	for j := i + 1; j < len(p.toks); j++ {
		if p.toks[j].Kind != Punct {
			continue
		}
		t := p.toks[j].Text
		if c, ok := pairs[t]; ok {
			stack = append(stack, c)
		} else if t == ")" || t == "]" || t == "}" {
			if t != stack[len(stack)-1] {
				return 0, fmt.Errorf("line %d: unbalanced %q", p.toks[j].Line, t)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return j, nil
			}
		}
	}
	return 0, fmt.Errorf("line %d: unclosed %q", p.toks[i].Line, p.toks[i].Text)
}

// members parses the class body toks[start:end].
func (p *parser) members(cls *Class, start, end int) error {
	i := start
	for i < end {
		m := &Member{Line: p.toks[i].Line, Start: p.toks[i].Pos}
		for p.is(i, "[") {
			closing, err := p.match(i)
			if err != nil {
				return err
			}
			m.Attributes = append(m.Attributes, p.attributes(i+1, closing)...)
			i = closing + 1
		}
		if i >= end {
			break
		}
		headerStart := i
		m.Line = p.toks[i].Line

		// Nested type declarations.
		if k := p.typeKeywordIn(headerStart, end); k >= 0 {
			nested, closing, err := p.class(k, end, cls)
			if err != nil {
				return err
			}
			if nested != nil {
				cls.Nested = append(cls.Nested, nested)
			}
			i = closing + 1
			continue
		}

		next, err := p.member(m, headerStart, end)
		if err != nil {
			return err
		}
		m.End = p.toks[next-1].End
		if m.Name != "" {
			cls.Members = append(cls.Members, m)
		}
		i = next
	}
	return nil
}

// typeKeywordIn reports the index of a class/struct/record keyword that starts
// a nested type declaration at i, or -1.
func (p *parser) typeKeywordIn(i, end int) int {
	for j := i; j < end; j++ {
		t := p.toks[j]
		if t.Kind != Ident {
			return -1
		}
		if typeKeywords[t.Text] {
			return j
		}
		switch t.Text {
		case "public", "private", "protected", "internal", "static", "sealed", "abstract", "partial", "readonly", "unsafe", "file", "new":
			continue
		default:
			return -1
		}
	}
	return -1
}

// member parses a member starting at i and returns the index after it.
func (p *parser) member(m *Member, i, end int) (int, error) {
	depth := 0
	for j := i; j < end; j++ {
		if p.toks[j].Kind != Punct {
			continue
		}
		switch t := p.toks[j].Text; {
		case t == "(" || t == "[":
			depth++
		case t == ")" || t == "]":
			depth--
		case depth > 0:
		case t == ";":
			m.Header = p.toks[i:j]
			p.header(m)
			return j + 1, nil
		case t == "=>":
			semi, err := p.until(j+1, end, ";")
			if err != nil {
				return end, err
			}
			m.Header = p.toks[i:j]
			m.Body = p.toks[j+1 : semi]
			p.header(m)
			return semi + 1, nil
		case t == "=":
			// Field or property initializer.
			semi, err := p.until(j+1, end, ";")
			if err != nil {
				return end, err
			}
			m.Header = p.toks[i:j]
			m.Body = p.toks[j+1 : semi]
			p.header(m)
			return semi + 1, nil
		case t == "{":
			closing, err := p.match(j)
			if err != nil {
				return end, err
			}
			m.Header = p.toks[i:j]
			m.Body = p.toks[j+1 : closing]
			p.header(m)
			next := closing + 1
			if p.is(next, "=") { // property initializer: { get; } = value;
				semi, err := p.until(next+1, end, ";")
				if err != nil {
					return end, err
				}
				next = semi + 1
			}
			return next, nil
		}
	}
	return end, nil
}

// until returns the index of the first text token at bracket depth 0 in
// toks[i:end].
func (p *parser) until(i, end int, text string) (int, error) {
	depth := 0
	for j := i; j < end; j++ {
		if p.toks[j].Kind != Punct {
			continue
		}
		switch t := p.toks[j].Text; t {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		default:
			if depth == 0 && t == text {
				return j, nil
			}
		}
	}
	return 0, fmt.Errorf("line %d: missing %q", p.toks[i].Line, text)
}

// header extracts the member name and, for methods, the parameters.
func (p *parser) header(m *Member) {
	h := m.Header
	for j, t := range h {
		if t.Kind == Punct && t.Text == "(" {
			nameIdx := j - 1
			if nameIdx >= 0 && h[nameIdx].Text == ">" { // generic method
				for d := 0; nameIdx >= 0; nameIdx-- {
					if h[nameIdx].Text == ">" {
						d++
					} else if h[nameIdx].Text == "<" {
						d--
						if d == 0 {
							nameIdx--
							break
						}
					}
				}
			}
			if nameIdx >= 0 && h[nameIdx].Kind == Ident {
				m.Name = h[nameIdx].Text
				m.IsMethod = true
				m.Params = parseParams(h[j+1 : len(h)-closingParenOffset(h, j)])
			}
			return
		}
	}
	for j := len(h) - 1; j >= 0; j-- {
		if h[j].Kind == Ident {
			m.Name = h[j].Text
			return
		}
	}
}

// closingParenOffset returns how many tokens follow the parameter list's
// closing parenthesis, given the opening parenthesis at open.
func closingParenOffset(h []Token, open int) int {
	depth := 0
	for j := open; j < len(h); j++ {
		switch h[j].Text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return len(h) - j
			}
		}
	}
	return 0
}

func parseParams(toks []Token) []Param {
	var params []Param
	for _, part := range SplitTop(toks) {
		if len(part) == 0 {
			continue
		}
		var prm Param
		eq := -1
		for j, t := range part {
			if t.Kind == Punct && t.Text == "=" {
				eq = j
				break
			}
		}
		decl := part
		if eq >= 0 {
			decl, prm.Default = part[:eq], part[eq+1:]
		}
		// Drop parameter attributes such as [NotNull].
		for len(decl) > 0 && decl[0].Text == "[" {
			k := 0
			for k < len(decl) && decl[k].Text != "]" {
				k++
			}
			decl = decl[min(k+1, len(decl)):]
		}
		if len(decl) == 0 {
			continue
		}
		prm.Name = decl[len(decl)-1].Text
		var typ []string
		for _, t := range decl[:len(decl)-1] {
			switch t.Text {
			case "params":
				prm.Params = true
			case "this", "ref", "out", "in", "scoped":
			default:
				typ = append(typ, t.Text)
			}
		}
		prm.Type = joinType(typ)
		params = append(params, prm)
	}
	return params
}

func joinType(parts []string) string {
	var b strings.Builder
	for i, s := range parts {
		if i > 0 && s != "?" && s != "[" && s != "]" && s != "<" && s != ">" && s != "," && s != "." &&
			parts[i-1] != "<" && parts[i-1] != "." && parts[i-1] != "[" {
			b.WriteByte(' ')
		}
		if s == "," {
			s = ", "
		}
		b.WriteString(s)
	}
	return b.String()
}

// attributes parses the attribute section toks[start:end] (inside "[ … ]").
func (p *parser) attributes(start, end int) []Attribute {
	toks := p.toks[start:end]
	if len(toks) >= 2 && toks[1].Text == ":" { // target specifier, e.g. [return: …]
		toks = toks[2:]
	}
	var attrs []Attribute
	for _, part := range SplitTop(toks) {
		if len(part) == 0 {
			continue
		}
		a := Attribute{Line: part[0].Line}
		j := 0
		for j < len(part) && part[j].Text != "(" {
			if part[j].Kind == Ident {
				a.Name = part[j].Text
			}
			j++
		}
		a.Name = strings.TrimSuffix(a.Name, "Attribute")
		if j < len(part) && part[len(part)-1].Text == ")" {
			a.Args = SplitArgs(part[j+1 : len(part)-1])
		}
		attrs = append(attrs, a)
	}
	return attrs
}

// SplitArgs splits an argument list into positional and named arguments.
func SplitArgs(toks []Token) []Arg {
	var args []Arg
	for _, part := range SplitTop(toks) {
		if len(part) == 0 {
			continue
		}
		if len(part) > 2 && part[0].Kind == Ident && part[1].Kind == Punct && (part[1].Text == "=" || part[1].Text == ":") {
			args = append(args, Arg{Name: part[0].Text, Expr: part[2:]})
			continue
		}
		args = append(args, Arg{Expr: part})
	}
	return args
}

// SplitTop splits toks on commas that are not nested in brackets or generic
// type argument lists.
func SplitTop(toks []Token) [][]Token {
	var parts [][]Token
	depth, last := 0, 0
	for j := 0; j < len(toks); j++ {
		t := toks[j]
		if t.Kind != Punct {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case "<":
			if k := genericClose(toks, j); k > 0 {
				j = k
			}
		case ",":
			if depth == 0 {
				parts = append(parts, toks[last:j])
				last = j + 1
			}
		}
	}
	if last < len(toks) || len(parts) > 0 {
		parts = append(parts, toks[last:])
	}
	return parts
}

// genericClose returns the index of the ">" closing a generic type argument
// list opened at i, or -1 when the "<" is a comparison.
func genericClose(toks []Token, i int) int {
	if i == 0 || toks[i-1].Kind != Ident {
		return -1
	}
	depth := 0
	for j := i; j < len(toks); j++ {
		t := toks[j]
		switch {
		case t.Text == "<":
			depth++
		case t.Text == ">":
			depth--
			if depth == 0 {
				return j
			}
		case t.Kind == Ident, t.Text == ",", t.Text == ".", t.Text == "?", t.Text == "[", t.Text == "]":
		default:
			return -1
		}
	}
	return -1
}

// Text returns the source text spanned by toks.
func (f *File) Text(toks []Token) string {
	if len(toks) == 0 {
		return ""
	}
	return f.Src[toks[0].Pos:toks[len(toks)-1].End]
}

// ParseAttributes parses a standalone attribute section such as
// `[InlineData("a", 1)]`, e.g. one found in a comment.
func ParseAttributes(text string) (*File, []Attribute, error) {
	toks, err := Lex(text)
	if err != nil {
		return nil, nil, err
	}
	if len(toks) < 2 || toks[0].Text != "[" {
		return nil, nil, fmt.Errorf("not an attribute section: %q", text)
	}
	p := &parser{toks: toks}
	closing, err := p.match(0)
	if err != nil {
		return nil, nil, err
	}
	return &File{Src: text}, p.attributes(1, closing), nil
}

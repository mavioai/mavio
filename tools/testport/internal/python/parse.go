package python

import (
	"strings"

	"github.com/mavioai/mavio/tools/testport/internal/csharp"
)

// Module is what Parse reads of a Python module.
type Module struct {
	src string
	// Constants are the module-level names assigned literal values.
	Constants map[string]csharp.Value
	// Marks are the decorators of the module's pytestmark, as tokens after
	// the "@".
	Marks [][]Token
	// Functions are the functions defined at module level and in classes,
	// in source order; nested functions are left out.
	Functions []*Function
}

// Function is a function defined at module level or in a class.
type Function struct {
	// Name is the function's name, preceded by its classes' names and a
	// dot for a method ("TestFoo.test_bar").
	Name string
	// Class is the name of its innermost class, empty at module level.
	Class string
	Line  int
	// Params are the names of its parameters.
	Params []string
	// Decorators are its decorators and its classes', innermost last, as
	// the tokens after each "@".
	Decorators [][]Token
}

// Parse reads the functions, constants and pytestmark of a module.
func Parse(src string) (*Module, error) {
	lines, err := Lines(src)
	if err != nil {
		return nil, err
	}
	m := &Module{src: src, Constants: map[string]csharp.Value{}}
	type block struct {
		def        bool
		name       string
		indent     int
		decorators [][]Token
	}
	var stack []block
	var pending [][]Token
	for _, l := range lines {
		for len(stack) > 0 && stack[len(stack)-1].indent >= l.Indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 && stack[len(stack)-1].def {
			continue // a function's body
		}
		t := l.Tokens
		if t[0].Kind == Op && t[0].Text == "@" {
			pending = append(pending, t[1:])
			continue
		}
		decorators := pending
		pending = nil
		if t[0].Kind == Name && t[0].Text == "async" && len(t) > 1 {
			t = t[1:]
		}
		if t[0].Kind != Name || len(t) < 2 || t[1].Kind != Name {
			if len(stack) == 0 {
				m.assign(t)
			}
			continue
		}
		switch t[0].Text {
		case "class":
			stack = append(stack, block{name: t[1].Text, indent: l.Indent, decorators: decorators})
		case "def":
			var names []string
			var inherited [][]Token
			for _, b := range stack {
				names = append(names, b.name)
				inherited = append(inherited, b.decorators...)
			}
			f := &Function{
				Name:       strings.Join(append(names, t[1].Text), "."),
				Line:       t[0].Line,
				Params:     params(t[2:]),
				Decorators: append(inherited, decorators...),
			}
			if len(names) > 0 {
				f.Class = names[len(names)-1]
			}
			m.Functions = append(m.Functions, f)
			stack = append(stack, block{def: true, name: t[1].Text, indent: l.Indent})
		}
	}
	return m, nil
}

// assign records a module-level "name = value": a literal value as a
// constant, a pytestmark's marks.
func (m *Module) assign(t []Token) {
	if len(t) < 3 || t[0].Kind != Name || t[1].Text != "=" {
		return
	}
	value := t[2:]
	if t[0].Text == "pytestmark" {
		if value[0].Text == "[" && value[len(value)-1].Text == "]" {
			m.Marks = append(m.Marks, Split(value[1:len(value)-1], ",")...)
		} else {
			m.Marks = append(m.Marks, value)
		}
		return
	}
	if v := m.Eval(value); Pure(v) {
		m.Constants[t[0].Text] = v
	} else {
		delete(m.Constants, t[0].Text)
	}
}

// params returns the parameter names of the tokens after a function's
// name, from its opening parenthesis.
func params(t []Token) []string {
	if len(t) == 0 || t[0].Text != "(" {
		return nil
	}
	end := closing(t, 0)
	var out []string
	for _, p := range Split(t[1:end], ",") {
		for len(p) > 0 && p[0].Kind == Op && (p[0].Text == "*" || p[0].Text == "**") {
			p = p[1:]
		}
		if len(p) > 0 && p[0].Kind == Name {
			out = append(out, p[0].Text)
		}
	}
	return out
}

// closing returns the index of the bracket closing the one at t[i], or
// len(t)-1 when it is not closed.
func closing(t []Token, i int) int {
	depth := 0
	for j := i; j < len(t); j++ {
		if t[j].Kind != Op {
			continue
		}
		switch t[j].Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(t) - 1
}

// Split splits t at the separator sep outside brackets, dropping an empty
// last part (a trailing separator).
func Split(t []Token, sep string) [][]Token {
	var out [][]Token
	depth, start := 0, 0
	for i, tok := range t {
		if tok.Kind != Op {
			continue
		}
		switch tok.Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case sep:
			if depth == 0 {
				out = append(out, t[start:i])
				start = i + 1
			}
		}
	}
	if start < len(t) {
		out = append(out, t[start:])
	}
	return out
}

// Arg is an argument of a call: its keyword, empty when positional, and
// its tokens.
type Arg struct {
	Keyword string
	Tokens  []Token
}

// Call reads t as a call of a dotted name, returning the name and the
// arguments; a bare dotted name is a call without arguments.
func Call(t []Token) (name string, args []Arg, ok bool) {
	i := 0
	for i < len(t) && t[i].Kind == Name {
		name += t[i].Text
		i++
		if i < len(t) && t[i].Text == "." {
			name += "."
			i++
			continue
		}
		break
	}
	if name == "" || strings.HasSuffix(name, ".") {
		return "", nil, false
	}
	if i == len(t) {
		return name, nil, true
	}
	if t[i].Text != "(" || closing(t, i) != len(t)-1 {
		return "", nil, false
	}
	for _, a := range Split(t[i+1:len(t)-1], ",") {
		if len(a) >= 2 && a[0].Kind == Name && a[1].Text == "=" {
			args = append(args, Arg{Keyword: a[0].Text, Tokens: a[2:]})
		} else {
			args = append(args, Arg{Tokens: a})
		}
	}
	return name, args, true
}

// Text returns the source text of t.
func (m *Module) Text(t []Token) string {
	if len(t) == 0 {
		return ""
	}
	return m.src[t[0].Pos:t[len(t)-1].End]
}

package csharp

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strings"
)

// Value is a JSON-encodable representation of a C# expression: string,
// json.Number, bool, nil, []any, Map, or one of the tagged forms below.
type Value = any

// Map is an ordered JSON object.
type Map []KV

// KV is one entry of a Map.
type KV struct {
	Key   string
	Value Value
}

// MarshalJSON encodes m as an object with keys in insertion order.
func (m Map) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range m {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		v, err := marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// Tagged forms. Keys start with "$" so they cannot be confused with data.
//
//	{"$symbol": "ExtraType.Trailer"}   enum members, constants, other identifiers
//	{"$type": "VideoFileInfo"}         typeof(…)
//	{"$new": "T", "args": […], "named": {…}, "init": {…}, "items": […]}
//	{"$expr": "TimeSpan.FromSeconds(1)"} anything that is not a constant
func symbol(s string) Map { return Map{{"$symbol", s}} }

// Eval converts the expression toks to a Value.
func (f *File) Eval(toks []Token) Value {
	if len(toks) == 0 {
		return nil
	}
	if v, ok := f.eval(toks); ok {
		return v
	}
	return Map{{"$expr", f.Text(toks)}}
}

func (f *File) eval(toks []Token) (Value, bool) {
	toks = stripParens(toks)
	if len(toks) == 0 {
		return nil, false
	}

	// Flag combinations: A.X | A.Y.
	if parts := splitOp(toks, "|"); len(parts) > 1 {
		var flags []Value
		for _, p := range parts {
			name, ok := dottedName(p)
			if !ok {
				return nil, false
			}
			flags = append(flags, name)
		}
		return Map{{"$flags", flags}}, true
	}

	// C# 12 collection expression: [a, b].
	if toks[0].Text == "[" && closeBracket(toks, 0) == len(toks)-1 {
		items := []Value{}
		for _, part := range SplitTop(toks[1 : len(toks)-1]) {
			if len(part) > 0 {
				items = append(items, f.Eval(part))
			}
		}
		return items, true
	}

	// Array.Empty<T>()
	if len(toks) >= 6 && toks[0].Text == "Array" && toks[1].Text == "." && toks[2].Text == "Empty" && toks[len(toks)-2].Text == "(" && toks[len(toks)-1].Text == ")" {
		return []Value{}, true
	}

	// String concatenation: "a" + "b".
	if parts := splitOp(toks, "+"); len(parts) > 1 {
		var b strings.Builder
		for _, p := range parts {
			v, ok := f.eval(p)
			s, isStr := v.(string)
			if !ok || !isStr {
				return nil, false
			}
			b.WriteString(s)
		}
		return b.String(), true
	}

	first := toks[0]
	switch {
	case len(toks) == 1 && (first.Kind == String || first.Kind == Char):
		return first.Value, true
	case len(toks) == 1 && first.Kind == Number:
		n, ok := number(first.Text)
		return n, ok
	case len(toks) == 2 && first.Text == "-" && toks[1].Kind == Number:
		n, ok := number(toks[1].Text)
		if !ok {
			return nil, false
		}
		return json.Number("-" + string(n)), true
	case len(toks) == 1 && first.Kind == Ident:
		switch first.Text {
		case "true":
			return true, true
		case "false":
			return false, true
		case "null", "default":
			return nil, true
		}
	case first.Kind == Ident && first.Text == "new":
		return f.evalNew(toks[1:])
	case first.Kind == Ident && (first.Text == "typeof" || first.Text == "nameof") && len(toks) > 3 && toks[1].Text == "(" && toks[len(toks)-1].Text == ")":
		inner := toks[2 : len(toks)-1]
		if first.Text == "typeof" {
			return Map{{"$type", f.Text(inner)}}, true
		}
		return inner[len(inner)-1].Text, true
	case first.Text == "(":
		// Cast: (T)expr.
		if closing := closeParen(toks, 0); closing > 0 && closing < len(toks)-1 && isTypeName(toks[1:closing]) {
			return f.eval(toks[closing+1:])
		}
	}

	if name, ok := dottedName(toks); ok {
		switch name {
		case "string.Empty", "String.Empty":
			return "", true
		}
		return symbol(name), true
	}
	return nil, false
}

func (f *File) evalNew(toks []Token) (Value, bool) {
	// new[] { … }
	if len(toks) >= 2 && toks[0].Text == "[" && toks[1].Text == "]" {
		return f.evalItems(toks[2:])
	}
	// Type name up to "(", "{" or "[".
	j := 0
	for j < len(toks) && toks[j].Text != "(" && toks[j].Text != "{" && toks[j].Text != "[" {
		if toks[j].Text == "<" {
			if k := genericClose(toks, j); k > 0 {
				j = k
			}
		}
		j++
	}
	typ := f.Text(toks[:j])
	rest := toks[j:]
	if len(rest) > 0 && rest[0].Text == "[" { // new T[] { … } or new T[n]
		closing := closeBracket(rest, 0)
		if closing < 0 {
			return nil, false
		}
		if closing == len(rest)-1 {
			if closing == 2 && rest[1].Text == "0" {
				return []Value{}, true // new T[0]
			}
			return nil, false // sized array without initializer
		}
		return f.evalItems(rest[closing+1:])
	}

	obj := Map{{"$new", typ}}
	if len(rest) > 0 && rest[0].Text == "(" {
		closing := closeParen(rest, 0)
		if closing < 0 {
			return nil, false
		}
		var args []Value
		var named Map
		for _, a := range SplitArgs(rest[1:closing]) {
			if a.Name != "" {
				named = append(named, KV{a.Name, f.Eval(a.Expr)})
			} else {
				args = append(args, f.Eval(a.Expr))
			}
		}
		if len(args) > 0 {
			obj = append(obj, KV{"args", args})
		}
		if len(named) > 0 {
			obj = append(obj, KV{"named", named})
		}
		rest = rest[closing+1:]
	}
	if len(rest) > 0 && rest[0].Text == "{" {
		closing := closeBracket(rest, 0)
		if closing != len(rest)-1 {
			return nil, false
		}
		var init Map
		var items []Value
		for _, a := range SplitArgs(rest[1:closing]) {
			if a.Name != "" {
				init = append(init, KV{a.Name, f.Eval(a.Expr)})
			} else {
				items = append(items, f.Eval(a.Expr))
			}
		}
		if len(init) > 0 {
			obj = append(obj, KV{"init", init})
		}
		if len(items) > 0 {
			obj = append(obj, KV{"items", items})
		}
		rest = nil
	}
	if len(rest) > 0 {
		return nil, false
	}
	return obj, true
}

// evalItems evaluates an array initializer "{ a, b }".
func (f *File) evalItems(toks []Token) (Value, bool) {
	if len(toks) < 2 || toks[0].Text != "{" || toks[len(toks)-1].Text != "}" {
		return nil, false
	}
	items := []Value{}
	for _, part := range SplitTop(toks[1 : len(toks)-1]) {
		if len(part) > 0 {
			items = append(items, f.Eval(part))
		}
	}
	return items, true
}

// number normalizes a C# numeric literal to a JSON number.
func number(text string) (json.Number, bool) {
	s := strings.ReplaceAll(text, "_", "")
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "0x") || strings.HasPrefix(lower, "0b") {
		base := 16
		if lower[1] == 'b' {
			base = 2
		}
		digits := strings.TrimRight(lower[2:], "ul")
		n, ok := new(big.Int).SetString(digits, base)
		if !ok {
			return "", false
		}
		return json.Number(n.String()), true
	}
	s = strings.TrimRight(s, "uUlLfFdDmM")
	if n, ok := new(big.Int).SetString(s, 10); ok { // also drops leading zeros
		return json.Number(n.String()), true
	}
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	if intPart, frac, ok := strings.Cut(s, "."); ok {
		if n, ok := new(big.Int).SetString(intPart, 10); ok {
			s = n.String() + "." + frac
		}
	}
	if !json.Valid([]byte(s)) {
		return "", false
	}
	return json.Number(s), true
}

func stripParens(toks []Token) []Token {
	for len(toks) >= 2 && toks[0].Text == "(" && closeParen(toks, 0) == len(toks)-1 {
		toks = toks[1 : len(toks)-1]
	}
	return toks
}

func closeParen(toks []Token, i int) int { return closeOf(toks, i, "(", ")") }
func closeBracket(toks []Token, i int) int {
	return closeOf(toks, i, toks[i].Text, map[string]string{"[": "]", "{": "}"}[toks[i].Text])
}

func closeOf(toks []Token, i int, open, closing string) int {
	depth := 0
	for j := i; j < len(toks); j++ {
		if toks[j].Kind != Punct {
			continue
		}
		switch toks[j].Text {
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// splitOp splits toks on a top-level binary operator.
func splitOp(toks []Token, op string) [][]Token {
	var parts [][]Token
	depth, last := 0, 0
	for j, t := range toks {
		if t.Kind != Punct {
			continue
		}
		switch t.Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
		case op:
			if depth == 0 && j > last {
				parts = append(parts, toks[last:j])
				last = j + 1
			}
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return append(parts, toks[last:])
}

// dottedName reports whether toks is a qualified name such as A.B.C.
func dottedName(toks []Token) (string, bool) {
	var b strings.Builder
	for j, t := range toks {
		if j%2 == 0 {
			if t.Kind != Ident {
				return "", false
			}
		} else if t.Text != "." {
			return "", false
		}
		b.WriteString(t.Text)
	}
	return b.String(), len(toks)%2 == 1
}

func isTypeName(toks []Token) bool {
	for _, t := range toks {
		if t.Kind != Ident && t.Text != "." && t.Text != "?" {
			return false
		}
	}
	return len(toks) > 0
}

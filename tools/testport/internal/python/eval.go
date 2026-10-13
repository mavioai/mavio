package python

import (
	"encoding/json"
	"math/big"
	"strings"

	"github.com/mavioai/mavio/tools/testport/internal/csharp"
)

// Eval converts the expression t to a value as package csharp represents
// them: strings, json.Number, bools, nil, []any for tuples, lists and
// sets, csharp.Map for dicts; names as {"$symbol": …}, unless a module
// constant gives their value; anything else as {"$expr": …} with its
// source text.
func (m *Module) Eval(t []Token) csharp.Value {
	if len(t) == 0 {
		return nil
	}
	if parts := Split(t, ","); len(parts) > 1 || len(parts) == 1 && t[len(t)-1].Text == "," {
		return m.items(parts)
	}
	first, last := t[0], t[len(t)-1]
	if first.Kind == Op && closing(t, 0) == len(t)-1 {
		inner := t[1 : len(t)-1]
		switch first.Text {
		case "(":
			if len(inner) == 0 {
				return []any{}
			}
			return m.Eval(inner)
		case "[":
			return m.items(Split(inner, ","))
		case "{":
			return m.dict(inner)
		}
	}
	if v, ok := m.literal(t); ok {
		return v
	}
	if name, ok := dotted(t); ok {
		if v, found := m.Constants[name]; found {
			return v
		}
		return csharp.Map{{Key: "$symbol", Value: name}}
	}
	return csharp.Map{{Key: "$expr", Value: m.src[first.Pos:last.End]}}
}

func (m *Module) items(parts [][]Token) []any {
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		out = append(out, m.Eval(p))
	}
	return out
}

// dict converts the inside of braces: a dict, or a set as a list.
func (m *Module) dict(inner []Token) csharp.Value {
	parts := Split(inner, ",")
	if len(parts) == 0 {
		return csharp.Map{}
	}
	var out csharp.Map
	for _, p := range parts {
		kv := Split(p, ":")
		if len(kv) != 2 {
			if out == nil {
				return m.items(parts) // a set
			}
			return csharp.Map{{Key: "$expr", Value: m.src[inner[0].Pos:inner[len(inner)-1].End]}}
		}
		key, ok := m.Eval(kv[0]).(string)
		if !ok {
			return csharp.Map{{Key: "$expr", Value: "{" + m.src[inner[0].Pos:inner[len(inner)-1].End] + "}"}}
		}
		out = append(out, csharp.KV{Key: key, Value: m.Eval(kv[1])})
	}
	return out
}

// literal converts strings (adjacent ones concatenated), numbers, signed
// numbers, True, False and None.
func (m *Module) literal(t []Token) (csharp.Value, bool) {
	if t[0].Kind == String {
		var b strings.Builder
		for _, s := range t {
			if s.Kind != String || strings.ContainsAny(s.Prefix, "bf") {
				return nil, false
			}
			b.WriteString(s.Value)
		}
		return b.String(), true
	}
	sign := ""
	if len(t) == 2 && t[0].Kind == Op && (t[0].Text == "-" || t[0].Text == "+") {
		sign, t = strings.TrimPrefix(t[0].Text, "+"), t[1:]
	}
	if len(t) != 1 {
		return nil, false
	}
	switch tok := t[0]; tok.Kind {
	case Number:
		return number(sign + tok.Text)
	case Name:
		if sign != "" {
			return nil, false
		}
		switch tok.Text {
		case "True":
			return true, true
		case "False":
			return false, true
		case "None":
			return nil, true
		}
	}
	return nil, false
}

// number converts a Python integer or float literal.
func number(text string) (csharp.Value, bool) {
	text = strings.ReplaceAll(text, "_", "")
	lower := strings.ToLower(text)
	if strings.HasSuffix(lower, "j") {
		return nil, false
	}
	neg := strings.HasPrefix(lower, "-")
	digits := strings.TrimPrefix(lower, "-")
	for prefix, base := range map[string]int{"0x": 16, "0o": 8, "0b": 2} {
		if rest, ok := strings.CutPrefix(digits, prefix); ok {
			n, ok := new(big.Int).SetString(rest, base)
			if !ok {
				return nil, false
			}
			if neg {
				n.Neg(n)
			}
			return json.Number(n.String()), true
		}
	}
	if _, ok := new(big.Float).SetString(lower); !ok {
		return nil, false
	}
	if strings.HasPrefix(digits, ".") {
		lower = strings.Replace(lower, ".", "0.", 1)
	}
	if strings.HasSuffix(lower, ".") {
		lower += "0"
	}
	return json.Number(lower), true
}

// dotted reads t as a name or a dotted name.
func dotted(t []Token) (string, bool) {
	var b strings.Builder
	for i, tok := range t {
		if i%2 == 0 && tok.Kind != Name || i%2 == 1 && tok.Text != "." {
			return "", false
		}
		b.WriteString(tok.Text)
	}
	return b.String(), len(t)%2 == 1
}

// Pure reports whether v holds no symbols and no expressions.
func Pure(v csharp.Value) bool {
	switch v := v.(type) {
	case []any:
		for _, e := range v {
			if !Pure(e) {
				return false
			}
		}
	case csharp.Map:
		for _, kv := range v {
			if strings.HasPrefix(kv.Key, "$") || !Pure(kv.Value) {
				return false
			}
		}
	}
	return true
}

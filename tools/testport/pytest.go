package testport

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/mavioai/mavio/tools/testport/internal/csharp"
	"github.com/mavioai/mavio/tools/testport/internal/python"
)

// ExtractPytest parses a pytest module and returns its tests as one
// TestClass named module: functions with @pytest.mark.parametrize as
// theories, keyed by argument name, the other test functions and methods
// as facts. Cases of pytest.param with skip or xfail marks, and tests
// marked so, or skipped by a condition, keep the reason. Argument values
// that are not literal lists are unsupported.
func ExtractPytest(src, module, origin string) (TestClass, error) {
	m, err := python.Parse(src)
	if err != nil {
		return TestClass{}, err
	}
	tc := TestClass{Class: module, Line: 1}
	moduleSkip := markSkip(m, m.Marks, origin)
	for _, f := range m.Functions {
		if !isPytest(f) {
			continue
		}
		skip := cmp.Or(moduleSkip, markSkip(m, f.Decorators, origin))
		var sets []paramSet
		var unsupported string
		for _, d := range f.Decorators {
			name, args, ok := python.Call(d)
			if !ok || !isMark(name, "parametrize") {
				continue
			}
			set, reason := parametrize(m, args, origin)
			if reason != "" {
				unsupported = reason
				break
			}
			sets = append(sets, set)
		}
		switch {
		case unsupported != "":
			tc.Unsupported = append(tc.Unsupported, Unsupported{Method: f.Name, Line: f.Line, Reason: unsupported})
		case len(sets) == 0:
			tc.Facts = append(tc.Facts, Fact{Method: f.Name, Line: f.Line, Skip: skip})
		default:
			tc.Theories = append(tc.Theories, theory(f, sets, skip))
		}
	}
	return tc, nil
}

// isPytest reports whether pytest collects f by its default rules: a
// function named test*, at module level or in a class named Test*.
func isPytest(f *python.Function) bool {
	base := f.Name[strings.LastIndex(f.Name, ".")+1:]
	if !strings.HasPrefix(base, "test") {
		return false
	}
	if f.Class == "" {
		return !strings.Contains(f.Name, ".")
	}
	return strings.HasPrefix(f.Class, "Test")
}

// isMark reports whether name is pytest's mark, as pytest.mark.<mark> or
// mark.<mark>.
func isMark(name, mark string) bool {
	return name == "pytest.mark."+mark || name == "mark."+mark
}

// markSkip returns the reason the marks of decorators skip a test: skip,
// xfail, or skipif with its condition.
func markSkip(m *python.Module, decorators [][]python.Token, origin string) string {
	for _, d := range decorators {
		name, args, ok := python.Call(d)
		if !ok {
			continue
		}
		if r := skipReason(m, name, args, origin); r != "" {
			return r
		}
	}
	return ""
}

func skipReason(m *python.Module, name string, args []python.Arg, origin string) string {
	reason := ""
	var cond []python.Token
	for _, a := range args {
		switch {
		case a.Keyword == "reason" || a.Keyword == "" && reason == "" && !isMark(name, "skipif"):
			if s, ok := m.Eval(a.Tokens).(string); ok {
				reason = s
			}
		case a.Keyword == "condition" || a.Keyword == "" && cond == nil:
			cond = a.Tokens
		}
	}
	switch {
	case isMark(name, "skip"):
		return cmp.Or(reason, "skipped in "+origin)
	case isMark(name, "xfail"):
		return "expected to fail in " + origin + suffix(reason)
	case isMark(name, "skipif"):
		return "skipped in " + origin + " when " + m.Text(cond) + suffix(reason)
	}
	return ""
}

func suffix(reason string) string {
	if reason == "" {
		return ""
	}
	return ": " + reason
}

// paramSet is the argument names and cases of one parametrize mark.
type paramSet struct {
	names []string
	cases []TheoryCase
}

// parametrize reads the arguments of a parametrize mark, or returns why it
// cannot.
func parametrize(m *python.Module, args []python.Arg, origin string) (paramSet, string) {
	var namesArg, valuesArg []python.Token
	for i, a := range args {
		switch {
		case a.Keyword == "argnames" || a.Keyword == "" && i == 0:
			namesArg = a.Tokens
		case a.Keyword == "argvalues" || a.Keyword == "" && i == 1:
			valuesArg = a.Tokens
		}
	}
	var set paramSet
	switch v := m.Eval(namesArg).(type) {
	case string:
		for _, n := range strings.Split(v, ",") {
			set.names = append(set.names, strings.TrimSpace(n))
		}
	case []any:
		for _, n := range v {
			s, ok := n.(string)
			if !ok {
				return set, "argument names that are not strings"
			}
			set.names = append(set.names, s)
		}
	default:
		return set, "argument names that are not strings"
	}
	if len(valuesArg) == 0 || valuesArg[0].Text != "[" && valuesArg[0].Text != "(" {
		return set, "argument values computed at run time: " + m.Text(valuesArg)
	}
	for _, item := range python.Split(valuesArg[1:len(valuesArg)-1], ",") {
		c := TheoryCase{Line: item[0].Line}
		values := item
		if name, pargs, ok := python.Call(item); ok && (name == "pytest.param" || name == "param") {
			values = nil
			var positional [][]python.Token
			for _, a := range pargs {
				switch a.Keyword {
				case "":
					positional = append(positional, a.Tokens)
				case "marks":
					c.Skip = paramMarks(m, a.Tokens, origin)
				}
			}
			if len(set.names) == 1 && len(positional) == 1 {
				values = positional[0]
			} else {
				c.Args = csharp.Map{}
				if len(positional) != len(set.names) {
					return set, fmt.Sprintf("line %d: %d values for %d arguments", c.Line, len(positional), len(set.names))
				}
				for i, n := range set.names {
					c.Args = append(c.Args, csharp.KV{Key: n, Value: m.Eval(positional[i])})
				}
			}
		}
		if values != nil {
			v := m.Eval(values)
			if len(set.names) == 1 {
				c.Args = csharp.Map{{Key: set.names[0], Value: v}}
			} else {
				list, ok := v.([]any)
				if !ok || len(list) != len(set.names) {
					return set, fmt.Sprintf("line %d: values that are not a tuple of %d", c.Line, len(set.names))
				}
				c.Args = csharp.Map{}
				for i, n := range set.names {
					c.Args = append(c.Args, csharp.KV{Key: n, Value: list[i]})
				}
			}
		}
		set.cases = append(set.cases, c)
	}
	return set, ""
}

// paramMarks returns the reason the marks of a pytest.param skip it.
func paramMarks(m *python.Module, t []python.Token, origin string) string {
	marks := [][]python.Token{t}
	if len(t) > 0 && (t[0].Text == "[" || t[0].Text == "(") {
		marks = python.Split(t[1:len(t)-1], ",")
	}
	return markSkip(m, marks, origin)
}

// theory combines the parametrize marks of f, the last applied first as
// pytest does, into one theory whose cases are their product.
func theory(f *python.Function, sets []paramSet, skip string) Theory {
	th := Theory{Method: f.Name, Line: f.Line, Skip: skip}
	cases := []TheoryCase{{}}
	for i := len(sets) - 1; i >= 0; i-- {
		s := sets[i]
		for _, n := range s.names {
			th.Params = append(th.Params, Param{Name: n})
		}
		var next []TheoryCase
		for _, a := range cases {
			for _, b := range s.cases {
				c := TheoryCase{Line: b.Line, Skip: cmp.Or(a.Skip, b.Skip)}
				c.Args = append(append(csharp.Map{}, a.Args...), b.Args...)
				next = append(next, c)
			}
		}
		cases = next
	}
	for i := range cases {
		cases[i].ID = fmt.Sprintf("%s/%d", f.Name, i+1)
	}
	th.Cases = cases
	return th
}

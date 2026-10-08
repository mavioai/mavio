// Package testport extracts test cases and test assets from Jellyfin's C#
// test projects into language-neutral JSON for Mavio's Go tests.
package testport

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mavioai/mavio/tools/testport/internal/csharp"
)

// TestClass is the extraction result for one C# test class.
type TestClass struct {
	Class       string        `json:"class"`
	Line        int           `json:"line"`
	Theories    []Theory      `json:"theories,omitempty"`
	Facts       []Fact        `json:"facts,omitempty"`
	Unsupported []Unsupported `json:"unsupported,omitempty"`
}

// Theory is a parameterized test method and its cases.
type Theory struct {
	Method string       `json:"method"`
	Line   int          `json:"line"`
	Params []Param      `json:"params"`
	Skip   string       `json:"skip,omitempty"`
	Cases  []TheoryCase `json:"cases"`
}

// Param describes a theory parameter.
type Param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TheoryCase is one set of arguments for a theory.
type TheoryCase struct {
	ID   string     `json:"id"`
	Line int        `json:"line"`
	Args csharp.Map `json:"args"`
	Skip string     `json:"skip,omitempty"`
	From string     `json:"from,omitempty"` // MemberData source member
}

// Fact is a non-parameterized test, listed so it can be ported by hand.
type Fact struct {
	Method string `json:"method"`
	Line   int    `json:"line"`
	Skip   string `json:"skip,omitempty"`
}

// Unsupported records test data that could not be extracted statically.
type Unsupported struct {
	Method string `json:"method"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// Extract parses a C# test file and returns its test classes, including
// nested classes, in source order. Classes without tests are omitted.
func Extract(src string) ([]TestClass, error) {
	f, err := csharp.Parse(src)
	if err != nil {
		return nil, err
	}
	var out []TestClass
	var walk func(cls *csharp.Class, prefix string)
	walk = func(cls *csharp.Class, prefix string) {
		tc := extractClass(f, cls)
		tc.Class = prefix + cls.Name
		if len(tc.Theories)+len(tc.Facts)+len(tc.Unsupported) > 0 {
			out = append(out, tc)
		}
		for _, n := range cls.Nested {
			walk(n, tc.Class+".")
		}
	}
	for _, cls := range f.Classes {
		walk(cls, "")
	}
	return out, nil
}

// ExtractConstants parses a C# file and returns the values of its const
// fields, keyed by "Class.Name" (nested classes as "Outer.Inner.Name"), in
// source order.
func ExtractConstants(src string) (csharp.Map, error) {
	f, err := csharp.Parse(src)
	if err != nil {
		return nil, err
	}
	var out csharp.Map
	var walk func(cls *csharp.Class, prefix string)
	walk = func(cls *csharp.Class, prefix string) {
		name := prefix + cls.Name
		for _, m := range cls.Members {
			if m.IsMethod || m.Body == nil || !slices.ContainsFunc(m.Header, func(t csharp.Token) bool {
				return t.Kind == csharp.Ident && t.Text == "const"
			}) {
				continue
			}
			out = append(out, csharp.KV{Key: name + "." + m.Name, Value: f.Eval(m.Body)})
		}
		for _, n := range cls.Nested {
			walk(n, name+".")
		}
	}
	for _, cls := range f.Classes {
		walk(cls, "")
	}
	return out, nil
}

func extractClass(f *csharp.File, cls *csharp.Class) TestClass {
	tc := TestClass{Line: cls.Line}
	seen := map[string]int{}
	prevEnd := cls.BodyStart
	for _, m := range cls.Members {
		leading := f.Src[prevEnd:m.Start]
		prevEnd = m.End
		if !m.IsMethod {
			continue
		}
		var theory, fact *csharp.Attribute
		for i := range m.Attributes {
			switch m.Attributes[i].Name {
			case "Theory":
				theory = &m.Attributes[i]
			case "Fact":
				fact = &m.Attributes[i]
			}
		}
		// Overloads get a numeric suffix so case IDs stay unique.
		method := m.Name
		if seen[m.Name]++; seen[m.Name] > 1 {
			method = fmt.Sprintf("%s_%d", m.Name, seen[m.Name])
		}
		switch {
		case fact != nil:
			tc.Facts = append(tc.Facts, Fact{Method: method, Line: m.Line, Skip: namedString(f, fact.Args, "Skip")})
		case theory != nil:
			th, unsupported := extractTheory(f, cls, m, method, leading+f.Src[m.Start:m.Header[0].Pos])
			th.Skip = namedString(f, theory.Args, "Skip")
			if len(th.Cases) > 0 {
				tc.Theories = append(tc.Theories, th)
			}
			tc.Unsupported = append(tc.Unsupported, unsupported...)
		}
	}
	return tc
}

// commentedInlineData matches an InlineData attribute that was commented out,
// optionally with a label: "// TODO: [InlineData(…)]".
var commentedInlineData = regexp.MustCompile(`(?m)^[ \t]*//[ \t]*(?:([A-Za-z]+)[ \t]*:[ \t]*)?(\[InlineData\b.*)$`)

// extractTheory extracts the cases of theory m. attrText is the source text
// before the method header, including its attributes and leading comments.
func extractTheory(f *csharp.File, cls *csharp.Class, m *csharp.Member, method, attrText string) (Theory, []Unsupported) {
	th := Theory{Method: method, Line: m.Line, Cases: []TheoryCase{}}
	for _, p := range m.Params {
		th.Params = append(th.Params, Param{Name: p.Name, Type: p.Type})
	}
	var unsupported []Unsupported
	add := func(line int, args csharp.Map, skip, from string) {
		th.Cases = append(th.Cases, TheoryCase{Line: line, Args: args, Skip: skip, From: from})
	}
	inline := func(af *csharp.File, a csharp.Attribute, line int, skip string) {
		var positional []csharp.Arg
		for _, arg := range a.Args {
			if arg.Name == "" {
				positional = append(positional, arg)
			}
		}
		if skip == "" {
			skip = namedString(af, a.Args, "Skip")
		}
		add(line, bindArgs(af, m.Params, positional), skip, "")
	}

	// InlineData cases that Jellyfin commented out (typically known failures)
	// are kept as skipped cases.
	startLine := m.Line - strings.Count(attrText, "\n")
	for _, loc := range commentedInlineData.FindAllStringSubmatchIndex(attrText, -1) {
		label, text := "", attrText[loc[4]:loc[5]]
		if loc[2] >= 0 {
			label = attrText[loc[2]:loc[3]]
		}
		af, attrs, err := csharp.ParseAttributes(text)
		if err != nil || len(attrs) != 1 {
			continue
		}
		reason := "commented out in Jellyfin"
		if label != "" {
			reason += " (" + label + ")"
		}
		inline(af, attrs[0], startLine+strings.Count(attrText[:loc[0]], "\n"), reason)
	}

	for _, a := range m.Attributes {
		switch a.Name {
		case "InlineData":
			inline(f, a, a.Line, "")
		case "MemberData":
			name, rows, reason := memberData(f, cls, a)
			if reason != "" {
				unsupported = append(unsupported, Unsupported{Method: method, Line: a.Line, Reason: reason})
				continue
			}
			for _, r := range rows {
				add(r.line, bindArgs(f, m.Params, r.args), "", name)
			}
		case "ClassData":
			unsupported = append(unsupported, Unsupported{Method: method, Line: a.Line, Reason: "ClassData is not supported; port by hand"})
		}
	}

	// Inline cases in source order, MemberData rows after them; IDs follow
	// that order.
	slices.SortStableFunc(th.Cases, func(a, b TheoryCase) int {
		if (a.From == "") != (b.From == "") {
			if a.From == "" {
				return -1
			}
			return 1
		}
		if a.From == "" {
			return a.Line - b.Line
		}
		return 0
	})
	for i := range th.Cases {
		th.Cases[i].ID = fmt.Sprintf("%s/%d", method, i+1)
	}
	return th, unsupported
}

// bindArgs maps positional arguments onto parameter names, filling in
// declared defaults and collecting params arrays.
func bindArgs(f *csharp.File, params []csharp.Param, args []csharp.Arg) csharp.Map {
	out := csharp.Map{}
	for i, p := range params {
		switch {
		case p.Params:
			rest := []csharp.Value{}
			for _, a := range args[min(i, len(args)):] {
				rest = append(rest, f.Eval(a.Expr))
			}
			out = append(out, csharp.KV{Key: p.Name, Value: rest})
		case i < len(args):
			out = append(out, csharp.KV{Key: p.Name, Value: f.Eval(args[i].Expr)})
		case p.Default != nil:
			out = append(out, csharp.KV{Key: p.Name, Value: f.Eval(p.Default)})
		}
	}
	// Arguments without a matching parameter are kept under positional keys.
	variadic := len(params) > 0 && params[len(params)-1].Params
	for i := len(params); i < len(args) && !variadic; i++ {
		out = append(out, csharp.KV{Key: fmt.Sprintf("$%d", i), Value: f.Eval(args[i].Expr)})
	}
	return out
}

func namedString(f *csharp.File, args []csharp.Arg, name string) string {
	for _, a := range args {
		if a.Name == name {
			if s, ok := f.Eval(a.Expr).(string); ok {
				return s
			}
			return f.Text(a.Expr)
		}
	}
	return ""
}

type row struct {
	line int
	args []csharp.Arg
}

// memberData resolves a [MemberData(nameof(X))] attribute to the rows
// produced by member X. It returns a reason when the data cannot be
// extracted statically.
func memberData(f *csharp.File, cls *csharp.Class, a csharp.Attribute) (string, []row, string) {
	var nameArg []csharp.Token
	for _, arg := range a.Args {
		switch {
		case arg.Name == "" && nameArg == nil:
			nameArg = arg.Expr
		case arg.Name == "MemberType":
			return "", nil, "MemberData from another type (" + f.Text(arg.Expr) + ") is not supported; port by hand"
		case arg.Name == "":
			return "", nil, "parameterized MemberData is not supported; port by hand"
		}
	}
	name, ok := f.Eval(nameArg).(string)
	if !ok {
		return "", nil, "MemberData name is not a constant: " + f.Text(nameArg)
	}
	m := cls.Find(name)
	if m == nil {
		return name, nil, "MemberData member " + name + " not found"
	}
	body := m.Body
	for _, t := range body {
		if t.Kind == csharp.Ident && slices.Contains([]string{"foreach", "for", "while", "Select", "Range", "GetValues"}, t.Text) {
			return name, nil, "data for " + name + " is computed (loop or query); port by hand"
		}
	}

	var rows []row
	// data.Add(a, b) — TheoryData.
	for i := 0; i+3 < len(body); i++ {
		if body[i].Text == "." && body[i+1].Text == "Add" && body[i+2].Text == "(" {
			end := closing(body, i+2)
			if end < 0 {
				break
			}
			rows = append(rows, row{line: body[i+1].Line, args: csharp.SplitArgs(body[i+3 : end])})
			i = end
		}
	}
	// yield return new object[] { a, b };
	for i := 0; i+1 < len(body); i++ {
		if body[i].Text == "yield" && body[i+1].Text == "return" {
			end := i + 2
			for end < len(body) && body[end].Text != ";" {
				end++
			}
			rows = append(rows, row{line: body[i].Line, args: arrayArgs(body[i+2 : end])})
			i = end
		}
	}
	// Collection initializers: new TheoryData<…> { { a, b }, … },
	// new List<object[]> { new object[] { … }, … }, new[] { new object[] { … } }.
	if len(rows) == 0 {
		rows = collectionRows(body)
	}
	if len(rows) == 0 {
		return name, nil, "no static data found in " + name + "; port by hand"
	}
	return name, rows, ""
}

func closing(toks []csharp.Token, open int) int {
	depth := 0
	for j := open; j < len(toks); j++ {
		if toks[j].Kind != csharp.Punct {
			continue
		}
		switch toks[j].Text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// arrayArgs turns "new object[] { a, b }" or "new[] { a, b }" into the
// argument list a, b; any other expression is a single argument.
func arrayArgs(toks []csharp.Token) []csharp.Arg {
	if len(toks) > 0 && toks[0].Text == "new" {
		for j := 1; j < len(toks); j++ {
			if toks[j].Text == "{" {
				if strings.Contains(textOf(toks[1:j]), "[") && closing(toks, j) == len(toks)-1 {
					return csharp.SplitArgs(toks[j+1 : len(toks)-1])
				}
				break
			}
			if toks[j].Text == "(" {
				break
			}
		}
	}
	return []csharp.Arg{{Expr: toks}}
}

func collectionRows(body []csharp.Token) []row {
	for i := 0; i < len(body); i++ {
		if body[i].Text != "new" {
			continue
		}
		j := i + 1
		for j < len(body) && body[j].Text != "{" && body[j].Text != "(" && body[j].Text != ";" {
			j++
		}
		if j < len(body) && body[j].Text == "(" {
			if end := closing(body, j); end > 0 {
				j = end + 1
			}
		}
		if j >= len(body) || body[j].Text != "{" {
			continue
		}
		typ := textOf(body[i+1 : j])
		if !strings.Contains(typ, "TheoryData") && !strings.Contains(typ, "object") && !strings.HasPrefix(typ, "[") {
			continue
		}
		end := closing(body, j)
		if end < 0 {
			return nil
		}
		var rows []row
		for _, elem := range csharp.SplitTop(body[j+1 : end]) {
			if len(elem) == 0 {
				continue
			}
			if elem[0].Text == "{" && closing(elem, 0) == len(elem)-1 {
				rows = append(rows, row{line: elem[0].Line, args: csharp.SplitArgs(elem[1 : len(elem)-1])})
				continue
			}
			rows = append(rows, row{line: elem[0].Line, args: arrayArgs(elem)})
		}
		return rows
	}
	return nil
}

func textOf(toks []csharp.Token) string {
	var b strings.Builder
	for _, t := range toks {
		b.WriteString(t.Text)
	}
	return b.String()
}

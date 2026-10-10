package cardigann

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dlclark/regexp2"
)

// Vars are the variables of templates, by name with their leading dot:
// nil, strings, or lists of strings for ranges.
type Vars map[string]any

// get returns a variable, failing for undefined ones as Prowlarr's
// dictionary does.
func (v Vars) get(name string) (any, error) {
	val, ok := v[name]
	if !ok {
		return nil, fmt.Errorf("the given key '%s' was not present in the dictionary", name)
	}
	return val, nil
}

// str returns a variable as a string; ok is false for nil and other
// types, as C#'s "as string" is.
func (v Vars) str(name string) (string, bool, error) {
	val, err := v.get(name)
	if err != nil {
		return "", false, err
	}
	s, ok := val.(string)
	return s, ok, nil
}

// clone returns a copy of the variables.
func (v Vars) clone() Vars {
	out := make(Vars, len(v))
	for k, val := range v {
		out[k] = val
	}
	return out
}

// isBlank reports whether s is empty or white space, as .NET's
// IsNullOrWhiteSpace does.
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func mustCompile(expr string) *regexp2.Regexp {
	return regexp2.MustCompile(expr, regexp2.None)
}

var (
	reReplaceTemplate = mustCompile(`{{\s*re_replace\s+(\..+?)\s+"(.*?)"\s+"(.*?)"\s*}}`)
	joinTemplate      = mustCompile(`{{\s*join\s+(\..+?)\s+"(.*?)"\s*}}`)
	// A logic function and two or more variables, optionally in
	// parentheses, or string literals.
	logicTemplate    = mustCompile(`\b(and|or|eq|ne)(?:\s+(\(?\.[^\)\s]+\)?|"[^"]+")){2,}`)
	ifElseTemplate   = mustCompile(`{{\s*if\s*(.+?)\s*}}(.*?){{\s*else\s*}}(.*?){{\s*end\s*}}`)
	rangeTemplate    = mustCompile(`{{\s*range\s*(((?<index>\$.+?),)((\s*(?<element>.+?)\s*(:=)\s*)))?(?<variable>.+?)\s*}}(?<prefix>.*?){{\.}}(?<postfix>.*?){{end}}`)
	variableTemplate = mustCompile(`{{\s*(\..+?)\s*}}`)
)

// maxLogicSteps bounds the rewriting of logic functions.
const maxLogicSteps = 1000

// eachMatch calls f for each match of re in s, in order; the matches are
// those of s as given, whatever f does.
func eachMatch(re *regexp2.Regexp, s string, f func(m *regexp2.Match) error) error {
	m, err := re.FindStringMatch(s)
	for err == nil && m != nil {
		if err = f(m); err != nil {
			return err
		}
		m, err = re.FindNextMatch(m)
	}
	return err
}

// group returns a group's text; "" when it did not match.
func group(m *regexp2.Match, n int) string {
	if g := m.GroupByNumber(n); g != nil && len(g.Captures) > 0 {
		return g.String()
	}
	return ""
}

func namedGroup(m *regexp2.Match, name string) string {
	if g := m.GroupByName(name); g != nil && len(g.Captures) > 0 {
		return g.String()
	}
	return ""
}

// applyTemplate expands a template as Prowlarr's ApplyGoTemplateText
// does: not Go's templates, but the forms definitions use, each expanded
// in turn by regular expressions. modify, if not nil, is applied to the
// values put in. Without variables, the base variables are used.
func (e *engine) applyTemplate(template string, vars Vars, modify func(string) string) (string, error) {
	if isBlank(template) || !strings.Contains(template, "{{") {
		return template, nil
	}
	if vars == nil {
		var err error
		if vars, err = e.baseVariables(); err != nil {
			return "", err
		}
	}
	if modify == nil {
		modify = func(s string) string { return s }
	}

	// {{ re_replace .Query.Keywords "[^a-zA-Z0-9]+" "%" }}
	t := template
	err := eachMatch(reReplaceTemplate, template, func(m *regexp2.Match) error {
		input, _, err := vars.str(group(m, 1))
		if err != nil {
			return err
		}
		re, err := regexp2.Compile(group(m, 2), regexp2.None)
		if err != nil {
			return err
		}
		expanded, err := re.Replace(input, group(m, 3), -1, -1)
		if err != nil {
			return err
		}
		t = strings.ReplaceAll(t, m.String(), modify(expanded))
		return nil
	})
	if err != nil {
		return "", err
	}

	// {{ join .Categories "," }}
	template = t
	err = eachMatch(joinTemplate, template, func(m *regexp2.Match) error {
		list, err := stringList(vars, group(m, 1))
		if err != nil {
			return err
		}
		t = strings.ReplaceAll(t, m.String(), modify(strings.Join(list, group(m, 2))))
		return nil
	})
	if err != nil {
		return "", err
	}

	if t, err = applyLogic(t, vars); err != nil {
		return "", err
	}

	// {{ if .Var }}...{{ else }}...{{ end }}
	template = t
	err = eachMatch(ifElseTemplate, template, func(m *regexp2.Match) error {
		cond := group(m, 1)
		if !strings.HasPrefix(cond, ".") {
			return fmt.Errorf("condition operation '%s' not implemented", cond)
		}
		v, err := vars.get(cond)
		if err != nil {
			return err
		}
		var holds bool
		switch v := v.(type) {
		case nil:
		case string:
			holds = !isBlank(v)
		case []string:
			holds = len(v) > 0
		case []int:
			holds = len(v) > 0
		default:
			return fmt.Errorf("unexpected type for variable %s: %T", cond, v)
		}
		result := group(m, 3)
		if holds {
			result = group(m, 2)
		}
		t = strings.ReplaceAll(t, m.String(), result)
		return nil
	})
	if err != nil {
		return "", err
	}

	// {{ range $i, $e := .Categories }}...{{.}}...{{end}}
	template = t
	err = eachMatch(rangeTemplate, template, func(m *regexp2.Match) error {
		list, err := stringList(vars, namedGroup(m, "variable"))
		if err != nil {
			return err
		}
		index := namedGroup(m, "index")
		prefix, postfix := namedGroup(m, "prefix"), namedGroup(m, "postfix")
		var b strings.Builder
		for i, v := range list {
			if !isBlank(index) {
				placeholder := "{{" + index + "}}"
				b.WriteString(strings.ReplaceAll(prefix, placeholder, strconv.Itoa(i)))
				b.WriteString(modify(v))
				b.WriteString(strings.ReplaceAll(postfix, placeholder, strconv.Itoa(i)))
			} else {
				b.WriteString(prefix + modify(v) + postfix)
			}
		}
		t = strings.ReplaceAll(t, m.String(), b.String())
		return nil
	})
	if err != nil {
		return "", err
	}

	// {{ .Var }}
	template = t
	err = eachMatch(variableTemplate, template, func(m *regexp2.Match) error {
		v, err := vars.get(group(m, 1))
		if err != nil {
			return err
		}
		var s string
		switch v := v.(type) {
		case nil:
		case string:
			s = v
		default:
			s = fmt.Sprint(v)
		}
		t = strings.ReplaceAll(t, m.String(), modify(s))
		return nil
	})
	if err != nil {
		return "", err
	}
	return t, nil
}

// stringList returns a variable that must be a list of strings.
func stringList(vars Vars, name string) ([]string, error) {
	v, err := vars.get(name)
	if err != nil {
		return nil, err
	}
	list, ok := v.([]string)
	if !ok {
		return nil, fmt.Errorf("variable %s is not a list of strings but %T", name, v)
	}
	return list, nil
}

// applyLogic replaces the logic functions and, or, eq and ne by their
// results, innermost first: and and or by the name of a variable, eq and
// ne by .True or .False.
func applyLogic(template string, vars Vars) (string, error) {
	runes := []rune(template)
	for range maxLogicSteps {
		m, err := logicTemplate.FindRunesMatch(runes)
		if err != nil {
			return "", err
		}
		if m == nil {
			return string(runes), nil
		}
		start, length := m.Index, m.Length
		name := group(m, 1)
		caps := m.GroupByNumber(2).Captures
		params := make([]string, 0, len(caps))
		for _, c := range caps {
			params = append(params, strings.Trim(c.String(), "()"))
		}
		var result string
		switch name {
		case "and", "or":
			// The first blank variable for and, the first other for or,
			// else the last; literals are dropped.
			params = slices.DeleteFunc(params, func(p string) bool { return strings.HasPrefix(p, `"`) })
			isAnd := name == "and"
			for _, p := range params {
				result = p
				s, _, err := vars.str(p)
				if err != nil {
					return "", err
				}
				if isBlank(s) == isAnd {
					break
				}
			}
		case "eq", "ne":
			// They take two parameters; the rest is left.
			if len(params) > 2 {
				length = caps[2].Index - start
			}
			values := make([]*string, 2)
			for i, p := range params[:2] {
				if strings.HasPrefix(p, `"`) {
					s := strings.Trim(p, `"`)
					values[i] = &s
					continue
				}
				s, ok, err := vars.str(p)
				if err != nil {
					return "", err
				}
				if ok {
					values[i] = &s
				}
			}
			equal := (values[0] == nil) == (values[1] == nil) && (values[0] == nil || *values[0] == *values[1])
			result = ".False"
			if equal == (name == "eq") {
				result = ".True"
			}
		}
		runes = slices.Concat(runes[:start], []rune(result), runes[start+length:])
	}
	return "", errors.New("too many logic functions in template")
}

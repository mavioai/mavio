package cardigann

import (
	"fmt"
	"strings"

	"github.com/dlclark/regexp2"
	"golang.org/x/net/html"
)

// handleSelector selects a value from an element, as Prowlarr's
// HandleSelector does: a templated text, or an element's attribute, text
// or the value of the first case it matches, filtered. ok is false when
// nothing is selected and the value is optional.
func (e *engine) handleSelector(s *SelectorBlock, dom *html.Node, vars Vars, required bool) (string, bool, error) {
	if s.Text != nil {
		v, err := e.applyTemplate(*s.Text, vars, nil)
		if err != nil {
			return "", false, err
		}
		v, err = e.applyFilters(v, s.Filters, vars)
		return v, err == nil, err
	}
	selection := dom
	if s.Selector != nil {
		sel, err := e.applyTemplate(*s.Selector, vars, nil)
		if err != nil {
			return "", false, err
		}
		ok, err := e.sel.matches(dom, sel)
		if err != nil {
			return "", false, err
		}
		if !ok {
			if selection, err = e.sel.query(dom, sel); err != nil {
				return "", false, err
			}
		}
		if selection == nil {
			if required {
				return "", false, fmt.Errorf("selector %q didn't match %s", sel, outerHTML(dom))
			}
			return "", false, nil
		}
	}
	if s.Remove != nil {
		nodes, err := e.sel.queryAll(selection, *s.Remove)
		if err != nil {
			return "", false, err
		}
		for _, n := range nodes {
			if n.Parent != nil {
				n.Parent.RemoveChild(n)
			}
		}
	}
	var value string
	switch {
	case s.Case != nil:
		found := false
		for _, c := range s.Case {
			ok, err := e.sel.matches(selection, c.Key)
			if err != nil {
				return "", false, err
			}
			if !ok {
				n, err := e.sel.query(selection, c.Key)
				if err != nil {
					return "", false, err
				}
				ok = n != nil
			}
			if ok {
				if value, err = e.applyTemplate(c.Value, vars, nil); err != nil {
					return "", false, err
				}
				found = true
				break
			}
		}
		if !found {
			if required {
				return "", false, fmt.Errorf("none of the case selectors %s matched %s", caseKeys(s.Case), outerHTML(selection))
			}
			return "", false, nil
		}
	case s.Attribute != nil:
		v, ok := attribute(selection, *s.Attribute)
		if !ok {
			if required {
				return "", false, fmt.Errorf("attribute %q is not set for element %s", *s.Attribute, outerHTML(selection))
			}
			return "", false, nil
		}
		value = v
	default:
		value = textContent(selection)
	}
	v, err := e.applyFilters(strings.TrimSpace(value), s.Filters, vars)
	return v, err == nil, err
}

func caseKeys(c Ordered[string]) string {
	keys := make([]string, 0, len(c))
	for _, e := range c {
		keys = append(keys, "["+e.Key+", "+e.Value+"]")
	}
	return `"` + strings.Join(keys, ",") + `"`
}

// handleJSONSelector selects a value from a JSON object, as Prowlarr's
// HandleJsonSelector does: arrays are joined by commas.
func (e *engine) handleJSONSelector(s *SelectorBlock, parent any, vars Vars, required bool) (string, bool, error) {
	if s.Text != nil {
		v, err := e.applyTemplate(*s.Text, vars, nil)
		if err != nil {
			return "", false, err
		}
		v, err = e.applyFilters(v, s.Filters, vars)
		return v, err == nil, err
	}
	var value *string
	if s.Selector != nil {
		sel, err := e.applyTemplate(strings.TrimLeft(*s.Selector, "."), vars, nil)
		if err != nil {
			return "", false, err
		}
		field, ok, err := e.jsonFieldSelector(parent, sel)
		if err != nil {
			return "", false, err
		}
		var selection any
		found := false
		if ok {
			if selection, found, err = selectToken(parent, field); err != nil {
				return "", false, err
			}
		}
		if !found {
			if required {
				return "", false, fmt.Errorf("selector %q didn't match JSON content %s", sel, truncate(jsonText(parent), 5*1024))
			}
			return "", false, nil
		}
		if a, ok := selection.([]any); ok {
			parts := make([]string, 0, len(a))
			for _, v := range a {
				parts = append(parts, jsonText(v))
			}
			v := strings.Join(parts, ",")
			value = &v
		} else {
			v, ok, err := jsonString(selection)
			if err != nil {
				return "", false, err
			}
			if ok {
				value = &v
			}
		}
	}
	if s.Case != nil {
		for _, c := range s.Case {
			if (value != nil && *value == c.Key) || c.Key == "*" {
				v, err := e.applyTemplate(c.Value, vars, nil)
				if err != nil {
					return "", false, err
				}
				value = &v
				break
			}
		}
		if value == nil {
			if required {
				return "", false, fmt.Errorf("none of the case selectors %s matched JSON content %s", caseKeys(s.Case), truncate(jsonText(parent), 5*1024))
			}
			return "", false, nil
		}
	}
	if value == nil {
		return "", false, nil
	}
	v, err := e.applyFilters(strings.TrimSpace(*value), s.Filters, vars)
	return v, err == nil, err
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// jsonSelectorFilter matches the filters of JSON selectors, such as
// :has(key), :not(key) and :contains(text).
var jsonSelectorFilter = regexp2.MustCompile(`\:(?<filter>.+?)\((?<key>.+?)\)(?=:|\z)`, regexp2.None)

// jsonFieldSelector returns the path part of a JSON selector, ok being
// false when the selected value does not pass its filters.
func (e *engine) jsonFieldSelector(parsed any, selector string) (string, bool, error) {
	path, _, _ := strings.Cut(selector, ":")
	obj := parsed
	if !isBlank(path) {
		v, found, err := selectToken(parsed, path)
		if err != nil {
			return "", false, err
		}
		if !found {
			return "", false, nil
		}
		obj = v
	}
	pass := true
	err := eachMatch(jsonSelectorFilter, selector, func(m *regexp2.Match) error {
		if !pass {
			return nil
		}
		filter, key := namedGroup(m, "filter"), namedGroup(m, "key")
		var has bool
		switch filter {
		case "has", "not":
			if inner, _ := jsonSelectorFilter.MatchString(key); inner {
				_, ok, err := e.jsonFieldSelector(obj, key)
				if err != nil {
					return err
				}
				has = ok
			} else {
				_, ok, err := selectToken(obj, key)
				if err != nil {
					return err
				}
				has = ok
			}
			if has != (filter == "has") {
				pass = false
			}
		case "contains":
			if !strings.Contains(jsonText(obj), key) {
				pass = false
			}
		default:
			e.log.Error("cardigann: unsupported selector", "definition", e.def.ID, "selector", selector)
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return path, pass, nil
}

// jsonRows returns the rows a JSON rows selector selects: the objects of
// the array at its path that pass its filters; ok is false when the path
// selects no array of objects.
func (e *engine) jsonRows(parsed any, selector string) ([]any, bool) {
	selector, err := e.applyTemplate(selector, nil, nil)
	if err != nil {
		e.log.Debug("cardigann: JSON rows not parsed", "selector", selector, "error", err)
		return nil, false
	}
	path, _, _ := strings.Cut(selector, ":")
	v, found, err := selectToken(parsed, path)
	if err != nil || !found {
		return nil, false
	}
	rows, ok := v.([]any)
	if !ok {
		return nil, false
	}
	var out []any
	for _, r := range rows {
		if _, ok := r.(*jsonObject); !ok {
			return nil, false
		}
		_, pass, err := e.jsonFieldSelector(r, selector[len(path):])
		if err != nil {
			return nil, false
		}
		if pass {
			out = append(out, r)
		}
	}
	return out, true
}

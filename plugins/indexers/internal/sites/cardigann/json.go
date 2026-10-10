package cardigann

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The JSON values read: nil, bool, jsonNumber, string, jsonDate, *jsonObject
// and []any, as Newtonsoft's JTokens are.
type (
	jsonNumber string
	// jsonDate is a string Newtonsoft reads as a date.
	jsonDate struct {
		raw string
		t   time.Time
	}
	jsonObject struct {
		keys   []string
		values map[string]any
	}
)

// parseJSON reads a JSON document keeping the order of objects' keys.
func parseJSON(s string, loc *time.Location) (any, error) {
	d := json.NewDecoder(strings.NewReader(s))
	d.UseNumber()
	v, err := readJSON(d, loc)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("additional text after the JSON value")
	}
	return v, nil
}

func readJSON(d *json.Decoder, loc *time.Location) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := &jsonObject{values: map[string]any{}}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return nil, err
				}
				key := k.(string)
				v, err := readJSON(d, loc)
				if err != nil {
					return nil, err
				}
				if _, ok := o.values[key]; !ok {
					o.keys = append(o.keys, key)
				}
				o.values[key] = v
			}
			_, err := d.Token()
			return o, err
		case '[':
			a := []any{}
			for d.More() {
				v, err := readJSON(d, loc)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			_, err := d.Token()
			return a, err
		}
	case json.Number:
		return jsonNumber(t), nil
	case string:
		if dt, ok := isoDate(t, loc); ok {
			return jsonDate{raw: t, t: dt}, nil
		}
		return t, nil
	}
	return tok, nil
}

// isoDatePattern matches the dates Newtonsoft reads from strings.
var isoDatePattern = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(\.\d{1,7})?(Z|[+-]\d{2}(?::?\d{2})?)?$`)

// isoDate reads an ISO 8601 date as Newtonsoft does: in UTC with Z, in
// loc with an offset, as is without.
func isoDate(s string, loc *time.Location) (time.Time, bool) {
	m := isoDatePattern.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	var nsec int
	if m[7] != "" {
		f := (m[7][1:] + "000000000")[:9]
		nsec, _ = strconv.Atoi(f)
	}
	zone := time.UTC
	switch z := m[8]; z {
	case "", "Z":
	default:
		h, _ := strconv.Atoi(z[1:3])
		mins := 0
		if rest := strings.TrimPrefix(z[3:], ":"); rest != "" {
			mins, _ = strconv.Atoi(rest)
		}
		off := h*3600 + mins*60
		if z[0] == '-' {
			off = -off
		}
		zone = time.FixedZone("", off)
	}
	t := time.Date(n(1), time.Month(n(2)), n(3), n(4), n(5), n(6), nsec, zone)
	if t.Month() != time.Month(n(2)) || t.Day() != n(3) {
		return time.Time{}, false
	}
	if z := m[8]; z != "" && z != "Z" {
		t = t.In(loc)
	}
	return t, true
}

// jsonString returns a value as Newtonsoft's Value<string> does; ok is
// false for null, and an error for objects and arrays.
func jsonString(v any) (string, bool, error) {
	switch v := v.(type) {
	case nil:
		return "", false, nil
	case string:
		return v, true, nil
	case jsonNumber:
		return dotNetNumber(string(v)), true, nil
	case bool:
		if v {
			return "True", true, nil
		}
		return "False", true, nil
	case jsonDate:
		return v.t.Format("01/02/2006 15:04:05"), true, nil
	}
	return "", false, fmt.Errorf("cannot read %s as a string", jsonKind(v))
}

// jsonText returns a value as JToken.ToString does: values as strings,
// objects and arrays as indented JSON.
func jsonText(v any) string {
	switch v.(type) {
	case *jsonObject, []any:
		var b bytes.Buffer
		writeJSON(&b, v, 0)
		return b.String()
	}
	s, _, _ := jsonString(v)
	return s
}

func writeJSON(b *bytes.Buffer, v any, depth int) {
	indent := func(d int) {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("  ", d))
	}
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case jsonNumber:
		b.WriteString(string(v))
	case string:
		writeJSONString(b, v)
	case jsonDate:
		writeJSONString(b, v.raw)
	case *jsonObject:
		if len(v.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, k := range v.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			indent(depth + 1)
			writeJSONString(b, k)
			b.WriteString(": ")
			writeJSON(b, v.values[k], depth+1)
		}
		indent(depth)
		b.WriteByte('}')
	case []any:
		if len(v) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			indent(depth + 1)
			writeJSON(b, e, depth+1)
		}
		indent(depth)
		b.WriteByte(']')
	}
}

func writeJSONString(b *bytes.Buffer, s string) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b.Truncate(b.Len() - 1)
}

func jsonKind(v any) string {
	switch v.(type) {
	case *jsonObject:
		return "an object"
	case []any:
		return "an array"
	}
	return "a value"
}

// dotNetNumber formats a JSON number as .NET does: integers as written,
// other numbers as the shortest double, in exponent form from 1E+15 and
// below 1E-05.
func dotNetNumber(raw string) string {
	if !strings.ContainsAny(raw, ".eE") {
		return raw
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsInf(f, 0) {
		return raw
	}
	if f == 0 {
		return "0"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	mant, exp, _ := strings.Cut(e, "e")
	x, _ := strconv.Atoi(exp)
	if x >= -5 && x < 15 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	sign := "+"
	if x < 0 {
		sign, x = "-", -x
	}
	return fmt.Sprintf("%sE%s%02d", mant, sign, x)
}

// selectToken selects a value by a JSON path, as Newtonsoft's SelectToken
// does: nil when nothing matches, an error when several do.
func selectToken(root any, path string) (any, bool, error) {
	matches, err := selectTokens(root, path)
	if err != nil {
		return nil, false, err
	}
	switch len(matches) {
	case 0:
		return nil, false, nil
	case 1:
		return matches[0], true, nil
	}
	return nil, false, errors.New("path returned multiple tokens")
}

// selectTokens selects the values a JSON path matches: properties by
// .name or ['name'], array items by [index], all children by * and
// descendants by ..name.
func selectTokens(root any, path string) ([]any, error) {
	cur := []any{root}
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "$")
	first := true
	for p != "" {
		recursive := false
		switch {
		case strings.HasPrefix(p, ".."):
			recursive = true
			p = p[2:]
		case strings.HasPrefix(p, "."):
			p = p[1:]
		case strings.HasPrefix(p, "["):
		case !first:
			return nil, fmt.Errorf("unexpected character in path %q", path)
		}
		first = false
		if recursive {
			var all []any
			for _, c := range cur {
				all = appendDescendants(all, c)
			}
			cur = all
			if p == "" || p[0] == '[' {
				continue
			}
		}
		if strings.HasPrefix(p, "[") {
			end := strings.IndexByte(p, ']')
			if end < 0 {
				return nil, fmt.Errorf("unterminated [ in path %q", path)
			}
			inner := strings.TrimSpace(p[1:end])
			p = p[end+1:]
			switch {
			case inner == "*":
				cur = children(cur)
			case strings.HasPrefix(inner, "'") || strings.HasPrefix(inner, `"`):
				name := strings.Trim(inner, `'"`)
				cur = properties(cur, name)
			default:
				i, err := strconv.Atoi(inner)
				if err != nil {
					return nil, fmt.Errorf("unsupported [%s] in path %q", inner, path)
				}
				var next []any
				for _, c := range cur {
					if a, ok := c.([]any); ok {
						if i < 0 {
							i += len(a)
						}
						if i >= 0 && i < len(a) {
							next = append(next, a[i])
						}
					}
				}
				cur = next
			}
			continue
		}
		end := strings.IndexAny(p, ".[")
		if end < 0 {
			end = len(p)
		}
		name := p[:end]
		p = p[end:]
		if name == "*" {
			cur = children(cur)
			continue
		}
		cur = properties(cur, name)
	}
	return cur, nil
}

func properties(cur []any, name string) []any {
	var next []any
	for _, c := range cur {
		if o, ok := c.(*jsonObject); ok {
			if v, ok := o.values[name]; ok {
				next = append(next, v)
			}
		}
	}
	return next
}

func children(cur []any) []any {
	var next []any
	for _, c := range cur {
		switch c := c.(type) {
		case *jsonObject:
			for _, k := range c.keys {
				next = append(next, c.values[k])
			}
		case []any:
			next = append(next, c...)
		}
	}
	return next
}

func appendDescendants(out []any, v any) []any {
	out = append(out, v)
	for _, c := range children([]any{v}) {
		out = appendDescendants(out, c)
	}
	return out
}

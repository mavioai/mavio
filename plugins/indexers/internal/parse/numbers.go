package parse

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	spaces = regexp.MustCompile(`\s+`)
	imdbID = regexp.MustCompile(`^(?:tt)?(\d{1,8})$`)
)

// NormalizeSpaces trims s and collapses its runs of white space to one
// space.
func NormalizeSpaces(s string) string {
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}

// normalizeNumber keeps the digits and separators of s. An integer's
// separators are all thousands separators, unless it has both kinds; a
// number's last separator is its decimal point.
func normalizeNumber(s string, integer bool) string {
	v := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == '.' || r == ',' {
			return r
		}
		return -1
	}, s)
	if v == "" {
		return "0"
	}
	if integer {
		if strings.Contains(v, ",") && strings.Contains(v, ".") {
			return v
		}
		return strings.ReplaceAll(v, ".", ",")
	}
	v = strings.ReplaceAll(v, ",", ".")
	if i := strings.LastIndex(v, "."); strings.Count(v, ".") > 1 {
		v = strings.ReplaceAll(v[:i], ".", "") + v[i:]
	}
	return v
}

// Float reads a number such as "2.222,22", "2,222.22" or "1000 grabs".
func Float(s string) (float64, error) {
	f, err := strconv.ParseFloat(normalizeNumber(s, false), 64)
	if err != nil {
		return 0, fmt.Errorf("parse number %q: %w", s, err)
	}
	return f, nil
}

// Int reads an integer such as "2.222", "2,222", "2 222" or "1000 grabs".
func Int(s string) (int64, error) {
	v := normalizeNumber(s, true)
	// Commas are thousands separators and a point the decimal point, whose
	// fraction must be zero.
	v = strings.ReplaceAll(v, ",", "")
	if whole, frac, ok := strings.Cut(v, "."); ok {
		if strings.Trim(frac, "0") != "" {
			return 0, fmt.Errorf("parse integer %q: has a fraction", s)
		}
		v = whole
	}
	if v == "" {
		v = "0"
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse integer %q: %w", s, err)
	}
	return n, nil
}

// LeadingInt reads the first run of digits in s, as in "asdf123asdf"; ok
// is false when s is blank.
func LeadingInt(s string) (n int64, ok bool) {
	if strings.TrimSpace(s) == "" {
		return 0, false
	}
	start := strings.IndexFunc(s, isASCIIDigit)
	if start < 0 {
		return 0, true
	}
	end := strings.IndexFunc(s[start:], func(r rune) bool { return !isASCIIDigit(r) })
	if end < 0 {
		end = len(s) - start
	}
	n, err := strconv.ParseInt(s[start:start+end], 10, 64)
	return n, err == nil
}

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

// Bytes reads a size such as "1,023.4 MB", "4.2 GiB" or "123456"; the
// units are binary whether or not they are written with an i.
func Bytes(s string) (int64, error) {
	unit := strings.ToLower(strings.ReplaceAll(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, s), "i", ""))
	v, err := Float(s)
	if err != nil {
		return 0, err
	}
	switch {
	case strings.Contains(unit, "kb"):
		v *= 1024
	case strings.Contains(unit, "mb"):
		v = v * 1024 * 1024
	case strings.Contains(unit, "gb"):
		v = v * 1024 * 1024 * 1024
	case strings.Contains(unit, "tb"):
		v = v * 1024 * 1024 * 1024 * 1024
	}
	return int64(v), nil
}

// IMDbID reads an IMDb ID such as "tt0183790" or "183790" into its
// number; ok is false for anything else.
func IMDbID(s string) (id int, ok bool) {
	m := imdbID.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	id, err := strconv.Atoi(m[1])
	return id, err == nil
}

// FullIMDbID returns an IMDb ID as "tt" and at least seven digits; "" for
// anything that is not one or is zero.
func FullIMDbID(s string) string {
	id, ok := IMDbID(s)
	if !ok || id == 0 {
		return ""
	}
	return fmt.Sprintf("tt%07d", id)
}

// QueryArg returns the first value of a URL's query argument; "" when the
// URL has none.
func QueryArg(rawURL, name string) string {
	_, q, ok := strings.Cut(rawURL, "?")
	if !ok {
		return ""
	}
	q, _, _ = strings.Cut(q, "#")
	values, _ := url.ParseQuery(q)
	return values.Get(name)
}

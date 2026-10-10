// Package jsonx reads JSON values as leniently as Newtonsoft.Json, which
// Prowlarr reads indexers' APIs with: numbers into strings, strings into
// numbers and booleans, 0 and 1 into booleans. Each type takes a JSON
// null as its zero value; pointers to them stay nil for null.
package jsonx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

var null = []byte("null")

// String is a string, a number's text or a boolean's "True" or "False".
type String string

// UnmarshalJSON implements json.Unmarshaler.
func (s *String) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, null):
		*s = ""
	case len(b) > 0 && b[0] == '"':
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = String(v)
	case bytes.Equal(b, []byte("true")):
		*s = "True"
	case bytes.Equal(b, []byte("false")):
		*s = "False"
	case len(b) > 0 && (b[0] == '-' || b[0] >= '0' && b[0] <= '9'):
		*s = String(b)
	default:
		return fmt.Errorf("cannot read %.40s as a string", b)
	}
	return nil
}

// text returns a scalar's text, ok false for null.
func text(b []byte) (string, bool, error) {
	var s String
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, null) {
		return "", false, nil
	}
	if err := s.UnmarshalJSON(b); err != nil {
		return "", false, err
	}
	return string(s), true, nil
}

// Int is a 32-bit integer, or a string of one.
type Int int

// UnmarshalJSON implements json.Unmarshaler.
func (n *Int) UnmarshalJSON(b []byte) error {
	v, err := readInt(b, 32)
	*n = Int(v)
	return err
}

// Int64 is a 64-bit integer, or a string of one.
type Int64 int64

// UnmarshalJSON implements json.Unmarshaler.
func (n *Int64) UnmarshalJSON(b []byte) error {
	v, err := readInt(b, 64)
	*n = Int64(v)
	return err
}

func readInt(b []byte, bits int) (int64, error) {
	s, ok, err := text(b)
	if err != nil || !ok {
		return 0, err
	}
	s = strings.TrimSpace(s)
	n, err := strconv.ParseInt(s, 10, bits)
	if err == nil {
		return n, nil
	}
	// Numbers with fractions of zero, such as 5.0, are integers.
	limit := math.Ldexp(1, bits-1)
	if f, ferr := strconv.ParseFloat(s, 64); ferr == nil && f == math.Trunc(f) && f >= -limit && f < limit {
		return int64(f), nil
	}
	return 0, fmt.Errorf("could not convert %q to an integer", s)
}

// Float is a number, or a string of one.
type Float float64

// UnmarshalJSON implements json.Unmarshaler.
func (f *Float) UnmarshalJSON(b []byte) error {
	s, ok, err := text(b)
	if err != nil || !ok {
		*f = 0
		return err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return fmt.Errorf("could not convert %q to a number", s)
	}
	*f = Float(v)
	return nil
}

// Bool is a boolean, a number, true when not 0, or a string of either.
type Bool bool

// UnmarshalJSON implements json.Unmarshaler.
func (v *Bool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case bytes.Equal(b, null), bytes.Equal(b, []byte("false")):
		*v = false
		return nil
	case bytes.Equal(b, []byte("true")):
		*v = true
		return nil
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		switch t := strings.TrimSpace(s); {
		case strings.EqualFold(t, "true"):
			*v = true
		case strings.EqualFold(t, "false"):
			*v = false
		default:
			return fmt.Errorf("string %q was not recognized as a valid boolean", s)
		}
		return nil
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("cannot read %.40s as a boolean", b)
	}
	*v = f != 0
	return nil
}

// Time is a date, in UTC: strings without a zone are taken to be UTC.
type Time struct{ time.Time }

// UnmarshalJSON implements json.Unmarshaler.
func (t *Time) UnmarshalJSON(b []byte) error {
	s, ok, err := text(b)
	if err != nil || !ok || s == "" {
		t.Time = time.Time{}
		return err
	}
	v, err := indexer.ParseDate(s)
	if err != nil {
		return err
	}
	t.Time = v
	return nil
}

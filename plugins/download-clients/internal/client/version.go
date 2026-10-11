package client

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Version is a client's version as .NET's Version: two to four numbers.
// Versions compare as .NET's do, a missing number below zero.
type Version []int

// ParseVersion reads a version as .NET's Version.Parse: two to four
// numbers separated by dots, with spaces around them allowed.
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, fmt.Errorf("unable to parse version %q", s)
	}
	v := make(Version, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("unable to parse version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

// AtLeast reports whether v is the version of the numbers given or later.
func (v Version) AtLeast(numbers ...int) bool { return slices.Compare(v, numbers) >= 0 }

// String writes the version back.
func (v Version) String() string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ".")
}

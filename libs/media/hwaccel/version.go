package hwaccel

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a dotted version number of two to four components. The zero
// Version is unknown.
type Version struct {
	parts [4]int
	n     int
}

// NewVersion returns the version with the given components; it panics
// unless there are two to four of them.
func NewVersion(parts ...int) Version {
	if len(parts) < 2 || len(parts) > 4 {
		panic("hwaccel: a version has two to four components")
	}
	var v Version
	v.n = copy(v.parts[:], parts)
	return v
}

// ParseVersionString parses "major.minor[.build[.revision]]".
func ParseVersionString(s string) (Version, bool) {
	fields := strings.Split(s, ".")
	if len(fields) < 2 || len(fields) > 4 {
		return Version{}, false
	}
	var v Version
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 || f[0] == '+' || f[0] == '-' {
			return Version{}, false
		}
		v.parts[i] = n
	}
	v.n = len(fields)
	return v, true
}

// IsZero reports whether v is unknown.
func (v Version) IsZero() bool { return v.n == 0 }

// Major returns the first component.
func (v Version) Major() int { return v.parts[0] }

// Minor returns the second component.
func (v Version) Minor() int { return v.parts[1] }

// Compare returns -1, 0 or +1 as v is older than, equal to or newer than w.
// A missing component sorts before any present one, so 4.4 < 4.4.0.
func (v Version) Compare(w Version) int {
	for i := range 4 {
		a, b := v.component(i), w.component(i)
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		}
	}
	return 0
}

func (v Version) component(i int) int {
	if i >= v.n {
		return -1
	}
	return v.parts[i]
}

// String formats v as dotted components; the zero Version is "unknown".
func (v Version) String() string {
	if v.n == 0 {
		return "unknown"
	}
	s := make([]string, v.n)
	for i := range v.n {
		s[i] = strconv.Itoa(v.parts[i])
	}
	return strings.Join(s, ".")
}

// MinVersion returns the oldest supported ffmpeg release, 4.4. Builds that
// do not state a release are accepted when their libraries are at least those
// of 4.4, see [ParseVersion].
func MinVersion() Version { return NewVersion(4, 4) }

// minLibraryVersions are the library versions of ffmpeg 4.4, see
// https://ffmpeg.org/download.html.
var minLibraryVersions = []struct {
	name    string
	version Version
}{
	{"libavutil", NewVersion(56, 70)},
	{"libavcodec", NewVersion(58, 134)},
	{"libavformat", NewVersion(58, 76)},
	{"libavdevice", NewVersion(58, 13)},
	{"libavfilter", NewVersion(7, 110)},
	{"libswscale", NewVersion(5, 9)},
	{"libswresample", NewVersion(3, 9)},
}

var (
	releasePattern = regexp.MustCompile(`^ffmpeg version n?((?:[0-9]+\.?)+)`)
	libraryPattern = regexp.MustCompile(`(lib\w+)\s+([0-9]+)\.\s*([0-9]+)`)
)

// ParseVersion returns the ffmpeg version stated in the output of
// "ffmpeg -version". Git builds state no release; for them it returns
// [MinVersion] when every library is at least as new as 4.4's, and
// the zero Version otherwise.
func ParseVersion(output string) Version {
	if m := releasePattern.FindStringSubmatch(output); m != nil {
		if v, ok := ParseVersionString(m[1]); ok {
			return v
		}
	}
	libs := map[string]Version{}
	for _, m := range libraryPattern.FindAllStringSubmatch(output, -1) {
		major, err1 := strconv.Atoi(m[2])
		minor, err2 := strconv.Atoi(m[3])
		if _, seen := libs[m[1]]; !seen && err1 == nil && err2 == nil {
			libs[m[1]] = NewVersion(major, minor)
		}
	}
	for _, lib := range minLibraryVersions {
		if v, ok := libs[lib.name]; !ok || v.Compare(lib.version) < 0 {
			return Version{}
		}
	}
	return MinVersion()
}

// ErrUnsupportedVersion is returned by [CheckVersion] for builds that are
// not ffmpeg or older than [MinVersion].
var ErrUnsupportedVersion = errors.New("unsupported ffmpeg")

// CheckVersion checks the output of "ffmpeg -version" for a supported build.
func CheckVersion(output string) error {
	if strings.Contains(strings.ToLower(output), "libav developers") {
		return fmt.Errorf("%w: avconv is not ffmpeg", ErrUnsupportedVersion)
	}
	v := ParseVersion(output)
	if v.IsZero() {
		return fmt.Errorf("%w: unknown version, %s or newer is required", ErrUnsupportedVersion, MinVersion())
	}
	if v.Compare(MinVersion()) < 0 {
		return fmt.Errorf("%w: version %s, %s or newer is required", ErrUnsupportedVersion, v, MinVersion())
	}
	return nil
}

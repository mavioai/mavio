package library

import "testing"

func TestIgnorePatternsCases(t *testing.T) {
	portedCases(t, "ignore_patterns.json", ported{
		run: map[string]func(t *testing.T, a args){
			"PathIgnored": func(t *testing.T, a args) {
				if got, want := IgnoredPath(a.str(t, "path")), a.boolean(t, "expected"); got != want {
					t.Errorf("got = %v, want = %v", got, want)
				}
			},
		},
	})
}

func TestMatchSegment(t *testing.T) {
	tests := []struct {
		pat, s string
		want   bool
	}{
		{"*.sample.???", "movie.SAMPLE.mkv", true},
		{"*.sample.???", "movie.sample.webm", false},
		{"sample.?", "sample.", false},
		{"*", "", true},
		{"a*b*c", "aXXbYc", true},
		{"a*b*c", "aXXbY", false},
	}
	for _, tt := range tests {
		if got := matchSegment(tt.pat, tt.s); got != tt.want {
			t.Errorf("matchSegment(%q, %q): got = %v, want = %v", tt.pat, tt.s, got, tt.want)
		}
	}
}

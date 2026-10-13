package titles

import "testing"

func TestSanitize(t *testing.T) {
	for in, want := range map[string]string{
		"Marvel's Agents of S.H.I.E.L.D.": "marvels agents of s h i e l d",
		"Stargate: Origins":               "stargate origins",
		"  Mayans  M.C. (2018) ":          "mayans m c 2018",
		"Grey’s Anatomy":                  "greys anatomy",
	} {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) got = %q, want = %q", in, got, want)
		}
	}
}

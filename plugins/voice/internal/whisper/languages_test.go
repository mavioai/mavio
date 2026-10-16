package whisper

import (
	"slices"
	"testing"
)

func TestLanguages(t *testing.T) {
	for _, c := range []struct{ tag, code string }{
		{"en-US", "en"}, {"jv", "jw"}, {"gsw", "de"}, {"zh-Hant-TW", "zh"}, {"haw", "haw"}, {"tlh", ""},
	} {
		if got := Code(c.tag); got != c.code {
			t.Errorf("Code(%q) = %q, want = %q", c.tag, got, c.code)
		}
	}
	for _, c := range []struct{ s, tag string }{
		{"jw", "jv"}, {"french", "fr"}, {"Haitian Creole", "ht"}, {"und", ""},
	} {
		if got := Tag(c.s); got != c.tag {
			t.Errorf("Tag(%q) = %q, want = %q", c.s, got, c.tag)
		}
	}
	if tags := Tags(); len(tags) != len(languages)+1 || !slices.Contains(tags, "gsw") || !slices.Contains(tags, "jv") {
		t.Errorf("Tags = %v", tags)
	}
}

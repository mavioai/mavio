package translator

import (
	"strings"
	"testing"
)

func TestMatch(t *testing.T) {
	langs := append(Codes("en", "fr", "zh-Hans", "zh-Hant", "pt-BR", "pt-PT", "nb", "sr-Cyrl", "sr-Latn"), Language{Code: "kazlat", Tag: "kk-Latn"})
	for _, c := range []struct {
		tag, want string
		ok        bool
	}{
		{"fr", "fr", true},
		{"FR", "fr", true},
		{"en-GB", "en", true},
		{"zh", "zh-Hans", true},
		{"zh-CN", "zh-Hans", true},
		{"zh-TW", "zh-Hant", true},
		{"pt", "pt-BR", true},
		{"pt-PT", "pt-PT", true},
		{"no", "nb", true},
		{"sr", "sr-Cyrl", true},
		{"kk-Latn", "kazlat", true},
		{"de", "", false},
		{"tlh", "", false},
		{"x-unknown", "", false},
	} {
		if got, ok := Match(c.tag, langs); got != c.want || ok != c.ok {
			t.Errorf("Match(%q) got = %q, %v, want = %q, %v", c.tag, got, ok, c.want, c.ok)
		}
	}
}

func TestTags(t *testing.T) {
	got := Tags([]Language{{Code: "en"}, {Code: "kazlat", Tag: "kk-Latn"}, {Code: "uzbcyr"}})
	if strings.Join(got, ",") != "en,kk-Latn" {
		t.Errorf("Tags got = %v, want = en,kk-Latn", got)
	}
}

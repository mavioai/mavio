package translator

import "testing"

func TestCode(t *testing.T) {
	convert := map[string]string{"he": "iw", "zh": "zh-CN", "zt": "zh-TW", "pb": "pt-BR"}
	for _, c := range []struct{ tag, want string }{
		{"fr", "fr"},
		{"he", "iw"},
		{"zh", "zh-CN"},
		{"zh-Hans", "zh-CN"},
		{"zh-Hant", "zh-TW"},
		{"zh-TW", "zh-TW"},
		{"pt-BR", "pt-BR"},
		{"pt", "pt"},
		{"pt-PT", "pt"},
		{"fil", "fil"},
	} {
		if got := Code(c.tag, convert); got != c.want {
			t.Errorf("Code(%q) got = %q, want = %q", c.tag, got, c.want)
		}
	}
}

func TestName(t *testing.T) {
	for _, c := range []struct{ tag, want string }{
		{"fr", "French"},
		{"pt-BR", "Brazilian Portuguese"},
		{"zh-Hant", "Traditional Chinese"},
	} {
		if got := Name(c.tag); got != c.want {
			t.Errorf("Name(%q) got = %q, want = %q", c.tag, got, c.want)
		}
	}
}

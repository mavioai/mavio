package library

import "testing"

func TestLanguageFinder(t *testing.T) {
	f := newLanguageFinder()
	tests := []struct {
		token, want string
		ok          bool
	}{
		{"en", "eng", true},
		{"eng", "eng", true},
		{"English", "eng", true},
		{"french", "fre", true},
		{"de", "ger", true},
		{"ger", "ger", true},
		{"fre", "fre", true},
		{"sdh", "", false},
		{"cc", "", false},
		{"deu", "ger", true},
		{"pt-BR", "pt-BR", true},
		{"chs", "zh-Hans", true},
		{"sc", "zh-Hans", true},
		{"简体", "zh-Hans", true},
		{"zh-cn", "zh-Hans", true},
		{"cht", "zh-Hant", true},
		{"big5", "zh-Hant", true},
		{"繁體", "zh-Hant", true},
		{"zh-tw", "zh-Hant", true},
		{"chi", "chi", true},
		{"中文", "chi", true},
		{"forced", "", false},
		{"1080p", "", false},
		{"xx", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		lang, ok := f.FindLanguage(tt.token)
		code := ""
		if ok {
			code = lang.ThreeLetter
			if lang.Name == "zh-Hans" || lang.Name == "zh-Hant" || lang.Name == "pt-BR" {
				code = lang.Name
			}
		}
		if ok != tt.ok || code != tt.want {
			t.Errorf("FindLanguage(%q) got = %q %v, want = %q %v", tt.token, code, ok, tt.want, tt.ok)
		}
	}
}

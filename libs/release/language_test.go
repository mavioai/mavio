package release

import "testing"

func TestLanguageByName(t *testing.T) {
	tests := []struct {
		name   string
		want   Language
		wantOK bool
	}{
		{"English", English, true},
		{"english", English, true},
		{"Portuguese (Brazil)", PortugueseBrazil, true},
		{"Spanish (Latino)", SpanishLatino, true},
		{"Unknown", LanguageUnknown, true},
		{"Original", Original, true},
		{"Georgian", Georgian, true},
		{"", LanguageUnknown, false},
		{"Klingon", LanguageUnknown, false},
	}
	for _, tt := range tests {
		got, ok := LanguageByName(tt.name)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("LanguageByName(%q) got = %v, %v, want = %v, %v", tt.name, got, ok, tt.want, tt.wantOK)
		}
	}
	for l := LanguageUnknown; l <= Original; l++ {
		if got, ok := LanguageByName(l.String()); !ok || got != l {
			t.Errorf("LanguageByName(%q) got = %v, %v, want = %v, true", l.String(), got, ok, l)
		}
	}
}

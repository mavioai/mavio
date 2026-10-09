package subtitle_test

import (
	"testing"

	"github.com/mavioai/mavio/libs/subtitle"
)

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"zh", "zh"},
		{"chi", "zh"},
		{"zho", "zh"},
		{"chinese", "zh"},
		{"中文", "zh"},
		{"双语", "zh"},
		{"chs", "zh-Hans"},
		{"sc", "zh-Hans"},
		{"gb", "zh-Hans"},
		{"zh-cn", "zh-Hans"},
		{"zh-CN", "zh-Hans"},
		{"zh-sg", "zh-Hans"},
		{"zh-hans", "zh-Hans"},
		{"简", "zh-Hans"},
		{"简体", "zh-Hans"},
		{"简中", "zh-Hans"},
		{"cht", "zh-Hant"},
		{"tc", "zh-Hant"},
		{"big5", "zh-Hant"},
		{"zh-tw", "zh-Hant"},
		{"zh-hk", "zh-Hant"},
		{"zh-mo", "zh-Hant"},
		{"zh-hant", "zh-Hant"},
		{"繁", "zh-Hant"},
		{"繁体", "zh-Hant"},
		{"繁體", "zh-Hant"},
		{"繁中", "zh-Hant"},
		{"en", "en"},
		{"eng", "en"},
		{"ja", "ja"},
		{"jpn", "ja"},
		{"ko", "ko"},
		{"kor", "ko"},
		{"fr", "fr"},
		{"fre", "fr"},
	}

	for _, tt := range tests {
		got := subtitle.NormalizeLanguage(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLanguage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMatchesLanguage(t *testing.T) {
	tests := []struct {
		pref   string
		actual string
		want   bool
	}{
		{"zh", "chs", true},
		{"zh", "cht", true},
		{"zh", "zh-CN", true},
		{"zh", "zh-TW", true},
		{"zh", "chi", true},
		{"zh-Hans", "chs", true},
		{"zh-Hans", "zh-CN", true},
		{"zh-Hans", "sc", true},
		{"zh-Hans", "cht", false},
		{"zh-Hant", "cht", true},
		{"zh-Hant", "zh-HK", true},
		{"zh-Hant", "chs", false},
		{"en", "eng", true},
		{"eng", "en", true},
		{"ja", "jpn", true},
		{"en", "ja", false},
	}

	for _, tt := range tests {
		got := subtitle.MatchesLanguage(tt.pref, tt.actual)
		if got != tt.want {
			t.Errorf("MatchesLanguage(%q, %q) = %v, want %v", tt.pref, tt.actual, got, tt.want)
		}
	}
}

func TestScoreSubtitle(t *testing.T) {
	stem := "Inception.2010.1080p"
	prefs := []string{"zh", "zh-Hans"}

	// 1. Exact stem match + ASS format bonus + preferred language
	// Base: +1000 (exact) + 200 (lang zh) + 20 (ass) = 1220
	score1 := subtitle.ScoreSubtitle(stem, "/media/Inception.2010.1080p.ass", "chs", prefs, false, false)
	if score1 != 1220 {
		t.Errorf("Exact stem ASS score = %d, want 1220", score1)
	}

	// 2. Prefix match + SRT format bonus + preferred language
	// Base: +500 (prefix) + 200 (lang zh) + 10 (srt) = 710
	score2 := subtitle.ScoreSubtitle(stem, "/media/Inception.2010.1080p.zh.srt", "", prefs, false, false)
	if score2 != 710 {
		t.Errorf("Prefix stem SRT score = %d, want 710", score2)
	}

	// 3. Prefix match + Forced tag penalty
	// Base: +500 (prefix) + 200 (lang) - 150 (forced) + 10 (srt) = 560
	score3 := subtitle.ScoreSubtitle(stem, "/media/Inception.2010.1080p.chs.forced.srt", "", prefs, true, false)
	if score3 != 560 {
		t.Errorf("Forced SRT score = %d, want 560", score3)
	}

	// 4. SDH penalty
	// Base: +500 (prefix) + 200 (lang) - 50 (sdh) + 10 (srt) = 660
	score4 := subtitle.ScoreSubtitle(stem, "/media/Inception.2010.1080p.chs.sdh.srt", "", prefs, false, true)
	if score4 != 660 {
		t.Errorf("SDH SRT score = %d, want 660", score4)
	}

	// 5. Non-preferred language penalty
	// Base: +500 (prefix) - 100 (non-pref) + 10 (srt) = 410
	score5 := subtitle.ScoreSubtitle(stem, "/media/Inception.2010.1080p.fr.srt", "fre", prefs, false, false)
	if score5 != 410 {
		t.Errorf("Non-preferred lang score = %d, want 410", score5)
	}
}

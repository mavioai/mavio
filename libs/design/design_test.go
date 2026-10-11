package design_test

import (
	"testing"

	design "github.com/mavioai/mavio/libs/design/gen/go"
)

func TestFormat(t *testing.T) {
	for _, tc := range []struct {
		locale, key string
		args        map[string]any
		want        string
	}{
		{"en", "nav.home", nil, "Home"},
		{"en", "home.latest", map[string]any{"library": "Movies"}, "Latest in Movies"},
		{"en", "item.episodeCount", map[string]any{"count": 1}, "1 episode"},
		{"en", "item.episodeCount", map[string]any{"count": 12}, "12 episodes"},
		{"en", "player.seekBack", map[string]any{"count": 10}, "Back 10 seconds"},
		{"zh-Hans", "item.episodeCount", map[string]any{"count": 1}, "1 集"},
		{"zh-Hans", "item.episodeNumber", map[string]any{"season": 2, "episode": 5}, "第 2 季第 5 集"},
		{"en", "no.such", nil, "no.such"},
	} {
		if got := design.Format(tc.locale, tc.key, tc.args); got != tc.want {
			t.Errorf("Format(%q, %q) = %q, want = %q", tc.locale, tc.key, got, tc.want)
		}
	}
}

func TestMatch(t *testing.T) {
	for _, tc := range []struct {
		preferred []string
		want      string
	}{
		{[]string{"zh-CN"}, "zh-Hans"},
		{[]string{"en-GB"}, "en"},
		{[]string{"fr-FR"}, "en"},
		{[]string{"fr-FR", "zh-Hans-SG"}, "zh-Hans"},
	} {
		if got := design.Match(tc.preferred...); got != tc.want {
			t.Errorf("Match(%q) = %q, want = %q", tc.preferred, got, tc.want)
		}
	}
}

func TestIcons(t *testing.T) {
	for name, icon := range design.Icons {
		if len(icon.Regular) == 0 || len(icon.Fill) == 0 {
			t.Errorf("icon %s has no paths in a weight", name)
		}
	}
}

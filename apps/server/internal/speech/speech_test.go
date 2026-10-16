package speech

import (
	"errors"
	"testing"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestSpeaks(t *testing.T) {
	for _, tc := range []struct {
		languages []string
		language  string
		want      bool
	}{
		{nil, "ja", true},
		{[]string{"en", "zh-Hans"}, "", true},
		{[]string{"en", "zh-Hans"}, "zh-CN", true},
		{[]string{"en", "zh-Hans"}, "en_US", true},
		{[]string{"en", "zh-Hans"}, "ja", false},
	} {
		if got := Speaks(tc.languages, tc.language); got != tc.want {
			t.Errorf("Speaks(%v, %q) = %v, want = %v", tc.languages, tc.language, got, tc.want)
		}
	}
}

func TestPick(t *testing.T) {
	recognizer := func(id string, timestamps bool, langs ...string) Recognizer {
		return Recognizer{ID: id, Info: pluginv1.RecognizerInfo_builder{Languages: langs, Timestamps: &timestamps}.Build()}
	}
	list := []Recognizer{
		recognizer("a/en", false, "en"),
		recognizer("b/any", true),
	}
	timestamps := func(i *pluginv1.RecognizerInfo) bool { return i.GetTimestamps() }
	for _, tc := range []struct {
		id, language string
		need         func(*pluginv1.RecognizerInfo) bool
		want         string
		err          bool
	}{
		{"", "en", nil, "a/en", false},
		{"", "ja", nil, "b/any", false},
		{"", "en", timestamps, "b/any", false},
		{"a/en", "en", nil, "a/en", false},
		{"a/en", "ja", nil, "", true},
		{"c/none", "", nil, "", true},
	} {
		got, err := Pick(list, tc.id, tc.language, tc.need)
		if got.ID != tc.want || (err != nil) != tc.err || (err != nil && !errors.Is(err, ErrNone)) {
			t.Errorf("Pick(%q, %q) = %q, %v, want = %q, error %v", tc.id, tc.language, got.ID, err, tc.want, tc.err)
		}
	}
}

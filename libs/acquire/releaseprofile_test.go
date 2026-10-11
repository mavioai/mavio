package acquire

import (
	"strings"
	"testing"
)

// TestCheckTerms ports Sonarr's ReleaseRestrictionsSpecificationFixture.
func TestCheckTerms(t *testing.T) {
	const title = "Dexter.S08E01.EDITED.WEBRip.x264-KYR"
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	tests := []struct {
		name              string
		title             string
		required, ignored string
		want              bool
	}{
		{"no restrictions", title, "", "", true},
		{"one required term", title, "WEBRip", "", true},
		{"no required term", title, "doesnt,exist", "", false},
		{"no ignored term", title, "", "ignored", true},
		{"one ignored term", title, "", "edited", false},
		{"required case EdiTED", title, "EdiTED", "", true},
		{"required case webrip", title, "webrip", "", true},
		{"required case X264", title, "X264", "", true},
		{"required case X264,NOTTHERE", title, "X264,NOTTHERE", "", true},
		{"ignored case EdiTED", title, "", "EdiTED", false},
		{"ignored case webrip", title, "", "webrip", false},
		{"ignored case X264", title, "", "X264", false},
		{"ignored case X264,NOTTHERE", title, "", "X264,NOTTHERE", false},
		{
			"one ignored and one required", "[ www.Speed.cd ] -Whose.Line.is.it.Anyway.US.S10E24.720p.HDTV.x264-BAJSKORV",
			"x264", "www.Speed.cd", false,
		},
		{"perl /WEB/", title, "/WEB/", "", true},
		{"perl /WEB<backspace>/", title, "/WEB\b/", "", false},
		{"perl /WEb/", title, "/WEb/", "", false},
		{`perl /\.WEB/`, title, `/\.WEB/`, "", true},
		{"perl /web/i", title, "/web/i", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profiles []ReleaseProfile
			if tt.required != "" || tt.ignored != "" {
				profiles = []ReleaseProfile{{Enabled: true, Required: split(tt.required), Ignored: split(tt.ignored)}}
			}
			var m TermMatcher
			got := m.CheckTerms(profiles, tt.title)
			if (got == "") != tt.want {
				t.Errorf("CheckTerms() = %q, want = accepted %v", got, tt.want)
			}
		})
	}
}

func TestReleaseProfile(t *testing.T) {
	p := ReleaseProfile{Enabled: true, IndexerID: 2, Tags: []int{1}, Required: []string{"/x/q"}}
	if err := p.Validate(); err == nil {
		t.Error("Validate() with modifier q = nil, want = an error")
	}
	p.Required = []string{"x264"}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() = %v, want = nil", err)
	}
	tests := []struct {
		tags    []int
		indexer int
		want    bool
	}{
		{[]int{1}, 2, true},
		{[]int{1}, 3, false},
		{[]int{4}, 2, false},
	}
	for _, tt := range tests {
		if got := p.AppliesTo(tt.tags, tt.indexer); got != tt.want {
			t.Errorf("AppliesTo(%v, %d) = %v, want = %v", tt.tags, tt.indexer, got, tt.want)
		}
	}
	p.Enabled = false
	if p.AppliesTo([]int{1}, 2) {
		t.Error("AppliesTo() of a disabled profile = true, want = false")
	}
}

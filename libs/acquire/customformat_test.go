package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// TestLanguageSpecification ports Radarr's SingleLanguageFixture,
// MultiLanguageFixture and OriginalLanguageFixture, whose movie's original
// language is English.
func TestLanguageSpecification(t *testing.T) {
	single := []release.Language{release.French}
	multi := []release.Language{release.English, release.French}
	english := []release.Language{release.English}
	tests := []struct {
		name      string
		languages []release.Language
		value     release.Language
		except    bool
		negate    bool
		want      bool
	}{
		{"single: same language", single, release.French, false, false, true},
		{"single: different language", single, release.Spanish, false, false, false},
		{"single: negated same language", single, release.French, false, true, false},
		{"single: negated different language", single, release.Spanish, false, true, true},
		{"single: negated except the only language", single, release.French, true, true, true},
		{"multi: one language", multi, release.French, false, false, true},
		{"multi: except with other languages", multi, release.French, true, false, true},
		{"multi: except original with others", multi, release.Original, true, false, true},
		{"multi: different language", multi, release.Spanish, false, false, false},
		{"multi: negated when one matches", multi, release.French, false, true, false},
		{"multi: negated when none matches", multi, release.Spanish, false, true, true},
		{"multi: negated except with others", multi, release.Spanish, true, true, false},
		{"original: same language", english, release.Original, false, false, true},
		{"original: different language", single, release.Original, false, false, false},
		{"original: negated same language", english, release.Original, false, true, false},
		{"original: negated different language", single, release.Original, false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Specification{Kind: SpecLanguage, Value: int(tt.value), ExceptLanguage: tt.except, Negate: tt.negate}
			in := &Input{Languages: tt.languages, OriginalLanguage: release.English, Size: 100 << 20, Filename: "Movie.Title.2024"}
			if got := s.Matches(in); got != tt.want {
				t.Errorf("Matches() = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestSpecifications(t *testing.T) {
	in := &Input{
		Title:        "Movie Title 2024 1080p BluRay REMUX-GRP",
		Filename:     "movie.mkv",
		Quality:      release.ParsedQuality{Quality: release.Remux1080p},
		ReleaseGroup: "GRP",
		Edition:      "Director's Cut",
		Year:         2024,
		Size:         5 << 30,
		Flags:        []string{FlagFreeleech, FlagInternal},
		ReleaseType:  release.SeasonPack,
	}
	tests := []struct {
		name string
		spec Specification
		want bool
	}{
		{"title", Specification{Kind: SpecReleaseTitle, Pattern: `\bremux\b`}, true},
		{"title from file name", Specification{Kind: SpecReleaseTitle, Pattern: `\.mkv$`}, true},
		{"title negated", Specification{Kind: SpecReleaseTitle, Pattern: `x265`, Negate: true}, true},
		{".NET lookbehind", Specification{Kind: SpecReleaseTitle, Pattern: `(?<=BluRay )REMUX`}, true},
		{"edition", Specification{Kind: SpecEdition, Pattern: `director`}, true},
		{"group", Specification{Kind: SpecReleaseGroup, Pattern: `^grp$`}, true},
		{"group no match", Specification{Kind: SpecReleaseGroup, Pattern: `^other$`}, false},
		{"flag", Specification{Kind: SpecIndexerFlag, Flag: FlagFreeleech}, true},
		{"flag missing", Specification{Kind: SpecIndexerFlag, Flag: FlagScene}, false},
		{"source", Specification{Kind: SpecSource, Value: int(release.SourceBluray)}, true},
		{"resolution", Specification{Kind: SpecResolution, Value: 1080}, true},
		{"resolution other", Specification{Kind: SpecResolution, Value: 2160}, false},
		{"modifier", Specification{Kind: SpecQualityModifier, Value: int(release.ModifierRemux)}, true},
		{"size in range", Specification{Kind: SpecSize, Min: 1, Max: 5}, true},
		{"size at min", Specification{Kind: SpecSize, Min: 5, Max: 10}, false},
		{"year", Specification{Kind: SpecYear, Min: 2020, Max: 2024}, true},
		{"year out", Specification{Kind: SpecYear, Min: 2000, Max: 2010}, false},
		{"release type", Specification{Kind: SpecReleaseType, Value: int(release.SeasonPack)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := CustomFormat{Name: tt.name, Specifications: []Specification{tt.spec}}
			if err := f.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}
			if got := f.Specifications[0].Matches(in); got != tt.want {
				t.Errorf("Matches() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// TestMatchFormats checks Radarr's CustomFormatCalculationService: within
// a kind one condition must hold and no required one fail; every kind must
// pass; matches come by name.
func TestMatchFormats(t *testing.T) {
	title := func(p string, required, negate bool) Specification {
		return Specification{Kind: SpecReleaseTitle, Pattern: p, Required: required, Negate: negate}
	}
	res := func(v int) Specification { return Specification{Kind: SpecResolution, Value: v} }
	formats := []CustomFormat{
		{ID: tid(1), Name: "x265", Specifications: []Specification{title(`x265|hevc`, false, false)}},
		{ID: tid(2), Name: "any of", Specifications: []Specification{title(`nope`, false, false), title(`grp`, false, false)}},
		{ID: tid(3), Name: "required fails", Specifications: []Specification{title(`grp`, false, false), title(`nope`, true, false)}},
		{ID: tid(4), Name: "Both kinds", Specifications: []Specification{title(`grp`, false, false), res(2160)}},
		{ID: tid(5), Name: "both kinds hold", Specifications: []Specification{title(`grp`, false, false), res(1080), res(720)}},
		{ID: tid(6), Name: "empty"},
		{ID: tid(7), Name: "Required negated", Specifications: []Specification{title(`x264`, true, true)}},
	}
	for i := range formats {
		if err := formats[i].Validate(); err != nil {
			t.Fatalf("Validate(%q) = %v", formats[i].Name, err)
		}
	}
	in := &Input{Title: "Movie 2024 1080p HEVC-GRP", Quality: release.ParsedQuality{Quality: release.WEBDL1080p}}
	var got []string
	for _, f := range MatchFormats(formats, in) {
		got = append(got, f.Name)
	}
	want := []string{"any of", "both kinds hold", "empty", "Required negated", "x265"}
	if len(got) != len(want) {
		t.Fatalf("MatchFormats() = %q, want = %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MatchFormats() = %q, want = %q", got, want)
		}
	}
}

func TestValidateCustomFormat(t *testing.T) {
	bad := []Specification{
		{Kind: SpecReleaseTitle},
		{Kind: SpecReleaseTitle, Pattern: `(`},
		{Kind: SpecLanguage, Value: -1},
		{Kind: SpecIndexerFlag},
		{Kind: SpecResolution, Value: 1000},
		{Kind: SpecSize, Min: 5, Max: 5},
		{Kind: SpecYear, Min: 2000, Max: 1999},
		{Kind: 99},
	}
	for _, s := range bad {
		f := CustomFormat{Name: "f", Specifications: []Specification{s}}
		if err := f.Validate(); err == nil {
			t.Errorf("Validate(%+v) = nil, want = an error", s)
		}
	}
	if err := (&CustomFormat{}).Validate(); err == nil {
		t.Error("Validate() of a nameless format = nil, want = an error")
	}
}

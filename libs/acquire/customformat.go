package acquire

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/libs/release"
)

// SpecKind is what a custom format specification looks at; the values
// are Radarr's orders of the specifications, ReleaseType is Sonarr's.
type SpecKind int

// Specification kinds.
const (
	SpecReleaseTitle    SpecKind = 1
	SpecEdition         SpecKind = 2
	SpecLanguage        SpecKind = 3
	SpecIndexerFlag     SpecKind = 4
	SpecSource          SpecKind = 5
	SpecResolution      SpecKind = 6
	SpecQualityModifier SpecKind = 7
	SpecSize            SpecKind = 8
	SpecReleaseGroup    SpecKind = 9
	SpecYear            SpecKind = 10
	SpecReleaseType     SpecKind = 11
)

// Indexer flags of releases, as indexers report them.
const (
	FlagFreeleech    = "freeleech"
	FlagHalfleech    = "halfleech"
	FlagDoubleUpload = "doubleupload"
	FlagInternal     = "internal"
	FlagScene        = "scene"
	FlagFreeleech75  = "freeleech75"
	FlagFreeleech25  = "freeleech25"
	FlagNuked        = "nuked"
	FlagPTPGolden    = "ptp_golden"
	FlagPTPApproved  = "ptp_approved"
)

// Specification is one condition of a custom format.
type Specification struct {
	Kind SpecKind
	Name string
	// Negate inverts the condition.
	Negate bool
	// Required makes the format fail when the condition does, whatever
	// the other conditions of its kind.
	Required bool

	// Pattern is the .NET regular expression, matched case-insensitively,
	// of ReleaseTitle, Edition and ReleaseGroup.
	Pattern string
	// Value is the release.Language of Language, the release.Source of
	// Source, the resolution of Resolution, the release.Modifier of
	// QualityModifier and the release.ReleaseType of ReleaseType.
	Value int
	// Flag is the indexer flag of IndexerFlag.
	Flag string
	// ExceptLanguage makes Language match releases in any other language.
	ExceptLanguage bool
	// Min and Max bound Size in gigabytes, min exclusive, and Year.
	Min, Max float64

	re *regexp2.Regexp
}

// CustomFormat is a named set of conditions on releases and files, which
// quality profiles score, as Radarr's and Sonarr's CustomFormat.
type CustomFormat struct {
	ID   int
	Name string
	// IncludeWhenRenaming puts the format's name in file names.
	IncludeWhenRenaming bool
	Specifications      []Specification
}

// regexTimeout bounds each match of a user's expression.
const regexTimeout = time.Second

// Validate checks the format and compiles its expressions.
func (f *CustomFormat) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return errors.New("the custom format has no name")
	}
	var errs []error
	for i := range f.Specifications {
		if err := f.Specifications[i].compile(); err != nil {
			errs = append(errs, fmt.Errorf("condition %q: %w", f.Specifications[i].Name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *Specification) compile() error {
	switch s.Kind {
	case SpecReleaseTitle, SpecEdition, SpecReleaseGroup:
		if strings.TrimSpace(s.Pattern) == "" {
			return errors.New("the regular expression is empty")
		}
		re, err := regexp2.Compile(s.Pattern, regexp2.IgnoreCase)
		if err != nil {
			return err
		}
		re.MatchTimeout = regexTimeout
		s.re = re
	case SpecLanguage:
		if s.Value < 0 || s.Value > int(release.Original) {
			return fmt.Errorf("unknown language %d", s.Value)
		}
	case SpecIndexerFlag:
		if s.Flag == "" {
			return errors.New("no flag")
		}
	case SpecSource:
		if s.Value < int(release.SourceUnknown) || s.Value > int(release.SourceBluray) {
			return fmt.Errorf("unknown source %d", s.Value)
		}
	case SpecResolution:
		if !slices.Contains([]int{0, 360, 480, 540, 576, 720, 1080, 2160}, s.Value) {
			return fmt.Errorf("unknown resolution %d", s.Value)
		}
	case SpecQualityModifier:
		if s.Value < int(release.ModifierNone) || s.Value > int(release.ModifierRemux) {
			return fmt.Errorf("unknown quality modifier %d", s.Value)
		}
	case SpecSize:
		if s.Min < 0 || s.Max <= s.Min {
			return fmt.Errorf("invalid size range %v–%v", s.Min, s.Max)
		}
	case SpecYear:
		if s.Min <= 0 || s.Max < s.Min {
			return fmt.Errorf("invalid year range %v–%v", s.Min, s.Max)
		}
	case SpecReleaseType:
		if s.Value < int(release.ReleaseUnknown) || s.Value > int(release.SeasonPack) {
			return fmt.Errorf("unknown release type %d", s.Value)
		}
	default:
		return fmt.Errorf("unknown kind %d", s.Kind)
	}
	return nil
}

// Input is what custom formats are matched against: a release or a file.
type Input struct {
	// Title is the release title: for movies, as release.SimplifyReleaseTitle
	// leaves it.
	Title    string
	Filename string
	Quality  release.ParsedQuality
	// Languages are the release's or file's languages.
	Languages []release.Language
	// OriginalLanguage is the movie's or series' original language.
	OriginalLanguage release.Language
	ReleaseGroup     string
	Edition          string
	Year             int
	// Size is in bytes.
	Size        int64
	Flags       []string
	ReleaseType release.ReleaseType
}

// Matches reports whether the specification holds for the input.
func (s *Specification) Matches(in *Input) bool {
	if s.Kind == SpecLanguage {
		return s.matchLanguage(in)
	}
	return s.match(in) != s.Negate
}

func (s *Specification) match(in *Input) bool {
	switch s.Kind {
	case SpecReleaseTitle:
		return s.matchString(in.Title) || s.matchString(in.Filename)
	case SpecEdition:
		return s.matchString(in.Edition)
	case SpecReleaseGroup:
		return s.matchString(in.ReleaseGroup)
	case SpecIndexerFlag:
		return slices.Contains(in.Flags, s.Flag)
	case SpecSource:
		return int(in.Quality.Quality.Source()) == s.Value
	case SpecResolution:
		return in.Quality.Quality.Resolution() == s.Value
	case SpecQualityModifier:
		return int(in.Quality.Quality.Modifier()) == s.Value
	case SpecSize:
		const gb = 1 << 30
		size := float64(in.Size)
		return size > s.Min*gb && size <= s.Max*gb
	case SpecYear:
		return in.Year > 0 && float64(in.Year) >= s.Min && float64(in.Year) <= s.Max
	case SpecReleaseType:
		return int(in.ReleaseType) == s.Value
	}
	return false
}

// matchLanguage is Radarr's LanguageSpecification: Original stands for
// the input's original language when it is known.
func (s *Specification) matchLanguage(in *Input) bool {
	compared := release.Language(s.Value)
	if compared == release.Original && in.OriginalLanguage != release.LanguageUnknown {
		compared = in.OriginalLanguage
	}
	var match bool
	if s.ExceptLanguage {
		match = slices.ContainsFunc(in.Languages, func(l release.Language) bool { return l != compared })
	} else {
		match = slices.Contains(in.Languages, compared)
	}
	return match != s.Negate
}

func (s *Specification) matchString(v string) bool {
	if v == "" {
		return false
	}
	if s.re == nil && s.compile() != nil {
		return false
	}
	ok, err := s.re.MatchString(v)
	return err == nil && ok
}

// Matches reports whether the format holds for the input: for each kind
// of its conditions, none required fails and one holds.
func (f *CustomFormat) Matches(in *Input) bool {
	type group struct{ any, requiredFailed bool }
	groups := map[SpecKind]*group{}
	for i := range f.Specifications {
		s := &f.Specifications[i]
		g := groups[s.Kind]
		if g == nil {
			g = &group{}
			groups[s.Kind] = g
		}
		ok := s.Matches(in)
		g.any = g.any || ok
		g.requiredFailed = g.requiredFailed || (s.Required && !ok)
	}
	for _, g := range groups {
		if g.requiredFailed || !g.any {
			return false
		}
	}
	return true
}

// MatchFormats returns the formats that hold for the input, by name.
func MatchFormats(formats []CustomFormat, in *Input) []CustomFormat {
	var out []CustomFormat
	for i := range formats {
		if formats[i].Matches(in) {
			out = append(out, formats[i])
		}
	}
	slices.SortStableFunc(out, func(a, b CustomFormat) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.Name, b.Name))
	})
	return out
}

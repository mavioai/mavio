package subtitles

import (
	"slices"

	"github.com/mavioai/mavio/libs/core"
)

// LanguageEquals treats pairs of languages as equal, as Bazarr's
// _LanguageEquals: subtitles in the second language of a pair serve
// requests for the first. Languages are equal only with the same forced
// and hearing-impaired flags.
type LanguageEquals []core.SubtitleLanguageEquivalence

// CheckSet returns langs with the To of every pair whose From is in langs:
// the languages that serve langs.
func (e LanguageEquals) CheckSet(langs []core.SubtitleLanguage) []core.SubtitleLanguage {
	out := slices.Clone(langs)
	for _, eq := range e {
		if slices.Contains(langs, eq.From) && !slices.Contains(out, eq.To) {
			out = append(out, eq.To)
		}
	}
	return out
}

// Translate returns langs with the From of every pair whose To is in langs:
// the languages to request from a provider so that it lists subtitles that
// serve langs.
func (e LanguageEquals) Translate(langs []core.SubtitleLanguage) []core.SubtitleLanguage {
	out := slices.Clone(langs)
	for _, eq := range e {
		if slices.Contains(langs, eq.To) && !slices.Contains(out, eq.From) {
			out = append(out, eq.From)
		}
	}
	return out
}

// Apply returns the language a subtitle in l counts as: the To of the
// first pair whose From is l, or l.
func (e LanguageEquals) Apply(l core.SubtitleLanguage) core.SubtitleLanguage {
	for _, eq := range e {
		if eq.From == l {
			return eq.To
		}
	}
	return l
}

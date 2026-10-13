package subtitles

import (
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// wantedHI is the hearing-impaired value of a wanted or existing subtitle,
// as Bazarr compares them.
type wantedHI int

const (
	hiFalse wantedHI = iota
	hiTrue
	hiExcluded
)

// wantedLang is a language Bazarr compares as a dictionary of language,
// forced and hi.
type wantedLang struct {
	language string
	forced   bool
	hi       wantedHI
}

func wantedOf(l core.SubtitleLanguage) wantedLang {
	w := wantedLang{language: l.Language, forced: l.Forced}
	if l.HearingImpaired {
		w.hi = hiTrue
	}
	return w
}

func itemWanted(it core.SubtitleProfileItem) wantedLang {
	w := wantedLang{language: it.Language, forced: it.Forced}
	switch it.HI {
	case core.SubtitleHIRequired:
		w.hi = hiTrue
	case core.SubtitleHIExcluded:
		w.hi = hiExcluded
	}
	return w
}

// Missing returns the subtitles a profile still wants for a video with the
// existing subtitles and audio languages, as Bazarr's
// list_missing_subtitles: nothing once the cutoff is met, otherwise the
// profile's languages (less those excluded or not included by audio) that
// no existing subtitle, or language equal to one, provides. A
// hearing-impaired subtitle provides its language unless the profile
// excludes hearing-impaired subtitles for it.
func Missing(p *core.SubtitleProfile, existing []core.SubtitleLanguage, audio []string, equals LanguageEquals) []core.SubtitleLanguage {
	matchesAudio := func(lang string) bool {
		return slices.ContainsFunc(audio, func(a string) bool { return strings.EqualFold(a, lang) })
	}
	var desired []wantedLang
	for _, it := range p.Items {
		if it.AudioExclude && matchesAudio(it.Language) || it.AudioOnlyInclude && !matchesAudio(it.Language) {
			continue
		}
		desired = append(desired, itemWanted(it))
	}
	var actual, expanded []wantedLang
	for _, l := range existing {
		actual = append(actual, wantedOf(l))
	}
	for _, l := range equals.CheckSet(existing) {
		expanded = append(expanded, wantedOf(l))
	}

	var cutoff []core.SubtitleProfileItem
	switch p.Cutoff {
	case 0:
	case core.SubtitleCutoffAny:
		cutoff = p.Items
	default:
		for _, it := range p.Items {
			if it.ID == p.Cutoff {
				cutoff = []core.SubtitleProfileItem{it}
			}
		}
	}
	for _, it := range cutoff {
		w := itemWanted(it)
		switch {
		case it.AudioOnlyInclude && !matchesAudio(it.Language):
			// Not wanted unless the audio has the language, so it does not
			// meet the cutoff.
		case it.AudioExclude && matchesAudio(it.Language),
			slices.Contains(actual, w), slices.Contains(expanded, w),
			w.hi == hiFalse && slices.Contains(actual, wantedLang{language: w.language, hi: hiTrue}),
			w.hi == hiExcluded && slices.Contains(actual, wantedLang{language: w.language, forced: w.forced}):
			return nil
		}
	}

	var missing []wantedLang
	for _, w := range desired {
		if !slices.Contains(actual, w) && !slices.Contains(expanded, w) {
			missing = append(missing, w)
		}
	}
	hiSetting := map[string]core.SubtitleHI{}
	for _, it := range p.Items {
		if !it.Forced {
			hiSetting[it.Language] = it.HI
		}
	}
	remove := func(w wantedLang) {
		if i := slices.Index(missing, w); i >= 0 {
			missing = slices.Delete(missing, i, i+1)
		}
	}
	for _, a := range actual {
		switch {
		case a.hi == hiTrue && hiSetting[a.language] != core.SubtitleHIExcluded:
			remove(wantedLang{language: a.language})
		case a.hi == hiFalse && hiSetting[a.language] == core.SubtitleHIExcluded:
			remove(wantedLang{language: a.language, hi: hiExcluded})
		}
	}

	var out []core.SubtitleLanguage
	for _, w := range missing {
		out = append(out, core.SubtitleLanguage{Language: w.language, Forced: w.forced, HearingImpaired: !w.forced && w.hi == hiTrue})
	}
	return out
}

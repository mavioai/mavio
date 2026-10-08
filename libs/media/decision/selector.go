package decision

import (
	"cmp"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// SubtitleMode is when subtitles play by default.
type SubtitleMode string

// Subtitle modes.
const (
	// SubtitlesDefault plays external subtitles and streams flagged default
	// or forced.
	SubtitlesDefault SubtitleMode = "default"
	// SubtitlesAlways plays full subtitles in a preferred language, else
	// forced ones.
	SubtitlesAlways SubtitleMode = "always"
	// SubtitlesOnlyForced plays forced subtitles in a preferred or
	// undefined language.
	SubtitlesOnlyForced SubtitleMode = "only_forced"
	// SubtitlesNone plays no subtitles.
	SubtitlesNone SubtitleMode = "none"
	// SubtitlesSmart plays subtitles in a preferred language unless the
	// audio already is in one; then it behaves like SubtitlesOnlyForced.
	SubtitlesSmart SubtitleMode = "smart"
)

// DefaultAudioIndex returns the index of the audio stream to play: the best
// scored one, or with preferDefault the best scored stream flagged default.
// It returns nil when there is no audio.
func DefaultAudioIndex(streams []core.MediaStream, languages []string, preferDefault bool) *int {
	sorted := sortedByScore(streams, core.StreamAudio, languages)
	if preferDefault {
		for _, st := range sorted {
			if st.Default {
				return ptr(st.Index)
			}
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	return ptr(sorted[0].Index)
}

// DefaultSubtitleIndex returns the index of the subtitle stream to play in
// mode, or nil for none; audioLanguage is the language of the audio that
// plays.
func DefaultSubtitleIndex(streams []core.MediaStream, languages []string, mode SubtitleMode, audioLanguage string) *int {
	if mode == SubtitlesNone || mode == "" {
		return nil
	}
	var subs []*core.MediaStream
	for i := range streams {
		if streams[i].Kind == core.StreamSubtitle {
			subs = append(subs, &streams[i])
		}
	}
	// External first, then default, then full tracks in a preferred
	// language, then forced ones in a preferred language, in an undefined
	// one and in any.
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(subs, func(x, y *core.MediaStream) int {
		return cmp.Or(
			cmp.Compare(b(isExternal(y)), b(isExternal(x))),
			cmp.Compare(b(y.Default), b(x.Default)),
			cmp.Compare(b(!y.Forced && matchesLanguage(y.Language, languages)), b(!x.Forced && matchesLanguage(x.Language, languages))),
			cmp.Compare(b(y.Forced && matchesLanguage(y.Language, languages)), b(x.Forced && matchesLanguage(x.Language, languages))),
			cmp.Compare(b(y.Forced && undefinedLanguage(y.Language)), b(x.Forced && undefinedLanguage(x.Language))),
			cmp.Compare(b(y.Forced), b(x.Forced)),
		)
	})
	first := func(keep func(*core.MediaStream) bool) *core.MediaStream {
		for _, st := range subs {
			if keep(st) {
				return st
			}
		}
		return nil
	}
	firstForced := func() *core.MediaStream {
		if f := onlyForced(subs, languages); len(f) > 0 {
			return f[0]
		}
		return nil
	}
	var st *core.MediaStream
	switch mode {
	case SubtitlesDefault:
		st = first(func(s *core.MediaStream) bool { return isExternal(s) || s.Default || s.Forced })
	case SubtitlesSmart:
		if !containsFold(languages, audioLanguage) {
			st = first(func(s *core.MediaStream) bool { return matchesLanguage(s.Language, languages) })
		} else {
			st = firstForced()
		}
	case SubtitlesAlways:
		st = first(func(s *core.MediaStream) bool { return !s.Forced && matchesLanguage(s.Language, languages) })
		if st == nil {
			st = firstForced()
		}
	case SubtitlesOnlyForced:
		st = firstForced()
	}
	if st == nil {
		return nil
	}
	return ptr(st.Index)
}

// SubtitleScores scores the subtitle streams that may play by default in
// mode, keyed by stream index, for [Source.SubtitleScores].
func SubtitleScores(streams []core.MediaStream, languages []string, mode SubtitleMode, audioLanguage string) map[int]int {
	if mode == SubtitlesNone || mode == "" {
		return nil
	}
	sorted := sortedByScore(streams, core.StreamSubtitle, languages)
	var kept []*core.MediaStream
	switch mode {
	case SubtitlesDefault:
		kept = filter(sorted, func(s *core.MediaStream) bool { return isExternal(s) || s.Default || s.Forced })
	case SubtitlesSmart:
		if !containsFold(languages, audioLanguage) {
			kept = filter(sorted, func(s *core.MediaStream) bool { return matchesLanguage(s.Language, languages) })
		} else {
			kept = onlyForced(sorted, languages)
		}
	case SubtitlesAlways:
		kept = filter(sorted, func(s *core.MediaStream) bool { return !s.Forced && matchesLanguage(s.Language, languages) })
		if len(kept) == 0 {
			kept = onlyForced(sorted, languages)
		}
	case SubtitlesOnlyForced:
		kept = onlyForced(sorted, languages)
	}
	scores := make(map[int]int, len(kept))
	for _, st := range kept {
		scores[st.Index] = StreamScore(st, languages)
	}
	return scores
}

// StreamScore ranks a stream by, in decreasing weight, its position in the
// preferred languages and whether it is forced, default, available as a
// separate file, text and external.
func StreamScore(st *core.MediaStream, languages []string) int {
	score := 1
	if i := slices.IndexFunc(languages, func(l string) bool { return strings.EqualFold(l, st.Language) }); i >= 0 {
		score = 101 - i
	}
	for _, v := range []bool{st.Forced, st.Default, supportsExternalStream(st), st.IsTextSubtitle(), isExternal(st)} {
		score *= 10
		if v {
			score += 2
		} else {
			score++
		}
	}
	return score
}

func sortedByScore(streams []core.MediaStream, kind core.StreamKind, languages []string) []*core.MediaStream {
	var out []*core.MediaStream
	for i := range streams {
		if streams[i].Kind == kind {
			out = append(out, &streams[i])
		}
	}
	slices.SortStableFunc(out, func(x, y *core.MediaStream) int {
		return cmp.Compare(StreamScore(y, languages), StreamScore(x, languages))
	})
	return out
}

func filter(streams []*core.MediaStream, keep func(*core.MediaStream) bool) []*core.MediaStream {
	var out []*core.MediaStream
	for _, st := range streams {
		if keep(st) {
			out = append(out, st)
		}
	}
	return out
}

// onlyForced returns the forced streams in a preferred or undefined
// language, preferred ones first.
func onlyForced(streams []*core.MediaStream, languages []string) []*core.MediaStream {
	out := filter(streams, func(s *core.MediaStream) bool {
		return s.Forced && (matchesLanguage(s.Language, languages) || undefinedLanguage(s.Language))
	})
	slices.SortStableFunc(out, func(x, y *core.MediaStream) int {
		b := func(v bool) int {
			if v {
				return 1
			}
			return 0
		}
		return cmp.Or(
			cmp.Compare(b(matchesLanguage(y.Language, languages)), b(matchesLanguage(x.Language, languages))),
			cmp.Compare(b(undefinedLanguage(y.Language)), b(undefinedLanguage(x.Language))),
		)
	})
	return out
}

// matchesLanguage reports whether lang is preferred; no preference matches
// any language.
func matchesLanguage(lang string, languages []string) bool {
	return len(languages) == 0 || containsFold(languages, lang)
}

func undefinedLanguage(lang string) bool {
	switch strings.ToLower(lang) {
	case "", "und", "unknown", "undetermined", "mul", "zxx":
		return true
	}
	return false
}

func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(v string) bool { return strings.EqualFold(v, s) })
}

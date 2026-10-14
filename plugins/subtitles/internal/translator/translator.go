// Package translator defines the subtitle translators the plugin
// implements, as Bazarr's translator services: they translate the text of
// a subtitle's cues to another language.
package translator

import (
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// Media describes what a subtitle being translated is of.
type Media struct {
	Episode bool
	// Title is the movie's title, or the series' name.
	Title       string
	Year        int
	ExternalIDs map[string]string
	Overview    string
	// Season, Number and EpisodeTitle are an episode's.
	Season, Number int
	EpisodeTitle   string
	// SourceFileName and FileName are the file names of the subtitle
	// translated and of the translation.
	SourceFileName, FileName string
}

// Request is cues to translate.
type Request struct {
	// Source and Target are the languages, as BCP 47 tags.
	Source, Target string
	// Lines are the cues' text, lines separated by "\n": plain, or as
	// SubRip holds it for translators keeping markup.
	Lines []string
	Media Media
}

// Translator translates cues.
type Translator interface {
	// Translate returns the translations of the request's lines, as many
	// and in their order; an empty one keeps its cue's text.
	Translate(ctx context.Context, req *Request) ([]string, error)
	// Credit is what translated, as subtitles may credit it.
	Credit() string
	// BatchSize is how many cues Translate takes at once; 0 for every
	// cue of a subtitle.
	BatchSize() int
}

// Definition is a translator the plugin implements.
type Definition struct {
	// Name is stable: "deepl".
	Name        string
	DisplayName string
	Settings    []provider.Setting
	// Markup is whether the translator keeps SubRip's markup.
	Markup bool
	// New makes the translator from its settings, a JSON object; its
	// failures are configuration errors.
	New func(env provider.Env, settings json.RawMessage) (Translator, error)
}

// Languages is implemented by translators that know the languages they
// translate.
type Languages interface {
	// Languages returns the BCP 47 tags of the languages the translator
	// translates from and to; none for any.
	Languages(ctx context.Context) (source, target []string, err error)
}

// Language is a language as a service names it.
type Language struct {
	// Code is the service's code.
	Code string
	// Tag is the language's BCP 47 tag; "" when Code is one.
	Tag string
}

// tag returns a language's BCP 47 tag.
func (l Language) tag() string {
	if l.Tag != "" {
		return l.Tag
	}
	return l.Code
}

// Tags returns the BCP 47 tags of languages, those that are not left out.
func Tags(langs []Language) []string {
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		if _, err := language.Parse(l.tag()); err == nil {
			out = append(out, l.tag())
		}
	}
	return out
}

// Match returns the code of the language among langs that a BCP 47 tag
// names: the one of the same tag, else the closest of the same language,
// such as "zh-Hans" for "zh" and "pt-BR" for "pt".
func Match(tag string, langs []Language) (string, bool) {
	var tags []language.Tag
	var codes []string
	for _, l := range langs {
		if strings.EqualFold(l.Code, tag) || strings.EqualFold(l.tag(), tag) {
			return l.Code, true
		}
		if t, err := language.Parse(l.tag()); err == nil {
			tags, codes = append(tags, t), append(codes, l.Code)
		}
	}
	t, err := language.Parse(tag)
	if err != nil || len(tags) == 0 {
		return "", false
	}
	_, i, conf := language.NewMatcher(tags).Match(t)
	// The matcher falls back on its first language with high confidence.
	want, _ := t.Base()
	got, _ := tags[i].Base()
	if conf < language.High || conf < language.Exact && want != got {
		return "", false
	}
	return codes[i], true
}

// Codes returns languages of the codes a service names them by.
func Codes(codes ...string) []Language {
	out := make([]Language, len(codes))
	for i, c := range codes {
		out[i] = Language{Code: c}
	}
	return out
}

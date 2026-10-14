// Package translator defines the subtitle translators the plugin
// implements, as Bazarr's translator services: they translate the text of
// a subtitle's cues to another language.
package translator

import (
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

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
	// Name is stable: "google_translate".
	Name        string
	DisplayName string
	Settings    []provider.Setting
	// Markup is whether the translator keeps SubRip's markup.
	Markup bool
	// New makes the translator from its settings, a JSON object; its
	// failures are configuration errors.
	New func(env provider.Env, settings json.RawMessage) (Translator, error)
}

// Name returns a language's English name, such as "French" or
// "Brazilian Portuguese", which services are told languages by.
func Name(tag string) string {
	t, err := language.Parse(tag)
	if err != nil {
		return tag
	}
	if n := display.English.Tags().Name(t); n != "" {
		return n
	}
	return tag
}

// Code returns a language's code as services such as Google Translate take
// it: the language's ISO 639-1 code, or its tag; Chinese and Brazilian
// Portuguese with their regions, as Bazarr's language_code_convert_dict.
func Code(tag string, convert map[string]string) string {
	t, err := language.Parse(tag)
	if err != nil {
		return tag
	}
	base, _ := t.Base()
	switch base.String() {
	case "zh":
		if s, _ := t.Script(); s.String() == "Hant" {
			return convert["zt"]
		}
		if r, _ := t.Region(); r.String() == "TW" || r.String() == "HK" || r.String() == "MO" {
			return convert["zt"]
		}
		return convert["zh"]
	case "pt":
		if r, conf := t.Region(); r.String() == "BR" && conf == language.Exact {
			if c, ok := convert["pb"]; ok {
				return c
			}
		}
	}
	if c, ok := convert[base.String()]; ok {
		return c
	}
	if a := provider.Alpha2(tag); a != "" {
		return a
	}
	return strings.ToLower(base.String())
}

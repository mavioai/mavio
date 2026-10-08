package naming

import (
	"strings"
)

// ExternalKind is the kind of an external file accompanying a video.
type ExternalKind int

// External file kinds.
const (
	ExternalSubtitle ExternalKind = iota
	ExternalAudio
	ExternalLyric
)

// Language is a language recognized in a file name.
type Language struct {
	// Name is the language tag, such as "en" or "pt-BR".
	Name string
	// ThreeLetter is the ISO 639-2 code, such as "eng".
	ThreeLetter string
}

// code returns the language as stored: the regional tag when there is a
// region, the three-letter code otherwise.
func (l Language) code() string {
	if strings.Contains(l.Name, "-") {
		return l.Name
	}
	return l.ThreeLetter
}

// LanguageFinder recognizes a language from a file name token such as
// "en", "eng" or "English".
type LanguageFinder interface {
	FindLanguage(token string) (Language, bool)
}

// ExternalFile is what the name of an external subtitle, audio or lyric
// file says about it.
type ExternalFile struct {
	Path              string
	Language          string
	Title             string
	IsDefault         bool
	IsForced          bool
	IsHearingImpaired bool
}

// ParseExternalFile parses an external file of the given kind. flags is
// the part of the file name after the video's name, such as ".en.forced";
// it yields the language, title and default, forced and hearing-impaired
// flags. It returns false when the extension does not fit the kind.
func (p *Parser) ParseExternalFile(kind ExternalKind, languages LanguageFinder, path, flags string) (ExternalFile, bool) {
	if path == "" {
		return ExternalFile{}, false
	}
	ext := extension(path)
	var known bool
	switch kind {
	case ExternalSubtitle:
		// .idx files carry the languages of VobSub subtitles.
		known = strings.EqualFold(ext, ".idx") || containsFold(p.opts.SubtitleFileExtensions, ext)
	case ExternalAudio:
		known = containsFold(p.opts.AudioFileExtensions, ext)
	case ExternalLyric:
		known = containsFold(p.opts.LyricFileExtensions, ext)
	}
	if !known {
		return ExternalFile{}, false
	}
	info := ExternalFile{Path: path}
	if flags == "" {
		return info, true
	}

	anyContains := func(list []string, s string) bool {
		for _, v := range list {
			if containsStringFold(s, v) {
				return true
			}
		}
		return false
	}
	for _, sep := range p.opts.MediaFlagDelimiters {
		rest := flags
		title := ""
		for rest != "" {
			i := strings.LastIndex(rest, string(sep))
			if i < 0 {
				break
			}
			slice := rest[i:]
			token := slice[len(string(sep)):]
			switch {
			case anyContains(p.opts.MediaDefaultFlags, token):
				info.IsDefault = true
				flags = replaceFold(flags, slice, "")
				rest = rest[:i]
				continue
			case anyContains(p.opts.MediaForcedFlags, token):
				info.IsForced = true
				flags = replaceFold(flags, slice, "")
				rest = rest[:i]
				continue
			}
			lang, found := Language{}, false
			if languages != nil {
				lang, found = languages.FindLanguage(token)
			}
			switch {
			case found && info.Language == "":
				info.Language = lang.code()
				flags = replaceFold(flags, slice, "")
			case found && info.Language == "hin":
				// "hi" is both Hindi and the hearing-impaired flag; it is
				// Hindi only when no other language follows.
				info.IsHearingImpaired = true
				info.Language = lang.code()
				flags = replaceFold(flags, slice, "")
			case containsFold(p.opts.MediaHearingImpairedFlags, token):
				info.IsHearingImpaired = true
				flags = replaceFold(flags, slice, "")
			default:
				title = slice + title
			}
			rest = rest[:i]
		}
		if len(title) >= len(string(sep)) {
			info.Title = title[len(string(sep)):]
		} else {
			info.Title = ""
		}
	}
	return info, true
}

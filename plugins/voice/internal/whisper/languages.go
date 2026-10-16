package whisper

import "strings"

// language is a language Whisper recognizes.
type language struct {
	// code is Whisper's: "jw" for Javanese.
	code, name string
}

// languages are Whisper's, as its tokenizer lists them.
var languages = [...]language{
	{"en", "English"},
	{"zh", "Chinese"},
	{"de", "German"},
	{"es", "Spanish"},
	{"ru", "Russian"},
	{"ko", "Korean"},
	{"fr", "French"},
	{"ja", "Japanese"},
	{"pt", "Portuguese"},
	{"tr", "Turkish"},
	{"pl", "Polish"},
	{"ca", "Catalan"},
	{"nl", "Dutch"},
	{"ar", "Arabic"},
	{"sv", "Swedish"},
	{"it", "Italian"},
	{"id", "Indonesian"},
	{"hi", "Hindi"},
	{"fi", "Finnish"},
	{"vi", "Vietnamese"},
	{"he", "Hebrew"},
	{"uk", "Ukrainian"},
	{"el", "Greek"},
	{"ms", "Malay"},
	{"cs", "Czech"},
	{"ro", "Romanian"},
	{"da", "Danish"},
	{"hu", "Hungarian"},
	{"ta", "Tamil"},
	{"no", "Norwegian"},
	{"th", "Thai"},
	{"ur", "Urdu"},
	{"hr", "Croatian"},
	{"bg", "Bulgarian"},
	{"lt", "Lithuanian"},
	{"la", "Latin"},
	{"mi", "Maori"},
	{"ml", "Malayalam"},
	{"cy", "Welsh"},
	{"sk", "Slovak"},
	{"te", "Telugu"},
	{"fa", "Persian"},
	{"lv", "Latvian"},
	{"bn", "Bengali"},
	{"sr", "Serbian"},
	{"az", "Azerbaijani"},
	{"sl", "Slovenian"},
	{"kn", "Kannada"},
	{"et", "Estonian"},
	{"mk", "Macedonian"},
	{"br", "Breton"},
	{"eu", "Basque"},
	{"is", "Icelandic"},
	{"hy", "Armenian"},
	{"ne", "Nepali"},
	{"mn", "Mongolian"},
	{"bs", "Bosnian"},
	{"kk", "Kazakh"},
	{"sq", "Albanian"},
	{"sw", "Swahili"},
	{"gl", "Galician"},
	{"mr", "Marathi"},
	{"pa", "Punjabi"},
	{"si", "Sinhala"},
	{"km", "Khmer"},
	{"sn", "Shona"},
	{"yo", "Yoruba"},
	{"so", "Somali"},
	{"af", "Afrikaans"},
	{"oc", "Occitan"},
	{"ka", "Georgian"},
	{"be", "Belarusian"},
	{"tg", "Tajik"},
	{"sd", "Sindhi"},
	{"gu", "Gujarati"},
	{"am", "Amharic"},
	{"yi", "Yiddish"},
	{"lo", "Lao"},
	{"uz", "Uzbek"},
	{"fo", "Faroese"},
	{"ht", "Haitian Creole"},
	{"ps", "Pashto"},
	{"tk", "Turkmen"},
	{"nn", "Nynorsk"},
	{"mt", "Maltese"},
	{"sa", "Sanskrit"},
	{"lb", "Luxembourgish"},
	{"my", "Myanmar"},
	{"bo", "Tibetan"},
	{"tl", "Tagalog"},
	{"mg", "Malagasy"},
	{"as", "Assamese"},
	{"tt", "Tatar"},
	{"haw", "Hawaiian"},
	{"ln", "Lingala"},
	{"ha", "Hausa"},
	{"ba", "Bashkir"},
	{"jw", "Javanese"},
	{"su", "Sundanese"},
}

// writtenAs are languages Whisper recognizes as another they are written
// in: Swiss German as German, as Bazarr's Whisper provider maps it.
var writtenAs = map[string]string{"gsw": "de"}

// Tags returns the BCP 47 tags of the languages Whisper recognizes.
func Tags() []string {
	out := make([]string, 0, len(languages)+len(writtenAs))
	for _, l := range languages {
		out = append(out, tagOf(l.code))
	}
	for t := range writtenAs {
		out = append(out, t)
	}
	return out
}

func tagOf(code string) string {
	if code == "jw" {
		return "jv"
	}
	return code
}

// Code returns Whisper's code of a BCP 47 tag, or "" for a language it
// does not recognize.
func Code(tag string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.ReplaceAll(tag, "_", "-")), "-")
	if w, ok := writtenAs[base]; ok {
		base = w
	}
	if base == "jv" {
		base = "jw"
	}
	for _, l := range languages {
		if l.code == base {
			return base
		}
	}
	return ""
}

// Tag returns the BCP 47 tag of a language Whisper names by its code or
// its name, as servers answer; "" for an unknown one.
func Tag(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, l := range languages {
		if l.code == s || strings.ToLower(l.name) == s {
			return tagOf(l.code)
		}
	}
	return ""
}

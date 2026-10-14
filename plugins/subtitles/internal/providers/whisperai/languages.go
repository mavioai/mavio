package whisperai

// whisperLanguage is a language Whisper transcribes, as Bazarr's
// whisper_language_data.
type whisperLanguage struct {
	// alpha2 is Whisper's code: "jw" for Javanese.
	alpha2, alpha3, name string
}

// languages are Whisper's languages, and Swiss German, which is
// transcribed as German.
var languages = [...]whisperLanguage{
	{"en", "eng", "English"},
	{"zh", "zho", "Chinese"},
	{"de", "deu", "German"},
	{"es", "spa", "Spanish"},
	{"ru", "rus", "Russian"},
	{"ko", "kor", "Korean"},
	{"fr", "fra", "French"},
	{"ja", "jpn", "Japanese"},
	{"pt", "por", "Portuguese"},
	{"tr", "tur", "Turkish"},
	{"pl", "pol", "Polish"},
	{"ca", "cat", "Catalan"},
	{"nl", "nld", "Dutch"},
	{"ar", "ara", "Arabic"},
	{"sv", "swe", "Swedish"},
	{"it", "ita", "Italian"},
	{"id", "ind", "Indonesian"},
	{"hi", "hin", "Hindi"},
	{"fi", "fin", "Finnish"},
	{"vi", "vie", "Vietnamese"},
	{"he", "heb", "Hebrew"},
	{"uk", "ukr", "Ukrainian"},
	{"el", "ell", "Greek"},
	{"ms", "msa", "Malay"},
	{"cs", "ces", "Czech"},
	{"ro", "ron", "Romanian"},
	{"da", "dan", "Danish"},
	{"hu", "hun", "Hungarian"},
	{"ta", "tam", "Tamil"},
	{"no", "nor", "Norwegian"},
	{"th", "tha", "Thai"},
	{"ur", "urd", "Urdu"},
	{"hr", "hrv", "Croatian"},
	{"bg", "bul", "Bulgarian"},
	{"lt", "lit", "Lithuanian"},
	{"la", "lat", "Latin"},
	{"mi", "mri", "Maori"},
	{"ml", "mal", "Malayalam"},
	{"cy", "cym", "Welsh"},
	{"sk", "slk", "Slovak"},
	{"te", "tel", "Telugu"},
	{"fa", "fas", "Persian"},
	{"lv", "lav", "Latvian"},
	{"bn", "ben", "Bengali"},
	{"sr", "srp", "Serbian"},
	{"az", "aze", "Azerbaijani"},
	{"sl", "slv", "Slovenian"},
	{"kn", "kan", "Kannada"},
	{"et", "est", "Estonian"},
	{"mk", "mkd", "Macedonian"},
	{"br", "bre", "Breton"},
	{"eu", "eus", "Basque"},
	{"is", "isl", "Icelandic"},
	{"hy", "hye", "Armenian"},
	{"ne", "nep", "Nepali"},
	{"mn", "mon", "Mongolian"},
	{"bs", "bos", "Bosnian"},
	{"kk", "kaz", "Kazakh"},
	{"sq", "sqi", "Albanian"},
	{"sw", "swa", "Swahili"},
	{"gl", "glg", "Galician"},
	{"mr", "mar", "Marathi"},
	{"pa", "pan", "Punjabi"},
	{"si", "sin", "Sinhala"},
	{"km", "khm", "Khmer"},
	{"sn", "sna", "Shona"},
	{"yo", "yor", "Yoruba"},
	{"so", "som", "Somali"},
	{"af", "afr", "Afrikaans"},
	{"oc", "oci", "Occitan"},
	{"ka", "kat", "Georgian"},
	{"be", "bel", "Belarusian"},
	{"tg", "tgk", "Tajik"},
	{"sd", "snd", "Sindhi"},
	{"gu", "guj", "Gujarati"},
	{"am", "amh", "Amharic"},
	{"yi", "yid", "Yiddish"},
	{"lo", "lao", "Lao"},
	{"uz", "uzb", "Uzbek"},
	{"fo", "fao", "Faroese"},
	{"ht", "hat", "Haitian Creole"},
	{"ps", "pus", "Pashto"},
	{"tk", "tuk", "Turkmen"},
	{"nn", "nno", "Nynorsk"},
	{"mt", "mlt", "Maltese"},
	{"sa", "san", "Sanskrit"},
	{"lb", "ltz", "Luxembourgish"},
	{"my", "mya", "Myanmar"},
	{"bo", "bod", "Tibetan"},
	{"tl", "tgl", "Tagalog"},
	{"mg", "mlg", "Malagasy"},
	{"as", "asm", "Assamese"},
	{"tt", "tat", "Tatar"},
	{"haw", "haw", "Hawaiian"},
	{"ln", "lin", "Lingala"},
	{"ha", "hau", "Hausa"},
	{"ba", "bak", "Bashkir"},
	{"jw", "jav", "Javanese"},
	{"su", "sun", "Sundanese"},
	{"gsw", "gsw", "Swiss German"},
}

// languageMapping maps the languages Whisper does not transcribe to
// those it transcribes them as.
var languageMapping = map[string]string{
	"gsw": "deu", // Swiss German as German.
	"und": "eng", // Undetermined as English.
}

// ambiguousCodes are the language codes of audio streams that do not say
// their language, whose language is detected.
var ambiguousCodes = []string{"alg", "art", "ath", "aus", "mis", "mul", "sgn", "und", "zxx"}

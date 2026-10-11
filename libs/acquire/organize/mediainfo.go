package organize

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/release"
)

// MediaInfo is the formatted media information of a file, as the
// MediaInfo tokens write it.
type MediaInfo struct {
	// VideoCodec is the formatted video codec, such as "x264", "h265" or
	// "AV1"; see FormatVideoCodec.
	VideoCodec string
	// VideoBitDepth is the bit depth of the video; zero is written as 8.
	VideoBitDepth int
	// VideoDynamicRangeType is the HDR format: "" for SDR, or "PQ",
	// "HDR10", "HDR10Plus", "HLG", "DV", "DV HDR10", "DV HDR10Plus",
	// "DV HLG" or "DV SDR". Any value makes the video dynamic range "HDR".
	VideoDynamicRangeType string
	// AudioCodec is the formatted audio codec, such as "DTS-HD MA" or
	// "EAC3 Atmos"; see FormatAudioCodec.
	AudioCodec string
	// AudioChannels is the channel count, such as 5.1; see
	// FormatAudioChannels.
	AudioChannels float64
	// AudioLanguages are the ISO 639 codes of the audio streams.
	AudioLanguages []string
	// SubtitleLanguages are the ISO 639 codes of the subtitle streams.
	SubtitleLanguages []string
	// Is3D reports a stereoscopic video (more than one view).
	Is3D bool
}

// VideoDynamicRange returns "HDR" for a video with an HDR format and ""
// otherwise.
func (mi MediaInfo) VideoDynamicRange() string {
	if mi.VideoDynamicRangeType != "" {
		return "HDR"
	}
	return ""
}

// FormatVideoCodec formats an ffprobe video codec name and codec tag
// (codecID) for file names. sceneName, the release or file name, decides
// between the "AVC", "x264" and "h264" or the "HEVC", "x265" and "h265"
// spellings of H.264 and H.265.
func FormatVideoCodec(format, codecID, sceneName string) string {
	result := strings.TrimSpace(format)
	if result == "" {
		return result
	}
	switch {
	case codecID == "x264":
		return "x264"
	case format == "h264":
		return sceneNameMatch(sceneName, "AVC", "x264", "h264")
	case codecID == "x265":
		return "x265"
	case format == "hevc":
		return sceneNameMatch(sceneName, "HEVC", "x265", "h265")
	case format == "mpeg2video":
		return "MPEG2"
	case format == "mpeg1video":
		return "MPEG"
	case format == "mpeg4" || strings.Contains(format, "msmpeg4"):
		switch {
		case strings.ToUpper(codecID) == "XVID":
			return "XviD"
		case codecID == "DIV3" || codecID == "DX50" || strings.ToUpper(codecID) == "DIVX":
			return "DivX"
		}
		return ""
	case format == "vc1":
		return "VC1"
	case format == "av1":
		return "AV1"
	case strings.Contains(format, "vp6"):
		return "VP6"
	case format == "vp7" || format == "vp8" || format == "vp9":
		return strings.ToUpper(format)
	case format == "wmv1" || format == "wmv2" || format == "wmv3":
		return "WMV"
	case slices.Contains([]string{"qtrle", "rpza", "rv10", "rv20", "rv30", "rv40", "cinepak", "rawvideo", "msvideo1"}, format):
		return ""
	}
	return result
}

// FormatAudioCodec formats an ffprobe audio codec name, codec tag
// (codecID) and profile for file names.
func FormatAudioCodec(format, codecID, profile string) string {
	switch {
	case format == "":
		return ""
	case codecID == "thd+":
		return "TrueHD Atmos"
	case format == "truehd":
		return "TrueHD"
	case format == "flac":
		return "FLAC"
	case format == "dts":
		switch profile {
		case "DTS:X":
			return "DTS-X"
		case "DTS-HD MA", "DTS-ES", "DTS-HD HRA", "DTS Express", "DTS 96/24":
			return profile
		}
		return "DTS"
	case codecID == "ec+3":
		return "EAC3 Atmos"
	case format == "eac3":
		return "EAC3"
	case format == "ac3":
		return "AC3"
	case format == "aac":
		if codecID == "A_AAC/MPEG4/LC/SBR" {
			return "HE-AAC"
		}
		return "AAC"
	case format == "mp3":
		return "MP3"
	case format == "mp2":
		return "MP2"
	case format == "opus":
		return "Opus"
	case strings.HasPrefix(format, "pcm_") || strings.HasPrefix(format, "adpcm_"):
		return "PCM"
	case format == "vorbis":
		return "Vorbis"
	case format == "wmav1" || format == "wmav2" || format == "wmapro":
		return "WMA"
	}
	return format
}

// FormatAudioChannels returns the channel count of a channel layout that
// starts with it, such as "5.1(side)", and channels otherwise.
func FormatAudioChannels(channelPositions string, channels float64) float64 {
	if len(channelPositions) >= 3 && isDigit(channelPositions[0]) && channelPositions[1] == '.' && isDigit(channelPositions[2]) {
		if v, err := strconv.ParseFloat(channelPositions[:3], 64); err == nil && v != 0 {
			return v
		}
	}
	return channels
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// sceneNameMatch returns the first token sceneName contains, ignoring case
// and its file extension, or the last token.
func sceneNameMatch(sceneName string, tokens ...string) string {
	name := ""
	if strings.TrimSpace(sceneName) != "" {
		name = strings.ToLower(release.RemoveFileExtension(sceneName, release.Movie))
	}
	for _, t := range tokens {
		if strings.Contains(name, strings.ToLower(t)) {
			return t
		}
	}
	return tokens[len(tokens)-1]
}

// languagesToken formats language codes as upper-case ISO 639-1 codes
// joined by "+". A filter "A+B" keeps the listed codes in its order, a
// filter "-A-B" removes them, and a filter ending in "+" appends "--"
// when it dropped any code. skipEnglishOnly writes nothing for English
// alone; quoted wraps a non-empty result in brackets.
func languagesToken(languages []string, filter string, skipEnglishOnly, quoted bool) string {
	var tokens []string
	for _, item := range languages {
		if strings.TrimSpace(item) != "" && item != "und" {
			tokens = append(tokens, twoLetterLanguageCode(strings.TrimSpace(item)))
		}
	}
	tokens = distinct(tokens)
	filtered := tokens
	if filter != "" {
		if strings.HasPrefix(filter, "-") {
			excluded := strings.Split(filter, "-")
			filtered = nil
			for _, t := range tokens {
				if !slices.Contains(excluded, t) {
					filtered = append(filtered, t)
				}
			}
		} else {
			filtered = nil
			for _, t := range distinct(strings.Split(filter, "+")) {
				if slices.Contains(tokens, t) {
					filtered = append(filtered, t)
				}
			}
		}
		if strings.HasSuffix(filter, "+") && len(filtered) != len(tokens) {
			filtered = append(filtered, "--")
		}
	}
	if skipEnglishOnly && len(filtered) == 1 && filtered[0] == "EN" {
		return ""
	}
	response := strings.Join(filtered, "+")
	if quoted && strings.TrimSpace(response) != "" {
		return "[" + response + "]"
	}
	return response
}

func distinct(s []string) []string {
	var out []string
	for _, v := range s {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// twoLetterLanguageCode returns the upper-case ISO 639-1 code of an ISO
// 639-1 or 639-2 code, or code itself when it is unknown.
func twoLetterLanguageCode(code string) string {
	lower := strings.ToLower(code)
	if mapped, ok := iso639BT[lower]; ok {
		lower = mapped
	}
	switch len(lower) {
	case 2:
		if _, ok := iso6391[lower]; ok {
			return strings.ToUpper(lower)
		}
	case 3:
		if two, ok := iso6392T[lower]; ok {
			return strings.ToUpper(two)
		}
	}
	return code
}

// iso639BT maps ISO 639-2/B codes, and a few individual languages, to
// their ISO 639-2/T codes, as Radarr does.
var iso639BT = map[string]string{
	"alb": "sqi", "arm": "hye", "baq": "eus", "bur": "mya", "chi": "zho", "cze": "ces", "dut": "nld",
	"fre": "fra", "geo": "kat", "ger": "deu", "gre": "ell", "gsw": "deu", "ice": "isl", "mac": "mkd",
	"mao": "mri", "may": "msa", "per": "fas", "rum": "ron", "slo": "slk", "tib": "bod", "wel": "cym",
	"khk": "mon", "mvf": "mon",
}

// iso6392T maps the ISO 639-2/T codes of the ISO 639-1 languages to their
// ISO 639-1 codes.
var iso6392T = map[string]string{
	"aar": "aa", "abk": "ab", "ave": "ae", "afr": "af", "aka": "ak", "amh": "am", "arg": "an", "ara": "ar",
	"asm": "as", "ava": "av", "aym": "ay", "aze": "az", "bak": "ba", "bel": "be", "bul": "bg", "bis": "bi",
	"bam": "bm", "ben": "bn", "bod": "bo", "bre": "br", "bos": "bs", "cat": "ca", "che": "ce", "cha": "ch",
	"cos": "co", "cre": "cr", "ces": "cs", "chu": "cu", "chv": "cv", "cym": "cy", "dan": "da", "deu": "de",
	"div": "dv", "dzo": "dz", "ewe": "ee", "ell": "el", "eng": "en", "epo": "eo", "spa": "es", "est": "et",
	"eus": "eu", "fas": "fa", "ful": "ff", "fin": "fi", "fij": "fj", "fao": "fo", "fra": "fr", "fry": "fy",
	"gle": "ga", "gla": "gd", "glg": "gl", "grn": "gn", "guj": "gu", "glv": "gv", "hau": "ha", "heb": "he",
	"hin": "hi", "hmo": "ho", "hrv": "hr", "hat": "ht", "hun": "hu", "hye": "hy", "her": "hz", "ina": "ia",
	"ind": "id", "ile": "ie", "ibo": "ig", "iii": "ii", "ipk": "ik", "ido": "io", "isl": "is", "ita": "it",
	"iku": "iu", "jpn": "ja", "jav": "jv", "kat": "ka", "kon": "kg", "kik": "ki", "kua": "kj", "kaz": "kk",
	"kal": "kl", "khm": "km", "kan": "kn", "kor": "ko", "kau": "kr", "kas": "ks", "kur": "ku", "kom": "kv",
	"cor": "kw", "kir": "ky", "lat": "la", "ltz": "lb", "lug": "lg", "lim": "li", "lin": "ln", "lao": "lo",
	"lit": "lt", "lub": "lu", "lav": "lv", "mlg": "mg", "mah": "mh", "mri": "mi", "mkd": "mk", "mal": "ml",
	"mon": "mn", "mar": "mr", "msa": "ms", "mlt": "mt", "mya": "my", "nau": "na", "nob": "nb", "nde": "nd",
	"nep": "ne", "ndo": "ng", "nld": "nl", "nno": "nn", "nor": "no", "nbl": "nr", "nav": "nv", "nya": "ny",
	"oci": "oc", "oji": "oj", "orm": "om", "ori": "or", "oss": "os", "pan": "pa", "pli": "pi", "pol": "pl",
	"pus": "ps", "por": "pt", "que": "qu", "roh": "rm", "run": "rn", "ron": "ro", "rus": "ru", "kin": "rw",
	"san": "sa", "srd": "sc", "snd": "sd", "sme": "se", "sag": "sg", "sin": "si", "slk": "sk", "slv": "sl",
	"smo": "sm", "sna": "sn", "som": "so", "sqi": "sq", "srp": "sr", "ssw": "ss", "sot": "st", "sun": "su",
	"swe": "sv", "swa": "sw", "tam": "ta", "tel": "te", "tgk": "tg", "tha": "th", "tir": "ti", "tuk": "tk",
	"tgl": "tl", "tsn": "tn", "ton": "to", "tur": "tr", "tso": "ts", "tat": "tt", "twi": "tw", "tah": "ty",
	"uig": "ug", "ukr": "uk", "urd": "ur", "uzb": "uz", "ven": "ve", "vie": "vi", "vol": "vo", "wln": "wa",
	"wol": "wo", "xho": "xh", "yid": "yi", "yor": "yo", "zha": "za", "zho": "zh", "zul": "zu",
}

// iso6391 is the set of ISO 639-1 codes.
var iso6391 = func() map[string]struct{} {
	m := make(map[string]struct{}, len(iso6392T))
	for _, two := range iso6392T {
		m[two] = struct{}{}
	}
	return m
}()

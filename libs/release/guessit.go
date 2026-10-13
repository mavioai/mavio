package release

import (
	"slices"
	"strings"
)

// The streaming services, video codecs and audio codecs of release names
// follow guessit 3.8.0 (LGPL-3.0), which Bazarr scores subtitles with:
// guessit's names as values, its patterns, and its rule that a streaming
// service stands right before a source or a video codec.

// guessitSeps is a character class of guessit's separators; a "-" in its
// patterns stands for one of them.
const guessitSeps = `[ \[\](){}+*|=\-_~#/\\.,;:]`

// streamingServices are guessit's streaming services and their patterns.
var streamingServices = []struct {
	name     string
	patterns []string
}{
	{"9Now", []string{`9NOW`}},
	{"A&E", []string{`AE`, `A\&E`}},
	{"ABC", []string{`AMBC`}},
	{"ABC Australia", []string{`AUBC`}},
	{"Al Jazeera English", []string{`AJAZ`}},
	{"AMC", []string{`AMC`}},
	{"Amazon Prime", []string{`AMZN`, `AMZN\-CBR`, `Amazon`, `Amazon` + sep + `?Prime`}},
	{"Adult Swim", []string{`AS`, `Adult` + sep + `?Swim`}},
	{"America's Test Kitchen", []string{`ATK`}},
	{"Animal Planet", []string{`ANPL`}},
	{"AnimeLab", []string{`ANLB`}},
	{"AOL", []string{`AOL`}},
	{"AppleTV", []string{`ATVP`, `ATV\+`, `APTV`}},
	{"ARD", []string{`ARD`}},
	{"BBC iPlayer", []string{`iP`, `BBC` + sep + `?iPlayer`}},
	{"Binge", []string{`BNGE`}},
	{"Blackpills", []string{`BKPL`}},
	{"BluTV", []string{`BLU`}},
	{"Boomerang", []string{`BOOM`}},
	{"Disney+", []string{`DSNP`}},
	{"BravoTV", []string{`BRAV`}},
	{"Canal+", []string{`CNLP`}},
	{"Cartoon Network", []string{`CN`}},
	{"CBC", []string{`CBC`}},
	{"CBS", []string{`CBS`}},
	{"CNBC", []string{`CNBC`}},
	{"Comedy Central", []string{`CC`, `Comedy` + sep + `?Central`}},
	{"Channel 4", []string{`ALL4`, `4OD`}},
	{"CHRGD", []string{`CHGD`}},
	{"Cinemax", []string{`CMAX`}},
	{"Country Music Television", []string{`CMT`}},
	{"Comedians in Cars Getting Coffee", []string{`CCGC`}},
	{"Crave", []string{`CRAV`}},
	{"Crunchy Roll", []string{`CR`, `Crunchy` + sep + `?Roll`}},
	{"Crackle", []string{`CRKL`}},
	{"CSpan", []string{`CSPN`}},
	{"CTV", []string{`CTV`}},
	{"CuriosityStream", []string{`CUR`}},
	{"CWSeed", []string{`CWS`}},
	{"Daisuki", []string{`DSKI`}},
	{"DC Universe", []string{`DCU`}},
	{"Deadhouse Films", []string{`DHF`}},
	{"DramaFever", []string{`DF`, `DramaFever`}},
	{"Digiturk Diledigin Yerde", []string{`DDY`}},
	{"Discovery", []string{`DISC`, `Discovery`}},
	{"Discovery Plus", []string{`DSCP`}},
	{"Disney", []string{`DSNY`, `Disney`}},
	{"DIY Network", []string{`DIY`}},
	{"Doc Club", []string{`DOCC`}},
	{"DPlay", []string{`DPLY`}},
	{"E!", []string{`ETV`}},
	{"ePix", []string{`EPIX`}},
	{"El Trece", []string{`ETTV`}},
	{"ESPN", []string{`ESPN`}},
	{"Esquire", []string{`ESQ`}},
	{"Facebook Watch", []string{`FBWatch`}},
	{"Family", []string{`FAM`}},
	{"Family Jr", []string{`FJR`}},
	{"Fandor", []string{`FANDOR`}},
	{"Food Network", []string{`FOOD`}},
	{"Fox", []string{`FOX`}},
	{"Fox Premium", []string{`FOXP`}},
	{"Foxtel", []string{`FXTL`}},
	{"Freeform", []string{`FREE`}},
	{"FYI Network", []string{`FYI`}},
	{"GagaOOLala", []string{`Gaga`}},
	{"Global", []string{`GLBL`}},
	{"GloboSat Play", []string{`GLOB`}},
	{"Hallmark", []string{`HLMK`}},
	{"HBO Go", []string{`HBO`, `HBO` + sep + `?Go`}},
	{"HBO Max", []string{`HMAX`}},
	{"HGTV", []string{`HGTV`}},
	{"History", []string{`HIST`, `History`}},
	{"Hulu", []string{`HULU`}},
	{"Investigation Discovery", []string{`ID`}},
	{"IFC", []string{`IFC`}},
	{"hoichoi", []string{`HoiChoi`}},
	{"iflix", []string{`IFX`}},
	{"iQIYI", []string{`iQIYI`}},
	{"iTunes", []string{`iTunes`, `(?-i:iT)`}},
	{"ITV", []string{`ITV`}},
	{"Knowledge Network", []string{`KNOW`}},
	{"Lifetime", []string{`LIFE`}},
	{"Motor Trend OnDemand", []string{`MTOD`}},
	{"MBC", []string{`MBC`, `MBCVOD`}},
	{"MSNBC", []string{`MNBC`}},
	{"MTV", []string{`MTV`}},
	{"MUBI", []string{`MUBI`}},
	{"National Audiovisual Institute", []string{`INA`}},
	{"National Film Board", []string{`NFB`}},
	{"National Geographic", []string{`NATG`, `National` + sep + `?Geographic`}},
	{"NBA TV", []string{`NBA`, `NBA` + sep + `?TV`}},
	{"NBC", []string{`NBC`}},
	{"Netflix", []string{`NF`, `Netflix`}},
	{"NFL", []string{`NFL`}},
	{"NFL Now", []string{`NFLN`}},
	{"NHL GameCenter", []string{`GC`}},
	{"Nickelodeon", []string{`NICK`, `Nickelodeon`, `NICKAPP`}},
	{"Norsk Rikskringkasting", []string{`NRK`}},
	{"OnDemandKorea", []string{`ODK`, `OnDemandKorea`}},
	{"Opto", []string{`OPTO`}},
	{"Oprah Winfrey Network", []string{`OWN`}},
	{"Paramount+", []string{`PMTP`, `PMNP`, `PMT\+`, `Paramount\+`, `ParamountPlus`}},
	{"PBS", []string{`PBS`}},
	{"PBS Kids", []string{`PBSK`}},
	{"Peacock", []string{`PCOK`, `Peacock`}},
	{"Playstation Network", []string{`PSN`}},
	{"Pluzz", []string{`PLUZ`}},
	{"PokerGO", []string{`POGO`}},
	{"Rakuten TV", []string{`RKTN`}},
	{"The Roku Channel", []string{`ROKU`}},
	{"RTE One", []string{`RTE`}},
	{"RUUTU", []string{`RUUTU`}},
	{"SBS", []string{`SBS`}},
	{"Science Channel", []string{`SCI`}},
	{"SeeSo", []string{`SESO`, `SeeSo`}},
	{"Shomi", []string{`SHMI`}},
	{"Showtime", []string{`SHO`}},
	{"Sony", []string{`SONY`}},
	{"Spike", []string{`SPIK`}},
	{"Spike TV", []string{`SPKE`, `Spike` + sep + `?TV`}},
	{"Sportsnet", []string{`SNET`}},
	{"Sprout", []string{`SPRT`}},
	{"Stan", []string{`STAN`}},
	{"Starz", []string{`STZ`}},
	{"Sveriges Television", []string{`SVT`}},
	{"SwearNet", []string{`SWER`}},
	{"Syfy", []string{`SYFY`}},
	{"TBS", []string{`TBS`}},
	{"TFou", []string{`TFOU`}},
	{"The CW", []string{`CW`, `The` + sep + `?CW`}},
	{"TLC", []string{`TLC`}},
	{"TubiTV", []string{`TUBI`}},
	{"TV3 Ireland", []string{`TV3`}},
	{"TV4 Sweeden", []string{`TV4`}},
	{"TVING", []string{`TVING`}},
	{"TV Land", []string{`TVL`, `TV` + sep + `?Land`}},
	{"TVNZ", []string{`TVNZ`}},
	{"UFC", []string{`UFC`}},
	{"UFC Fight Pass", []string{`FP`}},
	{"UKTV", []string{`UKTV`}},
	{"Univision", []string{`UNIV`}},
	{"USA Network", []string{`USAN`}},
	{"Velocity", []string{`VLCT`}},
	{"VH1", []string{`VH1`}},
	{"Viceland", []string{`VICE`}},
	{"Viki", []string{`VIKI`}},
	{"Vimeo", []string{`VMEO`}},
	{"VRV", []string{`VRV`}},
	{"W Network", []string{`WNET`}},
	{"WatchMe", []string{`WME`}},
	{"WWE Network", []string{`WWEN`}},
	{"Xbox Video", []string{`XBOX`}},
	{"Yahoo", []string{`YHOO`}},
	{"YouTube Red", []string{`RED`}},
	{"ZDF", []string{`ZDF`}},
}

// sep stands for a separator in the patterns of streamingServices.
const sep = guessitSeps

// streamingSuffix is the start of what follows a streaming service: a
// source or a video codec after separators, or, after separators or none,
// one of guessit's other properties tagged as streaming service suffixes.
const streamingSuffix = `(?:` + sep + `+(?:WEB` + sep + `?(?:DL|Rip|U?HD|Cap)|DL` + sep + `?WEB|U?HD` + sep + `?TV|[SP]D` + sep + `?TV|TV` + sep + `?Rip|DVD|Blu` + sep + `?ray|BD|BR` + sep + `?Rip|VOD|DSR|PPV|[hx]` + sep + `?26[2-5]|HEVC|AVC|XviD|DivX|VC` + sep + `?1|VP[789])|` + sep + `*` + streamingOther + `(?:$|` + sep + `))`

// streamingOther are guessit's other properties that may stand before or
// after a streaming service: Proper, Fix, HD, Full HD and Ultra HD.
const streamingOther = `(?:Proper|Repack\d*|Rerip\d*|Real|Fix(?:ed)?|Dirfix|Nfofix|Prooffix|U?HD|FHD|Full` + sep + `?HD|Ultra)`

var streamingServiceRegexes = func() []*regex {
	out := make([]*regex, len(streamingServices))
	for i, s := range streamingServices {
		service := `(?:` + strings.Join(s.patterns, "|") + `)`
		out[i] = re(`(?<=^|`+sep+`)`+service+`(?=`+streamingSuffix+`)|(?<=`+streamingOther+sep+`)`+service+`(?=$|`+sep+`)`, ignoreCase)
	}
	return out
}()

// ParseStreamingService returns the streaming service a release name
// names, as guessit names it ("Netflix", "Amazon Prime"), or "". The
// leftmost service standing before a source or a video codec wins.
func ParseStreamingService(title string) string {
	best, at := "", -1
	for i, r := range streamingServiceRegexes {
		if m := r.match(title); m != nil && !trailing(title, m.m.Index) && (at < 0 || m.m.Index < at) {
			best, at = streamingServices[i].name, m.m.Index
		}
	}
	return best
}

// codecPattern is a codec's guessit name and pattern.
type codecPattern struct {
	name    string
	pattern string
}

// videoCodecs are guessit's video codecs.
var videoCodecs = []codecPattern{
	{"RealVideo", `Rv\d{2}`},
	{"MPEG-2", `Mpe?g` + sep + `?2|[hx]` + sep + `?262`},
	{"DivX", `DVDivX|DivX`},
	{"Xvid", `XviD`},
	{"VC-1", `VC` + sep + `?1`},
	{"VP7", `VP7`},
	{"VP8", `VP80?`},
	{"VP9", `VP9`},
	{"H.263", `[hx]` + sep + `?263`},
	{"H.264", `[hx]` + sep + `?264|(?:MPEG` + sep + `?4)?AVC(?:HD)?`},
	{"H.265", `[hx]` + sep + `?265|HEVC`},
}

// audioCodecs are guessit's audio codecs.
var audioCodecs = []codecPattern{
	{"MP3", `MP3|LAME|LAME\d+` + sep + `?\d+`},
	{"MP2", `MP2`},
	{"Dolby Digital", `Dolby|DolbyDigital|Dolby` + sep + `Digital|DD|AC` + sep + `?3D?`},
	{"Dolby Atmos", `Atmos|Dolby` + sep + `?Atmos`},
	{"AAC", `AAC`},
	{"Dolby Digital Plus", `DDP|DD\+|E` + sep + `?AC` + sep + `?3`},
	{"FLAC", `Flac`},
	{"DTS", `DTS`},
	{"DTS-HD", `DTS` + sep + `?HD|DTS(?=` + sep + `?MA)`},
	{"DTS:X", `DTS:X|DTS-X|DTSX`},
	{"Dolby TrueHD", `True` + sep + `?HD`},
	{"Opus", `Opus`},
	{"Vorbis", `Vorbis`},
	{"PCM", `PCM`},
	{"LPCM", `LPCM`},
}

// codecRegexes compiles codec patterns: a codec stands between separators,
// or, followed by digits, before its channels or bit depth.
func codecRegexes(codecs []codecPattern) []*regex {
	out := make([]*regex, len(codecs))
	for i, c := range codecs {
		out[i] = re(`(?<=^|`+sep+`)(?:`+c.pattern+`)(?=$|`+sep+`|\d)`, ignoreCase)
	}
	return out
}

var (
	videoCodecRegexes = codecRegexes(videoCodecs)
	audioCodecRegexes = codecRegexes(audioCodecs)
)

// codecMatch is a codec found in a name.
type codecMatch struct {
	name              string
	order, start, end int
}

// findCodecs returns the codecs a name names, in order, a codec found
// within a longer one's text, or the same text as a codec listed after it,
// left out ("DTS" in "DTS-HD", or before "MA").
func findCodecs(title string, codecs []codecPattern, regexes []*regex) []codecMatch {
	var found []codecMatch
	for i, r := range regexes {
		for _, m := range r.matches(title) {
			if trailing(title, m.m.Index) {
				continue
			}
			found = append(found, codecMatch{codecs[i].name, i, m.m.Index, m.m.Index + m.m.Length})
		}
	}
	var out []codecMatch
	for _, f := range found {
		inside := slices.ContainsFunc(found, func(g codecMatch) bool {
			longer := g.end-g.start > f.end-f.start
			same := g.start == f.start && g.end == f.end && g.order > f.order
			return g.start <= f.start && f.end <= g.end && (longer || same)
		})
		if !inside && !slices.ContainsFunc(out, func(g codecMatch) bool { return g.name == f.name }) {
			out = append(out, f)
		}
	}
	slices.SortStableFunc(out, func(a, b codecMatch) int { return a.start - b.start })
	return out
}

// trailing reports whether text starting at start is in the release
// group ending a name after a dash ("x264-DTS", "H264-NFHD"), or is its
// DivX extension, which guessit takes for a container ("S02E15.divx").
func trailing(title string, start int) bool {
	runes := []rune(title)
	if start == 0 {
		return false
	}
	rest := string(runes[start:])
	switch runes[start-1] {
	case '-':
		return !strings.ContainsAny(rest, " []{}()+*|=-_~#/\\.,;:")
	case '.':
		return strings.EqualFold(rest, "divx")
	}
	return false
}

// ParseVideoCodec returns the video codec a release name names, as guessit
// names it ("H.264", "H.265", "Xvid"), or "".
func ParseVideoCodec(title string) string {
	if found := findCodecs(title, videoCodecs, videoCodecRegexes); len(found) > 0 {
		return found[0].name
	}
	return ""
}

// ParseAudioCodecs returns the audio codecs a release name names, as
// guessit names them ("Dolby Digital Plus", "DTS-HD", "Dolby Atmos").
func ParseAudioCodecs(title string) []string {
	var out []string
	for _, f := range findCodecs(title, audioCodecs, audioCodecRegexes) {
		out = append(out, f.name)
	}
	return out
}

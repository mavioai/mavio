package naming

import "github.com/mavioai/mavio/libs/core"

// Options holds the naming rules: extensions, regular expressions and
// tokens. DefaultOptions returns Jellyfin's rules. Regular expressions use
// .NET syntax (lookaround, atomic groups, named groups as (?<name>…)) and
// match case-insensitively.
type Options struct {
	VideoFileExtensions []string
	// VideoFlagDelimiters split a video file name into tokens for 3D
	// detection.
	VideoFlagDelimiters []rune
	StubFileExtensions  []string
	StubTypes           []StubTypeRule
	VideoStackRules     []FileStackRule
	// CleanDateTimes extract the name and year from a video name; the first
	// match wins.
	CleanDateTimes []string
	// CleanStrings strip release tags from a video name; each match
	// replaces the name with its "cleaned" group.
	CleanStrings           []string
	SubtitleFileExtensions []string
	LyricFileExtensions    []string
	AudioFileExtensions    []string
	// AlbumStackingPrefixes mark the folders of a multi-disc album
	// ("CD 1", "Disc 2").
	AlbumStackingPrefixes []string
	// ArtistSubfolders are folder names inside an artist folder that hold
	// albums.
	ArtistSubfolders []string
	// MediaFlagDelimiters split the flags of an external subtitle or audio
	// file name.
	MediaFlagDelimiters       []rune
	MediaForcedFlags          []string
	MediaDefaultFlags         []string
	MediaHearingImpairedFlags []string
	// EpisodeExpressions are tried in order; the first match wins.
	EpisodeExpressions []EpisodeExpression
	// MultipleEpisodeExpressions find the end of a multi-episode range.
	MultipleEpisodeExpressions []EpisodeExpression
	VideoExtraRules            []ExtraRule
	Format3DRules              []Format3DRule
	// AudioBookPartsExpressions capture "chapter" and "part" numbers.
	AudioBookPartsExpressions []string
	// AudioBookNamesExpressions capture "name" and "year".
	AudioBookNamesExpressions []string
}

// EpisodeExpression is a regular expression that recognizes an episode.
type EpisodeExpression struct {
	Expression string
	// ByDate expressions capture an air date; DateFormats lists its
	// layouts in .NET notation ("yyyy.MM.dd").
	ByDate      bool
	DateFormats []string
	// Optimistic expressions are guesses tried only when asked for.
	Optimistic bool
	// Named expressions capture the named groups "seriesname",
	// "seasonnumber", "epnumber" and "endingepnumber"; the others capture
	// the season and episode as groups 1 and 2.
	Named bool
	// SupportsAbsoluteNumbers is set for expressions that recognize
	// absolute episode numbers.
	SupportsAbsoluteNumbers bool
}

// ExtraRuleType says what an extra rule's token is matched against.
type ExtraRuleType int

// Extra rule types.
const (
	// ExtraSuffix matches the end of the file name without extension.
	ExtraSuffix ExtraRuleType = iota
	// ExtraFilename matches the whole file name without extension.
	ExtraFilename
	// ExtraRegex matches the file name against a regular expression.
	ExtraRegex
	// ExtraDirectoryName matches the name of the containing folder.
	ExtraDirectoryName
)

// MediaType is the kind of file an extra rule applies to.
type MediaType int

// Media types.
const (
	MediaAudio MediaType = iota
	MediaPhoto
	MediaVideo
)

// ExtraRule recognizes trailers, featurettes and other extras.
type ExtraRule struct {
	Kind  core.ExtraKind
	Type  ExtraRuleType
	Token string
	Media MediaType
}

// StubTypeRule maps the token of a stub file name ("movie.bluray.disc") to
// a stub type.
type StubTypeRule struct {
	Token    string
	StubType string
}

// FileStackRule recognizes a part of a multi-file video ("movie cd1.avi").
// The expression captures "filename", "parttype" and "number".
type FileStackRule struct {
	Expression string
	// Numerical rules number parts with digits, the others with letters.
	Numerical bool
}

// Format3DRule recognizes a 3D format token, optionally only after a
// preceding token ("3d.hsbs").
type Format3DRule struct {
	Token          string
	PrecedingToken string
}

// DefaultOptions returns Jellyfin's naming rules.
func DefaultOptions() Options {
	o := Options{
		VideoFileExtensions: []string{
			".001", ".3g2", ".3gp", ".amv", ".asf", ".asx", ".avi", ".bin", ".bivx", ".divx", ".dv", ".dvr-ms",
			".f4v", ".fli", ".flv", ".ifo", ".img", ".iso", ".m2t", ".m2ts", ".m2v", ".m4v", ".mkv", ".mk3d",
			".mov", ".mp4", ".mpe", ".mpeg", ".mpg", ".mts", ".mxf", ".nrg", ".nsv", ".nuv", ".ogm", ".ogv",
			".pva", ".qt", ".rec", ".rm", ".rmvb", ".strm", ".svq3", ".tp", ".ts", ".ty", ".viv", ".vob",
			".vp3", ".webm", ".wmv", ".wtv", ".xvid",
		},
		VideoFlagDelimiters: []rune{'(', ')', '-', '.', '_', '[', ']'},
		StubFileExtensions:  []string{".disc"},
		StubTypes: []StubTypeRule{
			{Token: "dvd", StubType: "dvd"},
			{Token: "hddvd", StubType: "hddvd"},
			{Token: "bluray", StubType: "bluray"},
			{Token: "brrip", StubType: "bluray"},
			{Token: "bd25", StubType: "bluray"},
			{Token: "bd50", StubType: "bluray"},
			{Token: "vhs", StubType: "vhs"},
			{Token: "HDTV", StubType: "tv"},
			{Token: "PDTV", StubType: "tv"},
			{Token: "DSR", StubType: "tv"},
		},
		VideoStackRules: []FileStackRule{
			{Expression: `^(?<filename>.*?)(?:(?<=[\]\)\}])|[ _.-]+)[\(\[]?(?<parttype>cd|dvd|part|pt|dis[ck])[ _.-]*(?<number>[0-9]+)[\)\]]?(?:\.[^.]+)?$`, Numerical: true},
			{Expression: `^(?<filename>.*?)(?:(?<=[\]\)\}])|[ _.-]+)[\(\[]?(?<parttype>cd|dvd|part|pt|dis[ck])[ _.-]*(?<number>[a-d])[\)\]]?(?:\.[^.]+)?$`, Numerical: false},
		},
		CleanDateTimes: []string{
			`(.+[^_\,\.\(\)\[\]\-])[_\.\(\)\[\]\-](19[0-9]{2}|20[0-9]{2})(?![0-9]+|\W[0-9]{2}\W[0-9]{2})([ _\,\.\(\)\[\]\-][^0-9]|).*(19[0-9]{2}|20[0-9]{2})*`,
			`(.+[^_\,\.\(\)\[\]\-])[ _\.\(\)\[\]\-]+(19[0-9]{2}|20[0-9]{2})(?![0-9]+|\W[0-9]{2}\W[0-9]{2})([ _\,\.\(\)\[\]\-][^0-9]|).*(19[0-9]{2}|20[0-9]{2})*`,
		},
		CleanStrings: []string{
			`^\s*(?<cleaned>.+?)[ _\,\.\(\)\[\]\-](3d|sbs|tab|hsbs|htab|mvc|HDR|HDC|UHD|UltraHD|4k|ac3|dts|custom|dc|divx|divx5|dsr|dsrip|dutch|dvd|dvdrip|dvdscr|dvdscreener|screener|dvdivx|cam|fragment|fs|hdtv|hdrip|hdtvrip|internal|limited|multi|subs|ntsc|ogg|ogm|pal|pdtv|proper|repack|rerip|retail|cd[1-9]|r5|bd5|bd|se|svcd|swedish|german|read.nfo|nfofix|unrated|ws|web-dl|telesync|ts|telecine|tc|brrip|bdrip|480p|480i|576p|576i|720p|720i|1080p|1080i|2160p|hrhd|hrhdtv|hddvd|bluray|blu-ray|x264|x265|h264|h265|xvid|xvidvd|xxx|www.www|AAC|DTS)(?=[ _\,\.\(\)\[\]\-]|$)`,
			`^\s*(?<cleaned>.+?)((\s*\[[^\]]+\]\s*)+)(\.[^\s]+)?$`,
			`^\s*(?<cleaned>.+?)\WE[0-9]+(-|~)E?[0-9]+(\W|$)`,
			`^\s*\[[^\]]+\](?!\.\w+$)\s*(?<cleaned>.+)`,
			`^\s*(?<cleaned>.+?)\s+-\s+[0-9]+\s*$`,
			`^\s*(?<cleaned>.+?)(([-._ ](trailer|sample))|-(scene|clip|behindthescenes|deleted|deletedscene|featurette|short|interview|other|extra))$`,
		},
		SubtitleFileExtensions: []string{".ass", ".mks", ".sami", ".smi", ".srt", ".ssa", ".sub", ".sup", ".vtt"},
		LyricFileExtensions:    []string{".lrc", ".elrc", ".txt"},
		AlbumStackingPrefixes:  []string{"cd", "digital media", "disc", "disk", "vol", "volume", "part", "act"},
		ArtistSubfolders: []string{
			"albums", "broadcasts", "bootlegs", "compilations", "dj-mixes", "eps", "live", "mixtapes", "others",
			"remixes", "singles", "soundtracks", "spokenwords", "streets",
		},
		AudioFileExtensions: []string{
			".669", ".3gp", ".aa", ".aac", ".aax", ".ac3", ".act", ".adp", ".adplug", ".adx", ".afc", ".amf",
			".aif", ".aifc", ".aiff", ".alac", ".amr", ".ape", ".ast", ".au", ".awb", ".cda", ".cue", ".dmf",
			".dsf", ".dsm", ".dsp", ".dts", ".dvf", ".eac3", ".ec3", ".far", ".flac", ".gdm", ".gsm", ".gym",
			".hps", ".imf", ".it", ".m15", ".m4a", ".m4b", ".mac", ".med", ".mka", ".mmf", ".mod", ".mogg",
			".mp2", ".mp3", ".mpa", ".mpc", ".mpp", ".mp+", ".msv", ".nmf", ".nsf", ".nsv", ".oga", ".ogg",
			".okt", ".opus", ".pls", ".ra", ".rf64", ".rm", ".s3m", ".sfx", ".shn", ".sid", ".stm", ".strm",
			".ult", ".uni", ".vox", ".wav", ".wma", ".wv", ".xm", ".xsp", ".ymf",
		},
		MediaFlagDelimiters:       []rune{'.'},
		MediaForcedFlags:          []string{"foreign", "forced"},
		MediaDefaultFlags:         []string{"default"},
		MediaHearingImpairedFlags: []string{"cc", "hi", "sdh"},
		EpisodeExpressions: []EpisodeExpression{
			// Kodi standard naming.
			// foo.s01.e01, foo.s01_e01, S01E02 foo, S01 - E02
			named(`.*(\\|\/)(?<seriesname>((?![Ss]([0-9]+)[][ ._-]*[Ee]([0-9]+))[^\\\/])*)?[Ss](?<seasonnumber>[0-9]+)[][ ._-]*[Ee](?<epnumber>[0-9]+)([^\\/]*)$`),
			// foo.ep01, foo.EP_01
			plain(`[\._ -]()[Ee][Pp]_?([0-9]+)([^\\/]*)$`),
			// foo.E01., foo.e01.
			plain(`[^\\/]*?()\.?[Ee]([0-9]+)\.([^\\/]*)$`),
			byDate(`(?<year>[0-9]{4})[._ -](?<month>[0-9]{2})[._ -](?<day>[0-9]{2})`, "yyyy.MM.dd", "yyyy-MM-dd", "yyyy_MM_dd", "yyyy MM dd"),
			byDate(`(?<day>[0-9]{2})[._ -](?<month>[0-9]{2})[._ -](?<year>[0-9]{4})`, "dd.MM.yyyy", "dd-MM-yyyy", "dd_MM_yyyy", "dd MM yyyy"),
			// Not Kodi: tried before the next expression, which misreads
			// "Series Season X Episode X - Title.avi", "Series S03 E09.avi",
			// "s3 e9 - Title.avi".
			named(`.*[\\\/]((?<seriesname>[^\\/]+?)\s)?[Ss](?:eason)?\s*(?<seasonnumber>[0-9]+)\s+[Ee](?:pisode)?\s*(?<epnumber>[0-9]+).*$`),
			// Not Kodi: "Foo Bar 889". Names with an SxxEyy marker are left to
			// the Kodi expression, or digits of the title would be read as
			// episodes ("S01E01 1-23-45 [Bluray-1080p]").
			named(`.*[\\\/](?![Ee]pisode)(?![^\\\/]*[Ss][0-9]+[][ ._-]*[Ee][0-9]+)(?<seriesname>[\w\s]+?)\s(?<epnumber>[0-9]{1,4})(-(?<endingepnumber>[0-9]{2,4}))*[^\\\/x]*$`),
			plain(`[\\\/\._ \[\(-]([0-9]+)x([0-9]+(?:(?:[a-i]|\.[1-9])(?![0-9]))?)([^\\\/]*)$`),
			// Not Kodi: "[bar] Foo - 1 [baz]", a special case of the next
			// expression that keeps digits in the series name.
			named(`.*[\\\/]?.*?(\[.*?\])+.*?(?<seriesname>[-\w\s]+?)[\s_]*-[\s_]*(?<epnumber>[0-9]+).*$`),
			// "Name - 101.mkv", "Name - 101 [720p].mkv", "Name - 101 (2020).mkv":
			// absolute numbers after a hyphen, common in anime.
			named(`.*[\\\/](?<seriesname>[^\\\/]+?)[\s_]+-[\s_]+(?<epnumber>[0-9]+)[\s_]*(?:\[.*?\]|\(.*?\))*[\s_]*(?:\.\w+)?$`),
			// /server/anything_102.mp4,
			// /server/james.corden.2017.04.20.anne.hathaway.720p.hdtv.x264-crooks.mkv,
			// /server/anything_1996.11.14.mp4
			{
				Expression: `[\\/._ -](?<seriesname>(?![0-9]+[0-9][0-9])([^\\\/_])*)[\\\/._ -](?<seasonnumber>[0-9]+)(?<epnumber>[0-9][0-9](?:(?:[a-i]|\.[1-9])(?![0-9]))?)([._ -][^\\\/]*)$`,
				Optimistic: true,
				Named:      true,
			},
			plain(`[\/._ -]p(?:ar)?t[_. -]()([ivx]+|[0-9]+)([._ -][^\/]*)$`),
			// End of Kodi standard naming.

			// "Episode 16", "Episode 16 - Title"
			named(`[Ee]pisode (?<epnumber>[0-9]+)(-(?<endingepnumber>[0-9]+))?[^\\\/]*$`),
			named(`.*(\\|\/)[sS]?(?<seasonnumber>[0-9]+)[xX](?<epnumber>[0-9]+)[^\\\/]*$`),
			named(`.*(\\|\/)[sS](?<seasonnumber>[0-9]+)[x,X]?[eE](?<epnumber>[0-9]+)[^\\\/]*$`),
			named(`.*(\\|\/)(?<seriesname>((?![sS]?[0-9]{1,4}[xX][0-9]{1,3})[^\\\/])*)?([sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]+))[^\\\/]*$`),
			named(`.*(\\|\/)(?<seriesname>[^\\\/]*)[sS](?<seasonnumber>[0-9]{1,4})[xX\.]?[eE](?<epnumber>[0-9]+)[^\\\/]*$`),
			// "01.avi"
			optimistic(`.*[\\\/](?<epnumber>[0-9]+)(-(?<endingepnumber>[0-9]+))*\.\w+$`),
			// "1-12 episode title"
			plain(`([0-9]+)-([0-9]+)`),
			// "01 - blah.avi", "01-blah.avi"
			optimistic(`.*(\\|\/)(?<epnumber>[0-9]{1,3})(-(?<endingepnumber>[0-9]{2,3}))*\s?-\s?[^\\\/]*$`),
			// "01.blah.avi"
			optimistic(`.*(\\|\/)(?<epnumber>[0-9]{1,3})(-(?<endingepnumber>[0-9]{2,3}))*\.[^\\\/]+$`),
			// "blah - 01.avi", "blah 2 - 01.avi", "blah - 01 blah.avi",
			// "blah 2 - 01 blah", "blah - 01 - blah.avi", "blah 2 - 01 - blah"
			optimistic(`.*[\\\/][^\\\/]* - (?<epnumber>[0-9]{1,3})(-(?<endingepnumber>[0-9]{2,3}))*[^\\\/]*$`),
			// "01 episode title.avi"
			optimistic(`[Ss]eason[\._ ](?<seasonnumber>[0-9]+)[\\\/](?<epnumber>[0-9]{1,3})([^\\\/]*)$`),
			// Series and season only: "the show/season 1", "the show/s01"
			named(`(.*(\\|\/))*(?<seriesname>.+)\/[Ss](eason)?[\. _\-]*(?<seasonnumber>[0-9]+)`),
			// Series and season only: "the show S01", "the show season 1"
			named(`(.*(\\|\/))*(?<seriesname>.+)[\. _\-]+[sS](eason)?[\. _\-]*(?<seasonnumber>[0-9]+)`),
			// Anime: "[Group][Series Name][21][1080p][FLAC][HASH]",
			// "[Group] Series Name [04][BDRIP]"
			named(`(?:\[(?:[^\]]+)\]\s*)?(?<seriesname>\[[^\]]+\]|[^[\]]+)\s*\[(?<epnumber>[0-9]+)\]`),
		},
		VideoExtraRules: []ExtraRule{
			{core.ExtraTrailer, ExtraDirectoryName, "trailers", MediaVideo},
			{core.ExtraThemeVideo, ExtraDirectoryName, "backdrops", MediaVideo},
			{core.ExtraThemeSong, ExtraDirectoryName, "theme-music", MediaAudio},
			{core.ExtraBehindTheScene, ExtraDirectoryName, "behind the scenes", MediaVideo},
			{core.ExtraDeletedScene, ExtraDirectoryName, "deleted scenes", MediaVideo},
			{core.ExtraInterview, ExtraDirectoryName, "interviews", MediaVideo},
			{core.ExtraScene, ExtraDirectoryName, "scenes", MediaVideo},
			{core.ExtraSample, ExtraDirectoryName, "samples", MediaVideo},
			{core.ExtraSample, ExtraDirectoryName, "sample", MediaVideo},
			{core.ExtraShort, ExtraDirectoryName, "shorts", MediaVideo},
			{core.ExtraFeaturette, ExtraDirectoryName, "featurettes", MediaVideo},
			{core.ExtraOther, ExtraDirectoryName, "extras", MediaVideo},
			{core.ExtraOther, ExtraDirectoryName, "extra", MediaVideo},
			{core.ExtraOther, ExtraDirectoryName, "other", MediaVideo},
			{core.ExtraClip, ExtraDirectoryName, "clips", MediaVideo},
			{core.ExtraTrailer, ExtraFilename, "trailer", MediaVideo},
			{core.ExtraSample, ExtraFilename, "sample", MediaVideo},
			{core.ExtraThemeSong, ExtraFilename, "theme", MediaAudio},
			{core.ExtraTrailer, ExtraSuffix, "-trailer", MediaVideo},
			{core.ExtraTrailer, ExtraSuffix, ".trailer", MediaVideo},
			{core.ExtraTrailer, ExtraSuffix, "_trailer", MediaVideo},
			{core.ExtraTrailer, ExtraSuffix, "- trailer", MediaVideo},
			{core.ExtraSample, ExtraSuffix, "-sample", MediaVideo},
			{core.ExtraSample, ExtraSuffix, ".sample", MediaVideo},
			{core.ExtraSample, ExtraSuffix, "_sample", MediaVideo},
			{core.ExtraSample, ExtraSuffix, "- sample", MediaVideo},
			{core.ExtraScene, ExtraSuffix, "-scene", MediaVideo},
			{core.ExtraClip, ExtraSuffix, "-clip", MediaVideo},
			{core.ExtraInterview, ExtraSuffix, "-interview", MediaVideo},
			{core.ExtraBehindTheScene, ExtraSuffix, "-behindthescenes", MediaVideo},
			{core.ExtraDeletedScene, ExtraSuffix, "-deleted", MediaVideo},
			{core.ExtraDeletedScene, ExtraSuffix, "-deletedscene", MediaVideo},
			{core.ExtraFeaturette, ExtraSuffix, "-featurette", MediaVideo},
			{core.ExtraShort, ExtraSuffix, "-short", MediaVideo},
			{core.ExtraOther, ExtraSuffix, "-extra", MediaVideo},
			{core.ExtraOther, ExtraSuffix, "-other", MediaVideo},
		},
		Format3DRules: []Format3DRule{
			// Kodi rules.
			{Token: "hsbs", PrecedingToken: "3d"},
			{Token: "sbs", PrecedingToken: "3d"},
			{Token: "htab", PrecedingToken: "3d"},
			{Token: "tab", PrecedingToken: "3d"},
			// Media Browser rules.
			{Token: "fsbs"},
			{Token: "hsbs"},
			{Token: "sbs"},
			{Token: "ftab"},
			{Token: "htab"},
			{Token: "tab"},
			{Token: "sbs3d"},
			{Token: "mvc"},
		},
		AudioBookPartsExpressions: []string{
			// Chapters, like "CH 01".
			`ch(?:apter)?[\s_-]?(?<chapter>[0-9]+)`,
			// Parts, like "Part 02".
			`p(?:ar)?t[\s_-]?(?<part>[0-9]+)`,
			// A chapter often starts the file name.
			`^(?<chapter>[0-9]+)`,
			// A part often ends it.
			`(?<!ch(?:apter) )(?<part>[0-9]+)$`,
			// "0001_005" (chapter_part).
			`(?<chapter>[0-9]+)_(?<part>[0-9]+)`,
			// Audiobooks ripped from CDs are numbered by disc.
			`dis(?:c|k)[\s_-]?(?<chapter>[0-9]+)`,
		},
		AudioBookNamesExpressions: []string{
			// A year in brackets after the name: "Batman (2020)".
			`^(?<name>.+?)\s*\(\s*(?<year>[0-9]{4})\s*\)\s*$`,
			`^\s*(?<name>[^ ].*?)\s*$`,
		},
	}
	for _, e := range []string{
		`.*(\\|\/)[sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3})((-| - )[0-9]{1,4}[eExX](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)[sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3})((-| - )[0-9]{1,4}[xX][eE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)[sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3})((-| - )?[xXeE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)[sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3})(-[xE]?[eE]?(?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>((?![sS]?[0-9]{1,4}[xX][0-9]{1,3})[^\\\/])*)?([sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3}))((-| - )[0-9]{1,4}[xXeE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>((?![sS]?[0-9]{1,4}[xX][0-9]{1,3})[^\\\/])*)?([sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3}))((-| - )[0-9]{1,4}[xX][eE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>((?![sS]?[0-9]{1,4}[xX][0-9]{1,3})[^\\\/])*)?([sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3}))((-| - )?[xXeE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>((?![sS]?[0-9]{1,4}[xX][0-9]{1,3})[^\\\/])*)?([sS]?(?<seasonnumber>[0-9]{1,4})[xX](?<epnumber>[0-9]{1,3}))(-[xX]?[eE]?(?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>[^\\\/]*)[sS](?<seasonnumber>[0-9]{1,4})[xX\.]?[eE](?<epnumber>[0-9]{1,3})((-| - )?[xXeE](?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
		`.*(\\|\/)(?<seriesname>[^\\\/]*)[sS](?<seasonnumber>[0-9]{1,4})[xX\.]?[eE](?<epnumber>[0-9]{1,3})(-[xX]?[eE]?(?<endingepnumber>[0-9]{1,3}))+[^\\\/]*$`,
	} {
		o.MultipleEpisodeExpressions = append(o.MultipleEpisodeExpressions, named(e))
	}
	return o
}

// plain returns an expression capturing season and episode as groups 1 and 2.
func plain(expr string) EpisodeExpression {
	return EpisodeExpression{Expression: expr, SupportsAbsoluteNumbers: true}
}

// named returns an expression with named groups.
func named(expr string) EpisodeExpression {
	return EpisodeExpression{Expression: expr, Named: true, SupportsAbsoluteNumbers: true}
}

// optimistic returns a named expression that is only a guess.
func optimistic(expr string) EpisodeExpression {
	return EpisodeExpression{Expression: expr, Named: true, Optimistic: true, SupportsAbsoluteNumbers: true}
}

// byDate returns an expression capturing an air date in one of formats.
func byDate(expr string, formats ...string) EpisodeExpression {
	return EpisodeExpression{Expression: expr, ByDate: true, DateFormats: formats, SupportsAbsoluteNumbers: true}
}

package release

// Radarr's movie title expressions, ported verbatim from its Parser.

const editionPattern = `\(?\b(?<edition>(((Recut.|Extended.|Ultimate.)?(Director.?s|Collector.?s|Theatrical|Ultimate|Extended|Despecialized|(Special|Rouge|Final|Assembly|Imperial|Diamond|Signature|Hunter|Rekall)(?=(.(Cut|Edition|Version)))|\d{2,3}(th)?.Anniversary)(?:.(Cut|Edition|Version))?(.(Extended|Uncensored|Remastered|Unrated|Uncut|Open.?Matte|IMAX|Fan.?Edit))?|((Uncensored|Remastered|Unrated|Uncut|Open?.Matte|IMAX|Fan.?Edit|Restored|((2|3|4)in1))))))\b\)?`

// movieTitleRegexes are tried in order; the first that matches and yields a
// result wins.
var movieTitleRegexes = []*regex{
	// Anime [Subgroup] with year explicitly in parentheses, e.g. [Sub] Title 1993 (1993) [VHS]
	// This must come before the generic anime+year regex to correctly handle titles that contain a year as part of the name
	re(`^(?:\[(?<subgroup>.+?)\][-_. ]?)(?<title>(?![(\[]).+?)\s*\((?<year>(1(8|9)|20)\d{2})\).*?(?<hash>\[\w{8}\])?(?:$|\.)`, ignoreCase),
	// Anime [Subgroup] and Year
	re(`^(?:\[(?<subgroup>.+?)\][-_. ]?)(?<title>(?![(\[]).+?)?(?:(?:[-_\W](?<![)\[!]))*(?<year>(1(8|9)|20)\d{2}(?!p|i|x|\d+|\]|\W\d+)))+.*?(?<hash>\[\w{8}\])?(?:$|\.)`, ignoreCase),
	// Anime [Subgroup] no year, versioned title, hash
	re(`^(?:\[(?<subgroup>.+?)\][-_. ]?)(?<title>(?![(\[]).+?)((v)(?:\d{1,2})(?:([-_. ])))(\[.*)?(?:[\[(][^])])?.*?(?<hash>\[\w{8}\])(?:$|\.)`, ignoreCase),
	// Anime [Subgroup] no year, info in double sets of brackets, hash
	re(`^(?:\[(?<subgroup>.+?)\][-_. ]?)(?<title>(?![(\[]).+?)(\[.*).*?(?<hash>\[\w{8}\])(?:$|\.)`, ignoreCase),
	// Anime [Subgroup] no year, info in parentheses or brackets, hash
	re(`^(?:\[(?<subgroup>.+?)\][-_. ]?)(?<title>(?![(\[]).+)(?:[\[(][^])]).*?(?<hash>\[\w{8}\])(?:$|\.)`, ignoreCase),
	// Some german or french tracker formats (missing year, ...) (Only applies to german and TrueFrench releases) - see ParserFixture for examples and tests - french removed as it broke all movies w/ french titles
	re(`^(?<title>(?![(\[]).+?)((\W|_))(`+editionPattern+`.{1,3})?(?:(?<!(19|20)\d{2}.*?)(?<!(?:Good|The)[_ .-])(German|TrueFrench))(.+?)(?=((19|20)\d{2}|$))(?<year>(19|20)\d{2}(?!p|i|\d+|\]|\W\d+))?(\W+|_|$)(?!\\)`, ignoreCase),
	// Special, Despecialized, etc. Edition Movies, e.g: Mission.Impossible.3.Special.Edition.2011
	re(`^(?<title>(?![(\[]).+?)?(?:(?:[-_\W](?<![)\[!]))*`+editionPattern+`.{1,3}(?<year>(1(8|9)|20)\d{2}(?!p|i|\d+|\]|\W\d+)))+(\W+|_|$)(?!\\)`, ignoreCase),
	// Normal movie format, e.g: Mission.Impossible.3.2011
	re(`^(?<title>(?![(\[]).+?)?(?:(?:[-_\W](?<![)\[!]))*(?<year>(1(8|9)|20)\d{2}(?!p|i|(1(8|9)|20)\d{2}|\]|\W(1(8|9)|20)\d{2})))+(\W+|_|$)(?!\\)`, ignoreCase),
	// PassThePopcorn Torrent names: Star.Wars[PassThePopcorn]
	re(`^(?<title>.+?)?(?:(?:[-_\W](?<![()\[!]))*(?<year>(\[\w *\])))+(\W+|_|$)(?!\\)`, ignoreCase),
	// That did not work? Maybe some tool uses [] for years. Who would do that?
	re(`^(?<title>(?![(\[]).+?)?(?:(?:[-_\W](?<![)!]))*(?<year>(1(8|9)|20)\d{2}(?!p|i|\d+|\W\d+)))+(\W+|_|$)(?!\\)`, ignoreCase),
	// As a last resort for movies that have ( or [ in their title.
	re(`^(?<title>.+?)?(?:(?:[-_\W](?<![)\[!]))*(?<year>(1(8|9)|20)\d{2}(?!p|i|\d+|\]|\W\d+)))+(\W+|_|$)(?!\\)`, ignoreCase),
}

// movieFolderRegexes are also tried for folder names.
var movieFolderRegexes = []*regex{
	// When year comes first.
	re(`^(?:(?:[-_\W](?<![)!]))*(?<year>(19|20)\d{2}(?!p|i|\d+|\W\d+)))+(\W+|_|$)(?<title>.+?)?$`),
}

var (
	reportEditionRegex = re(`^.+?`+editionPattern, ignoreCase)
	hardcodedSubsRegex = re(`\b((?<hcsub>(\w+(?<!SOFT|MULTI|HORRIBLE)SUBS?))|(?<hc>(HC|SUBBED)))\b`, ignoreCase, patternWS)

	movieRejectHashedRegexes = []*regex{
		// Generic match for md5 and mixed-case hashes.
		re(`^[0-9a-zA-Z]{32}`),
		// Generic match for shorter lower-case hashes.
		re(`^[a-z0-9]{24}$`),
		// Format seen on some NZBGeek releases
		// Be very strict with these coz they are very close to the valid 101 ep numbering.
		re(`^[A-Z]{11}\d{3}$`),
		re(`^[a-z]{12}\d{3}$`),
		// Backup filename (Unknown origins)
		re(`^Backup_\d{5,}S\d{2}-\d{2}$`),
		// 123 - Started appearing December 2014
		re(`^123$`),
		// abc - Started appearing January 2015
		re(`^abc$`, ignoreCase),
		// abc - Started appearing 2020
		re(`^abc[-_. ]xyz`, ignoreCase),
		// b00bs - Started appearing January 2015
		re(`^b00bs$`, ignoreCase),
	}

	movieReversedTitleRegex        = re(`(?:^|[-._ ])(p027|p0801)[-._ ]`)
	alternativeTitleRegex          = re(`[ ]+(?:AKA|\/)[ ]+`, ignoreCase)
	bracketedAlternativeTitleRegex = re(`(.*) \([ ]*AKA[ ]+(.*)\)`, ignoreCase)
	normalizeAlternativeTitleRegex = re(`[ ]+(?:A\.K\.A\.)[ ]+`, ignoreCase)
	movieNormalizeRegex            = re(`((?:\b|_)(?<!^|[^a-zA-Z0-9_']\w[^a-zA-Z0-9_'])([aà](?!$|[^a-zA-Z0-9_']\w[^a-zA-Z0-9_'])|an|the|and|or|of)(?!$)(?:\b|_))|\W|_`, ignoreCase)
	reportImdbIDRegex              = re(`(?<imdbid>tt\d{7,8})`, ignoreCase)
	reportTmdbIDRegex              = re(`tmdb(id)?-(?<tmdbid>\d+)`, ignoreCase)
	movieSimpleTitleRegex          = rr(`(?:(480|540|576|720|1080|2160)[ip]|[xh][\W_]?26[45]|DD\W?5\W1|[<>?*]|848x480|1280x720|1920x1080|3840x2160|4096x2160|(8|10)b(it)?|10-bit)\s*?(?![a-b0-9])`, ``, ignoreCase)
	simpleReleaseTitleRegex        = re(`\s*(?:[<>?*|])`, ignoreCase)
	specialCharRegex               = re(`(\&|\:|\\|\/)+`)
	specialEpisodeWordRegex        = re(`\b(part|special|edition|christmas)\b\s?`, ignoreCase)
)

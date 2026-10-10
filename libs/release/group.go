package release

import (
	"strconv"
	"strings"
)

type groupRules struct {
	preSubstitutions []regexReplace
	clean            regexReplace
	// exceptions are the expressions for groups that do not follow the
	// "-Group" convention, in the app's order.
	exceptions []*regex
	group      *regex
}

var groupRuleSets = map[Kind]groupRules{
	Series: {
		preSubstitutions: seriesPreSubstitutions,
		clean:            seriesCleanReleaseGroup,
		exceptions:       []*regex{seriesExceptionReleaseGroup, seriesExceptionReleaseGroupExact},
		group:            seriesReleaseGroupRegex,
	},
	Movie: {
		clean: rr(`(-(RP|1|NZBGeek|Obfuscated|Obfuscation|Scrambled|sample|Pre|postbot|xpost|Rakuv[a-z0-9]*|WhiteRev|BUYMORE|AsRequested|AlternativeToRequested|GEROV|Z0iDS3N|Chamele0n|4P|4Planet|AlteZachen|RePACKPOST))+$`, ``, ignoreCase),
		exceptions: []*regex{
			re(`\b(?<releasegroup>KRaLiMaRKo|E\.N\.D|D\-Z0N3|Koten_Gars|BluDragon|ZØNEHD|HQMUX|VARYG|YIFY|YTS(.(MX|LT|AG))?|TMd|Eml HDTeam|LMain|DarQ|BEN THE MEN|TAoE|QxR|126811)\b`, ignoreCase),
			re(`(?<=[._ \[])(?<releasegroup>(Silence|afm72|Panda|Ghost|MONOLITH|Tigole|Joy|ImE|UTR|t3nzin|Anime Time|Project Angel|Hakata Ramen|HONE|GiLG|Vyndros|SEV|Garshasp|Kappa|Natty|RCVR|SAMPA|YOGI|r00t|EDGE2020|RZeroX|FreetheFish|Anna|Bandi|Qman|theincognito|HDO|DusIctv|DHD|CtrlHD|-ZR-|ADC|XZVN|RH|Kametsu|Celdra)(?=\]|\)))`, ignoreCase),
		},
		group: re(`-(?<releasegroup>[a-z0-9]+(?<part2>-[a-z0-9]+)?(?!.+?(?:480p|576p|720p|1080p|2160p)))(?<!(?:WEB-(DL|Rip)|Blu-Ray|480p|576p|720p|1080p|2160p|DTS-HD|DTS-X|DTS-MA|DTS-ES|-ES|-EN|-CAT|-ENG|-JAP|-GER|-FRA|-FRE|-ITA|-HDRip|\d{1,2}-bit|[ ._]\d{4}-\d{2}|-\d{2}|tmdb(id)?-(?<tmdbid>\d+)|(?<imdbid>tt\d{7,8}))(?:\k<part2>)?)(?:\b|[-._ ]|$)|[-._ ]\[(?<releasegroup>[a-z0-9]+)\]$`, ignoreCase),
	},
}

// ParseReleaseGroup returns the release group of a title, or "".
func ParseReleaseGroup(title string, kind Kind) string {
	rules := groupRuleSets[kind]
	title = RemoveFileExtension(strings.TrimSpace(title), kind)
	for _, r := range rules.preSubstitutions {
		if r.tryReplace(&title) {
			break
		}
	}
	title = websitePrefixRegex.apply(title)
	title = cleanTorrentSuffixRegex.apply(title)
	if m := animeReleaseGroupRegex.match(title); m != nil {
		return m.value("subgroup")
	}
	title = rules.clean.apply(title)
	for _, r := range rules.exceptions {
		if matches := r.matches(title); len(matches) > 0 {
			return matches[len(matches)-1].value("releasegroup")
		}
	}
	matches := rules.group.matches(title)
	if len(matches) == 0 {
		return ""
	}
	group := matches[len(matches)-1].value("releasegroup")
	if _, err := strconv.Atoi(group); err == nil || invalidReleaseGroupRegex.isMatch(group) {
		return ""
	}
	return group
}

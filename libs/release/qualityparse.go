package release

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// qualityRules are the expressions that differ between Radarr (Movie) and
// Sonarr (Series).
type qualityRules struct {
	source, resolution, codec, remux, germanRemux, brDisk, version *regex
}

var qualityRuleSets = map[Kind]qualityRules{
	Movie: {
		source: re(`\b(?:
                    (?<bluray>M?Blu[-_. ]?Ray|HD[-_. ]?DVD|BD(?!$)|UHD2?BD|BDISO|BDMux|BD25|BD50|BR[-_. ]?DISK)|
                    (?<webdl>WEB[-_. ]?DL(?:mux)?|AmazonHD|AmazonSD|iTunesHD|MaxdomeHD|NetflixU?HD|WebHD|HBOMaxHD|DisneyHD|[. ]WEB[. ](?:[xh][ .]?26[45]|AVC|HEVC|DDP?5[. ]1)|[. ](?-i:WEB)$|(?:\d{3,4}0p)[-. ](?:Hybrid[-_. ]?)?WEB[-. ]|[-. ]WEB[-. ]\d{3,4}0p|\b\s\/\sWEB\s\/\s\b|(?:AMZN|NF|DP)[. -]WEB[. -](?!Rip))|
                    (?<webrip>WebRip|Web-Rip|WEBMux)|
                    (?<hdtv>HDTV)|
                    (?<bdrip>BDRip|BDLight|HD[-_. ]?DVDRip|UHDBDRip)|
                    (?<brrip>BRRip)|
                    (?<dvdr>\d?x?M?DVD-?[R59])|
                    (?<dvd>DVD(?!-R)|DVDRip|xvidvd)|
                    (?<dsr>WS[-_. ]DSR|DSR)|
                    (?<regional>R[0-9]{1}|REGIONAL)|
                    (?<scr>SCR|SCREENER|DVDSCR|DVDSCREENER)|
                    (?<ts>TS[-_. ]|TELESYNCH?|HD-TS|HDTS|PDVD|TSRip|HDTSRip)|
                    (?<tc>TC|TELECINE|HD-TC|HDTC)|
                    (?<cam>CAMRIP|(?:NEW)?CAM|HD-?CAM(?:Rip)?|HQCAM)|
                    (?<wp>WORKPRINT|WP)|
                    (?<pdtv>PDTV)|
                    (?<sdtv>SDTV)|
                    (?<tvrip>TVRip)
                    )(?:\b|$|[ .])`, ignoreCase, patternWS),
		resolution:  re(`\b(?:(?<R360p>360p)|(?<R480p>480p|480i|640x480|848x480)|(?<R540p>540p)|(?<R576p>576p)|(?<R720p>720p|1280x720|960p)|(?<R1080p>1080p|1920x1080|1440p|FHD|1080i|4kto1080p)|(?<R2160p>2160p|3840x2160|4k[-_. ](?:UHD|HEVC|BD|H\.?265)|(?:UHD|HEVC|BD|H\.?265)[-_. ]4k))\b`, ignoreCase),
		codec:       re(`\b(?:(?<x264>x264)|(?<h264>h264)|(?<xvidhd>XvidHD)|(?<xvid>X-?vid)|(?<divx>divx))\b`, ignoreCase),
		remux:       re(`(?:[_. \[]|\d{4}p-|\bHybrid-)(?<remux>(?:(BD|UHD)[-_. ]?)?Remux)\b|(?<remux>(?:(BD|UHD)[-_. ]?)?Remux[_. ]\d{4}p)`, ignoreCase),
		germanRemux: re(`((?<=\d{4}).*German.*([DM]L)?)(?=.*\b(AVC|HEVC|VC[_. -]?1|MVC|MPEG[_. -]?2))(?=.*Blu-?ray)`, ignoreCase),
		brDisk:      re(`^(?!.*\b((?<!HD[._ -]|HD)DVD|BDRip|720p|MKV|XviD|WMV|d3g|(BD)?REMUX|^(?=.*1080p)(?=.*HEVC)|[xh][-_. ]?26[45]|German.*[DM]L|((?<=\d{4}).*German.*([DM]L)?)(?=.*\b(AVC|HEVC|VC[-_. ]?1|MVC|MPEG[-_. ]?2)\b))\b)(((?=.*\b(Blu[-_. ]?ray|BD|HD[-_. ]?DVD)\b)(?=.*\b(AVC|HEVC|VC[-_. ]?1|MVC|MPEG[-_. ]?2|BDMV|ISO)\b))|^((?=.*\b(((?=.*\b((.*_)?COMPLETE.*|Dis[ck])\b)(?=.*(Blu[-_. ]?ray|HD[-_. ]?DVD)))|3D[-_. ]?BD|BR[-_. ]?DISK|Full[-_. ]?Blu[-_. ]?ray|^((?=.*((BD|UHD)[-_. ]?(25|50|66|100|ISO)))))))).*`, ignoreCase),
		version:     re(`\d[-._ ]?v(?<version>\d)[-._ ]|\[v(?<version>\d)\]|repack(?<version>\d)|rerip(?<version>\d)`, ignoreCase),
	},
	Series: {
		source: re(`\b(?:
                    (?<bluray>BluRay|Blu-Ray|HD-?DVD|BDMux|BD(?!$))|
                    (?<webdl>WEB[-_. ]DL(?:mux)?|WEBDL|AmazonHD|AmazonSD|iTunesHD|MaxdomeHD|NetflixU?HD|WebHD|HBOMaxHD|DisneyHD|[. ]WEB[. ](?:[xh][ .]?26[45]|AVC|HEVC|DDP?5[. ]1)|[. ](?-i:WEB)$|(?:720|1080|2160)p[-. ]WEB[-. ]|[-. ]WEB[-. ](?:720|1080|2160)p|\b\s\/\sWEB\s\/\s\b|(?:AMZN|NF|DP)[. -]WEB[. -](?!Rip))|
                    (?<webrip>WebRip|Web-Rip|WEBMux)|
                    (?<hdtv>HDTV)|
                    (?<bdrip>BDRip|BDLight)|
                    (?<brrip>BRRip)|
                    (?<dvd>DVD|DVDRip|NTSC|PAL|xvidvd)|
                    (?<dsr>WS[-_. ]DSR|DSR)|
                    (?<pdtv>PDTV)|
                    (?<sdtv>SDTV)|
                    (?<tvrip>TVRip)
                    )(?:\b|$|[ .])`, ignoreCase, patternWS),
		resolution: re(`\b(?:(?<R360p>360p)|(?<R480p>480p|480i|640x480|848x480)|(?<R540p>540p)|(?<R576p>576p)|(?<R720p>720p|1280x720|960p)|(?<R1080p>1080p|1920x1080|1440p|FHD|1080i|4kto1080p)|(?<R2160p>2160p|3840x2160|4k[-_. ](?:UHD|HEVC|BD|H265)|(?:UHD|HEVC|BD|H265)[-_. ]4k))\b`, ignoreCase),
		codec:      re(`\b(?:(?<x264>x264)|(?<h264>h264)|(?<xvidhd>XvidHD)|(?<xvid>Xvid)|(?<divx>divx))\b`, ignoreCase),
		remux:      re(`(?:[_. ]|\d{4}p-|\bHybrid-)(?<remux>(?:(BD|UHD)[-_. ]?)?Remux)\b|(?<remux>(?:(BD|UHD)[-_. ]?)?Remux[_. ]\d{4}p)`, ignoreCase),
		version:    re(`\d[-._ ]?v(?<version>\d)[-._ ]|\[v(?<version>\d)\]|repack(?<version>\d)|rerip(?<version>\d)|(?:480|576|720|1080|2160)p[._ ]v(?<version>\d)`, ignoreCase),
	},
}

var (
	rawHDRegex                 = re(`\b(?<rawhd>RawHD|Raw[-_. ]HD)\b`, ignoreCase)
	mpeg2Regex                 = re(`\b(?<mpeg2>MPEG[-_. ]?2)\b`)
	properRegex                = re(`\b(?<proper>proper)\b`, ignoreCase)
	repackRegex                = re(`\b(?<repack>repack\d?|rerip\d?)\b`, ignoreCase)
	realRegex                  = re(`\b(?<real>REAL)\b`)
	alternativeResolutionRegex = re(`\b(?<R2160p>UHD)\b|(?<R2160p>\[4K\])`, ignoreCase)
	otherSourceRegex           = re(`(?<hdtv>HD[-_. ]TV)|(?<sdtv>SD[-_. ]TV)`, ignoreCase)
	animeBlurayRegex           = re(`bd(?:720|1080|2160)|(?<=[-_. (\[])bd(?=[-_. )\]])`, ignoreCase)
	animeWebDLRegex            = re(`\[WEB\]|[\[\(]WEB[ .]`, ignoreCase)
	highDefPdtvRegex           = re(`hr[-_. ]ws`, ignoreCase)
)

// ParseQuality parses the quality of a release or file name, falling back to
// the file extension.
func ParseQuality(name string, kind Kind) ParsedQuality {
	name = strings.TrimSpace(name)
	if name == "" {
		return newParsedQuality()
	}
	result := ParseQualityName(name, kind)
	if result.Quality == Unknown && !strings.ContainsRune(name, 0) {
		result.Quality = QualityForExtension(pathExtension(name), kind)
		result.SourceDetection = DetectedExtension
		result.ResolutionDetection = DetectedExtension
		if kind == Movie {
			result.ModifierDetection = DetectedExtension
		}
	}
	return result
}

// ParseQualityName parses the quality of a release or file name without
// looking at the file extension, except to pick the source of a name that
// only states a resolution.
func ParseQualityName(name string, kind Kind) ParsedQuality {
	rules := qualityRuleSets[kind]
	movie := kind == Movie
	normalized := strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	result := parseQualityModifiers(name, normalized, rules)

	sourceMatches := rules.source.matches(normalized)
	var src *match
	if len(sourceMatches) > 0 {
		src = sourceMatches[len(sourceMatches)-1]
	}
	resolution := parseResolution(normalized, rules)
	codec := rules.codec.match(normalized)
	codecHas := func(g string) bool { return codec != nil && codec.has(g) }
	remux := rules.remux.isMatch(normalized) || movie && rules.germanRemux.isMatch(normalized)
	brDisk := movie && rules.brDisk.isMatch(normalized)
	set := func(q Quality) ParsedQuality { result.Quality = q; return result }
	remuxOr := func(r, q Quality) ParsedQuality {
		if remux {
			return set(r)
		}
		return set(q)
	}
	sd := resolution == 360 || resolution == 480 || resolution == 540 || resolution == 576

	if rawHDRegex.isMatch(normalized) && !brDisk {
		result.SourceDetection = DetectedName
		result.ResolutionDetection = DetectedName
		return set(RawHD)
	}
	if resolution != 0 {
		result.ResolutionDetection = DetectedName
	}

	if src != nil {
		result.SourceDetection = DetectedName
		switch {
		case src.has("bluray"):
			switch {
			case brDisk:
				return set(BRDisk)
			case codecHas("xvid") || codecHas("divx"):
				return set(Bluray480p)
			case resolution == 2160:
				return remuxOr(Remux2160p, Bluray2160p)
			case resolution == 1080:
				return remuxOr(Remux1080p, Bluray1080p)
			case resolution == 720:
				return set(Bluray720p)
			case resolution == 576:
				return set(Bluray576p)
			case resolution == 360 || resolution == 480 || resolution == 540:
				return set(Bluray480p)
			case remux:
				// Treat a remux without a resolution as 1080p.
				return set(Remux1080p)
			}
			return set(Bluray720p)

		case src.has("webdl"):
			switch {
			case resolution == 2160:
				return set(WEBDL2160p)
			case resolution == 1080:
				return set(WEBDL1080p)
			case resolution == 720, strings.Contains(name, "[WEBDL]"):
				return set(WEBDL720p)
			}
			return set(WEBDL480p)

		case src.has("webrip"):
			switch resolution {
			case 2160:
				return set(WEBRip2160p)
			case 1080:
				return set(WEBRip1080p)
			case 720:
				return set(WEBRip720p)
			}
			return set(WEBRip480p)

		case movie && src.has("scr"):
			return set(DVDSCR)
		case movie && src.has("cam"):
			return set(Cam)
		case movie && src.has("ts"):
			result.Resolution = resolution
			return set(Telesync)
		case movie && src.has("tc"):
			return set(Telecine)
		case movie && src.has("wp"):
			return set(Workprint)
		case movie && src.has("regional"):
			return set(Regional)

		case src.has("hdtv"):
			switch {
			case mpeg2Regex.isMatch(normalized):
				return set(RawHD)
			case resolution == 2160:
				return set(HDTV2160p)
			case resolution == 1080:
				return set(HDTV1080p)
			case resolution == 720, strings.Contains(name, "[HDTV]"):
				return set(HDTV720p)
			}
			return set(SDTV)

		case src.has("bdrip"), src.has("brrip"):
			switch {
			case resolution == 720:
				return set(Bluray720p)
			case resolution == 1080:
				return set(Bluray1080p)
			case resolution == 2160:
				return set(Bluray2160p)
			case resolution == 576 && movie:
				return set(Bluray576p)
			}
			return set(Bluray480p)

		case movie && src.has("dvdr"):
			return set(DVDR)
		case src.has("dvd"):
			return set(DVD)

		case src.has("pdtv"), src.has("sdtv"), src.has("dsr"), src.has("tvrip"):
			switch {
			case resolution == 1080 || containsFold(normalized, "1080p"):
				return set(HDTV1080p)
			case resolution == 720 || containsFold(normalized, "720p"):
				return set(HDTV720p)
			case highDefPdtvRegex.isMatch(normalized):
				result.ResolutionDetection = DetectedName
				return set(HDTV720p)
			}
			return set(SDTV)
		}
	}

	if src == nil && remux && resolution != 0 {
		result.SourceDetection = DetectedUnknown
		switch resolution {
		case 480:
			return set(Bluray480p)
		case 720:
			return set(Bluray720p)
		case 2160:
			return set(Remux2160p)
		case 1080:
			return set(Remux1080p)
		}
	}

	if animeBlurayRegex.isMatch(normalized) {
		result.SourceDetection = DetectedName
		switch {
		case sd || containsFold(normalized, "480p"):
			result.ResolutionDetection = DetectedName
			return set(DVD)
		case resolution == 1080 || containsFold(normalized, "1080p"):
			result.ResolutionDetection = DetectedName
			return remuxOr(Remux1080p, Bluray1080p)
		case resolution == 2160 || containsFold(normalized, "2160p"):
			result.ResolutionDetection = DetectedName
			return remuxOr(Remux2160p, Bluray2160p)
		case remux && resolution != 720:
			return set(Remux1080p)
		}
		return set(Bluray720p)
	}

	if animeWebDLRegex.isMatch(normalized) {
		result.SourceDetection = DetectedName
		switch {
		case sd || containsFold(normalized, "480p"):
			result.ResolutionDetection = DetectedName
			return set(WEBDL480p)
		case resolution == 1080 || containsFold(normalized, "1080p"):
			result.ResolutionDetection = DetectedName
			return set(WEBDL1080p)
		case resolution == 2160 || containsFold(normalized, "2160p"):
			result.ResolutionDetection = DetectedName
			return set(WEBDL2160p)
		}
		return set(WEBDL720p)
	}

	if resolution != 0 {
		// The resolution alone: take the source from a remux tag or the
		// file extension.
		hint, known := Unknown, false
		if remux {
			result.SourceDetection = DetectedName
			hint, known = Remux1080p, true
		} else if q := QualityForExtension(pathExtension(name), kind); q != Unknown {
			result.SourceDetection = DetectedExtension
			hint, known = q, true
		}
		result.ResolutionDetection = DetectedName
		target := resolution
		if sd {
			target = 480
		}
		if !known {
			return set(map[int]Quality{2160: HDTV2160p, 1080: HDTV1080p, 720: HDTV720p, 480: SDTV}[target])
		}
		return set(findBySourceAndResolution(kind, hint, target, remux))
	}

	if codecHas("x264") {
		return set(SDTV)
	}

	if movie && containsFold(normalized, "848x480") || !movie && strings.Contains(normalized, "848x480") {
		result.ResolutionDetection = DetectedName
		switch {
		case strings.Contains(normalized, "dvd"):
			result.SourceDetection = DetectedName
			return set(DVD)
		case containsFold(normalized, "bluray"):
			result.SourceDetection = DetectedName
			return set(Bluray480p)
		}
		return set(SDTV)
	}

	for _, r := range []struct {
		res           string
		bluray, else_ Quality
	}{{"1280x720", Bluray720p, HDTV720p}, {"1920x1080", Bluray1080p, HDTV1080p}} {
		if containsFold(normalized, r.res) {
			result.ResolutionDetection = DetectedName
			if containsFold(normalized, "bluray") {
				result.SourceDetection = DetectedName
				return set(r.bluray)
			}
			return set(r.else_)
		}
	}

	for _, r := range []struct {
		token string
		q     Quality
	}{{"bluray720p", Bluray720p}, {"bluray1080p", Bluray1080p}, {"bluray2160p", Bluray2160p}} {
		if containsFold(normalized, r.token) {
			result.SourceDetection = DetectedName
			result.ResolutionDetection = DetectedName
			return set(r.q)
		}
	}

	if m := otherSourceRegex.match(normalized); m != nil {
		switch {
		case m.has("sdtv"):
			result.SourceDetection = DetectedName
			return set(SDTV)
		case m.has("hdtv"):
			result.SourceDetection = DetectedName
			return set(HDTV720p)
		}
	}
	return result
}

func parseResolution(name string, rules qualityRules) int {
	m := rules.resolution.match(name)
	alt := alternativeResolutionRegex.match(name)
	if m == nil && alt == nil {
		return 0
	}
	if m != nil {
		for _, r := range []struct {
			group string
			res   int
		}{{"R360p", 360}, {"R480p", 480}, {"R540p", 540}, {"R576p", 576}, {"R720p", 720}, {"R1080p", 1080}, {"R2160p", 2160}} {
			if m.has(r.group) {
				return r.res
			}
		}
	}
	if alt != nil && alt.has("R2160p") {
		return 2160
	}
	return 0
}

func parseQualityModifiers(name, normalized string, rules qualityRules) ParsedQuality {
	result := newParsedQuality()
	version := rules.version.match(normalized)
	versionN := 0
	if version != nil {
		versionN, _ = strconv.Atoi(version.value("version"))
		result.Revision.Version = versionN
		result.RevisionDetection = DetectedName
	}
	if properRegex.isMatch(normalized) {
		result.Revision.Version = 2
		if version != nil {
			result.Revision.Version = versionN + 1
		}
		result.RevisionDetection = DetectedName
	}
	if repackRegex.isMatch(normalized) {
		result.Revision.Version = 2
		if version != nil {
			result.Revision.Version = versionN + 1
		}
		result.Revision.IsRepack = true
		result.RevisionDetection = DetectedName
	}
	if n := len(realRegex.matches(name)); n > 0 {
		result.Revision.Real = n
		result.RevisionDetection = DetectedName
	}
	return result
}

// containsFold reports whether s contains substr, ignoring case.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// catalogEntry places a quality in one application's catalog.
type catalogEntry struct {
	q          Quality
	source     int // the application's own source order
	resolution int
	modifier   Modifier
}

// Sonarr's sources: Unknown, Television, TelevisionRaw, Web, WebRip, DVD,
// Bluray, BlurayRaw.
const (
	sonarrUnknown = iota
	sonarrTelevision
	sonarrTelevisionRaw
	sonarrWeb
	sonarrWebRip
	sonarrDVD
	sonarrBluray
	sonarrBlurayRaw
)

// catalogs list each application's qualities in its own order.
var catalogs = map[Kind][]catalogEntry{
	Series: {
		{Unknown, sonarrUnknown, 0, 0},
		{SDTV, sonarrTelevision, 480, 0},
		{DVD, sonarrDVD, 480, 0},
		{WEBRip480p, sonarrWebRip, 480, 0},
		{WEBDL480p, sonarrWeb, 480, 0},
		{Bluray480p, sonarrBluray, 480, 0},
		{Bluray576p, sonarrBluray, 576, 0},
		{HDTV720p, sonarrTelevision, 720, 0},
		{WEBRip720p, sonarrWebRip, 720, 0},
		{WEBDL720p, sonarrWeb, 720, 0},
		{Bluray720p, sonarrBluray, 720, 0},
		{Bluray1080p, sonarrBluray, 1080, 0},
		{HDTV1080p, sonarrTelevision, 1080, 0},
		{WEBRip1080p, sonarrWebRip, 1080, 0},
		{WEBDL1080p, sonarrWeb, 1080, 0},
		{RawHD, sonarrTelevisionRaw, 1080, 0},
		{HDTV2160p, sonarrTelevision, 2160, 0},
		{WEBRip2160p, sonarrWebRip, 2160, 0},
		{WEBDL2160p, sonarrWeb, 2160, 0},
		{Bluray2160p, sonarrBluray, 2160, 0},
		{Remux1080p, sonarrBlurayRaw, 1080, 0},
		{Remux2160p, sonarrBlurayRaw, 2160, 0},
	},
	Movie: radarrCatalog(),
}

func radarrCatalog() []catalogEntry {
	order := []Quality{
		Unknown, Workprint, Cam, Telesync, Telecine, DVDSCR, Regional, SDTV, DVD, DVDR, HDTV720p, HDTV1080p, HDTV2160p,
		WEBDL480p, WEBDL720p, WEBDL1080p, WEBDL2160p, WEBRip480p, WEBRip720p, WEBRip1080p, WEBRip2160p, Bluray480p,
		Bluray576p, Bluray720p, Bluray1080p, Bluray2160p, Remux1080p, Remux2160p, BRDisk, RawHD,
	}
	out := make([]catalogEntry, len(order))
	for i, q := range order {
		out[i] = catalogEntry{q, int(q.Source()), q.Resolution(), q.Modifier()}
	}
	return out
}

func catalogEntryOf(kind Kind, q Quality) catalogEntry {
	for _, e := range catalogs[kind] {
		if e.q == q {
			return e
		}
	}
	return catalogs[kind][0]
}

// findBySourceAndResolution ports Sonarr's and Radarr's QualityFinder: the
// quality with the source of hint (or the remux source) and resolution, or
// the nearest one.
func findBySourceAndResolution(kind Kind, hint Quality, resolution int, remux bool) Quality {
	catalog := catalogs[kind]
	source := catalogEntryOf(kind, hint).source
	if kind == Series {
		if remux {
			source = sonarrBlurayRaw
		}
		return findSeriesQuality(source, resolution)
	}

	modifier := ModifierNone
	if remux {
		source, modifier = int(SourceBluray), ModifierRemux
	}
	if i := slices.IndexFunc(catalog, func(e catalogEntry) bool {
		return e.source == source && e.resolution == resolution && e.modifier == modifier
	}); i >= 0 {
		return catalog[i].q
	}
	var unknownRes []catalogEntry
	for _, e := range catalog {
		if e.source == source && e.resolution == 0 && e.modifier == modifier && e.q != Unknown {
			unknownRes = append(unknownRes, e)
		}
	}
	if len(unknownRes) == 1 {
		return unknownRes[0].q
	}
	for _, e := range unknownRes {
		if e.source >= source {
			return e.q
		}
	}
	return nearest(catalog, source, func(e catalogEntry) bool { return e.modifier == modifier && e.resolution == resolution })
}

// findSeriesQuality ports Sonarr's QualityFinder.FindBySourceAndResolution:
// the series quality of a source and resolution, or the nearest one.
func findSeriesQuality(source, resolution int) Quality {
	catalog := catalogs[Series]
	if i := slices.IndexFunc(catalog, func(e catalogEntry) bool { return e.source == source && e.resolution == resolution }); i >= 0 {
		return catalog[i].q
	}
	if resolution < 720 {
		switch source {
		case sonarrTelevision:
			return SDTV
		case sonarrWeb:
			return WEBDL480p
		case sonarrWebRip:
			return WEBRip480p
		}
	}
	return nearest(catalog, source, func(e catalogEntry) bool { return e.resolution == resolution })
}

// nearest returns the first quality matching keep, in order of source, whose
// source is at least source; or Unknown.
func nearest(catalog []catalogEntry, source int, keep func(catalogEntry) bool) Quality {
	var candidates []catalogEntry
	for _, e := range catalog {
		if keep(e) {
			candidates = append(candidates, e)
		}
	}
	slices.SortStableFunc(candidates, func(a, b catalogEntry) int { return cmp.Compare(a.source, b.source) })
	for _, e := range candidates {
		if e.source >= source {
			return e.q
		}
	}
	return Unknown
}

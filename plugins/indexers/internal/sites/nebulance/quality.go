package nebulance

import (
	"regexp"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

var (
	sourceRe = regexp.MustCompile(`(?i)\b(?:` +
		`(?P<bluray>BluRay|Blu-Ray|HDDVD|BD)|` +
		`(?P<webdl>WEB[-_. ]DL|WEBDL|WebRip|iTunesHD|WebHD)|` +
		`(?P<hdtv>HDTV)|` +
		`(?P<bdrip>BDRip)|` +
		`(?P<brrip>BRRip)|` +
		`(?P<dvd>DVD|DVDRip|NTSC|PAL|xvidvd)|` +
		`(?P<dsr>WS[-_. ]DSR|DSR)|` +
		`(?P<pdtv>PDTV)|` +
		`(?P<sdtv>SDTV)|` +
		`(?P<tvrip>TVRip)` +
		`)\b`)
	rawHDRe      = regexp.MustCompile(`(?i)\b(?P<rawhd>TrollHD|RawHD|1080i[-_. ]HDTV|Raw[-_. ]HD|MPEG[-_. ]?2)\b`)
	resolutionRe = regexp.MustCompile(`(?i)\b(?:(?P<q480p>480p|640x480|848x480)|(?P<q576p>576p)|(?P<q720p>720p|1280x720)|(?P<q1080p>1080p|1920x1080)|(?P<q2160p>2160p))\b`)
	codecRe      = regexp.MustCompile(`(?i)\b(?:(?P<x264>x264)|(?P<h264>h264)|(?P<xvidhd>XvidHD)|(?P<xvid>Xvid)|(?P<divx>divx))\b`)
	highDefPDTV  = regexp.MustCompile(`(?i)hr[-_. ]ws`)
)

// groups returns the named groups of the first match of a regular
// expression that matched.
func groups(re *regexp.Regexp, s string) map[string]bool {
	out := map[string]bool{}
	m := re.FindStringSubmatchIndex(s)
	if m == nil {
		return out
	}
	for i, name := range re.SubexpNames() {
		if name != "" && m[2*i] >= 0 {
			out[name] = true
		}
	}
	return out
}

// tvCategory returns the TV category of a release by the quality its
// title tells, as Prowlarr's TvCategoryFromQualityParser does.
func tvCategory(title string) *indexer.Category {
	name := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(title), "_", " ")))
	source := groups(sourceRe, name)
	res := groups(resolutionRe, name)
	codec := groups(codecRe, name)
	switch {
	case source["webdl"]:
		switch {
		case res["q2160p"]:
			return indexer.TVUHD
		case res["q1080p"] || res["q720p"]:
			return indexer.TVHD
		case res["q480p"]:
			return indexer.TVSD
		}
	case source["hdtv"]:
		switch {
		case res["q2160p"]:
			return indexer.TVUHD
		case res["q1080p"] || res["q720p"]:
			return indexer.TVHD
		}
		return indexer.TVSD
	}
	if source["bluray"] || source["bdrip"] || source["brrip"] {
		switch {
		case codec["xvid"] || codec["divx"]:
			return indexer.TVSD
		case res["q2160p"]:
			return indexer.TVUHD
		case res["q1080p"] || res["q720p"]:
			return indexer.TVHD
		case res["q480p"] || res["q576p"]:
			return indexer.TVSD
		}
	}
	if source["dvd"] {
		return indexer.TVSD
	}
	if source["pdtv"] || source["sdtv"] || source["dsr"] || source["tvrip"] {
		if highDefPDTV.MatchString(name) {
			return indexer.TVHD
		}
		return indexer.TVSD
	}
	if rawHDRe.MatchString(name) {
		return indexer.TVHD
	}
	return indexer.TV
}

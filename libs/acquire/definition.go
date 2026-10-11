package acquire

import "github.com/mavioai/mavio/libs/release"

// Definition is what Radarr and Sonarr let users set per quality: its
// title, its place among the qualities, and the sizes of releases of it
// in megabytes per minute of runtime.
type Definition struct {
	Quality release.Quality
	Title   string
	// GroupName names the group the quality is put in by default
	// profiles, shared by the qualities of the same Weight.
	GroupName string
	// Weight orders the qualities in default profiles, lowest first.
	Weight int
	// MinSize is the least size, 0 for none.
	MinSize float64
	// MaxSize is the greatest size, 0 for no limit.
	MaxSize float64
	// PreferredSize is the size preferred, 0 for no preference.
	PreferredSize float64
}

func def(q release.Quality, weight int, minSize, maxSize, preferred float64, group string) Definition {
	return Definition{
		Quality: q, Title: q.String(), GroupName: group, Weight: weight,
		MinSize: minSize, MaxSize: maxSize, PreferredSize: preferred,
	}
}

// DefaultDefinitions returns the default definitions of a kind's
// qualities: Radarr's for movies and Sonarr's, whose catalog lacks the
// qualities of pre-releases and discs, for series.
func DefaultDefinitions(kind release.Kind) []Definition {
	if kind == release.Series {
		return []Definition{
			def(release.Unknown, 1, 1, 199.9, 95, ""),
			def(release.SDTV, 2, 2, 100, 95, ""),
			def(release.WEBRip480p, 3, 2, 100, 95, "WEB 480p"),
			def(release.WEBDL480p, 3, 2, 100, 95, "WEB 480p"),
			def(release.DVD, 4, 2, 100, 95, ""),
			def(release.Bluray480p, 5, 2, 100, 95, ""),
			def(release.Bluray576p, 6, 2, 100, 95, ""),
			def(release.HDTV720p, 7, 3, 125, 95, ""),
			def(release.HDTV1080p, 8, 4, 125, 95, ""),
			def(release.RawHD, 9, 4, 0, 95, ""),
			def(release.WEBRip720p, 10, 3, 130, 95, "WEB 720p"),
			def(release.WEBDL720p, 10, 3, 130, 95, "WEB 720p"),
			def(release.Bluray720p, 11, 4, 130, 95, ""),
			def(release.WEBRip1080p, 12, 4, 130, 95, "WEB 1080p"),
			def(release.WEBDL1080p, 12, 4, 130, 95, "WEB 1080p"),
			def(release.Bluray1080p, 13, 4, 155, 95, ""),
			def(release.Remux1080p, 14, 35, 0, 95, ""),
			def(release.HDTV2160p, 15, 35, 199.9, 95, ""),
			def(release.WEBRip2160p, 16, 35, 0, 95, "WEB 2160p"),
			def(release.WEBDL2160p, 16, 35, 0, 95, "WEB 2160p"),
			def(release.Bluray2160p, 17, 35, 0, 95, ""),
			def(release.Remux2160p, 18, 35, 0, 95, ""),
		}
	}
	return []Definition{
		def(release.Unknown, 1, 0, 100, 95, ""),
		def(release.Workprint, 2, 0, 100, 95, ""),
		def(release.Cam, 3, 0, 100, 95, ""),
		def(release.Telesync, 4, 0, 100, 95, ""),
		def(release.Telecine, 5, 0, 100, 95, ""),
		def(release.Regional, 6, 0, 100, 95, ""),
		def(release.DVDSCR, 7, 0, 100, 95, ""),
		def(release.SDTV, 8, 0, 100, 95, ""),
		def(release.DVD, 9, 0, 100, 95, ""),
		def(release.DVDR, 10, 0, 100, 95, ""),
		def(release.WEBDL480p, 11, 0, 100, 95, "WEB 480p"),
		def(release.WEBRip480p, 11, 0, 100, 95, "WEB 480p"),
		def(release.Bluray480p, 12, 0, 100, 95, ""),
		def(release.Bluray576p, 13, 0, 100, 95, ""),
		def(release.HDTV720p, 14, 0, 100, 95, ""),
		def(release.WEBDL720p, 15, 0, 100, 95, "WEB 720p"),
		def(release.WEBRip720p, 15, 0, 100, 95, "WEB 720p"),
		def(release.Bluray720p, 16, 0, 100, 95, ""),
		def(release.HDTV1080p, 17, 0, 100, 95, ""),
		def(release.WEBDL1080p, 18, 0, 100, 95, "WEB 1080p"),
		def(release.WEBRip1080p, 18, 0, 100, 95, "WEB 1080p"),
		def(release.Bluray1080p, 19, 0, 0, 0, ""),
		def(release.Remux1080p, 20, 0, 0, 0, ""),
		def(release.HDTV2160p, 21, 0, 0, 0, ""),
		def(release.WEBDL2160p, 22, 0, 0, 0, "WEB 2160p"),
		def(release.WEBRip2160p, 22, 0, 0, 0, "WEB 2160p"),
		def(release.Bluray2160p, 23, 0, 0, 0, ""),
		def(release.Remux2160p, 24, 0, 0, 0, ""),
		def(release.BRDisk, 25, 0, 0, 0, ""),
		def(release.RawHD, 26, 0, 0, 0, ""),
	}
}

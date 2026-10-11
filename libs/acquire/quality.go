package acquire

import (
	"fmt"

	"github.com/mavioai/mavio/libs/release"
)

// qualityIDs are Radarr's IDs of the qualities, which name them in
// profiles and storage; Sonarr's agree but for its remuxes, 20 and 21,
// which are Radarr's 30 and 31.
var qualityIDs = map[release.Quality]int{
	release.Unknown:     0,
	release.SDTV:        1,
	release.DVD:         2,
	release.WEBDL1080p:  3,
	release.HDTV720p:    4,
	release.WEBDL720p:   5,
	release.Bluray720p:  6,
	release.Bluray1080p: 7,
	release.WEBDL480p:   8,
	release.HDTV1080p:   9,
	release.RawHD:       10,
	release.WEBRip480p:  12,
	release.WEBRip720p:  14,
	release.WEBRip1080p: 15,
	release.HDTV2160p:   16,
	release.WEBRip2160p: 17,
	release.WEBDL2160p:  18,
	release.Bluray2160p: 19,
	release.Bluray480p:  20,
	release.Bluray576p:  21,
	release.BRDisk:      22,
	release.DVDR:        23,
	release.Workprint:   24,
	release.Cam:         25,
	release.Telesync:    26,
	release.Telecine:    27,
	release.DVDSCR:      28,
	release.Regional:    29,
	release.Remux1080p:  30,
	release.Remux2160p:  31,
}

var qualitiesByID = func() map[int]release.Quality {
	m := make(map[int]release.Quality, len(qualityIDs))
	for q, id := range qualityIDs {
		m[id] = q
	}
	return m
}()

// QualityID returns a quality's ID, Radarr's.
func QualityID(q release.Quality) int { return qualityIDs[q] }

// QualityByID returns the quality of an ID, and false for an unknown ID.
func QualityByID(id int) (release.Quality, bool) {
	q, ok := qualitiesByID[id]
	return q, ok
}

// Qualities lists every quality in Radarr's order.
var Qualities = []release.Quality{
	release.Unknown, release.Workprint, release.Cam, release.Telesync, release.Telecine, release.DVDSCR,
	release.Regional, release.SDTV, release.DVD, release.DVDR, release.HDTV720p, release.HDTV1080p,
	release.HDTV2160p, release.WEBDL480p, release.WEBDL720p, release.WEBDL1080p, release.WEBDL2160p,
	release.WEBRip480p, release.WEBRip720p, release.WEBRip1080p, release.WEBRip2160p, release.Bluray480p,
	release.Bluray576p, release.Bluray720p, release.Bluray1080p, release.Bluray2160p, release.Remux1080p,
	release.Remux2160p, release.BRDisk, release.RawHD,
}

// QualityModel is a quality with its revision, as Radarr's QualityModel.
type QualityModel struct {
	Quality  release.Quality
	Revision release.Revision
}

// NewQualityModel returns a quality at revision 1.
func NewQualityModel(q release.Quality) QualityModel {
	return QualityModel{Quality: q, Revision: release.Revision{Version: 1}}
}

// ModelOf returns the quality model of a parsed quality.
func ModelOf(p release.ParsedQuality) QualityModel {
	return QualityModel{Quality: p.Quality, Revision: p.Revision}
}

// Equal reports whether two models have the same quality and revision,
// ignoring whether the revision is a repack.
func (m QualityModel) Equal(o QualityModel) bool {
	return m.Quality == o.Quality && m.Revision.Version == o.Revision.Version && m.Revision.Real == o.Revision.Real
}

func (m QualityModel) String() string {
	s := fmt.Sprintf("%s v%d", m.Quality, m.Revision.Version)
	if m.Revision.Real > 0 {
		s += fmt.Sprintf(" Real:%d", m.Revision.Real)
	}
	return s
}

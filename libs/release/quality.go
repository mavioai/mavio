package release

import "fmt"

// Kind selects the rule set where Sonarr and Radarr differ.
type Kind int

// Kinds of releases.
const (
	// Movie follows Radarr.
	Movie Kind = iota + 1
	// Series follows Sonarr.
	Series
)

// Source is where a release was captured from, in Radarr's order of
// increasing fidelity.
type Source int

// Sources.
const (
	SourceUnknown Source = iota
	SourceCam
	SourceTelesync
	SourceTelecine
	SourceWorkprint
	SourceDVD
	SourceTV
	SourceWebDL
	SourceWebRip
	SourceBluray
)

// Modifier qualifies a source.
type Modifier int

// Modifiers.
const (
	ModifierNone Modifier = iota
	ModifierRegional
	ModifierScreener
	ModifierRawHD
	ModifierBRDisk
	ModifierRemux
)

// Quality is a release quality: a source, a resolution and a modifier. The
// set is the union of Sonarr's and Radarr's; the names are Radarr's.
type Quality int

// Qualities.
const (
	Unknown Quality = iota
	Workprint
	Cam
	Telesync
	Telecine
	DVDSCR
	Regional
	SDTV
	DVD
	DVDR
	HDTV720p
	HDTV1080p
	HDTV2160p
	WEBDL480p
	WEBDL720p
	WEBDL1080p
	WEBDL2160p
	WEBRip480p
	WEBRip720p
	WEBRip1080p
	WEBRip2160p
	Bluray480p
	Bluray576p
	Bluray720p
	Bluray1080p
	Bluray2160p
	Remux1080p
	Remux2160p
	BRDisk
	RawHD
)

type qualityDef struct {
	name       string
	source     Source
	resolution int
	modifier   Modifier
}

var qualityDefs = [...]qualityDef{
	Unknown:     {"Unknown", SourceUnknown, 0, ModifierNone},
	Workprint:   {"WORKPRINT", SourceWorkprint, 0, ModifierNone},
	Cam:         {"CAM", SourceCam, 0, ModifierNone},
	Telesync:    {"TELESYNC", SourceTelesync, 0, ModifierNone},
	Telecine:    {"TELECINE", SourceTelecine, 0, ModifierNone},
	DVDSCR:      {"DVDSCR", SourceDVD, 480, ModifierScreener},
	Regional:    {"REGIONAL", SourceDVD, 480, ModifierRegional},
	SDTV:        {"SDTV", SourceTV, 480, ModifierNone},
	DVD:         {"DVD", SourceDVD, 0, ModifierNone},
	DVDR:        {"DVD-R", SourceDVD, 480, ModifierRemux},
	HDTV720p:    {"HDTV-720p", SourceTV, 720, ModifierNone},
	HDTV1080p:   {"HDTV-1080p", SourceTV, 1080, ModifierNone},
	HDTV2160p:   {"HDTV-2160p", SourceTV, 2160, ModifierNone},
	WEBDL480p:   {"WEBDL-480p", SourceWebDL, 480, ModifierNone},
	WEBDL720p:   {"WEBDL-720p", SourceWebDL, 720, ModifierNone},
	WEBDL1080p:  {"WEBDL-1080p", SourceWebDL, 1080, ModifierNone},
	WEBDL2160p:  {"WEBDL-2160p", SourceWebDL, 2160, ModifierNone},
	WEBRip480p:  {"WEBRip-480p", SourceWebRip, 480, ModifierNone},
	WEBRip720p:  {"WEBRip-720p", SourceWebRip, 720, ModifierNone},
	WEBRip1080p: {"WEBRip-1080p", SourceWebRip, 1080, ModifierNone},
	WEBRip2160p: {"WEBRip-2160p", SourceWebRip, 2160, ModifierNone},
	Bluray480p:  {"Bluray-480p", SourceBluray, 480, ModifierNone},
	Bluray576p:  {"Bluray-576p", SourceBluray, 576, ModifierNone},
	Bluray720p:  {"Bluray-720p", SourceBluray, 720, ModifierNone},
	Bluray1080p: {"Bluray-1080p", SourceBluray, 1080, ModifierNone},
	Bluray2160p: {"Bluray-2160p", SourceBluray, 2160, ModifierNone},
	Remux1080p:  {"Remux-1080p", SourceBluray, 1080, ModifierRemux},
	Remux2160p:  {"Remux-2160p", SourceBluray, 2160, ModifierRemux},
	BRDisk:      {"BR-DISK", SourceBluray, 1080, ModifierBRDisk},
	RawHD:       {"Raw-HD", SourceTV, 1080, ModifierRawHD},
}

func (q Quality) def() qualityDef {
	if q < 0 || int(q) >= len(qualityDefs) {
		return qualityDefs[Unknown]
	}
	return qualityDefs[q]
}

// String returns the quality's name, e.g. "WEBDL-1080p".
func (q Quality) String() string {
	if q < 0 || int(q) >= len(qualityDefs) {
		return fmt.Sprintf("Quality(%d)", int(q))
	}
	return qualityDefs[q].name
}

// Source returns the quality's source.
func (q Quality) Source() Source { return q.def().source }

// Resolution returns the quality's vertical resolution, or 0 when unknown.
func (q Quality) Resolution() int { return q.def().resolution }

// Modifier returns the quality's modifier.
func (q Quality) Modifier() Modifier { return q.def().modifier }

// Revision distinguishes releases of the same quality: Version counts
// proper/repack/vN re-releases, Real counts REAL tags.
type Revision struct {
	Version  int
	Real     int
	IsRepack bool
}

// Compare orders revisions by Real, then Version.
func (r Revision) Compare(o Revision) int {
	switch {
	case r.Real != o.Real:
		return cmpInt(r.Real, o.Real)
	default:
		return cmpInt(r.Version, o.Version)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Detection says where a part of a parsed quality came from.
type Detection int

// Detections.
const (
	DetectedUnknown Detection = iota
	DetectedName
	DetectedExtension
	DetectedMediaInfo
)

// ParsedQuality is the quality parsed from a name.
type ParsedQuality struct {
	Quality  Quality
	Revision Revision
	// Resolution overrides Quality.Resolution() when non-zero (Radarr records
	// the resolution of TELESYNC releases).
	Resolution int

	SourceDetection     Detection
	ResolutionDetection Detection
	ModifierDetection   Detection
	RevisionDetection   Detection
}

// newParsedQuality returns an Unknown quality at revision 1.
func newParsedQuality() ParsedQuality {
	return ParsedQuality{Revision: Revision{Version: 1}}
}

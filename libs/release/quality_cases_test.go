package release

import (
	"strings"
	"testing"
)

// qualitySymbols maps Sonarr's and Radarr's Quality members.
var qualitySymbols = map[string]Quality{
	"Unknown": Unknown, "WORKPRINT": Workprint, "CAM": Cam, "TELESYNC": Telesync, "TELECINE": Telecine,
	"DVDSCR": DVDSCR, "REGIONAL": Regional, "SDTV": SDTV, "DVD": DVD, "DVDR": DVDR,
	"HDTV720p": HDTV720p, "HDTV1080p": HDTV1080p, "HDTV2160p": HDTV2160p,
	"WEBDL480p": WEBDL480p, "WEBDL720p": WEBDL720p, "WEBDL1080p": WEBDL1080p, "WEBDL2160p": WEBDL2160p,
	"WEBRip480p": WEBRip480p, "WEBRip720p": WEBRip720p, "WEBRip1080p": WEBRip1080p, "WEBRip2160p": WEBRip2160p,
	"Bluray480p": Bluray480p, "Bluray576p": Bluray576p, "Bluray720p": Bluray720p, "Bluray1080p": Bluray1080p,
	"Bluray2160p": Bluray2160p, "Remux1080p": Remux1080p, "Remux2160p": Remux2160p,
	"Bluray1080pRemux": Remux1080p, "Bluray2160pRemux": Remux2160p, "BRDISK": BRDisk, "RAWHD": RawHD,
}

// sonarrQualityNames are Sonarr's display names where they differ from
// Radarr's.
var sonarrQualityNames = map[Quality]string{Remux1080p: "Bluray-1080p Remux", Remux2160p: "Bluray-2160p Remux"}

var radarrSources = map[string]Source{
	"UNKNOWN": SourceUnknown, "CAM": SourceCam, "TELESYNC": SourceTelesync, "TELECINE": SourceTelecine,
	"WORKPRINT": SourceWorkprint, "DVD": SourceDVD, "TV": SourceTV, "WEBDL": SourceWebDL, "WEBRIP": SourceWebRip,
	"BLURAY": SourceBluray,
}

var radarrModifiers = map[string]Modifier{
	"NONE": ModifierNone, "REGIONAL": ModifierRegional, "SCREENER": ModifierScreener, "RAWHD": ModifierRawHD,
	"BRDISK": ModifierBRDisk, "REMUX": ModifierRemux,
}

var radarrResolutions = map[string]int{
	"Unknown": 0, "R360p": 360, "R480p": 480, "R540p": 540, "R576p": 576, "R720p": 720, "R1080p": 1080, "R2160p": 2160,
}

func quality(t *testing.T, a args, name string) Quality {
	t.Helper()
	s := a.symbol(t, name)
	q, ok := qualitySymbols[s]
	if !ok {
		t.Fatalf("unknown quality %s", s)
	}
	return q
}

func checkQuality(t *testing.T, kind Kind, title string, want Quality, proper bool) {
	t.Helper()
	got := ParseQuality(title, kind)
	if got.Quality != want {
		t.Errorf("ParseQuality(%q).Quality: got = %v, want = %v", title, got.Quality, want)
	}
	wantVersion := 1
	if proper {
		wantVersion = 2
	}
	if got.Revision.Version != wantVersion {
		t.Errorf("ParseQuality(%q).Revision.Version: got = %d, want = %d", title, got.Revision.Version, wantVersion)
	}
}

// checkMovieQuality ports Radarr's ParseAndVerifyQuality.
func checkMovieQuality(t *testing.T, title string, source Source, proper bool, resolution int, modifier Modifier) {
	t.Helper()
	got := ParseQuality(title, Movie)
	res := got.Quality.Resolution()
	if got.Resolution != 0 {
		res = got.Resolution
	}
	if resolution != 0 && res != resolution {
		t.Errorf("ParseQuality(%q) resolution: got = %d (%v), want = %d", title, res, got.Quality, resolution)
	}
	if got.Quality.Source() != source {
		t.Errorf("ParseQuality(%q) source: got = %v (%v), want = %v", title, got.Quality.Source(), got.Quality, source)
	}
	if modifier != ModifierNone && got.Quality.Modifier() != modifier {
		t.Errorf("ParseQuality(%q) modifier: got = %v (%v), want = %v", title, got.Quality.Modifier(), got.Quality, modifier)
	}
	wantVersion := 1
	if proper {
		wantVersion = 2
	}
	if got.Revision.Version != wantVersion {
		t.Errorf("ParseQuality(%q).Revision.Version: got = %d, want = %d", title, got.Revision.Version, wantVersion)
	}
}

func checkDetection(kind Kind, source, resolution Detection) func(t *testing.T, a args) {
	return func(t *testing.T, a args) {
		title := a.str(t, "title")
		got := ParseQuality(title, kind)
		if got.SourceDetection != source || got.ResolutionDetection != resolution {
			t.Errorf("ParseQuality(%q) detection: got = %v/%v, want = %v/%v", title, got.SourceDetection, got.ResolutionDetection, source, resolution)
		}
	}
}

func checkRevision(kind Kind) func(t *testing.T, a args) {
	return func(t *testing.T, a args) {
		title := a.str(t, "title")
		got := ParseQuality(title, kind)
		if v := a.integer(t, "version"); got.Revision.Version != v {
			t.Errorf("ParseQuality(%q).Revision.Version: got = %d, want = %d", title, got.Revision.Version, v)
		}
		if a.has("isRepack") && got.Revision.IsRepack != a.boolean(t, "isRepack") {
			t.Errorf("ParseQuality(%q).Revision.IsRepack: got = %v, want = %v", title, got.Revision.IsRepack, a.boolean(t, "isRepack"))
		}
	}
}

func otherSourceTitles(s string) []string {
	var out []string
	for _, c := range []string{"-", ".", " ", "_"} {
		out = append(out, "My series S01E01 "+strings.ReplaceAll(s, " ", c))
	}
	return out
}

func TestSeriesQualityCases(t *testing.T) {
	run := map[string]func(t *testing.T, a args){
		"quality_parse": func(t *testing.T, a args) {
			checkQuality(t, Series, a.str(t, "title"), Unknown, a.boolean(t, "proper"))
		},
		"parsing_our_own_quality_enum_name": func(t *testing.T, a args) {
			q := quality(t, a, "quality")
			name := q.String()
			if n, ok := sonarrQualityNames[q]; ok {
				name = n
			}
			title := "My series S01E01 [" + name + "]"
			if got := ParseQuality(title, Series).Quality; got != q {
				t.Errorf("ParseQuality(%q): got = %v, want = %v", title, got, q)
			}
		},
		"should_parse_quality_from_other_source": func(t *testing.T, a args) {
			for _, title := range otherSourceTitles(a.str(t, "qualityString")) {
				checkQuality(t, Series, title, quality(t, a, "quality"), false)
			}
		},
		"should_parse_full_quality_from_name":                         checkDetection(Series, DetectedName, DetectedName),
		"should_parse_resolution_from_name":                           checkDetection(Series, DetectedUnknown, DetectedName),
		"should_parse_source_from_name":                               checkDetection(Series, DetectedName, DetectedUnknown),
		"should_parse_quality_from_extension":                         checkDetection(Series, DetectedExtension, DetectedExtension),
		"should_parse_resolution_from_name_and_source_from_extension": checkDetection(Series, DetectedExtension, DetectedName),
		"should_be_able_to_parse_repack":                              checkRevision(Series),
		"should_be_able_to_parse_anime_version":                       checkRevision(Series),
	}
	for method, q := range map[string]Quality{
		"should_parse_sdtv_quality": SDTV, "should_parse_dvd_quality": DVD, "should_parse_webdl480p_quality": WEBDL480p,
		"should_parse_bluray480p_quality": Bluray480p, "should_parse_webrip480p_quality": WEBRip480p,
		"should_parse_bluray576p_quality": Bluray576p, "should_parse_hdtv720p_quality": HDTV720p,
		"should_parse_hdtv1080p_quality": HDTV1080p, "should_parse_hdtv2160p_quality": HDTV2160p,
		"should_parse_webdl720p_quality": WEBDL720p, "should_parse_webrip720p_quality": WEBRip720p,
		"should_parse_webdl1080p_quality": WEBDL1080p, "should_parse_webrip1080p_quality": WEBRip1080p,
		"should_parse_webdl2160p_quality": WEBDL2160p, "should_parse_webrip2160p_quality": WEBRip2160p,
		"should_parse_bluray720p_quality": Bluray720p, "should_parse_bluray1080p_quality": Bluray1080p,
		"should_parse_bluray1080p_remux_quality": Remux1080p, "should_parse_bluray2160p_quality": Bluray2160p,
		"should_parse_bluray2160p_remux_quality": Remux2160p, "should_parse_raw_quality": RawHD,
	} {
		run[method] = func(t *testing.T, a args) { checkQuality(t, Series, a.str(t, "title"), q, a.boolean(t, "proper")) }
	}
	portedCases(t, "series/quality_parser.json", ported{run: run})
}

func TestMovieQualityCases(t *testing.T) {
	verify := func(source Source, resolution int, modifier Modifier) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			checkMovieQuality(t, a.str(t, "title"), source, a.boolean(t, "proper"), resolution, modifier)
		}
	}
	run := map[string]func(t *testing.T, a args){
		"should_parse_ts":                              verify(SourceTelesync, 720, ModifierNone),
		"should_parse_cam":                             verify(SourceCam, 0, ModifierNone),
		"should_parse_bluray576p_quality":              verify(SourceBluray, 576, ModifierNone),
		"should_parse_remux720p_as_bluray720p_quality": verify(SourceBluray, 720, ModifierNone),
		"should_parse_remux1080p_quality":              verify(SourceBluray, 1080, ModifierRemux),
		"should_parse_remux2160p_quality":              verify(SourceBluray, 2160, ModifierRemux),
		"should_parse_brdisk_1080p_quality":            verify(SourceBluray, 1080, ModifierBRDisk),
		"quality_parse":                                verify(SourceUnknown, 0, ModifierNone),
		"should_parse_quality_from_other_source": func(t *testing.T, a args) {
			modifier := ModifierNone
			if a.has("modifier") {
				modifier = radarrModifiers[a.symbol(t, "modifier")]
			}
			for _, title := range otherSourceTitles(a.str(t, "qualityString")) {
				checkMovieQuality(t, title, radarrSources[a.symbol(t, "source")], false, radarrResolutions[a.symbol(t, "resolution")], modifier)
			}
		},
		"should_parse_full_quality_from_name":                         checkDetection(Movie, DetectedName, DetectedName),
		"should_parse_resolution_from_name":                           checkDetection(Movie, DetectedUnknown, DetectedName),
		"should_parse_source_from_name":                               checkDetection(Movie, DetectedName, DetectedUnknown),
		"should_parse_quality_from_extension":                         checkDetection(Movie, DetectedExtension, DetectedExtension),
		"should_parse_resolution_from_name_and_source_from_extension": checkDetection(Movie, DetectedExtension, DetectedName),
		"should_be_able_to_parse_repack":                              checkRevision(Movie),
	}
	for method, q := range map[string]Quality{
		"should_parse_sdtv_quality": SDTV, "should_parse_dvd_quality": DVD, "should_parse_dvdr_quality": DVDR,
		"should_parse_webdl480p_quality": WEBDL480p, "should_parse_webrip480p_quality": WEBRip480p,
		"should_parse_bluray480p_quality": Bluray480p, "should_parse_hdtv720p_quality": HDTV720p,
		"should_parse_hdtv1080p_quality": HDTV1080p, "should_parse_hdtv2160p_quality": HDTV2160p,
		"should_parse_webdl720p_quality": WEBDL720p, "should_parse_webrip720p_quality": WEBRip720p,
		"should_parse_webdl1080p_quality": WEBDL1080p, "should_parse_webrip1080p_quality": WEBRip1080p,
		"should_parse_webdl2160p_quality": WEBDL2160p, "should_parse_webrip2160p_quality": WEBRip2160p,
		"should_parse_bluray720p_quality": Bluray720p, "should_parse_bluray1080p_quality": Bluray1080p,
		"should_parse_bluray2160p_quality": Bluray2160p, "should_parse_raw_quality": RawHD,
	} {
		run[method] = verify(q.Source(), q.Resolution(), q.Modifier())
	}
	portedCases(t, "movie/quality_parser.json", ported{run: run})
}

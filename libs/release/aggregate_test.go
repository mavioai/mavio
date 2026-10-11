package release

import "testing"

// Ports Sonarr's AggregateQualityFixture.
func TestAggregateSeriesQuality(t *testing.T) {
	rev := func(v int) *Revision { return &Revision{Version: v} }
	mediaInfo := func(resolution int) QualityAugment {
		return QualityAugment{SourceConfidence: ConfidenceDefault, Resolution: resolution, ResolutionConfidence: ConfidenceMediaInfo}
	}
	extension := QualityAugment{SeriesSourceTelevision, ConfidenceFallback, 720, ConfidenceFallback, rev(0), ConfidenceFallback}
	name := QualityAugment{SeriesSourceTelevision, ConfidenceDefault, 480, ConfidenceDefault, rev(0), ConfidenceDefault}
	releaseName := QualityAugment{Source: SeriesSourceWeb, SourceConfidence: ConfidenceMediaInfo, ResolutionConfidence: ConfidenceDefault}
	withRevision := func(a QualityAugment, v int, c Confidence) QualityAugment {
		a.Revision, a.RevisionConfidence = rev(v), c
		return a
	}
	for _, tt := range []struct {
		name                       string
		augments                   []QualityAugment
		want                       Quality
		source, resolution, revDet Detection
		version                    int
	}{
		{"HDTV-720p from the extension", []QualityAugment{extension}, HDTV720p, DetectedExtension, DetectedExtension, DetectedUnknown, 1},
		{"SDTV when HDTV-720p came from the extension", []QualityAugment{extension, name}, SDTV, DetectedName, DetectedName, DetectedUnknown, 1},
		{"HDTV-1080p from the extension and media info", []QualityAugment{extension, mediaInfo(1080)}, HDTV1080p, DetectedExtension, DetectedMediaInfo, DetectedUnknown, 1},
		{"HDTV-1080p from the name and media info", []QualityAugment{name, mediaInfo(1080)}, HDTV1080p, DetectedName, DetectedMediaInfo, DetectedUnknown, 1},
		{"WEBDL-480p from the release name's source", []QualityAugment{name, releaseName}, WEBDL480p, DetectedName, DetectedName, DetectedUnknown, 1},
		{"proper in the name", []QualityAugment{withRevision(name, 2, ConfidenceTag), releaseName}, WEBDL480p, DetectedName, DetectedName, DetectedName, 2},
		{"v0 in the name", []QualityAugment{withRevision(name, 0, ConfidenceTag), releaseName}, WEBDL480p, DetectedName, DetectedName, DetectedName, 0},
		{
			"v0 in the name, v2 in the release name",
			[]QualityAugment{withRevision(name, 0, ConfidenceTag), withRevision(name, 2, ConfidenceTag)},
			SDTV, DetectedName, DetectedName, DetectedName, 2,
		},
		{
			"Bluray-576p from the name and media info",
			[]QualityAugment{{SeriesSourceBluray, ConfidenceDefault, 480, ConfidenceDefault, rev(0), ConfidenceTag}, mediaInfo(576)},
			Bluray576p, DetectedName, DetectedMediaInfo, DetectedName, 0,
		},
		{
			"SDTV from HDTV in the name and 576p media info",
			[]QualityAugment{{SeriesSourceTelevision, ConfidenceDefault, 480, ConfidenceDefault, rev(0), ConfidenceTag}, mediaInfo(576)},
			SDTV, DetectedName, DetectedMediaInfo, DetectedName, 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := AggregateSeriesQuality(tt.augments...)
			if got.Quality != tt.want || got.SourceDetection != tt.source || got.ResolutionDetection != tt.resolution ||
				got.RevisionDetection != tt.revDet || got.Revision.Version != tt.version {
				t.Errorf("AggregateSeriesQuality() = %v (source %v, resolution %v, revision %v v%d), want = %v (%v, %v, %v v%d)",
					got.Quality, got.SourceDetection, got.ResolutionDetection, got.RevisionDetection, got.Revision.Version,
					tt.want, tt.source, tt.resolution, tt.revDet, tt.version)
			}
		})
	}
}

// Ports Sonarr's AugmentQualityFromMediaInfoFixture.
func TestAugmentFromVideo(t *testing.T) {
	for _, tt := range []struct {
		width, height, want int
	}{
		{4096, 1, 2160},
		{4000, 1, 2160},
		{3840, 1, 2160},
		{3200, 1, 2160},
		{2000, 1, 1080},
		{1920, 1, 1080},
		{1440, 1080, 1080},
		{1800, 1, 1080},
		{1490, 1, 720},
		{1280, 1, 720},
		{1200, 1, 720},
		{1000, 1, 576},
		{720, 576, 576},
		{800, 1, 480},
		{720, 1, 480},
		{600, 1, 480},
		{100, 1, 480},
		{1, 2160, 2160},
		{1, 1080, 1080},
		{1, 720, 720},
		{1, 576, 576},
		{1, 480, 480},
	} {
		got, ok := AugmentFromVideo(tt.width, tt.height)
		if !ok || got.Resolution != tt.want || got.ResolutionConfidence != ConfidenceMediaInfo {
			t.Errorf("AugmentFromVideo(%d, %d) = %d, %v, want = %d", tt.width, tt.height, got.Resolution, ok, tt.want)
		}
	}
	if _, ok := AugmentFromVideo(0, 0); ok {
		t.Error("AugmentFromVideo(0, 0) told a resolution")
	}
}

func TestAugmentFromName(t *testing.T) {
	a := AugmentFromName(ParseQuality("Series.Title.S01E01.720p.HDTV.x264-GRP.mkv", Series))
	if a.Source != SeriesSourceTelevision || a.SourceConfidence != ConfidenceTag || a.Resolution != 720 || a.ResolutionConfidence != ConfidenceTag {
		t.Errorf("AugmentFromName(HDTV-720p) = %+v", a)
	}
	a = AugmentFromName(ParseQuality("Series Title - S01E01.mkv", Series))
	if a.SourceConfidence != ConfidenceFallback || a.ResolutionConfidence != ConfidenceFallback || a.RevisionConfidence != ConfidenceFallback {
		t.Errorf("AugmentFromName(extension only) = %+v, want = fallbacks", a)
	}
}

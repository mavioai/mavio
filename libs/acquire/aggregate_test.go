package acquire

import (
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

// The augments of Radarr's AggregateQualityFixture.
func fileExtensionAugment() *QualityAugment {
	return &QualityAugment{
		Source: release.SourceTV, SourceConfidence: ConfidenceFallback, Resolution: 720, ResolutionConfidence: ConfidenceFallback,
		ModifierConfidence: ConfidenceFallback, Revision: &release.Revision{Version: 1}, RevisionConfidence: ConfidenceFallback,
	}
}

func nameAugment(revision int, revisionConfidence Confidence) *QualityAugment {
	return &QualityAugment{
		Source: release.SourceTV, SourceConfidence: ConfidenceDefault, Resolution: 480, ResolutionConfidence: ConfidenceDefault,
		ModifierConfidence: ConfidenceFallback, Revision: &release.Revision{Version: revision}, RevisionConfidence: revisionConfidence,
	}
}

func mediaInfoAugment() *QualityAugment {
	return &QualityAugment{SourceConfidence: ConfidenceDefault, Resolution: 1080, ResolutionConfidence: ConfidenceMediaInfo, ModifierConfidence: ConfidenceDefault, RevisionConfidence: ConfidenceDefault}
}

func releaseNameAugment() *QualityAugment {
	return &QualityAugment{Source: release.SourceWebDL, SourceConfidence: ConfidenceMediaInfo, ResolutionConfidence: ConfidenceDefault, ModifierConfidence: ConfidenceDefault, RevisionConfidence: ConfidenceDefault}
}

// TestAggregateQuality ports Radarr's AggregateQualityFixture.
func TestAggregateQuality(t *testing.T) {
	for _, tc := range []struct {
		name               string
		augments           []*QualityAugment
		want               release.Quality
		source, resolution release.Detection
		revision           int
		revisionDetection  release.Detection
	}{
		{"HDTV-720p from the extension when the others say nothing", []*QualityAugment{fileExtensionAugment(), nil}, release.HDTV720p, release.DetectedExtension, release.DetectedExtension, 1, release.DetectedUnknown},
		{"SDTV when HDTV-720p came from the extension", []*QualityAugment{fileExtensionAugment(), nameAugment(1, ConfidenceDefault)}, release.SDTV, release.DetectedName, release.DetectedName, 1, release.DetectedUnknown},
		{"HDTV-1080p when the media info says 1080p over the extension", []*QualityAugment{fileExtensionAugment(), mediaInfoAugment()}, release.HDTV1080p, release.DetectedExtension, release.DetectedMediaInfo, 1, release.DetectedUnknown},
		{"HDTV-1080p when the media info says 1080p over the name", []*QualityAugment{nameAugment(1, ConfidenceDefault), mediaInfoAugment()}, release.HDTV1080p, release.DetectedName, release.DetectedMediaInfo, 1, release.DetectedUnknown},
		{"WEBDL-480p when the release name says WEB-DL", []*QualityAugment{nameAugment(1, ConfidenceDefault), releaseNameAugment()}, release.WEBDL480p, release.DetectedName, release.DetectedName, 1, release.DetectedUnknown},
		{"version 2 when the name says proper", []*QualityAugment{nameAugment(2, ConfidenceTag), releaseNameAugment()}, release.WEBDL480p, release.DetectedName, release.DetectedName, 2, release.DetectedName},
		{"version 0 when the file name says v0", []*QualityAugment{nameAugment(0, ConfidenceTag), releaseNameAugment()}, release.WEBDL480p, release.DetectedName, release.DetectedName, 0, release.DetectedName},
		{"version 2 when the file name says v0 and the release name v2", []*QualityAugment{nameAugment(0, ConfidenceTag), nameAugment(2, ConfidenceTag)}, release.SDTV, release.DetectedName, release.DetectedName, 2, release.DetectedName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AggregateQuality(tc.augments...)
			if got.Quality != tc.want || got.SourceDetection != tc.source || got.ResolutionDetection != tc.resolution {
				t.Errorf("quality = %s (source %d, resolution %d), want = %s (source %d, resolution %d)",
					got.Quality, got.SourceDetection, got.ResolutionDetection, tc.want, tc.source, tc.resolution)
			}
			if got.Revision.Version != tc.revision || got.RevisionDetection != tc.revisionDetection {
				t.Errorf("revision = %d (%d), want = %d (%d)", got.Revision.Version, got.RevisionDetection, tc.revision, tc.revisionDetection)
			}
		})
	}
}

// TestAugmentMediaInfoQuality ports Radarr's
// AugmentQualityFromMediaInfoFixture.
func TestAugmentMediaInfoQuality(t *testing.T) {
	for _, tc := range []struct {
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
		{1024, 1, 576},
		{768, 576, 576},
		{800, 1, 480},
		{720, 1, 480},
		{600, 1, 480},
		{100, 1, 480},
	} {
		got := AugmentMediaInfoQuality(tc.width, tc.height, "")
		if got == nil || got.Resolution != tc.want || got.Source != release.SourceUnknown {
			t.Errorf("AugmentMediaInfoQuality(%d, %d) = %+v, want = %dp of an unknown source", tc.width, tc.height, got, tc.want)
		}
	}
	for _, tc := range []struct{ width, height int }{{0, 0}, {0, 1}} {
		if got := AugmentMediaInfoQuality(tc.width, tc.height, ""); got != nil {
			t.Errorf("AugmentMediaInfoQuality(%d, %d) = %+v, want = nil", tc.width, tc.height, got)
		}
	}
	for _, tc := range []struct {
		title string
		want  release.Source
	}{
		{"Movie.Title.2008.WEB.x264-Radarr", release.SourceWebDL},
		{"Movie.Title.2008.x264-Radarr", release.SourceUnknown},
	} {
		got := AugmentMediaInfoQuality(1920, 1080, tc.title)
		if got == nil || got.Resolution != 1080 || got.Source != tc.want {
			t.Errorf("AugmentMediaInfoQuality(%q) = %+v, want = 1080p of source %d", tc.title, got, tc.want)
		}
	}
}

// TestFindBySourceAndResolution checks the nearest qualities of Radarr's
// QualityFinder.
func TestFindBySourceAndResolution(t *testing.T) {
	for _, tc := range []struct {
		source     release.Source
		resolution int
		modifier   release.Modifier
		want       release.Quality
	}{
		{release.SourceBluray, 1080, release.ModifierNone, release.Bluray1080p},
		{release.SourceBluray, 1080, release.ModifierRemux, release.Remux1080p},
		{release.SourceDVD, 0, release.ModifierNone, release.DVD},
		{release.SourceDVD, 1080, release.ModifierNone, release.DVD},
		{release.SourceTelesync, 720, release.ModifierNone, release.Telesync},
		{release.SourceUnknown, 1080, release.ModifierNone, release.HDTV1080p},
		{release.SourceWebRip, 576, release.ModifierNone, release.Bluray576p},
		{release.SourceUnknown, 0, release.ModifierNone, release.Unknown},
	} {
		if got := FindBySourceAndResolution(tc.source, tc.resolution, tc.modifier); got != tc.want {
			t.Errorf("FindBySourceAndResolution(%d, %d, %d) = %s, want = %s", tc.source, tc.resolution, tc.modifier, got, tc.want)
		}
	}
}

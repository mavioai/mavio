package release

// SeriesSource is the source of a quality as Sonarr's QualitySource.
type SeriesSource int

// Sonarr's sources.
const (
	SeriesSourceUnknown       SeriesSource = sonarrUnknown
	SeriesSourceTelevision    SeriesSource = sonarrTelevision
	SeriesSourceTelevisionRaw SeriesSource = sonarrTelevisionRaw
	SeriesSourceWeb           SeriesSource = sonarrWeb
	SeriesSourceWebRip        SeriesSource = sonarrWebRip
	SeriesSourceDVD           SeriesSource = sonarrDVD
	SeriesSourceBluray        SeriesSource = sonarrBluray
	SeriesSourceBlurayRaw     SeriesSource = sonarrBlurayRaw
)

// SeriesSourceOf returns the Sonarr source of a series quality.
func SeriesSourceOf(q Quality) SeriesSource { return SeriesSource(catalogEntryOf(Series, q).source) }

// FindSeriesQuality returns the series quality of a source and resolution,
// or the nearest one, as Sonarr's QualityFinder.FindBySourceAndResolution.
func FindSeriesQuality(source SeriesSource, resolution int) Quality {
	return findSeriesQuality(int(source), resolution)
}

// Confidence is how sure a part of a quality is, as Sonarr's Confidence,
// in increasing order.
type Confidence int

// Confidences.
const (
	ConfidenceFallback Confidence = iota
	ConfidenceDefault
	ConfidenceTag
	ConfidenceMediaInfo
)

// QualityAugment is what one source of a file's quality tells of it, as
// Sonarr's AugmentQualityResult: a source, a resolution and a revision,
// each with how sure it is. A zero resolution and a nil revision tell
// nothing.
type QualityAugment struct {
	Source               SeriesSource
	SourceConfidence     Confidence
	Resolution           int
	ResolutionConfidence Confidence
	Revision             *Revision
	RevisionConfidence   Confidence
}

// AugmentFromName returns what a quality parsed from a name tells, as
// Sonarr's augmenters of file, folder, download and release names: the
// parts the name states are tags, the others fallbacks.
func AugmentFromName(p ParsedQuality) QualityAugment {
	confidence := func(d Detection) Confidence {
		if d == DetectedName {
			return ConfidenceTag
		}
		return ConfidenceFallback
	}
	resolution := p.Quality.Resolution()
	if p.Resolution != 0 {
		resolution = p.Resolution
	}
	revision := p.Revision
	return QualityAugment{
		Source: SeriesSourceOf(p.Quality), SourceConfidence: confidence(p.SourceDetection),
		Resolution: resolution, ResolutionConfidence: confidence(p.ResolutionDetection),
		Revision: &revision, RevisionConfidence: confidence(p.RevisionDetection),
	}
}

// AugmentFromVideo returns the resolution the frame size of a file's
// video tells, as Sonarr's AugmentQualityFromMediaInfo; false without a
// frame size.
func AugmentFromVideo(width, height int) (QualityAugment, bool) {
	resolution := 0
	switch {
	case width >= 3200 || height >= 2100:
		resolution = 2160
	case width >= 1800 || height >= 1000:
		resolution = 1080
	case width >= 1200 || height >= 700:
		resolution = 720
	case width >= 1000 || height >= 560:
		resolution = 576
	case width > 0 && height > 0:
		resolution = 480
	default:
		return QualityAugment{}, false
	}
	return QualityAugment{SourceConfidence: ConfidenceDefault, Resolution: resolution, ResolutionConfidence: ConfidenceMediaInfo}, true
}

// AggregateSeriesQuality returns the quality of a file from what its
// sources tell, in order, as Sonarr's AggregateQuality: the source and
// resolution told with the most confidence, the first known one until
// then, and the revision told with the most confidence, the highest
// among equals; version 1 when none is.
func AggregateSeriesQuality(augments ...QualityAugment) ParsedQuality {
	source, sourceConfidence := SeriesSourceUnknown, ConfidenceDefault
	resolution, resolutionConfidence := 0, ConfidenceDefault
	revision, revisionConfidence := Revision{Version: 1}, ConfidenceDefault
	for _, a := range augments {
		if source == SeriesSourceUnknown || (a.SourceConfidence > sourceConfidence && a.Source != SeriesSourceUnknown) {
			source, sourceConfidence = a.Source, a.SourceConfidence
		}
		if resolution == 0 || (a.ResolutionConfidence > resolutionConfidence && a.Resolution > 0) {
			resolution, resolutionConfidence = a.Resolution, a.ResolutionConfidence
		}
		if a.Revision != nil {
			if a.RevisionConfidence > revisionConfidence ||
				(a.RevisionConfidence == revisionConfidence && a.Revision.Compare(revision) > 0) {
				revision, revisionConfidence = *a.Revision, a.RevisionConfidence
			}
		}
	}
	out := ParsedQuality{Quality: FindSeriesQuality(source, resolution), Revision: revision}
	switch resolutionConfidence {
	case ConfidenceMediaInfo:
		out.ResolutionDetection = DetectedMediaInfo
	case ConfidenceFallback:
		out.ResolutionDetection = DetectedExtension
	default:
		out.ResolutionDetection = DetectedName
	}
	out.SourceDetection = DetectedName
	if sourceConfidence == ConfidenceFallback {
		out.SourceDetection = DetectedExtension
	}
	if revisionConfidence == ConfidenceTag {
		out.RevisionDetection = DetectedName
	}
	return out
}

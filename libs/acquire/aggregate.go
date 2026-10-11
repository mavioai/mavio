package acquire

import (
	"strings"

	"github.com/mavioai/mavio/libs/release"
)

// Confidence is how sure a source of an imported file's quality is of a
// part of it, as Radarr's Confidence, in increasing order.
type Confidence int

// Confidences.
const (
	// ConfidenceFallback is a part guessed, such as from an extension.
	ConfidenceFallback Confidence = iota
	// ConfidenceDefault is a part no source has stated.
	ConfidenceDefault
	// ConfidenceTag is a part a name states.
	ConfidenceTag
	// ConfidenceMediaInfo is a part read from the file itself.
	ConfidenceMediaInfo
)

// QualityAugment is what one source, such as the file name, the folder
// name, the release name or the media info, says of an imported file's
// quality, as Radarr's AugmentQualityResult.
type QualityAugment struct {
	Source               release.Source
	SourceConfidence     Confidence
	Resolution           int
	ResolutionConfidence Confidence
	Modifier             release.Modifier
	ModifierConfidence   Confidence
	// Revision is nil when the source says nothing of it.
	Revision           *release.Revision
	RevisionConfidence Confidence
}

// AugmentParsedQuality returns what a quality parsed from a name says, as
// Radarr's AugmentQualityFromFileName, FromFolder, FromDownloadClientItem
// and FromReleaseName: the parts detected in the name are tags, the
// others fallbacks.
func AugmentParsedQuality(p release.ParsedQuality) *QualityAugment {
	confidence := func(d release.Detection) Confidence {
		if d == release.DetectedName {
			return ConfidenceTag
		}
		return ConfidenceFallback
	}
	revision := p.Revision
	return &QualityAugment{
		Source: p.Quality.Source(), SourceConfidence: confidence(p.SourceDetection),
		Resolution: p.Quality.Resolution(), ResolutionConfidence: confidence(p.ResolutionDetection),
		Modifier: p.Quality.Modifier(), ModifierConfidence: confidence(p.ModifierDetection),
		Revision: &revision, RevisionConfidence: confidence(p.RevisionDetection),
	}
}

// AugmentMediaInfoQuality returns what the media info of a file says of
// its quality, as Radarr's AugmentQualityFromMediaInfo: the resolution
// of its video's width and height, and the source and modifier of the
// title in its container when one is there; nil without a video size.
func AugmentMediaInfoQuality(width, height int, title string) *QualityAugment {
	a := &QualityAugment{SourceConfidence: ConfidenceDefault, ModifierConfidence: ConfidenceDefault, RevisionConfidence: ConfidenceDefault}
	if title = strings.TrimSpace(title); title != "" {
		if p := release.ParseQualityName(title, release.Movie); p.Quality.Source() != release.SourceUnknown && p.SourceDetection == release.DetectedName {
			a.Source, a.SourceConfidence = p.Quality.Source(), ConfidenceMediaInfo
			a.Modifier, a.ModifierConfidence = p.Quality.Modifier(), ConfidenceMediaInfo
		}
	}
	switch {
	case width >= 3200 || height >= 2100:
		a.Resolution = 2160
	case width >= 1800 || height >= 1000:
		a.Resolution = 1080
	case width >= 1200 || height >= 700:
		a.Resolution = 720
	case width >= 1000 || height >= 560:
		a.Resolution = 576
	case width > 0 && height > 0:
		a.Resolution = 480
	default:
		return nil
	}
	a.ResolutionConfidence = ConfidenceMediaInfo
	return a
}

// AggregateQuality combines what the sources say of an imported file's
// quality, in Radarr's order of the file name, the folder name, the
// download, the media info and the release name, as Radarr's
// AggregateQuality: each part is taken from the most confident source
// that knows it, a modifier from the highest, and the revision from the
// most confident source, the highest among equally confident ones. Nil
// augments are skipped.
func AggregateQuality(augments ...*QualityAugment) release.ParsedQuality {
	source, sourceConfidence := release.SourceUnknown, ConfidenceDefault
	resolution, resolutionConfidence := 0, ConfidenceDefault
	modifier, modifierConfidence := release.ModifierNone, ConfidenceDefault
	revision, revisionConfidence := release.Revision{Version: 1}, ConfidenceDefault
	for _, a := range augments {
		if a == nil {
			continue
		}
		if source == release.SourceUnknown || (a.SourceConfidence > sourceConfidence && a.Source != release.SourceUnknown) {
			source, sourceConfidence = a.Source, a.SourceConfidence
		}
		if resolution == 0 || (a.ResolutionConfidence > resolutionConfidence && a.Resolution > 0) {
			resolution, resolutionConfidence = a.Resolution, a.ResolutionConfidence
		}
		if a.Modifier > modifier || (a.ModifierConfidence > modifierConfidence && a.Modifier != release.ModifierNone) {
			modifier, modifierConfidence = a.Modifier, a.ModifierConfidence
		}
		if a.Revision != nil {
			switch {
			case a.RevisionConfidence > revisionConfidence:
				revision, revisionConfidence = *a.Revision, a.RevisionConfidence
			case a.RevisionConfidence == revisionConfidence && a.Revision.Compare(revision) > 0:
				revision = *a.Revision
			}
		}
	}

	p := release.ParsedQuality{Quality: FindBySourceAndResolution(source, resolution, modifier), Revision: revision}
	switch resolutionConfidence {
	case ConfidenceMediaInfo:
		p.ResolutionDetection = release.DetectedMediaInfo
	case ConfidenceFallback:
		p.ResolutionDetection = release.DetectedExtension
	default:
		p.ResolutionDetection = release.DetectedName
	}
	p.SourceDetection = release.DetectedName
	if sourceConfidence == ConfidenceFallback {
		p.SourceDetection = release.DetectedExtension
	}
	if revisionConfidence == ConfidenceTag {
		p.RevisionDetection = release.DetectedName
	}
	return p
}

// FindBySourceAndResolution returns the quality of a source, resolution
// and modifier, or the nearest one, as Radarr's
// QualityFinder.FindBySourceAndResolution: the quality of the source and
// modifier without a resolution, else that of the resolution and modifier
// with the lowest source not below the source, else Unknown.
func FindBySourceAndResolution(source release.Source, resolution int, modifier release.Modifier) release.Quality {
	var unknownResolution, sameResolution []release.Quality
	for q := release.Unknown; q <= release.RawHD; q++ {
		if q.Source() == source && q.Resolution() == resolution && q.Modifier() == modifier {
			return q
		}
		if q != release.Unknown && q.Source() == source && q.Resolution() == 0 && q.Modifier() == modifier {
			unknownResolution = append(unknownResolution, q)
		}
		if q.Modifier() == modifier && q.Resolution() == resolution {
			sameResolution = append(sameResolution, q)
		}
	}
	if len(unknownResolution) > 0 {
		return unknownResolution[0]
	}
	best := release.Unknown
	for _, q := range sameResolution {
		if q.Source() >= source && (best == release.Unknown || q.Source() < best.Source()) {
			best = q
		}
	}
	return best
}

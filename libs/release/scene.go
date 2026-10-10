package release

import (
	"strings"
)

// SceneTitle returns the scene name of a release, or "" when the title
// might not be one. It prefers false negatives over false positives.
func SceneTitle(title string, kind Kind) string {
	if !strings.Contains(title, ".") || strings.Contains(title, " ") {
		return ""
	}
	if kind == Series {
		p := ParseSeriesTitle(title)
		if p == nil || p.ReleaseGroup == "" || p.Quality.Quality == Unknown || strings.TrimSpace(p.SeriesTitle) == "" {
			return ""
		}
		return title
	}
	p := ParseMovieTitle(title, false)
	if p == nil || p.ReleaseGroup == "" || p.Quality.Quality == Unknown ||
		strings.TrimSpace(p.PrimaryMovieTitle()) == "" || strings.TrimSpace(p.ReleaseTitle) == "" {
		return ""
	}
	return p.ReleaseTitle
}

// IsSceneTitle reports whether title is a scene release name.
func IsSceneTitle(title string, kind Kind) bool { return SceneTitle(title, kind) != "" }

package subtitles

import (
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// ProfileFor resolves the subtitle profile of a movie or series, as
// Bazarr's Radarr and Sonarr sync: the assigned profile, overridden when
// tags are in use by the first profile, in order, whose tag the item has,
// and removed when the item has one of the no-profile tags. It returns
// nil for no profile.
func ProfileFor(profiles []core.SubtitleProfile, assigned core.ID, tags []string, useTags bool, noProfileTags []string) *core.SubtitleProfile {
	hasTag := func(t string) bool {
		return t != "" && slices.ContainsFunc(tags, func(x string) bool { return strings.EqualFold(x, t) })
	}
	id := assigned
	if useTags {
		for _, p := range profiles {
			if hasTag(p.Tag) {
				id = p.ID
				break
			}
		}
		if slices.ContainsFunc(noProfileTags, hasTag) {
			return nil
		}
	}
	if id.IsZero() {
		return nil
	}
	for i := range profiles {
		if profiles[i].ID == id {
			return &profiles[i]
		}
	}
	return nil
}

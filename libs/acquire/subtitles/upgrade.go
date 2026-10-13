package subtitles

import (
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Upgrade is a subtitle download worth replacing with a better one.
type Upgrade struct {
	Event core.SubtitleHistoryEvent
	// Root is the event of the first download the upgrades replace.
	Root core.ID
}

// Upgradable returns the downloads to try upgrading, latest first, as
// Bazarr's get_upgradable_*_subtitles: for each video and language, the
// latest download since the last deletion, if it is within the upgrade
// window and scored more than 3 below the maximum. The actions are
// downloads and upgrades, and manual downloads and uploads when the
// settings upgrade those too.
func Upgradable(events []core.SubtitleHistoryEvent, s *core.SubtitleSettings, now time.Time) []Upgrade {
	if !s.Upgrade {
		return nil
	}
	actions := []core.SubtitleAction{core.SubtitleDownloaded, core.SubtitleUpgraded}
	if s.UpgradeManual {
		actions = append(actions, core.SubtitleManual, core.SubtitleUploaded)
	}
	type key struct {
		path string
		lang core.SubtitleLanguage
	}
	deleted := map[key]time.Time{}
	for _, e := range events {
		k := key{e.VideoPath, e.Language}
		if e.Action == core.SubtitleDeleted && e.Date.After(deleted[k]) {
			deleted[k] = e.Date
		}
	}
	latest := map[key]time.Time{}
	for _, e := range events {
		k := key{e.VideoPath, e.Language}
		if slices.Contains(actions, e.Action) && e.Date.After(deleted[k]) && e.Date.After(latest[k]) {
			latest[k] = e.Date
		}
	}
	since := now.AddDate(0, 0, -s.UpgradeDays)
	var cands []core.SubtitleHistoryEvent
	for _, e := range events {
		k := key{e.VideoPath, e.Language}
		if slices.Contains(actions, e.Action) && e.Date.After(deleted[k]) && e.Date.Equal(latest[k]) &&
			e.Date.After(since) && e.Score < e.MaxScore-3 {
			cands = append(cands, e)
		}
	}
	slices.SortStableFunc(cands, func(a, b core.SubtitleHistoryEvent) int { return b.Date.Compare(a.Date) })
	var out []Upgrade
	for _, e := range cands {
		// An event another candidate upgraded from was already upgraded.
		if slices.ContainsFunc(cands, func(o core.SubtitleHistoryEvent) bool { return o.UpgradedFrom == e.ID }) {
			continue
		}
		root := e.UpgradedFrom
		if root.IsZero() {
			root = e.ID
		}
		out = append(out, Upgrade{Event: e, Root: root})
	}
	return out
}

// StillDesired reports whether a profile still wants subtitles in l: a
// language whose hearing-impaired subtitles are neither required nor
// excluded is wanted of either kind.
func StillDesired(p *core.SubtitleProfile, l core.SubtitleLanguage) bool {
	for _, it := range p.Items {
		if it.Language != l.Language {
			continue
		}
		switch {
		case it.Forced:
			if l.Forced {
				return true
			}
		case l.Forced:
		case it.HI == core.SubtitleHIRequired:
			if l.HearingImpaired {
				return true
			}
		case it.HI == core.SubtitleHIExcluded:
			if !l.HearingImpaired {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// UpgradeLanguage returns the language to search in upgrading subtitles in
// l: hearing-impaired subtitles only if the profile requires them.
func UpgradeLanguage(p *core.SubtitleProfile, l core.SubtitleLanguage) core.SubtitleLanguage {
	if l.HearingImpaired && !slices.ContainsFunc(p.Items, func(it core.SubtitleProfileItem) bool {
		return it.Language == l.Language && it.HI == core.SubtitleHIRequired
	}) {
		l.HearingImpaired = false
	}
	return l
}

// UpgradeMinScore returns the lowest score of a subtitle that upgrades
// the event's.
func UpgradeMinScore(e core.SubtitleHistoryEvent) int { return e.Score + 1 }

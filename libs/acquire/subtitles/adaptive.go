package subtitles

import (
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// SearchActive reports whether to search again for subtitles that failed
// to be found, as Bazarr's adaptive searching: always while the first
// failure is younger than the delay, then once per delta.
func SearchActive(s *core.SubtitleSettings, attempt *core.SubtitleSearchAttempt, now time.Time) bool {
	if !s.AdaptiveSearching || attempt == nil {
		return true
	}
	return attempt.First.Add(s.AdaptiveDelay).After(now) || !attempt.Last.Add(s.AdaptiveDelta).After(now)
}

// RecordAttempt returns the attempt after a failed search at now: the
// first failure is kept, the latest replaced.
func RecordAttempt(attempt *core.SubtitleSearchAttempt, itemID core.ID, l core.SubtitleLanguage, now time.Time) core.SubtitleSearchAttempt {
	if attempt == nil {
		return core.SubtitleSearchAttempt{ItemID: itemID, Language: l, First: now, Last: now}
	}
	a := *attempt
	a.Last = now
	return a
}

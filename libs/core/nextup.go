package core

import (
	"cmp"
	"math"
	"slices"
)

// CompareAiredOrder orders episodes as they aired, as Jellyfin's
// AiredEpisodeOrderComparer does: regular episodes by season and episode
// number, then by premiere date; specials (season 0) by where they air,
// given by AirsBeforeSeasonNumber, AirsAfterSeasonNumber and
// AirsBeforeEpisodeNumber. Other kinds of items sort after episodes. It
// returns -1, 0 or +1.
func CompareAiredOrder(x, y *Item) int {
	xEpisode, yEpisode := x.Kind == KindEpisode, y.Kind == KindEpisode
	switch {
	case !xEpisode && !yEpisode:
		return 0
	case !xEpisode:
		return 1
	case !yEpisode:
		return -1
	}
	xSpecial, ySpecial := intOr(x.ParentIndexNumber, -1) == 0, intOr(y.ParentIndexNumber, -1) == 0
	switch {
	case xSpecial && ySpecial:
		return cmp.Compare(specialOrder(x), specialOrder(y))
	case !xSpecial && !ySpecial:
		c := cmp.Compare(intOr(x.ParentIndexNumber, -1)*1000+intOr(x.IndexNumber, -1),
			intOr(y.ParentIndexNumber, -1)*1000+intOr(y.IndexNumber, -1))
		if c == 0 && x.PremiereDate != nil && y.PremiereDate != nil {
			c = x.PremiereDate.Compare(*y.PremiereDate)
		}
		return c
	case !xSpecial:
		return compareToSpecial(x, y)
	default:
		return -compareToSpecial(y, x)
	}
}

// compareToSpecial compares a regular episode with a special, following
// TheTVDB's placement of specials.
func compareToSpecial(episode, special *Item) int {
	season := intOr(episode.ParentIndexNumber, -1)
	specialSeason := intOr(special.AirsAfterSeasonNumber, intOr(special.AirsBeforeSeasonNumber, -1))
	if season != specialSeason {
		return cmp.Compare(season, specialSeason)
	}
	if special.AirsAfterSeasonNumber != nil {
		return -1 // the special airs after the season
	}
	if special.AirsBeforeEpisodeNumber == nil {
		return 1 // the special airs before the season
	}
	if episode.IndexNumber == nil {
		return 0
	}
	if *episode.IndexNumber == *special.AirsBeforeEpisodeNumber {
		return 1 // the special airs before the episode
	}
	return cmp.Compare(*episode.IndexNumber, *special.AirsBeforeEpisodeNumber)
}

// specialOrder orders specials by season, after or before it, the episode
// they air before, and their own number.
func specialOrder(it *Item) int64 {
	v := int64(intOr(it.AirsAfterSeasonNumber, intOr(it.AirsBeforeSeasonNumber, 0))) * 1_000_000_000
	if it.AirsAfterSeasonNumber != nil {
		v += 1_000_000
	}
	v += int64(intOr(it.AirsBeforeEpisodeNumber, 0)) * 1000
	return v + int64(intOr(it.IndexNumber, 0))
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// NextEpisode picks the episode of a series to watch next, as Jellyfin's
// Next Up does. episodes are the series' present episodes the user may
// access, data the user's state for them.
//
// The next episode is the first unplayed regular episode after the last
// played one by season and episode number, or the first of all when none
// was played. Specials placed with AirsBeforeSeasonNumber or
// AirsAfterSeasonNumber come in between in aired order. Unless
// includeResumable is set, an episode with a resume position is not
// offered: it belongs to continue watching. NextEpisode returns nil when
// the series has nothing to watch next.
func NextEpisode(episodes []Item, data map[ID]UserData, includeResumable bool) *Item {
	played := func(it *Item) bool { return data[it.ID].Played }
	number := func(p *int) int { return intOr(p, math.MinInt) }
	byNumber := func(a, b *Item) int {
		return cmp.Or(cmp.Compare(number(a.ParentIndexNumber), number(b.ParentIndexNumber)),
			cmp.Compare(number(a.IndexNumber), number(b.IndexNumber)))
	}

	var last, next *Item
	var specials []*Item
	for i := range episodes {
		ep := &episodes[i]
		switch {
		case intOr(ep.ParentIndexNumber, -1) == 0:
			if ep.AirsBeforeSeasonNumber != nil || ep.AirsAfterSeasonNumber != nil {
				specials = append(specials, ep)
			}
		case played(ep) && (last == nil || byNumber(ep, last) > 0):
			last = ep
		}
	}
	for i := range episodes {
		ep := &episodes[i]
		if intOr(ep.ParentIndexNumber, -1) == 0 || played(ep) {
			continue
		}
		// After the last played episode, which needs both numbers to place
		// it: in a later season, or later in its season.
		if last != nil && last.ParentIndexNumber != nil && last.IndexNumber != nil {
			season, number := *last.ParentIndexNumber, *last.IndexNumber
			after := ep.ParentIndexNumber != nil && (*ep.ParentIndexNumber > season ||
				*ep.ParentIndexNumber == season && ep.IndexNumber != nil && *ep.IndexNumber > number)
			if !after {
				continue
			}
		}
		if next == nil || byNumber(ep, next) < 0 {
			next = ep
		}
	}

	if len(specials) > 0 {
		considered := specials
		if last != nil {
			considered = append(considered, last)
		}
		if next != nil {
			considered = append(considered, next)
		}
		slices.SortStableFunc(considered, CompareAiredOrder)
		if last != nil {
			i := slices.Index(considered, last)
			considered = considered[i+1:]
		}
		next = nil
		for _, ep := range considered {
			if !played(ep) {
				next = ep
				break
			}
		}
	}

	if next != nil && !includeResumable && data[next.ID].Position > 0 {
		return nil
	}
	return next
}

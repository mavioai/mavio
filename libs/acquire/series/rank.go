package series

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
)

// Compare orders two decisions on candidates for the same series, the
// better last, as Sonarr's DownloadDecisionComparer: by quality, custom
// format score, preferred protocol, season packs then episode count,
// lowest episode number, indexer priority, seeders and peers of
// torrents, age of usenet releases, then size.
func (e *Env) Compare(x, y *Decision) int {
	a, b := x.Candidate, y.Candidate
	comparers := []func(a, b *Candidate) int{
		e.compareQuality,
		func(a, b *Candidate) int { return cmp.Compare(a.FormatScore, b.FormatScore) },
		e.compareProtocol,
		compareEpisodeCount,
		func(a, b *Candidate) int { return -cmp.Compare(minEpisodeNumber(a), minEpisodeNumber(b)) },
		func(a, b *Candidate) int { return -cmp.Compare(a.Release.IndexerPriority, b.Release.IndexerPriority) },
		comparePeers,
		e.compareAge,
		e.compareSize,
	}
	for _, c := range comparers {
		if r := c(a, b); r != 0 {
			return r
		}
	}
	return 0
}

func (e *Env) compareQuality(a, b *Candidate) int {
	p := a.Series.Profile
	if r := p.Index(a.Quality.Quality, false).Compare(p.Index(b.Quality.Quality, false), true); r != 0 || e.Config.Propers == acquire.PropersDoNotPrefer {
		return r
	}
	return a.Quality.Revision.Compare(b.Quality.Revision)
}

func (e *Env) compareProtocol(a, b *Candidate) int {
	preferred := func(c *Candidate) int {
		if c.Release.Protocol == e.delayProfile(c.Series.Tags).PreferredProtocol {
			return 1
		}
		return 0
	}
	return cmp.Compare(preferred(a), preferred(b))
}

// compareEpisodeCount prefers season packs, then more episodes of anime
// and fewer of other series.
func compareEpisodeCount(a, b *Candidate) int {
	pack := func(c *Candidate) int {
		if c.Parsed.FullSeason {
			return 1
		}
		return 0
	}
	if r := cmp.Compare(pack(a), pack(b)); r != 0 {
		return r
	}
	if a.Series.Type == Anime && b.Series.Type == Anime {
		return cmp.Compare(len(a.Episodes), len(b.Episodes))
	}
	return -cmp.Compare(len(a.Episodes), len(b.Episodes))
}

// minEpisodeNumber is the lowest episode number of a candidate, 0 when
// it has no episodes.
func minEpisodeNumber(c *Candidate) int {
	if len(c.Episodes) == 0 {
		return 0
	}
	n := c.Episodes[0].EpisodeNumber
	for _, ep := range c.Episodes[1:] {
		n = min(n, ep.EpisodeNumber)
	}
	return n
}

// logScore is the rounded order of magnitude of a count, 0 when unknown.
func logScore(n *int) float64 {
	if n == nil || *n <= 0 {
		return 0
	}
	return math.RoundToEven(math.Log10(float64(*n)))
}

func comparePeers(a, b *Candidate) int {
	if a.Release.Protocol != acquire.ProtocolTorrent || b.Release.Protocol != acquire.ProtocolTorrent {
		return 0
	}
	if r := cmp.Compare(logScore(a.Release.Seeders), logScore(b.Release.Seeders)); r != 0 {
		return r
	}
	return cmp.Compare(logScore(a.Release.Peers), logScore(b.Release.Peers))
}

func (e *Env) compareAge(a, b *Candidate) int {
	if a.Release.Protocol != acquire.ProtocolUsenet || b.Release.Protocol != acquire.ProtocolUsenet {
		return 0
	}
	now := e.now()
	score := func(c *Candidate) int {
		age := c.Release.Age(now)
		switch {
		case age < time.Hour:
			return 1000
		case age <= 24*time.Hour:
			return 100
		case int(age/(24*time.Hour)) <= 7:
			return 10
		}
		return 1
	}
	return cmp.Compare(score(a), score(b))
}

func (e *Env) compareSize(a, b *Candidate) int {
	score := func(c *Candidate) int64 {
		preferred := e.definition(c.Quality.Quality).PreferredSize
		if runtime := c.Series.Runtime; preferred > 0 && runtime > 0 {
			// Closest to the preferred size.
			d := roundTo(c.Release.Size-int64(runtime)*megabytes(preferred), megabytes(200))
			if d < 0 {
				d = -d
			}
			return -d
		}
		// Without a preferred size, the largest.
		return roundTo(c.Release.Size, megabytes(200))
	}
	return cmp.Compare(score(a), score(b))
}

// Prioritize orders decisions best first within each series, series in
// the order they first appear, and decisions on candidates matching no
// series last, as Sonarr's DownloadDecisionPriorizationService.
func (e *Env) Prioritize(decisions []Decision) []Decision {
	var order []core.ID
	groups := map[core.ID][]Decision{}
	var unknown []Decision
	for _, d := range decisions {
		if d.Candidate.Series == nil {
			unknown = append(unknown, d)
			continue
		}
		id := d.Candidate.Series.ID
		if _, ok := groups[id]; !ok {
			order = append(order, id)
		}
		groups[id] = append(groups[id], d)
	}
	out := make([]Decision, 0, len(decisions))
	for _, id := range order {
		g := groups[id]
		slices.SortStableFunc(g, func(x, y Decision) int { return -e.Compare(&x, &y) })
		out = append(out, g...)
	}
	return append(out, unknown...)
}

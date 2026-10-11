package acquire

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Compare orders two decisions on candidates for the same target, the
// better last, as Radarr's DownloadDecisionComparer: by quality, custom
// format score, preferred protocol, indexer priority, indexer flags,
// seeders and peers of torrents, age of usenet releases, then size.
func (e *Env) Compare(x, y *Decision) int {
	a, b := x.Candidate, y.Candidate
	comparers := []func(a, b *Candidate) int{
		e.compareQuality,
		func(a, b *Candidate) int { return cmp.Compare(a.FormatScore, b.FormatScore) },
		e.compareProtocol,
		func(a, b *Candidate) int { return -cmp.Compare(a.Release.IndexerPriority, b.Release.IndexerPriority) },
		e.compareFlags,
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
	p := a.Target.Profile
	if r := p.Index(a.Quality.Quality, false).Compare(p.Index(b.Quality.Quality, false), true); r != 0 || e.Config.Propers == PropersDoNotPrefer {
		return r
	}
	return a.Quality.Revision.Compare(b.Quality.Revision)
}

func (e *Env) compareProtocol(a, b *Candidate) int {
	preferred := func(c *Candidate) int {
		if c.Release.Protocol == e.delayProfile(c.Target.Tags).PreferredProtocol {
			return 1
		}
		return 0
	}
	return cmp.Compare(preferred(a), preferred(b))
}

// flagScore scores the indexer flags of a release.
func flagScore(r *Release) int {
	score := 0
	for _, f := range []string{FlagDoubleUpload, FlagFreeleech, FlagPTPApproved, FlagPTPGolden, FlagInternal} {
		if r.HasFlag(f) {
			score += 2
		}
	}
	if r.HasFlag(FlagHalfleech) {
		score++
	}
	return score
}

func (e *Env) compareFlags(a, b *Candidate) int {
	if !e.Config.PreferIndexerFlags {
		return 0
	}
	return cmp.Compare(flagScore(a.Release), flagScore(b.Release))
}

// logScore is the rounded order of magnitude of a count, 0 when unknown.
func logScore(n *int) float64 {
	if n == nil || *n <= 0 {
		return 0
	}
	return math.RoundToEven(math.Log10(float64(*n)))
}

func comparePeers(a, b *Candidate) int {
	if a.Release.Protocol != ProtocolTorrent || b.Release.Protocol != ProtocolTorrent {
		return 0
	}
	if r := cmp.Compare(logScore(a.Release.Seeders), logScore(b.Release.Seeders)); r != 0 {
		return r
	}
	return cmp.Compare(logScore(a.Release.Peers), logScore(b.Release.Peers))
}

func (e *Env) compareAge(a, b *Candidate) int {
	if a.Release.Protocol != ProtocolUsenet || b.Release.Protocol != ProtocolUsenet {
		return 0
	}
	now := e.now()
	score := func(c *Candidate) float64 {
		age := c.Release.Age(now)
		days := int(age / (24 * time.Hour))
		switch {
		case age < time.Hour:
			return 1000
		case age <= 24*time.Hour:
			return 100
		case days <= 7:
			return 10
		}
		return -math.RoundToEven(math.Log10(float64(days)))
	}
	return cmp.Compare(score(a), score(b))
}

func (e *Env) compareSize(a, b *Candidate) int {
	score := func(c *Candidate) int64 {
		preferred := e.definition(c.Quality.Quality).PreferredSize
		runtime := 0
		for _, it := range c.Items {
			runtime += it.Runtime
		}
		if preferred > 0 && runtime > 0 {
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

// Prioritize orders decisions best first within each target, targets in
// the order they first appear, and decisions on candidates matching no
// target last, as Radarr's DownloadDecisionPriorizationService.
func (e *Env) Prioritize(decisions []Decision) []Decision {
	var order []core.ID
	groups := map[core.ID][]Decision{}
	var unknown []Decision
	for _, d := range decisions {
		if d.Candidate.Target == nil {
			unknown = append(unknown, d)
			continue
		}
		id := d.Candidate.Target.ID
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

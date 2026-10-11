package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
)

// SearchMovie searches the indexers for a wanted movie and grabs the best
// release, keeping the delayed ones pending, as Radarr's
// MovieSearchService. An unmonitored movie is searched for only when a
// user asks.
func (r *Runtime) SearchMovie(ctx context.Context, itemID core.ID, userInvoked bool) error {
	rn, err := r.newRun(ctx)
	if err != nil {
		return err
	}
	m, err := rn.movie(ctx, itemID)
	if err != nil {
		return err
	}
	if !m.item.Monitored && !userInvoked {
		r.logger().DebugContext(ctx, "movie is not monitored; not searching", "item", itemID)
		return nil
	}
	res, err := r.searcher().Search(ctx, releases.Search{ItemID: itemID})
	if err != nil {
		return err
	}
	for _, f := range res.Failures {
		r.logger().WarnContext(ctx, "indexer failed", "plugin", f.Provider, "indexer", f.Indexer, "err", f.Message)
	}
	b := newBatch()
	for _, rel := range res.Releases {
		b.add(FromSearch(rel))
	}
	s := &acquire.Search{TargetID: itemID, ItemIDs: []core.ID{itemID}, UserInvoked: userInvoked}
	decisions := rn.engine.DecideMovies(ctx, b.releases, s, false, rn.findMovie(b))
	processed, err := r.process(ctx, rn, b, decisions)
	if err != nil {
		return err
	}
	r.logger().InfoContext(ctx, "movie search completed", "item", itemID, "releases", len(b.releases),
		"grabbed", len(processed.grabbed), "pending", len(processed.pending))
	w := m.wanted
	w.LastSearchedAt = r.now()
	return r.Store.Wanted().Put(ctx, &w)
}

// SyncRSS reads the indexers' recent releases and decides on them with
// the pending releases, for every wanted movie, as Radarr's
// RssSyncService, and for the episodes of every wanted series, as
// Sonarr's; pending releases rejected for good are dropped.
func (r *Runtime) SyncRSS(ctx context.Context) error {
	rn, err := r.newRun(ctx)
	if err != nil {
		return err
	}
	res, err := r.searcher().Search(ctx, releases.Search{})
	if err != nil {
		return err
	}
	for _, f := range res.Failures {
		r.logger().WarnContext(ctx, "indexer failed", "plugin", f.Provider, "indexer", f.Indexer, "err", f.Message)
	}
	// Movies and episodes are decided on apart, each with its pending
	// releases.
	b, eb := newBatch(), newBatch()
	for _, rel := range res.Releases {
		b.add(FromSearch(rel))
		eb.add(FromSearch(rel))
	}
	pending, err := r.pending(ctx, core.NilID)
	if err != nil {
		return err
	}
	for _, p := range pending {
		// Releases of indexers left out after failing wait, as Radarr's
		// FilterBlockedIndexers.
		if ix, ok := rn.indexer(ctx, p.release.IndexerRef()); ok && ix.HasDisabledUntil() && ix.GetDisabledUntil().AsTime().After(r.now()) {
			continue
		}
		if isSeriesPending(p.PendingRelease) {
			eb.targets[eb.add(p.release)] = p.ItemID
		} else {
			b.targets[b.add(p.release)] = p.ItemID
		}
	}
	decisions := rn.engine.DecideMovies(ctx, b.releases, nil, false, rn.findMovie(b))
	processed, err := r.process(ctx, rn, b, decisions)
	if err != nil {
		return err
	}
	for _, d := range processed.rejected {
		if d.Candidate.Target == nil {
			continue
		}
		rel := b.of[d.Candidate.Release]
		if err := r.removePending(ctx, d.Candidate.Target.ID, func(p *Release) bool { return sameRelease(p, rel) }); err != nil {
			return err
		}
	}
	find, special, err := rn.findEpisodes(ctx, eb)
	if err != nil {
		return err
	}
	edecisions := rn.sengine.DecideEpisodes(ctx, eb.releases, nil, false, find, special)
	eprocessed, err := r.processEpisodes(ctx, rn, eb, edecisions)
	if err != nil {
		return err
	}
	for _, d := range eprocessed.rejected {
		if d.Candidate == nil || d.Candidate.Series == nil {
			continue
		}
		rel := eb.of[d.Candidate.Release]
		if err := r.removePending(ctx, d.Candidate.Series.ID, func(p *Release) bool { return sameRelease(p, rel) }); err != nil {
			return err
		}
	}
	r.logger().InfoContext(ctx, "RSS sync completed", "releases", len(b.releases),
		"grabbed", len(processed.grabbed)+len(eprocessed.grabbed), "pending", len(processed.pending)+len(eprocessed.pending))
	return nil
}

// isSeriesPending reports whether a pending release is of episodes.
func isSeriesPending(p core.PendingRelease) bool {
	return len(p.ItemIDs) > 1 || (len(p.ItemIDs) == 1 && p.ItemIDs[0] != p.ItemID)
}

// processed is what processing decisions did.
type processed struct {
	grabbed, pending, rejected []acquire.Decision
}

// grabResult is how a grab ended, as Radarr's ProcessedDecisionResult.
type grabResult int

const (
	grabGrabbed grabResult = iota
	grabRejected
	grabFailed
	grabSkipped
)

// process grabs the best approved release of each movie and keeps the
// delayed ones pending, as Radarr's ProcessDownloadDecisions: when a
// protocol's download client fails, that protocol's later releases are
// kept pending too.
func (r *Runtime) process(ctx context.Context, rn *run, b *batch, decisions []acquire.Decision) (processed, error) {
	var out processed
	var qualified []acquire.Decision
	for _, d := range decisions {
		if d.Rejected() {
			out.rejected = append(out.rejected, d)
		}
		if (d.Approved() || d.TemporarilyRejected()) && d.Candidate.Target != nil {
			qualified = append(qualified, d)
		}
	}
	failed := map[acquire.Protocol]bool{}
	var adds []pendingAdd
	done := func(list []acquire.Decision, d acquire.Decision) bool {
		return slices.ContainsFunc(list, func(o acquire.Decision) bool { return o.Candidate.Target.ID == d.Candidate.Target.ID })
	}
	keep := func(d acquire.Decision, reason core.PendingReason) {
		if done(out.grabbed, d) || done(out.pending, d) {
			reason = core.PendingFallback
		}
		adds = append(adds, pendingAdd{itemID: d.Candidate.Target.ID, rel: b.of[d.Candidate.Release], reason: reason})
		out.pending = append(out.pending, d)
	}
	for _, d := range rn.env.Prioritize(qualified) {
		if done(out.grabbed, d) {
			continue
		}
		if d.TemporarilyRejected() {
			keep(d, core.PendingDelay)
			continue
		}
		protocol := d.Candidate.Release.Protocol
		if failed[protocol] {
			keep(d, core.PendingClientUnavailable)
			continue
		}
		switch r.grabDecision(ctx, rn, b, d) {
		case grabGrabbed:
			out.grabbed = append(out.grabbed, d)
		case grabRejected:
			out.rejected = append(out.rejected, d)
		case grabFailed:
			keep(d, core.PendingClientUnavailable)
			failed[protocol] = true
		}
	}
	if len(adds) > 0 {
		if err := r.addPending(ctx, adds); err != nil {
			return out, err
		}
	}
	return out, nil
}

// grabDecision grabs an approved release and tells how it went.
func (r *Runtime) grabDecision(ctx context.Context, rn *run, b *batch, d acquire.Decision) grabResult {
	c := d.Candidate
	m, err := rn.movie(ctx, c.Target.ID)
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't grab release", "release", c.Release.Title, "err", err)
		return grabSkipped
	}
	_, err = r.grab(ctx, rn, grabRequest{movie: m, rel: b.of[c.Release], candidate: c})
	switch {
	case err == nil:
		return grabGrabbed
	case errors.Is(err, errReleaseUnavailable):
		r.logger().WarnContext(ctx, "release not available", "release", c.Release.Title, "indexer", c.Release.Indexer, "err", err)
		return grabRejected
	case errors.Is(err, errClientUnavailable):
		r.logger().DebugContext(ctx, "download client unavailable; keeping release pending", "release", c.Release.Title, "err", err)
		return grabFailed
	}
	r.logger().WarnContext(ctx, "couldn't add release to the download queue", "release", c.Release.Title, "indexer", c.Release.Indexer, "err", err)
	return grabSkipped
}

// Decided is a release with the decision on it.
type Decided struct {
	Release    releases.Release
	Rejections []acquire.Rejection
	Formats    []string
	Score      int
	// EpisodeIDs are the episodes an episode release maps to.
	EpisodeIDs []core.ID
}

// Decide decides on the releases a search found for a wanted movie, or
// for a wanted series, a season or an episode of it, as for an
// interactive search, and returns them best first, the rejected last.
// An item that is not of a monitored wanted movie or series fails with
// ErrNotWanted.
func (r *Runtime) Decide(ctx context.Context, itemID core.ID, found []releases.Release) ([]Decided, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return nil, err
	}
	it, err := r.Store.Items().Get(ctx, itemID)
	if errors.Is(err, core.ErrNotFound) {
		return nil, ErrNotWanted
	}
	if err != nil {
		return nil, err
	}
	if isEpisodeKind(it.Kind) {
		return r.decideEpisodes(ctx, rn, it, found)
	}
	m, err := rn.movie(ctx, itemID)
	if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
		return nil, ErrNotWanted
	}
	if err != nil {
		return nil, err
	}
	if !m.item.Monitored {
		return nil, ErrNotWanted
	}
	b := newBatch()
	index := map[*acquire.Release]int{}
	for i, rel := range found {
		index[b.add(FromSearch(rel))] = i
	}
	s := &acquire.Search{TargetID: itemID, ItemIDs: []core.ID{itemID}, UserInvoked: true, Interactive: true}
	decisions := rn.env.Prioritize(rn.engine.DecideMovies(ctx, b.releases, s, false, rn.findMovie(b)))
	class := func(d acquire.Decision) int {
		switch {
		case d.Approved():
			return 0
		case d.TemporarilyRejected():
			return 1
		}
		return 2
	}
	slices.SortStableFunc(decisions, func(x, y acquire.Decision) int { return cmp.Compare(class(x), class(y)) })
	out := make([]Decided, 0, len(found))
	seen := map[int]bool{}
	for _, d := range decisions {
		i := index[d.Candidate.Release]
		seen[i] = true
		dd := Decided{Release: found[i], Rejections: d.Rejections, Score: d.Candidate.FormatScore}
		for _, f := range d.Candidate.Formats {
			dd.Formats = append(dd.Formats, f.Name)
		}
		out = append(out, dd)
	}
	// Releases whose titles do not parse are left out of the decisions
	// of searches only when they were not searched for; keep any left.
	for i, rel := range found {
		if !seen[i] {
			out = append(out, Decided{Release: rel, Rejections: []acquire.Rejection{{Reason: acquire.ReasonUnableToParse, Message: "Unable to parse release"}}})
		}
	}
	return out, nil
}

// ErrNotWanted is returned for items that are not of wanted movies or
// series.
var ErrNotWanted = fmt.Errorf("%w: not wanted", core.ErrNotFound)

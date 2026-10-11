package series

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// The reasons of rejections only episode releases get. Unknown series,
// unmonitored series and releases of another series than searched for
// are rejected with acquire.ReasonUnknownTarget,
// acquire.ReasonNotMonitored and acquire.ReasonWrongTarget.
const (
	ReasonUnknownEpisode              acquire.Reason = "unknownEpisode"
	ReasonMatchesAnotherSeries        acquire.Reason = "matchesAnotherSeries"
	ReasonEpisodeNotMonitored         acquire.Reason = "episodeNotMonitored"
	ReasonWrongEpisode                acquire.Reason = "wrongEpisode"
	ReasonWrongSeason                 acquire.Reason = "wrongSeason"
	ReasonFullSeason                  acquire.Reason = "fullSeason"
	ReasonFullSeasonNotAired          acquire.Reason = "fullSeasonNotAired"
	ReasonMultiSeason                 acquire.Reason = "multiSeason"
	ReasonSplitEpisode                acquire.Reason = "splitEpisode"
	ReasonUnknownReleaseGroup         acquire.Reason = "unknownReleaseGroup"
	ReasonReleaseGroupDoesNotMatch    acquire.Reason = "releaseGroupDoesNotMatch"
	ReasonExistingFileHasMoreEpisodes acquire.Reason = "existingFileHasMoreEpisodes"
	ReasonAmbiguousNumbering          acquire.Reason = "ambiguousNumbering"
	ReasonNotSeasonPack               acquire.Reason = "notSeasonPack"
)

// Spec is a check an episode release must pass to be grabbed, as
// Sonarr's IDownloadDecisionEngineSpecification.
type Spec interface {
	// Name names the check in errors.
	Name() string
	Priority() acquire.Priority
	// Temporary reports whether the check's rejections may pass later.
	Temporary() bool
	// Check returns why the candidate fails, or nil. s is nil for RSS
	// syncs and pushed releases.
	Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*acquire.Rejection, error)
}

// Root returns a root specification as one for episodes: it checks the
// embedded candidate, with the search's series and episodes as the
// target and items searched for.
func Root(spec acquire.Spec) Spec { return rootSpec{spec: spec} }

type rootSpec struct{ spec acquire.Spec }

func (r rootSpec) Name() string               { return r.spec.Name() }
func (r rootSpec) Priority() acquire.Priority { return r.spec.Priority() }
func (r rootSpec) Temporary() bool            { return r.spec.Temporary() }
func (r rootSpec) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	return r.spec.Check(ctx, e.Env, c.Candidate, s.root())
}

// EpisodePorts are what the episode decisions read from the rest of the
// server besides the root Ports.
type EpisodePorts interface {
	// EpisodesBySeason returns the episodes of a season, in order.
	EpisodesBySeason(ctx context.Context, seriesID core.ID, season int) ([]*Episode, error)
	// EpisodeIDsInFile returns the IDs of the episodes a file holds.
	EpisodeIDsInFile(ctx context.Context, fileID core.ID) ([]core.ID, error)
	// FileExists reports whether an episode file of the series is on
	// disk.
	FileExists(ctx context.Context, s *Series, f *acquire.File) (bool, error)
	// Pending returns the releases kept pending for any of the episodes
	// of the series.
	Pending(ctx context.Context, seriesID core.ID, episodeIDs []core.ID) ([]acquire.Release, error)
}

// Env is what episode decisions are made with.
type Env struct {
	*acquire.Env
	Episodes EpisodePorts
	// AutoUnmonitorPreviouslyDownloaded is set when episodes whose files
	// are deleted are no longer monitored.
	AutoUnmonitorPreviouslyDownloaded bool
}

func (e *Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

func (e *Env) upgrades() acquire.Upgrades { return acquire.Upgrades{Propers: e.Config.Propers} }

// definition returns the definition of a quality, or a zero one.
func (e *Env) definition(q release.Quality) acquire.Definition {
	for _, d := range e.Definitions {
		if d.Quality == q {
			return d
		}
	}
	return acquire.Definition{Quality: q}
}

// delayProfile returns the delay profile for tags.
func (e *Env) delayProfile(tags []string) *acquire.DelayProfile {
	if p := acquire.BestDelayProfile(e.DelayProfiles, tags); p != nil {
		return p
	}
	return &acquire.DelayProfile{EnableUsenet: true, EnableTorrent: true, PreferredProtocol: acquire.ProtocolUsenet}
}

// Decision is the decision on an episode release, as Sonarr's
// DownloadDecision.
type Decision struct {
	Candidate  *Candidate
	Rejections []acquire.Rejection
}

// Approved reports whether nothing rejected the release.
func (d *Decision) Approved() bool { return len(d.Rejections) == 0 }

// TemporarilyRejected reports whether the release was rejected, only
// by checks that may pass later.
func (d *Decision) TemporarilyRejected() bool {
	return len(d.Rejections) > 0 && !slices.ContainsFunc(d.Rejections, func(r acquire.Rejection) bool { return !r.Temporary })
}

// Rejected reports whether a check rejected the release for good.
func (d *Decision) Rejected() bool {
	return slices.ContainsFunc(d.Rejections, func(r acquire.Rejection) bool { return !r.Temporary })
}

// Engine makes decisions on episode candidates with its
// specifications.
type Engine struct {
	Env   *Env
	Specs []Spec
}

// Evaluate runs the specifications on a candidate by priority, stopping
// after the first priority that rejects it. A specification that fails
// rejects the candidate with acquire.ReasonDecisionError.
func (e *Engine) Evaluate(ctx context.Context, c *Candidate, s *Search) Decision {
	specs := slices.Clone(e.Specs)
	slices.SortStableFunc(specs, func(a, b Spec) int { return cmp.Compare(a.Priority(), b.Priority()) })
	var rejections []acquire.Rejection
	for i := 0; i < len(specs); {
		j := i
		for j < len(specs) && specs[j].Priority() == specs[i].Priority() {
			j++
		}
		for _, spec := range specs[i:j] {
			r, err := spec.Check(ctx, e.Env, c, s)
			switch {
			case err != nil:
				rejections = append(rejections, acquire.Rejection{Reason: acquire.ReasonDecisionError, Message: fmt.Sprintf("%s: %v", spec.Name(), err)})
			case r != nil:
				r.Temporary = spec.Temporary()
				rejections = append(rejections, *r)
			}
		}
		if len(rejections) > 0 {
			break
		}
		i = j
	}
	return Decision{Candidate: c, Rejections: rejections}
}

// reject returns a rejection with a formatted message.
func reject(reason acquire.Reason, format string, args ...any) *acquire.Rejection {
	return &acquire.Rejection{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

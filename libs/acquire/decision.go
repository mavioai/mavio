package acquire

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// Reason is why a release was rejected, as Radarr's
// DownloadRejectionReason.
type Reason string

// The rejection reasons.
const (
	ReasonUnknown                           Reason = "unknown"
	ReasonUnknownTarget                     Reason = "unknownTarget"
	ReasonUnableToParse                     Reason = "unableToParse"
	ReasonError                             Reason = "error"
	ReasonDecisionError                     Reason = "decisionError"
	ReasonAvailability                      Reason = "availability"
	ReasonMinimumAgeDelay                   Reason = "minimumAgeDelay"
	ReasonMinimumAgeDelayPushed             Reason = "minimumAgeDelayPushed"
	ReasonNotMonitored                      Reason = "notMonitored"
	ReasonHistoryRecentCutoffMet            Reason = "historyRecentCutoffMet"
	ReasonHistoryCdhDisabledCutoffMet       Reason = "historyCdhDisabledCutoffMet"
	ReasonHistoryHigherPreference           Reason = "historyHigherPreference"
	ReasonHistoryHigherRevision             Reason = "historyHigherRevision"
	ReasonHistoryCutoffMet                  Reason = "historyCutoffMet"
	ReasonHistoryCustomFormatCutoffMet      Reason = "historyCustomFormatCutoffMet"
	ReasonHistoryCustomFormatScore          Reason = "historyCustomFormatScore"
	ReasonHistoryCustomFormatScoreIncrement Reason = "historyCustomFormatScoreIncrement"
	ReasonHistoryUpgradesNotAllowed         Reason = "historyUpgradesNotAllowed"
	ReasonNoMatchingTag                     Reason = "noMatchingTag"
	ReasonPropersDisabled                   Reason = "propersDisabled"
	ReasonProperForOldFile                  Reason = "properForOldFile"
	ReasonWrongTarget                       Reason = "wrongTarget"
	ReasonUnknownRuntime                    Reason = "unknownRuntime"
	ReasonBelowMinimumSize                  Reason = "belowMinimumSize"
	ReasonAboveMaximumSize                  Reason = "aboveMaximumSize"
	ReasonAlreadyImportedSameHash           Reason = "alreadyImportedSameHash"
	ReasonAlreadyImportedSameName           Reason = "alreadyImportedSameName"
	ReasonIndexerDisabled                   Reason = "indexerDisabled"
	ReasonBlocklisted                       Reason = "blocklisted"
	ReasonCustomFormatMinimumScore          Reason = "customFormatMinimumScore"
	ReasonMinimumFreeSpace                  Reason = "minimumFreeSpace"
	ReasonHardcodeSubtitles                 Reason = "hardcodeSubtitles"
	ReasonWantedLanguage                    Reason = "wantedLanguage"
	ReasonMaximumSizeExceeded               Reason = "maximumSizeExceeded"
	ReasonMinimumAge                        Reason = "minimumAge"
	ReasonMaximumAge                        Reason = "maximumAge"
	ReasonSample                            Reason = "sample"
	ReasonProtocolDisabled                  Reason = "protocolDisabled"
	ReasonQualityNotWanted                  Reason = "qualityNotWanted"
	ReasonQualityUpgradesDisabled           Reason = "qualityUpgradesDisabled"
	ReasonQueueHigherPreference             Reason = "queueHigherPreference"
	ReasonQueueHigherRevision               Reason = "queueHigherRevision"
	ReasonQueueCutoffMet                    Reason = "queueCutoffMet"
	ReasonQueueCustomFormatCutoffMet        Reason = "queueCustomFormatCutoffMet"
	ReasonQueueCustomFormatScore            Reason = "queueCustomFormatScore"
	ReasonQueueCustomFormatScoreIncrement   Reason = "queueCustomFormatScoreIncrement"
	ReasonQueueUpgradesNotAllowed           Reason = "queueUpgradesNotAllowed"
	ReasonQueuePropersDisabled              Reason = "queuePropersDisabled"
	ReasonRaw                               Reason = "raw"
	ReasonMustContainMissing                Reason = "mustContainMissing"
	ReasonMustNotContainPresent             Reason = "mustNotContainPresent"
	ReasonRepackDisabled                    Reason = "repackDisabled"
	ReasonRepackUnknownReleaseGroup         Reason = "repackUnknownReleaseGroup"
	ReasonRepackReleaseGroupDoesNotMatch    Reason = "repackReleaseGroupDoesNotMatch"
	ReasonRequiredFlags                     Reason = "requiredFlags"
	ReasonMinimumSeeders                    Reason = "minimumSeeders"
	ReasonDiskHigherPreference              Reason = "diskHigherPreference"
	ReasonDiskHigherRevision                Reason = "diskHigherRevision"
	ReasonDiskCutoffMet                     Reason = "diskCutoffMet"
	ReasonDiskCustomFormatCutoffMet         Reason = "diskCustomFormatCutoffMet"
	ReasonDiskCustomFormatScore             Reason = "diskCustomFormatScore"
	ReasonDiskCustomFormatScoreIncrement    Reason = "diskCustomFormatScoreIncrement"
	ReasonDiskUpgradesNotAllowed            Reason = "diskUpgradesNotAllowed"
)

// Rejection is why a release is not grabbed. A temporary rejection may
// pass later, so the release is kept pending.
type Rejection struct {
	Reason    Reason
	Message   string
	Temporary bool
}

// reject returns a permanent rejection with a formatted message.
func reject(reason Reason, format string, args ...any) *Rejection {
	return &Rejection{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

// Decision is what the engine decided about a candidate.
type Decision struct {
	Candidate  *Candidate
	Rejections []Rejection
}

// Approved reports whether nothing rejected the candidate.
func (d *Decision) Approved() bool { return len(d.Rejections) == 0 }

// TemporarilyRejected reports whether the candidate was rejected only
// for now.
func (d *Decision) TemporarilyRejected() bool {
	return len(d.Rejections) > 0 && !slices.ContainsFunc(d.Rejections, func(r Rejection) bool { return !r.Temporary })
}

// Rejected reports whether something rejected the candidate for good.
func (d *Decision) Rejected() bool {
	return slices.ContainsFunc(d.Rejections, func(r Rejection) bool { return !r.Temporary })
}

// Priority orders the evaluation of specifications: those of a later
// priority run only when every earlier one accepted.
type Priority int

// The specification priorities.
const (
	PriorityDefault Priority = 0
	// PriorityDisk is for checks that touch the disk.
	PriorityDisk Priority = 1
)

// Spec is a check a release must pass to be grabbed, as Radarr's
// IDownloadDecisionEngineSpecification.
type Spec interface {
	// Name names the check in errors.
	Name() string
	Priority() Priority
	// Temporary reports whether the check's rejections may pass later.
	Temporary() bool
	// Check returns why the candidate fails, or nil. s is nil for RSS
	// syncs and pushed releases.
	Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error)
}

// Config is the settings the decisions follow, as Radarr's
// IConfigService.
type Config struct {
	Propers            ProperDownload
	PreferIndexerFlags bool
	// CompletedDownloadHandling is set when finished downloads are
	// imported automatically.
	CompletedDownloadHandling bool
	// MaximumSize is in bytes; 0 for none.
	MaximumSize int64
	// MinimumAge is how old usenet releases must be.
	MinimumAge time.Duration
	// Retention is in days; 0 for none.
	Retention          int
	AllowHardcodedSubs bool
	// WhitelistedHardcodedSubs are the hardcoded subtitles allowed when
	// others are not.
	WhitelistedHardcodedSubs []string
	SkipFreeSpaceCheck       bool
	// MinimumFreeSpace is in bytes.
	MinimumFreeSpace int64
	// AvailabilityDelay is in days.
	AvailabilityDelay int
}

// HistoryEventType is what a history event records.
type HistoryEventType int

// The history event types the decisions use.
const (
	HistoryGrabbed HistoryEventType = iota + 1
	HistoryImported
	HistoryDownloadFailed
	HistoryFileDeleted
	HistoryFileRenamed
	HistoryIgnored
)

// HistoryEvent is an event in an item's history.
type HistoryEvent struct {
	Type        HistoryEventType
	ItemID      core.ID
	Date        time.Time
	DownloadID  string
	SourceTitle string
	Quality     QualityModel
	// Formats are the custom formats the source title matches.
	Formats []CustomFormat
}

// QueueEntry is a download in the queue.
type QueueEntry struct {
	TargetID core.ID
	ItemIDs  []core.ID
	Title    string
	Size     int64
	Quality  QualityModel
	// Formats are the custom formats the download matches.
	Formats []CustomFormat
	// FailedPending is set when the download failed and waits to be
	// removed.
	FailedPending bool
}

// IndexerInfo is what the decisions need of an indexer.
type IndexerInfo struct {
	// ID names the indexer as "<plugin ID>/<indexer ID>".
	ID   string
	Name string
	Tags []string
	// RequiredFlags are flags a release must carry one of.
	RequiredFlags []string
	// MinimumSeeders is for torrent indexers; 0 for none.
	MinimumSeeders int
	// MultiLanguages are the languages its MULTi releases have.
	MultiLanguages []release.Language
	// DisabledUntil is set while the indexer is left out after failing.
	DisabledUntil time.Time
	// SeasonSearchMaximumSingleEpisodeAge is in days; season searches
	// want season packs of seasons whose last episode aired longer ago.
	// 0 for no limit.
	SeasonSearchMaximumSingleEpisodeAge int
}

// Ports are what the decisions read from the rest of the server.
type Ports interface {
	// History returns the history of the items, newest first.
	History(ctx context.Context, itemIDs []core.ID) ([]HistoryEvent, error)
	// Queue returns the downloads in the queue.
	Queue(ctx context.Context) ([]QueueEntry, error)
	// Pending returns the releases kept pending for a target.
	Pending(ctx context.Context, targetID core.ID) ([]Release, error)
	// Blocklisted reports whether the release is blocklisted for the
	// target.
	Blocklisted(ctx context.Context, targetID core.ID, r *Release) (bool, error)
	// Indexer returns an indexer, or nil when there is none by the ID.
	Indexer(ctx context.Context, id string) (*IndexerInfo, error)
	// FreeSpace returns the free space of the volume holding path; ok
	// is false when it is unknown.
	FreeSpace(ctx context.Context, path string) (free int64, ok bool, err error)
}

// Env is what decisions are made with.
type Env struct {
	Now             func() time.Time
	Config          Config
	Definitions     []Definition
	DelayProfiles   []DelayProfile
	ReleaseProfiles []ReleaseProfile
	Formats         []CustomFormat
	Terms           *TermMatcher
	Ports           Ports
}

func (e *Env) now() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

func (e *Env) upgrades() Upgrades { return Upgrades{Propers: e.Config.Propers} }

// definition returns the definition of a quality, or a zero one.
func (e *Env) definition(q release.Quality) Definition {
	for _, d := range e.Definitions {
		if d.Quality == q {
			return d
		}
	}
	return Definition{Quality: q}
}

// delayProfile returns the delay profile for tags.
func (e *Env) delayProfile(tags []string) *DelayProfile {
	if p := BestDelayProfile(e.DelayProfiles, tags); p != nil {
		return p
	}
	return &DelayProfile{EnableUsenet: true, EnableTorrent: true, PreferredProtocol: ProtocolUsenet}
}

func (e *Env) terms() *TermMatcher {
	if e.Terms == nil {
		e.Terms = &TermMatcher{}
	}
	return e.Terms
}

// Engine makes decisions on candidates with its specifications.
type Engine struct {
	Env   *Env
	Specs []Spec
}

// Evaluate runs the specifications on a candidate by priority, stopping
// after the first priority that rejects it. A specification that fails
// rejects the candidate with ReasonDecisionError.
func (e *Engine) Evaluate(ctx context.Context, c *Candidate, s *Search) Decision {
	specs := slices.Clone(e.Specs)
	slices.SortStableFunc(specs, func(a, b Spec) int { return cmp.Compare(a.Priority(), b.Priority()) })
	var rejections []Rejection
	for i := 0; i < len(specs); {
		j := i
		for j < len(specs) && specs[j].Priority() == specs[i].Priority() {
			j++
		}
		for _, spec := range specs[i:j] {
			r, err := spec.Check(ctx, e.Env, c, s)
			switch {
			case err != nil:
				rejections = append(rejections, Rejection{Reason: ReasonDecisionError, Message: fmt.Sprintf("%s: %v", spec.Name(), err)})
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

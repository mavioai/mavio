package acquire

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// permanent gives a specification the default priority and permanent
// rejections.
type permanent struct{}

func (permanent) Priority() Priority { return PriorityDefault }
func (permanent) Temporary() bool    { return false }

// temporary gives a specification the default priority and temporary
// rejections.
type temporary struct{}

func (temporary) Priority() Priority { return PriorityDefault }
func (temporary) Temporary() bool    { return true }

// MovieSpecs returns the specifications Radarr decides on movie releases
// with.
func MovieSpecs() []Spec {
	return []Spec{
		AcceptableSize{},
		AlreadyImported{},
		BlockedIndexer{},
		Blocklist{},
		CustomFormatAllowedByProfile{},
		FreeSpace{},
		HardcodeSubs{},
		LanguageWanted{},
		MaximumSize{},
		MinimumAge{},
		NotSample{},
		ProtocolAllowed{},
		QualityAllowedByProfile{},
		Queue{},
		RawDisk{},
		ReleaseRestrictions{},
		Repack{},
		RequiredIndexerFlags{},
		Retention{},
		TorrentSeeding{},
		UpgradeAllowed{},
		UpgradeDisk{},
		Available{},
		Delay{},
		History{},
		IndexerTag{},
		Monitored{},
		Pending{},
		Proper{},
		SearchTarget{},
	}
}

// megabytes converts megabytes to bytes as Radarr's Megabytes does.
func megabytes(mb float64) int64 { return int64(math.RoundToEven(mb * 1024 * 1024)) }

// roundTo rounds n to a multiple of level, halves to even, as Radarr's
// Round.
func roundTo(n, level int64) int64 {
	return int64(math.RoundToEven(float64(n)/float64(level))) * level
}

// sizeSuffix formats a size in bytes as Radarr's SizeSuffix: "1.5 GB".
func sizeSuffix(b int64) string {
	if b < 0 {
		return "-" + sizeSuffix(-b)
	}
	if b == 0 {
		return "0 B"
	}
	suffixes := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	mag := int(math.Log(float64(b)) / math.Log(1024))
	mag = min(mag, len(suffixes)-1)
	v := float64(b) / math.Pow(1024, float64(mag))
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if i := strings.IndexByte(s, '.'); i > 3 {
		var sb strings.Builder
		for j, r := range s[:i] {
			if j > 0 && (i-j)%3 == 0 {
				sb.WriteByte(',')
			}
			sb.WriteRune(r)
		}
		s = sb.String() + s[i:]
	}
	return s + " " + suffixes[mag]
}

// formatNames joins the names of custom formats.
func formatNames(formats []CustomFormat) string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.Name
	}
	return strings.Join(names, ", ")
}

// languageNames joins the names of languages.
func languageNames(langs []release.Language) string {
	names := make([]string, len(langs))
	for i, l := range langs {
		names[i] = l.String()
	}
	return strings.Join(names, ", ")
}

// formatFloat formats a number as .NET does by default.
func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// AcceptableSize rejects releases smaller or larger than the quality's
// definition allows for the movie's runtime.
type AcceptableSize struct{ permanent }

// Name implements Spec.
func (AcceptableSize) Name() string { return "AcceptableSize" }

// Check implements Spec.
func (AcceptableSize) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.Size == 0 {
		return nil, nil
	}
	def := e.definition(c.Quality.Quality)
	runtime := 0
	for _, it := range c.Items {
		runtime += it.Runtime
	}
	if runtime == 0 {
		// The median runtime of movies.
		runtime = 110
	}
	if def.MinSize > 0 {
		minSize := megabytes(def.MinSize) * int64(runtime)
		if c.Release.Size < minSize {
			return reject(ReasonBelowMinimumSize, "%s is smaller than minimum allowed %s (for %s)", sizeSuffix(c.Release.Size), sizeSuffix(minSize), c.Target.Title), nil
		}
	}
	if def.MaxSize > 0 {
		maxSize := megabytes(def.MaxSize) * int64(runtime)
		if c.Release.Size > maxSize {
			return reject(ReasonAboveMaximumSize, "%s is larger than maximum allowed %s (for %s)", sizeSuffix(c.Release.Size), sizeSuffix(maxSize), c.Target.Title), nil
		}
	}
	return nil, nil
}

// AlreadyImported rejects the release last grabbed and imported again
// when its imported quality differed from the grabbed one, so that it is
// not grabbed over and over.
type AlreadyImported struct{ permanent }

// Name implements Spec.
func (AlreadyImported) Name() string { return "AlreadyImported" }

// Check implements Spec.
func (AlreadyImported) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if !e.Config.CompletedDownloadHandling || len(c.Files()) == 0 {
		return nil, nil
	}
	history, err := e.Ports.History(ctx, itemIDs(c))
	if err != nil {
		return nil, err
	}
	gi := slices.IndexFunc(history, func(h HistoryEvent) bool { return h.Type == HistoryGrabbed })
	if gi < 0 {
		return nil, nil
	}
	grabbed := history[gi]
	ii := slices.IndexFunc(history, func(h HistoryEvent) bool {
		return h.Type == HistoryImported && h.DownloadID == grabbed.DownloadID
	})
	if ii < 0 || grabbed.Quality.Equal(history[ii].Quality) {
		return nil, nil
	}
	r := c.Release
	if r.Protocol == ProtocolTorrent && r.InfoHash != "" && strings.ToUpper(r.InfoHash) == grabbed.DownloadID {
		return reject(ReasonAlreadyImportedSameHash, "Has same torrent hash as a grabbed and imported release"), nil
	}
	if strings.EqualFold(r.Title, grabbed.SourceTitle) {
		return reject(ReasonAlreadyImportedSameName, "Has same release name as a grabbed and imported release"), nil
	}
	return nil, nil
}

// itemIDs returns the IDs of the candidate's items.
func itemIDs(c *Candidate) []core.ID {
	ids := make([]core.ID, len(c.Items))
	for i, it := range c.Items {
		ids[i] = it.ID
	}
	return ids
}

// BlockedIndexer rejects releases of indexers left out after failing.
type BlockedIndexer struct{ temporary }

// Name implements Spec.
func (BlockedIndexer) Name() string { return "BlockedIndexer" }

// Check implements Spec.
func (BlockedIndexer) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.IndexerID == "" {
		return nil, nil
	}
	ix, err := e.Ports.Indexer(ctx, c.Release.IndexerID)
	if err != nil || ix == nil {
		return nil, err
	}
	if ix.DisabledUntil.After(e.now()) {
		return reject(ReasonIndexerDisabled, "Indexer %s is blocked till %s due to failures, cannot grab release.", c.Release.Indexer, ix.DisabledUntil.Format(time.RFC3339)), nil
	}
	return nil, nil
}

// Blocklist rejects blocklisted releases.
type Blocklist struct{ permanent }

// Name implements Spec.
func (Blocklist) Name() string { return "Blocklist" }

// Check implements Spec.
func (Blocklist) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	ok, err := e.Ports.Blocklisted(ctx, c.Target.ID, c.Release)
	if err != nil || !ok {
		return nil, err
	}
	return reject(ReasonBlocklisted, "Release is blocklisted"), nil
}

// CustomFormatAllowedByProfile rejects releases whose custom formats
// score below the profile's minimum.
type CustomFormatAllowedByProfile struct{ permanent }

// Name implements Spec.
func (CustomFormatAllowedByProfile) Name() string { return "CustomFormatAllowedByProfile" }

// Check implements Spec.
func (CustomFormatAllowedByProfile) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if minScore := c.Target.Profile.MinFormatScore; c.FormatScore < minScore {
		return reject(ReasonCustomFormatMinimumScore, "Custom Formats %s have score %d below profile minimum %d", formatNames(c.Formats), c.FormatScore, minScore), nil
	}
	return nil, nil
}

// FreeSpace rejects releases that would not leave the minimum free
// space once imported.
type FreeSpace struct{}

// Name implements Spec.
func (FreeSpace) Name() string { return "FreeSpace" }

// Priority implements Spec.
func (FreeSpace) Priority() Priority { return PriorityDisk }

// Temporary implements Spec.
func (FreeSpace) Temporary() bool { return false }

// Check implements Spec.
func (FreeSpace) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if e.Config.SkipFreeSpaceCheck || c.Target.Path == "" {
		return nil, nil
	}
	free, ok, err := e.Ports.FreeSpace(ctx, c.Target.Path)
	if err != nil || !ok {
		return nil, err
	}
	remaining := free - c.Release.Size
	if remaining <= 0 {
		return reject(ReasonMinimumFreeSpace, "Importing after download will exceed available disk space"), nil
	}
	if remaining < e.Config.MinimumFreeSpace {
		return reject(ReasonMinimumFreeSpace, "Not enough free space (%s) to import after download: %s. (Settings: Media Management: Minimum Free Space)", sizeSuffix(e.Config.MinimumFreeSpace), sizeSuffix(remaining)), nil
	}
	return nil, nil
}

// HardcodeSubs rejects releases with hardcoded subtitles unless they are
// allowed or whitelisted.
type HardcodeSubs struct{ permanent }

// Name implements Spec.
func (HardcodeSubs) Name() string { return "HardcodeSubs" }

// Check implements Spec.
func (HardcodeSubs) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	subs := c.HardcodedSubs
	if e.Config.AllowHardcodedSubs || strings.TrimSpace(subs) == "" {
		return nil, nil
	}
	for _, w := range e.Config.WhitelistedHardcodedSubs {
		if strings.TrimSpace(w) != "" && strings.Contains(strings.ToLower(subs), strings.ToLower(w)) {
			return nil, nil
		}
	}
	return reject(ReasonHardcodeSubtitles, "Hardcode subs found: %s", subs), nil
}

// LanguageWanted rejects movie releases without the profile's language.
type LanguageWanted struct{ permanent }

// Name implements Spec.
func (LanguageWanted) Name() string { return "Language" }

// Check implements Spec.
func (LanguageWanted) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*Rejection, error) {
	wanted := c.Target.Profile.Language
	switch wanted {
	case release.LanguageUnknown:
		return nil, nil
	case release.Original:
		if original := c.Target.OriginalLanguage; !slices.Contains(c.Languages, original) {
			return reject(ReasonWantedLanguage, "Original Language (%s) is wanted, but found %s", original, languageNames(c.Languages)), nil
		}
		return nil, nil
	}
	if !slices.Contains(c.Languages, wanted) {
		return reject(ReasonWantedLanguage, "%s is wanted, but found %s", wanted, languageNames(c.Languages)), nil
	}
	return nil, nil
}

// MaximumSize rejects releases larger than the configured maximum.
type MaximumSize struct{ permanent }

// Name implements Spec.
func (MaximumSize) Name() string { return "MaximumSize" }

// Check implements Spec.
func (MaximumSize) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	maxSize, size := e.Config.MaximumSize, c.Release.Size
	if maxSize == 0 || size == 0 || size <= maxSize {
		return nil, nil
	}
	return reject(ReasonMaximumSizeExceeded, "%s is too big, maximum size is %s (Settings->Indexers->Maximum Size)", sizeSuffix(size), sizeSuffix(maxSize)), nil
}

// MinimumAge holds back usenet releases younger than the configured
// minimum age, so that they have propagated.
type MinimumAge struct{ temporary }

// Name implements Spec.
func (MinimumAge) Name() string { return "MinimumAge" }

// Check implements Spec.
func (MinimumAge) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.Protocol != ProtocolUsenet || e.Config.MinimumAge == 0 {
		return nil, nil
	}
	age := c.Release.Age(e.now())
	if age < e.Config.MinimumAge {
		return reject(ReasonMinimumAge, "Only %s minutes old, minimum age is %s minutes", formatFloat(math.Round(age.Minutes()*10)/10), formatFloat(e.Config.MinimumAge.Minutes())), nil
	}
	return nil, nil
}

// NotSample rejects samples.
type NotSample struct{ permanent }

// Name implements Spec.
func (NotSample) Name() string { return "NotSample" }

// Check implements Spec.
func (NotSample) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if strings.Contains(strings.ToLower(c.Release.Title), "sample") && c.Release.Size < megabytes(70) {
		return reject(ReasonSample, "Sample"), nil
	}
	return nil, nil
}

// ProtocolAllowed rejects releases of a protocol the delay profile does
// not enable.
type ProtocolAllowed struct{ permanent }

// Name implements Spec.
func (ProtocolAllowed) Name() string { return "Protocol" }

// Check implements Spec.
func (ProtocolAllowed) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	p := e.delayProfile(c.Target.Tags)
	what := targetNoun(c.Target)
	switch {
	case c.Release.Protocol == ProtocolUsenet && !p.EnableUsenet:
		return reject(ReasonProtocolDisabled, "Usenet is not enabled for this %s", what), nil
	case c.Release.Protocol == ProtocolTorrent && !p.EnableTorrent:
		return reject(ReasonProtocolDisabled, "Torrent is not enabled for this %s", what), nil
	}
	return nil, nil
}

// targetNoun names the kind of a target in messages.
func targetNoun(t *Target) string {
	if t.Kind == release.Series {
		return "series"
	}
	return "movie"
}

// QualityAllowedByProfile rejects releases of a quality the profile does
// not allow.
type QualityAllowedByProfile struct{ permanent }

// Name implements Spec.
func (QualityAllowedByProfile) Name() string { return "QualityAllowedByProfile" }

// Check implements Spec.
func (QualityAllowedByProfile) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if !c.Target.Profile.Allowed(c.Quality.Quality) {
		return reject(ReasonQualityNotWanted, "%s is not wanted in profile", c.Quality.Quality), nil
	}
	return nil, nil
}

// Queue rejects releases that would not upgrade a download already in
// the queue.
type Queue struct{ permanent }

// Name implements Spec.
func (Queue) Name() string { return "Queue" }

// Check implements Spec.
func (Queue) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	queue, err := e.Ports.Queue(ctx)
	if err != nil {
		return nil, err
	}
	p := c.Target.Profile
	u := e.upgrades()
	for _, q := range queue {
		if q.TargetID != c.Target.ID || !sharesItem(q.ItemIDs, c) {
			continue
		}
		// A failed download waiting to be removed is being replaced.
		if q.FailedPending {
			continue
		}
		if !u.CutoffNotMet(p, q.Quality, q.Formats, &c.Quality) {
			return reject(ReasonQueueCutoffMet, "Quality for release in queue already meets cutoff: %s", q.Quality), nil
		}
		switch u.IsUpgradable(p, q.Quality, q.Formats, c.Quality, c.Formats) {
		case UpgradeBetterQuality:
			return reject(ReasonQueueHigherPreference, "Release in queue is of equal or higher preference: %s", q.Quality), nil
		case UpgradeBetterRevision:
			return reject(ReasonQueueHigherRevision, "Release in queue is of equal or higher revision: %s", revisionString(q.Quality.Revision)), nil
		case UpgradeQualityCutoff:
			return reject(ReasonQueueCutoffMet, "Release in queue meets quality cutoff: %s", p.cutoffItem(p.Cutoff)), nil
		case UpgradeCustomFormatCutoff:
			return reject(ReasonQueueCustomFormatCutoffMet, "Release in queue meets Custom Format cutoff: %d", p.CutoffFormatScore), nil
		case UpgradeCustomFormatScore:
			return reject(ReasonQueueCustomFormatScore, "Release in queue has an equal or higher Custom Format score: %d", p.FormatScore(q.Formats)), nil
		case UpgradeMinCustomFormatScore:
			return reject(ReasonQueueCustomFormatScoreIncrement, "Release in queue has Custom Format score within Custom Format score increment: %d", p.MinUpgradeFormatScore), nil
		case UpgradeNotAllowed:
			return reject(ReasonQueueUpgradesNotAllowed, "Release in queue and Quality Profile '%s' does not allow upgrades", p.Name), nil
		}
		if IsRevisionUpgrade(q.Quality, c.Quality) && e.Config.Propers == PropersDoNotUpgrade {
			return reject(ReasonQueuePropersDisabled, "Proper downloading is disabled"), nil
		}
	}
	return nil, nil
}

// sharesItem reports whether ids, when given, name one of the
// candidate's items.
func sharesItem(ids []core.ID, c *Candidate) bool {
	if len(ids) == 0 {
		return true
	}
	return slices.ContainsFunc(c.Items, func(it *Item) bool { return slices.Contains(ids, it.ID) })
}

// revisionString formats a revision as Radarr's Revision.ToString.
func revisionString(r release.Revision) string {
	s := fmt.Sprintf("v%d", r.Version)
	if r.Real > 0 {
		s += fmt.Sprintf(" Real:%d", r.Real)
	}
	return s
}

// The titles of raw Blu-ray and DVD releases.
var discRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:dis[ck])(?:[-_. ]\d+[-_. ])(?:(?:(?:480|720|1080|2160)[ip]|)[-_. ])?(?:Blu\-?ray)`),
	regexp.MustCompile(`(?i)(?:(?:480|720|1080|2160)[ip]|)[-_. ](?:full)[-_. ](?:Blu\-?ray)`),
	regexp.MustCompile(`(?i)(?:\d?x?M?DVD-?[R59])(?:[ ._]|$)`),
}

// RawDisk rejects raw Blu-ray and DVD releases.
type RawDisk struct{ permanent }

// Name implements Spec.
func (RawDisk) Name() string { return "RawDisk" }

// Check implements Spec.
func (RawDisk) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release == nil {
		return nil, nil
	}
	for _, re := range discRegexes {
		if re.MatchString(c.Release.Title) {
			return reject(ReasonRaw, "Raw Bluray/DVD release"), nil
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.Release.Container)) {
	case "vob", "iso":
		return reject(ReasonRaw, "Raw DVD release"), nil
	case "m2ts":
		return reject(ReasonRaw, "Raw Bluray release"), nil
	}
	return nil, nil
}

// ReleaseRestrictions rejects releases missing a required term of a
// release profile or carrying an ignored one.
type ReleaseRestrictions struct{ permanent }

// Name implements Spec.
func (ReleaseRestrictions) Name() string { return "ReleaseRestrictions" }

// Check implements Spec.
func (ReleaseRestrictions) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	var profiles []ReleaseProfile
	for _, p := range e.ReleaseProfiles {
		if p.AppliesTo(c.Target.Tags, c.Release.IndexerID) {
			profiles = append(profiles, p)
		}
	}
	msg := e.terms().CheckTerms(profiles, c.Release.Title)
	switch {
	case msg == "":
		return nil, nil
	case strings.HasPrefix(msg, "Does not contain"):
		return &Rejection{Reason: ReasonMustContainMissing, Message: msg}, nil
	default:
		return &Rejection{Reason: ReasonMustNotContainPresent, Message: msg}, nil
	}
}

// Repack rejects repacks of another release group than the file's.
type Repack struct{ permanent }

// Name implements Spec.
func (Repack) Name() string { return "Repack" }

// Check implements Spec.
func (Repack) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if !c.Quality.Revision.IsRepack || e.Config.Propers == PropersDoNotPrefer {
		return nil, nil
	}
	for _, f := range c.Files() {
		if !IsRevisionUpgrade(f.Quality, c.Quality) {
			continue
		}
		if e.Config.Propers == PropersDoNotUpgrade {
			return reject(ReasonRepackDisabled, "Repack downloading is disabled"), nil
		}
		if strings.TrimSpace(f.ReleaseGroup) == "" {
			return reject(ReasonRepackUnknownReleaseGroup, "Unable to determine release group for the existing file"), nil
		}
		if strings.TrimSpace(c.ReleaseGroup) == "" {
			return reject(ReasonRepackUnknownReleaseGroup, "Unable to determine release group for this release"), nil
		}
		if !strings.EqualFold(f.ReleaseGroup, c.ReleaseGroup) {
			return reject(ReasonRepackReleaseGroupDoesNotMatch, "Release is a repack for a different release group. Release Group: %s. File release group: %s", c.ReleaseGroup, f.ReleaseGroup), nil
		}
	}
	return nil, nil
}

// RequiredIndexerFlags rejects torrents without one of the flags their
// indexer requires.
type RequiredIndexerFlags struct{ permanent }

// Name implements Spec.
func (RequiredIndexerFlags) Name() string { return "RequiredIndexerFlags" }

// Check implements Spec.
func (RequiredIndexerFlags) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.IndexerID == "" || c.Release.Protocol != ProtocolTorrent {
		return nil, nil
	}
	ix, err := e.Ports.Indexer(ctx, c.Release.IndexerID)
	if err != nil || ix == nil || len(ix.RequiredFlags) == 0 {
		return nil, err
	}
	if slices.ContainsFunc(ix.RequiredFlags, c.Release.HasFlag) {
		return nil, nil
	}
	return reject(ReasonRequiredFlags, "None of the required indexer flags %s where found. Found flags: %s", strings.Join(ix.RequiredFlags, ", "), strings.Join(c.Release.Flags, ", ")), nil
}

// Retention rejects usenet releases older than the configured
// retention.
type Retention struct{ permanent }

// Name implements Spec.
func (Retention) Name() string { return "Retention" }

// Check implements Spec.
func (Retention) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.Protocol != ProtocolUsenet {
		return nil, nil
	}
	age := int(c.Release.Age(e.now()) / (24 * time.Hour))
	if r := e.Config.Retention; r > 0 && age > r {
		return reject(ReasonMaximumAge, "Older than configured retention"), nil
	}
	return nil, nil
}

// TorrentSeeding rejects torrents with fewer seeders than their indexer
// requires.
type TorrentSeeding struct{ permanent }

// Name implements Spec.
func (TorrentSeeding) Name() string { return "TorrentSeeding" }

// Check implements Spec.
func (TorrentSeeding) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	r := c.Release
	if r.Protocol != ProtocolTorrent || r.IndexerID == "" {
		return nil, nil
	}
	ix, err := e.Ports.Indexer(ctx, r.IndexerID)
	if err != nil || ix == nil {
		return nil, err
	}
	if r.Seeders != nil && *r.Seeders < ix.MinimumSeeders {
		return reject(ReasonMinimumSeeders, "Not enough seeders: %d. Minimum seeders: %d", *r.Seeders, ix.MinimumSeeders), nil
	}
	return nil, nil
}

// UpgradeAllowed rejects upgrades of existing files when the profile
// does not allow upgrades.
type UpgradeAllowed struct{ permanent }

// Name implements Spec.
func (UpgradeAllowed) Name() string { return "UpgradeAllowed" }

// Check implements Spec.
func (UpgradeAllowed) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	for _, f := range c.Files() {
		if !e.upgrades().IsUpgradeAllowed(c.Target.Profile, f.Quality, f.Formats, c.Quality, c.Formats) {
			return reject(ReasonQualityUpgradesDisabled, "Existing file and the Quality profile does not allow upgrades"), nil
		}
	}
	return nil, nil
}

// UpgradeDisk rejects releases that would not upgrade the existing
// files.
type UpgradeDisk struct{ permanent }

// Name implements Spec.
func (UpgradeDisk) Name() string { return "UpgradeDisk" }

// Check implements Spec.
func (UpgradeDisk) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	p := c.Target.Profile
	u := e.upgrades()
	for _, f := range c.Files() {
		if !u.CutoffNotMet(p, f.Quality, f.Formats, &c.Quality) {
			return reject(ReasonDiskCutoffMet, "Existing file meets cutoff: %s [%s]", p.cutoffItem(p.effectiveCutoff()), formatNames(f.Formats)), nil
		}
		switch u.IsUpgradable(p, f.Quality, f.Formats, c.Quality, c.Formats) {
		case UpgradeBetterQuality:
			return reject(ReasonDiskHigherPreference, "Existing file on disk is of equal or higher preference: %s", f.Quality), nil
		case UpgradeBetterRevision:
			return reject(ReasonDiskHigherRevision, "Existing file on disk is of equal or higher revision: %s", revisionString(f.Quality.Revision)), nil
		case UpgradeQualityCutoff:
			return reject(ReasonDiskCutoffMet, "Existing file on disk meets quality cutoff: %s", p.cutoffItem(p.Cutoff)), nil
		case UpgradeCustomFormatCutoff:
			return reject(ReasonDiskCustomFormatCutoffMet, "Existing file on disk meets Custom Format cutoff: %d", p.CutoffFormatScore), nil
		case UpgradeCustomFormatScore:
			return reject(ReasonDiskCustomFormatScore, "Existing file on disk has a equal or higher Custom Format score: %d", p.FormatScore(f.Formats)), nil
		case UpgradeMinCustomFormatScore:
			return reject(ReasonDiskCustomFormatScoreIncrement, "Existing file on disk has Custom Format score within Custom Format score increment: %d", p.MinUpgradeFormatScore), nil
		case UpgradeNotAllowed:
			return reject(ReasonDiskUpgradesNotAllowed, "Existing file on disk and Quality Profile '%s' does not allow upgrades", p.Name), nil
		}
	}
	return nil, nil
}

// Available rejects releases of movies not yet available, unless a user
// searched for them.
type Available struct{ permanent }

// Name implements Spec.
func (Available) Name() string { return "Availability" }

// Check implements Spec.
func (Available) Check(_ context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s != nil && s.UserInvoked {
		return nil, nil
	}
	delay := e.Config.AvailabilityDelay
	for _, it := range c.Items {
		if a := it.Availability; a != nil && !a.Available(e.now(), delay) {
			return reject(ReasonAvailability, "Movie %s will only be considered available %d days after %s", c.Target.Title, delay, a.Minimum), nil
		}
	}
	return nil, nil
}

// String returns the status's name.
func (s MovieStatus) String() string {
	switch s {
	case StatusTBA:
		return "TBA"
	case StatusAnnounced:
		return "Announced"
	case StatusInCinemas:
		return "InCinemas"
	case StatusReleased:
		return "Released"
	}
	return strconv.Itoa(int(s))
}

// Delay holds releases back for the delay of their protocol, waiting for
// a better one, unless they are already the best the profile wants.
type Delay struct{ temporary }

// Name implements Spec.
func (Delay) Name() string { return "Delay" }

// Check implements Spec.
func (Delay) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s != nil && s.UserInvoked {
		return nil, nil
	}
	p := c.Target.Profile
	dp := e.delayProfile(c.Target.Tags)
	delay := dp.Delay(c.Release.Protocol)
	preferred := c.Release.Protocol == dp.PreferredProtocol
	if delay == 0 {
		return nil, nil
	}
	if preferred {
		u := e.upgrades()
		for _, f := range c.Files() {
			if u.IsUpgradable(p, f.Quality, f.Formats, c.Quality, c.Formats) == UpgradeAccepted && IsRevisionUpgrade(f.Quality, c.Quality) {
				return nil, nil
			}
		}
	}
	if dp.BypassIfHighestQuality && preferred && p.CompareQualities(c.Quality.Quality, p.LastAllowedQuality(), false) >= 0 {
		return nil, nil
	}
	if dp.BypassIfAboveCustomFormatScore && preferred && c.FormatScore >= dp.MinimumCustomFormatScore {
		return nil, nil
	}
	pending, err := e.Ports.Pending(ctx, c.Target.ID)
	if err != nil {
		return nil, err
	}
	now := e.now()
	var oldest time.Duration
	for i := range pending {
		oldest = max(oldest, pending[i].Age(now))
	}
	if len(pending) > 0 && oldest > delay {
		return nil, nil
	}
	if c.Release.Age(now) < delay {
		return reject(ReasonMinimumAgeDelay, "Waiting for better quality release"), nil
	}
	return nil, nil
}

// History rejects releases during RSS syncs that would not upgrade one
// grabbed in the last 12 hours, or at all when completed downloads are
// not handled.
type History struct{ permanent }

// Name implements Spec.
func (History) Name() string { return "History" }

// Check implements Spec.
func (History) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s != nil {
		return nil, nil
	}
	p := c.Target.Profile
	u := e.upgrades()
	for _, id := range itemIDs(c) {
		history, err := e.Ports.History(ctx, []core.ID{id})
		if err != nil {
			return nil, err
		}
		if len(history) == 0 || history[0].Type != HistoryGrabbed {
			continue
		}
		h := history[0]
		recent := h.Date.After(e.now().Add(-12 * time.Hour))
		if !recent && e.Config.CompletedDownloadHandling {
			continue
		}
		if !u.CutoffNotMet(p, h.Quality, h.Formats, &c.Quality) {
			if recent {
				return reject(ReasonHistoryRecentCutoffMet, "Recent grab event in history already meets cutoff: %s", h.Quality), nil
			}
			return reject(ReasonHistoryCdhDisabledCutoffMet, "CDH is disabled and grab event in history already meets cutoff: %s", h.Quality), nil
		}
		subject := "CDH is disabled and"
		if recent {
			subject = "Recent"
		}
		switch u.IsUpgradable(p, h.Quality, h.Formats, c.Quality, c.Formats) {
		case UpgradeBetterQuality:
			return reject(ReasonHistoryHigherPreference, "%s grab event in history is of equal or higher preference: %s", subject, h.Quality), nil
		case UpgradeBetterRevision:
			return reject(ReasonHistoryHigherRevision, "%s grab event in history is of equal or higher revision: %s", subject, revisionString(h.Quality.Revision)), nil
		case UpgradeQualityCutoff:
			return reject(ReasonHistoryCutoffMet, "%s grab event in history meets quality cutoff: %s", subject, p.cutoffItem(p.Cutoff)), nil
		case UpgradeCustomFormatCutoff:
			return reject(ReasonHistoryCustomFormatCutoffMet, "%s grab event in history meets Custom Format cutoff: %d", subject, p.CutoffFormatScore), nil
		case UpgradeCustomFormatScore:
			return reject(ReasonHistoryCustomFormatScore, "%s grab event in history has an equal or higher Custom Format score: %d", subject, p.FormatScore(h.Formats)), nil
		case UpgradeMinCustomFormatScore:
			return reject(ReasonHistoryCustomFormatScoreIncrement, "%s grab event in history has Custom Format score within Custom Format score increment: %d", subject, p.MinUpgradeFormatScore), nil
		case UpgradeNotAllowed:
			return reject(ReasonHistoryUpgradesNotAllowed, "%s grab event in history and Quality Profile '%s' does not allow upgrades", subject, p.Name), nil
		}
	}
	return nil, nil
}

// IndexerTag rejects releases of indexers with tags none of the
// target's share.
type IndexerTag struct{ permanent }

// Name implements Spec.
func (IndexerTag) Name() string { return "IndexerTag" }

// Check implements Spec.
func (IndexerTag) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*Rejection, error) {
	if c.Release.IndexerID == "" {
		return nil, nil
	}
	ix, err := e.Ports.Indexer(ctx, c.Release.IndexerID)
	if err != nil || ix == nil {
		return nil, err
	}
	if len(ix.Tags) > 0 && !slices.ContainsFunc(ix.Tags, func(t string) bool { return slices.Contains(c.Target.Tags, t) }) {
		return reject(ReasonNoMatchingTag, "%s tags do not match any of the indexer tags", capitalize(targetNoun(c.Target))), nil
	}
	return nil, nil
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Monitored rejects releases of unmonitored movies, unless a user
// searched for them.
type Monitored struct{ permanent }

// Name implements Spec.
func (Monitored) Name() string { return "Monitored" }

// Check implements Spec.
func (Monitored) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s != nil && s.UserInvoked {
		return nil, nil
	}
	if !c.Target.Monitored {
		return reject(ReasonNotMonitored, "%s is not monitored", capitalize(targetNoun(c.Target))), nil
	}
	return nil, nil
}

// Pending holds back pushed releases and releases of automatic searches
// while a release for the same target is pending.
type Pending struct{ temporary }

// Name implements Spec.
func (Pending) Name() string { return "Pending" }

// Check implements Spec.
func (Pending) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error) {
	if c.Source == SourceRSS || (s != nil && s.UserInvoked) {
		return nil, nil
	}
	pending, err := e.Ports.Pending(ctx, c.Target.ID)
	if err != nil || len(pending) == 0 {
		return nil, err
	}
	return reject(ReasonMinimumAgeDelayPushed, "Release containing at least one matching %s is already pending, delaying pushed release", targetNoun(c.Target)), nil
}

// Proper rejects, during RSS syncs, propers of files when propers are not
// upgraded to, or of files added more than a week ago.
type Proper struct{ permanent }

// Name implements Spec.
func (Proper) Name() string { return "Proper" }

// Check implements Spec.
func (Proper) Check(_ context.Context, e *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s != nil || e.Config.Propers == PropersDoNotPrefer {
		return nil, nil
	}
	now := e.now()
	weekAgo := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -7)
	for _, f := range c.Files() {
		if !IsRevisionUpgrade(f.Quality, c.Quality) {
			continue
		}
		if e.Config.Propers == PropersDoNotUpgrade {
			return reject(ReasonPropersDisabled, "Proper downloading is disabled"), nil
		}
		if f.DateAdded.Before(weekAgo) {
			return reject(ReasonProperForOldFile, "Proper for old file"), nil
		}
	}
	return nil, nil
}

// SearchTarget rejects releases a search found for another target.
type SearchTarget struct{ permanent }

// Name implements Spec.
func (SearchTarget) Name() string { return "SearchTarget" }

// Check implements Spec.
func (SearchTarget) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*Rejection, error) {
	if s == nil || c.Target.ID == s.TargetID {
		return nil, nil
	}
	return reject(ReasonWrongTarget, "Wrong %s", targetNoun(c.Target)), nil
}

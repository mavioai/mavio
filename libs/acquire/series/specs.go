package series

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// permanent gives a specification the default priority and permanent
// rejections.
type permanent struct{}

func (permanent) Priority() acquire.Priority { return acquire.PriorityDefault }
func (permanent) Temporary() bool            { return false }

// temporary gives a specification the default priority and temporary
// rejections.
type temporary struct{}

func (temporary) Priority() acquire.Priority { return acquire.PriorityDefault }
func (temporary) Temporary() bool            { return true }

// Specs returns the specifications Sonarr decides on episode releases
// with: those written for episodes here, and those Sonarr shares with
// Radarr from the root package.
func Specs() []Spec {
	return []Spec{
		AcceptableSize{},
		AlreadyImported{},
		AnimeVersionUpgrade{},
		Root(acquire.BlockedIndexer{}),
		Root(acquire.Blocklist{}),
		CustomFormatAllowedByProfile{},
		Root(acquire.FreeSpace{}),
		FullSeason{},
		Root(acquire.MaximumSize{}),
		Root(acquire.MinimumAge{}),
		MultiSeason{},
		Root(acquire.NotSample{}),
		Root(acquire.ProtocolAllowed{}),
		Root(acquire.QualityAllowedByProfile{}),
		Queue{},
		Root(acquire.RawDisk{}),
		Root(acquire.ReleaseRestrictions{}),
		Root(acquire.Repack{}),
		Root(acquire.Retention{}),
		SameEpisodesGrab{},
		SceneNumbering{},
		SeasonPackOnly{},
		SplitEpisode{},
		Root(acquire.TorrentSeeding{}),
		Root(acquire.UpgradeAllowed{}),
		UpgradeDisk{},
		Delay{},
		DeletedEpisodeFile{},
		Root(acquire.History{}),
		Root(acquire.IndexerTag{}),
		MonitoredEpisode{},
		Root(acquire.Proper{}),
		EpisodeRequested{},
		SeasonMatch{},
		Root(acquire.SearchTarget{}),
		SingleEpisodeSearchMatch{},
	}
}

// megabytes converts megabytes to bytes as Sonarr's Megabytes does.
func megabytes(mb float64) int64 { return int64(math.RoundToEven(mb * 1024 * 1024)) }

// roundTo rounds n to a multiple of level, halves to even, as Sonarr's
// Round.
func roundTo(n, level int64) int64 {
	return int64(math.RoundToEven(float64(n)/float64(level))) * level
}

// sizeSuffix formats a size in bytes as Sonarr's SizeSuffix: "1.5 GB".
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
func formatNames(formats []acquire.CustomFormat) string {
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.Name
	}
	return strings.Join(names, ", ")
}

// revisionString formats a revision as Sonarr's Revision.ToString.
func revisionString(r release.Revision) string {
	s := fmt.Sprintf("v%d", r.Version)
	if r.Real > 0 {
		s += fmt.Sprintf(" Real:%d", r.Real)
	}
	return s
}

// cutoffItem returns the profile item the quality or group of ID id is
// in.
func cutoffItem(p *acquire.QualityProfile, id int) acquire.ProfileItem {
	i := p.IndexOf(id, false).Index
	if i < 0 || i >= len(p.Items) {
		return acquire.ProfileItem{}
	}
	return p.Items[i]
}

// effectiveCutoff is the ID of the quality or group that meets the
// cutoff: the first allowed quality when upgrades are not allowed.
func effectiveCutoff(p *acquire.QualityProfile) int {
	if p.UpgradeAllowed {
		return p.Cutoff
	}
	return acquire.QualityID(p.FirstAllowedQuality())
}

// AcceptableSize rejects releases smaller or larger than the quality's
// definition allows for the runtime of their episodes. Episodes of a new
// series whose runtime is not yet known count as 45 minutes long.
type AcceptableSize struct{ permanent }

// Name implements Spec.
func (AcceptableSize) Name() string { return "AcceptableSize" }

// Check implements Spec.
func (AcceptableSize) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if c.Parsed.Special || c.Release.Size == 0 {
		return nil, nil
	}
	seriesRuntime := c.Series.Runtime
	if seriesRuntime == 0 && len(c.Episodes) > 0 {
		first := -1
		for _, n := range c.Series.Seasons {
			if n > 0 && (first < 0 || n < first) {
				first = n
			}
		}
		if first < 0 {
			return nil, errors.New("the series has no seasons")
		}
		season, err := e.Episodes.EpisodesBySeason(ctx, c.Series.ID, first)
		if err != nil {
			return nil, err
		}
		if len(season) == 0 {
			return nil, fmt.Errorf("season %d has no episodes", first)
		}
		pilot := season[0]
		if c.Episodes[0].SeasonNumber == pilot.SeasonNumber {
			graceEnd := pilot.AirDateUTC
			if graceEnd.IsZero() {
				graceEnd = c.Release.PublishDate
			}
			graceEnd = graceEnd.Add(24 * time.Hour)
			if !slices.ContainsFunc(c.Episodes, func(ep *Episode) bool {
				return !ep.AirDateUTC.IsZero() && !ep.AirDateUTC.Before(graceEnd)
			}) {
				seriesRuntime = 45
			}
		}
	}
	runtime := 0
	for _, ep := range c.Episodes {
		if ep.Runtime > 0 {
			runtime += ep.Runtime
		} else {
			runtime += seriesRuntime
		}
	}
	if runtime == 0 {
		return reject(acquire.ReasonUnknownRuntime, "Runtime of all episodes is 0, unable to validate size until it is available"), nil
	}
	runtimeMsg := strconv.Itoa(runtime) + "min"
	if len(c.Episodes) != 1 {
		runtimeMsg = fmt.Sprintf("%dx %dmin", len(c.Episodes), runtime)
	}
	def := e.definition(c.Quality.Quality)
	size := c.Release.Size
	if minSize := megabytes(def.MinSize) * int64(runtime); size < minSize {
		return reject(acquire.ReasonBelowMinimumSize, "%s is smaller than minimum allowed %s (for %s)", sizeSuffix(size), sizeSuffix(minSize), runtimeMsg), nil
	}
	if def.MaxSize == 0 {
		return nil, nil
	}
	if maxSize := megabytes(def.MaxSize) * int64(runtime); size > maxSize {
		return reject(acquire.ReasonAboveMaximumSize, "%s is larger than maximum allowed %s (for %s)", sizeSuffix(size), sizeSuffix(maxSize), runtimeMsg), nil
	}
	return nil, nil
}

// AlreadyImported rejects the release last grabbed and imported again
// for an episode when its imported quality differed from the grabbed
// one, so that it is not grabbed over and over.
type AlreadyImported struct{ permanent }

// Name implements Spec.
func (AlreadyImported) Name() string { return "AlreadyImported" }

// Check implements Spec.
func (AlreadyImported) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if !e.Config.CompletedDownloadHandling {
		return nil, nil
	}
	r := c.Release
	for _, ep := range c.Episodes {
		if ep.File == nil {
			continue
		}
		history, err := e.Ports.History(ctx, []core.ID{ep.ID})
		if err != nil {
			return nil, err
		}
		gi := slices.IndexFunc(history, func(h acquire.HistoryEvent) bool { return h.Type == acquire.HistoryGrabbed })
		if gi < 0 {
			continue
		}
		grabbed := history[gi]
		ii := slices.IndexFunc(history, func(h acquire.HistoryEvent) bool {
			return h.Type == acquire.HistoryImported && h.DownloadID == grabbed.DownloadID
		})
		if ii < 0 || grabbed.Quality.Equal(history[ii].Quality) {
			continue
		}
		if r.Protocol == acquire.ProtocolTorrent && r.InfoHash != "" && strings.ToUpper(r.InfoHash) == grabbed.DownloadID {
			return reject(acquire.ReasonAlreadyImportedSameHash, "Has same torrent hash as a grabbed and imported release"), nil
		}
		if strings.EqualFold(r.Title, grabbed.SourceTitle) {
			return reject(acquire.ReasonAlreadyImportedSameName, "Has same release name as a grabbed and imported release"), nil
		}
	}
	return nil, nil
}

// AnimeVersionUpgrade rejects new versions of anime episodes by another
// release group than the existing file's.
type AnimeVersionUpgrade struct{ permanent }

// Name implements Spec.
func (AnimeVersionUpgrade) Name() string { return "AnimeVersionUpgrade" }

// Check implements Spec.
func (AnimeVersionUpgrade) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if c.Series.Type != Anime || e.Config.Propers == acquire.PropersDoNotPrefer {
		return nil, nil
	}
	group := c.Parsed.ReleaseGroup
	for _, f := range c.files() {
		if !acquire.IsRevisionUpgrade(f.Quality, c.Quality) {
			continue
		}
		if strings.TrimSpace(f.ReleaseGroup) == "" {
			return reject(ReasonUnknownReleaseGroup, "Existing release group is unknown"), nil
		}
		if strings.TrimSpace(group) == "" {
			return reject(ReasonUnknownReleaseGroup, "Release group is unknown"), nil
		}
		if f.ReleaseGroup != group {
			return reject(ReasonReleaseGroupDoesNotMatch, "%s does not match existing release group %s", group, f.ReleaseGroup), nil
		}
	}
	return nil, nil
}

// CustomFormatAllowedByProfile rejects releases whose custom formats
// score below the series profile's minimum.
type CustomFormatAllowedByProfile struct{ permanent }

// Name implements Spec.
func (CustomFormatAllowedByProfile) Name() string { return "CustomFormatAllowedByProfile" }

// Check implements Spec.
func (CustomFormatAllowedByProfile) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if minScore := c.Series.Profile.MinFormatScore; c.FormatScore < minScore {
		return reject(acquire.ReasonCustomFormatMinimumScore, "Custom Formats %s have score %d below Series profile minimum %d", formatNames(c.Formats), c.FormatScore, minScore), nil
	}
	return nil, nil
}

// FullSeason rejects season packs of seasons not fully aired.
type FullSeason struct{ permanent }

// Name implements Spec.
func (FullSeason) Name() string { return "FullSeason" }

// Check implements Spec.
func (FullSeason) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if !c.Parsed.FullSeason {
		return nil, nil
	}
	aired := e.now().Add(24 * time.Hour)
	if slices.ContainsFunc(c.Episodes, func(ep *Episode) bool { return ep.AirDateUTC.IsZero() || ep.AirDateUTC.After(aired) }) {
		return reject(ReasonFullSeasonNotAired, "Full season release rejected. All episodes haven't aired yet."), nil
	}
	return nil, nil
}

// MultiSeason rejects releases of several seasons.
type MultiSeason struct{ permanent }

// Name implements Spec.
func (MultiSeason) Name() string { return "MultiSeason" }

// Check implements Spec.
func (MultiSeason) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if c.Parsed.IsMultiSeason {
		return reject(ReasonMultiSeason, "Multi-season releases are not supported"), nil
	}
	return nil, nil
}

// Queue rejects releases that would not upgrade a download of the same
// episodes already in the queue.
type Queue struct{ permanent }

// Name implements Spec.
func (Queue) Name() string { return "Queue" }

// Check implements Spec.
func (Queue) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	queue, err := e.Ports.Queue(ctx)
	if err != nil {
		return nil, err
	}
	p := c.Series.Profile
	u := e.upgrades()
	ids := episodeIDs(c.Episodes)
	for _, q := range queue {
		if q.TargetID != c.Series.ID || !slices.ContainsFunc(q.ItemIDs, func(id core.ID) bool { return slices.Contains(ids, id) }) {
			continue
		}
		// A failed download waiting to be removed is being replaced.
		if q.FailedPending {
			continue
		}
		if !u.CutoffNotMet(p, q.Quality, q.Formats, &c.Quality) {
			return reject(acquire.ReasonQueueCutoffMet, "Release in queue already meets cutoff: %s", q.Quality), nil
		}
		switch u.IsUpgradable(p, q.Quality, q.Formats, c.Quality, c.Formats) {
		case acquire.UpgradeBetterQuality:
			return reject(acquire.ReasonQueueHigherPreference, "Release in queue is of equal or higher preference: %s", q.Quality), nil
		case acquire.UpgradeBetterRevision:
			return reject(acquire.ReasonQueueHigherRevision, "Release in queue is of equal or higher revision: %s", revisionString(q.Quality.Revision)), nil
		case acquire.UpgradeQualityCutoff:
			return reject(acquire.ReasonQueueCutoffMet, "Release in queue meets quality cutoff: %s", cutoffItem(p, p.Cutoff)), nil
		case acquire.UpgradeCustomFormatCutoff:
			return reject(acquire.ReasonQueueCustomFormatCutoffMet, "Release in queue meets Custom Format cutoff: %d", p.CutoffFormatScore), nil
		case acquire.UpgradeCustomFormatScore:
			return reject(acquire.ReasonQueueCustomFormatScore, "Release in queue has an equal or higher Custom Format score: %d", p.FormatScore(q.Formats)), nil
		case acquire.UpgradeMinCustomFormatScore:
			return reject(acquire.ReasonQueueCustomFormatScoreIncrement, "Release in queue has Custom Format score within Custom Format score increment: %d", p.MinUpgradeFormatScore), nil
		case acquire.UpgradeNotAllowed:
			return reject(acquire.ReasonQueueUpgradesNotAllowed, "Release in queue and Quality Profile '%s' does not allow upgrades", p.Name), nil
		}
		if acquire.IsRevisionUpgrade(q.Quality, c.Quality) && e.Config.Propers == acquire.PropersDoNotUpgrade {
			return reject(acquire.ReasonQueuePropersDisabled, "Proper downloading is disabled"), nil
		}
	}
	return nil, nil
}

// SameEpisodesGrab rejects releases that lack episodes an existing file
// of theirs holds, as replacing the file would lose them.
type SameEpisodesGrab struct{ permanent }

// Name implements Spec.
func (SameEpisodesGrab) Name() string { return "SameEpisodesGrab" }

// Check implements Spec.
func (SameEpisodesGrab) Check(ctx context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	ok, err := SameEpisodes(ctx, e.Episodes, c.Episodes)
	if err != nil || ok {
		return nil, err
	}
	return reject(ReasonExistingFileHasMoreEpisodes, "Episode file on disk contains more episodes than this release contains"), nil
}

// SameEpisodes reports whether the files of episodes hold no other
// episodes, as Sonarr's SameEpisodesSpecification.
func SameEpisodes(ctx context.Context, ports EpisodePorts, episodes []*Episode) (bool, error) {
	ids := episodeIDs(episodes)
	var seen []core.ID
	for _, ep := range episodes {
		if ep.File == nil || slices.Contains(seen, ep.File.ID) {
			continue
		}
		seen = append(seen, ep.File.ID)
		inFile, err := ports.EpisodeIDsInFile(ctx, ep.File.ID)
		if err != nil {
			return false, err
		}
		if slices.ContainsFunc(inFile, func(id core.ID) bool { return !slices.Contains(ids, id) }) {
			return false, nil
		}
	}
	return true, nil
}

// SceneNumbering holds back releases found by a scene mapping whose
// numbering is ambiguous, as Sonarr's SceneMappingSpecification.
type SceneNumbering struct{ temporary }

// Name implements Spec.
func (SceneNumbering) Name() string { return "SceneMapping" }

// Check implements Spec.
func (SceneNumbering) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	m := c.SceneMapping
	if m == nil || strings.TrimSpace(m.SceneOrigin) == "" {
		return nil, nil
	}
	origin, _, _ := strings.Cut(m.SceneOrigin, ":")
	if origin != "mixed" {
		return nil, nil
	}
	if strings.TrimSpace(m.Comment) != "" {
		// Sonarr leaves the placeholder unformatted.
		return &acquire.Rejection{Reason: ReasonAmbiguousNumbering, Message: "{0} has ambiguous numbering"}, nil
	}
	return reject(ReasonAmbiguousNumbering, "Ambiguous numbering"), nil
}

// SeasonPackOnly rejects, in season searches, single episodes of a
// standard series' season whose last episode aired longer ago than the
// indexer's SeasonSearchMaximumSingleEpisodeAge, so that a season pack
// is grabbed instead.
type SeasonPackOnly struct{ permanent }

// Name implements Spec.
func (SeasonPackOnly) Name() string { return "SeasonPackOnly" }

// Check implements Spec.
func (SeasonPackOnly) Check(_ context.Context, e *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s == nil || len(s.Episodes) == 1 {
		return nil, nil
	}
	days := c.SeasonSearchMaximumSingleEpisodeAge
	if days <= 0 || c.Series.Type != Standard || c.Parsed.FullSeason || len(c.Episodes) == 0 {
		return nil, nil
	}
	season := c.Episodes[0].SeasonNumber
	var last time.Time
	found := false
	for _, ep := range s.Episodes {
		if ep.AirDateUTC.IsZero() || ep.SeasonNumber != season {
			continue
		}
		found = true
		if ep.AirDateUTC.After(last) {
			last = ep.AirDateUTC
		}
	}
	if found && last.Before(e.now().AddDate(0, 0, -days)) {
		return reject(ReasonNotSeasonPack, "Last episode in this season aired more than %d days ago, season pack required.", days), nil
	}
	return nil, nil
}

// SplitEpisode rejects releases of part of an episode.
type SplitEpisode struct{ permanent }

// Name implements Spec.
func (SplitEpisode) Name() string { return "SplitEpisode" }

// Check implements Spec.
func (SplitEpisode) Check(_ context.Context, _ *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	if c.Parsed.IsSplitEpisode {
		return reject(ReasonSplitEpisode, "Split episode releases are not supported"), nil
	}
	return nil, nil
}

// UpgradeDisk rejects releases that would not upgrade the existing
// episode files.
type UpgradeDisk struct{ permanent }

// Name implements Spec.
func (UpgradeDisk) Name() string { return "UpgradeDisk" }

// Check implements Spec.
func (UpgradeDisk) Check(_ context.Context, e *Env, c *Candidate, _ *Search) (*acquire.Rejection, error) {
	p := c.Series.Profile
	u := e.upgrades()
	for _, f := range c.files() {
		if !u.CutoffNotMet(p, f.Quality, f.Formats, &c.Quality) {
			return reject(acquire.ReasonDiskCutoffMet, "Existing file meets cutoff: %s", cutoffItem(p, effectiveCutoff(p))), nil
		}
		switch u.IsUpgradable(p, f.Quality, f.Formats, c.Quality, c.Formats) {
		case acquire.UpgradeBetterQuality:
			return reject(acquire.ReasonDiskHigherPreference, "Existing file on disk is of equal or higher preference: %s", f.Quality), nil
		case acquire.UpgradeBetterRevision:
			return reject(acquire.ReasonDiskHigherRevision, "Existing file on disk is of equal or higher revision: %s", revisionString(f.Quality.Revision)), nil
		case acquire.UpgradeQualityCutoff:
			return reject(acquire.ReasonDiskCutoffMet, "Existing file on disk meets quality cutoff: %s", cutoffItem(p, p.Cutoff)), nil
		case acquire.UpgradeCustomFormatCutoff:
			return reject(acquire.ReasonDiskCustomFormatCutoffMet, "Existing file on disk meets Custom Format cutoff: %d", p.CutoffFormatScore), nil
		case acquire.UpgradeCustomFormatScore:
			return reject(acquire.ReasonDiskCustomFormatScore, "Existing file on disk has a equal or higher Custom Format score: %d", p.FormatScore(f.Formats)), nil
		case acquire.UpgradeMinCustomFormatScore:
			return reject(acquire.ReasonDiskCustomFormatScoreIncrement, "Existing file on disk has Custom Format score within Custom Format score increment: %d", p.MinUpgradeFormatScore), nil
		case acquire.UpgradeNotAllowed:
			return reject(acquire.ReasonDiskUpgradesNotAllowed, "Existing file on disk and Quality Profile '%s' does not allow upgrades", p.Name), nil
		}
	}
	return nil, nil
}

// Delay holds releases back for the delay of their protocol, waiting for
// a better one, unless they are already the best the profile wants or a
// new revision of an existing file's quality.
type Delay struct{ temporary }

// Name implements Spec.
func (Delay) Name() string { return "Delay" }

// Check implements Spec.
func (Delay) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s != nil && s.UserInvoked {
		return nil, nil
	}
	p := c.Series.Profile
	dp := e.delayProfile(c.Series.Tags)
	delay := dp.Delay(c.Release.Protocol)
	preferred := c.Release.Protocol == dp.PreferredProtocol
	if delay == 0 {
		return nil, nil
	}
	if preferred {
		for _, f := range c.files() {
			if p.CompareQualities(c.Quality.Quality, f.Quality.Quality, false) == 0 && c.Quality.Revision.Compare(f.Quality.Revision) > 0 {
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
	pending, err := e.Episodes.Pending(ctx, c.Series.ID, episodeIDs(c.Episodes))
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
		return reject(acquire.ReasonMinimumAgeDelay, "Waiting for better quality release"), nil
	}
	return nil, nil
}

// DeletedEpisodeFile holds back, during RSS syncs, releases of episodes
// whose files went missing from disk while missing episodes are no
// longer monitored, until the episodes are unmonitored.
type DeletedEpisodeFile struct{}

// Name implements Spec.
func (DeletedEpisodeFile) Name() string { return "DeletedEpisodeFile" }

// Priority implements Spec.
func (DeletedEpisodeFile) Priority() acquire.Priority { return acquire.PriorityDisk }

// Temporary implements Spec.
func (DeletedEpisodeFile) Temporary() bool { return true }

// Check implements Spec.
func (DeletedEpisodeFile) Check(ctx context.Context, e *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if !e.AutoUnmonitorPreviouslyDownloaded || s != nil {
		return nil, nil
	}
	for _, f := range c.distinctFiles() {
		ok, err := e.Episodes.FileExists(ctx, c.Series, f)
		if err != nil {
			return nil, err
		}
		if !ok {
			return reject(acquire.ReasonNotMonitored, "Series is not monitored"), nil
		}
	}
	return nil, nil
}

// MonitoredEpisode rejects releases of unmonitored series or episodes,
// unless a search asked for unmonitored episodes too.
type MonitoredEpisode struct{ permanent }

// Name implements Spec.
func (MonitoredEpisode) Name() string { return "MonitoredEpisode" }

// Check implements Spec.
func (MonitoredEpisode) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s != nil && !s.MonitoredEpisodesOnly {
		return nil, nil
	}
	if !c.Series.Monitored {
		return reject(acquire.ReasonNotMonitored, "Series is not monitored"), nil
	}
	if !slices.ContainsFunc(c.Episodes, func(ep *Episode) bool { return !ep.Monitored }) {
		return nil, nil
	}
	if len(c.Episodes) == 1 {
		return reject(ReasonEpisodeNotMonitored, "Episode is not monitored"), nil
	}
	return reject(ReasonEpisodeNotMonitored, "One or more episodes is not monitored"), nil
}

// EpisodeRequested rejects releases a search found for none of the
// episodes it asked for.
type EpisodeRequested struct{ permanent }

// Name implements Spec.
func (EpisodeRequested) Name() string { return "EpisodeRequested" }

// Check implements Spec.
func (EpisodeRequested) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s == nil {
		return nil, nil
	}
	wanted := episodeIDs(s.Episodes)
	if slices.ContainsFunc(c.Episodes, func(ep *Episode) bool { return slices.Contains(wanted, ep.ID) }) {
		return nil, nil
	}
	if len(c.Episodes) == 0 {
		return reject(ReasonWrongEpisode, "Episode wasn't requested"), nil
	}
	episodes := slices.Clone(c.Episodes)
	slices.SortStableFunc(episodes, func(a, b *Episode) int {
		if a.SeasonNumber != b.SeasonNumber {
			return a.SeasonNumber - b.SeasonNumber
		}
		return a.EpisodeNumber - b.EpisodeNumber
	})
	first, last := episodes[0], episodes[len(episodes)-1]
	if len(episodes) > 1 {
		return reject(ReasonWrongEpisode, "Episode wasn't requested: %dx%d-%d", first.SeasonNumber, first.EpisodeNumber, last.EpisodeNumber), nil
	}
	return reject(ReasonWrongEpisode, "Episode wasn't requested: %dx%d", first.SeasonNumber, first.EpisodeNumber), nil
}

// SeasonMatch rejects releases a season search found for another
// season.
type SeasonMatch struct{ permanent }

// Name implements Spec.
func (SeasonMatch) Name() string { return "SeasonMatch" }

// Check implements Spec.
func (SeasonMatch) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s == nil || s.Kind != SeasonSearch || s.SeasonNumber == c.Parsed.SeasonNumber {
		return nil, nil
	}
	return reject(ReasonWrongSeason, "Wrong season"), nil
}

// SingleEpisodeSearchMatch rejects releases an episode search found for
// another season or episode, or season packs.
type SingleEpisodeSearchMatch struct{ permanent }

// Name implements Spec.
func (SingleEpisodeSearchMatch) Name() string { return "SingleEpisodeSearchMatch" }

// Check implements Spec.
func (SingleEpisodeSearchMatch) Check(_ context.Context, _ *Env, c *Candidate, s *Search) (*acquire.Rejection, error) {
	if s == nil {
		return nil, nil
	}
	switch s.Kind {
	case SingleEpisodeSearch:
		if s.SeasonNumber != c.Parsed.SeasonNumber {
			return reject(ReasonWrongSeason, "Wrong season"), nil
		}
		if len(c.Parsed.EpisodeNumbers) == 0 {
			return reject(ReasonFullSeason, "Full season pack"), nil
		}
		if !slices.Contains(c.Parsed.EpisodeNumbers, s.EpisodeNumber) {
			return reject(ReasonWrongEpisode, "Wrong episode"), nil
		}
	case AnimeEpisodeSearch:
		if c.Parsed.FullSeason && !s.IsSeasonSearch {
			return reject(ReasonFullSeason, "Full season pack"), nil
		}
	}
	return nil, nil
}

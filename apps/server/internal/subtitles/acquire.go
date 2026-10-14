package subtitles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// SearchItem searches for the subtitles a movie or an episode is missing
// and downloads the best of each language, as Bazarr's
// generate_subtitles: languages whose searches failed lately are left
// out by adaptive searching, and those not found are recorded for it. It
// returns the downloads' history events.
func (r *Runtime) SearchItem(ctx context.Context, itemID core.ID) ([]core.SubtitleHistoryEvent, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return nil, err
	}
	missing := t.missing(&s)
	if err := r.keepWanted(ctx, t, missing); err != nil {
		return nil, err
	}
	if len(missing) == 0 {
		return nil, nil
	}
	attempts, err := r.Store.SubtitleSearchAttempts().List(ctx, itemID)
	if err != nil {
		return nil, err
	}
	now := r.now()
	var langs []core.SubtitleLanguage
	for _, l := range missing {
		i := slices.IndexFunc(attempts, func(a core.SubtitleSearchAttempt) bool { return a.Language == l })
		var a *core.SubtitleSearchAttempt
		if i >= 0 {
			a = &attempts[i]
		}
		if subtitles.SearchActive(&s, a, now) {
			langs = append(langs, l)
		}
	}
	if len(langs) == 0 {
		return nil, nil
	}
	events, err := r.acquire(ctx, t, &s, langs, acquisition{action: core.SubtitleDownloaded})
	for _, l := range langs {
		if slices.ContainsFunc(events, func(e core.SubtitleHistoryEvent) bool { return e.Language == l }) {
			continue
		}
		if err := r.Store.SubtitleSearchAttempts().Record(ctx, itemID, l, now); err != nil {
			return events, err
		}
	}
	return events, err
}

// acquisition says how acquire chooses and records subtitles.
type acquisition struct {
	action core.SubtitleAction
	// minScore, when set, is the lowest score taken; otherwise the
	// settings' minimum.
	minScore int
	// upgradedFrom is the event an upgrade replaces.
	upgradedFrom core.ID
}

// acquire searches for subtitles of the target in langs and downloads the
// best of each, as Bazarr's download_best_subtitles: candidates are
// filtered by the blocklist and the profile's words, ranked by score and
// tried in order until one downloads; subtitles that cannot be used are
// blocklisted. It returns the downloads' history events.
func (r *Runtime) acquire(ctx context.Context, t *target, s *core.SubtitleSettings, langs []core.SubtitleLanguage, a acquisition) ([]core.SubtitleHistoryEvent, error) {
	mode := func(l core.SubtitleLanguage) subtitles.HIMode {
		if t.profile == nil {
			return subtitles.HIAllowed
		}
		return subtitles.HIModeFor(t.profile, l)
	}
	video := r.pluginVideo(ctx, t, s)
	all, _ := r.search(ctx, t, video, langs, mode)
	if len(all) == 0 {
		return nil, nil
	}
	blocklist, err := r.blocklist(ctx, t.item.ID)
	if err != nil {
		return nil, err
	}
	var ban *subtitles.BanList
	if t.profile != nil {
		if ban, err = subtitles.NewBanList(t.profile); err != nil {
			return nil, err
		}
	}
	byKey := map[[2]string]*found{}
	cands := make([]subtitles.Candidate, len(all))
	for i := range all {
		byKey[[2]string{all[i].Provider, all[i].ID}] = &all[i]
		cands[i] = all[i].Candidate
	}
	cands = subtitles.Filter(cands, subtitles.LanguageEquals(s.LanguageEquals), ban, blocklist)
	minScore := a.minScore
	if minScore == 0 {
		minScore = subtitles.MinScore(s, t.episode())
	}
	var events []core.SubtitleHistoryEvent
	var errs []error
	for _, l := range langs {
		for _, ranked := range subtitles.Choose(cands, t.video, []core.SubtitleLanguage{l}, mode(l), minScore) {
			if ranked.Language.Forced != l.Forced {
				continue
			}
			f := byKey[[2]string{ranked.Provider, ranked.ID}]
			e, err := r.download(ctx, t, f, ranked, video, s, a)
			if errors.Is(err, errBadSubtitle) {
				r.logger().WarnContext(ctx, "blocklisting unusable subtitle", "item", t.item.ID, "provider", f.Provider, "subtitle", f.ID, "err", err)
				if err := r.addBlocklist(ctx, t, f.Provider, f.ID, ranked.Language, err.Error()); err != nil {
					return events, err
				}
				continue
			}
			if err != nil {
				// A failure to save is the server's, not the subtitle's:
				// the language is given up for now.
				errs = append(errs, err)
				break
			}
			if e == nil {
				continue
			}
			events = append(events, *e)
			break
		}
	}
	return events, errors.Join(errs...)
}

// download fetches and saves a ranked subtitle and records it. It returns
// nil for a subtitle the provider failed to give, errBadSubtitle for one
// that cannot be used, and other errors for failures to save.
func (r *Runtime) download(ctx context.Context, t *target, f *found, ranked subtitles.Ranked, video *pluginv1.SubtitleVideo, s *core.SubtitleSettings, a acquisition) (*core.SubtitleHistoryEvent, error) {
	data, format, err := r.fetch(ctx, t, f, video, s)
	if errors.Is(err, errBadSubtitle) {
		return nil, err
	}
	if err != nil {
		r.logger().WarnContext(ctx, "subtitle download failed", "item", t.item.ID, "provider", f.Provider, "err", err)
		return nil, nil
	}
	p, err := r.save(ctx, t, ranked.Language, format, data, s)
	if err != nil {
		return nil, fmt.Errorf("save subtitle of %s: %w", t.item.Name, err)
	}
	maxScore := subtitles.MaxScore(t.episode())
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: a.action, ItemID: t.item.ID, Date: r.now(), Language: ranked.Language,
		Provider: f.Provider, SubtitleID: f.ID, Score: min(ranked.Score, maxScore), MaxScore: maxScore,
		Path: p, VideoPath: t.source().Path, Matched: ranked.Final.Sorted(), NotMatched: notMatched(ranked.Final, t.episode()),
		UpgradedFrom: a.upgradedFrom,
	}
	e.Description = fmt.Sprintf("%s subtitles downloaded from %s with a score of %d%%.", ranked.Language, f.Provider, percent(e.Score, maxScore))
	if t.series != nil {
		e.ParentID = t.series.ID
	}
	if err := r.record(ctx, t, &e); err != nil {
		return nil, err
	}
	r.syncDownloaded(ctx, t, &e, s)
	return &e, nil
}

// notMatched returns the properties scoring counts that a subtitle did
// not match.
func notMatched(final subtitles.Matches, episode bool) []string {
	var out []string
	for prop := range subtitles.ScoresFor(episode) {
		if !final[prop] && prop != subtitles.MatchHash {
			out = append(out, prop)
		}
	}
	slices.Sort(out)
	return out
}

func percent(score, maxScore int) int {
	if maxScore <= 0 {
		return 0
	}
	return score * 100 / maxScore
}

// record adds a history event, publishes it as an activity and finds
// anew what the item is missing.
func (r *Runtime) record(ctx context.Context, t *target, e *core.SubtitleHistoryEvent) error {
	if err := r.Store.SubtitleHistory().Add(ctx, *e); err != nil {
		return fmt.Errorf("record subtitle history: %w", err)
	}
	r.updateWanted(ctx, t.item.ID)
	attrs := map[string]string{"language": e.Language.String(), "action": string(e.Action), "path": e.Path}
	if e.Provider != "" {
		attrs["provider"], attrs["subtitle"] = e.Provider, e.SubtitleID
	}
	if e.MaxScore > 0 {
		attrs["score"] = strconv.Itoa(percent(e.Score, e.MaxScore))
	}
	r.publish(core.Activity{
		Type: "subtitle." + string(e.Action), ItemID: t.item.ID, Title: titleOf(e.Action) + " " + t.displayName(),
		Message: e.Description, Attributes: attrs,
	})
	return nil
}

func titleOf(a core.SubtitleAction) string {
	switch a {
	case core.SubtitleDownloaded, core.SubtitleManual:
		return "Downloaded a subtitle of"
	case core.SubtitleUpgraded:
		return "Upgraded a subtitle of"
	case core.SubtitleUploaded:
		return "Uploaded a subtitle of"
	case core.SubtitleSynced:
		return "Synchronized a subtitle of"
	case core.SubtitleDeleted:
		return "Deleted a subtitle of"
	case core.SubtitleExtracted:
		return "Extracted a subtitle of"
	}
	return "Changed a subtitle of"
}

// displayName names a movie, or an episode with its series and numbers.
func (t *target) displayName() string {
	if t.series == nil || t.item.ParentIndexNumber == nil || t.item.IndexNumber == nil {
		return t.item.Name
	}
	return fmt.Sprintf("%s S%02dE%02d", t.series.Name, *t.item.ParentIndexNumber, *t.item.IndexNumber)
}

// blocklist returns the blocklisted subtitles of an item.
func (r *Runtime) blocklist(ctx context.Context, itemID core.ID) ([]core.SubtitleBlocklistEntry, error) {
	page, err := r.Store.SubtitleBlocklist().List(ctx, itemID, 0, 0)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// addBlocklist blocklists a subtitle for the target.
func (r *Runtime) addBlocklist(ctx context.Context, t *target, provider, id string, l core.SubtitleLanguage, message string) error {
	b := core.SubtitleBlocklistEntry{
		ID: core.NewID(), ItemID: t.item.ID, Provider: provider, SubtitleID: id, Language: l, Date: r.now(), Message: message,
	}
	if t.series != nil {
		b.ParentID = t.series.ID
	}
	return r.Store.SubtitleBlocklist().Add(ctx, &b)
}

// SearchWanted searches for the missing subtitles of every movie, or
// every episode, in turn.
func (r *Runtime) SearchWanted(ctx context.Context, kind core.ItemKind) error {
	if len(r.providers()) == 0 {
		return nil
	}
	ids, err := r.wantedIDs(ctx, kind)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := r.SearchItem(ctx, id); err != nil {
			r.logger().WarnContext(ctx, "subtitle search failed", "item", id, "err", err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d of %d subtitle searches failed: %w", len(errs), len(ids), errs[0])
	}
	return nil
}

// Upgrade searches for better subtitles than those downloaded within the
// upgrade window below the maximum score, as Bazarr's upgrade_subtitles,
// and replaces them with those scoring higher.
func (r *Runtime) Upgrade(ctx context.Context) error {
	s := r.settings()
	if !s.Upgrade || len(r.providers()) == 0 {
		return nil
	}
	now := r.now()
	var events []core.SubtitleHistoryEvent
	for offset := 0; ; {
		page, err := r.Store.SubtitleHistory().List(ctx, core.SubtitleHistoryQuery{Since: now.AddDate(0, 0, -s.UpgradeDays), Limit: 500, Offset: offset})
		if err != nil {
			return err
		}
		events = append(events, page.Items...)
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			break
		}
	}
	var errs []error
	for _, u := range subtitles.Upgradable(events, &s, now) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.upgrade(ctx, u, &s); err != nil {
			r.logger().WarnContext(ctx, "subtitle upgrade failed", "item", u.Event.ItemID, "err", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *Runtime) upgrade(ctx context.Context, u subtitles.Upgrade, s *core.SubtitleSettings) error {
	t, err := r.load(ctx, u.Event.ItemID, s)
	if errors.Is(err, core.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// The video was replaced since, or the profile no longer wants the
	// language.
	if t.source().Path != u.Event.VideoPath || t.profile == nil || !subtitles.StillDesired(t.profile, u.Event.Language) {
		return nil
	}
	l := subtitles.UpgradeLanguage(t.profile, u.Event.Language)
	_, err = r.acquire(ctx, t, s, []core.SubtitleLanguage{l}, acquisition{
		action: core.SubtitleUpgraded, minScore: subtitles.UpgradeMinScore(u.Event), upgradedFrom: u.Event.ID,
	})
	return err
}

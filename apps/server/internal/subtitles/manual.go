package subtitles

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// ItemSubtitles are a movie's or an episode's subtitles and profile.
type ItemSubtitles struct {
	Profile *core.SubtitleProfile
	// Assigned is the profile assigned to the movie or series, or else
	// to its library.
	Assigned core.ID
	Streams  []ItemSubtitle
	Missing  []core.SubtitleLanguage
}

// ItemSubtitle is a subtitle stream of a video.
type ItemSubtitle struct {
	Stream   core.MediaStream
	Language core.SubtitleLanguage
	// Counted is set for streams that provide their language.
	Counted bool
}

// Item returns a movie's or an episode's subtitles, profile and missing
// subtitles.
func (r *Runtime) Item(ctx context.Context, itemID core.ID) (ItemSubtitles, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return ItemSubtitles{}, err
	}
	out := ItemSubtitles{Profile: t.profile, Assigned: t.assigned, Missing: t.missing(&s)}
	for _, st := range t.subs {
		out.Streams = append(out.Streams, ItemSubtitle{Stream: st.stream, Language: st.language, Counted: st.counted})
	}
	return out, nil
}

// Scored is a subtitle a manual search found.
type Scored struct {
	Provider, ID string
	Language     core.SubtitleLanguage
	ReleaseNames []string
	Score        int
	MaxScore     int
	Matched      []string
	NotMatched   []string
	PageLink     string
	Uploader     string
	Format       string
	Blocklisted  bool
}

// ManualSearch searches for subtitles of an item in langs, or in its
// profile's languages when none are given, and returns them all scored,
// best first, as Bazarr's manual search does: the blocklisted are marked
// rather than left out, and the minimum score does not apply.
func (r *Runtime) ManualSearch(ctx context.Context, itemID core.ID, langs []core.SubtitleLanguage) ([]Scored, []string, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return nil, nil, err
	}
	if len(langs) == 0 && t.profile != nil {
		for _, it := range t.profile.Items {
			langs = append(langs, core.SubtitleLanguage{Language: it.Language, Forced: it.Forced, HearingImpaired: it.HI == core.SubtitleHIRequired})
		}
	}
	if len(langs) == 0 {
		return nil, nil, fmt.Errorf("%w: no languages to search subtitles in", core.ErrInvalid)
	}
	mode := func(l core.SubtitleLanguage) subtitles.HIMode {
		if l.HearingImpaired {
			return subtitles.HIForce
		}
		return subtitles.HIAllowed
	}
	all, failed := r.search(ctx, t, r.pluginVideo(ctx, t, &s), langs, mode)
	blocklist, err := r.blocklist(ctx, itemID)
	if err != nil {
		return nil, nil, err
	}
	maxScore := subtitles.MaxScore(t.episode())
	equals := subtitles.LanguageEquals(s.LanguageEquals)
	out := make([]Scored, 0, len(all))
	for _, f := range all {
		f.Language = equals.Apply(f.Language)
		hi := f.Language.HearingImpaired
		score, _, final := subtitles.Score(f.Matches, subtitles.ScoreSubject{HashVerifiable: f.HashVerifiable, HearingImpaired: hi}, t.video, &hi)
		out = append(out, Scored{
			Provider: f.Provider, ID: f.ID, Language: f.Language, ReleaseNames: ReleaseNames(f.remote.GetReleaseNames()),
			Score: min(score, maxScore), MaxScore: maxScore, Matched: final.Sorted(), NotMatched: notMatched(final, t.episode()),
			PageLink: f.remote.GetPageLink(), Uploader: f.remote.GetUploader(), Format: f.remote.GetFormat(),
			Blocklisted: slices.ContainsFunc(blocklist, func(b core.SubtitleBlocklistEntry) bool {
				return b.Provider == f.Provider && b.SubtitleID == f.ID
			}),
		})
	}
	slices.SortStableFunc(out, func(a, b Scored) int { return cmp.Compare(b.Score, a.Score) })
	return out, failed, nil
}

// ManualDownload downloads a subtitle a manual search found, by its
// provider ("<plugin>/<provider>") and ID, in a language, and returns its
// history event.
func (r *Runtime) ManualDownload(ctx context.Context, itemID core.ID, provider, id string, l core.SubtitleLanguage) (core.SubtitleHistoryEvent, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	pluginID, name, ok := strings.Cut(provider, "/")
	if !ok {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: subtitle provider %q names no plugin", core.ErrInvalid, provider)
	}
	i := slices.IndexFunc(r.providers(), func(p Provider) bool { return p.ID == pluginID })
	if i < 0 {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("subtitle provider plugin %s: %w", pluginID, core.ErrNotFound)
	}
	video := r.pluginVideo(ctx, t, &s)
	// The subtitle is described as the search found it, when it still
	// does, to score it.
	f := &found{
		Candidate: subtitles.Candidate{Provider: provider, ID: id, Language: l, Matches: subtitles.Matches{}},
		plugin:    r.providers()[i], provider: name, remote: &pluginv1.RemoteSubtitle{},
	}
	ranked := subtitles.Ranked{Candidate: f.Candidate, Final: subtitles.Matches{}}
	if all, _ := r.search(ctx, t, video, []core.SubtitleLanguage{l}, func(core.SubtitleLanguage) subtitles.HIMode { return subtitles.HIAllowed }); len(all) > 0 {
		if j := slices.IndexFunc(all, func(c found) bool { return c.Provider == provider && c.ID == id }); j >= 0 {
			f = &all[j]
			f.Language = l
			hi := l.HearingImpaired
			score, without, final := subtitles.Score(f.Matches, subtitles.ScoreSubject{HashVerifiable: f.HashVerifiable, HearingImpaired: hi}, t.video, &hi)
			ranked = subtitles.Ranked{Candidate: f.Candidate, Score: score, ScoreWithoutHash: without, Final: final}
		}
	}
	e, err := r.download(ctx, t, f, ranked, video, &s, acquisition{action: core.SubtitleManual})
	if errors.Is(err, errBadSubtitle) {
		if berr := r.addBlocklist(ctx, t, provider, id, l, err.Error()); berr != nil {
			return core.SubtitleHistoryEvent{}, berr
		}
		return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %w", core.ErrInvalid, err)
	}
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	if e == nil {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s did not give the subtitle", ErrUnavailable, provider)
	}
	return *e, nil
}

// Upload saves a subtitle file for an item in a language, as Bazarr's
// manual upload, and returns its path.
func (r *Runtime) Upload(ctx context.Context, itemID core.ID, data []byte, format string, l core.SubtitleLanguage) (string, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return "", err
	}
	original := t.profile != nil && t.profile.OriginalFormat
	data, format, err = r.prepare(data, format, l.Language, original, &s)
	if err != nil {
		return "", fmt.Errorf("%w: %w", core.ErrInvalid, err)
	}
	p, err := r.save(ctx, t, l, format, data, &s)
	if err != nil {
		return "", err
	}
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: core.SubtitleUploaded, ItemID: t.item.ID, Date: r.now(), Language: l,
		Path: p, VideoPath: t.source().Path, Description: fmt.Sprintf("%s subtitles uploaded.", l),
	}
	if t.series != nil {
		e.ParentID = t.series.ID
	}
	if err := r.record(ctx, t, &e); err != nil {
		return "", err
	}
	r.syncDownloaded(ctx, t, &e, &s)
	return p, nil
}

// fileOf returns the subtitle file of an item's video at p.
func (t *target) fileOf(p string) (subtitleStream, error) {
	i := slices.IndexFunc(t.subs, func(s subtitleStream) bool { return s.stream.ExternalPath == p })
	if i < 0 {
		return subtitleStream{}, fmt.Errorf("%w: %s is not a subtitle file of %s", core.ErrNotFound, p, t.item.Name)
	}
	return t.subs[i], nil
}

// Delete deletes a subtitle file of an item.
func (r *Runtime) Delete(ctx context.Context, itemID core.ID, p string) error {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return err
	}
	sub, err := t.fileOf(p)
	if err != nil {
		return err
	}
	if err := r.Subtitles.Remove(ctx, t.lib, itemID, p); err != nil {
		return err
	}
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: core.SubtitleDeleted, ItemID: t.item.ID, Date: r.now(), Language: sub.language,
		Path: p, VideoPath: t.source().Path, Description: fmt.Sprintf("%s subtitles deleted.", sub.language),
	}
	if t.series != nil {
		e.ParentID = t.series.ID
	}
	return r.record(ctx, t, &e)
}

// Modify applies subtitle mods to a subtitle file of an item.
func (r *Runtime) Modify(ctx context.Context, itemID core.ID, p string, mods []string) error {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return err
	}
	sub, err := t.fileOf(p)
	if err != nil {
		return err
	}
	data, err := r.Subtitles.ReadFile(ctx, t.lib, itemID, p)
	if err != nil {
		return err
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".")
	if !subtitle.CanParse(format) {
		return fmt.Errorf("%w: %s subtitles are not modified", ErrUnavailable, format)
	}
	text, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return err
	}
	parsed, err := subtitle.Parse(text, format)
	if err != nil {
		return err
	}
	if data, err = applyMods(parsed, format, sub.language.Language, mods); err != nil {
		return err
	}
	return r.Subtitles.WriteFile(ctx, t.lib, itemID, p, data)
}

// Sync synchronizes a subtitle file of an item against its video's audio.
func (r *Runtime) Sync(ctx context.Context, itemID core.ID, p string) (SyncResult, error) {
	s := r.settings()
	t, err := r.load(ctx, itemID, &s)
	if err != nil {
		return SyncResult{}, err
	}
	sub, err := t.fileOf(p)
	if err != nil {
		return SyncResult{}, err
	}
	return r.sync(ctx, t, p, sub.language, &s)
}

// BlocklistEvent blocklists the subtitle a history event downloaded,
// deletes its file and searches again for the item's missing subtitles,
// as Bazarr's history blacklisting.
func (r *Runtime) BlocklistEvent(ctx context.Context, eventID core.ID) error {
	e, err := r.historyEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if e.Provider == "" || e.SubtitleID == "" {
		return fmt.Errorf("%w: history event %s downloaded no subtitle", core.ErrInvalid, eventID)
	}
	s := r.settings()
	t, err := r.load(ctx, e.ItemID, &s)
	if err != nil {
		return err
	}
	if err := r.addBlocklist(ctx, t, e.Provider, e.SubtitleID, e.Language, "blocklisted by hand"); err != nil {
		return err
	}
	if _, err := t.fileOf(e.Path); err == nil {
		if err := r.Delete(ctx, e.ItemID, e.Path); err != nil {
			return err
		}
	}
	_, err = r.QueueSearch(ctx, []core.ID{e.ItemID})
	return err
}

// historyEvent returns a subtitle history event by ID.
func (r *Runtime) historyEvent(ctx context.Context, id core.ID) (core.SubtitleHistoryEvent, error) {
	for offset := 0; ; {
		page, err := r.Store.SubtitleHistory().List(ctx, core.SubtitleHistoryQuery{Limit: 500, Offset: offset})
		if err != nil {
			return core.SubtitleHistoryEvent{}, err
		}
		for _, e := range page.Items {
			if e.ID == id {
				return e, nil
			}
		}
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= page.Total {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("subtitle history event %s: %w", id, core.ErrNotFound)
		}
	}
}

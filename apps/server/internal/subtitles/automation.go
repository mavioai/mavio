package subtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"golang.org/x/text/language"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/subtitle"
)

// JobTranslateWanted translates the subtitles movies and episodes lack,
// every Translation.Automation.Interval.
const JobTranslateWanted = "subtitles.translate_wanted"

// TranslateWanted queues translations of the subtitles movies and
// episodes lack, as Lingarr's automation: up to MaxPerRun movies and
// episodes, taken in turns and starting after those of the run before,
// whose file is old enough get the languages they lack translated from a
// subtitle they have in one of the source languages. A translation
// pending for the same language is not queued again.
func (r *Runtime) TranslateWanted(ctx context.Context) error {
	s := r.settings()
	a := &s.Translation.Automation
	if !a.Enabled || len(a.SourceLanguages) == 0 {
		return nil
	}
	movies, err := r.wantedIDs(ctx, core.KindMovie)
	if err != nil {
		return err
	}
	episodes, err := r.wantedIDs(ctx, core.KindEpisode)
	if err != nil {
		return err
	}
	// Movies and episodes take turns.
	var ids []core.ID
	for i := range max(len(movies), len(episodes)) {
		if i < len(movies) {
			ids = append(ids, movies[i])
		}
		if i < len(episodes) {
			ids = append(ids, episodes[i])
		}
	}
	if len(ids) == 0 {
		return nil
	}
	r.autoMu.Lock()
	cursor := r.autoCursor % len(ids)
	r.autoMu.Unlock()
	now := r.now()
	queued, looked := 0, 0
	for ; looked < len(ids) && queued < a.MaxPerRun; looked++ {
		id := ids[(cursor+looked)%len(ids)]
		n, err := r.translateMissing(ctx, id, &s, now)
		if err != nil {
			r.logger().WarnContext(ctx, "translate wanted subtitles", "item", id, "err", err)
			continue
		}
		queued += n
	}
	r.autoMu.Lock()
	r.autoCursor = cursor + looked
	r.autoMu.Unlock()
	r.logger().InfoContext(ctx, "wanted subtitles translation queued", "translations", queued)
	return nil
}

// translateMissing queues translations of what a movie or an episode
// lacks, reporting how many were queued.
func (r *Runtime) translateMissing(ctx context.Context, id core.ID, s *core.SubtitleSettings, now time.Time) (int, error) {
	t, err := r.load(ctx, id, s)
	if err != nil {
		return 0, err
	}
	a := &s.Translation.Automation
	age := a.MovieAge
	if t.episode() {
		age = a.EpisodeAge
	}
	if added := t.item.DateAdded; !added.IsZero() && now.Sub(added) < age {
		return 0, nil
	}
	missing := t.missing(s)
	if len(missing) == 0 {
		return 0, nil
	}
	src, ok := r.automationSource(ctx, t, a.SourceLanguages)
	if !ok {
		return 0, nil
	}
	n := 0
	for _, l := range missing {
		if sameLanguage(l.Language, src.language.Language) {
			continue
		}
		p := TranslatePayload{ItemID: id, Path: src.path, Language: l}
		if src.path == "" {
			p.StreamIndex = &src.stream
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return n, err
		}
		job := core.Job{
			ID: core.NewID(), Kind: JobTranslate, Payload: payload, MaxAttempts: 1, RunAt: now,
			UniqueKey: fmt.Sprintf("%s:%s:%s", JobTranslate, id, l),
		}
		added, err := r.Store.Jobs().Enqueue(ctx, &job)
		if err != nil {
			return n, fmt.Errorf("queue subtitle translation: %w", err)
		}
		if added {
			n++
		}
	}
	return n, nil
}

// automationSourceSubtitle is the subtitle the automation translates from.
type automationSourceSubtitle struct {
	language core.SubtitleLanguage
	path     string
	stream   int
}

// automationSource returns the subtitle of a target the automation
// translates from: one in the first of the languages it has, a file
// before an embedded stream, and not forced.
func (r *Runtime) automationSource(ctx context.Context, t *target, languages []string) (automationSourceSubtitle, bool) {
	extractable := r.Extracted != nil && r.readable(ctx, t.source().Path)
	for _, l := range languages {
		var embedded *subtitleStream
		for i := range t.subs {
			sub := &t.subs[i]
			if sub.language.Forced || !sameLanguage(sub.language.Language, l) {
				continue
			}
			if p := sub.stream.ExternalPath; p != "" {
				if subtitle.CanParse(strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".")) {
					return automationSourceSubtitle{language: sub.language, path: p}, true
				}
				continue
			}
			if embedded == nil && extractable && sub.stream.IsTextSubtitle() {
				embedded = sub
			}
		}
		if embedded != nil {
			return automationSourceSubtitle{language: embedded.language, stream: embedded.stream.Index}, true
		}
	}
	return automationSourceSubtitle{}, false
}

// sameLanguage reports whether two language tags name the same language,
// whatever their region or script.
func sameLanguage(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	at, err := language.Parse(a)
	if err != nil {
		return false
	}
	bt, err := language.Parse(b)
	if err != nil {
		return false
	}
	return base(at) == base(bt)
}

// automationInterval is how often the translation automation runs; 0
// when it is off.
func automationInterval(s core.SubtitleSettings) time.Duration {
	if a := &s.Translation.Automation; a.Enabled && len(a.SourceLanguages) > 0 {
		return a.Interval
	}
	return 0
}

package subtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// JobProofread proofreads a translation; the payload is a
// ProofreadPayload.
const JobProofread = "subtitles.proofread"

// ProofreadPayload is the payload of JobProofread jobs: a subtitle file of
// an item the server translated, and the AI model proofreading it,
// "ai/<provider>/<model>" or empty for the first that translated it, else
// the first among the settings' translators.
type ProofreadPayload struct {
	ItemID     core.ID `json:"item_id"`
	Path       string  `json:"path"`
	Translator string  `json:"translator,omitempty"`
}

// proofreader returns the AI model proofreading a translation: the one
// named, else the first that translated it, else the first among the
// settings' translators.
func (r *Runtime) proofreader(ctx context.Context, name string, rec *core.SubtitleTranslationRecord, s *core.SubtitleTranslationSettings) (*translator, error) {
	if name == "" {
		names := slices.Concat(rec.Translators, []string{s.Translator}, s.FallbackTranslators)
		for _, n := range names {
			if strings.HasPrefix(n, aiPrefix) {
				name = n
				break
			}
		}
	}
	rest, ok := strings.CutPrefix(name, aiPrefix)
	if !ok {
		if name == "" {
			return nil, fmt.Errorf("%w: no AI model is among the subtitle translators", ErrUnavailable)
		}
		return nil, fmt.Errorf("%w: only AI models proofread, not %s", core.ErrInvalid, name)
	}
	return r.aiTranslator(ctx, rest)
}

// translationOf returns the event that recorded the translation saved at
// a path.
func (r *Runtime) translationOf(ctx context.Context, itemID core.ID, p string) (core.SubtitleHistoryEvent, error) {
	q := core.SubtitleHistoryQuery{ItemIDs: []core.ID{itemID}, Actions: []core.SubtitleAction{core.SubtitleTranslated}, Limit: core.MaxPageSize}
	for {
		page, err := r.Store.SubtitleHistory().List(ctx, q)
		if err != nil {
			return core.SubtitleHistoryEvent{}, err
		}
		for _, e := range page.Items {
			if e.Path == p && e.ItemID == itemID && e.Translation != nil {
				return e, nil
			}
		}
		q.Offset += len(page.Items)
		if len(page.Items) == 0 || q.Offset >= page.Total {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s was not translated by the server", core.ErrInvalid, p)
		}
	}
}

// QueueProofread checks a proofreading and queues it, returning its job's
// ID.
func (r *Runtime) QueueProofread(ctx context.Context, p ProofreadPayload) (core.ID, error) {
	s := r.settings()
	t, err := r.load(ctx, p.ItemID, &s)
	if err != nil {
		return core.ID{}, err
	}
	if _, err := t.fileOf(p.Path); err != nil {
		return core.ID{}, err
	}
	e, err := r.translationOf(ctx, p.ItemID, p.Path)
	if err != nil {
		return core.ID{}, err
	}
	if _, err := r.proofreader(ctx, p.Translator, e.Translation, &s.Translation); err != nil {
		return core.ID{}, err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return core.ID{}, err
	}
	job := core.Job{ID: core.NewID(), Kind: JobProofread, Payload: payload, MaxAttempts: 1, RunAt: r.now()}
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return core.ID{}, fmt.Errorf("queue subtitle proofreading: %w", err)
	}
	return job.ID, nil
}

// readCues reads and parses a subtitle.
func readCues(ctx context.Context, name string, read func(context.Context) ([]byte, string, error)) (*subtitle.Subtitle, string, error) {
	data, format, err := read(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", name, err)
	}
	text, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s: %w", core.ErrInvalid, name, err)
	}
	sub, err := subtitle.Parse(text, format)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s: %w", core.ErrInvalid, name, err)
	}
	return sub, format, nil
}

// Proofread proofreads a translation with an AI model, as Lingarr's: each
// cue's translation is given with the cue of the same timing in the
// subtitle it was translated from, and replaced by the model's answer
// when that differs. The translation is saved again and its record notes
// the cues revised.
func (r *Runtime) Proofread(ctx context.Context, p ProofreadPayload) (int, error) {
	s := r.settings()
	ts := &s.Translation
	t, err := r.load(ctx, p.ItemID, &s)
	if err != nil {
		return 0, err
	}
	file, err := t.fileOf(p.Path)
	if err != nil {
		return 0, err
	}
	e, err := r.translationOf(ctx, p.ItemID, p.Path)
	if err != nil {
		return 0, err
	}
	model, err := r.proofreader(ctx, p.Translator, e.Translation, ts)
	if err != nil {
		return 0, err
	}
	rec := *e.Translation
	srcPayload := TranslatePayload{ItemID: p.ItemID, Path: rec.SourcePath}
	if rec.SourcePath == "" {
		srcPayload.StreamIndex = &rec.SourceStreamIndex
	}
	src, err := r.translationSource(ctx, t, srcPayload)
	if err != nil {
		return 0, fmt.Errorf("the subtitle %s was translated from: %w", p.Path, err)
	}
	source, _, err := readCues(ctx, src.name, src.read)
	if err != nil {
		return 0, err
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(p.Path)), ".")
	translated, _, err := readCues(ctx, path.Base(p.Path), func(ctx context.Context) ([]byte, string, error) {
		data, err := r.Subtitles.ReadFile(ctx, t.item.ID, p.Path)
		return data, format, err
	})
	if err != nil {
		return 0, err
	}
	type timing struct{ start, end time.Duration }
	sources := map[timing]string{}
	for _, c := range source.Cues {
		if text := strings.TrimSpace(subtitle.PlainText(c.Text)); text != "" {
			sources[timing{c.Start, c.End}] = strings.Join(strings.Split(text, "\n"), " ")
		}
	}
	c := chained{translator: model, source: rec.SourceLanguage, target: e.Language.Language}
	values := map[string]string{
		"sourceLanguage": languageName(c.source, ts.LanguageCodes), "targetLanguage": languageName(c.target, ts.LanguageCodes),
		"lineToTranslate": "", "contextBefore": "", "contextAfter": "",
	}
	revised := 0
	for i, cue := range translated.Cues {
		sourceLine, ok := sources[timing{cue.Start, cue.End}]
		lines := strings.Split(cue.Text, "\n")
		line := strings.Join(lines, " ")
		if ts.PreserveLineBreaks {
			line = cue.Text
		}
		if !ok || strings.TrimSpace(line) == "" {
			continue
		}
		values["sourceLine"], values["translatedLine"] = sourceLine, line
		resp, err := r.complete(ctx, ts, &rec, model, pluginv1.CompleteAiRequest_builder{
			System:   proto.String(fillPrompt(ts.ProofreadPrompt, values)),
			Messages: []*pluginv1.AiMessage{userMessage(fillPrompt(ts.ProofreadUserPrompt, values))},
		}.Build())
		if err != nil {
			return 0, fmt.Errorf("proofread %s with %s: %w", path.Base(p.Path), model.name, err)
		}
		answer := strings.TrimSpace(resp.GetText())
		if answer == "" || answer == line {
			continue
		}
		translated.Cues[i].Text = strings.Join(subtitleLines(answer, len(lines), ts.PreserveLineBreaks, ts.StripFormatting), "\n")
		revised++
	}
	if revised > 0 {
		out, err := subtitle.Write(translated, format)
		if err != nil {
			return 0, err
		}
		if _, err := r.save(ctx, t, file.language, format, out, &s); err != nil {
			return 0, err
		}
	}
	rec.Proofread, rec.Revised = true, rec.Revised+revised
	if err := r.Store.SubtitleHistory().SetTranslation(ctx, e.ID, rec); err != nil {
		return 0, fmt.Errorf("record the proofreading: %w", err)
	}
	r.logger().InfoContext(ctx, "subtitle proofread", "item", t.item.ID, "path", p.Path, "revised", revised)
	return revised, nil
}

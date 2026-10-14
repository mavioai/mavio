package subtitles

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/subtitle"
)

// aiPrefix starts the names of the AI gateway's models as translators.
const aiPrefix = "ai/"

// translator is a translator of translations: a translator plugin's, or a
// model of the AI gateway.
type translator struct {
	// name is "<plugin>/<translator>" or "ai/<provider>/<model>"; credit
	// is what translations credit.
	name, credit string
	// sources and targets are the languages it translates from and into;
	// empty for any.
	sources, targets []string
	// plugin and info are a translator plugin's translator.
	plugin Translator
	info   *pluginv1.SubtitleTranslatorInfo
	// ai, provider and model are a model's.
	ai              pluginv1connect.AiGatewayServiceClient
	provider, model string
}

func (t *translator) isAI() bool { return t.ai != nil }

// chained is a translator of a translation with the languages it
// translates between: those asked for, or the closest it knows.
type chained struct {
	*translator
	source, target string
	tier           matchTier
}

// pluginTranslators lists the translators of the translator plugins by
// name, with the first enabled one and the plugins that failed to list
// theirs.
type pluginTranslators struct {
	byName map[string]translator
	first  string
	failed map[string]error
}

func (r *Runtime) pluginTranslators(ctx context.Context) pluginTranslators {
	out := pluginTranslators{byName: map[string]translator{}, failed: map[string]error{}}
	for _, p := range r.translators() {
		resp, err := p.Client.ListSubtitleTranslators(ctx, &pluginv1.ListSubtitleTranslatorsRequest{})
		if err != nil {
			out.failed[p.ID] = err
			continue
		}
		for _, t := range resp.GetTranslators() {
			name := providerName(p.ID, t.GetName())
			out.byName[name] = translator{
				name: name, credit: t.GetDisplayName(), sources: t.GetSourceLanguages(), targets: t.GetTargetLanguages(),
				plugin: p, info: t,
			}
			if out.first == "" && t.GetEnabled() {
				out.first = name
			}
		}
	}
	return out
}

// aiGateway returns the AI gateway plugin's client.
func (r *Runtime) aiGateway() (pluginv1connect.AiGatewayServiceClient, error) {
	if r.AI != nil {
		if c, ok := r.AI(); ok {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%w: no AI gateway plugin is running", ErrUnavailable)
}

// resolveTranslator returns the translator a name names.
func (r *Runtime) resolveTranslator(ctx context.Context, name string, plugins *pluginTranslators) (*translator, error) {
	if rest, ok := strings.CutPrefix(name, aiPrefix); ok {
		return r.aiTranslator(ctx, rest)
	}
	if t, ok := plugins.byName[name]; ok {
		if !t.info.GetEnabled() {
			return nil, fmt.Errorf("%w: subtitle translator %s is not enabled", ErrUnavailable, name)
		}
		return &t, nil
	}
	for id, err := range plugins.failed {
		if strings.HasPrefix(name, id+"/") {
			return nil, fmt.Errorf("%w: subtitle translator plugin %s: %w", ErrUnavailable, id, err)
		}
	}
	return nil, fmt.Errorf("subtitle translator %s: %w", name, core.ErrNotFound)
}

// aiTranslator returns a model of the AI gateway, "<provider>/<model>",
// which must be usable.
func (r *Runtime) aiTranslator(ctx context.Context, name string) (*translator, error) {
	provider, model, ok := strings.Cut(name, "/")
	if !ok || provider == "" || model == "" {
		return nil, fmt.Errorf("%w: translator %q is not \"ai/<provider>/<model>\"", core.ErrInvalid, aiPrefix+name)
	}
	ai, err := r.aiGateway()
	if err != nil {
		return nil, err
	}
	find := func(available bool) (*pluginv1.AiModel, error) {
		resp, err := ai.ListAiModels(ctx, pluginv1.ListAiModelsRequest_builder{Provider: &provider, Available: &available}.Build())
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, fmt.Errorf("AI provider %s: %w", provider, core.ErrNotFound)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: AI gateway: %w", ErrUnavailable, err)
		}
		i := slices.IndexFunc(resp.GetModels(), func(m *pluginv1.AiModel) bool { return m.GetId() == model })
		if i < 0 {
			return nil, nil
		}
		return resp.GetModels()[i], nil
	}
	m, err := find(false)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("AI model %s: %w", name, core.ErrNotFound)
	}
	if usable, err := find(true); err != nil || usable == nil {
		return nil, cmp.Or(err, fmt.Errorf("%w: AI provider %s has no credential", ErrUnavailable, provider))
	}
	return &translator{name: aiPrefix + name, credit: cmp.Or(m.GetName(), model), ai: ai, provider: provider, model: model}, nil
}

// chain returns the translators of a translation from source to target:
// the one named, else the settings', else the first enabled one of a
// plugin, then the settings' fallbacks; those translating between the
// languages exactly come first, as Lingarr's. A fallback that cannot be
// used is left out.
func (r *Runtime) chain(ctx context.Context, name string, s *core.SubtitleTranslationSettings, source, target string) ([]chained, error) {
	plugins := r.pluginTranslators(ctx)
	first := cmp.Or(name, s.Translator, plugins.first)
	if first == "" {
		return nil, fmt.Errorf("%w: no subtitle translator is enabled", ErrUnavailable)
	}
	names := []string{first}
	for _, n := range s.FallbackTranslators {
		if !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	var out []chained
	for i, n := range names {
		t, err := r.resolveTranslator(ctx, n, &plugins)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			r.logger().WarnContext(ctx, "fallback subtitle translator", "translator", n, "err", err)
			continue
		}
		src, st := matchLanguage(source, t.sources)
		tgt, tt := matchLanguage(target, t.targets)
		if st == matchNone || tt == matchNone {
			r.logger().InfoContext(ctx, "subtitle translator skipped", "translator", n, "source", source, "target", target)
			continue
		}
		out = append(out, chained{translator: t, source: src, target: tgt, tier: max(st, tt)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no subtitle translator translates %s to %s", ErrUnavailable, source, target)
	}
	slices.SortStableFunc(out, func(a, b chained) int { return cmp.Compare(a.tier, b.tier) })
	return out, nil
}

// textMode is how cues are given to translators.
type textMode int

const (
	// textPlain is the cues' plain text.
	textPlain textMode = iota
	// textSubRip is with SubRip's markup.
	textSubRip
	// textAsIs is as the subtitle holds it.
	textAsIs
)

// cueItem is a cue of a translation with text.
type cueItem struct {
	// cue is its index in the subtitle, position its number among those
	// translated, from 1.
	cue, position int
	start, end    time.Duration
	lines         []string
	// translated is its translation's lines; nil until translated.
	translated []string
}

// text is the cue's text as translators get it: its lines, joined with
// spaces unless line breaks are kept.
func (it *cueItem) text(preserve bool) string {
	if preserve {
		return strings.Join(it.lines, "\n")
	}
	return strings.Join(it.lines, " ")
}

// translation is a translation running.
type translation struct {
	r              *Runtime
	s              *core.SubtitleTranslationSettings
	source, target string
	candidates     []chained
	media          *pluginv1.SubtitleTranslationMedia
	logger         *slog.Logger
	items          []*cueItem
	// credit is that of the first translator that translated.
	credit string
	record *core.SubtitleTranslationRecord
}

// run translates the items in chunks of the first translator's size,
// each by the first translator that does not fail.
func (tr *translation) run(ctx context.Context) error {
	// Cues stacked with the same timing and text, as fansub ASS files
	// layer them, are translated once.
	seen := map[string]*cueItem{}
	var todo []*cueItem
	dup := map[*cueItem]*cueItem{}
	for _, it := range tr.items {
		key := fmt.Sprint(it.start, "|", it.end, "|", it.text(true))
		if first, ok := seen[key]; ok {
			dup[it] = first
			continue
		}
		seen[key] = it
		todo = append(todo, it)
	}
	size := tr.chunkSize(tr.candidates[0], len(todo))
	for start := 0; start < len(todo); start += size {
		chunk := todo[start:min(start+size, len(todo))]
		var errs []error
		done := false
		for _, c := range tr.candidates {
			err := tr.translateWith(ctx, c, chunk)
			if err == nil {
				done = true
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			tr.logger.WarnContext(ctx, "subtitle translator failed", "translator", c.name, "err", err)
			errs = append(errs, fmt.Errorf("%s: %w", c.name, err))
		}
		if !done {
			return fmt.Errorf("every translator failed for cues %d–%d: %w", chunk[0].position, chunk[len(chunk)-1].position, errors.Join(errs...))
		}
		tr.logger.DebugContext(ctx, "translating subtitle", "done", min(start+size, len(todo)), "of", len(todo))
	}
	for it, first := range dup {
		it.translated = first.translated
	}
	tr.record.Cues = len(tr.items)
	return nil
}

// chunkSize is how many cues a translator takes at once.
func (tr *translation) chunkSize(c chained, n int) int {
	size := 0
	switch {
	case c.isAI() && !tr.s.Batch:
		size = 1
	case c.isAI():
		size = tr.s.MaxBatchSize
	default:
		size = int(c.info.GetBatchSize())
	}
	if size <= 0 {
		size = n
	}
	return max(size, 1)
}

// translateWith translates the items of a chunk not yet translated with
// a translator, in its own chunks.
func (tr *translation) translateWith(ctx context.Context, c chained, chunk []*cueItem) error {
	var todo []*cueItem
	for _, it := range chunk {
		if it.translated == nil {
			todo = append(todo, it)
		}
	}
	size := tr.chunkSize(c, len(todo))
	for start := 0; start < len(todo); start += size {
		part := todo[start:min(start+size, len(todo))]
		var err error
		switch {
		case c.isAI() && !tr.s.Batch:
			err = tr.translateCue(ctx, c, part[0])
		case c.isAI():
			err = tr.translateBatch(ctx, c, part)
		default:
			err = tr.translatePlugin(ctx, c, part)
		}
		if err != nil {
			return err
		}
		if !slices.Contains(tr.record.Translators, c.name) {
			tr.record.Translators = append(tr.record.Translators, c.name)
		}
		if tr.credit == "" {
			tr.credit = c.credit
		}
		for _, it := range part {
			tr.record.Characters += int64(utf8.RuneCountInString(it.text(tr.s.PreserveLineBreaks)))
		}
	}
	return nil
}

// finish sets an item's translation from what a translator answered;
// an empty answer keeps the cue.
func (tr *translation) finish(it *cueItem, translated string) {
	if tr.s.StripFormatting {
		translated = subtitle.PlainText(translated)
	}
	if strings.TrimSpace(translated) == "" {
		it.translated = it.lines
		return
	}
	it.translated = subtitleLines(translated, len(it.lines), tr.s.PreserveLineBreaks, tr.s.StripFormatting)
}

// values are the placeholders of prompts for a translator.
func (tr *translation) values(c chained) map[string]string {
	return map[string]string{
		"sourceLanguage":  languageName(c.source, tr.s.LanguageCodes),
		"targetLanguage":  languageName(c.target, tr.s.LanguageCodes),
		"lineToTranslate": "", "contextBefore": "", "contextAfter": "",
	}
}

// contextLine is a cue given to AI models as context, with its
// translation when it has one.
type contextLine struct {
	Position    int    `json:"position"`
	Line        string `json:"line"`
	Translation string `json:"translation,omitempty"`
}

// context returns the cues before or after an item given as context, as
// Lingarr's BuildContext.
func (tr *translation) context(it *cueItem, before bool) string {
	i := it.position - 1
	from, to := i+1, min(len(tr.items), i+1+tr.s.ContextAfter)
	if before {
		from, to = max(0, i-tr.s.ContextBefore), i
	}
	var lines []string
	for _, c := range tr.items[from:to] {
		line := strings.Join(c.lines, " ")
		if !tr.s.TranslatedContext {
			lines = append(lines, line)
			continue
		}
		cl := contextLine{Position: c.position, Line: line}
		if before && c.translated != nil {
			cl.Translation = strings.TrimSpace(strings.Join(c.translated, " "))
		}
		lines = append(lines, marshalJSON(cl))
	}
	return strings.Join(lines, "\n")
}

// marshalJSON encodes v without escaping HTML.
func marshalJSON(v any) string {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	_ = e.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// translateCue translates a cue with an AI model, line by line when line
// breaks are kept, with the cues around it as context.
func (tr *translation) translateCue(ctx context.Context, c chained, it *cueItem) error {
	values := tr.values(c)
	values["contextBefore"], values["contextAfter"] = tr.context(it, true), tr.context(it, false)
	lines := []string{it.text(false)}
	if tr.s.PreserveLineBreaks && len(it.lines) > 1 {
		lines = it.lines
	}
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			out = append(out, line)
			continue
		}
		values["lineToTranslate"] = line
		resp, err := tr.r.complete(ctx, tr.s, tr.record, c.translator, pluginv1.CompleteAiRequest_builder{
			System:   proto.String(fillPrompt(tr.s.Prompt, values)),
			Messages: []*pluginv1.AiMessage{userMessage(fillPrompt(tr.s.UserPrompt, values))},
		}.Build())
		if err != nil {
			return err
		}
		out = append(out, strings.TrimSpace(resp.GetText()))
	}
	if len(out) > 1 {
		it.translated = out
		return nil
	}
	tr.finish(it, out[0])
	return nil
}

// batchItem is a cue of a batch, and of its translation.
type batchItem struct {
	Position int    `json:"position"`
	Line     string `json:"line"`
}

// batchSchema is the JSON schema of batches' translations.
const batchSchema = `{"type":"object","properties":{"translations":{"type":"array","items":{"type":"object",` +
	`"properties":{"position":{"type":"integer"},"line":{"type":"string"}},"required":["position","line"],` +
	`"additionalProperties":false}}},"required":["translations"],"additionalProperties":false}`

// translateBatch translates cues with an AI model at once, as Lingarr's
// batches: the cues go as JSON, and come back by position; a cue missing
// keeps its text.
func (tr *translation) translateBatch(ctx context.Context, c chained, items []*cueItem) error {
	batch := make([]batchItem, len(items))
	for i, it := range items {
		batch[i] = batchItem{Position: it.position, Line: it.text(tr.s.PreserveLineBreaks)}
	}
	resp, err := tr.r.complete(ctx, tr.s, tr.record, c.translator, pluginv1.CompleteAiRequest_builder{
		System:       proto.String(fillPrompt(tr.s.Prompt, tr.values(c))),
		Messages:     []*pluginv1.AiMessage{userMessage(marshalJSON(batch))},
		OutputSchema: proto.String(batchSchema),
	}.Build())
	if err != nil {
		return err
	}
	var answer struct {
		Translations []batchItem `json:"translations"`
	}
	if err := json.Unmarshal([]byte(resp.GetOutputJson()), &answer); err != nil {
		return fmt.Errorf("the model's translations: %w", err)
	}
	byPosition := map[int][]string{}
	for _, a := range answer.Translations {
		if strings.TrimSpace(a.Line) != "" {
			byPosition[a.Position] = append(byPosition[a.Position], a.Line)
		}
	}
	for _, it := range items {
		lines, ok := byPosition[it.position]
		if !ok {
			tr.logger.WarnContext(ctx, "cue missing from the translations", "position", it.position)
			it.translated = it.lines
			continue
		}
		tr.finish(it, strings.Join(lines, "\n"))
	}
	return nil
}

// translatePlugin translates cues with a translator plugin.
func (tr *translation) translatePlugin(ctx context.Context, c chained, items []*cueItem) error {
	lines := make([]string, len(items))
	for i, it := range items {
		lines[i] = it.text(tr.s.PreserveLineBreaks)
	}
	var resp *pluginv1.TranslateSubtitleLinesResponse
	err := tr.r.retry(ctx, tr.s, func(ctx context.Context) error {
		var err error
		resp, err = c.plugin.Client.TranslateSubtitleLines(ctx, pluginv1.TranslateSubtitleLinesRequest_builder{
			Translator: proto.String(c.info.GetName()), SourceLanguage: proto.String(c.source),
			TargetLanguage: proto.String(c.target), Lines: lines, Media: tr.media,
		}.Build())
		return err
	})
	if err != nil {
		return err
	}
	if n := len(resp.GetLines()); n != len(items) {
		return fmt.Errorf("%d lines translated instead of %d", n, len(items))
	}
	for i, line := range resp.GetLines() {
		tr.finish(items[i], line)
	}
	if tr.credit == "" {
		tr.credit = cmp.Or(resp.GetCredit(), c.credit)
	}
	return nil
}

func userMessage(text string) *pluginv1.AiMessage {
	return pluginv1.AiMessage_builder{Role: pluginv1.AiRole_AI_ROLE_USER.Enum(), Text: &text}.Build()
}

// complete asks an AI model, retrying as the settings say, and adds what
// it used to a record.
func (r *Runtime) complete(ctx context.Context, s *core.SubtitleTranslationSettings, rec *core.SubtitleTranslationRecord, t *translator, req *pluginv1.CompleteAiRequest) (*pluginv1.CompleteAiResponse, error) {
	req.SetProvider(t.provider)
	req.SetModel(t.model)
	var resp *pluginv1.CompleteAiResponse
	err := r.retry(ctx, s, func(ctx context.Context) error {
		var err error
		resp, err = t.ai.CompleteAi(ctx, req)
		return err
	})
	if err != nil {
		return nil, err
	}
	u := resp.GetUsage()
	rec.InputTokens += u.GetInputTokens()
	rec.OutputTokens += u.GetOutputTokens()
	rec.Cost += u.GetCost()
	return resp, nil
}

// retry runs a request to a translator, each attempt bounded by the
// request timeout, until it succeeds, fails for a reason not worth
// retrying, or has been attempted MaxRetries times, waiting RetryDelay
// multiplied by RetryMultiplier after each attempt, as Lingarr's.
// Rate limits and unavailable services are worth retrying.
func (r *Runtime) retry(ctx context.Context, s *core.SubtitleTranslationSettings, do func(context.Context) error) error {
	delay := s.RetryDelay
	for attempt := 1; ; attempt++ {
		actx, cancel := context.WithTimeout(ctx, s.RequestTimeout)
		err := do(actx)
		cancel()
		code := connect.CodeOf(err)
		if err == nil || attempt >= s.MaxRetries || ctx.Err() != nil ||
			code != connect.CodeUnavailable && code != connect.CodeResourceExhausted {
			return err
		}
		r.logger().WarnContext(ctx, "subtitle translation retried", "attempt", attempt, "of", s.MaxRetries, "delay", delay, "err", err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay *= time.Duration(s.RetryMultiplier)
	}
}

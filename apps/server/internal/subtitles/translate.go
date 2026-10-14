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

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// JobTranslate translates a subtitle of a movie or episode; the payload is
// a TranslatePayload.
const JobTranslate = "subtitles.translate"

// TranslatePayload is the payload of JobTranslate jobs: the subtitle
// translated, an external file at Path or the embedded stream at
// StreamIndex, the language translated into and the first translator,
// "<plugin>/<translator>" or "ai/<provider>/<model>", or empty for the
// settings'.
type TranslatePayload struct {
	ItemID      core.ID               `json:"item_id"`
	Path        string                `json:"path,omitempty"`
	StreamIndex *int                  `json:"stream_index,omitempty"`
	Language    core.SubtitleLanguage `json:"language"`
	Translator  string                `json:"translator,omitempty"`
}

// TranslatorState is a translator of a translator plugin.
type TranslatorState struct {
	// ID is "<plugin>/<translator>".
	ID, PluginID, Name, DisplayName string
	Enabled                         bool
}

func (r *Runtime) translators() []Translator {
	if r.Translators == nil {
		return nil
	}
	return r.Translators()
}

// ListTranslators lists the translators of the translator plugins, with
// the plugins that failed to list theirs.
func (r *Runtime) ListTranslators(ctx context.Context) ([]TranslatorState, []string) {
	var out []TranslatorState
	var failed []string
	for _, p := range r.translators() {
		resp, err := p.Client.ListSubtitleTranslators(ctx, &pluginv1.ListSubtitleTranslatorsRequest{})
		if err != nil {
			r.logger().WarnContext(ctx, "list subtitle translators", "plugin", p.ID, "err", err)
			failed = append(failed, p.ID)
			continue
		}
		for _, t := range resp.GetTranslators() {
			out = append(out, TranslatorState{
				ID: providerName(p.ID, t.GetName()), PluginID: p.ID, Name: t.GetName(),
				DisplayName: t.GetDisplayName(), Enabled: t.GetEnabled(),
			})
		}
	}
	return out, failed
}

// QueueTranslate checks a translation and queues it, returning its job's
// ID.
func (r *Runtime) QueueTranslate(ctx context.Context, p TranslatePayload) (core.ID, error) {
	s := r.settings()
	t, err := r.load(ctx, p.ItemID, &s)
	if err != nil {
		return core.ID{}, err
	}
	if err := p.Language.Validate(); err != nil {
		return core.ID{}, err
	}
	src, err := r.translationSource(ctx, t, p)
	if err != nil {
		return core.ID{}, err
	}
	if _, err := r.chain(ctx, p.Translator, &s.Translation, src.language.Language, p.Language.Language); err != nil {
		return core.ID{}, err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return core.ID{}, err
	}
	job := core.Job{ID: core.NewID(), Kind: JobTranslate, Payload: payload, MaxAttempts: 1, RunAt: r.now()}
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return core.ID{}, fmt.Errorf("queue subtitle translation: %w", err)
	}
	return job.ID, nil
}

// translationSource is a subtitle translated.
type translationSource struct {
	language core.SubtitleLanguage
	// name is the file's name, or describes the stream.
	name string
	// read returns the subtitle and its format.
	read func(ctx context.Context) ([]byte, string, error)
}

// translationSource returns the subtitle a translation translates: an
// external file of the target, or an embedded text subtitle stream from
// the extraction cache, which videos on cloud drives are not read for.
func (r *Runtime) translationSource(ctx context.Context, t *target, p TranslatePayload) (translationSource, error) {
	var src translationSource
	switch {
	case p.Path != "":
		sub, err := t.fileOf(p.Path)
		if err != nil {
			return src, err
		}
		format := strings.TrimPrefix(strings.ToLower(path.Ext(p.Path)), ".")
		if !subtitle.CanParse(format) {
			return src, fmt.Errorf("%w: %s subtitles are not translated", ErrUnavailable, format)
		}
		src = translationSource{language: sub.language, name: path.Base(p.Path), read: func(ctx context.Context) ([]byte, string, error) {
			data, err := r.Subtitles.ReadFile(ctx, t.item.ID, p.Path)
			return data, format, err
		}}
	case p.StreamIndex != nil:
		video := t.source()
		i := slices.IndexFunc(video.Streams, func(st core.MediaStream) bool { return st.Index == *p.StreamIndex })
		if i < 0 {
			return src, fmt.Errorf("%w: %s has no stream %d", core.ErrNotFound, t.item.Name, *p.StreamIndex)
		}
		st := video.Streams[i]
		if st.Kind != core.StreamSubtitle || st.ExternalPath != "" || !st.IsTextSubtitle() {
			return src, fmt.Errorf("%w: stream %d of %s is not an embedded text subtitle", core.ErrInvalid, st.Index, t.item.Name)
		}
		if r.Extracted == nil || !r.readable(ctx, video.Path) {
			return src, fmt.Errorf("%w: embedded subtitles of %s cannot be extracted", ErrUnavailable, t.item.Name)
		}
		src = translationSource{
			language: core.SubtitleLanguage{Language: Tag(st.Language), Forced: st.Forced, HearingImpaired: st.HearingImpaired && !st.Forced},
			name:     fmt.Sprintf("stream %d", st.Index),
			read: func(ctx context.Context) ([]byte, string, error) {
				return r.Extracted(ctx, t.item.ID, video, &st)
			},
		}
	default:
		return src, fmt.Errorf("%w: a translation needs a subtitle file or stream", core.ErrInvalid)
	}
	if src.language.Language == "" {
		return src, fmt.Errorf("%w: the language of %s is unknown", core.ErrInvalid, src.name)
	}
	return src, nil
}

// translateFormats are the formats translations keep with the profile's
// original format; others are saved as SubRip.
var translateFormats = []string{"srt", "ass", "ssa", "vtt"}

// Translate translates a subtitle of a movie or episode into another
// language, as Lingarr's translation: each cue's text, plain, with
// SubRip's markup or as the subtitle holds it, as the translators take it,
// by the first of the translators that does not fail; timing and styles
// are kept. The translation keeps the subtitle's format when the profile
// keeps original formats and the cues need not become SubRip's, else it is
// SubRip, with a first cue crediting the translator when the settings say
// so. It is saved as subtitles are and recorded with the settings' score
// and what it translated.
func (r *Runtime) Translate(ctx context.Context, p TranslatePayload) (core.SubtitleHistoryEvent, error) {
	s := r.settings()
	ts := &s.Translation
	t, err := r.load(ctx, p.ItemID, &s)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	src, err := r.translationSource(ctx, t, p)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	candidates, err := r.chain(ctx, p.Translator, ts, src.language.Language, p.Language.Language)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	data, format, err := src.read(ctx)
	if err != nil {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("read %s: %w", src.name, err)
	}
	text, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s: %w", core.ErrInvalid, src.name, err)
	}
	sub, err := subtitle.Parse(text, format)
	if err != nil {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s: %w", core.ErrInvalid, src.name, err)
	}
	if ts.Validation.Enabled {
		if err := validateSubtitle(len(data), sub.Cues, &ts.Validation); err != nil {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("%s: %w", src.name, err)
		}
	}

	// The cues go as the subtitle holds them to AI models, with SubRip's
	// markup when a translator keeps it, else as plain text.
	out := format
	if ts.StripFormatting || t.profile == nil || !t.profile.OriginalFormat || !slices.Contains(translateFormats, format) {
		out = subtitle.SRT
	}
	mode := textPlain
	if !ts.StripFormatting {
		allAI := !slices.ContainsFunc(candidates, func(c chained) bool { return !c.isAI() })
		markup := !slices.ContainsFunc(candidates, func(c chained) bool { return !c.isAI() && !c.info.GetMarkup() })
		switch {
		case allAI && out == format:
			mode = textAsIs
		case markup:
			mode, out = textSubRip, subtitle.SRT
		}
	}
	if mode == textSubRip && format != subtitle.SRT {
		if text, err = subtitle.Write(sub, subtitle.SRT); err == nil {
			sub, err = subtitle.Parse(text, subtitle.SRT)
		}
		if err != nil {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s: %w", core.ErrInvalid, src.name, err)
		}
	}
	stem := strings.TrimSuffix(path.Base(t.source().Path), path.Ext(t.source().Path))
	tr := &translation{
		r: r, s: ts, source: src.language.Language, target: p.Language.Language, candidates: candidates,
		media:  r.translationMedia(t, src.name, fileName(stem, p.Language, out, &s)),
		logger: r.logger().With("item", t.item.ID, "source", src.name), record: &core.SubtitleTranslationRecord{SourceLanguage: src.language.Language},
	}
	for i, c := range sub.Cues {
		// Cues without spoken text, such as SSA drawings, keep theirs.
		spoken := subtitle.SpokenText(c.Text)
		content := c.Text
		if mode == textPlain {
			content = spoken
		}
		if strings.TrimSpace(spoken) != "" {
			tr.items = append(tr.items, &cueItem{cue: i, position: len(tr.items) + 1, start: c.Start, end: c.End, lines: strings.Split(content, "\n")})
		}
	}
	if err := tr.run(ctx); err != nil {
		return core.SubtitleHistoryEvent{}, fmt.Errorf("translate %s: %w", src.name, err)
	}
	for _, it := range tr.items {
		if it.translated != nil {
			sub.Cues[it.cue].Text = strings.Join(it.translated, "\n")
		}
	}
	if ts.FixOverlaps {
		fixOverlaps(sub.Cues)
	}
	if ts.Credit && out == subtitle.SRT {
		sub.Cues = creditCue(sub.Cues, tr.credit)
	}
	result, err := subtitle.Write(sub, out)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	saved, err := r.save(ctx, t, p.Language, out, result, &s)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	if p.Path != "" {
		tr.record.SourcePath = p.Path
	} else {
		tr.record.SourceStreamIndex = *p.StreamIndex
	}
	maxScore := subtitles.MaxScore(t.episode())
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: core.SubtitleTranslated, ItemID: t.item.ID, Date: r.now(), Language: p.Language,
		Score: ts.DefaultScore * maxScore / 100, MaxScore: maxScore, Path: saved, VideoPath: t.source().Path,
		Description: fmt.Sprintf("%s subtitles translated to %s with %s.", src.language, p.Language, tr.credit),
		Translation: tr.record,
	}
	if t.series != nil {
		e.ParentID = t.series.ID
	}
	if err := r.record(ctx, t, &e); err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	return e, nil
}

// translationMedia describes what the target is to translators.
func (r *Runtime) translationMedia(t *target, source, name string) *pluginv1.SubtitleTranslationMedia {
	kind := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if t.episode() {
		kind = pluginv1.MediaKind_MEDIA_KIND_EPISODE
	}
	b := pluginv1.SubtitleTranslationMedia_builder{
		Kind: kind.Enum(), Title: proto.String(t.owner.Name), Year: proto.Int32(int32(t.owner.ProductionYear)),
		ExternalIds: ids(t.owner.ExternalIDs), Overview: proto.String(t.owner.Overview),
		SourceFileName: proto.String(source), FileName: proto.String(name),
	}
	if t.episode() {
		b.Season, b.Episode = proto.Int32(int32(t.video.Season)), proto.Int32(int32(t.video.EpisodeNumber))
		b.EpisodeTitle = proto.String(t.item.Name)
	}
	return b.Build()
}

// creditCue returns cues with a first one crediting the translator, as
// Bazarr's add_translator_info: until the first cue, or for at most five
// seconds from the first.
func creditCue(cues []subtitle.Cue, credit string) []subtitle.Cue {
	end := 5 * time.Second
	if len(cues) > 0 {
		end = min(cues[0].Start, end)
	}
	start := time.Duration(0)
	if end == 5*time.Second {
		start = time.Second
	}
	out := append([]subtitle.Cue{{Start: start, End: end, Text: creditPrefix + credit + " # "}}, cues...)
	for i := range out {
		out[i].Index = i + 1
	}
	return out
}

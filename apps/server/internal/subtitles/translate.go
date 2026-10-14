package subtitles

import (
	"cmp"
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
// StreamIndex, the language translated into and the translator,
// "<plugin>/<translator>" or empty for the settings'.
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

// translator returns the translator a translation names, else the
// settings', else the first enabled one, with its description.
func (r *Runtime) translator(ctx context.Context, name string, s *core.SubtitleSettings) (Translator, *pluginv1.SubtitleTranslatorInfo, error) {
	name = cmp.Or(name, s.Translation.Translator)
	for _, p := range r.translators() {
		resp, err := p.Client.ListSubtitleTranslators(ctx, &pluginv1.ListSubtitleTranslatorsRequest{})
		if err != nil {
			if strings.HasPrefix(name, p.ID+"/") {
				return Translator{}, nil, fmt.Errorf("%w: subtitle translator plugin %s: %w", ErrUnavailable, p.ID, err)
			}
			continue
		}
		for _, t := range resp.GetTranslators() {
			if name == "" && t.GetEnabled() || name == providerName(p.ID, t.GetName()) {
				if !t.GetEnabled() {
					return Translator{}, nil, fmt.Errorf("%w: subtitle translator %s is not enabled", ErrUnavailable, name)
				}
				return p, t, nil
			}
		}
	}
	if name == "" {
		return Translator{}, nil, fmt.Errorf("%w: no subtitle translator is enabled", ErrUnavailable)
	}
	return Translator{}, nil, fmt.Errorf("subtitle translator %s: %w", name, core.ErrNotFound)
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
	if _, err := r.translationSource(ctx, t, p); err != nil {
		return core.ID{}, err
	}
	if _, _, err := r.translator(ctx, p.Translator, &s); err != nil {
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
// language, as Bazarr's translation: the cues' text, plain or with
// SubRip's markup as the translator takes it, in its batches; timing and
// styles are kept. The translation keeps the subtitle's format when the
// profile keeps original formats and the translator takes plain text,
// else it is SubRip, with a first cue crediting the translator when the
// settings say so. It is saved as subtitles are and recorded with the
// settings' score.
func (r *Runtime) Translate(ctx context.Context, p TranslatePayload) (core.SubtitleHistoryEvent, error) {
	s := r.settings()
	t, err := r.load(ctx, p.ItemID, &s)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	src, err := r.translationSource(ctx, t, p)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	tr, info, err := r.translator(ctx, p.Translator, &s)
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
	markup := info.GetMarkup()
	out := format
	if markup || t.profile == nil || !t.profile.OriginalFormat || !slices.Contains(translateFormats, format) {
		out = subtitle.SRT
	}
	if markup && format != subtitle.SRT {
		// The cues take SubRip's markup.
		if text, err = subtitle.Write(sub, subtitle.SRT); err == nil {
			sub, err = subtitle.Parse(text, subtitle.SRT)
		}
		if err != nil {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("%w: %s: %w", core.ErrInvalid, src.name, err)
		}
	}
	// Cues without text have nothing to translate.
	var cues []int
	var lines []string
	for i, c := range sub.Cues {
		line := c.Text
		if !markup {
			line = subtitle.PlainText(line)
		}
		if strings.TrimSpace(line) != "" {
			cues, lines = append(cues, i), append(lines, line)
		}
	}
	stem := strings.TrimSuffix(path.Base(t.source().Path), path.Ext(t.source().Path))
	media := r.translationMedia(t, src.name, fileName(stem, p.Language, out, &s))
	batch := int(info.GetBatchSize())
	if batch <= 0 {
		batch = max(len(lines), 1)
	}
	credit := info.GetDisplayName()
	for start := 0; start < len(lines); start += batch {
		end := min(start+batch, len(lines))
		resp, err := tr.Client.TranslateSubtitleLines(ctx, pluginv1.TranslateSubtitleLinesRequest_builder{
			Translator: proto.String(info.GetName()), SourceLanguage: proto.String(src.language.Language),
			TargetLanguage: proto.String(p.Language.Language), Lines: lines[start:end], Media: media,
		}.Build())
		if err != nil {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("translate %s with %s: %w", src.name, providerName(tr.ID, info.GetName()), err)
		}
		if n := len(resp.GetLines()); n != end-start {
			return core.SubtitleHistoryEvent{}, fmt.Errorf("translate %s with %s: %d lines translated instead of %d",
				src.name, providerName(tr.ID, info.GetName()), n, end-start)
		}
		for i, line := range resp.GetLines() {
			if line != "" {
				sub.Cues[cues[start+i]].Text = line
			}
		}
		credit = cmp.Or(resp.GetCredit(), credit)
		r.logger().DebugContext(ctx, "translating subtitle", "item", t.item.ID, "source", src.name, "done", end, "of", len(lines))
	}
	if s.Translation.Credit && out == subtitle.SRT {
		sub.Cues = creditCue(sub.Cues, credit)
	}
	result, err := subtitle.Write(sub, out)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	saved, err := r.save(ctx, t, p.Language, out, result, &s)
	if err != nil {
		return core.SubtitleHistoryEvent{}, err
	}
	maxScore := subtitles.MaxScore(t.episode())
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: core.SubtitleTranslated, ItemID: t.item.ID, Date: r.now(), Language: p.Language,
		Score: s.Translation.DefaultScore * maxScore / 100, MaxScore: maxScore, Path: saved, VideoPath: t.source().Path,
		Description: fmt.Sprintf("%s subtitles translated to %s with %s.", src.language, p.Language, credit),
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
	out := append([]subtitle.Cue{{Start: start, End: end, Text: "# Subtitles translated with " + credit + " # "}}, cues...)
	for i := range out {
		out[i].Index = i + 1
	}
	return out
}

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/deepl"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/gtranslate"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/libre"
)

// translatorDefinitions are the translators the plugin implements, the
// translation services of Lingarr.
func translatorDefinitions() []translator.Definition {
	return append([]translator.Definition{deepl.Definition(), libre.Definition()}, gtranslate.Definitions()...)
}

// ListSubtitleTranslators implements SubtitleTranslatorService.
// The languages of enabled translators that list theirs are listed too;
// none when listing them fails.
func (p *plugin) ListSubtitleTranslators(ctx context.Context, _ *pluginv1.ListSubtitleTranslatorsRequest) (*pluginv1.ListSubtitleTranslatorsResponse, error) {
	p.mu.RLock()
	defs := p.translatorDefs
	cts := make([]*configuredTranslator, len(defs))
	for i := range defs {
		cts[i] = p.translators[defs[i].Name]
	}
	p.mu.RUnlock()
	out := make([]*pluginv1.SubtitleTranslatorInfo, 0, len(defs))
	for i := range defs {
		def, ct := &defs[i], cts[i]
		batch := 0
		var source, target []string
		if ct != nil && ct.translator != nil {
			batch = ct.translator.BatchSize()
			if l, ok := ct.translator.(translator.Languages); ok {
				var err error
				if source, target, err = l.Languages(ctx); err != nil {
					p.log.WarnContext(ctx, "list translator languages", "translator", def.Name, "err", err)
				}
			}
		}
		out = append(out, pluginv1.SubtitleTranslatorInfo_builder{
			Name: proto.String(def.Name), DisplayName: proto.String(def.DisplayName), Settings: settingFields(def.Settings),
			Enabled: proto.Bool(ct != nil), BatchSize: proto.Int32(int32(batch)), Markup: proto.Bool(def.Markup),
			SourceLanguages: source, TargetLanguages: target,
		}.Build())
	}
	return pluginv1.ListSubtitleTranslatorsResponse_builder{Translators: out}.Build(), nil
}

// TranslateSubtitleLines implements SubtitleTranslatorService.
func (p *plugin) TranslateSubtitleLines(ctx context.Context, req *pluginv1.TranslateSubtitleLinesRequest) (*pluginv1.TranslateSubtitleLinesResponse, error) {
	p.mu.RLock()
	ct := p.translators[req.GetTranslator()]
	p.mu.RUnlock()
	if ct == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s: translator not enabled", req.GetTranslator()))
	}
	if ct.err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, ct.err)
	}
	m := req.GetMedia()
	tr := &translator.Request{
		Source: req.GetSourceLanguage(), Target: req.GetTargetLanguage(), Lines: req.GetLines(),
		Media: translator.Media{
			Episode: m.GetKind() == pluginv1.MediaKind_MEDIA_KIND_EPISODE, Title: m.GetTitle(), Year: int(m.GetYear()),
			ExternalIDs: m.GetExternalIds(), Overview: m.GetOverview(), Season: int(m.GetSeason()), Number: int(m.GetEpisode()),
			EpisodeTitle: m.GetEpisodeTitle(), SourceFileName: m.GetSourceFileName(), FileName: m.GetFileName(),
		},
	}
	lines, err := ct.translator.Translate(ctx, tr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, connect.NewError(connect.CodeCanceled, ctx.Err())
		}
		p.log.WarnContext(ctx, "translate subtitle", "translator", ct.def.Name, "err", err)
		return nil, connect.NewError(translateCode(err), err)
	}
	if len(lines) != len(req.GetLines()) {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("%s translated %d lines of %d", ct.def.Name, len(lines), len(req.GetLines())))
	}
	return pluginv1.TranslateSubtitleLinesResponse_builder{Lines: lines, Credit: proto.String(ct.translator.Credit())}.Build(), nil
}

// translateCode is the code of a translator's failure.
func translateCode(err error) connect.Code {
	switch provider.KindOf(err) {
	case provider.TooManyRequests, provider.APIThrottled:
		return connect.CodeResourceExhausted
	case provider.AuthenticationError, provider.PermissionError, provider.ConfigurationError, provider.SearchLimitReached:
		return connect.CodeFailedPrecondition
	case provider.Timeout:
		return connect.CodeDeadlineExceeded
	}
	return connect.CodeUnavailable
}

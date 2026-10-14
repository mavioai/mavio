package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/gemini"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/google"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translators/lingarr"
)

// translatorDefinitions are the translators the plugin implements, as
// Bazarr's.
func translatorDefinitions() []translator.Definition {
	return []translator.Definition{google.Definition(), gemini.Definition(), lingarr.Definition()}
}

// ListSubtitleTranslators implements SubtitleTranslatorService.
func (p *plugin) ListSubtitleTranslators(context.Context, *pluginv1.ListSubtitleTranslatorsRequest) (*pluginv1.ListSubtitleTranslatorsResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*pluginv1.SubtitleTranslatorInfo, 0, len(p.translatorDefs))
	for i := range p.translatorDefs {
		def := &p.translatorDefs[i]
		ct := p.translators[def.Name]
		batch := 0
		if ct != nil && ct.translator != nil {
			batch = ct.translator.BatchSize()
		}
		out = append(out, pluginv1.SubtitleTranslatorInfo_builder{
			Name: proto.String(def.Name), DisplayName: proto.String(def.DisplayName), Settings: settingFields(def.Settings),
			Enabled: proto.Bool(ct != nil), BatchSize: proto.Int32(int32(batch)), Markup: proto.Bool(def.Markup),
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
	case provider.AuthenticationError, provider.PermissionError, provider.ConfigurationError:
		return connect.CodeFailedPrecondition
	case provider.Timeout:
		return connect.CodeDeadlineExceeded
	}
	return connect.CodeUnavailable
}

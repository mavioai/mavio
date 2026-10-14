package main

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// ListSubtitleProviders implements SubtitleProviderService.
func (p *plugin) ListSubtitleProviders(context.Context, *pluginv1.ListSubtitleProvidersRequest) (*pluginv1.ListSubtitleProvidersResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*pluginv1.SubtitleProviderInfo, 0, len(p.defs))
	for i := range p.defs {
		def := &p.defs[i]
		info := pluginv1.SubtitleProviderInfo_builder{
			Name: proto.String(def.Name), DisplayName: proto.String(def.DisplayName), Settings: settingFields(def.Settings),
			Enabled: proto.Bool(p.enabled[def.Name] != nil), Languages: def.Languages,
			HashVerifiable: proto.Bool(def.HashVerifiable), HearingImpairedVerifiable: proto.Bool(def.HearingImpairedVerifiable),
			Movies: proto.Bool(def.Movies), Episodes: proto.Bool(def.Episodes),
		}.Build()
		if st, ok := p.throttle.Throttled(def.Name); ok {
			info.SetThrottledUntil(timestamppb.New(st.Until))
			info.SetThrottleReason(st.Reason)
		}
		out = append(out, info)
	}
	return pluginv1.ListSubtitleProvidersResponse_builder{Providers: out}.Build(), nil
}

// SearchSubtitles implements SubtitleProviderService: the providers asked
// for, or every enabled one, are searched at once, leaving out those that
// are throttled, take none of the languages or not the video's kind.
func (p *plugin) SearchSubtitles(ctx context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	v := video(req.GetVideo())
	langs := make([]provider.Language, 0, len(req.GetLanguages()))
	for _, l := range req.GetLanguages() {
		langs = append(langs, provider.Language{Tag: l.GetLanguage(), Forced: l.GetForced(), HearingImpaired: l.GetHearingImpaired()})
	}
	var names []string
	p.mu.RLock()
	for name := range p.enabled {
		if len(req.GetProviders()) == 0 || slices.Contains(req.GetProviders(), name) {
			names = append(names, name)
		}
	}
	p.mu.RUnlock()
	slices.Sort(names)

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		subs     []*pluginv1.RemoteSubtitle
		failures []*pluginv1.SubtitleProviderFailure
	)
	for _, name := range names {
		cp, err := p.lookup(name)
		if err != nil {
			continue
		}
		if v.Episode && !cp.def.Episodes || !v.Episode && !cp.def.Movies {
			continue
		}
		want := supported(cp.def, langs)
		if len(want) == 0 {
			continue
		}
		if _, ok := p.throttle.Throttled(name); ok {
			continue
		}
		wg.Go(func() {
			found, err := p.search(ctx, cp, v, want)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if f := p.fail(ctx, cp.def, err); f != nil {
					failures = append(failures, f)
				}
				return
			}
			for _, s := range found {
				subs = append(subs, remote(cp.def, s))
			}
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, connect.NewError(connect.CodeCanceled, err)
	}
	return pluginv1.SearchSubtitlesResponse_builder{Subtitles: subs, Failures: failures}.Build(), nil
}

func (p *plugin) search(ctx context.Context, cp *configured, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if cp.err != nil {
		return nil, cp.err
	}
	start := p.clock()
	found, err := cp.provider.Search(ctx, v, langs)
	p.log.DebugContext(ctx, "search subtitles", "provider", cp.def.Name, "found", len(found), "took", p.clock().Sub(start), "err", err)
	return found, err
}

// supported returns the languages a provider has subtitles in.
func supported(def *provider.Definition, langs []provider.Language) []provider.Language {
	if len(def.Languages) == 0 {
		return langs
	}
	var out []provider.Language
	for _, l := range langs {
		if slices.Contains(def.Languages, l.Tag) {
			out = append(out, l)
		}
	}
	return out
}

// DownloadSubtitle implements SubtitleProviderService.
func (p *plugin) DownloadSubtitle(ctx context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error) {
	cp, err := p.lookup(req.GetProvider())
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if st, ok := p.throttle.Throttled(cp.def.Name); ok {
		return pluginv1.DownloadSubtitleResponse_builder{Failure: pluginv1.SubtitleProviderFailure_builder{
			Provider: proto.String(cp.def.Name), Kind: failureKind(reasonKind(st.Reason)).Enum(),
			Message: proto.String("throttled: " + st.Reason), ThrottledUntil: timestamppb.New(st.Until),
		}.Build()}.Build(), nil
	}
	if cp.err != nil {
		return pluginv1.DownloadSubtitleResponse_builder{Failure: p.fail(ctx, cp.def, cp.err)}.Build(), nil
	}
	f, err := cp.provider.Download(ctx, video(req.GetVideo()), req.GetId())
	if err != nil {
		if ctx.Err() != nil {
			return nil, connect.NewError(connect.CodeCanceled, ctx.Err())
		}
		return pluginv1.DownloadSubtitleResponse_builder{Failure: p.fail(ctx, cp.def, err)}.Build(), nil
	}
	return pluginv1.DownloadSubtitleResponse_builder{
		Data: f.Data, Format: proto.String(f.Format), Language: language(f.Language),
	}.Build(), nil
}

// fail records a provider's failure and describes it; nil when the
// request was canceled. A bad subtitle is to be blocklisted rather than
// throttled for.
func (p *plugin) fail(ctx context.Context, def *provider.Definition, err error) *pluginv1.SubtitleProviderFailure {
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return nil
	}
	kind := provider.KindOf(err)
	f := pluginv1.SubtitleProviderFailure_builder{
		Provider: proto.String(def.Name), Kind: failureKind(kind).Enum(), Message: proto.String(err.Error()),
	}.Build()
	if kind == provider.BadSubtitle {
		p.log.WarnContext(ctx, "bad subtitle", "provider", def.Name, "err", err)
		return f
	}
	now := p.clock()
	d := provider.Throttle(def, kind, now)
	if p.throttle.Fail(def.Name, kind.String(), d, provider.Counted(kind)) {
		f.SetThrottledUntil(timestamppb.New(now.Add(d)))
		p.log.WarnContext(ctx, "throttle subtitle provider", "provider", def.Name, "reason", kind, "for", d.Round(time.Second), "err", err)
	} else {
		p.log.WarnContext(ctx, "subtitle provider failed", "provider", def.Name, "err", err)
	}
	return f
}

// reasonKind returns the kind of a throttle's reason.
func reasonKind(reason string) provider.Kind {
	for k := provider.Other; k <= provider.BadSubtitle; k++ {
		if k.String() == reason {
			return k
		}
	}
	return provider.Other
}

// failureKind returns the kind of failure the server is told.
func failureKind(k provider.Kind) pluginv1.SubtitleFailureKind {
	switch k {
	case provider.TooManyRequests, provider.APIThrottled:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_TOO_MANY_REQUESTS
	case provider.DownloadLimitExceeded, provider.SearchLimitReached:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_DOWNLOAD_LIMIT
	case provider.ServiceUnavailable:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_SERVICE_UNAVAILABLE
	case provider.AuthenticationError, provider.PermissionError:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_AUTHENTICATION
	case provider.ConfigurationError:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_CONFIGURATION
	case provider.BadSubtitle:
		return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_BAD_SUBTITLE
	}
	return pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_OTHER
}

func language(l provider.Language) *pluginv1.SubtitleLanguage {
	return pluginv1.SubtitleLanguage_builder{
		Language: proto.String(l.Tag), Forced: proto.Bool(l.Forced), HearingImpaired: proto.Bool(l.HearingImpaired),
	}.Build()
}

// remote describes a subtitle a provider found.
func remote(def *provider.Definition, s provider.Subtitle) *pluginv1.RemoteSubtitle {
	hash, hi := def.HashVerifiable, def.HearingImpairedVerifiable
	if s.HashVerifiable != nil {
		hash = *s.HashVerifiable
	}
	if s.HearingImpairedVerifiable != nil {
		hi = *s.HearingImpairedVerifiable
	}
	return pluginv1.RemoteSubtitle_builder{
		Provider: proto.String(def.Name), Id: proto.String(s.ID), Language: language(s.Language),
		ReleaseNames: s.ReleaseNames, Matches: s.Matches, HashVerifiable: proto.Bool(hash), HearingImpairedVerifiable: proto.Bool(hi),
		Format: proto.String(s.Format), PageLink: proto.String(s.PageLink), Uploader: proto.String(s.Uploader),
		Comment: proto.String(s.Comment), DownloadCount: proto.Int32(int32(min(s.Downloads, math.MaxInt32))),
	}.Build()
}

// video returns the video a request describes.
func video(v *pluginv1.SubtitleVideo) *provider.Video {
	return &provider.Video{
		Episode: v.GetKind() == pluginv1.MediaKind_MEDIA_KIND_EPISODE,
		Title:   v.GetTitle(), AlternativeTitles: v.GetAlternativeTitles(), Year: int(v.GetYear()), ExternalIDs: v.GetExternalIds(),
		Series: v.GetSeries(), AlternativeSeries: v.GetAlternativeSeries(), SeriesExternalIDs: v.GetSeriesExternalIds(),
		Season: int(v.GetSeason()), Number: int(v.GetEpisode()), AbsoluteNumber: int(v.GetAbsoluteEpisode()),
		OriginalSeries: v.GetOriginalSeries(), Special: v.GetSpecial(),
		ReleaseName: v.GetReleaseName(), Source: v.GetSource(), ReleaseGroup: v.GetReleaseGroup(), Resolution: v.GetResolution(),
		VideoCodec: v.GetVideoCodec(), AudioCodec: v.GetAudioCodec(), Edition: v.GetEdition(), StreamingService: v.GetStreamingService(),
		FrameRate: v.GetFrameRate(), FileName: v.GetFileName(), FileSize: v.GetFileSize(), FileHash: v.GetFileHash(),
		AudioLanguages: v.GetAudioLanguages(), OriginalLanguage: v.GetOriginalLanguage(),
		AudioURL: v.GetAudioUrl(), AudioStreams: audioStreams(v.GetAudioStreams()),
	}
}

func audioStreams(streams []*pluginv1.SubtitleAudioStream) []provider.AudioStream {
	var out []provider.AudioStream
	for _, s := range streams {
		out = append(out, provider.AudioStream{Index: int(s.GetIndex()), Language: s.GetLanguage()})
	}
	return out
}

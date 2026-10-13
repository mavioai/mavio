package subtitles

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/acquire/subtitles"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/subtitle"
)

// found is a subtitle a provider plugin offers.
type found struct {
	subtitles.Candidate
	plugin Provider
	// provider is the plugin's provider, and remote the subtitle as the
	// plugin describes it.
	provider string
	remote   *pluginv1.RemoteSubtitle
}

// providerName names a plugin's provider as history records it.
func providerName(pluginID, provider string) string { return pluginID + "/" + provider }

// pluginVideo returns the video as the provider plugins take it, hashing
// the file first when its hash is not known.
func (r *Runtime) pluginVideo(ctx context.Context, t *target, s *core.SubtitleSettings) *pluginv1.SubtitleVideo {
	src, v := t.source(), t.video
	if src.FileHash == "" && !s.SkipHashing {
		if hash, err := r.hash(ctx, t.lib, src.Path); err != nil {
			r.logger().DebugContext(ctx, "no hash for subtitle search", "path", src.Path, "err", err)
		} else {
			src.FileHash = hash
			if err := r.Store.MediaSources().Replace(ctx, t.item.ID, t.sources); err != nil {
				r.logger().WarnContext(ctx, "keep video hash", "item", t.item.ID, "err", err)
			}
		}
	}
	kind := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if v.Episode {
		kind = pluginv1.MediaKind_MEDIA_KIND_EPISODE
	}
	b := pluginv1.SubtitleVideo_builder{
		Kind: kind.Enum(), Title: proto.String(v.Title), AlternativeTitles: v.AlternativeTitles, Year: proto.Int32(int32(v.Year)),
		ExternalIds: ids(t.item.ExternalIDs), Special: proto.Bool(v.Special),
		ReleaseName: proto.String(t.scene), Source: proto.String(v.Source), ReleaseGroup: proto.String(v.ReleaseGroup),
		Resolution: proto.String(v.Resolution), VideoCodec: proto.String(v.VideoCodec), AudioCodec: proto.String(v.AudioCodec),
		Edition: proto.String(v.Edition), StreamingService: proto.String(v.StreamingService),
		FileName: proto.String(path.Base(src.Path)), FileSize: proto.Int64(src.Size), FileHash: proto.String(src.FileHash),
		AudioLanguages: t.audio, OriginalLanguage: proto.String(Tag(t.owner.OriginalLanguage)),
	}
	if v.Episode {
		b.Series, b.AlternativeSeries, b.OriginalSeries = proto.String(v.Series), v.AlternativeSeries, proto.Bool(v.OriginalSeries)
		b.Season, b.Episode = proto.Int32(int32(v.Season)), proto.Int32(int32(v.EpisodeNumber))
		if t.item.AbsoluteNumber != nil {
			b.AbsoluteEpisode = proto.Int32(int32(*t.item.AbsoluteNumber))
		}
		if t.series != nil {
			b.SeriesExternalIds = ids(t.series.ExternalIDs)
		}
	}
	for _, st := range src.Streams {
		if st.Kind == core.StreamVideo {
			if fps := st.FrameRate.Float(); fps > 0 {
				b.FrameRate = proto.Float64(fps)
			}
			break
		}
	}
	if !s.SkipHashing && r.readable(ctx, src.Path) {
		if hash, err := r.napiprojektHash(ctx, t.lib, src); err == nil {
			b.ExternalIds[napiprojektID] = hash
		}
	}
	return b.Build()
}

// napiprojektID is the external ID NapiProjekt finds subtitles by.
const napiprojektID = "napiprojekt"

func ids(m map[core.Provider]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if v != "" {
			out[string(k)] = v
		}
	}
	return out
}

// hash returns the OpenSubtitles hash of a video of a library.
func (r *Runtime) hash(ctx context.Context, lib core.Library, p string) (string, error) {
	root, name, ok := library.RootOf(lib.Paths, p)
	if !ok {
		return "", fmt.Errorf("%s is outside the library's folders", p)
	}
	f, err := r.Storages.OpenFile(ctx, root, name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	return subtitle.FileHash(f, info.Size())
}

// search asks the provider plugins for subtitles of a video in langs, and
// returns them with the providers that failed.
func (r *Runtime) search(ctx context.Context, t *target, video *pluginv1.SubtitleVideo, langs []core.SubtitleLanguage, mode func(core.SubtitleLanguage) subtitles.HIMode) ([]found, []string) {
	var wanted []*pluginv1.SubtitleLanguage
	for _, l := range langs {
		wanted = append(wanted, pluginv1.SubtitleLanguage_builder{
			Language: proto.String(l.Language), Forced: proto.Bool(l.Forced),
			HearingImpaired: proto.Bool(l.HearingImpaired || mode(l) == subtitles.HIForce),
		}.Build())
	}
	var out []found
	var failed []string
	for _, p := range r.providers() {
		resp, err := p.Client.SearchSubtitles(ctx, pluginv1.SearchSubtitlesRequest_builder{Video: video, Languages: wanted}.Build())
		if err != nil {
			r.logger().WarnContext(ctx, "subtitle search failed", "plugin", p.ID, "item", t.item.ID, "err", err)
			failed = append(failed, p.ID)
			continue
		}
		for _, f := range resp.GetFailures() {
			failed = append(failed, providerName(p.ID, f.GetProvider()))
		}
		for _, s := range resp.GetSubtitles() {
			l := s.GetLanguage()
			lang := Tag(l.GetLanguage())
			if lang == "" {
				continue
			}
			names := ReleaseNames(s.GetReleaseNames())
			out = append(out, found{
				Candidate: subtitles.Candidate{
					Provider: providerName(p.ID, s.GetProvider()), ID: s.GetId(),
					Language:    core.SubtitleLanguage{Language: lang, Forced: l.GetForced(), HearingImpaired: l.GetHearingImpaired() && !l.GetForced()},
					ReleaseInfo: strings.Join(names, "\n"), HashVerifiable: s.GetHashVerifiable(),
					HearingImpairedVerifiable: s.GetHearingImpairedVerifiable(),
					Matches:                   subtitles.CandidateMatches(t.video, subtitles.NewMatches(s.GetMatches()...), names, false),
				},
				plugin: p, provider: s.GetProvider(), remote: s,
			})
		}
	}
	return out, failed
}

// ReleaseNames returns the release names a provider gave a subtitle, as
// Bazarr's manual search lists them: names separated by commas are
// several.
func ReleaseNames(names []string) []string {
	var out []string
	for _, n := range names {
		for part := range strings.SplitSeq(n, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// errBadSubtitle marks subtitles that cannot be used, which are
// blocklisted.
var errBadSubtitle = errors.New("unusable subtitle")

// fetch downloads a subtitle and prepares it for saving: decoded to
// UTF-8, converted to SubRip unless the profile keeps the original format,
// and modified by the settings' mods. Subtitles that cannot be used fail
// with errBadSubtitle.
func (r *Runtime) fetch(ctx context.Context, t *target, f *found, video *pluginv1.SubtitleVideo, s *core.SubtitleSettings) ([]byte, string, error) {
	resp, err := f.plugin.Client.DownloadSubtitle(ctx, pluginv1.DownloadSubtitleRequest_builder{
		Provider: proto.String(f.provider), Id: proto.String(f.ID), Video: video,
	}.Build())
	if err != nil {
		return nil, "", fmt.Errorf("download subtitle from %s: %w", f.Provider, err)
	}
	if fail := resp.GetFailure(); fail != nil {
		err := fmt.Errorf("download subtitle from %s: %s", f.Provider, fail.GetMessage())
		if fail.GetKind() == pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_BAD_SUBTITLE {
			err = fmt.Errorf("%w: %w", errBadSubtitle, err)
		}
		return nil, "", err
	}
	original := t.profile != nil && t.profile.OriginalFormat
	return r.prepare(resp.GetData(), cmp.Or(resp.GetFormat(), f.remote.GetFormat()), f.Language.Language, original, s)
}

// applyMods applies Subzero's mods to a subtitle in a language and
// writes it in its format.
func applyMods(sub *subtitle.Subtitle, format, lang string, mods []string) ([]byte, error) {
	if _, err := sub.ApplyMods(lang, mods); err != nil {
		if errors.Is(err, subtitle.ErrInvalidMod) {
			err = fmt.Errorf("%w: %w", core.ErrInvalid, err)
		}
		return nil, fmt.Errorf("modify subtitle: %w", err)
	}
	return subtitle.Write(sub, format)
}

// saveFormats are the subtitle formats saved.
var saveFormats = []string{"srt", "ass", "ssa", "vtt", "sub"}

// prepare readies a subtitle file for saving; see fetch.
func (r *Runtime) prepare(data []byte, format, lang string, original bool, s *core.SubtitleSettings) ([]byte, string, error) {
	format = strings.ToLower(cmp.Or(format, "srt"))
	if !slices.Contains(saveFormats, format) {
		return nil, "", fmt.Errorf("%w: format %q", errBadSubtitle, format)
	}
	if format == "sub" {
		return data, format, nil
	}
	text, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errBadSubtitle, err)
	}
	parsed, err := subtitle.Parse(text, format)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", errBadSubtitle, err)
	}
	changed := false
	if !original && format != "srt" {
		if text, err = subtitle.Write(parsed, "srt"); err != nil {
			return nil, "", fmt.Errorf("%w: %w", errBadSubtitle, err)
		}
		format, changed = "srt", true
	}
	if len(s.Mods) > 0 {
		if text, err = applyMods(parsed, format, lang, s.Mods); err != nil {
			return nil, "", err
		}
		changed = true
	}
	if !s.UTF8 && !changed {
		// Kept as the provider encoded it.
		return data, format, nil
	}
	return text, format, nil
}

// fileName returns the name of a subtitle of the video named stem,
// without its extension, as Bazarr names them:
// "<video>.<language>[.forced|.<hi>].<format>", or "<video>.<format>"
// when the settings save a single language.
func fileName(stem string, l core.SubtitleLanguage, format string, s *core.SubtitleSettings) string {
	name := stem
	if !s.SingleLanguage {
		name += "." + l.Language
		switch {
		case l.Forced:
			name += ".forced"
		case l.HearingImpaired:
			name += "." + s.HIExtension
		}
	}
	return name + "." + format
}

// save writes a subtitle of the target's video, beside it or in the
// subtitle folder, replacing one of the same name, and returns its path.
func (r *Runtime) save(ctx context.Context, t *target, l core.SubtitleLanguage, format string, data []byte, s *core.SubtitleSettings) (string, error) {
	st, err := r.Subtitles.Save(ctx, t.lib, t.item.ID, t.source().ID, func(_ fs.FS, stem string) (string, error) {
		return fileName(stem, l, format, s), nil
	}, data)
	if err != nil {
		return "", err
	}
	return st.ExternalPath, nil
}

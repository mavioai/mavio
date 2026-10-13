package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyIMDb = "imdb"
	keyTMDB = "tmdb"
)

// languages maps the host's language codes to OpenSubtitles' where they
// differ.
var languages = map[string]string{"zh-Hans": "zh-cn", "zh-Hant": "zh-tw", "zh": "zh-cn", "pt-BR": "pt-br", "pt": "pt-pt"}

// hostLanguage maps OpenSubtitles' language codes back.
func hostLanguage(s string) string {
	switch strings.ToLower(s) {
	case "zh-cn":
		return "zh-Hans"
	case "zh-tw":
		return "zh-Hant"
	case "pt-pt":
		return "pt"
	case "pt-br":
		return "pt-BR"
	}
	return s
}

// formats are the subtitle formats by file extension.
var formats = []string{"srt", "ass", "ssa", "vtt", "sub"}

// providerName is the provider's name within the plugin.
const providerName = "opensubtitlescom"

func (p *plugin) ListSubtitleProviders(context.Context, *pluginv1.ListSubtitleProvidersRequest) (*pluginv1.ListSubtitleProvidersResponse, error) {
	p.mu.Lock()
	enabled := p.cfg.APIKey != ""
	p.mu.Unlock()
	return pluginv1.ListSubtitleProvidersResponse_builder{Providers: []*pluginv1.SubtitleProviderInfo{pluginv1.SubtitleProviderInfo_builder{
		Name: proto.String(providerName), DisplayName: proto.String("OpenSubtitles.com"), Enabled: proto.Bool(enabled),
		HearingImpairedVerifiable: proto.Bool(true), Movies: proto.Bool(true), Episodes: proto.Bool(true),
	}.Build()}}.Build(), nil
}

func (p *plugin) SearchSubtitles(ctx context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	var out []*pluginv1.RemoteSubtitle
	for _, l := range req.GetLanguages() {
		found, err := p.search(ctx, req.GetVideo(), l)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return pluginv1.SearchSubtitlesResponse_builder{Subtitles: out}.Build(), nil
}

// search lists the subtitles of a video in one language.
func (p *plugin) search(ctx context.Context, v *pluginv1.SubtitleVideo, l *pluginv1.SubtitleLanguage) ([]*pluginv1.RemoteSubtitle, error) {
	lang := cmp.Or(languages[l.GetLanguage()], strings.ToLower(l.GetLanguage()))
	if lang == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no language to search subtitles in"))
	}
	params := url.Values{"languages": {lang}}
	if v.GetFileHash() != "" {
		params.Set("moviehash", v.GetFileHash())
	}
	ids, name := v.GetExternalIds(), v.GetTitle()
	prefix, idMatch := "", "imdb_id"
	if v.GetKind() == pluginv1.MediaKind_MEDIA_KIND_EPISODE {
		ids, name, prefix, idMatch = v.GetSeriesExternalIds(), v.GetSeries(), "parent_", "series_imdb_id"
		if v.GetSeason() > 0 {
			params.Set("season_number", strconv.Itoa(int(v.GetSeason())))
		}
		if v.GetEpisode() > 0 {
			params.Set("episode_number", strconv.Itoa(int(v.GetEpisode())))
		}
	}
	var known []string
	switch {
	case ids[keyIMDb] != "":
		params.Set(prefix+"imdb_id", strings.TrimLeft(strings.TrimPrefix(ids[keyIMDb], "tt"), "0"))
		known = append(known, idMatch)
	case ids[keyTMDB] != "":
		params.Set(prefix+"tmdb_id", ids[keyTMDB])
	case name != "":
		params.Set("query", name)
		if v.GetYear() > 0 && prefix == "" {
			params.Set("year", strconv.Itoa(int(v.GetYear())))
		}
	case v.GetFileHash() == "":
		return nil, nil
	}
	if l.GetForced() {
		params.Set("foreign_parts_only", "only")
	} else {
		params.Set("foreign_parts_only", "exclude")
	}
	if !l.GetHearingImpaired() {
		params.Set("hearing_impaired", "exclude")
	}
	found, err := withSession(ctx, p, func(c *client) ([]osSubtitle, error) { return c.search(ctx, params) })
	if err != nil {
		return nil, err
	}
	var out []*pluginv1.RemoteSubtitle
	for _, s := range found {
		a := s.Attributes
		if len(a.Files) == 0 {
			continue
		}
		f := a.Files[0]
		language := hostLanguage(a.Language)
		matches := slices.Clone(known)
		if a.MovieHashMatch {
			matches = append(matches, "hash")
		}
		out = append(out, pluginv1.RemoteSubtitle_builder{
			Provider:                  proto.String(providerName),
			Id:                        proto.String(encodeID(f.FileID, language, a.HearingImpaired, a.ForeignPartsOnly)),
			ReleaseNames:              []string{cmp.Or(a.Release, f.FileName)},
			Language:                  subtitleLanguage(language, a.HearingImpaired, a.ForeignPartsOnly),
			Matches:                   matches,
			HearingImpairedVerifiable: proto.Bool(true),
			Format:                    proto.String(format(f.FileName)),
			DownloadCount:             proto.Int32(int32(min(a.DownloadCount, math.MaxInt32))),
			Comment:                   proto.String(a.Comments),
		}.Build())
	}
	return out, nil
}

func subtitleLanguage(language string, hearingImpaired, forced bool) *pluginv1.SubtitleLanguage {
	return pluginv1.SubtitleLanguage_builder{
		Language: proto.String(language), HearingImpaired: proto.Bool(hearingImpaired), Forced: proto.Bool(forced),
	}.Build()
}

// format is a subtitle file's format; SubRip unless its name says.
func format(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if slices.Contains(formats, ext) {
		return ext
	}
	return "srt"
}

// encodeID carries what a download needs to describe the file.
func encodeID(fileID int64, language string, hearingImpaired, forced bool) string {
	return fmt.Sprintf("%d|%s|%t|%t", fileID, language, hearingImpaired, forced)
}

func decodeID(id string) (fileID int64, language string, hearingImpaired, forced bool, err error) {
	parts := strings.Split(id, "|")
	if len(parts) != 4 {
		return 0, "", false, false, fmt.Errorf("malformed subtitle ID %q", id)
	}
	if fileID, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		return 0, "", false, false, fmt.Errorf("malformed subtitle ID %q", id)
	}
	return fileID, parts[1], parts[2] == "true", parts[3] == "true", nil
}

func (p *plugin) DownloadSubtitle(ctx context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error) {
	fileID, language, hi, forced, err := decodeID(req.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var name string
	data, err := withSession(ctx, p, func(c *client) ([]byte, error) {
		data, n, err := c.download(ctx, fileID)
		name = n
		return data, err
	})
	if err != nil {
		return nil, err
	}
	return pluginv1.DownloadSubtitleResponse_builder{
		Data: data, Format: proto.String(format(name)), Language: subtitleLanguage(language, hi, forced),
	}.Build(), nil
}

// withSession runs a call, logging in again once when the session expired.
func withSession[T any](ctx context.Context, p *plugin, call func(*client) (T, error)) (T, error) {
	var zero T
	for attempt := 0; ; attempt++ {
		c, err := p.client(ctx)
		if err != nil {
			return zero, err
		}
		out, err := call(c)
		if errors.Is(err, errUnauthorized) && c.token != "" && attempt == 0 {
			p.loggedOut()
			continue
		}
		return out, err
	}
}

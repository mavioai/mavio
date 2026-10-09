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

func (p *plugin) SearchSubtitles(ctx context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	q := req.GetQuery()
	lang := cmp.Or(languages[q.GetLanguage()], strings.ToLower(q.GetLanguage()))
	if lang == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no language to search subtitles in"))
	}
	params := url.Values{"languages": {lang}}
	if q.GetFileHash() != "" {
		params.Set("moviehash", q.GetFileHash())
	}
	ids, name := q.GetExternalIds(), q.GetName()
	prefix := ""
	if q.GetKind() == pluginv1.MediaKind_MEDIA_KIND_EPISODE {
		ids, name, prefix = q.GetSeriesExternalIds(), q.GetSeriesName(), "parent_"
		if q.HasSeasonNumber() {
			params.Set("season_number", strconv.Itoa(int(q.GetSeasonNumber())))
		}
		if q.HasEpisodeNumber() {
			params.Set("episode_number", strconv.Itoa(int(q.GetEpisodeNumber())))
		}
	}
	switch {
	case ids[keyIMDb] != "":
		params.Set(prefix+"imdb_id", strings.TrimLeft(strings.TrimPrefix(ids[keyIMDb], "tt"), "0"))
	case ids[keyTMDB] != "":
		params.Set(prefix+"tmdb_id", ids[keyTMDB])
	case name != "":
		params.Set("query", name)
		if q.GetYear() > 0 && prefix == "" {
			params.Set("year", strconv.Itoa(int(q.GetYear())))
		}
	case q.GetFileHash() == "":
		return &pluginv1.SearchSubtitlesResponse{}, nil
	}
	if q.GetForced() {
		params.Set("foreign_parts_only", "only")
	} else {
		params.Set("foreign_parts_only", "exclude")
	}
	if !q.GetHearingImpaired() {
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
		out = append(out, pluginv1.RemoteSubtitle_builder{
			Id:              proto.String(encodeID(f.FileID, language, a.HearingImpaired, a.ForeignPartsOnly)),
			Name:            proto.String(cmp.Or(a.Release, f.FileName)),
			Language:        proto.String(language),
			Format:          proto.String(format(f.FileName)),
			HearingImpaired: proto.Bool(a.HearingImpaired),
			Forced:          proto.Bool(a.ForeignPartsOnly),
			Score:           proto.Float64(score(a.MovieHashMatch, a.HearingImpaired == q.GetHearingImpaired(), a.Ratings, a.DownloadCount)),
			DownloadCount:   proto.Int32(int32(min(a.DownloadCount, math.MaxInt32))),
			HashMatch:       proto.Bool(a.MovieHashMatch),
			Comment:         proto.String(a.Comments),
		}.Build())
	}
	slices.SortStableFunc(out, func(a, b *pluginv1.RemoteSubtitle) int { return cmp.Compare(b.GetScore(), a.GetScore()) })
	if limit := int(req.GetLimit()); limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return pluginv1.SearchSubtitlesResponse_builder{Subtitles: out}.Build(), nil
}

// score ranks a subtitle from 0 to 1: made for the very file first, then
// those flagged as asked, then the best rated and most downloaded.
func score(hashMatch, flagsMatch bool, ratings float64, downloads int) float64 {
	s := 0.0
	if hashMatch {
		s += 0.5
	}
	if flagsMatch {
		s += 0.2
	}
	s += min(ratings, 10) / 10 * 0.15
	s += min(math.Log10(float64(downloads)+1)/5, 1) * 0.15
	return s
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
		Data: data, Format: proto.String(format(name)), Language: proto.String(language),
		HearingImpaired: proto.Bool(hi), Forced: proto.Bool(forced),
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

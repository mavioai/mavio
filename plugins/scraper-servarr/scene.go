package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// The sources of scene mappings.
const (
	sourceServices = "services"
	sourceXEM      = "xem"
)

// sonarrSceneMapping is Sonarr's SceneMapping as its services list it.
type sonarrSceneMapping struct {
	TvdbID            int    `json:"tvdbId"`
	Title             string `json:"title"`
	SearchTitle       string `json:"searchTitle"`
	Season            *int   `json:"season"`
	SceneSeasonNumber *int   `json:"sceneSeasonNumber"`
	SceneOrigin       string `json:"sceneOrigin"`
	SearchMode        *int   `json:"searchMode"`
	Comment           string `json:"comment"`
	FilterRegex       string `json:"filterRegex"`
}

// xemResult is XEM's answer, whose data a failure leaves empty or of
// another type.
type xemResult struct {
	Result  string          `json:"result"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

// xemIgnoredErrors are XEM's failures that mean it has no numbering.
var xemIgnoredErrors = []string{"no single connection", "no show with the tvdb_id"}

// getXEM fetches an XEM answer and decodes its data into out; a failure
// XEM's means it has no numbering leaves out as it is.
func (p *plugin) getXEM(ctx context.Context, path string, query url.Values, out any) error {
	var r xemResult
	if err := p.xem.get(ctx, path, query, &r); err != nil {
		return err
	}
	if strings.EqualFold(r.Result, "failure") {
		if slices.ContainsFunc(xemIgnoredErrors, func(e string) bool { return strings.Contains(r.Message, e) }) {
			return nil
		}
		return fmt.Errorf("xem %s: %s", path, r.Message)
	}
	if err := json.Unmarshal(r.Data, out); err != nil {
		return fmt.Errorf("xem %s: %w", path, err)
	}
	return nil
}

// xemString is a string or a number of XEM's.
func xemString(raw json.RawMessage) string {
	return strings.Trim(string(raw), `"`)
}

type xemNumbers struct {
	Season   int `json:"season"`
	Episode  int `json:"episode"`
	Absolute int `json:"absolute"`
}

type xemMapping struct {
	Scene *xemNumbers `json:"scene"`
	Tvdb  *xemNumbers `json:"tvdb"`
}

// ListSceneMappings lists the scene mappings of Sonarr's services and
// XEM's names. A source that fails is left out, so that the host keeps
// what it listed before; the call fails only when both do.
func (p *plugin) ListSceneMappings(ctx context.Context, _ *pluginv1.ListSceneMappingsRequest) (*pluginv1.ListSceneMappingsResponse, error) {
	services, serr := p.servicesMappings(ctx)
	if serr != nil {
		slog.WarnContext(ctx, "scene mappings of Sonarr's services", "err", serr)
	}
	xem, xerr := p.xemMappings(ctx)
	if xerr != nil {
		slog.WarnContext(ctx, "scene mappings of XEM", "err", xerr)
	}
	if serr != nil && xerr != nil {
		return nil, errors.Join(serr, xerr)
	}
	return pluginv1.ListSceneMappingsResponse_builder{Mappings: append(services, xem...)}.Build(), nil
}

func (p *plugin) servicesMappings(ctx context.Context) ([]*pluginv1.SceneMapping, error) {
	var ms []sonarrSceneMapping
	if err := p.services.get(ctx, "/scenemapping", nil, &ms); err != nil {
		return nil, err
	}
	out := make([]*pluginv1.SceneMapping, 0, len(ms))
	for _, m := range ms {
		sm := sceneMapping(sourceServices, m.TvdbID, m.Title, m.SearchTitle)
		if sm == nil {
			continue
		}
		if m.Season != nil {
			sm.SetSeasonNumber(int32(*m.Season))
		}
		if m.SceneSeasonNumber != nil {
			sm.SetSceneSeasonNumber(int32(*m.SceneSeasonNumber))
		}
		if m.SearchMode != nil {
			sm.SetSearchMode(pluginv1.SceneSearchMode(*m.SearchMode & 3))
		}
		sm.SetSceneOrigin(m.SceneOrigin)
		sm.SetFilterRegex(m.FilterRegex)
		sm.SetComment(m.Comment)
		out = append(out, sm)
	}
	return out, nil
}

// sceneMapping starts a mapping; nil when it has no title or search title,
// which Sonarr skips as invalid.
func sceneMapping(source string, tvdbID int, title, searchTitle string) *pluginv1.SceneMapping {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(searchTitle) == "" || tvdbID <= 0 {
		return nil
	}
	return pluginv1.SceneMapping_builder{
		Source: proto.String(source), SeriesExternalIds: map[string]string{keyTVDB: strconv.Itoa(tvdbID)},
		Title: proto.String(title), SearchTitle: proto.String(searchTitle),
	}.Build()
}

// xemMappings lists XEM's names of series with the seasons releases
// number with them, as GetSceneTvdbNames does.
func (p *plugin) xemMappings(ctx context.Context) ([]*pluginv1.SceneMapping, error) {
	var data map[string][]map[string]json.RawMessage
	if err := p.getXEM(ctx, "/allNames", url.Values{"seasonNumbers": {"True"}, "origin": {"tvdb"}}, &data); err != nil {
		return nil, err
	}
	var out []*pluginv1.SceneMapping
	for _, key := range slices.Sorted(maps.Keys(data)) {
		tvdbID, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		for _, names := range data[key] {
			for _, title := range slices.Sorted(maps.Keys(names)) {
				season, err := strconv.Atoi(xemString(names[title]))
				// Fate/Zero names only its first season.
				if err != nil || (tvdbID == 79151 && season > 1) {
					continue
				}
				if sm := sceneMapping(sourceXEM, tvdbID, title, title); sm != nil {
					sm.SetSceneSeasonNumber(int32(season))
					out = append(out, sm)
				}
			}
		}
	}
	return out, nil
}

func (p *plugin) ListDailySeries(ctx context.Context, _ *pluginv1.ListDailySeriesRequest) (*pluginv1.ListDailySeriesResponse, error) {
	var ds []struct {
		TvdbID int `json:"tvdbId"`
	}
	if err := p.services.get(ctx, "/dailyseries", nil, &ds); err != nil {
		return nil, err
	}
	out := make([]*pluginv1.SeriesExternalIds, 0, len(ds))
	for _, d := range ds {
		if d.TvdbID > 0 {
			out = append(out, pluginv1.SeriesExternalIds_builder{ExternalIds: map[string]string{keyTVDB: strconv.Itoa(d.TvdbID)}}.Build())
		}
	}
	return pluginv1.ListDailySeriesResponse_builder{Series: out}.Build(), nil
}

// xemSeriesTTL is how long the series XEM has numbering for are kept, as
// Sonarr keeps them.
const xemSeriesTTL = 3 * time.Hour

// GetSceneNumbering lists XEM's numbering of a series by TVDB ID, for the
// series XEM has numbering for; mappings without scene numbers are left
// out, as Sonarr leaves them.
func (p *plugin) GetSceneNumbering(ctx context.Context, req *pluginv1.GetSceneNumberingRequest) (*pluginv1.GetSceneNumberingResponse, error) {
	notFound := pluginv1.GetSceneNumberingResponse_builder{Found: proto.Bool(false)}.Build()
	tvdbID, err := strconv.Atoi(req.GetSeriesExternalIds()[keyTVDB])
	if err != nil || tvdbID <= 0 {
		return notFound, nil
	}
	if has, known := p.xemHasMap(ctx, tvdbID); known && !has {
		return notFound, nil
	}
	var mappings []xemMapping
	if err := p.getXEM(ctx, "/all", url.Values{"id": {strconv.Itoa(tvdbID)}, "origin": {"tvdb"}}, &mappings); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	var episodes []*pluginv1.SceneNumbering
	for _, m := range mappings {
		if m.Scene == nil || m.Tvdb == nil || *m.Scene == (xemNumbers{}) {
			continue
		}
		episodes = append(episodes, pluginv1.SceneNumbering_builder{
			SeasonNumber: proto.Int32(int32(m.Tvdb.Season)), EpisodeNumber: proto.Int32(int32(m.Tvdb.Episode)),
			AbsoluteNumber:    proto.Int32(int32(m.Tvdb.Absolute)),
			SceneSeasonNumber: proto.Int32(int32(m.Scene.Season)), SceneEpisodeNumber: proto.Int32(int32(m.Scene.Episode)),
			SceneAbsoluteNumber: proto.Int32(int32(m.Scene.Absolute)),
		}.Build())
	}
	if len(episodes) == 0 {
		return notFound, nil
	}
	return pluginv1.GetSceneNumberingResponse_builder{
		Found: proto.Bool(true), Provider: proto.String(keyTVDB), Episodes: episodes,
	}.Build(), nil
}

// xemHasMap tells whether XEM has numbering for a series, from the list
// of such series it keeps for three hours; known is false when the list
// could not be fetched.
func (p *plugin) xemHasMap(ctx context.Context, tvdbID int) (has, known bool) {
	p.xemMu.Lock()
	defer p.xemMu.Unlock()
	if now := p.clock(); p.xemSeries == nil || now.After(p.xemExpires) {
		var ids []json.RawMessage
		err := p.getXEM(ctx, "/havemap", url.Values{"origin": {"tvdb"}}, &ids)
		if err != nil || len(ids) == 0 {
			slog.WarnContext(ctx, "series XEM has numbering for", "err", err)
			if p.xemSeries == nil {
				return false, false
			}
			// Keep the list a while longer, as Sonarr does.
			p.xemExpires = now.Add(xemSeriesTTL)
		} else {
			p.xemSeries = map[int]bool{}
			for _, raw := range ids {
				if id, err := strconv.Atoi(xemString(raw)); err == nil && id > 0 {
					p.xemSeries[id] = true
				}
			}
			p.xemExpires = now.Add(xemSeriesTTL)
		}
	}
	return p.xemSeries[tvdbID], true
}

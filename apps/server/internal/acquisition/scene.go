package acquisition

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// JobSceneMappings refreshes the scene mappings and daily series of the
// scene mapping providers, every SceneMappingInterval, as Sonarr's
// UpdateSceneMappingCommand, then the scene numbering of wanted series,
// which Sonarr refreshes with its series.
const JobSceneMappings = "acquisition.scene_mappings"

// SceneMappingInterval is how often the scene mappings are refreshed, as
// Sonarr's.
const SceneMappingInterval = 3 * time.Hour

// SceneProvider is a scene mapping provider plugin.
type SceneProvider struct {
	ID     string
	Client pluginv1connect.SceneMappingProviderServiceClient
}

// sceneCache keeps what the scene mapping providers list, as Sonarr's
// SceneMappingService and DailySeriesService cache it.
type sceneCache struct {
	mu sync.Mutex
	// mappings and daily are by provider: a provider that fails keeps
	// what it listed before.
	mappings map[string][]series.SceneMapping
	daily    map[string][]int
	index    *series.SceneMappings
}

func (r *Runtime) sceneProviders() []SceneProvider {
	if r.SceneProviders == nil {
		return nil
	}
	return r.SceneProviders()
}

// UpdateSceneMappings lists the scene mappings and daily series of every
// scene mapping provider, as Sonarr's SceneMappingService.UpdateMappings
// and DailySeriesService; a provider that fails keeps what it listed
// before.
func (r *Runtime) UpdateSceneMappings(ctx context.Context) error {
	providers := r.sceneProviders()
	mappings := map[string][]series.SceneMapping{}
	daily := map[string][]int{}
	r.scene.mu.Lock()
	maps.Copy(mappings, r.scene.mappings)
	maps.Copy(daily, r.scene.daily)
	r.scene.mu.Unlock()
	live := map[string]bool{}
	for _, p := range providers {
		live[p.ID] = true
		resp, err := p.Client.ListSceneMappings(ctx, &pluginv1.ListSceneMappingsRequest{})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't list scene mappings", "plugin", p.ID, "err", err)
		} else {
			list := make([]series.SceneMapping, 0, len(resp.GetMappings()))
			for _, m := range resp.GetMappings() {
				if sm, ok := sceneMappingFromPlugin(m); ok {
					list = append(list, sm)
				}
			}
			mappings[p.ID] = list
		}
		dresp, err := p.Client.ListDailySeries(ctx, &pluginv1.ListDailySeriesRequest{})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't list daily series", "plugin", p.ID, "err", err)
			continue
		}
		var ids []int
		for _, s := range dresp.GetSeries() {
			if id, err := strconv.Atoi(s.GetExternalIds()["tvdb"]); err == nil && id > 0 {
				ids = append(ids, id)
			}
		}
		daily[p.ID] = ids
	}
	// Providers no longer running are forgotten.
	maps.DeleteFunc(mappings, func(id string, _ []series.SceneMapping) bool { return !live[id] })
	maps.DeleteFunc(daily, func(id string, _ []int) bool { return !live[id] })
	var all []series.SceneMapping
	for _, id := range slices.Sorted(maps.Keys(mappings)) {
		all = append(all, mappings[id]...)
	}
	index := series.NewSceneMappings(all)
	r.scene.mu.Lock()
	r.scene.mappings, r.scene.daily, r.scene.index = mappings, daily, index
	r.scene.mu.Unlock()
	r.logger().InfoContext(ctx, "updated scene mappings", "mappings", index.Len())
	return nil
}

// sceneMappingFromPlugin returns the mapping of a plugin's, of a series
// by TVDB ID.
func sceneMappingFromPlugin(m *pluginv1.SceneMapping) (series.SceneMapping, bool) {
	tvdb, err := strconv.Atoi(m.GetSeriesExternalIds()["tvdb"])
	if err != nil || tvdb <= 0 {
		return series.SceneMapping{}, false
	}
	sm := series.SceneMapping{
		Title: m.GetTitle(), TvdbID: tvdb, SearchTerm: m.GetSearchTitle(), SceneOrigin: m.GetSceneOrigin(),
		Comment: m.GetComment(), FilterRegex: m.GetFilterRegex(), Type: "ServicesProvider",
	}
	if m.GetSource() == "xem" {
		sm.Type = "XemService"
	}
	if sm.SearchTerm == "" {
		sm.SearchTerm = sm.Title
	}
	if m.HasSeasonNumber() && m.GetSeasonNumber() >= 0 {
		sm.SeasonNumber = new(int(m.GetSeasonNumber()))
	}
	if m.HasSceneSeasonNumber() && m.GetSceneSeasonNumber() >= 0 {
		sm.SceneSeasonNumber = new(int(m.GetSceneSeasonNumber()))
	}
	switch m.GetSearchMode() {
	case pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_ID:
		sm.SearchMode = series.SearchModeSearchID
	case pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_TITLE:
		sm.SearchMode = series.SearchModeSearchTitle
	case pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_ID_AND_TITLE:
		sm.SearchMode = series.SearchModeSearchIDAndTitle
	}
	return sm, true
}

// sceneMappings returns the scene mappings, refreshed first if they never
// were.
func (r *Runtime) sceneMappings(ctx context.Context) *series.SceneMappings {
	r.scene.mu.Lock()
	index := r.scene.index
	r.scene.mu.Unlock()
	if index == nil && len(r.sceneProviders()) > 0 {
		if err := r.UpdateSceneMappings(ctx); err != nil {
			r.logger().WarnContext(ctx, "couldn't update scene mappings", "err", err)
		}
		r.scene.mu.Lock()
		index = r.scene.index
		r.scene.mu.Unlock()
	}
	if index == nil {
		return series.NewSceneMappings(nil)
	}
	return index
}

// isDaily reports whether the scene mapping providers list a series as
// daily, as Sonarr's DailySeriesService.IsDailySeries.
func (r *Runtime) isDaily(ctx context.Context, tvdbID int) bool {
	r.sceneMappings(ctx)
	r.scene.mu.Lock()
	defer r.scene.mu.Unlock()
	for _, ids := range r.scene.daily {
		if slices.Contains(ids, tvdbID) {
			return true
		}
	}
	return false
}

// UpdateScenes refreshes the scene mappings and daily series, then the
// scene numbering of every wanted series, as the job JobSceneMappings
// does.
func (r *Runtime) UpdateScenes(ctx context.Context) error {
	if err := r.UpdateSceneMappings(ctx); err != nil {
		return err
	}
	wanted, err := r.Store.Wanted().List(ctx)
	if err != nil {
		return err
	}
	for _, w := range wanted {
		it, err := r.Store.Items().Get(ctx, w.ItemID)
		if err != nil {
			return err
		}
		if it.Kind != core.KindSeries {
			continue
		}
		if err := r.updateSceneNumbering(ctx, it); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().ErrorContext(ctx, "couldn't update scene numbering", "series", it.Name, "err", err)
		}
	}
	return nil
}

// updateSceneNumbering keeps on a series' episodes how releases number
// them, as the first scene mapping provider that knows the series tells,
// extrapolated to the episodes it leaves out, as Sonarr's
// XemService.PerformUpdate. A series no provider knows loses its scene
// numbering; without providers, or when one fails and none knows the
// series, it is kept.
func (r *Runtime) updateSceneNumbering(ctx context.Context, it core.Item) error {
	tvdbID, _ := strconv.Atoi(it.ExternalIDs[core.ProviderTVDB])
	providers := r.sceneProviders()
	if tvdbID <= 0 || len(providers) == 0 {
		return nil
	}
	var mappings []series.XemMapping
	found, failed := false, false
	req := pluginv1.GetSceneNumberingRequest_builder{SeriesExternalIds: map[string]string{"tvdb": strconv.Itoa(tvdbID)}}.Build()
	for _, p := range providers {
		resp, err := p.Client.GetSceneNumbering(ctx, req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.logger().WarnContext(ctx, "couldn't get scene numbering", "plugin", p.ID, "tvdb", tvdbID, "err", err)
			failed = true
			continue
		}
		if !resp.GetFound() || (resp.GetProvider() != "" && resp.GetProvider() != "tvdb") {
			continue
		}
		found = true
		for _, e := range resp.GetEpisodes() {
			mappings = append(mappings, series.XemMapping{
				Tvdb: series.XemNumber{Season: int(e.GetSeasonNumber()), Episode: int(e.GetEpisodeNumber())},
				Scene: series.XemNumber{
					Season: int(e.GetSceneSeasonNumber()), Episode: int(e.GetSceneEpisodeNumber()), Absolute: int(e.GetSceneAbsoluteNumber()),
				},
			})
		}
		break
	}
	if !found && failed {
		return nil
	}
	var states []core.EpisodeState
	if found {
		episodes, err := r.episodeNumbers(ctx, it.ID)
		if err != nil {
			return err
		}
		for _, n := range series.NumberScenes(episodes, mappings) {
			st := core.EpisodeState{
				EpisodeNumber: core.EpisodeNumber{Season: n.Season, Episode: n.Episode},
				SceneSeason:   n.SceneSeason, SceneEpisode: n.SceneEpisode, SceneAbsolute: n.SceneAbsolute,
				UnverifiedSceneNumbering: n.Unverified,
			}
			if st.HasSceneNumbering() {
				states = append(states, st)
			}
		}
	}
	return r.Store.EpisodeStates().PutSceneNumbering(ctx, it.ID, states)
}

// episodeNumbers returns the seasons and numbers of a series' episodes,
// with those multi-episode files cover.
func (r *Runtime) episodeNumbers(ctx context.Context, seriesID core.ID) ([][2]int, error) {
	seen := map[[2]int]bool{}
	var out [][2]int
	for c, err := range r.Store.Items().Walk(ctx, core.ItemQuery{
		ParentID: seriesID, Recursive: true, Kinds: []core.ItemKind{core.KindEpisode}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return nil, err
		}
		if c.ParentIndexNumber == nil || c.IndexNumber == nil {
			continue
		}
		last := *c.IndexNumber
		if c.IndexNumberEnd != nil && *c.IndexNumberEnd > last {
			last = *c.IndexNumberEnd
		}
		for n := *c.IndexNumber; n <= last; n++ {
			k := [2]int{*c.ParentIndexNumber, n}
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	slices.SortFunc(out, func(a, b [2]int) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
	return out, nil
}

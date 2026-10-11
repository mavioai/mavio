package acquisition

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/acquire/series"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// JobSceneMappings refreshes the scene mappings and daily series of the
// scene mapping providers, every SceneMappingInterval, as Sonarr's
// UpdateSceneMappingCommand.
const JobSceneMappings = "acquisition.scene_mappings"

// SceneMappingInterval is how often the scene mappings are refreshed, as
// Sonarr's.
const SceneMappingInterval = 3 * time.Hour

// sceneNumberingTTL is how long the scene numbering of a series is kept
// before it is asked for again.
const sceneNumberingTTL = 12 * time.Hour

// SceneProvider is a scene mapping provider plugin.
type SceneProvider struct {
	ID     string
	Client pluginv1connect.SceneMappingProviderServiceClient
}

// sceneNumber is how releases number an episode.
type sceneNumber struct {
	season, episode, absolute int
	hasAbsolute               bool
}

// sceneNumbering is the scene numbering of a series as a provider told,
// by TVDB season and episode.
type sceneNumbering struct {
	at       time.Time
	episodes map[[2]int]sceneNumber
}

// sceneCache keeps what the scene mapping providers list, as Sonarr's
// SceneMappingService and DailySeriesService cache it, and the scene
// numbering of series, which Sonarr stores on episodes.
type sceneCache struct {
	mu sync.Mutex
	// mappings and daily are by provider: a provider that fails keeps
	// what it listed before.
	mappings  map[string][]series.SceneMapping
	daily     map[string][]int
	index     *series.SceneMappings
	numbering map[int]*sceneNumbering
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

// sceneNumbers returns how releases number a series' episodes, by TVDB
// season and episode, as the first provider that knows the series tells
// (Sonarr's XemService); nil when none does. Numberings are kept for
// sceneNumberingTTL.
func (r *Runtime) sceneNumbers(ctx context.Context, tvdbID int) map[[2]int]sceneNumber {
	if tvdbID <= 0 {
		return nil
	}
	now := r.now()
	r.scene.mu.Lock()
	cached := r.scene.numbering[tvdbID]
	r.scene.mu.Unlock()
	if cached != nil && now.Sub(cached.at) < sceneNumberingTTL {
		return cached.episodes
	}
	n := &sceneNumbering{at: now}
	req := pluginv1.GetSceneNumberingRequest_builder{SeriesExternalIds: map[string]string{"tvdb": strconv.Itoa(tvdbID)}}.Build()
	for _, p := range r.sceneProviders() {
		resp, err := p.Client.GetSceneNumbering(ctx, req)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			r.logger().WarnContext(ctx, "couldn't get scene numbering", "plugin", p.ID, "tvdb", tvdbID, "err", err)
			continue
		}
		if !resp.GetFound() || (resp.GetProvider() != "" && resp.GetProvider() != "tvdb") {
			continue
		}
		n.episodes = map[[2]int]sceneNumber{}
		for _, e := range resp.GetEpisodes() {
			n.episodes[[2]int{int(e.GetSeasonNumber()), int(e.GetEpisodeNumber())}] = sceneNumber{
				season: int(e.GetSceneSeasonNumber()), episode: int(e.GetSceneEpisodeNumber()),
				absolute: int(e.GetSceneAbsoluteNumber()), hasAbsolute: e.GetSceneAbsoluteNumber() > 0,
			}
		}
		break
	}
	r.scene.mu.Lock()
	if r.scene.numbering == nil {
		r.scene.numbering = map[int]*sceneNumbering{}
	}
	r.scene.numbering[tvdbID] = n
	r.scene.mu.Unlock()
	return n.episodes
}

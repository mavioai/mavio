package acquisition

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func sceneNumber(season, episode, sceneSeason, sceneEpisode int) *pluginv1.SceneNumbering {
	return pluginv1.SceneNumbering_builder{
		SeasonNumber: new(int32(season)), EpisodeNumber: new(int32(episode)),
		SceneSeasonNumber: new(int32(sceneSeason)), SceneEpisodeNumber: new(int32(sceneEpisode)),
	}.Build()
}

// sceneStates returns a series' stored scene numbering as "SxE>SxE",
// with "?" after unverified numbers.
func (e *seriesEnv) sceneStates(seriesID core.ID) []string {
	e.t.Helper()
	states, err := e.db.EpisodeStates().List(e.ctx, seriesID)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []string
	for _, s := range states {
		if !s.HasSceneNumbering() {
			continue
		}
		v := slot{s.Season, s.Episode}.String() + ">"
		if s.SceneSeason != nil && s.SceneEpisode != nil {
			v += slot{*s.SceneSeason, *s.SceneEpisode}.String()
		}
		if s.UnverifiedSceneNumbering {
			v += "?"
		}
		out = append(out, v)
	}
	return out
}

func TestSceneNumberingKeptOnEpisodes(t *testing.T) {
	e := newSeriesEnv(t)
	// The scene numbers S01E03 as S02E01, and the seasons after shifted.
	e.scene.numbering = map[string][]*pluginv1.SceneNumbering{"81189": {sceneNumber(1, 3, 2, 1)}}
	s := e.addSeries(81189, core.SeriesStandard)
	want := []string{slot{1, 3}.String() + ">" + slot{2, 1}.String(), slot{2, 1}.String() + ">" + slot{3, 1}.String() + "?", slot{2, 2}.String() + ">" + slot{3, 2}.String() + "?"}
	if got := e.sceneStates(s.ID); !slices.Equal(got, want) {
		t.Fatalf("scene numbering = %v, want = %v", got, want)
	}

	// Searches and releases number the episode as the scene does.
	ep3 := e.episode(s.ID, 1, 3)
	const title = "Breaking.Bad.S02E01.720p.HDTV.x264-GRP"
	e.tvReleases(gb, title)
	e.searchEpisodes(s.ID, ep3.ID)
	if req := e.indexer.requests[0]; req.GetSeason() != 2 || req.GetEpisode() != "1" {
		t.Errorf("search = season %d episode %q, want = the scene's S02E01", req.GetSeason(), req.GetEpisode())
	}
	if got := e.client.titles(); !slices.Equal(got, []string{title}) {
		t.Fatalf("grabbed = %v, want = [%s]", got, title)
	}
	if d := e.download(hashOf(title)); !slices.Equal(d.ItemIDs, []core.ID{ep3.ID}) {
		t.Errorf("download episodes = %v, want = S01E03", d.ItemIDs)
	}

	// A provider failing keeps the numbering; the series no longer known
	// loses it.
	e.scene.numberingErr = errors.New("down")
	if err := e.rt.UpdateScenes(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.sceneStates(s.ID); !slices.Equal(got, want) {
		t.Errorf("scene numbering after a failure = %v, want = %v", got, want)
	}
	e.scene.numberingErr = nil
	e.scene.numbering = nil
	if err := e.rt.UpdateScenes(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.sceneStates(s.ID); len(got) != 0 {
		t.Errorf("scene numbering of an unknown series = %v, want = none", got)
	}
}

func TestEpisodesKeepTheirLastSearchTimes(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	season1 := []core.Item{e.episode(s.ID, 1, 1), e.episode(s.ID, 1, 2), e.episode(s.ID, 1, 3)}
	e.tvReleases(gb)
	searched := e.clock
	e.searchEpisodes(s.ID, ids(season1...)...)
	states, err := e.db.EpisodeStates().List(e.ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, st := range states {
		if st.LastSearchedAt.Equal(searched) {
			got = append(got, slot{st.Season, st.Episode}.String())
		}
	}
	if want := []string{slot{1, 1}.String(), slot{1, 2}.String(), slot{1, 3}.String()}; !slices.Equal(got, want) {
		t.Errorf("episodes searched = %v, want = %v", got, want)
	}

	// Searching for the missing episodes searches the season never
	// searched for first.
	e.clock = e.clock.Add(time.Hour)
	before := len(e.indexer.requests)
	if err := e.rt.SearchEpisodes(e.ctx, EpisodeSearchPayload{Scope: ScopeMissing}); err != nil {
		t.Fatal(err)
	}
	var seasons []int32
	for _, r := range e.indexer.requests[before:] {
		if len(r.GetExternalIds()) > 0 && !slices.Contains(seasons, r.GetSeason()) {
			seasons = append(seasons, r.GetSeason())
		}
	}
	if !slices.Equal(seasons, []int32{2, 1}) {
		t.Errorf("seasons searched = %v, want = [2 1]", seasons)
	}
}

func TestSceneMappingSearchModes(t *testing.T) {
	for _, tt := range []struct {
		mode pluginv1.SceneSearchMode
		want releases.SearchMode
	}{
		{pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_UNSPECIFIED, releases.SearchDefault},
		{pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_ID, releases.SearchByID},
		{pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_TITLE, releases.SearchByTitle},
		{pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_ID_AND_TITLE, releases.SearchByIDAndTitle},
	} {
		m, ok := sceneMappingFromPlugin(pluginv1.SceneMapping_builder{
			SeriesExternalIds: map[string]string{"tvdb": "81189"}, Title: new("Breaking Bad"), SearchMode: tt.mode.Enum(),
		}.Build())
		if !ok {
			t.Fatalf("mapping of mode %v dropped", tt.mode)
		}
		if got := queryMode(m.SearchMode); got != tt.want {
			t.Errorf("mode %v searches as %v, want = %v", tt.mode, got, tt.want)
		}
	}
}

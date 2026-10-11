package series

import "slices"

// XemNumber is an episode's season, number and absolute number.
type XemNumber struct {
	Season, Episode, Absolute int
}

// XemMapping maps an episode as TVDB numbers it to how the scene numbers
// it, as Sonarr's XemSceneTvdbMapping.
type XemMapping struct {
	Tvdb, Scene XemNumber
}

// XemNumbering is how the scene numbers an episode, by its TVDB season
// and number.
type XemNumbering struct {
	Season, Episode int
	// SceneSeason, SceneEpisode and SceneAbsolute are nil when the scene
	// numbers the episode as TVDB does.
	SceneSeason, SceneEpisode, SceneAbsolute *int
	// Unverified is set when the numbering is extrapolated from that of
	// other episodes, or the episode is likely to be numbered apart from
	// TVDB though no mapping says how.
	Unverified bool
}

// NumberScenes numbers a series' episodes, given by TVDB season and
// number, as the mappings tell, extrapolating the mappings to the
// episodes they leave out, as Sonarr's XemService.PerformUpdate and
// ExtrapolateMappings. Mappings of episodes not given, and mappings whose
// scene numbers are all zero, are skipped.
func NumberScenes(episodes [][2]int, mappings []XemMapping) []XemNumbering {
	out := make([]XemNumbering, len(episodes))
	index := make(map[[2]int]int, len(episodes))
	for i, e := range episodes {
		out[i] = XemNumbering{Season: e[0], Episode: e[1]}
		index[e] = i
	}
	for _, m := range mappings {
		i, ok := index[[2]int{m.Tvdb.Season, m.Tvdb.Episode}]
		if !ok || (m.Scene.Absolute == 0 && m.Scene.Season == 0 && m.Scene.Episode == 0) {
			continue
		}
		out[i].SceneSeason, out[i].SceneEpisode = new(m.Scene.Season), new(m.Scene.Episode)
		if m.Scene.Absolute > 0 {
			out[i].SceneAbsolute = new(m.Scene.Absolute)
		} else {
			out[i].SceneAbsolute = nil
		}
	}
	if slices.ContainsFunc(out, func(e XemNumbering) bool { return e.SceneEpisode != nil && *e.SceneSeason != 0 }) {
		extrapolate(out, mappings)
	}
	return out
}

// extrapolate is Sonarr's XemService.ExtrapolateMappings.
func extrapolate(episodes []XemNumbering, mappings []XemMapping) {
	mappedSeasons := map[int]bool{}
	for _, e := range episodes {
		if e.Season != 0 && e.SceneEpisode != nil {
			mappedSeasons[e.Season] = true
		}
	}
	sceneEpisodes := map[int]map[int]bool{}
	firstTvdbEpisode := map[int]int{}
	lastSceneSeason, lastTvdbSeason := 0, 0
	for i, m := range mappings {
		if sceneEpisodes[m.Scene.Season] == nil {
			sceneEpisodes[m.Scene.Season] = map[int]bool{}
		}
		sceneEpisodes[m.Scene.Season][m.Scene.Episode] = true
		if first, ok := firstTvdbEpisode[m.Tvdb.Season]; !ok || m.Tvdb.Episode < first {
			firstTvdbEpisode[m.Tvdb.Season] = m.Tvdb.Episode
		}
		if i == 0 || m.Scene.Season > lastSceneSeason {
			lastSceneSeason = m.Scene.Season
		}
		if i == 0 || m.Tvdb.Season > lastTvdbSeason {
			lastTvdbSeason = m.Tvdb.Season
		}
	}

	// Episodes not on XEM are unverified when a mapping exists for an
	// earlier episode of their season, or a mapping numbers an episode as
	// they are, or their season follows the last mapped one, shifted.
	for i := range episodes {
		e := &episodes[i]
		if e.Season == 0 || e.SceneEpisode != nil {
			continue
		}
		if mappedSeasons[e.Season] {
			if firstTvdbEpisode[e.Season] <= e.Episode || sceneEpisodes[e.Season][e.Episode] {
				e.Unverified = true
			}
		} else if lastSceneSeason != lastTvdbSeason && e.Season > lastTvdbSeason {
			e.Unverified = true
		}
	}

	for i := range episodes {
		e := &episodes[i]
		if e.Season == 0 || e.SceneEpisode != nil || e.Season < lastTvdbSeason || !e.Unverified {
			continue
		}
		var season []XemMapping
		for _, m := range mappings {
			if m.Tvdb.Season == e.Season {
				season = append(season, m)
			}
		}
		if slices.ContainsFunc(season, func(m XemMapping) bool { return m.Tvdb.Episode >= e.Episode }) {
			continue
		}
		if len(season) > 0 {
			// The last of ties, as Sonarr's OrderBy(…).Last().
			last := season[0]
			for _, m := range season {
				if m.Tvdb.Episode >= last.Tvdb.Episode {
					last = m
				}
			}
			var lastOfScene *XemMapping
			for j := range mappings {
				m := &mappings[j]
				if m.Scene.Season == last.Scene.Season && (lastOfScene == nil || m.Scene.Episode >= lastOfScene.Scene.Episode) {
					lastOfScene = m
				}
			}
			if lastOfScene.Tvdb.Season == 0 {
				continue
			}
			offset := e.Episode - last.Tvdb.Episode
			e.SceneSeason, e.SceneEpisode = new(last.Scene.Season), new(last.Scene.Episode+offset)
			if last.Scene.Absolute > 0 {
				e.SceneAbsolute = new(last.Scene.Absolute + offset)
			}
		} else if lastTvdbSeason != lastSceneSeason {
			e.SceneSeason, e.SceneEpisode = new(lastSceneSeason+e.Season-lastTvdbSeason), new(e.Episode)
		}
	}
}

package series

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/libs/release"
)

// SceneMappingService finds the scene mappings of release titles, as
// Sonarr's ISceneMappingService.
type SceneMappingService interface {
	// FindSceneMapping returns the mapping of a parsed series title, of a
	// release title and a scene season (-1 for any), or nil. It fails
	// when the title maps to several series.
	FindSceneMapping(seriesTitle, releaseTitle string, sceneSeasonNumber int) (*SceneMapping, error)
	// GetSceneSeasonNumber returns the scene season a series title is
	// of, if the mappings say.
	GetSceneSeasonNumber(seriesTitle, releaseTitle string) (int, bool, error)
}

// SceneMappings are the scene mappings of the scene mapping providers,
// indexed as Sonarr's SceneMappingService caches them. The zero value
// has none.
type SceneMappings struct {
	byTerm map[string][]*SceneMapping
	byTvdb map[int][]*SceneMapping
}

// NewSceneMappings indexes mappings. Mappings without a title or search
// term are left out, as Sonarr's UpdateMappings does.
func NewSceneMappings(mappings []SceneMapping) *SceneMappings {
	m := &SceneMappings{byTerm: map[string][]*SceneMapping{}, byTvdb: map[int][]*SceneMapping{}}
	for i := range mappings {
		sm := mappings[i]
		if strings.TrimSpace(sm.Title) == "" || strings.TrimSpace(sm.SearchTerm) == "" {
			continue
		}
		if sm.ParseTerm == "" {
			sm.ParseTerm = release.CleanSeriesTitle(sm.Title)
		}
		m.byTerm[sm.ParseTerm] = append(m.byTerm[sm.ParseTerm], &sm)
		m.byTvdb[sm.TvdbID] = append(m.byTvdb[sm.TvdbID], &sm)
	}
	return m
}

// Len returns the number of mappings.
func (m *SceneMappings) Len() int {
	if m == nil {
		return 0
	}
	n := 0
	for _, l := range m.byTvdb {
		n += len(l)
	}
	return n
}

// FindByTvdbID returns the mappings of a series.
func (m *SceneMappings) FindByTvdbID(tvdbID int) []*SceneMapping {
	if m == nil {
		return nil
	}
	return m.byTvdb[tvdbID]
}

// GetSceneNames returns the English titles to search a series by for
// seasons or scene seasons, as Sonarr's SceneMappingService.GetSceneNames:
// those of the seasons, and those of no season unless they number as
// TVDB does.
func (m *SceneMappings) GetSceneNames(tvdbID int, seasonNumbers, sceneSeasonNumbers []int) []string {
	var names []string
	for _, n := range m.FindByTvdbID(tvdbID) {
		season, sceneSeason := orNone(n.SeasonNumber), orNone(n.SceneSeasonNumber)
		if !slices.Contains(seasonNumbers, season) && !slices.Contains(sceneSeasonNumbers, sceneSeason) &&
			(season != -1 || sceneSeason != -1 || n.SceneOrigin == "tvdb") {
			continue
		}
		if !isEnglish(n.SearchTerm) || slices.ContainsFunc(names, func(s string) bool { return strings.EqualFold(s, n.SearchTerm) }) {
			continue
		}
		names = append(names, n.SearchTerm)
	}
	return names
}

// orNone returns a season number, -1 for none.
func orNone(n *int) int {
	if n == nil {
		return -1
	}
	return *n
}

// isEnglish reports whether a title has Latin-1 characters only.
func isEnglish(title string) bool {
	for _, r := range title {
		if r > 255 {
			return false
		}
	}
	return true
}

// FindTvdbID returns the TVDB ID of the series a title maps to.
func (m *SceneMappings) FindTvdbID(seriesTitle, releaseTitle string, sceneSeasonNumber int) (int, bool, error) {
	sm, err := m.FindSceneMapping(seriesTitle, releaseTitle, sceneSeasonNumber)
	if err != nil || sm == nil {
		return 0, false, err
	}
	return sm.TvdbID, true, nil
}

// ErrInvalidSceneMapping is returned when a title maps to several
// series, as Sonarr's InvalidSceneMappingException.
var ErrInvalidSceneMapping = errors.New("scene mapping matches several series")

// FindSceneMapping implements SceneMappingService.
func (m *SceneMappings) FindSceneMapping(seriesTitle, releaseTitle string, sceneSeasonNumber int) (*SceneMapping, error) {
	if strings.TrimSpace(seriesTitle) == "" {
		return nil, nil
	}
	mappings := m.findMappings(seriesTitle, releaseTitle)
	if mappings == nil {
		return nil, nil
	}
	mappings = filterBySeason(mappings, sceneSeasonNumber)
	var distinct []*SceneMapping
	for _, sm := range mappings {
		if !slices.ContainsFunc(distinct, func(d *SceneMapping) bool { return d.TvdbID == sm.TvdbID }) {
			distinct = append(distinct, sm)
		}
	}
	switch len(distinct) {
	case 0:
		return nil, nil
	case 1:
		return distinct[0], nil
	}
	ids := make([]string, len(distinct))
	for i, d := range distinct {
		ids[i] = fmt.Sprint(d.TvdbID)
	}
	return nil, fmt.Errorf("%w: %q matches TVDB IDs %s", ErrInvalidSceneMapping, releaseTitle, strings.Join(ids, ", "))
}

// GetSceneSeasonNumber implements SceneMappingService.
func (m *SceneMappings) GetSceneSeasonNumber(seriesTitle, releaseTitle string) (int, bool, error) {
	sm, err := m.FindSceneMapping(seriesTitle, releaseTitle, -1)
	if err != nil || sm == nil || sm.SceneSeasonNumber == nil {
		return 0, false, err
	}
	return *sm.SceneSeasonNumber, true, nil
}

// findMappings returns the mappings of a series title: those whose
// filter matches the release title, else those without a filter; among
// several, those titled exactly so, else those of the closest title.
func (m *SceneMappings) findMappings(seriesTitle, releaseTitle string) []*SceneMapping {
	if m == nil {
		return nil
	}
	candidates, ok := m.byTerm[release.CleanSeriesTitle(seriesTitle)]
	if !ok {
		return nil
	}
	candidates = filterByRelease(candidates, releaseTitle)
	if len(candidates) <= 1 {
		return candidates
	}
	var exact []*SceneMapping
	for _, c := range candidates {
		if c.Title == seriesTitle {
			exact = append(exact, c)
		}
	}
	if len(exact) > 0 {
		slices.SortStableFunc(exact, func(a, b *SceneMapping) int { return cmp.Compare(orNone(b.SeasonNumber), orNone(a.SeasonNumber)) })
		return exact
	}
	sorted := slices.Clone(candidates)
	slices.SortStableFunc(sorted, func(a, b *SceneMapping) int {
		if r := cmp.Compare(levenshtein(seriesTitle, a.Title, 10, 1, 10), levenshtein(seriesTitle, b.Title, 10, 1, 10)); r != 0 {
			return r
		}
		return cmp.Compare(orNone(b.SeasonNumber), orNone(a.SeasonNumber))
	})
	var out []*SceneMapping
	for _, c := range candidates {
		if c.Title == sorted[0].Title {
			out = append(out, c)
		}
	}
	return out
}

// filterByRelease returns the candidates whose filter matches the
// release title, else those without a filter.
func filterByRelease(candidates []*SceneMapping, releaseTitle string) []*SceneMapping {
	var filtered, normal []*SceneMapping
	for _, c := range candidates {
		if strings.TrimSpace(c.FilterRegex) != "" {
			filtered = append(filtered, c)
		} else {
			normal = append(normal, c)
		}
	}
	if strings.TrimSpace(releaseTitle) == "" {
		return normal
	}
	simple := release.SimplifySeriesTitle(releaseTitle)
	var matching []*SceneMapping
	for _, c := range filtered {
		re, err := regexp2.Compile(c.FilterRegex, regexp2.None)
		if err != nil {
			continue
		}
		re.MatchTimeout = time.Second
		if ok, _ := re.MatchString(simple); ok {
			matching = append(matching, c)
		}
	}
	if len(matching) > 0 {
		return matching
	}
	return normal
}

// filterBySeason returns, for a scene season, the latest mapping of each
// title for a season up to it among those of a season; else those of
// any season.
func filterBySeason(candidates []*SceneMapping, sceneSeasonNumber int) []*SceneMapping {
	var filtered, normal []*SceneMapping
	for _, c := range candidates {
		if orNone(c.SceneSeasonNumber) != -1 && orNone(c.SeasonNumber) != -1 {
			filtered = append(filtered, c)
		} else {
			normal = append(normal, c)
		}
	}
	if sceneSeasonNumber == -1 || len(filtered) == 0 {
		return normal
	}
	var titles []string
	best := map[string]*SceneMapping{}
	for _, c := range filtered {
		if *c.SceneSeasonNumber > sceneSeasonNumber {
			continue
		}
		b, ok := best[c.Title]
		if !ok {
			titles = append(titles, c.Title)
		}
		if !ok || *c.SceneSeasonNumber > *b.SceneSeasonNumber || (*c.SceneSeasonNumber == *b.SceneSeasonNumber && *c.SeasonNumber > *b.SeasonNumber) {
			best[c.Title] = c
		}
	}
	out := make([]*SceneMapping, len(titles))
	for i, t := range titles {
		out[i] = best[t]
	}
	return out
}

// levenshtein is Sonarr's LevenshteinDistance with its costs.
func levenshtein(text, other string, costInsert, costDelete, costSubstitute int) int {
	if text == other {
		return 0
	}
	t, o := []rune(text), []rune(other)
	if len(t) == 0 {
		return len(o) * costInsert
	}
	if len(o) == 0 {
		return len(t) * costDelete
	}
	matrix := make([]int, len(o)+1)
	for i := 1; i < len(matrix); i++ {
		matrix[i] = i * costInsert
	}
	for i := range t {
		topLeft := matrix[0]
		matrix[0] += costDelete
		for j := range o {
			top, left := matrix[j], matrix[j+1]
			sub := topLeft
			if t[i] != o[j] {
				sub += costSubstitute
			}
			topLeft = matrix[j+1]
			matrix[j+1] = min(top+costInsert, left+costDelete, sub)
		}
	}
	return matrix[len(o)]
}

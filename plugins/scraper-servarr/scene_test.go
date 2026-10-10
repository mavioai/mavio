package main

import (
	"fmt"
	"slices"
	"testing"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

const sceneMappings = `[
	{"mappingId": "s-1", "tvdbId": 152831, "title": "Adventure Time With Finn and Jake", "searchTitle": "Adventure Time", "season": -1},
	{"mappingId": "s-2", "tvdbId": 262148, "title": "Mayday", "searchTitle": "Mayday", "season": -1, "filterRegex": "(?i)Mayday\\.S01E"},
	{"mappingId": "s-3", "tvdbId": 1, "title": "Show S2", "searchTitle": "Show", "season": 2, "sceneSeasonNumber": 1,
	 "sceneOrigin": "tvdb", "searchMode": 2, "comment": "renamed"},
	{"mappingId": "s-4", "tvdbId": 2, "title": "", "searchTitle": "Nameless", "season": -1}]`

const xemNames = `{"result": "success", "message": "", "data": {
	"79151": [{"Fate/Zero": 1}, {"Fate Zero 2": 2}],
	"262980": [{"Attack on Titan": "all"}, {"Shingeki no Kyojin": 1}, {"AoT Season 2": "2"}]}}`

func describe(ms []*pluginv1.SceneMapping) []string {
	var out []string
	for _, m := range ms {
		s := fmt.Sprintf("%s %s %q/%q", m.GetSource(), m.GetSeriesExternalIds()["tvdb"], m.GetTitle(), m.GetSearchTitle())
		if m.HasSeasonNumber() {
			s += fmt.Sprintf(" season %d", m.GetSeasonNumber())
		}
		if m.HasSceneSeasonNumber() {
			s += fmt.Sprintf(" scene season %d", m.GetSceneSeasonNumber())
		}
		if m.GetSearchMode() != pluginv1.SceneSearchMode_SCENE_SEARCH_MODE_UNSPECIFIED {
			s += " " + m.GetSearchMode().String()
		}
		for _, extra := range []string{m.GetSceneOrigin(), m.GetFilterRegex(), m.GetComment()} {
			if extra != "" {
				s += " " + extra
			}
		}
		out = append(out, s)
	}
	return out
}

func TestListSceneMappings(t *testing.T) {
	services := []string{
		`services 152831 "Adventure Time With Finn and Jake"/"Adventure Time" season -1`,
		`services 262148 "Mayday"/"Mayday" season -1 (?i)Mayday\.S01E`,
		`services 1 "Show S2"/"Show" season 2 scene season 1 SCENE_SEARCH_MODE_TITLE tvdb renamed`,
	}
	xem := []string{
		`xem 262980 "Shingeki no Kyojin"/"Shingeki no Kyojin" scene season 1`,
		`xem 262980 "AoT Season 2"/"AoT Season 2" scene season 2`,
		`xem 79151 "Fate/Zero"/"Fate/Zero" scene season 1`,
	}
	tests := []struct {
		name    string
		routes  map[string]string
		want    []string
		wantErr bool
	}{
		{"both sources", map[string]string{"GET /services/scenemapping": sceneMappings, "GET /xem/allNames": xemNames}, slices.Concat(services, xem), false},
		{"XEM fails", map[string]string{"GET /services/scenemapping": sceneMappings, "GET /xem/allNames": "!down"}, services, false},
		{"XEM reports a failure", map[string]string{"GET /xem/allNames": `{"result": "failure", "message": "broken", "data": []}`, "GET /services/scenemapping": sceneMappings}, services, false},
		{"Sonarr's services fail", map[string]string{"GET /services/scenemapping": "!down", "GET /xem/allNames": xemNames}, xem, false},
		{"both fail", map[string]string{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPlugin(t, tt.routes)
			resp, err := p.ListSceneMappings(t.Context(), &pluginv1.ListSceneMappingsRequest{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ListSceneMappings() error = %v, want error %v", err, tt.wantErr)
			}
			// XEM's data is a map: its series are listed in the order of their IDs as strings.
			got := describe(resp.GetMappings())
			if !slices.Equal(got, tt.want) {
				t.Errorf("mappings =\n%q\nwant =\n%q", got, tt.want)
			}
			for _, r := range f.requests {
				if r.path == "/xem/allNames" && r.query != "origin=tvdb&seasonNumbers=True" {
					t.Errorf("XEM query = %q", r.query)
				}
			}
		})
	}
}

func TestListDailySeries(t *testing.T) {
	p, _ := newTestPlugin(t, map[string]string{"GET /services/dailyseries": `[{"tvdbId": 70336}, {"tvdbId": 0}, {"tvdbId": 71256}]`})
	resp, err := p.ListDailySeries(t.Context(), &pluginv1.ListDailySeriesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range resp.GetSeries() {
		ids = append(ids, s.GetExternalIds()["tvdb"])
	}
	if want := []string{"70336", "71256"}; !slices.Equal(ids, want) {
		t.Errorf("daily series = %v, want = %v", ids, want)
	}
}

func TestGetSceneNumbering(t *testing.T) {
	all := `{"result": "success", "message": "", "data": [
		{"scene": {"season": 1, "episode": 1, "absolute": 1}, "tvdb": {"season": 1, "episode": 1, "absolute": 1}},
		{"scene": {"season": 2, "episode": 1, "absolute": 14}, "tvdb": {"season": 1, "episode": 14, "absolute": 14}},
		{"scene": {"season": 0, "episode": 0, "absolute": 0}, "tvdb": {"season": 1, "episode": 15, "absolute": 15}},
		{"tvdb": {"season": 1, "episode": 16, "absolute": 16}}]}`
	tests := []struct {
		name      string
		routes    map[string]string
		ids       map[string]string
		want      []string
		wantPaths []string
	}{
		{
			name:      "mapped",
			routes:    map[string]string{"GET /xem/havemap": `{"result": "success", "data": ["262980", 79151]}`, "GET /xem/all": all},
			ids:       map[string]string{"tvdb": "262980"},
			want:      []string{"1x1 #1 -> 1x1 #1", "1x14 #14 -> 2x1 #14"},
			wantPaths: []string{"GET /xem/havemap", "GET /xem/all"},
		},
		{
			name:      "not on XEM",
			routes:    map[string]string{"GET /xem/havemap": `{"result": "success", "data": ["79151"]}`, "GET /xem/all": all},
			ids:       map[string]string{"tvdb": "262980"},
			wantPaths: []string{"GET /xem/havemap"},
		},
		{
			name:      "XEM's list unavailable",
			routes:    map[string]string{"GET /xem/havemap": "!down", "GET /xem/all": all},
			ids:       map[string]string{"tvdb": "262980"},
			want:      []string{"1x1 #1 -> 1x1 #1", "1x14 #14 -> 2x1 #14"},
			wantPaths: []string{"GET /xem/havemap", "GET /xem/all"},
		},
		{
			name: "no show",
			routes: map[string]string{
				"GET /xem/havemap": `{"result": "success", "data": ["262980"]}`,
				"GET /xem/all":     `{"result": "failure", "message": "no show with the tvdb_id 262980 found", "data": {}}`,
			},
			ids:       map[string]string{"tvdb": "262980"},
			wantPaths: []string{"GET /xem/havemap", "GET /xem/all"},
		},
		{name: "no TVDB ID", ids: map[string]string{"tmdb": "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, f := newTestPlugin(t, tt.routes)
			resp, err := p.GetSceneNumbering(t.Context(), pluginv1.GetSceneNumberingRequest_builder{SeriesExternalIds: tt.ids}.Build())
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, e := range resp.GetEpisodes() {
				got = append(got, fmt.Sprintf("%dx%d #%d -> %dx%d #%d", e.GetSeasonNumber(), e.GetEpisodeNumber(), e.GetAbsoluteNumber(),
					e.GetSceneSeasonNumber(), e.GetSceneEpisodeNumber(), e.GetSceneAbsoluteNumber()))
			}
			if !slices.Equal(got, tt.want) || resp.GetFound() != (tt.want != nil) {
				t.Errorf("GetSceneNumbering() = %v %q, want = %q", resp.GetFound(), got, tt.want)
			}
			if resp.GetFound() && resp.GetProvider() != "tvdb" {
				t.Errorf("provider = %q, want = tvdb", resp.GetProvider())
			}
			if !slices.Equal(f.paths(), tt.wantPaths) {
				t.Errorf("requests = %v, want = %v", f.paths(), tt.wantPaths)
			}
		})
	}

	// XEM's list is kept for three hours.
	p, f := newTestPlugin(t, map[string]string{"GET /xem/havemap": `{"result": "success", "data": ["1"]}`})
	req := pluginv1.GetSceneNumberingRequest_builder{SeriesExternalIds: map[string]string{"tvdb": "2"}}.Build()
	for range 2 {
		if resp, err := p.GetSceneNumbering(t.Context(), req); err != nil || resp.GetFound() {
			t.Fatalf("GetSceneNumbering() = %v, %v", resp, err)
		}
	}
	if n := len(f.paths()); n != 1 {
		t.Errorf("requests = %v, want XEM's list once", f.paths())
	}
}

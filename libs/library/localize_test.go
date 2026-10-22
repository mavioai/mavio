package library

import (
	"context"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// langProvider describes Spirited Away in the languages it has.
type langProvider struct {
	names, overviews map[string]string
	languages        []string
}

func (p *langProvider) Name() string { return "lang" }

func (p *langProvider) Metadata(_ context.Context, l Lookup) (*metadata.Result, error) {
	p.languages = append(p.languages, l.Language)
	return &metadata.Result{Item: core.Item{Name: p.names[l.Language], Overview: p.overviews[l.Language]}}, nil
}

func TestLocalize(t *testing.T) {
	ctx := t.Context()
	m := newMemStore()
	lib := core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies}
	ja := core.Library{ID: core.NewID(), Name: "Anime", Kind: core.LibraryMovies, PreferredLanguage: "ja"}
	for _, l := range []*core.Library{&lib, &ja} {
		if err := m.Libraries().Create(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	movie := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindMovie, Name: "spirited away", DateAdded: time.Now()}
	fixed := core.Item{ID: core.NewID(), LibraryID: ja.ID, Kind: core.KindMovie, Name: "spirited away", DateAdded: time.Now()}
	if err := m.Items().Upsert(ctx, movie, fixed); err != nil {
		t.Fatal(err)
	}
	p := &langProvider{
		names:     map[string]string{"en": "Spirited Away", "zh": "千与千寻", "ja": "千と千尋の神隠し"},
		overviews: map[string]string{"en": "A girl enters the spirit world.", "zh": "少女千寻误入神灵世界。"},
	}
	r := &Refresher{Store: m, Providers: []Provider{p}, Metadata: func() core.MetadataSettings { return core.MetadataSettings{} }}

	it, err := r.RefreshWith(ctx, lib, movie.ID, RefreshOptions{})
	if err != nil || it.Name != "Spirited Away" || p.languages[0] != "en" {
		t.Fatalf("refresh following clients = %q in %v, %v, want English", it.Name, p.languages, err)
	}
	if err := r.Localize(ctx, lib, movie.ID, "en"); err != nil || len(p.languages) != 1 {
		t.Errorf("Localize in the language kept asked the providers in %v, %v", p.languages, err)
	}
	if err := r.Localize(ctx, lib, movie.ID, "zh"); err != nil {
		t.Fatal(err)
	}
	l, err := m.Localizations().Get(ctx, movie.ID, "zh")
	if err != nil || l.Name != "千与千寻" || l.Overview != "少女千寻误入神灵世界。" {
		t.Errorf("zh = %+v, %v", l, err)
	}

	// Locked fields keep what the localization has; a refresh fetches the
	// languages again.
	it.LockedFields = []core.MetadataField{core.FieldName}
	if err := m.Items().Upsert(ctx, it); err != nil {
		t.Fatal(err)
	}
	p.names["zh"], p.overviews["zh"] = "神隐少女", "十岁的千寻。"
	p.languages = nil
	if _, err := r.RefreshWith(ctx, lib, movie.ID, RefreshOptions{}); err != nil {
		t.Fatal(err)
	}
	if l, err = m.Localizations().Get(ctx, movie.ID, "zh"); err != nil || l.Name != "千与千寻" || l.Overview != "十岁的千寻。" {
		t.Errorf("zh after a refresh = %+v, %v, want the locked name kept and the overview fetched again", l, err)
	}
	if len(p.languages) != 2 || p.languages[1] != "zh" {
		t.Errorf("a refresh asked in %v, want [en zh]", p.languages)
	}

	// An item in a library with a language of its own does not follow.
	p.languages = nil
	if err := r.Localize(ctx, ja, fixed.ID, "zh"); err != nil || len(p.languages) != 0 {
		t.Errorf("Localize of an item kept in Japanese asked in %v, %v", p.languages, err)
	}

	// The job does what Localize does; the locked name is taken in no
	// language, the item's showing.
	j := &Jobs{Store: m, Refresher: r}
	if _, err := j.localize(ctx, LocalizeJob(movie.ID, "ja", time.Now())); err != nil {
		t.Fatal(err)
	}
	if l, err := m.Localizations().Get(ctx, movie.ID, "ja"); err != nil || l.Name != "" || l.Overview != "" {
		t.Errorf("ja = %+v, %v", l, err)
	}
}

func TestMetadataLanguage(t *testing.T) {
	lib := core.Library{}
	series := &core.Item{Kind: core.KindSeries, MetadataLanguage: "ja"}
	tests := []struct {
		name        string
		it, series  *core.Item
		lib         core.Library
		server      string
		want        string
		wantFollows bool
	}{
		{"nothing set follows clients", &core.Item{}, nil, lib, "", "en", true},
		{"the server's", &core.Item{}, nil, lib, "de", "de", false},
		{"the library's over the server's", &core.Item{}, nil, core.Library{PreferredLanguage: "fr"}, "de", "fr", false},
		{"the series' over the library's", &core.Item{Kind: core.KindEpisode}, series, core.Library{PreferredLanguage: "fr"}, "", "ja", false},
		{"the item's over the series'", &core.Item{Kind: core.KindEpisode, MetadataLanguage: "ko"}, series, lib, "", "ko", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, follows := MetadataLanguage(tt.it, tt.series, tt.lib, core.MetadataSettings{PreferredLanguage: tt.server})
			if got != tt.want || follows != tt.wantFollows {
				t.Errorf("MetadataLanguage = %q, %v, want = %q, %v", got, follows, tt.want, tt.wantFollows)
			}
		})
	}
}

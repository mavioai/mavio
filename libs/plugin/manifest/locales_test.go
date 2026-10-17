package manifest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/text/language"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestLoadLocales(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		want    Locales
		wantErr bool
	}{
		{name: "none", want: nil},
		{
			name:  "languages",
			files: map[string]string{"zh-Hans.json": `{"Movies":"电影"}`, "de.json": `{"Movies":"Filme"}`, "notes.txt": "x"},
			want:  Locales{"zh-Hans": {"Movies": "电影"}, "de": {"Movies": "Filme"}},
		},
		{name: "not canonical", files: map[string]string{"zh-hans.json": `{}`}, wantErr: true},
		{name: "not a tag", files: map[string]string{"chinese!.json": `{}`}, wantErr: true},
		{name: "english", files: map[string]string{"en.json": `{}`}, wantErr: true},
		{name: "not strings", files: map[string]string{"de.json": `{"Movies":1}`}, wantErr: true},
		{name: "empty translation", files: map[string]string{"de.json": `{"Movies":""}`}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.files != nil {
				if err := os.Mkdir(filepath.Join(dir, LocalesDir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for name, data := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, LocalesDir, name), []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := LoadLocales(dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadLocales() err = %v, want error = %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Fatalf("LoadLocales() = %v, want = %v", got, tt.want)
			}
			for tag, strs := range tt.want {
				if got[tag]["Movies"] != strs["Movies"] {
					t.Errorf("LoadLocales()[%s] = %v, want = %v", tag, got[tag], strs)
				}
			}
		})
	}
}

func TestTranslator(t *testing.T) {
	l := Locales{"zh-Hans": {"Movies": "电影"}, "de": {"Movies": "Filme"}}
	tests := []struct {
		wanted string
		want   string
	}{
		{"zh-CN", "电影"},
		{"zh-Hans", "电影"},
		{"zh-CN,en;q=0.8", "电影"},
		{"de-AT", "Filme"},
		{"en-US,zh-CN;q=0.5", "Movies"},
		{"fr", "Movies"},
		{"", "Movies"},
	}
	for _, tt := range tests {
		wanted, _, err := language.ParseAcceptLanguage(tt.wanted)
		if err != nil {
			t.Fatal(err)
		}
		tr := l.Translator(wanted)
		if got := tr("Movies"); got != tt.want {
			t.Errorf("Translator(%q)(Movies) = %q, want = %q", tt.wanted, got, tt.want)
		}
		if got := tr("Series"); got != "Series" {
			t.Errorf("Translator(%q)(Series) = %q, want = Series", tt.wanted, got)
		}
	}
}

func TestLocalize(t *testing.T) {
	m := valid()
	m.SetDescription("Metadata from TMDB.")
	withTasks(m, task("refresh", 0, 0))
	m.GetTasks()[0].SetDescription("Refreshes <everything> & more.")
	m.SetExternalIdKinds([]*pluginv1.ExternalIdKind{pluginv1.ExternalIdKind_builder{Key: proto.String("trakt"), Name: proto.String("Trakt")}.Build()})
	m.SetContainers([]*pluginv1.ContainerDependency{pluginv1.ContainerDependency_builder{Name: proto.String("solver"), Description: proto.String("Solves challenges")}.Build()})
	// The schema's members keep their order; data named like keywords,
	// such as a default's description or a property named title, does not
	// change.
	m.SetConfigSchema(`{"type":"object","properties":{"zeta":{"type":"string","title":"Zeta","description":"Last <one>"},` +
		`"title":{"type":"object","default":{"description":"Zeta"},"properties":{"x":{"type":"number","minimum":1.5,"title":"Zeta"}}},` +
		`"list":{"type":"array","items":{"title":"Zeta","enum":["Zeta",null,true]}},"either":{"anyOf":[{"title":"Zeta"}]}},` +
		`"description":"Zeta"}`)
	tr := map[string]string{
		"TMDB": "TMDB", "Metadata from TMDB.": "来自 TMDB 的元数据。", "Task refresh": "刷新", "Refreshes <everything> & more.": "刷新<全部>",
		"Trakt": "特拉克特", "Solves challenges": "求解质询", "Zeta": "泽塔", "Last <one>": "最后<一个>",
	}
	got := Localize(m, func(s string) string {
		if t, ok := tr[s]; ok {
			return t
		}
		return s
	})
	for _, c := range []struct{ name, got, want string }{
		{"description", got.GetDescription(), "来自 TMDB 的元数据。"},
		{"task name", got.GetTasks()[0].GetName(), "刷新"},
		{"task description", got.GetTasks()[0].GetDescription(), "刷新<全部>"},
		{"external ID kind", got.GetExternalIdKinds()[0].GetName(), "特拉克特"},
		{"container", got.GetContainers()[0].GetDescription(), "求解质询"},
		{"schema", got.GetConfigSchema(), `{"type":"object","properties":{"zeta":{"type":"string","title":"泽塔","description":"最后<一个>"},` +
			`"title":{"type":"object","default":{"description":"Zeta"},"properties":{"x":{"type":"number","minimum":1.5,"title":"泽塔"}}},` +
			`"list":{"type":"array","items":{"title":"泽塔","enum":["Zeta",null,true]}},"either":{"anyOf":[{"title":"泽塔"}]}},` +
			`"description":"泽塔"}`},
		{"original", m.GetDescription(), "Metadata from TMDB."},
	} {
		if c.got != c.want {
			t.Errorf("%s = %s, want = %s", c.name, c.got, c.want)
		}
	}

	want := []string{"TMDB", "Metadata from TMDB.", "Task refresh", "Refreshes <everything> & more.", "Trakt", "Solves challenges", "Zeta", "Last <one>"}
	if got := Strings(m); !slices.Equal(got, want) {
		t.Errorf("Strings() = %q, want = %q", got, want)
	}

	m.SetConfigSchema(`{"title":"Zeta"`)
	if got := Localize(m, func(string) string { return "x" }).GetConfigSchema(); got != `{"title":"Zeta"` {
		t.Errorf("Localize() of an unreadable schema = %s, want it unchanged", got)
	}
}

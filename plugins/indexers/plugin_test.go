package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	p, err := newPlugin(slog.New(slog.DiscardHandler), "test", t.TempDir(), "macos 15.0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.halt)
	return p
}

func configure(t *testing.T, p *plugin, c any) {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Configure(context.Background(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(string(data))}.Build()); err != nil {
		t.Fatal(err)
	}
}

func TestCodeDefinitions(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range codeDefinitions() {
		if d.ID == "" || d.Name == "" || d.New == nil {
			t.Errorf("definition %q: ID, Name and New are required", d.ID)
		}
		if seen[d.ID] {
			t.Errorf("definition ID %q is repeated", d.ID)
		}
		seen[d.ID] = true
	}
	for _, id := range []string{"newznab", "torznab", "torrentrss", "nzbgeek", "nzb-life", "tabula-rasa", "animetosho-usenet", "animetosho", "torrent-network", "iptorrents", "bithdtv", "hdtorrents", "immortalseed", "libble", "norbits", "pixelhd", "nzbindex", "torrentscsv", "knaben", "torrentpotato", "headphones", "rutracker", "beyondhd", "mteamtp", "nebulance", "torrentday", "myanonamouse", "shazbat", "pretome", "revolutiontt", "scenehd", "speedcd", "torrentbytes", "torrentsyndikat", "uniotaku", "xspeeds", "animebytes", "anidex", "anidub", "bakabt", "subsplease", "toloka"} {
		if !seen[id] {
			t.Errorf("definition %q is missing", id)
		}
	}
}

func TestSlug(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"AnimeTosho (Usenet)", "animetosho-usenet"},
		{"Nzb.life", "nzb-life"},
		{"Tabula Rasa", "tabula-rasa"},
		{"NZBgeek", "nzbgeek"},
	} {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

func TestNewznabPresetSettings(t *testing.T) {
	var tabula preset
	for _, p := range newznabPresets {
		if p.name == "Tabula Rasa" {
			tabula = p
		}
	}
	tests := []struct {
		data string
		want string
	}{
		{`{"api_key":"k"}`, "https://www.tabula-rasa.pw /api/v1/api k"},
		{`{"base_url":"https://mirror.example","api_path":"/x"}`, "https://mirror.example /x "},
	}
	for _, tt := range tests {
		s, err := newznabSettings([]byte(tt.data), tabula)
		if err != nil {
			t.Fatal(err)
		}
		if got := s.BaseURL + " " + s.APIPath + " " + s.APIKey; got != tt.want {
			t.Errorf("newznabSettings(%s) = %q, want = %q", tt.data, got, tt.want)
		}
	}
}

func TestQuery(t *testing.T) {
	req := pluginv1.SearchReleasesRequest_builder{
		Type: pluginv1.IndexerSearchType_INDEXER_SEARCH_TYPE_TV.Enum(), Query: proto.String("Show"),
		ExternalIds: map[string]string{"imdb": "tt0944947", "tvdb": "121361"},
		Season:      proto.Int32(0), Episode: proto.String("3"),
		Title: proto.String("The Show"), MinAge: proto.Int32(0), MaxAge: proto.Int32(30),
		MaxSize: proto.Int64(5000),
	}.Build()
	q, err := query(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Type", q.Type, indexer.SearchTV},
		{"IMDbID", q.IMDbID, "0944947"},
		{"TVDBID", q.TVDBID, 121361},
		{"Season", q.Season != nil && *q.Season == 0, true},
		{"Episode", q.Episode, "3"},
		{"Categories", q.Categories, []int{5000}},
		{"Title", q.Title, "The Show"},
		{"MinAge", q.MinAge, indexer.Ptr(0)},
		{"MaxAge", q.MaxAge, indexer.Ptr(30)},
		{"MinSize", q.MinSize, (*int64)(nil)},
		{"MaxSize", q.MaxSize, indexer.Ptr[int64](5000)},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if _, err := query(pluginv1.SearchReleasesRequest_builder{ExternalIds: map[string]string{"tmdb": "x"}}.Build()); err == nil {
		t.Errorf("query(tmdb x) = nil, want = an error")
	}
	if q, _ := query(&pluginv1.SearchReleasesRequest{}); q.Season != nil || q.Categories != nil {
		t.Errorf("query({}) = %+v, want = no season and no categories", q)
	}
}

func TestReleaseProtoMediaDetails(t *testing.T) {
	r := releaseProto("btn", &indexer.Release{
		Title: "Show.S01.1080p.BluRay", Origin: "Internal", Source: "Bluray", Container: "M2TS", Codec: "H.264", Resolution: "1080p",
	})
	for _, c := range []struct{ name, got, want string }{
		{"Origin", r.GetOrigin(), "Internal"},
		{"Source", r.GetSource(), "Bluray"},
		{"Container", r.GetContainer(), "M2TS"},
		{"Codec", r.GetCodec(), "H.264"},
		{"Resolution", r.GetResolution(), "1080p"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want = %q", c.name, c.got, c.want)
		}
	}
}

func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "responses", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// torznabServer serves AnimeTosho's recorded caps and search, and a
// torrent file.
func torznabServer(t *testing.T) *httptest.Server {
	caps := readFile(t, "Torznab/torznab_animetosho_caps.xml")
	feed := readFile(t, "Torznab/torznab_animetosho.xml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/file.torrent":
			w.Header().Set("Content-Type", "application/x-bittorrent")
			_, _ = io.WriteString(w, "d8:announce0:e")
		case r.URL.Query().Get("t") == "caps":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(caps)
		default:
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write(feed)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTorznabIndexer(t *testing.T) {
	srv := torznabServer(t)
	p := newTestPlugin(t)
	configure(t, p, map[string]any{
		"indexers": []any{
			map[string]any{
				"id": "tosho", "definition": "torznab", "settings": map[string]any{"base_url": srv.URL},
				"tags": []any{"anime"}, "required_flags": []any{"freeleech"}, "minimum_seeders": 3, "multi_languages": []any{"French", "Original"},
				"seed_ratio": 1.5, "seed_time": 60, "season_pack_seed_time": 120, "reject_blocklisted_torrent_hashes": true, "anime_standard_format_search": true,
			},
			map[string]any{"id": "off", "definition": "torznab", "enabled": false, "settings": map[string]any{"base_url": srv.URL}},
			map[string]any{"id": "broken", "definition": "nosuchdefinition"},
		},
		"definitions": map[string]any{"update": false},
	})
	ctx := context.Background()

	list, err := p.ListIndexers(ctx, &pluginv1.ListIndexersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(list.GetIndexers()); got != 2 {
		t.Fatalf("indexers = %d, want = 2", got)
	}
	tosho := list.GetIndexers()[0]
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Id", tosho.GetId(), "tosho"},
		{"Name", tosho.GetName(), "Generic Torznab"},
		{"Protocol", tosho.GetProtocol(), pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT},
		{"Priority", tosho.GetPriority(), int32(25)},
		{"Enabled", tosho.GetEnabled(), true},
		{"Failure", tosho.GetFailure(), ""},
		{"has categories", len(tosho.GetCapabilities().GetCategories()) > 0, true},
		{"off Enabled", list.GetIndexers()[1].GetEnabled(), false},
		{"Tags", tosho.GetTags(), []string{"anime"}},
		{"RequiredFlags", tosho.GetRequiredFlags(), []string{"freeleech"}},
		{"MinimumSeeders", tosho.GetMinimumSeeders(), int32(3)},
		{"MultiLanguages", tosho.GetMultiLanguages(), []string{"French", "Original"}},
		{"SeedRatio", tosho.GetSeedRatio(), 1.5},
		{"SeedTime", tosho.GetSeedTime().AsDuration(), time.Hour},
		{"SeasonPackSeedTime", tosho.GetSeasonPackSeedTime().AsDuration(), 2 * time.Hour},
		{"AnimeStandardFormatSearch", tosho.GetAnimeStandardFormatSearch(), true},
		{"off AnimeStandardFormatSearch", list.GetIndexers()[1].GetAnimeStandardFormatSearch(), false},
		{"off MinimumSeeders", list.GetIndexers()[1].GetMinimumSeeders(), int32(1)},
		{"off HasSeedRatio", list.GetIndexers()[1].HasSeedRatio(), false},
		{"off HasSeedTime", list.GetIndexers()[1].HasSeedTime(), false},
		{"RejectBlocklistedTorrentHashes", tosho.GetRejectBlocklistedTorrentHashes(), true},
		{"off RejectBlocklistedTorrentHashes", list.GetIndexers()[1].GetRejectBlocklistedTorrentHashes(), false},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}

	res, err := p.SearchReleases(ctx, &pluginv1.SearchReleasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GetFailures()) != 0 {
		t.Fatalf("failures = %v, want = none", res.GetFailures())
	}
	if len(res.GetReleases()) == 0 {
		t.Fatal("releases = 0, want = some")
	}
	for _, r := range res.GetReleases() {
		if r.GetIndexerId() != "tosho" {
			t.Errorf("release of %q, want = only tosho's, the other being disabled", r.GetIndexerId())
		}
		if r.GetTitle() == "" || r.GetDownloadUrl() == "" && r.GetMagnetUrl() == "" {
			t.Errorf("release %v lacks a title or a link", r)
		}
	}

	file, err := p.GetReleaseFile(ctx, pluginv1.GetReleaseFileRequest_builder{IndexerId: proto.String("tosho"), DownloadUrl: proto.String(srv.URL + "/file.torrent")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if string(file.GetData()) != "d8:announce0:e" || file.GetContentType() != "application/x-bittorrent" {
		t.Errorf("file = %q (%s), want = the torrent", file.GetData(), file.GetContentType())
	}

	test, err := p.TestIndexer(ctx, pluginv1.TestIndexerRequest_builder{DefinitionId: proto.String("torznab"), SettingsJson: proto.String(`{"base_url":"` + srv.URL + `"}`)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if test.GetFailure() != "" {
		t.Errorf("test failure = %q, want = none", test.GetFailure())
	}
	test, err = p.TestIndexer(ctx, pluginv1.TestIndexerRequest_builder{DefinitionId: proto.String("torznab"), SettingsJson: proto.String(`{"base_url":"ftp://x"}`)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if test.GetFailure() == "" {
		t.Errorf("test failure = none, want = invalid settings")
	}
}

func TestConfigureInvalid(t *testing.T) {
	p := newTestPlugin(t)
	for _, c := range []string{
		`{"indexers":[{"definition":"torznab"}]}`,
		`{"indexers":[{"id":"a","definition":"torznab"},{"id":"a","definition":"torznab"}]}`,
		`{"indexers":[{"id":"a","definition":"torznab","limits_unit":"week"}]}`,
		`{"indexers":[{"id":"a","definition":"torznab","required_flags":["shiny"]}]}`,
		`{"indexers":[{"id":"a","definition":"torznab","minimum_seeders":-1}]}`,
		`{"indexers":[{"id":"a","definition":"torznab","seed_time":-1}]}`,
		`{"proxies":[{"kind":"gopher","host":"x"}]}`,
	} {
		if _, err := p.Configure(context.Background(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(c)}.Build()); err == nil {
			t.Errorf("Configure(%s) = nil, want = an error", c)
		}
	}
}

// publicDefinition is a Cardigann definition of a public site at the
// address %s.
const publicDefinition = `
id: publicsite
name: Public Site
description: A public site
language: en-US
type: public
links: ["%s/"]
caps:
  categorymappings:
    - {id: 1, cat: Movies, desc: "Movies"}
  modes:
    search: [q]
settings:
  - {name: sort, type: select, label: Sort, default: seeders, options: {seeders: Seeders, added: Added}}
  - {name: info_cookie, type: info_cookie}
search:
  paths:
    - path: browse
  inputs:
    q: "{{ .Keywords }}"
    sort: "{{ .Config.sort }}"
  rows:
    selector: tr.row
  fields:
    category:
      text: 1
    title:
      selector: a.title
    details:
      selector: a.title
      attribute: href
    download:
      selector: a.dl
      attribute: href
    size:
      text: 1 GB
    seeders:
      text: 10
    leechers:
      text: 2
    date:
      text: now
`

func definitionsPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCardigannDefinitions(t *testing.T) {
	var agent string
	var sorts []string
	var pkg []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/master/11/package.zip":
			agent = r.Header.Get("User-Agent")
			_, _ = w.Write(pkg)
		case "/browse":
			sorts = append(sorts, r.URL.Query().Get("sort"))
			_, _ = io.WriteString(w, `<html><body><table><tr class="row"><td><a class="title" href="/t/1">A Film 2020 1080p</a><a class="dl" href="/d/1.torrent">DL</a></td></tr></table></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	pkg = definitionsPackage(t, map[string]string{
		"publicsite.yml": strings.ReplaceAll(publicDefinition, "%s", srv.URL),
		// Blocked as Prowlarr implements it in code.
		"hdbits.yml":    strings.ReplaceAll(publicDefinition, "Public Site", "HDBits YAML"),
		"../escape.yml": "id: x",
	})

	p := newTestPlugin(t)
	configure(t, p, map[string]any{
		"indexers":    []any{map[string]any{"id": "pub", "definition": "publicsite", "settings": map[string]any{"sort": "added"}}},
		"definitions": map[string]any{"url": srv.URL + "/master/11"},
	})
	ctx := context.Background()
	var def *pluginv1.IndexerDefinition
	for deadline := time.Now().Add(10 * time.Second); def == nil && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		res, err := p.ListDefinitions(ctx, &pluginv1.ListDefinitionsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range res.GetDefinitions() {
			if d.GetId() == "publicsite" {
				def = d
			}
			if d.GetName() == "HDBits YAML" {
				t.Fatal("the hdbits definition is listed, want = blocked")
			}
		}
	}
	if def == nil {
		t.Fatal("definition publicsite not listed after the update")
	}
	if want := "Prowlarr/2.6.5.5620 (macos 15.0)"; agent != want {
		t.Errorf("User-Agent = %q, want = %q", agent, want)
	}
	var names []string
	for _, f := range def.GetFields() {
		names = append(names, f.GetName())
	}
	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"Privacy", def.GetPrivacy(), pluginv1.IndexerPrivacy_INDEXER_PRIVACY_PUBLIC},
		{"fields", names, []string{"base_url", "sort", "info_cookie"}},
		{"sort type", def.GetFields()[1].GetType(), pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT},
		{"sort default", def.GetFields()[1].GetDefaultJson(), `"seeders"`},
		{"sort options", len(def.GetFields()[1].GetOptions()), 2},
		{"info type", def.GetFields()[2].GetType(), pluginv1.SettingFieldType_SETTING_FIELD_TYPE_INFO},
	} {
		if !equal(c.got, c.want) {
			t.Errorf("%s = %v, want = %v", c.name, c.got, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(p.store.dir, "..", "escape.yml")); err == nil {
		t.Error("a file of the package escaped the definitions' directory")
	}

	res, err := p.SearchReleases(ctx, pluginv1.SearchReleasesRequest_builder{Query: proto.String("film")}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GetFailures()) != 0 || len(res.GetReleases()) != 1 {
		t.Fatalf("releases = %v, failures = %v, want = one release", res.GetReleases(), res.GetFailures())
	}
	r := res.GetReleases()[0]
	if r.GetTitle() != "A Film 2020 1080p" || r.GetDownloadUrl() != srv.URL+"/d/1.torrent" {
		t.Errorf("release = %v, want = A Film 2020 1080p at /d/1.torrent", r)
	}
	if !slices.Equal(sorts, []string{"added"}) {
		t.Errorf("sorts = %v, want = [added]", sorts)
	}
}

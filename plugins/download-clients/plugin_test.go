package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// fakeClient is a client whose answers the test sets.
type fakeClient struct {
	items     []client.Item
	err       error
	added     *client.Release
	removed   []string
	imported  []string
	noMagnets bool
}

func (c *fakeClient) Add(_ context.Context, r *client.Release) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	if c.noMagnets && len(r.Data) == 0 {
		return "", client.ErrMagnetNotSupported
	}
	c.added = r
	return "ID", nil
}

func (c *fakeClient) Items(context.Context, client.Options) ([]client.Item, error) {
	return c.items, c.err
}

func (c *fakeClient) Remove(_ context.Context, item client.Item, _ bool) error {
	c.removed = append(c.removed, item.ID)
	return nil
}

func (c *fakeClient) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: true, OutputRootFolders: []string{"/downloads"}}, nil
}

func (c *fakeClient) Test(context.Context) error { return c.err }

func (c *fakeClient) Host() string { return "localhost" }

func (c *fakeClient) MarkImported(_ context.Context, item client.Item) error {
	c.imported = append(c.imported, item.ID)
	return nil
}

func (c *fakeClient) ImportItem(_ context.Context, item client.Item, _ client.Options) (client.Item, error) {
	item.OutputPath = "/imported/" + item.Title
	return item, nil
}

// newTestPlugin returns the plugin with a fake definition whose clients
// are those of clients, by their "name" setting.
func newTestPlugin(t *testing.T, clients map[string]*fakeClient) *plugin {
	t.Helper()
	p, err := newPlugin(slog.New(slog.DiscardHandler), "test", t.TempDir(), http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	def := client.Definition{
		ID: "fake", Name: "Fake", Protocol: client.Torrent, PrefersTorrentFile: true,
		Fields: []client.Field{
			{Name: "name", Label: "Name", Type: client.FieldText},
			{Name: "priority", Label: "Priority", Type: client.FieldSelect, Default: 0, Options: []client.Option{{Value: 0, Label: "Last"}}},
		},
		New: func(raw []byte, _ client.Env) (client.Client, error) {
			var s struct{ Name string }
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, err
			}
			c, ok := clients[s.Name]
			if !ok {
				return nil, errors.New("unknown fake")
			}
			return c, nil
		},
	}
	p.defs = append(p.defs, def)
	p.byID[def.ID] = def
	return p
}

func configure(t *testing.T, p *plugin, names ...string) {
	t.Helper()
	var c config
	for _, n := range names {
		c.Clients = append(c.Clients, clientConfig{ID: n, Definition: "fake", Settings: json.RawMessage(`{"name":"` + n + `"}`)})
	}
	data, _ := json.Marshal(c)
	if _, err := p.Configure(context.Background(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(string(data))}.Build()); err != nil {
		t.Fatal(err)
	}
}

func TestDefinitions(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range definitions() {
		if d.ID == "" || d.Name == "" || d.New == nil || d.Protocol == 0 {
			t.Errorf("definition %q: ID, Name, Protocol and New are required", d.ID)
		}
		if seen[d.ID] {
			t.Errorf("definition ID %q is repeated", d.ID)
		}
		seen[d.ID] = true
		names := map[string]bool{}
		for _, f := range d.Fields {
			if names[f.Name] {
				t.Errorf("definition %q: field %q is repeated", d.ID, f.Name)
			}
			names[f.Name] = true
		}
		// The defaults name settings the client knows; clients whose
		// required settings have no default refuse them, but not as
		// unknown.
		defaults := map[string]any{}
		for _, f := range d.Fields {
			if f.Default != nil {
				defaults[f.Name] = f.Default
			}
		}
		data, err := json.Marshal(defaults)
		if err != nil {
			t.Fatalf("definition %q: defaults: %v", d.ID, err)
		}
		if _, err := d.New(data, client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)}); err != nil && strings.Contains(err.Error(), "unknown field") {
			t.Errorf("definition %q: New() with the defaults error = %v", d.ID, err)
		}
	}
}

func TestListDefinitionsAndClients(t *testing.T) {
	p := newTestPlugin(t, map[string]*fakeClient{"a": {}})
	configure(t, p, "a")
	defs, err := p.ListDownloadClientDefinitions(context.Background(), &pluginv1.ListDownloadClientDefinitionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var fake *pluginv1.DownloadClientDefinition
	for _, d := range defs.GetDefinitions() {
		if d.GetId() == "fake" {
			fake = d
		}
	}
	if fake == nil || len(fake.GetFields()) != 2 || fake.GetFields()[1].GetDefaultJson() != "0" ||
		fake.GetFields()[1].GetOptions()[0].GetLabel() != "Last" {
		t.Fatalf("fake definition = %v, want its two fields", fake)
	}
	list, err := p.ListDownloadClients(context.Background(), &pluginv1.ListDownloadClientsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetClients()) != 1 {
		t.Fatalf("clients = %v, want one", list.GetClients())
	}
	c := list.GetClients()[0]
	if c.GetId() != "a" || c.GetName() != "Fake" || !c.GetEnabled() || c.GetPriority() != 1 || !c.GetPrefersTorrentFile() ||
		c.GetHost() != "localhost" || !c.GetRemoveCompletedDownloads() || c.HasDisabledUntil() {
		t.Errorf("client = %v, want a's defaults", c)
	}
}

func TestAddDownload(t *testing.T) {
	a := &fakeClient{noMagnets: true}
	p := newTestPlugin(t, map[string]*fakeClient{"a": a})
	configure(t, p, "a")
	ctx := context.Background()

	_, err := p.AddDownload(ctx, pluginv1.AddDownloadRequest_builder{
		ClientId: proto.String("a"), Title: proto.String("T"),
		MagnetUrl: proto.String("magnet:?xt=urn:btih:CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951"),
	}.Build())
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("AddDownload(magnet) code = %v, want = %v", connect.CodeOf(err), connect.CodeUnimplemented)
	}
	resp, err := p.AddDownload(ctx, pluginv1.AddDownloadRequest_builder{
		ClientId: proto.String("a"), Title: proto.String("T"), Kind: pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES.Enum(),
		Recent: proto.Bool(true), Data: []byte("d"), SeedRatio: proto.Float64(1.5), SeedTime: durationpb.New(time.Hour),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	r := a.added
	if resp.GetDownloadId() != "ID" || r.Kind != client.Series || !r.Recent || *r.SeedRatio != 1.5 || *r.SeedTime != time.Hour {
		t.Errorf("AddDownload() = %q with %+v, want ID with the series release", resp.GetDownloadId(), r)
	}
	if _, err := p.AddDownload(ctx, pluginv1.AddDownloadRequest_builder{ClientId: proto.String("b")}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("AddDownload(unknown) code = %v, want = %v", connect.CodeOf(err), connect.CodeNotFound)
	}

	a.err = &client.UnavailableError{Err: errors.New("refused")}
	if _, err := p.AddDownload(ctx, pluginv1.AddDownloadRequest_builder{ClientId: proto.String("a"), Data: []byte("d")}.Build()); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("AddDownload(unavailable) code = %v, want = %v", connect.CodeOf(err), connect.CodeUnavailable)
	}
	if _, _, disabled := p.statuses.Disabled("a"); !disabled {
		t.Errorf("a is not disabled after failing")
	}
}

func TestListDownloads(t *testing.T) {
	five := 5 * time.Minute
	a := &fakeClient{items: []client.Item{{ID: "H1", Title: "Movie", Kind: client.Movie, Status: client.Downloading, RemainingTime: &five}}}
	b := &fakeClient{err: errors.New("boom")}
	p := newTestPlugin(t, map[string]*fakeClient{"a": a, "b": b})
	configure(t, p, "a", "b")
	resp, err := p.ListDownloads(context.Background(), pluginv1.ListDownloadsRequest_builder{IncludeStatuses: proto.Bool(true)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetDownloads()) != 1 || resp.GetDownloads()[0].GetClientId() != "a" ||
		resp.GetDownloads()[0].GetKind() != pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE ||
		resp.GetDownloads()[0].GetStatus() != pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING ||
		resp.GetDownloads()[0].GetRemainingTime().AsDuration() != five {
		t.Errorf("downloads = %v, want a's movie", resp.GetDownloads())
	}
	if len(resp.GetStatuses()) != 1 || resp.GetStatuses()[0].GetOutputRootFolders()[0] != "/downloads" {
		t.Errorf("statuses = %v, want a's", resp.GetStatuses())
	}
	if len(resp.GetFailures()) != 1 || resp.GetFailures()[0].GetClientId() != "b" || !resp.GetFailures()[0].HasDisabledUntil() {
		t.Errorf("failures = %v, want b disabled", resp.GetFailures())
	}

	// A disabled client is left out until asked for.
	resp, err = p.ListDownloads(context.Background(), &pluginv1.ListDownloadsRequest{})
	if err != nil || len(resp.GetFailures()) != 0 {
		t.Errorf("ListDownloads() failures = %v, %v, want none with b disabled", resp.GetFailures(), err)
	}
}

func TestDownloadActions(t *testing.T) {
	a := &fakeClient{items: []client.Item{{ID: "H1", Title: "Movie"}}}
	p := newTestPlugin(t, map[string]*fakeClient{"a": a})
	configure(t, p, "a")
	ctx := context.Background()
	got, err := p.GetDownload(ctx, pluginv1.GetDownloadRequest_builder{ClientId: proto.String("a"), DownloadId: proto.String("H1")}.Build())
	if err != nil || got.GetDownload().GetOutputPath() != "/imported/Movie" {
		t.Errorf("GetDownload() = %v, %v, want the import item", got, err)
	}
	if _, err := p.GetDownload(ctx, pluginv1.GetDownloadRequest_builder{ClientId: proto.String("a"), DownloadId: proto.String("H2")}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetDownload(missing) code = %v, want = %v", connect.CodeOf(err), connect.CodeNotFound)
	}
	if _, err := p.RemoveDownload(ctx, pluginv1.RemoveDownloadRequest_builder{ClientId: proto.String("a"), DownloadId: proto.String("H2")}.Build()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.MarkDownloadImported(ctx, pluginv1.MarkDownloadImportedRequest_builder{ClientId: proto.String("a"), DownloadId: proto.String("H1")}.Build()); err != nil {
		t.Fatal(err)
	}
	if len(a.removed) != 1 || a.removed[0] != "H2" || len(a.imported) != 1 || a.imported[0] != "H1" {
		t.Errorf("removed, imported = %v, %v, want [H2], [H1]", a.removed, a.imported)
	}
}

func TestTestDownloadClient(t *testing.T) {
	p := newTestPlugin(t, map[string]*fakeClient{"a": {}, "bad": {err: errors.New("no")}})
	ctx := context.Background()
	resp, err := p.TestDownloadClient(ctx, pluginv1.TestDownloadClientRequest_builder{DefinitionId: proto.String("fake"), SettingsJson: proto.String(`{"name":"a"}`)}.Build())
	if err != nil || resp.HasFailure() {
		t.Errorf("TestDownloadClient(a) = %v, %v, want success", resp, err)
	}
	resp, err = p.TestDownloadClient(ctx, pluginv1.TestDownloadClientRequest_builder{DefinitionId: proto.String("fake"), SettingsJson: proto.String(`{"name":"bad"}`)}.Build())
	if err != nil || resp.GetFailure() != "no" {
		t.Errorf("TestDownloadClient(bad) = %v, %v, want the failure", resp, err)
	}
}

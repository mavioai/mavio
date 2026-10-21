package server_test

import (
	"net/http"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// TestServerProxies chooses proxies for the server's own requests and for
// a WASM plugin's, and keeps a proxy they use from being deleted.
func TestServerProxies(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	catalogURL := catalog(t)
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: t.TempDir(),
		PluginCatalog: catalogURL,
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("proxies"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	pluginAdmin := systemv1connect.NewPluginManagementServiceClient(http.DefaultClient, srv.url, token)
	proxyAdmin := systemv1connect.NewProxyServiceClient(http.DefaultClient, srv.url, token)

	created, err := proxyAdmin.CreateProxy(ctx, systemv1.CreateProxyRequest_builder{Proxy: systemv1.Proxy_builder{
		Name: new("Office"), Kind: new(systemv1.ProxyKind_PROXY_KIND_HTTP), Host: new("proxy.lan"), Port: new(int32(3128)),
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}
	office := created.GetProxy().GetId()

	// The server's requests use the system proxy until they are given
	// others; an unknown proxy is refused.
	got, err := proxyAdmin.GetOutboundProxies(ctx, &systemv1.GetOutboundProxiesRequest{})
	if err != nil {
		t.Fatalf("GetOutboundProxies: %v", err)
	}
	set := got.GetProxies()
	if set.GetPluginCatalogs() != "system" || set.GetImages() != "system" || set.GetMavioDev() != "system" || set.GetAcme() != "system" {
		t.Errorf("default proxies got = %v, want = the system proxy for everything", set)
	}
	set.SetImages(office)
	set.SetAcme("")
	if _, err := proxyAdmin.UpdateOutboundProxies(ctx, systemv1.UpdateOutboundProxiesRequest_builder{Proxies: set}.Build()); err != nil {
		t.Fatalf("UpdateOutboundProxies: %v", err)
	}
	got, err = proxyAdmin.GetOutboundProxies(ctx, &systemv1.GetOutboundProxiesRequest{})
	if p := got.GetProxies(); err != nil || p.GetImages() != office || p.GetAcme() != "" || p.GetMavioDev() != "system" {
		t.Errorf("proxies after the update got = %v, %v", p, err)
	}
	set.SetMavioDev("01a14422-0000-7000-8000-000000000000")
	if _, err := proxyAdmin.UpdateOutboundProxies(ctx, systemv1.UpdateOutboundProxiesRequest_builder{Proxies: set}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("UpdateOutboundProxies with an unknown proxy got = %v, want = invalid_argument", err)
	}

	// A WASM plugin uses the system proxy until given another.
	if _, err := pluginAdmin.InstallPlugin(ctx, systemv1.InstallPluginRequest_builder{PluginId: new("dev.mavio.smoke")}.Build()); err != nil {
		t.Fatalf("InstallPlugin: %v", err)
	}
	pluginProxy := func() string {
		t.Helper()
		list, err := pluginAdmin.ListPlugins(ctx, &systemv1.ListPluginsRequest{})
		if err != nil || len(list.GetPlugins()) != 1 {
			t.Fatalf("ListPlugins = %v, %v", list, err)
		}
		return list.GetPlugins()[0].GetProxy()
	}
	if got := pluginProxy(); got != "system" {
		t.Errorf("installed plugin's proxy got = %q, want = system", got)
	}
	resp, err := pluginAdmin.SetPluginProxy(ctx, systemv1.SetPluginProxyRequest_builder{PluginId: new("dev.mavio.smoke"), Proxy: new(office)}.Build())
	if err != nil || resp.GetPlugin().GetProxy() != office || pluginProxy() != office {
		t.Errorf("SetPluginProxy got = %v, %v; want the plugin through %s", resp, err, office)
	}
	if _, err := pluginAdmin.SetPluginProxy(ctx, systemv1.SetPluginProxyRequest_builder{PluginId: new("dev.mavio.smoke"), Proxy: new("nope")}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("SetPluginProxy with an unknown proxy got = %v, want = invalid_argument", err)
	}

	// The proxy in use is not deleted until nothing uses it.
	del := systemv1.DeleteProxyRequest_builder{Id: new(office)}.Build()
	if _, err := proxyAdmin.DeleteProxy(ctx, del); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("DeleteProxy in use got = %v, want = failed_precondition", err)
	}
	if _, err := pluginAdmin.UninstallPlugin(ctx, systemv1.UninstallPluginRequest_builder{PluginId: new("dev.mavio.smoke")}.Build()); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}
	set.SetMavioDev("system")
	set.SetImages("")
	if _, err := proxyAdmin.UpdateOutboundProxies(ctx, systemv1.UpdateOutboundProxiesRequest_builder{Proxies: set}.Build()); err != nil {
		t.Fatalf("UpdateOutboundProxies: %v", err)
	}
	if _, err := proxyAdmin.DeleteProxy(ctx, del); err != nil {
		t.Errorf("DeleteProxy unused: %v", err)
	}
}

// Command scraper-servarr is the metadata provider of Servarr's services,
// which need no keys: movies from Radarr's metadata service
// (api.radarr.video), with its lists of movies to discover; series and
// their episodes, in TheTVDB's order with absolute numbers, from Sonarr's
// SkyHook (skyhook.sonarr.tv); and, from Sonarr's services
// (services.sonarr.tv) and XEM (thexem.info), the titles releases use for
// series, the series they name by air date and the numbering they use for
// episodes. Requests carry the User-Agent of the application whose
// service it is, as that application builds it. It builds as plugin.wasm
// (GOOS=wasip1, -buildmode=c-shared) and, for the process runtime, as a
// native executable.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

//go:embed manifest.json
var manifestJSON []byte

// shipped is the manifest shipped next to the plugin, which Describe
// returns as is.
var shipped = func() *pluginv1.Manifest {
	m := &pluginv1.Manifest{}
	if err := protojson.Unmarshal(manifestJSON, m); err != nil {
		panic(fmt.Sprintf("manifest.json: %v", err))
	}
	return m
}()

func init() {
	p := newPlugin()
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(p))
	guest.Handle(pluginv1connect.NewEpisodeListProviderServiceHandler(p))
	guest.Handle(pluginv1connect.NewDiscoveryProviderServiceHandler(p))
	guest.Handle(pluginv1connect.NewSceneMappingProviderServiceHandler(p))
}

// The services' base URLs.
const (
	radarrMetadataURL = "https://api.radarr.video/v1"
	skyHookURL        = "https://skyhook.sonarr.tv/v1/tvdb"
	sonarrServicesURL = "https://services.sonarr.tv/v1"
	xemURL            = "https://thexem.info/map"
)

// The versions of Radarr and Sonarr ported, whose User-Agent requests
// carry.
const (
	radarrVersion = "6.4.4.10684"
	sonarrVersion = "4.0.20.3012"
)

// plugin implements the lifecycle, metadata provider, episode list
// provider, discovery provider and scene mapping provider services.
type plugin struct {
	radarr, skyHook, services, xem *client
	// now is the clock; nil for time.Now.
	now func() time.Time

	// shows caches SkyHook's series, which each of their seasons and
	// episodes is looked up in.
	shows cache[int, skyShow]
	// xemSeries caches the series XEM has numbering for.
	xemMu      sync.Mutex
	xemSeries  map[int]bool
	xemExpires time.Time
}

func newPlugin() *plugin {
	radarr := userAgent("Radarr", radarrVersion)
	sonarr := userAgent("Sonarr", sonarrVersion)
	return &plugin{
		radarr:   &client{baseURL: radarrMetadataURL, userAgent: radarr},
		skyHook:  &client{baseURL: skyHookURL, userAgent: sonarr},
		services: &client{baseURL: sonarrServicesURL, userAgent: sonarr},
		xem:      &client{baseURL: xemURL, userAgent: sonarr},
	}
}

// userAgent is the User-Agent of a Servarr application, as its
// UserAgentBuilder makes it.
func userAgent(app, version string) func() string {
	return func() string { return fmt.Sprintf("%s/%s (%s)", app, version, guest.HostOS()) }
}

func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	return pluginv1.DescribeResponse_builder{Manifest: shipped}.Build(), nil
}

func (p *plugin) Configure(context.Context, *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	return &pluginv1.ConfigureResponse{}, nil
}

func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

func (p *plugin) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

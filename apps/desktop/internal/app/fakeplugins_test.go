package app

import (
	"context"
	"errors"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

// fakeTMDBSchema is the TMDB plugin's configuration schema: a key, a
// language among two, and a switch needing its cache.
const fakeTMDBSchema = `{"type": "object", "properties": {
	"api_key": {"type": "string"},
	"language": {"type": "string", "enum": ["en", "de"], "default": "en"},
	"cache": {"type": "string", "default": "cache"},
	"use_cache": {"type": "boolean", "x-mavio-requires-container": "cache"}
}}`

// fakePluginState is what a fake server's plugins hold: those installed,
// TMDB's configuration and how its cache runs.
type fakePluginState struct {
	installed []*systemv1.Plugin
	config    string
	cacheMode pluginv1.ContainerMode
}

func newFakePluginState() *fakePluginState {
	return &fakePluginState{
		installed: []*systemv1.Plugin{
			systemv1.Plugin_builder{Manifest: pluginv1.Manifest_builder{
				Id: new("dev.mavio.mail"), Name: new("Mail"), Capabilities: []pluginv1.Capability{pluginv1.Capability_CAPABILITY_PASSWORD_RESET},
			}.Build(), State: new(systemv1.PluginState_PLUGIN_STATE_READY)}.Build(),
			systemv1.Plugin_builder{Manifest: pluginv1.Manifest_builder{
				Id: new("dev.mavio.tmdb"), Name: new("TMDB"), Version: new("1.0.0"), Author: new("Mavio"),
				Description: new("Metadata of movies and shows from The Movie Database."), License: new("GPL-3.0"),
				Category: new(pluginv1.Category_CATEGORY_METADATA), ConfigSchema: new(fakeTMDBSchema),
				Containers: []*pluginv1.ContainerDependency{pluginv1.ContainerDependency_builder{Name: new("cache")}.Build()},
			}.Build(), State: new(systemv1.PluginState_PLUGIN_STATE_READY)}.Build(),
		},
		config:    `{"api_key": "abc", "cache": "cache"}`,
		cacheMode: pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED,
	}
}

func (s *fakePlugins) ListPlugins(context.Context, *systemv1.ListPluginsRequest) (*systemv1.ListPluginsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	var out []*systemv1.Plugin
	for _, p := range s.f.plugins.installed {
		out = append(out, proto.CloneOf(p))
	}
	return systemv1.ListPluginsResponse_builder{Plugins: out}.Build(), nil
}

func (s *fakePlugins) ListCatalogPlugins(context.Context, *systemv1.ListCatalogPluginsRequest) (*systemv1.ListCatalogPluginsResponse, error) {
	version := func(v string) *systemv1.CatalogVersion {
		return systemv1.CatalogVersion_builder{Version: new(v)}.Build()
	}
	return systemv1.ListCatalogPluginsResponse_builder{Plugins: []*systemv1.CatalogPlugin{
		systemv1.CatalogPlugin_builder{
			Id: new("dev.mavio.tmdb"), Name: new("TMDB"), Category: new(pluginv1.Category_CATEGORY_METADATA),
			CatalogUrl: new("https://plugins.mavio.test/catalog.json"), Versions: []*systemv1.CatalogVersion{version("1.1.0"), version("1.0.0")},
		}.Build(),
		systemv1.CatalogPlugin_builder{
			Id: new("dev.mavio.opensubtitles"), Name: new("OpenSubtitles"), Description: new("Subtitles from OpenSubtitles.com."),
			Category: new(pluginv1.Category_CATEGORY_SUBTITLES), CatalogUrl: new("https://plugins.mavio.test/catalog.json"),
			Versions: []*systemv1.CatalogVersion{version("2.0.0")},
		}.Build(),
	}}.Build(), nil
}

func (s *fakePlugins) InstallPlugin(_ context.Context, r *systemv1.InstallPluginRequest) (*systemv1.InstallPluginResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	for _, p := range s.f.plugins.installed {
		if p.GetManifest().GetId() == r.GetPluginId() {
			p.GetManifest().SetVersion(cmpOr(r.GetVersion(), "1.1.0"))
			return systemv1.InstallPluginResponse_builder{Plugin: proto.CloneOf(p)}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("no such plugin"))
}

func (s *fakePlugins) UninstallPlugin(_ context.Context, r *systemv1.UninstallPluginRequest) (*systemv1.UninstallPluginResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.plugins.installed = slices.DeleteFunc(s.f.plugins.installed, func(p *systemv1.Plugin) bool { return p.GetManifest().GetId() == r.GetPluginId() })
	return &systemv1.UninstallPluginResponse{}, nil
}

func (s *fakePlugins) GetPluginConfig(context.Context, *systemv1.GetPluginConfigRequest) (*systemv1.GetPluginConfigResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return systemv1.GetPluginConfigResponse_builder{ConfigJson: new(s.f.plugins.config)}.Build(), nil
}

func (s *fakePlugins) SetPluginConfig(_ context.Context, r *systemv1.SetPluginConfigRequest) (*systemv1.SetPluginConfigResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.plugins.config = r.GetConfigJson()
	return systemv1.SetPluginConfigResponse_builder{ConfigJson: new(s.f.plugins.config)}.Build(), nil
}

func (s *fakePlugins) ListPluginContainers(context.Context, *systemv1.ListPluginContainersRequest) (*systemv1.ListPluginContainersResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return systemv1.ListPluginContainersResponse_builder{Containers: []*systemv1.PluginContainer{systemv1.PluginContainer_builder{
		PluginId:   new("dev.mavio.tmdb"),
		Dependency: pluginv1.ContainerDependency_builder{Name: new("cache"), Description: new("Caches responses")}.Build(),
		Status:     pluginv1.ContainerStatus_builder{Mode: new(s.f.plugins.cacheMode)}.Build(),
	}.Build()}}.Build(), nil
}

func (s *fakePlugins) SetPluginContainer(_ context.Context, r *systemv1.SetPluginContainerRequest) (*systemv1.SetPluginContainerResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.plugins.cacheMode = r.GetMode()
	return &systemv1.SetPluginContainerResponse{}, nil
}

func (s *fakePlugins) GetPluginCatalogs(context.Context, *systemv1.GetPluginCatalogsRequest) (*systemv1.GetPluginCatalogsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return systemv1.GetPluginCatalogsResponse_builder{Urls: slices.Clone(s.f.catalogs)}.Build(), nil
}

func (s *fakePlugins) UpdatePluginCatalogs(_ context.Context, r *systemv1.UpdatePluginCatalogsRequest) (*systemv1.UpdatePluginCatalogsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.catalogs = slices.Clone(r.GetUrls())
	return systemv1.UpdatePluginCatalogsResponse_builder{Urls: slices.Clone(s.f.catalogs)}.Build(), nil
}

// Command scraper-openlibrary is the metadata provider for Open Library:
// books and audiobooks, found by ISBN, Open Library work ID, or title and
// author, with their authors, subjects and covers. It builds as
// plugin.wasm (GOOS=wasip1, -buildmode=c-shared) and, for the process
// runtime, as a native executable. It needs no configuration.
package main

import (
	"context"
	_ "embed"
	"fmt"

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

// Open Library's API and cover service.
const (
	defaultBaseURL   = "https://openlibrary.org"
	defaultCoversURL = "https://covers.openlibrary.org"
)

func init() {
	p := &plugin{baseURL: defaultBaseURL, coversURL: defaultCoversURL}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(p))
}

// plugin implements the lifecycle and metadata provider services.
type plugin struct {
	baseURL, coversURL string
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

func (p *plugin) client() *client {
	return &client{http: guest.HTTPClient(), baseURL: p.baseURL, coversURL: p.coversURL}
}

// Command testplugin is the plugin used by the host tests. It builds for both
// runtimes: as plugin.wasm (GOOS=wasip1, -buildmode=c-shared) and as a native
// executable.
//
// Searches act on the lookup name:
//
//	"config"        returns the current configuration in the overview
//	"events"        returns the types of the events consumed so far, which
//	                the data folder keeps, as WASM instances share no memory
//	"fetch:<url>"   GETs the URL with guest.HTTPClient and returns the body
//	"host:<path>"   GETs the path of the host API with guest.HostClient and
//	                returns the status and body
//	"crash"         exits the plugin
//	"panic"         panics in the handler
//	"slow"          sleeps for two seconds
//	anything else   echoes the name
//
// Its HTTP routes: "/echo" answers with the method, path, query, the
// Authorization and Mavio-Plugin-Token headers and the body; "/big" with
// 17 MiB.
//
// As a device controller it takes commands to its device "tv" and rejects
// others, naming the device and the user.
//
// Its tasks: "write" writes a file to its data folder and reads it back;
// "slow" sleeps for two seconds.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/guest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID and Version must match the test manifests.
const (
	ID      = "org.mavio.testplugin"
	Version = "0.1.0"
)

func init() {
	guest.Handle(pluginv1connect.NewPluginServiceHandler(&lifecycle{}))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(&provider{}))
	guest.Handle(pluginv1connect.NewTaskRunnerServiceHandler(tasks{}))
	guest.Handle(pluginv1connect.NewEventConsumerServiceHandler(consumer{}))
	guest.Handle(pluginv1connect.NewDeviceControllerServiceHandler(devices{}))
	routes := http.NewServeMux()
	routes.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "%s %s %s auth=%q token=%q body=%s", r.Method, r.URL.Path, r.URL.RawQuery,
			r.Header.Get("Authorization"), r.Header.Get("Mavio-Plugin-Token"), body)
	})
	routes.HandleFunc("/big", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 17<<20))
	})
	guest.HandleHTTP(routes)
}

type consumer struct{}

func (consumer) Consume(_ context.Context, req *pluginv1.ConsumeRequest) (*pluginv1.ConsumeResponse, error) {
	f, err := os.OpenFile(filepath.Join(guest.DataDir(), "events"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for _, e := range req.GetEvents() {
		if _, err := fmt.Fprintln(f, e.GetType()); err != nil {
			return nil, err
		}
	}
	return &pluginv1.ConsumeResponse{}, f.Close()
}

type devices struct{}

func (devices) SendCommand(_ context.Context, req *pluginv1.SendCommandRequest) (*pluginv1.SendCommandResponse, error) {
	if req.GetDeviceId() != "tv" || !req.GetCommand().HasPlay() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("%s cannot do that for %s", req.GetDeviceId(), req.GetUserName()))
	}
	return &pluginv1.SendCommandResponse{}, nil
}

type tasks struct{}

func (tasks) RunTask(_ context.Context, req *pluginv1.RunTaskRequest) (*pluginv1.RunTaskResponse, error) {
	switch req.GetTaskId() {
	case "write":
		file := filepath.Join(guest.DataDir(), "note.txt")
		if err := os.WriteFile(file, []byte("kept"), 0o600); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return pluginv1.RunTaskResponse_builder{Message: proto.String(file + ": " + string(data))}.Build(), nil
	case "slow":
		time.Sleep(2 * time.Second)
		return pluginv1.RunTaskResponse_builder{Message: proto.String("slept")}.Build(), nil
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no task %s", req.GetTaskId()))
}

var (
	mu     sync.Mutex
	config string
)

type lifecycle struct{}

func (lifecycle) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(Version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

func (lifecycle) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	if strings.Contains(req.GetConfigJson(), "reject") {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("configuration rejected"))
	}
	mu.Lock()
	config = req.GetConfigJson()
	mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func (lifecycle) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (lifecycle) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

type provider struct{}

func (provider) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	name := req.GetLookup().GetName()
	overview := name
	switch {
	case name == "config":
		mu.Lock()
		overview = config
		mu.Unlock()
	case name == "events":
		data, err := os.ReadFile(filepath.Join(guest.DataDir(), "events"))
		if err != nil {
			return nil, err
		}
		overview = strings.Join(strings.Fields(string(data)), " ")
	case strings.HasPrefix(name, "fetch:"):
		resp, err := guest.HTTPClient().Get(strings.TrimPrefix(name, "fetch:"))
		if err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		overview = string(body)
	case strings.HasPrefix(name, "host:"):
		resp, err := guest.HostClient().Get(guest.HostURL + strings.TrimPrefix(name, "host:"))
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		overview = fmt.Sprintf("%d %s", resp.StatusCode, body)
	case name == "crash":
		os.Exit(3)
	case name == "panic":
		panic("boom")
	case name == "slow":
		time.Sleep(2 * time.Second)
	}
	result := pluginv1.SearchResult_builder{Name: proto.String(name), Overview: proto.String(overview)}.Build()
	return pluginv1.SearchResponse_builder{Results: []*pluginv1.SearchResult{result}}.Build(), nil
}

func (provider) GetMetadata(context.Context, *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
}

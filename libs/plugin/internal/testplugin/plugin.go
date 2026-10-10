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
// As an image provider it offers a poster named after the lookup.
//
// As a local metadata reader it takes the name from the first line of the
// first file; as a saver it writes the name to "<media name>.title".
//
// As a metadata processor it adds the tag "processed".
//
// As a resolver it leaves out entries named "skip*" and claims folders
// holding "claim.me", each other file of which is a movie named after it.
//
// As a lyrics provider it finds synced lyrics "<name>" of every track,
// whose single line is the track's name.
//
// As an intro provider it plays first the items whose IDs the file
// "intros" of its data folder lists, one per line.
//
// As an image generator it makes a 2 × 2 PNG primary image.
//
// As a media source provider it gives every item the source "Remote"
// whose URL the file "source" of its data folder holds, if any.
//
// For password resets it appends "<user name> <PIN>" to the file "resets"
// of its data folder.
//
// As a device controller it takes commands to its device "tv" and rejects
// others, naming the device and the user.
//
// Its tasks: "write" writes a file to its data folder and reads it back;
// "slow" sleeps for two seconds.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	guest.Handle(pluginv1connect.NewImageProviderServiceHandler(images{}))
	guest.Handle(pluginv1connect.NewLocalMetadataServiceHandler(local{}))
	guest.Handle(pluginv1connect.NewMetadataSaverServiceHandler(local{}))
	guest.Handle(pluginv1connect.NewMetadataProcessorServiceHandler(processor{}))
	guest.Handle(pluginv1connect.NewLyricsProviderServiceHandler(lyrics{}))
	guest.Handle(pluginv1connect.NewResolverServiceHandler(resolver{}))
	guest.Handle(pluginv1connect.NewIntroProviderServiceHandler(intros{}))
	guest.Handle(pluginv1connect.NewImageGeneratorServiceHandler(generator{}))
	guest.Handle(pluginv1connect.NewMediaSourceProviderServiceHandler(sources{}))
	guest.Handle(pluginv1connect.NewPasswordResetServiceHandler(resets{}))
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

type images struct{}

func (images) GetImages(_ context.Context, req *pluginv1.GetImagesRequest) (*pluginv1.GetImagesResponse, error) {
	return pluginv1.GetImagesResponse_builder{Images: []*pluginv1.RemoteImage{pluginv1.RemoteImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String("https://images.test/" + req.GetLookup().GetName() + ".jpg"),
	}.Build()}}.Build(), nil
}

type local struct{}

func (local) ReadMetadata(_ context.Context, req *pluginv1.ReadMetadataRequest) (*pluginv1.ReadMetadataResponse, error) {
	if len(req.GetFiles()) == 0 {
		return &pluginv1.ReadMetadataResponse{}, nil
	}
	name, _, _ := strings.Cut(string(req.GetFiles()[0].GetContent()), "\n")
	return pluginv1.ReadMetadataResponse_builder{
		Found: proto.Bool(true), Metadata: pluginv1.Metadata_builder{Name: &name}.Build(),
	}.Build(), nil
}

func (local) SaveMetadata(_ context.Context, req *pluginv1.SaveMetadataRequest) (*pluginv1.SaveMetadataResponse, error) {
	return pluginv1.SaveMetadataResponse_builder{Files: []*pluginv1.LocalFile{pluginv1.LocalFile_builder{
		Name: proto.String(req.GetMediaName() + ".title"), Content: []byte(req.GetMetadata().GetName() + "\n"),
	}.Build()}}.Build(), nil
}

type processor struct{}

func (processor) ProcessMetadata(_ context.Context, req *pluginv1.ProcessMetadataRequest) (*pluginv1.ProcessMetadataResponse, error) {
	md := pluginv1.Metadata_builder{Tags: append(req.GetMetadata().GetTags(), "processed")}.Build()
	return pluginv1.ProcessMetadataResponse_builder{Changed: proto.Bool(true), Metadata: md}.Build(), nil
}

type lyrics struct{}

func (lyrics) SearchLyrics(_ context.Context, req *pluginv1.SearchLyricsRequest) (*pluginv1.SearchLyricsResponse, error) {
	return pluginv1.SearchLyricsResponse_builder{Lyrics: []*pluginv1.RemoteLyrics{pluginv1.RemoteLyrics_builder{
		Id: proto.String(req.GetName()), Name: proto.String(req.GetName()), Synced: proto.Bool(true),
	}.Build()}}.Build(), nil
}

func (lyrics) DownloadLyrics(_ context.Context, req *pluginv1.DownloadLyricsRequest) (*pluginv1.DownloadLyricsResponse, error) {
	return pluginv1.DownloadLyricsResponse_builder{Content: proto.String("[00:01.00]" + req.GetId() + "\n"), Synced: proto.Bool(true)}.Build(), nil
}

type resolver struct{}

func (resolver) Ignore(_ context.Context, req *pluginv1.IgnoreRequest) (*pluginv1.IgnoreResponse, error) {
	var names []string
	for _, e := range req.GetFolder().GetEntries() {
		if strings.HasPrefix(e.GetName(), "skip") {
			names = append(names, e.GetName())
		}
	}
	return pluginv1.IgnoreResponse_builder{Names: names}.Build(), nil
}

func (resolver) Resolve(_ context.Context, req *pluginv1.ResolveRequest) (*pluginv1.ResolveResponse, error) {
	entries := req.GetFolder().GetEntries()
	if !slices.ContainsFunc(entries, func(e *pluginv1.FolderEntry) bool { return e.GetName() == "claim.me" }) {
		return &pluginv1.ResolveResponse{}, nil
	}
	var items []*pluginv1.ResolvedItem
	for _, e := range entries {
		if e.GetName() != "claim.me" && !e.GetIsDir() {
			items = append(items, pluginv1.ResolvedItem_builder{
				Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), Entry: proto.String(e.GetName()),
				Name: proto.String(strings.TrimSuffix(e.GetName(), filepath.Ext(e.GetName()))),
			}.Build())
		}
	}
	return pluginv1.ResolveResponse_builder{Claimed: proto.Bool(true), Items: items}.Build(), nil
}

// dataFile returns the lines of a file of the data folder; none when it
// does not exist.
func dataFile(name string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(guest.DataDir(), name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return strings.Fields(string(data)), err
}

type intros struct{}

func (intros) GetIntros(context.Context, *pluginv1.GetIntrosRequest) (*pluginv1.GetIntrosResponse, error) {
	ids, err := dataFile("intros")
	if err != nil {
		return nil, err
	}
	return pluginv1.GetIntrosResponse_builder{ItemIds: ids}.Build(), nil
}

type generator struct{}

func (generator) GenerateImages(_ context.Context, req *pluginv1.GenerateImagesRequest) (*pluginv1.GenerateImagesResponse, error) {
	if !slices.Contains(req.GetImageKinds(), pluginv1.ImageKind_IMAGE_KIND_PRIMARY) {
		return &pluginv1.GenerateImagesResponse{}, nil
	}
	img := image.NewGray(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return pluginv1.GenerateImagesResponse_builder{Images: []*pluginv1.GeneratedImage{pluginv1.GeneratedImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Content: buf.Bytes(),
	}.Build()}}.Build(), nil
}

type sources struct{}

func (sources) GetMediaSources(context.Context, *pluginv1.GetMediaSourcesRequest) (*pluginv1.GetMediaSourcesResponse, error) {
	urls, err := dataFile("source")
	if err != nil || len(urls) == 0 {
		return &pluginv1.GetMediaSourcesResponse{}, err
	}
	return pluginv1.GetMediaSourcesResponse_builder{Sources: []*pluginv1.RemoteMediaSource{pluginv1.RemoteMediaSource_builder{
		Id: proto.String("remote"), Name: proto.String("Remote"), Url: proto.String(urls[0]),
	}.Build()}}.Build(), nil
}

type resets struct{}

func (resets) StartReset(_ context.Context, req *pluginv1.StartResetRequest) (*pluginv1.StartResetResponse, error) {
	f, err := os.OpenFile(filepath.Join(guest.DataDir(), "resets"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, req.GetUserName(), req.GetPin()); err != nil {
		return nil, err
	}
	return &pluginv1.StartResetResponse{}, f.Close()
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

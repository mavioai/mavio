// Command smokeplugin is the metadata provider used by the smoke tests. It
// builds for both runtimes: as plugin.wasm (GOOS=wasip1, -buildmode=c-shared)
// and as a native executable. It knows two films, "Farewell My Concubine"
// and "The Concubine", and searches carelessly: shorter names first,
// whatever the year. With "image_base" configured, it offers two posters
// and a backdrop of each film under that URL. It has a handler for every
// capability; the manifest of each test picks those the server uses
// (capabilities.go describes the rest).
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/plugin/guest"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID must match the smoke test manifest.
const ID = "org.mavio.smoke"

// Version must match the manifest too; builds of other versions set it
// with -ldflags "-X main.Version=…".
var Version = "0.1.0"

// TMDBID is the external ID of "Farewell My Concubine"; OtherTMDBID that
// of "The Concubine".
const (
	TMDBID      = "10997"
	OtherTMDBID = "117974"
)

type film struct {
	id, name, original, overview, collection, collectionID string
	year                                                   int32
	genres                                                 []string
}

var films = []film{
	{
		id: TMDBID, name: "Farewell My Concubine", original: "霸王别姬", year: 1993,
		overview: "Two boys meet at an opera training school in Peking in 1924.", genres: []string{"Drama", "Romance"},
		collection: "Chen Kaige Classics", collectionID: "4242",
	},
	{id: OtherTMDBID, name: "The Concubine", original: "후궁: 제왕의 첩", year: 2012, overview: "A palace intrigue.", genres: []string{"History"}},
}

// config is the plugin's configuration.
var config struct {
	sync.Mutex
	ImageBase string `json:"image_base"`
}

func init() {
	guest.Handle(pluginv1connect.NewPluginServiceHandler(lifecycle{}))
	guest.Handle(pluginv1connect.NewMetadataProviderServiceHandler(provider{}))
	guest.Handle(pluginv1connect.NewAuthProviderServiceHandler(authProvider{}))
	guest.Handle(pluginv1connect.NewNotifierServiceHandler(notifier{}))
	guest.Handle(pluginv1connect.NewMediaSegmentProviderServiceHandler(segments{}))
	guest.Handle(pluginv1connect.NewTaskRunnerServiceHandler(tasks{}))
	guest.Handle(pluginv1connect.NewEventConsumerServiceHandler(consumer{}))
	guest.Handle(pluginv1connect.NewDeviceControllerServiceHandler(devices{}))
	routes := http.NewServeMux()
	// whoami answers with who the host says the caller is, and whether the
	// caller's Authorization header came through.
	routes.HandleFunc("GET /whoami", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "user=%q admin=%q authorization=%q", r.Header.Get("Mavio-User-Name"),
			r.Header.Get("Mavio-User-Admin"), r.Header.Get("Authorization"))
	})
	// devices lists a remote device per line of the body, named after it.
	routes.HandleFunc("POST /devices", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var list []*pluginv1.Device
		for id := range strings.FieldsSeq(string(body)) {
			list = append(list, pluginv1.Device_builder{
				Id: proto.String(id), Name: proto.String("Device " + id), Product: proto.String("Smoke TV"),
				Commands: []pluginv1.CommandKind{pluginv1.CommandKind_COMMAND_KIND_PLAY, pluginv1.CommandKind_COMMAND_KIND_PLAY_STATE},
			}.Build())
		}
		host := pluginv1connect.NewHostServiceClient(guest.HostClient(), guest.HostURL)
		if _, err := host.SetDevices(r.Context(), pluginv1.SetDevicesRequest_builder{Devices: list}.Build()); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	guest.HandleHTTP(routes)
}

// devices plays the first item of a Play command, acting as the user who
// sent it and the device, and takes play state commands without doing
// anything.
type devices struct{}

func (devices) SendCommand(ctx context.Context, req *pluginv1.SendCommandRequest) (*pluginv1.SendCommandResponse, error) {
	play := req.GetCommand().GetPlay()
	if play == nil {
		return &pluginv1.SendCommandResponse{}, nil
	}
	client := &http.Client{Transport: headers{
		guest.UserHeader: req.GetUserName(), guest.DeviceHeader: req.GetDeviceId(),
	}}
	playbacks := playbackv1connect.NewPlaybackServiceClient(client, guest.HostURL)
	_, err := playbacks.StartPlayback(ctx, playbackv1.StartPlaybackRequest_builder{
		ItemId: proto.String(play.GetItemIds()[0]), Capabilities: &playbackv1.ClientCapabilities{},
	}.Build())
	if err != nil {
		return nil, err
	}
	return &pluginv1.SendCommandResponse{}, nil
}

// headers adds headers to host API requests.
type headers map[string]string

func (h headers) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h {
		r.Header.Set(k, v)
	}
	return guest.HostClient().Transport.RoundTrip(r)
}

// consumer appends a line per event to "events" in the data folder: its
// type, then its attributes as key=value in key order.
type consumer struct{}

func (consumer) Consume(_ context.Context, req *pluginv1.ConsumeRequest) (*pluginv1.ConsumeResponse, error) {
	f, err := os.OpenFile(filepath.Join(guest.DataDir(), "events"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for _, e := range req.GetEvents() {
		line := []string{e.GetType()}
		for _, k := range slices.Sorted(maps.Keys(e.GetAttributes())) {
			line = append(line, k+"="+e.GetAttributes()[k])
		}
		if _, err := fmt.Fprintln(f, strings.Join(line, " ")); err != nil {
			return nil, err
		}
	}
	return &pluginv1.ConsumeResponse{}, f.Close()
}

// tasks runs "count", which counts its runs in the data folder, and
// "fail", which fails.
type tasks struct{}

func (tasks) RunTask(_ context.Context, req *pluginv1.RunTaskRequest) (*pluginv1.RunTaskResponse, error) {
	if req.GetTaskId() != "count" {
		return nil, errors.New("task failed")
	}
	file := filepath.Join(guest.DataDir(), "runs")
	data, _ := os.ReadFile(file)
	runs, _ := strconv.Atoi(string(data))
	runs++
	if err := os.WriteFile(file, []byte(strconv.Itoa(runs)), 0o600); err != nil {
		return nil, err
	}
	return pluginv1.RunTaskResponse_builder{Message: proto.String(fmt.Sprintf("run %d", runs))}.Build(), nil
}

// segments finds a two-second intro and outro in every video.
type segments struct{}

func (segments) GetSegments(_ context.Context, req *pluginv1.GetSegmentsRequest) (*pluginv1.GetSegmentsResponse, error) {
	d := req.GetDuration().AsDuration()
	if d < 5*time.Second {
		return &pluginv1.GetSegmentsResponse{}, nil
	}
	seg := func(kind pluginv1.SegmentKind, start, end time.Duration) *pluginv1.Segment {
		return pluginv1.Segment_builder{Kind: kind.Enum(), Start: durationpb.New(start), End: durationpb.New(end)}.Build()
	}
	return pluginv1.GetSegmentsResponse_builder{Segments: []*pluginv1.Segment{
		seg(pluginv1.SegmentKind_SEGMENT_KIND_INTRO, 0, 2*time.Second),
		seg(pluginv1.SegmentKind_SEGMENT_KIND_OUTRO, d-2*time.Second, d),
	}}.Build(), nil
}

// authProvider accepts the password "directory" of any user, as
// administrators when their name starts with "admin".
type authProvider struct{}

func (authProvider) Authenticate(_ context.Context, req *pluginv1.AuthenticateRequest) (*pluginv1.AuthenticateResponse, error) {
	ok := req.GetPassword() == "directory"
	return pluginv1.AuthenticateResponse_builder{
		Authenticated: proto.Bool(ok), DisplayName: proto.String(strings.ToUpper(req.GetUsername())),
		Admin: proto.Bool(strings.HasPrefix(req.GetUsername(), "admin")),
	}.Build(), nil
}

// notifier rejects events without a type and appends the type of the
// others to "notified" in the data folder.
type notifier struct{}

func (notifier) Notify(_ context.Context, req *pluginv1.NotifyRequest) (*pluginv1.NotifyResponse, error) {
	if req.GetEvent().GetType() == "" {
		return nil, errors.New("event without a type")
	}
	if err := appendData("notified", req.GetEvent().GetType()); err != nil {
		return nil, err
	}
	return &pluginv1.NotifyResponse{}, nil
}

type lifecycle struct{}

func (lifecycle) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(Version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

func (lifecycle) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	config.Lock()
	defer config.Unlock()
	config.ImageBase = ""
	_ = json.Unmarshal([]byte(req.GetConfigJson()), &config)
	return &pluginv1.ConfigureResponse{}, nil
}

func (lifecycle) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

func (lifecycle) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

type provider struct{}

func (provider) Search(_ context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	name := strings.ToLower(req.GetLookup().GetName())
	var found []film
	for _, f := range films {
		if name != "" && strings.Contains(strings.ToLower(f.name), name) {
			found = append(found, f)
		}
	}
	slices.SortStableFunc(found, func(a, b film) int { return cmp.Compare(len(a.name), len(b.name)) })
	resp := &pluginv1.SearchResponse{}
	for _, f := range found {
		resp.SetResults(append(resp.GetResults(), pluginv1.SearchResult_builder{
			Name:        proto.String(f.name),
			Year:        proto.Int32(f.year),
			ExternalIds: map[string]string{"tmdb": f.id},
			Score:       proto.Float64(1),
		}.Build()))
	}
	return resp, nil
}

func (provider) GetMetadata(_ context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	i := slices.IndexFunc(films, func(f film) bool { return f.id == req.GetLookup().GetExternalIds()["tmdb"] })
	if i < 0 {
		return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	f := films[i]
	ids := map[string]string{"tmdb": f.id}
	if f.id == TMDBID {
		ids["imdb"] = "tt0106332"
	}
	if f.collectionID != "" {
		ids["tmdb_collection"] = f.collectionID
	}
	md := pluginv1.Metadata_builder{
		Name:           proto.String(f.name),
		OriginalTitle:  proto.String(f.original),
		Overview:       proto.String(f.overview),
		ProductionYear: proto.Int32(f.year),
		Genres:         f.genres,
		ExternalIds:    ids,
		CollectionName: proto.String(f.collection),
	}.Build()
	if f.id == TMDBID {
		md.SetPeople([]*pluginv1.PersonCredit{
			pluginv1.PersonCredit_builder{Name: proto.String("张国荣"), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(), Role: proto.String("Cheng Dieyi")}.Build(),
			pluginv1.PersonCredit_builder{Name: proto.String("陈凯歌"), Kind: pluginv1.CreditKind_CREDIT_KIND_DIRECTOR.Enum()}.Build(),
		})
	}
	config.Lock()
	base := config.ImageBase
	config.Unlock()
	if base != "" {
		image := func(kind pluginv1.ImageKind, name string, score float64) *pluginv1.RemoteImage {
			return pluginv1.RemoteImage_builder{
				Kind: kind.Enum(), Url: proto.String(base + "/" + f.id + "/" + name), Width: proto.Int32(1000),
				Height: proto.Int32(1500), Score: proto.Float64(score),
			}.Build()
		}
		md.SetImages([]*pluginv1.RemoteImage{
			image(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, "poster.jpg", 2),
			image(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, "poster-alt.jpg", 1),
			image(pluginv1.ImageKind_IMAGE_KIND_BACKDROP, "fanart.jpg", 1),
		})
	}
	return pluginv1.GetMetadataResponse_builder{Found: proto.Bool(true), Metadata: md}.Build(), nil
}

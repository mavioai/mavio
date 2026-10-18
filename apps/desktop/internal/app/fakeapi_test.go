package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// fakeAPI serves the server's API that the screens call through the core,
// with the fixed data of the snapshot tests.
type fakeAPI struct {
	libraryv1connect.UnimplementedItemServiceHandler
	f *fakeCore
}

// fakeUsers serves the user signed in, whose playback preferences it
// keeps.
type fakeUsers struct {
	userv1connect.UnimplementedUserServiceHandler
	f *fakeCore
}

// fakeSessions serves the user's devices: this one, and a television
// playing, which signing out removes.
type fakeSessions struct {
	sessionv1connect.UnimplementedSessionServiceHandler
	f *fakeCore
}

// apiHandler returns the fake server's Connect handlers.
func (f *fakeCore) apiHandler() http.Handler {
	mux := http.NewServeMux()
	api := &fakeAPI{f: f}
	mux.Handle(libraryv1connect.NewItemServiceHandler(api))
	mux.Handle(userv1connect.NewUserServiceHandler(&fakeUsers{f: f}))
	mux.Handle(sessionv1connect.NewSessionServiceHandler(&fakeSessions{f: f}))
	f.prefs = userv1.PlaybackPreferences_builder{
		AudioLanguages: []string{"jpn", "eng"}, SubtitleLanguages: []string{"eng"},
		SubtitleMode: new(userv1.SubtitleMode_SUBTITLE_MODE_FOREIGN), PlayDefaultAudioTrack: new(true),
	}.Build()
	f.sessions = []*sessionv1.Session{
		sessionv1.Session_builder{
			Id: new("6f1c1d1e-0000-4000-8000-000000000001"), DeviceName: new("MacBook Pro"), Client: new("Mavio for Mac"),
			Online: new(true), Current: new(true),
		}.Build(),
		sessionv1.Session_builder{
			Id: new("6f1c1d1e-0000-4000-8000-000000000002"), DeviceName: new("Living Room TV"), Client: new("Mavio for Android TV"),
			Online: new(true), NowPlaying: sessionv1.NowPlaying_builder{Item: libraryv1.Item_builder{Name: new("The Long Orbit")}.Build()}.Build(),
		}.Build(),
	}
	return mux
}

func (u *fakeUsers) GetCurrentUser(context.Context, *userv1.GetCurrentUserRequest) (*userv1.GetCurrentUserResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	r := u.f.record()
	return userv1.GetCurrentUserResponse_builder{User: userv1.User_builder{
		Id: new(r.GetUserId()), Name: new(r.GetUserName()), Admin: new(true), PlaybackPreferences: u.f.prefs,
	}.Build()}.Build(), nil
}

func (u *fakeUsers) UpdatePlaybackPreferences(_ context.Context, r *userv1.UpdatePlaybackPreferencesRequest) (*userv1.UpdatePlaybackPreferencesResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	u.f.prefs = r.GetPlaybackPreferences()
	return userv1.UpdatePlaybackPreferencesResponse_builder{PlaybackPreferences: u.f.prefs}.Build(), nil
}

func (s *fakeSessions) ListSessions(context.Context, *sessionv1.ListSessionsRequest) (*sessionv1.ListSessionsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return sessionv1.ListSessionsResponse_builder{Sessions: slices.Clone(s.f.sessions)}.Build(), nil
}

func (s *fakeSessions) RevokeSession(_ context.Context, r *sessionv1.RevokeSessionRequest) (*sessionv1.RevokeSessionResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.sessions = slices.DeleteFunc(s.f.sessions, func(x *sessionv1.Session) bool { return x.GetId() == r.GetId() })
	return &sessionv1.RevokeSessionResponse{}, nil
}

// callServer answers a CallServer as the core does: with the fake
// server's response, or its error as the core's.
func (f *fakeCore) callServer(ctx context.Context, c *capiv1.CallServer) (*capiv1.Message, error) {
	if f.api == nil {
		return nil, &capi.Error{Code: capiv1.Error_CODE_UNREACHABLE}
	}
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "http://mavio.test"+c.GetProcedure(), bytes.NewReader(c.GetRequest()))
	r.Header.Set("Content-Type", "application/proto")
	w := httptest.NewRecorder()
	f.api.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		var e struct{ Code, Message string }
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		code := capiv1.Error_CODE_SERVER
		if e.Code == connect.CodeNotFound.String() {
			code = capiv1.Error_CODE_NOT_FOUND
		}
		return nil, &capi.Error{Code: code, Reason: e.Message}
	}
	return capiv1.Message_builder{Response: capiv1.ServerResponse_builder{Body: w.Body.Bytes(), BaseUrl: f.images}.Build()}.Build(), nil
}

func (api *fakeAPI) image(id string, kind libraryv1.ImageKind) *libraryv1.Image {
	return libraryv1.Image_builder{Kind: &kind, Url: new("/images/" + id)}.Build()
}

// item returns the fixed item of an ID as the server lists it.
func (api *fakeAPI) item(id string) *libraryv1.Item {
	it := api.f.s.Items[id]
	kind := map[string]libraryv1.ItemKind{
		"movie": libraryv1.ItemKind_ITEM_KIND_MOVIE, "series": libraryv1.ItemKind_ITEM_KIND_SERIES, "episode": libraryv1.ItemKind_ITEM_KIND_EPISODE,
	}[it.Kind]
	b := libraryv1.Item_builder{
		Id: &id, Kind: &kind, Name: new(it.Name), LibraryId: new(it.Library), ProductionYear: new(int32(it.Year)),
		Images: []*libraryv1.Image{
			api.image(id+"-poster", libraryv1.ImageKind_IMAGE_KIND_PRIMARY), api.image(id+"-thumb", libraryv1.ImageKind_IMAGE_KIND_THUMB),
		},
	}
	if it.RuntimeMinutes > 0 {
		b.Runtime = durationpb.New(time.Duration(it.RuntimeMinutes) * time.Minute)
	}
	if it.Kind == "episode" {
		b.SeriesName = new(api.f.s.Items[it.Series].Name)
		b.ParentIndexNumber, b.IndexNumber = new(int32(it.Season)), new(int32(it.Episode))
	}
	return b.Build()
}

// person is the person every item credits.
func (api *fakeAPI) person(id string) *libraryv1.Person {
	switch id {
	case "mara":
		return libraryv1.Person_builder{
			Id: new("mara"), Name: new("Mara Ellison"), BirthPlace: new("Halifax, Nova Scotia, Canada"),
			BirthDate: timestamppb.New(time.Date(1984, 3, 12, 0, 0, 0, 0, time.UTC)),
			Overview:  new("Mara Ellison is a made-up actor of the snapshot tests, whose biography runs long enough to be cut after a few lines and shown whole with More."),
			Images:    []*libraryv1.Image{api.image("mara-photo", libraryv1.ImageKind_IMAGE_KIND_PRIMARY)},
			ExternalUrls: []*libraryv1.ExternalUrl{
				libraryv1.ExternalUrl_builder{Name: new("TMDB"), Url: new("https://www.themoviedb.org/person/1")}.Build(),
			},
		}.Build()
	case "jonah":
		return libraryv1.Person_builder{Id: new("jonah"), Name: new("Jonah Pike")}.Build()
	case "ines":
		return libraryv1.Person_builder{Id: new("ines"), Name: new("Inès Varga")}.Build()
	}
	return nil
}

func (api *fakeAPI) credit(id string, kind libraryv1.CreditKind, role string, order int32) *libraryv1.Credit {
	return libraryv1.Credit_builder{Person: api.person(id), Kind: &kind, Role: &role, Order: &order}.Build()
}

func (api *fakeAPI) GetItem(_ context.Context, r *libraryv1.GetItemRequest) (*libraryv1.GetItemResponse, error) {
	if _, ok := api.f.s.Items[r.GetId()]; !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	it := api.item(r.GetId())
	it.SetStudios([]string{"Northlight Pictures"})
	it.SetTags([]string{"space station", "slow burn"})
	it.SetOriginalLanguage("en")
	it.SetCommunityRating(7.8)
	it.SetCriticRating(91)
	it.SetDateAdded(timestamppb.New(api.f.s.Now.Add(-48 * time.Hour)))
	it.SetCollectionName("The Orbit Collection")
	it.SetExternalUrls([]*libraryv1.ExternalUrl{libraryv1.ExternalUrl_builder{Name: new("IMDb"), Url: new("https://www.imdb.com/title/tt0000001")}.Build()})
	stream := func(index int32, kind libraryv1.StreamKind, codec, language string, channels int32) *libraryv1.MediaStream {
		b := libraryv1.MediaStream_builder{Index: &index, Kind: &kind, Codec: &codec, Language: &language, Channels: &channels}
		if kind == libraryv1.StreamKind_STREAM_KIND_VIDEO {
			b.Width, b.Height = new(int32(1920)), new(int32(1080))
		}
		return b.Build()
	}
	return libraryv1.GetItemResponse_builder{
		Item: it,
		MediaSources: []*libraryv1.MediaSource{libraryv1.MediaSource_builder{
			Id: new(r.GetId() + "-source"), Path: new("/media/" + r.GetId() + ".mkv"), SizeBytes: new(int64(4_200_000_000)),
			Streams: []*libraryv1.MediaStream{
				stream(0, libraryv1.StreamKind_STREAM_KIND_VIDEO, "hevc", "", 0),
				stream(1, libraryv1.StreamKind_STREAM_KIND_AUDIO, "aac", "eng", 2),
				stream(2, libraryv1.StreamKind_STREAM_KIND_AUDIO, "ac3", "jpn", 6),
				stream(3, libraryv1.StreamKind_STREAM_KIND_SUBTITLE, "subrip", "eng", 0),
			},
		}.Build()},
		Credits: []*libraryv1.Credit{
			api.credit("mara", libraryv1.CreditKind_CREDIT_KIND_ACTOR, "Commander Hale", 0),
			api.credit("jonah", libraryv1.CreditKind_CREDIT_KIND_ACTOR, "Theo", 1),
			api.credit("ines", libraryv1.CreditKind_CREDIT_KIND_DIRECTOR, "", 0),
			api.credit("jonah", libraryv1.CreditKind_CREDIT_KIND_WRITER, "", 1),
		},
	}.Build(), nil
}

func (api *fakeAPI) GetPerson(_ context.Context, r *libraryv1.GetPersonRequest) (*libraryv1.GetPersonResponse, error) {
	p := api.person(r.GetId())
	if p == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return libraryv1.GetPersonResponse_builder{Person: p}.Build(), nil
}

func (api *fakeAPI) ListItems(_ context.Context, r *libraryv1.ListItemsRequest) (*libraryv1.ListItemsResponse, error) {
	var items []*libraryv1.Item
	switch {
	case r.GetOwnerId() != "":
		trailer := libraryv1.ExtraKind_EXTRA_KIND_TRAILER
		items = append(items, libraryv1.Item_builder{
			Id: new(r.GetOwnerId() + "-trailer"), Name: new("Official Trailer"), Extra: &trailer,
			Runtime: durationpb.New(2*time.Minute + 30*time.Second),
		}.Build())
	case r.GetPersonId() != "":
		for _, id := range []string{"orbit", "harbor", "atlas", "atlas-s2e3"} {
			items = append(items, api.item(id))
		}
	case len(r.GetKinds()) == 1 && r.GetKinds()[0] == libraryv1.ItemKind_ITEM_KIND_COLLECTION:
		items = append(items, libraryv1.Item_builder{
			Id: new("orbit-collection"), Name: new("The Orbit Collection"), Kind: new(libraryv1.ItemKind_ITEM_KIND_COLLECTION),
		}.Build())
	}
	return libraryv1.ListItemsResponse_builder{Items: items, Total: new(int32(len(items)))}.Build(), nil
}

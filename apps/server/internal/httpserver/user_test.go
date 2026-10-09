package httpserver_test

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// login signs a device in and returns its token.
func login(t *testing.T, url, name, password, deviceID string) string {
	t.Helper()
	resp, err := authv1connect.NewAuthServiceClient(http.DefaultClient, url).Login(t.Context(),
		authv1.LoginRequest_builder{Name: &name, Password: &password, Device: device(deviceID)}.Build())
	if err != nil {
		t.Fatalf("Login %s: %v", name, err)
	}
	return resp.GetAccessToken()
}

func wantCode(t *testing.T, what string, err error, want connect.Code) {
	t.Helper()
	if got := connect.CodeOf(err); err == nil || got != want {
		t.Errorf("%s: err = %v, want code %v", what, err, want)
	}
}

func TestUserService(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	asAdmin := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(signUp(t, url)))

	lib := core.NewID().String()
	created, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("bob"), Password: new("pw1"),
		Policy: userv1.UserPolicy_builder{LibraryIds: []string{lib}, MaxParentalRating: new(int32(12))}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if pol := created.GetUser().GetPolicy(); pol.GetAllLibraries() || len(pol.GetLibraryIds()) != 1 || pol.GetLibraryIds()[0] != lib || pol.GetMaxParentalRating() != 12 {
		t.Errorf("created policy = %v", pol)
	}
	// An unset maximum rating is unrestricted; zero allows content for all
	// ages only.
	for name, max := range map[string]*int32{"dave": nil, "erin": new(int32(0))} {
		u, err := asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{
			Name: &name, Password: new("pw"), Policy: userv1.UserPolicy_builder{MaxParentalRating: max}.Build(),
		}.Build())
		if pol := u.GetUser().GetPolicy(); err != nil || pol.HasMaxParentalRating() != (max != nil) || pol.GetMaxParentalRating() != 0 {
			t.Errorf("%s's policy = %v, %v", name, pol, err)
		}
	}
	_, err = asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("BOB"), Password: new("x")}.Build())
	wantCode(t, "duplicate name", err, connect.CodeAlreadyExists)
	_, err = asAdmin.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("carol")}.Build())
	wantCode(t, "no password", err, connect.CodeInvalidArgument)
	if list, err := asAdmin.ListUsers(ctx, &userv1.ListUsersRequest{}); err != nil || len(list.GetUsers()) != 4 {
		t.Errorf("ListUsers = %v, %v", list, err)
	}

	// Bob is no administrator.
	phone, tv := login(t, url, "bob", "pw1", "phone"), login(t, url, "bob", "pw1", "tv")
	asBob := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(phone))
	_, err = asBob.ListUsers(ctx, &userv1.ListUsersRequest{})
	wantCode(t, "ListUsers as bob", err, connect.CodePermissionDenied)
	_, err = asBob.SetPassword(ctx, userv1.SetPasswordRequest_builder{UserId: new(created.GetUser().GetId()), NewPassword: new("x"), CurrentPassword: new("wrong")}.Build())
	wantCode(t, "wrong current password", err, connect.CodePermissionDenied)
	_, err = asBob.SetPassword(ctx, userv1.SetPasswordRequest_builder{UserId: new(created.GetUser().GetId() + ""), NewPassword: new("pw2"), CurrentPassword: new("pw1")}.Build())
	if err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	// The password change ends bob's other sessions.
	_, err = userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(tv)).GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
	wantCode(t, "other session after password change", err, connect.CodeUnauthenticated)
	if me, err := asBob.GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{}); err != nil || me.GetUser().GetName() != "bob" {
		t.Errorf("GetCurrentUser = %v, %v", me, err)
	}
	login(t, url, "bob", "pw2", "tv")

	prefs, err := asBob.UpdatePreferences(ctx, userv1.UpdatePreferencesRequest_builder{Preferences: userv1.UserPreferences_builder{
		AudioLanguages: []string{"jpn"}, SubtitleLanguages: []string{"eng"}, SubtitleMode: new(userv1.SubtitleMode_SUBTITLE_MODE_SMART),
	}.Build()}.Build())
	if err != nil || prefs.GetPreferences().GetSubtitleMode() != userv1.SubtitleMode_SUBTITLE_MODE_SMART || prefs.GetPreferences().GetAudioLanguages()[0] != "jpn" {
		t.Errorf("UpdatePreferences = %v, %v", prefs, err)
	}

	// The last enabled administrator stays.
	adminID := mustCurrent(t, asAdmin).GetId()
	_, err = asAdmin.UpdateUser(ctx, userv1.UpdateUserRequest_builder{Id: &adminID, Name: new("admin"), Admin: new(false)}.Build())
	wantCode(t, "demote the last admin", err, connect.CodeFailedPrecondition)
	_, err = asAdmin.DeleteUser(ctx, userv1.DeleteUserRequest_builder{Id: &adminID}.Build())
	wantCode(t, "delete the last admin", err, connect.CodeFailedPrecondition)
	updated, err := asAdmin.UpdateUser(ctx, userv1.UpdateUserRequest_builder{
		Id: new(created.GetUser().GetId()), Name: new("robert"), Admin: new(true),
		Policy: userv1.UserPolicy_builder{AllLibraries: new(true)}.Build(),
	}.Build())
	if err != nil || updated.GetUser().GetName() != "robert" || !updated.GetUser().GetAdmin() || !updated.GetUser().GetPolicy().GetAllLibraries() {
		t.Fatalf("UpdateUser = %v, %v", updated, err)
	}
	if _, err := asAdmin.DeleteUser(ctx, userv1.DeleteUserRequest_builder{Id: &adminID}.Build()); err != nil {
		t.Errorf("delete an admin while another remains: %v", err)
	}
}

func mustCurrent(t *testing.T, c userv1connect.UserServiceClient) *userv1.User {
	t.Helper()
	me, err := c.GetCurrentUser(t.Context(), &userv1.GetCurrentUserRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return me.GetUser()
}

func TestUserDataService(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	data := userv1connect.NewUserDataServiceClient(http.DefaultClient, url, withToken(signUp(t, url)))

	lib := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	if err := s.Libraries().Create(ctx, &lib); err != nil {
		t.Fatal(err)
	}
	series := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindSeries, Name: "Show"}
	season := core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: series.ID, Kind: core.KindSeason, Name: "Season 1"}
	ep1 := core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: season.ID, Kind: core.KindEpisode, Name: "Pilot", Runtime: time.Hour}
	ep2 := core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: season.ID, Kind: core.KindEpisode, Name: "Second", Runtime: time.Hour}
	if err := s.Items().Upsert(ctx, series, season, ep1, ep2); err != nil {
		t.Fatal(err)
	}
	update := func(id core.ID, b userv1.UpdateUserDataRequest_builder) *userv1.UserData {
		t.Helper()
		b.ItemId = new(id.String())
		resp, err := data.UpdateUserData(ctx, b.Build())
		if err != nil {
			t.Fatalf("UpdateUserData: %v", err)
		}
		return resp.GetUserData()
	}
	get := func(ids ...core.ID) map[string]*userv1.UserData {
		t.Helper()
		var strs []string
		for _, id := range ids {
			strs = append(strs, id.String())
		}
		resp, err := data.GetUserData(ctx, userv1.GetUserDataRequest_builder{ItemIds: strs}.Build())
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]*userv1.UserData{}
		for _, d := range resp.GetUserData() {
			out[d.GetItemId()] = d
		}
		return out
	}

	d := update(ep1.ID, userv1.UpdateUserDataRequest_builder{Position: durationpb.New(20 * time.Minute), Favorite: new(true), Rating: new(8.5)})
	if d.GetPosition().AsDuration() != 20*time.Minute || !d.GetFavorite() || d.GetRating() != 8.5 || d.GetPlayed() {
		t.Errorf("episode after update = %v", d)
	}

	// Marking the series played marks its episodes.
	update(series.ID, userv1.UpdateUserDataRequest_builder{Played: new(true)})
	got := get(ep1.ID, ep2.ID)
	for _, id := range []core.ID{ep1.ID, ep2.ID} {
		if d := got[id.String()]; !d.GetPlayed() || d.GetPlayCount() != 1 || d.HasPosition() || !d.HasLastPlayedTime() {
			t.Errorf("%s after marking the series = %v", id, d)
		}
	}
	if !got[ep1.ID.String()].GetFavorite() {
		t.Error("marking played cleared the favorite")
	}
	update(season.ID, userv1.UpdateUserDataRequest_builder{Played: new(false)})
	if d := get(ep2.ID)[ep2.ID.String()]; d.GetPlayed() || d.GetPlayCount() != 0 || d.HasLastPlayedTime() {
		t.Errorf("after unmarking the season = %v", d)
	}

	_, err := data.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: new(core.NewID().String()), Favorite: new(true)}.Build())
	wantCode(t, "unknown item", err, connect.CodeNotFound)
	_, err = data.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: new(ep1.ID.String()), Rating: new(11.0)}.Build())
	wantCode(t, "rating out of range", err, connect.CodeInvalidArgument)
}

func TestDisplayPreferencesService(t *testing.T) {
	ctx := t.Context()
	url := newServer(t)
	token := signUp(t, url)
	prefs := userv1connect.NewDisplayPreferencesServiceClient(http.DefaultClient, url, withToken(token))

	got, err := prefs.GetDisplayPreferences(ctx, userv1.GetDisplayPreferencesRequest_builder{Client: new("web"), View: new("home")}.Build())
	if err != nil || len(got.GetPreferences().GetValues()) != 0 || got.GetPreferences().HasUpdateTime() {
		t.Errorf("unset = %v, %v", got, err)
	}
	set, err := prefs.SetDisplayPreferences(ctx, userv1.SetDisplayPreferencesRequest_builder{
		Client: new("web"), View: new("home"), Values: map[string]string{"sections": "resume,nextup,latest"},
	}.Build())
	if err != nil || !set.GetPreferences().HasUpdateTime() {
		t.Fatalf("set = %v, %v", set, err)
	}
	// Another device of the same user reads them back.
	other := userv1connect.NewDisplayPreferencesServiceClient(http.DefaultClient, url, withToken(login(t, url, "admin", "secret", "phone")))
	got, err = other.GetDisplayPreferences(ctx, userv1.GetDisplayPreferencesRequest_builder{Client: new("web"), View: new("home")}.Build())
	if err != nil || got.GetPreferences().GetValues()["sections"] != "resume,nextup,latest" {
		t.Errorf("from another device = %v, %v", got, err)
	}
	_, err = prefs.SetDisplayPreferences(ctx, userv1.SetDisplayPreferencesRequest_builder{Client: new("web"), View: new("home"), Values: map[string]string{"": "x"}}.Build())
	wantCode(t, "empty name", err, connect.CodeInvalidArgument)
}

package httpserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
	"github.com/mavioai/mavio/libs/store"
)

// catalog holds a films and a shows library.
type catalog struct {
	films, shows                                      core.Library
	alien, up, trailer, series, season, pilot, finale core.Item
	ridley                                            core.Person
}

func seedCatalog(t *testing.T, s *store.Store) catalog {
	t.Helper()
	ctx := t.Context()
	var c catalog
	c.films = core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	c.shows = core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	for _, lib := range []*core.Library{&c.films, &c.shows} {
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	item := func(lib core.Library, parent core.ID, kind core.ItemKind, name string) core.Item {
		return core.Item{ID: core.NewID(), LibraryID: lib.ID, ParentID: parent, Kind: kind, Name: name, Path: lib.Paths[0] + "/" + name}
	}
	c.alien = item(c.films, core.NilID, core.KindMovie, "Alien")
	c.alien.ProductionYear, c.alien.OfficialRating, c.alien.ParentalRating, c.alien.Runtime = 1979, "R", new(17), 117*time.Minute
	c.alien.Genres, c.alien.ExternalIDs = []string{"Horror"}, map[core.Provider]string{core.ProviderTMDB: "348"}
	c.up = item(c.films, core.NilID, core.KindMovie, "Up")
	c.up.ProductionYear = 2009
	c.trailer = item(c.films, core.NilID, core.KindVideo, "Alien Trailer")
	c.trailer.Extra, c.trailer.OwnerID = core.ExtraTrailer, c.alien.ID
	c.series = item(c.shows, core.NilID, core.KindSeries, "Show")
	c.series.OfficialRating, c.series.ParentalRating = "TV-MA", new(17)
	c.season = item(c.shows, c.series.ID, core.KindSeason, "Season 1")
	c.pilot = item(c.shows, c.season.ID, core.KindEpisode, "Pilot")
	c.pilot.IndexNumber, c.pilot.ParentIndexNumber, c.pilot.OfficialRating, c.pilot.ParentalRating = new(1), new(1), "TV-Y7", new(7)
	c.pilot.ProductionYear, c.pilot.Genres = 2008, []string{"Comedy"}
	c.finale = item(c.shows, c.season.ID, core.KindEpisode, "Finale")
	c.finale.IndexNumber, c.finale.ParentIndexNumber = new(2), new(1)
	if err := s.Items().Upsert(ctx, c.alien, c.up, c.trailer, c.series, c.season, c.pilot, c.finale); err != nil {
		t.Fatal(err)
	}
	c.ridley = core.Person{ID: core.NewID(), Name: "Ridley Scott", ExternalIDs: map[core.Provider]string{core.ProviderIMDb: "nm0000631"}}
	if err := s.People().Upsert(ctx, c.ridley); err != nil {
		t.Fatal(err)
	}
	if err := s.People().ReplaceCredits(ctx, c.alien.ID, []core.Credit{{PersonID: c.ridley.ID, Kind: core.CreditDirector}}); err != nil {
		t.Fatal(err)
	}
	for owner, kind := range map[core.ID]core.ImageKind{c.alien.ID: core.ImagePrimary, c.ridley.ID: core.ImagePrimary} {
		if err := s.Images().Replace(ctx, owner, []core.Image{{Kind: kind, Path: "/art/" + owner.String() + ".jpg", Width: 400, Height: 600}}); err != nil {
			t.Fatal(err)
		}
	}
	src := core.MediaSource{
		ID: core.NewID(), ItemID: c.alien.ID, Path: c.alien.Path + ".mkv", Container: "mkv", Duration: 117 * time.Minute,
		Streams: []core.MediaStream{
			{
				Index: 0, Kind: core.StreamVideo, Codec: "hevc", Width: 3840, Height: 2160, ColorTransfer: "smpte2084", ColorPrimaries: "bt2020",
				FrameRate: core.Rational{Num: 24000, Den: 1001},
			},
			{Index: 1, Kind: core.StreamSubtitle, Codec: "hdmv_pgs_subtitle", Language: "eng"},
		},
	}
	if err := s.MediaSources().Replace(ctx, c.alien.ID, []core.MediaSource{src}); err != nil {
		t.Fatal(err)
	}
	return c
}

func names(items []*libraryv1.Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.GetName())
	}
	return out
}

func TestItemService(t *testing.T) {
	ctx := t.Context()
	url, s := startServer(t, nil)
	c := seedCatalog(t, s)
	token := signUp(t, url)
	asAdmin := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(token))

	got, err := asAdmin.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.alien.ID.String())}.Build())
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	it := got.GetItem()
	if it.GetName() != "Alien" || it.GetPath() == "" || it.GetOfficialRating() != "R" || it.GetExternalIds()["tmdb"] != "348" ||
		it.GetRuntime().AsDuration() != 117*time.Minute || len(it.GetImages()) != 1 || it.GetImages()[0].GetWidth() != 400 {
		t.Errorf("item = %v", it)
	}
	if u := it.GetExternalUrls(); len(u) != 1 || u[0].GetName() != "TMDB" || u[0].GetUrl() != "https://www.themoviedb.org/movie/348" {
		t.Errorf("external URLs = %v", u)
	}
	if cr := got.GetCredits(); len(cr) != 1 || cr[0].GetPerson().GetName() != "Ridley Scott" || cr[0].GetKind() != libraryv1.CreditKind_CREDIT_KIND_DIRECTOR ||
		len(cr[0].GetPerson().GetImages()) != 1 {
		t.Errorf("credits = %v", cr)
	}
	ms := got.GetMediaSources()
	if len(ms) != 1 || ms[0].GetPath() == "" || ms[0].GetStreams()[0].GetRange() != libraryv1.VideoRange_VIDEO_RANGE_HDR10 ||
		ms[0].GetStreams()[0].GetFrameRate().GetDen() != 1001 || ms[0].GetStreams()[1].GetTextBased() {
		t.Errorf("media sources = %v", ms)
	}

	list := func(c libraryv1connect.ItemServiceClient, b libraryv1.ListItemsRequest_builder) *libraryv1.ListItemsResponse {
		t.Helper()
		resp, err := c.ListItems(ctx, b.Build())
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		return resp
	}
	byName := []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_NAME)}.Build()}
	movie := []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}
	tests := []struct {
		name string
		req  libraryv1.ListItemsRequest_builder
		want []string
	}{
		{"movies", libraryv1.ListItemsRequest_builder{Kinds: movie, Sort: byName}, []string{"Alien", "Up"}},
		{"with extras", libraryv1.ListItemsRequest_builder{LibraryIds: []string{c.films.ID.String()}, IncludeExtras: new(true), Sort: byName}, []string{"Alien", "Alien Trailer", "Up"}},
		{"search", libraryv1.ListItemsRequest_builder{Search: new("alie")}, []string{"Alien"}},
		{"genre", libraryv1.ListItemsRequest_builder{Genres: []string{"Horror"}}, []string{"Alien"}},
		{"year", libraryv1.ListItemsRequest_builder{YearFrom: new(int32(2000)), Kinds: movie}, []string{"Up"}},
		{"person", libraryv1.ListItemsRequest_builder{PersonId: new(c.ridley.ID.String())}, []string{"Alien"}},
		{"descendants", libraryv1.ListItemsRequest_builder{ParentId: new(c.series.ID.String()), Recursive: new(true), Sort: byName}, []string{"Finale", "Pilot", "Season 1"}},
		{"top level", libraryv1.ListItemsRequest_builder{LibraryIds: []string{c.shows.ID.String()}, TopLevel: new(true)}, []string{"Show"}},
		{"page", libraryv1.ListItemsRequest_builder{Kinds: movie, Sort: byName, Limit: new(int32(1)), Offset: new(int32(1))}, []string{"Up"}},
	}
	for _, tt := range tests {
		if got := names(list(asAdmin, tt.req).GetItems()); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %q, want = %q", tt.name, got, tt.want)
		}
	}
	if resp := list(asAdmin, libraryv1.ListItemsRequest_builder{Kinds: movie, Limit: new(int32(1))}); resp.GetTotal() != 2 {
		t.Errorf("total = %d, want 2", resp.GetTotal())
	}
	_, err = asAdmin.ListItems(ctx, libraryv1.ListItemsRequest_builder{Recursive: new(true)}.Build())
	wantCode(t, "recursive without parent", err, connect.CodeInvalidArgument)
	_, err = asAdmin.ListItems(ctx, libraryv1.ListItemsRequest_builder{ParentId: new(c.series.ID.String()), TopLevel: new(true)}.Build())
	wantCode(t, "top level within a parent", err, connect.CodeInvalidArgument)

	// Per-user filters.
	data := userv1connect.NewUserDataServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := data.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: new(c.up.ID.String()), Played: new(true)}.Build()); err != nil {
		t.Fatal(err)
	}
	if got := names(list(asAdmin, libraryv1.ListItemsRequest_builder{Kinds: movie, Played: new(true)}).GetItems()); !slices.Equal(got, []string{"Up"}) {
		t.Errorf("played = %q", got)
	}

	// Next up follows the played pilot, with its series and season.
	if _, err := data.UpdateUserData(ctx, userv1.UpdateUserDataRequest_builder{ItemId: new(c.pilot.ID.String()), Played: new(true)}.Build()); err != nil {
		t.Fatal(err)
	}
	next, err := asAdmin.ListNextUp(ctx, libraryv1.ListNextUpRequest_builder{}.Build())
	if err != nil || len(next.GetItems()) != 1 {
		t.Fatalf("next up = %v, %v", next, err)
	}
	if ep := next.GetItems()[0]; ep.GetName() != "Finale" || ep.GetSeriesId() != c.series.ID.String() || ep.GetSeriesName() != "Show" ||
		ep.GetSeasonId() != c.season.ID.String() || ep.GetSeasonName() != "Season 1" {
		t.Errorf("next up = %v", ep)
	}
	// The latest items: the series stands for its two episodes, the
	// trailer is an extra.
	latest, err := asAdmin.ListLatestItems(ctx, libraryv1.ListLatestItemsRequest_builder{}.Build())
	if got := names(latest.GetItems()); err != nil || len(got) != 3 || !slices.Contains(got, "Show") || !slices.Contains(got, "Alien") || !slices.Contains(got, "Up") {
		t.Errorf("latest = %q, %v", got, err)
	}
	latest, err = asAdmin.ListLatestItems(ctx, libraryv1.ListLatestItemsRequest_builder{LibraryIds: []string{c.films.ID.String()}, Played: new(true)}.Build())
	if got := names(latest.GetItems()); err != nil || !slices.Equal(got, []string{"Up"}) {
		t.Errorf("played latest films = %q, %v", got, err)
	}

	// A user limited to shows rated up to 12 sees neither film library nor paths.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(token))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{
		Name: new("kid"), Password: new("pw"),
		Policy: userv1.UserPolicy_builder{LibraryIds: []string{c.shows.ID.String()}, MaxParentalRating: new(int32(12))}.Build(),
	}.Build()); err != nil {
		t.Fatal(err)
	}
	asKid := libraryv1connect.NewItemServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "tablet")))
	// The series is rated above the kid's limit.
	series := []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_SERIES}
	if got := list(asKid, libraryv1.ListItemsRequest_builder{Kinds: series}); len(got.GetItems()) != 0 {
		t.Errorf("kid's series = %q", names(got.GetItems()))
	}
	_, err = asKid.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.series.ID.String())}.Build())
	wantCode(t, "kid gets a rated series", err, connect.CodeNotFound)
	if got := list(asKid, libraryv1.ListItemsRequest_builder{LibraryIds: []string{c.films.ID.String()}}); len(got.GetItems()) != 0 {
		t.Errorf("kid's films = %q", names(got.GetItems()))
	}
	_, err = asKid.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.alien.ID.String())}.Build())
	wantCode(t, "kid gets a film", err, connect.CodeNotFound)
	// Unrated episodes take the series' rating; a rated one is allowed.
	_, err = asKid.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.finale.ID.String())}.Build())
	wantCode(t, "kid gets an unrated episode of a rated series", err, connect.CodeNotFound)
	episodes := []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}
	if got := names(list(asKid, libraryv1.ListItemsRequest_builder{Kinds: episodes}).GetItems()); !slices.Equal(got, []string{"Pilot"}) {
		t.Errorf("kid's episodes = %q", got)
	}
	pilot, err := asKid.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(c.pilot.ID.String())}.Build())
	if err != nil || pilot.GetItem().HasPath() || pilot.GetItem().GetIndexNumber() != 1 {
		t.Errorf("kid's pilot = %v, %v", pilot, err)
	}
	if p, err := asKid.GetPerson(ctx, libraryv1.GetPersonRequest_builder{Id: new(c.ridley.ID.String())}.Build()); err != nil || p.GetPerson().GetName() != "Ridley Scott" ||
		len(p.GetPerson().GetExternalUrls()) != 1 || p.GetPerson().GetExternalUrls()[0].GetUrl() != "https://www.imdb.com/name/nm0000631" {
		t.Errorf("GetPerson = %v, %v", p, err)
	}
	// Only administrators look for lyrics; without lyrics providers there
	// are none.
	_, err = asKid.SearchRemoteLyrics(ctx, libraryv1.SearchRemoteLyricsRequest_builder{ItemId: new(c.pilot.ID.String())}.Build())
	wantCode(t, "kid searches lyrics", err, connect.CodePermissionDenied)
	if got, err := asAdmin.SearchRemoteLyrics(ctx, libraryv1.SearchRemoteLyricsRequest_builder{ItemId: new(c.pilot.ID.String())}.Build()); err != nil || len(got.GetLyrics()) != 0 {
		t.Errorf("SearchRemoteLyrics = %v, %v", got, err)
	}
	_, err = asAdmin.DownloadRemoteLyrics(ctx, libraryv1.DownloadRemoteLyricsRequest_builder{
		ItemId: new(c.pilot.ID.String()), Provider: new("nobody"), Id: new("1"),
	}.Build())
	wantCode(t, "download lyrics from nobody", err, connect.CodeNotFound)

	// Without intro providers nothing plays first; items the kid may not
	// see have no intros either.
	playbacks := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, url, withToken(token))
	if got, err := playbacks.ListIntros(ctx, playbackv1.ListIntrosRequest_builder{ItemId: new(c.alien.ID.String())}.Build()); err != nil || len(got.GetItems()) != 0 {
		t.Errorf("ListIntros = %v, %v", got, err)
	}
	kidPlaybacks := playbackv1connect.NewPlaybackServiceClient(http.DefaultClient, url, withToken(login(t, url, "kid", "pw", "phone")))
	_, err = kidPlaybacks.ListIntros(ctx, playbackv1.ListIntrosRequest_builder{ItemId: new(c.alien.ID.String())}.Build())
	wantCode(t, "kid lists intros of a film", err, connect.CodeNotFound)

	// The kid's latest items: only the pilot, not its rated series.
	latest, err = asKid.ListLatestItems(ctx, libraryv1.ListLatestItemsRequest_builder{}.Build())
	if got := names(latest.GetItems()); err != nil || !slices.Equal(got, []string{"Pilot"}) {
		t.Errorf("kid's latest = %q, %v", got, err)
	}

	// Value and person lists count only what the caller may access.
	values := func(client libraryv1connect.ItemServiceClient, req *libraryv1.ListValuesRequest) []string {
		t.Helper()
		resp, err := client.ListValues(ctx, req)
		if err != nil {
			t.Fatalf("ListValues: %v", err)
		}
		out := []string{}
		for _, v := range resp.GetValues() {
			out = append(out, fmt.Sprintf("%s %d", v.GetValue(), v.GetItemCount()))
		}
		return out
	}
	people := func(client libraryv1connect.ItemServiceClient, req *libraryv1.ListPeopleRequest) []string {
		t.Helper()
		resp, err := client.ListPeople(ctx, req)
		if err != nil {
			t.Fatalf("ListPeople: %v", err)
		}
		out := []string{}
		for _, p := range resp.GetPeople() {
			out = append(out, fmt.Sprintf("%s %d %d", p.GetPerson().GetName(), p.GetItemCount(), len(p.GetPerson().GetImages())))
		}
		return out
	}
	genre, year := libraryv1.ValueKind_VALUE_KIND_GENRE, libraryv1.ValueKind_VALUE_KIND_YEAR
	director := []libraryv1.CreditKind{libraryv1.CreditKind_CREDIT_KIND_DIRECTOR}
	listTests := []struct {
		name string
		got  []string
		want []string
	}{
		{"admin genres", values(asAdmin, libraryv1.ListValuesRequest_builder{Kind: &genre}.Build()), []string{"Comedy 1", "Horror 1"}},
		{"admin film genres", values(asAdmin, libraryv1.ListValuesRequest_builder{Kind: &genre, LibraryIds: []string{c.films.ID.String()}}.Build()), []string{"Horror 1"}},
		{"admin years", values(asAdmin, libraryv1.ListValuesRequest_builder{Kind: &year}.Build()), []string{"1979 1", "2008 1", "2009 1"}},
		{"admin episode years", values(asAdmin, libraryv1.ListValuesRequest_builder{Kind: &year, ItemKinds: episodes}.Build()), []string{"2008 1"}},
		{"kid genres", values(asKid, libraryv1.ListValuesRequest_builder{Kind: &genre}.Build()), []string{"Comedy 1"}},
		{"kid film genres", values(asKid, libraryv1.ListValuesRequest_builder{Kind: &genre, LibraryIds: []string{c.films.ID.String()}}.Build()), []string{}},
		{"admin directors", people(asAdmin, libraryv1.ListPeopleRequest_builder{CreditKinds: director, Search: new("ridley")}.Build()), []string{"Ridley Scott 1 1"}},
		{"kid people", people(asKid, libraryv1.ListPeopleRequest_builder{}.Build()), []string{}},
	}
	for _, tt := range listTests {
		if !slices.Equal(tt.got, tt.want) {
			t.Errorf("%s: got = %q, want = %q", tt.name, tt.got, tt.want)
		}
	}
	_, err = asAdmin.ListValues(ctx, libraryv1.ListValuesRequest_builder{Kind: &year, Search: new("19")}.Build())
	wantCode(t, "search years", err, connect.CodeInvalidArgument)
	_, err = asAdmin.ListValues(ctx, libraryv1.ListValuesRequest_builder{}.Build())
	wantCode(t, "no value kind", err, connect.CodeInvalidArgument)
}

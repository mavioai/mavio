package main

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// fakeTMDB serves canned API responses by path and records the requests.
type fakeTMDB struct {
	mu       sync.Mutex
	requests []*http.Request
	routes   map[string]string
}

func (f *fakeTMDB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()
	if r.URL.Query().Get("api_key") != "key" && r.Header.Get("Authorization") != "Bearer a.b.c" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status_message":"Invalid API key: You must be granted a valid key."}`))
		return
	}
	body, ok := f.routes[r.URL.Path]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status_message":"The resource you requested could not be found."}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func (f *fakeTMDB) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		out = append(out, r.URL.Path)
	}
	return out
}

var routes = map[string]string{
	"/3/search/movie": `{"results":[
		{"id":10674,"title":"Mulan","original_title":"Mulan","release_date":"1998-06-18"},
		{"id":337401,"title":"Mulan","original_title":"Mulan","release_date":"2020-09-04","poster_path":"/mulan.jpg"},
		{"id":752662,"title":"Hua Mulan","original_title":"花木兰","release_date":"2020-12-01"}]}`,
	"/3/movie/337401": `{"id":337401,"title":"Mulan","original_title":"Mulan","overview":"A young woman disguises herself.",
		"tagline":"Loyal. Brave. True.","release_date":"2020-09-04","runtime":115,"vote_average":7.1,"imdb_id":"tt4566758",
		"genres":[{"name":"Adventure"},{"name":"Fantasy"}],"production_companies":[{"name":"Walt Disney Pictures"}],
		"belongs_to_collection":{"id":1166519,"name":"Mulan Collection"},
		"credits":{"cast":[
			{"id":2,"name":"Donnie Yen","character":"Commander Tung","order":1},
			{"id":1,"name":"Yifei Liu","character":"Mulan","order":0,"profile_path":"/liu.jpg"}],
		"crew":[
			{"id":3,"name":"Niki Caro","department":"Directing","job":"Director"},
			{"id":3,"name":"Niki Caro","department":"Directing","job":"Director"},
			{"id":4,"name":"Rick Jaffa","department":"Writing","job":"Screenplay"},
			{"id":5,"name":"Mandy Walker","department":"Camera","job":"Director of Photography"}]},
		"images":{
			"posters":[
				{"file_path":"/en.jpg","iso_639_1":"en","iso_3166_1":"US","vote_average":9},
				{"file_path":"/de.jpg","iso_639_1":"de","iso_3166_1":"DE","vote_average":5},
				{"file_path":"/none.jpg","iso_639_1":"xx","vote_average":7}],
			"backdrops":[
				{"file_path":"/bd-en.jpg","iso_639_1":"en","vote_average":8},
				{"file_path":"/bd.jpg","iso_639_1":null,"vote_average":6}]},
		"keywords":{"keywords":[{"name":"warrior"}]},
		"release_dates":{"results":[
			{"iso_3166_1":"US","release_dates":[{"certification":""},{"certification":"PG-13"}]},
			{"iso_3166_1":"DE","release_dates":[{"certification":"12"}]}]}}`,
	"/3/find/tt0120762": `{"movie_results":[{"id":10674}],"tv_results":[]}`,
	"/3/movie/10674":    `{"id":10674,"title":"Mulan","release_date":"1998-06-18"}`,
	"/3/find/81189":     `{"movie_results":[],"tv_results":[{"id":1396}]}`,
	"/3/tv/1396": `{"id":1396,"name":"Breaking Bad","original_name":"Breaking Bad","first_air_date":"2008-01-20",
		"last_air_date":"2013-09-29","status":"Ended","episode_run_time":[45,47],"vote_average":8.9,
		"genres":[{"name":"Drama"}],"networks":[{"name":"AMC"}],
		"created_by":[{"id":66633,"name":"Vince Gilligan","profile_path":"/vg.jpg"}],
		"aggregate_credits":{"cast":[{"id":17419,"name":"Bryan Cranston","order":0,"roles":[{"character":"Walter White","episode_count":62}]}]},
		"external_ids":{"imdb_id":"tt0903747","tvdb_id":81189},
		"keywords":{"results":[{"name":"drug dealer"}]},
		"content_ratings":{"results":[{"iso_3166_1":"US","rating":"TV-MA"}]}}`,
	"/3/tv/1396/season/1": `{"id":3572,"name":"Season 1","air_date":"2008-01-20","season_number":1,
		"images":{"posters":[{"file_path":"/s1.jpg","iso_639_1":"en"}]},"external_ids":{"tvdb_id":30272}}`,
	"/3/tv/1396/season/1/episode/2": `{"id":62086,"name":"Cat's in the Bag...","air_date":"2008-01-27","runtime":48,"vote_average":8.2,
		"credits":{"cast":[{"id":17419,"name":"Bryan Cranston","character":"Walter White","order":0}],
			"guest_stars":[{"id":1,"name":"Max Arciniega","character":"Krazy-8","order":0}],
			"crew":[{"id":2,"name":"Adam Bernstein","department":"Directing","job":"Director"}]},
		"images":{"stills":[{"file_path":"/still.jpg"}]},
		"external_ids":{"imdb_id":"tt1054724","tvdb_id":349232}}`,
}

func newTestPlugin(t *testing.T, cfg string) (*plugin, *fakeTMDB) {
	t.Helper()
	fake := &fakeTMDB{routes: routes}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	p := &plugin{baseURL: srv.URL + "/3"}
	if cfg != "" {
		if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(cfg)}.Build()); err != nil {
			t.Fatal(err)
		}
	}
	return p, fake
}

func lookup(kind pluginv1.MediaKind, name string, year int32, ids map[string]string) *pluginv1.Lookup {
	return pluginv1.Lookup_builder{Kind: kind.Enum(), Name: proto.String(name), Year: proto.Int32(year), ExternalIds: ids}.Build()
}

func metadataFor(t *testing.T, p *plugin, l *pluginv1.Lookup) *pluginv1.Metadata {
	t.Helper()
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if !resp.GetFound() {
		t.Fatal("GetMetadata() found nothing, want = metadata")
	}
	return resp.GetMetadata()
}

func TestManifest(t *testing.T) {
	p, _ := newTestPlugin(t, "")
	resp, err := p.Describe(t.Context(), &pluginv1.DescribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	m := resp.GetManifest()
	if err := manifest.Validate(m); err != nil {
		t.Errorf("Validate() = %v, want = nil", err)
	}
	if err := manifest.ValidateConfig(m, `{"api_key":"key","max_cast_members":5}`); err != nil {
		t.Errorf("ValidateConfig() = %v, want = nil", err)
	}
	if err := manifest.ValidateConfig(m, `{}`); err == nil {
		t.Error("ValidateConfig({}) = nil, want = an error for the missing key")
	}
}

func TestUnconfigured(t *testing.T) {
	p, _ := newTestPlugin(t, "")
	h, err := p.Health(t.Context(), &pluginv1.HealthRequest{})
	if err != nil || h.GetHealthy() {
		t.Errorf("Health() = %v, %v, want = unhealthy", h, err)
	}
	_, err = p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Mulan", 0, nil)}.Build())
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Errorf("GetMetadata() code = %v, want = %v", got, connect.CodeFailedPrecondition)
	}
}

func TestMovieBySearch(t *testing.T) {
	p, fake := newTestPlugin(t, `{"api_key":"key","max_cast_members":1}`)
	l := lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Mulan", 2020, nil)
	l.SetLanguage("de")
	l.SetCountry("DE")
	md := metadataFor(t, p, l)

	if got, want := fake.paths(), []string{"/3/search/movie", "/3/movie/337401"}; !slices.Equal(got, want) {
		t.Errorf("requests = %v, want = %v", got, want)
	}
	q := fake.requests[1].URL.Query()
	if got, want := q.Get("include_image_language"), "de,null,en"; got != want {
		t.Errorf("include_image_language = %q, want = %q", got, want)
	}
	if got, want := q.Get("language"), "de"; got != want {
		t.Errorf("language = %q, want = %q", got, want)
	}
	for _, c := range []struct{ name, got, want string }{
		{"name", md.GetName(), "Mulan"},
		{"premiere date", md.GetPremiereDate(), "2020-09-04"},
		{"official rating", md.GetOfficialRating(), "FSK-12"},
		{"tagline", md.GetTagline(), "Loyal. Brave. True."},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want = %q", c.name, c.got, c.want)
		}
	}
	if got := md.GetProductionYear(); got != 2020 {
		t.Errorf("production year = %d, want = 2020", got)
	}
	if got := md.GetRuntime().AsDuration(); got != 115*time.Minute {
		t.Errorf("runtime = %v, want = 1h55m", got)
	}
	wantIDs := map[string]string{"tmdb": "337401", "imdb": "tt4566758", "tmdb_collection": "1166519"}
	if got := md.GetExternalIds(); !maps.Equal(got, wantIDs) {
		t.Errorf("external IDs = %v, want = %v", got, wantIDs)
	}
	if got := md.GetCollectionName(); got != "Mulan Collection" {
		t.Errorf("collection = %q, want = Mulan Collection", got)
	}
	if got, want := md.GetGenres(), []string{"Adventure", "Fantasy"}; !slices.Equal(got, want) {
		t.Errorf("genres = %v, want = %v", got, want)
	}
	// One actor, the top billed; the director once; the camera crew not.
	var people []string
	for _, p := range md.GetPeople() {
		people = append(people, p.GetKind().String()+":"+p.GetName()+":"+p.GetRole())
	}
	wantPeople := []string{"CREDIT_KIND_ACTOR:Yifei Liu:Mulan", "CREDIT_KIND_DIRECTOR:Niki Caro:Director", "CREDIT_KIND_WRITER:Rick Jaffa:Screenplay"}
	if !slices.Equal(people, wantPeople) {
		t.Errorf("people = %v, want = %v", people, wantPeople)
	}
	if got, want := md.GetPeople()[0].GetImageUrl(), "https://image.tmdb.org/t/p/original/liu.jpg"; got != want {
		t.Errorf("profile image = %q, want = %q", got, want)
	}
	// German posters first, then those without text; backdrops without
	// text first.
	var imgs []string
	for _, img := range md.GetImages() {
		imgs = append(imgs, strings.TrimPrefix(img.GetUrl(), "https://image.tmdb.org/t/p/original")+"@"+img.GetLanguage())
	}
	wantImgs := []string{"/de.jpg@de", "/none.jpg@", "/en.jpg@en", "/bd.jpg@", "/bd-en.jpg@en"}
	if !slices.Equal(imgs, wantImgs) {
		t.Errorf("images = %v, want = %v", imgs, wantImgs)
	}
}

func TestMovieRatingFallsBackToUS(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"key"}`)
	l := lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "337401"})
	l.SetCountry("FR")
	if got := metadataFor(t, p, l).GetOfficialRating(); got != "PG-13" {
		t.Errorf("official rating = %q, want = PG-13", got)
	}
}

func TestMovieByIMDbID(t *testing.T) {
	p, fake := newTestPlugin(t, `{"api_key":"a.b.c"}`)
	md := metadataFor(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Mulan", 1998, map[string]string{"tmdb": "tt0120762", "imdb": "tt0120762"}))
	if got := md.GetExternalIds()["tmdb"]; got != "10674" {
		t.Errorf("tmdb ID = %q, want = 10674", got)
	}
	if got, want := fake.paths(), []string{"/3/find/tt0120762", "/3/movie/10674"}; !slices.Equal(got, want) {
		t.Errorf("requests = %v, want = %v", got, want)
	}
	// A read access token goes in the header, not the URL.
	if q := fake.requests[0].URL.Query(); q.Has("api_key") {
		t.Errorf("query = %v, want = no api_key", q)
	}
}

func TestSeries(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"key"}`)
	md := metadataFor(t, p, lookup(pluginv1.MediaKind_MEDIA_KIND_SERIES, "Breaking Bad", 2008, map[string]string{"tvdb": "81189"}))
	if got := md.GetSeriesStatus(); got != pluginv1.SeriesStatus_SERIES_STATUS_ENDED {
		t.Errorf("status = %v, want = ended", got)
	}
	if got := md.GetEndDate(); got != "2013-09-29" {
		t.Errorf("end date = %q, want = 2013-09-29", got)
	}
	if got := md.GetOfficialRating(); got != "TV-MA" {
		t.Errorf("official rating = %q, want = TV-MA", got)
	}
	if got := md.GetRuntime().AsDuration(); got != 45*time.Minute {
		t.Errorf("runtime = %v, want = 45m", got)
	}
	wantIDs := map[string]string{"tmdb": "1396", "imdb": "tt0903747", "tvdb": "81189"}
	if got := md.GetExternalIds(); !maps.Equal(got, wantIDs) {
		t.Errorf("external IDs = %v, want = %v", got, wantIDs)
	}
	var people []string
	for _, p := range md.GetPeople() {
		people = append(people, p.GetKind().String()+":"+p.GetName()+":"+p.GetRole())
	}
	if want := []string{"CREDIT_KIND_ACTOR:Bryan Cranston:Walter White", "CREDIT_KIND_CREATOR:Vince Gilligan:"}; !slices.Equal(people, want) {
		t.Errorf("people = %v, want = %v", people, want)
	}
}

func TestSeasonAndEpisode(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"key"}`)
	series := map[string]string{"tmdb": "1396"}

	season := pluginv1.Lookup_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_SEASON.Enum(), SeriesExternalIds: series, SeasonNumber: proto.Int32(1)}.Build()
	md := metadataFor(t, p, season)
	if got, want := md.GetName(), "Season 1"; got != want {
		t.Errorf("season name = %q, want = %q", got, want)
	}
	if got := md.GetExternalIds()["tvdb"]; got != "30272" {
		t.Errorf("season tvdb ID = %q, want = 30272", got)
	}

	episode := pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_EPISODE.Enum(), SeriesExternalIds: series,
		SeasonNumber: proto.Int32(1), EpisodeNumber: proto.Int32(2),
	}.Build()
	md = metadataFor(t, p, episode)
	if got, want := md.GetName(), "Cat's in the Bag..."; got != want {
		t.Errorf("episode name = %q, want = %q", got, want)
	}
	var kinds []pluginv1.CreditKind
	for _, p := range md.GetPeople() {
		kinds = append(kinds, p.GetKind())
	}
	wantKinds := []pluginv1.CreditKind{pluginv1.CreditKind_CREDIT_KIND_ACTOR, pluginv1.CreditKind_CREDIT_KIND_GUEST_STAR, pluginv1.CreditKind_CREDIT_KIND_DIRECTOR}
	if !slices.Equal(kinds, wantKinds) {
		t.Errorf("credit kinds = %v, want = %v", kinds, wantKinds)
	}
	if imgs := md.GetImages(); len(imgs) != 1 || imgs[0].GetKind() != pluginv1.ImageKind_IMAGE_KIND_PRIMARY {
		t.Errorf("images = %v, want = the still as primary image", imgs)
	}

	// Without the series' IDs an episode cannot be found.
	episode.SetSeriesExternalIds(nil)
	resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: episode}.Build())
	if err != nil || resp.GetFound() {
		t.Errorf("GetMetadata() = %v, %v, want = not found", resp, err)
	}
}

func TestNotFound(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"key"}`)
	for _, l := range []*pluginv1.Lookup{
		lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "999"}),
		lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "nm0000123"}),
		lookup(pluginv1.MediaKind_MEDIA_KIND_MUSIC_ALBUM, "Abbey Road", 1969, nil),
	} {
		resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
		if err != nil || resp.GetFound() {
			t.Errorf("GetMetadata(%v) = %v, %v, want = not found", l, resp, err)
		}
	}
}

func TestErrorsHideAPIKey(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"wrong-secret"}`)
	_, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "1"})}.Build())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("GetMetadata() = %v, want = an unauthorized error", err)
	}
	if strings.Contains(err.Error(), "wrong-secret") {
		t.Errorf("error %q contains the API key", err)
	}

	// Transport errors carry the URL, which must not leak either.
	p.baseURL = "http://127.0.0.1:1/3"
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = p.GetMetadata(ctx, pluginv1.GetMetadataRequest_builder{Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "", 0, map[string]string{"tmdb": "1"})}.Build())
	if err == nil || strings.Contains(err.Error(), "wrong-secret") {
		t.Errorf("GetMetadata() = %v, want = an error without the API key", err)
	}
}

func TestSearch(t *testing.T) {
	p, _ := newTestPlugin(t, `{"api_key":"key"}`)
	resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{
		Lookup: lookup(pluginv1.MediaKind_MEDIA_KIND_MOVIE, "Mulan", 2020, nil),
		Limit:  proto.Int32(2),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range resp.GetResults() {
		got = append(got, r.GetExternalIds()["tmdb"])
	}
	if want := []string{"337401", "10674"}; !slices.Equal(got, want) {
		t.Errorf("results = %v, want = %v", got, want)
	}
	if r := resp.GetResults()[0]; r.GetScore() != 1 || r.GetYear() != 2020 || r.GetImageUrl() != "https://image.tmdb.org/t/p/original/mulan.jpg" {
		t.Errorf("best result = %v, want = score 1, year 2020 and a poster", r)
	}
}

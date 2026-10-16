package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// The Rotten Tomatoes search's hits, by type and query.
var hits = map[string]string{
	`movie Dark Knight`: `[
		{"title":"The Dark Knight Rises","releaseYear":2012,"vanity":"the_dark_knight_rises","rottenTomatoes":{"criticsScore":87,"audienceScore":90}},
		{"title":"The Dark Knight","releaseYear":2008,"vanity":"the_dark_knight","rottenTomatoes":{"criticsScore":94,"audienceScore":94,"certifiedFresh":true}}]`,
	`movie Nosferatu`: `[
		{"title":"Nosferatu","releaseYear":1922,"vanity":"nosferatu","rottenTomatoes":{"criticsScore":97,"audienceScore":88}},
		{"title":"Nosferatu","releaseYear":2024,"vanity":"nosferatu_2024","rottenTomatoes":{"audienceScore":72}}]`,
	`tv Stranger Things`: `[{"title":"Stranger Things","releaseYear":2016,"vanity":"stranger_things","rottenTomatoes":{"criticsScore":91,"audienceScore":89,"certifiedFresh":true}}]`,
	`movie Unscored`:     `[{"title":"Unscored","releaseYear":2020,"vanity":"unscored"}]`,
	`movie Elsewhere`:    `[{"title":"Somewhere Else Entirely","releaseYear":2020,"vanity":"x","rottenTomatoes":{"criticsScore":50}}]`,
}

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rt/queries", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Algolia-Api-Key") != algoliaAPIKey {
			t.Errorf("search without the key")
		}
		var body struct {
			Requests []struct{ Query, Params string } `json:"requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Requests) != 1 {
			t.Fatalf("search body: %v", err)
		}
		q := body.Requests[0]
		kind := "movie"
		if strings.Contains(q.Params, "%22tv%22") {
			kind = "tv"
		}
		h, ok := hits[kind+" "+q.Query]
		if !ok {
			h = "[]"
		}
		fmt.Fprintf(w, `{"results":[{"index":"content_rt","hits":%s}]}`, h)
	})
	mux.HandleFunc("GET /radarr/movie/imdb/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("id") {
		case "tt0468569":
			_, _ = w.Write([]byte(`[{"ImdbId":"tt0468569","MovieRatings":{"Imdb":{"Count":3238161,"Value":9.1}}}]`))
		case "tt0000001":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &plugin{http: srv.Client(), algolia: srv.URL + "/rt", radarr: srv.URL + "/radarr"}
}

func TestGetRatings(t *testing.T) {
	p := newTestPlugin(t)
	tests := []struct {
		name    string
		kind    pluginv1.MediaKind
		title   string
		year    int32
		imdb    string
		want    []string
		wantErr bool
	}{
		{
			name: "movie", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, title: "The Dark Knight", year: 2008, imdb: "tt0468569",
			want: []string{
				"ROTTEN_TOMATOES_CRITICS 94 0 certified https://www.rottentomatoes.com/m/the_dark_knight",
				"ROTTEN_TOMATOES_AUDIENCE 94 0 https://www.rottentomatoes.com/m/the_dark_knight",
				"IMDB 9.1 3238161 https://www.imdb.com/title/tt0468569",
			},
		},
		{
			name: "by year", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, title: "Nosferatu", year: 2024,
			want: []string{"ROTTEN_TOMATOES_AUDIENCE 72 0 https://www.rottentomatoes.com/m/nosferatu_2024"},
		},
		{
			name: "series", kind: pluginv1.MediaKind_MEDIA_KIND_SERIES, title: "Stranger Things", year: 2016, imdb: "tt4574334",
			want: []string{
				"ROTTEN_TOMATOES_CRITICS 91 0 https://www.rottentomatoes.com/tv/stranger_things",
				"ROTTEN_TOMATOES_AUDIENCE 89 0 https://www.rottentomatoes.com/tv/stranger_things",
			},
		},
		{name: "unscored", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, title: "Unscored", year: 2020},
		{name: "no match", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, title: "Elsewhere", year: 2020},
		{name: "unknown to Radarr", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, imdb: "tt9999999"},
		{
			name: "one service failing", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, title: "Stranger Things", imdb: "tt0000001",
		},
		{name: "every service failing", kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE, imdb: "tt0000001", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := map[string]string{}
			if tt.imdb != "" {
				ids["imdb"] = tt.imdb
			}
			resp, err := p.GetRatings(t.Context(), pluginv1.GetRatingsRequest_builder{
				MediaKind: tt.kind.Enum(), ExternalIds: ids, Name: proto.String(tt.title), Year: proto.Int32(tt.year),
			}.Build())
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error %v", err, tt.wantErr)
			}
			var got []string
			for _, r := range resp.GetRatings() {
				s := fmt.Sprintf("%s %v %d", strings.TrimPrefix(r.GetSource().String(), "RATING_SOURCE_"), r.GetScore(), r.GetVotes())
				if r.GetCertified() {
					s += " certified"
				}
				got = append(got, s+" "+r.GetUrl())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestJaro(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"martha", "marhta", 0.944},
		{"dixon", "dicksonx", 0.767},
		{"abc", "xyz", 0},
		{"", "", 1},
		{"a", "", 0},
	}
	for _, tt := range tests {
		if got := jaro(tt.a, tt.b); got < tt.want-0.001 || got > tt.want+0.001 {
			t.Errorf("jaro(%q, %q) = %.3f, want = %.3f", tt.a, tt.b, got, tt.want)
		}
	}
}

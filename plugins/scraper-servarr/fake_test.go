package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// request is a request the fake services received.
type request struct {
	method, path, query, userAgent, body string
}

// fakeServarr serves canned answers of Servarr's services by method, path
// and query, else by method and path, under the prefixes /radarr, /skyhook, /services and /xem, and
// records the requests.
type fakeServarr struct {
	mu       sync.Mutex
	requests []request
	routes   map[string]string
}

func (f *fakeServarr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, request{r.Method, r.URL.Path, r.URL.RawQuery, r.UserAgent(), string(body)})
	answer, ok := f.routes[r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery]
	if !ok {
		answer, ok = f.routes[r.Method+" "+r.URL.Path]
	}
	f.mu.Unlock()
	switch {
	case !ok:
		http.NotFound(w, r)
	case strings.HasPrefix(answer, "!"):
		http.Error(w, answer[1:], http.StatusServiceUnavailable)
	default:
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	}
}

func (f *fakeServarr) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		out = append(out, r.method+" "+r.path)
	}
	return out
}

func (f *fakeServarr) last() request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

// newTestPlugin returns a plugin calling a fake of Servarr's services
// with the routes, at a fixed time.
func newTestPlugin(t *testing.T, routes map[string]string) (*plugin, *fakeServarr) {
	t.Helper()
	f := &fakeServarr{routes: routes}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p := newPlugin()
	p.radarr.baseURL = srv.URL + "/radarr"
	p.skyHook.baseURL = srv.URL + "/skyhook"
	p.services.baseURL = srv.URL + "/services"
	p.xem.baseURL = srv.URL + "/xem"
	p.now = func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }
	return p, f
}

const matrix = `{
  "TmdbId": 603, "ImdbId": "tt0133093", "Title": "The Matrix", "OriginalTitle": "The Matrix", "OriginalLanguage": "en",
  "Overview": "A hacker learns the truth.", "Runtime": 136, "Popularity": 47.2, "Year": 1999,
  "Premier": "1998-12-24T00:00:00Z", "InCinema": "1999-03-31T00:00:00Z", "PhysicalRelease": "1999-11-25T00:00:00Z",
  "DigitalRelease": "1999-09-21T00:00:00Z",
  "Images": [
    {"CoverType": "Poster", "Url": "https://image.tmdb.org/t/p/original/poster.jpg"},
    {"CoverType": "Fanart", "Url": "https://image.tmdb.org/t/p/original/fanart.jpg"},
    {"CoverType": "Clearlogo", "Url": "https://image.tmdb.org/t/p/original/logo.png"},
    {"CoverType": "Unknown", "Url": "https://image.tmdb.org/t/p/original/other.jpg"}],
  "Ratings": [{"Count": 28914, "Value": 8.1, "Origin": "Tmdb", "Type": "User"}],
  "MovieRatings": {"Tmdb": {"Count": 28914, "Value": 8.261, "Type": "User"}, "RottenTomatoes": {"Count": 0, "Value": 83, "Type": "User"}},
  "Genres": ["Action", "Science Fiction"], "Keywords": ["cyberpunk", "simulated reality "],
  "Studio": "Village Roadshow Pictures",
  "Certifications": [{"Country": "AU", "Certification": "M"}, {"Country": "US", "Certification": "R"}, {"Country": "DE", "Certification": "16"}],
  "Collection": {"TmdbId": 2344, "Name": "The Matrix Collection"},
  "Credits": {
    "Cast": [
      {"Name": "Keanu Reeves", "Order": 0, "Character": "Neo", "TmdbId": 6384, "Images": [{"CoverType": "Headshot", "Url": "https://image.tmdb.org/t/p/original/keanu.jpg"}]},
      {"Name": " ", "Order": 1, "Character": "Nobody", "TmdbId": 1, "Images": []}],
    "Crew": [
      {"Name": "Lana Wachowski", "Job": "Director", "Department": "Directing", "TmdbId": 9340, "Images": []},
      {"Name": "Lana Wachowski", "Job": "Director", "Department": "Directing", "TmdbId": 9340, "Images": []},
      {"Name": "Lana Wachowski", "Job": "Writer", "Department": "Writing", "TmdbId": 9340, "Images": []},
      {"Name": "Joel Silver", "Job": "Producer", "Department": "Production", "TmdbId": 1271, "Images": []},
      {"Name": "Bill Pope", "Job": "Director of Photography", "Department": "Camera", "TmdbId": 1272, "Images": []}]},
  "AlternativeTitles": [{"Title": "Matrix", "Type": "Tmdb", "Language": "de"}],
  "Translations": [
    {"Title": "The Matrix", "Overview": "A hacker learns the truth.", "Language": "en-US"},
    {"Title": "Matrix", "Overview": "Ein Hacker erfährt die Wahrheit.", "Language": "de-DE"},
    {"Title": "Matrix (AT)", "Overview": "", "Language": "de-AT"}],
  "Recommendations": [{"TmdbId": 604, "Title": "The Matrix Reloaded"}, {"TmdbId": 605, "Title": "The Matrix Revolutions"}]
}`

const breakingBad = `{
  "tvdbId": 81189, "title": "Breaking Bad", "overview": "A teacher turns to crime.",
  "firstAired": "2008-01-20", "lastAired": "2013-09-29", "tvRageId": 18164, "tvMazeId": 169, "tmdbId": 1396, "imdbId": "tt0903747",
  "status": "Ended", "runtime": 48, "network": "AMC", "originalLanguage": "eng", "genres": ["Crime", "Drama"],
  "contentRating": "tv-ma", "rating": {"count": 2688063, "value": "9.5"},
  "alternativeTitles": [{"title": "Breaking Bad: Original Minisodes"}],
  "actors": [
    {"name": "Bryan Cranston", "character": "Walter White", "image": "https://artworks.thetvdb.com/banners/person/1.jpg"},
    {"name": "Aaron Paul", "character": "Jesse Pinkman"}],
  "images": [
    {"coverType": "Banner", "url": "https://artworks.thetvdb.com/banners/graphical/81189-g21.jpg"},
    {"coverType": "Poster", "url": "https://artworks.thetvdb.com/banners/posters/81189-10.jpg"}],
  "seasons": [
    {"seasonNumber": 0, "images": [{"coverType": "Poster", "url": "https://artworks.thetvdb.com/banners/seasons/81189-0.jpg"}]},
    {"seasonNumber": 1, "images": [{"coverType": "Poster", "url": "https://artworks.thetvdb.com/banners/seasons/81189-1.jpg"}]}],
  "episodes": [
    {"tvdbId": 3859781, "seasonNumber": 0, "episodeNumber": 1, "title": "Good Cop / Bad Cop", "airDate": "2009-02-17", "runtime": 3},
    {"tvdbId": 349232, "seasonNumber": 1, "episodeNumber": 1, "absoluteEpisodeNumber": 1, "title": "Pilot", "airDate": "2008-01-20",
     "runtime": 58, "overview": "Walter is diagnosed.", "image": "https://artworks.thetvdb.com/banners/episodes/81189/349232.jpg", "rating": {"count": 10, "value": 8.2}},
    {"tvdbId": 349235, "seasonNumber": 1, "episodeNumber": 2, "absoluteEpisodeNumber": 2, "title": "Cat's in the Bag...", "airDate": "2008-01-27"},
    {"tvdbId": 349236, "seasonNumber": 2, "episodeNumber": 1, "absoluteEpisodeNumber": 8, "title": "Seven Thirty-Seven", "airDate": "2009-03-08"}]
}`

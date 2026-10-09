package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// defaultBaseURL is TMDB's API v3.
const defaultBaseURL = "https://api.themoviedb.org/3"

// imageBaseURL serves TMDB's images; sizes are path segments.
const imageBaseURL = "https://image.tmdb.org/t/p/"

// errNotFound is TMDB's answer for unknown IDs.
var errNotFound = errors.New("tmdb: not found")

// client calls the TMDB API.
type client struct {
	http    *http.Client
	baseURL string
	// key is an API key, sent as a query parameter, or an API read access
	// token, a JWT sent as a bearer token.
	key string
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	if query == nil {
		query = url.Values{}
	}
	bearer := strings.Count(c.key, ".") == 2
	if !bearer {
		query.Set("api_key", c.key)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if bearer {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// Do's error carries the URL, and with it the API key.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode != http.StatusOK:
		var body struct {
			Message string `json:"status_message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body)
		return fmt.Errorf("tmdb %s: %s: %s", path, resp.Status, body.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	return nil
}

func (c *client) searchMovies(ctx context.Context, name string, year int, language string, adult bool) ([]tmdbSearchMovie, error) {
	q := url.Values{"query": {name}, "include_adult": {strconv.FormatBool(adult)}}
	setLanguage(q, language)
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}
	var page struct {
		Results []tmdbSearchMovie `json:"results"`
	}
	return page.Results, c.get(ctx, "/search/movie", q, &page)
}

func (c *client) searchSeries(ctx context.Context, name string, year int, language string, adult bool) ([]tmdbSearchSeries, error) {
	q := url.Values{"query": {name}, "include_adult": {strconv.FormatBool(adult)}}
	setLanguage(q, language)
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	var page struct {
		Results []tmdbSearchSeries `json:"results"`
	}
	return page.Results, c.get(ctx, "/search/tv", q, &page)
}

// find looks up TMDB entries by another provider's ID; source is
// "imdb_id" or "tvdb_id".
func (c *client) find(ctx context.Context, id, source string) (tmdbFind, error) {
	var f tmdbFind
	err := c.get(ctx, "/find/"+url.PathEscape(id), url.Values{"external_source": {source}}, &f)
	return f, err
}

func (c *client) movie(ctx context.Context, id int, language, country string) (tmdbMovie, error) {
	var m tmdbMovie
	q := detailsQuery(language, country, "credits,images,keywords,release_dates")
	return m, c.get(ctx, "/movie/"+strconv.Itoa(id), q, &m)
}

func (c *client) series(ctx context.Context, id int, language, country string) (tmdbSeries, error) {
	var s tmdbSeries
	q := detailsQuery(language, country, "aggregate_credits,images,keywords,external_ids,content_ratings")
	return s, c.get(ctx, "/tv/"+strconv.Itoa(id), q, &s)
}

func (c *client) season(ctx context.Context, seriesID, season int, language, country string) (tmdbSeason, error) {
	var s tmdbSeason
	q := detailsQuery(language, country, "credits,images,external_ids")
	return s, c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", seriesID, season), q, &s)
}

func (c *client) episode(ctx context.Context, seriesID, season, episode int, language, country string) (tmdbEpisode, error) {
	var e tmdbEpisode
	q := detailsQuery(language, country, "credits,images,external_ids")
	return e, c.get(ctx, fmt.Sprintf("/tv/%d/season/%d/episode/%d", seriesID, season, episode), q, &e)
}

func setLanguage(q url.Values, language string) {
	if language != "" {
		q.Set("language", language)
	}
}

func detailsQuery(language, country, appends string) url.Values {
	q := url.Values{"append_to_response": {appends}, "include_image_language": {imageLanguages(language, country)}}
	setLanguage(q, language)
	return q
}

// The subset of TMDB's responses the plugin uses.
type (
	tmdbSearchMovie struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		ReleaseDate   string  `json:"release_date"`
		Overview      string  `json:"overview"`
		PosterPath    string  `json:"poster_path"`
		Popularity    float64 `json:"popularity"`
	}
	tmdbSearchSeries struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		FirstAirDate string `json:"first_air_date"`
		Overview     string `json:"overview"`
		PosterPath   string `json:"poster_path"`
	}
	tmdbFind struct {
		MovieResults []tmdbSearchMovie  `json:"movie_results"`
		TVResults    []tmdbSearchSeries `json:"tv_results"`
	}

	tmdbName struct {
		Name string `json:"name"`
	}
	tmdbCast struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Character   string `json:"character"`
		Order       int    `json:"order"`
		ProfilePath string `json:"profile_path"`
	}
	tmdbRole struct {
		Character    string `json:"character"`
		EpisodeCount int    `json:"episode_count"`
	}
	tmdbAggregateCast struct {
		ID          int        `json:"id"`
		Name        string     `json:"name"`
		Order       int        `json:"order"`
		ProfilePath string     `json:"profile_path"`
		Roles       []tmdbRole `json:"roles"`
	}
	tmdbCrew struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Department  string `json:"department"`
		Job         string `json:"job"`
		ProfilePath string `json:"profile_path"`
	}
	tmdbCredits struct {
		Cast       []tmdbCast `json:"cast"`
		GuestStars []tmdbCast `json:"guest_stars"`
		Crew       []tmdbCrew `json:"crew"`
	}
	tmdbImage struct {
		FilePath    string  `json:"file_path"`
		Width       int     `json:"width"`
		Height      int     `json:"height"`
		Language    string  `json:"iso_639_1"`
		Region      string  `json:"iso_3166_1"`
		VoteAverage float64 `json:"vote_average"`
	}
	tmdbImages struct {
		Posters   []tmdbImage `json:"posters"`
		Backdrops []tmdbImage `json:"backdrops"`
		Logos     []tmdbImage `json:"logos"`
		Stills    []tmdbImage `json:"stills"`
	}
	tmdbExternalIDs struct {
		IMDbID string `json:"imdb_id"`
		TVDBID int    `json:"tvdb_id"`
	}

	tmdbMovie struct {
		ID            int        `json:"id"`
		Title         string     `json:"title"`
		OriginalTitle string     `json:"original_title"`
		Overview      string     `json:"overview"`
		Tagline       string     `json:"tagline"`
		ReleaseDate   string     `json:"release_date"`
		Runtime       int        `json:"runtime"`
		VoteAverage   float64    `json:"vote_average"`
		IMDbID        string     `json:"imdb_id"`
		Genres        []tmdbName `json:"genres"`
		Companies     []tmdbName `json:"production_companies"`
		Collection    *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"belongs_to_collection"`
		Credits  tmdbCredits `json:"credits"`
		Images   tmdbImages  `json:"images"`
		Keywords struct {
			Keywords []tmdbName `json:"keywords"`
		} `json:"keywords"`
		ReleaseDates struct {
			Results []struct {
				Country string `json:"iso_3166_1"`
				Dates   []struct {
					Certification string `json:"certification"`
				} `json:"release_dates"`
			} `json:"results"`
		} `json:"release_dates"`
	}

	tmdbSeries struct {
		ID             int        `json:"id"`
		Name           string     `json:"name"`
		OriginalName   string     `json:"original_name"`
		Overview       string     `json:"overview"`
		Tagline        string     `json:"tagline"`
		FirstAirDate   string     `json:"first_air_date"`
		LastAirDate    string     `json:"last_air_date"`
		Status         string     `json:"status"`
		EpisodeRunTime []int      `json:"episode_run_time"`
		VoteAverage    float64    `json:"vote_average"`
		Genres         []tmdbName `json:"genres"`
		Networks       []tmdbName `json:"networks"`
		CreatedBy      []tmdbCast `json:"created_by"`
		Credits        struct {
			Cast []tmdbAggregateCast `json:"cast"`
			Crew []tmdbCrew          `json:"crew"`
		} `json:"aggregate_credits"`
		Images      tmdbImages      `json:"images"`
		ExternalIDs tmdbExternalIDs `json:"external_ids"`
		Keywords    struct {
			Results []tmdbName `json:"results"`
		} `json:"keywords"`
		ContentRatings struct {
			Results []struct {
				Country string `json:"iso_3166_1"`
				Rating  string `json:"rating"`
			} `json:"results"`
		} `json:"content_ratings"`
	}

	tmdbSeason struct {
		ID           int             `json:"id"`
		Name         string          `json:"name"`
		Overview     string          `json:"overview"`
		AirDate      string          `json:"air_date"`
		SeasonNumber int             `json:"season_number"`
		Credits      tmdbCredits     `json:"credits"`
		Images       tmdbImages      `json:"images"`
		ExternalIDs  tmdbExternalIDs `json:"external_ids"`
	}

	tmdbEpisode struct {
		ID          int             `json:"id"`
		Name        string          `json:"name"`
		Overview    string          `json:"overview"`
		AirDate     string          `json:"air_date"`
		Runtime     int             `json:"runtime"`
		VoteAverage float64         `json:"vote_average"`
		Credits     tmdbCredits     `json:"credits"`
		Images      tmdbImages      `json:"images"`
		ExternalIDs tmdbExternalIDs `json:"external_ids"`
	}
)

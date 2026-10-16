package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// errNotFound is a service's answer for a title it does not know.
var errNotFound = errors.New("not found")

// imdb tells a movie's IMDb rating, by its IMDb ID, from Radarr's metadata
// service, as Seerr does; none when the service does not know it.
func (p *plugin) imdb(ctx context.Context, id string) ([]*pluginv1.Rating, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.radarr+"/movie/imdb/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	var movies []struct {
		ImdbID       string `json:"ImdbId"`
		MovieRatings struct {
			Imdb *struct {
				Count int64   `json:"Count"`
				Value float64 `json:"Value"`
			} `json:"Imdb"`
		} `json:"MovieRatings"`
	}
	switch err := p.do(req, "Radarr's metadata service", &movies); {
	case errors.Is(err, errNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	}
	if len(movies) == 0 || movies[0].ImdbID != id || movies[0].MovieRatings.Imdb == nil || movies[0].MovieRatings.Imdb.Value <= 0 {
		return nil, nil
	}
	r := movies[0].MovieRatings.Imdb
	return []*pluginv1.Rating{pluginv1.Rating_builder{
		Source: pluginv1.RatingSource_RATING_SOURCE_IMDB.Enum(), Score: proto.Float64(r.Value), Votes: proto.Int64(r.Count),
		Url: proto.String("https://www.imdb.com/title/" + id),
	}.Build()}, nil
}

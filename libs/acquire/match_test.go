package acquire

import (
	"errors"
	"testing"

	"github.com/mavioai/mavio/libs/release"
)

func TestMatchMovie(t *testing.T) {
	movies := []MovieRef{
		{ID: tid(1), Title: "The Matrix", Year: 1999, ImdbID: "tt0133093", TmdbID: 603},
		{ID: tid(2), Title: "Rocky II", Year: 1979, TmdbID: 1367},
		{ID: tid(3), Title: "Amélie", OriginalTitle: "Le Fabuleux Destin d'Amélie Poulain", Year: 2001},
		{ID: tid(4), Title: "Twin", Year: 2010},
		{ID: tid(5), Title: "Twin", Year: 2010},
		{
			ID: tid(6), Title: "Crouching Tiger, Hidden Dragon", Year: 2000, SecondaryYear: 2001, TmdbID: 146,
			AlternativeTitles: []string{"Wo Hu Cang Long"}, TranslatedTitles: []string{"Tiger and Dragon"},
		},
		{ID: tid(7), Title: "Spirited Away", Year: 2001, TranslatedTitles: []string{"Chihiros Reise ins Zauberland"}},
	}
	tests := []struct {
		name     string
		title    string
		imdb     string
		tmdb     int
		searched int // index of the searched movie, or -1
		want     int // index, or -1
		wantErr  error
	}{
		{"by imdb id", "Something.Else.1999.1080p.BluRay.x264-GRP", "tt0133093", 0, -1, 0, nil},
		{"imdb id without tt", "Something.Else.1999.1080p.BluRay.x264-GRP", "0133093", 0, -1, 0, nil},
		{"imdb id with wrong year", "Something.Else.2005.1080p.BluRay.x264-GRP", "tt0133093", 0, -1, -1, nil},
		{"by tmdb id", "Whatever.1999.720p.WEB-DL-GRP", "", 603, -1, 0, nil},
		{"by title and year", "The.Matrix.1999.1080p.BluRay.x264-GRP", "", 0, -1, 0, nil},
		{"title with wrong year", "The.Matrix.2003.1080p.BluRay.x264-GRP", "", 0, -1, -1, nil},
		{"roman numerals", "Rocky.2.1979.1080p.BluRay.x264-GRP", "", 0, -1, 1, nil},
		{"original title", "Le.Fabuleux.Destin.d'Amelie.Poulain.2001.1080p.BluRay-GRP", "", 0, -1, 2, nil},
		{"unknown title", "Another.Movie.2020.1080p.WEB-DL-GRP", "", 0, -1, -1, nil},
		{"ambiguous", "Twin.2010.1080p.WEB-DL-GRP", "", 0, -1, -1, ErrMultipleMovies},
		{"search by title", "The.Matrix.1999.1080p.BluRay.x264-GRP", "", 0, 0, 0, nil},
		{"search with roman numerals", "Rocky.2.1979.1080p.BluRay.x264-GRP", "", 0, 1, 1, nil},
		{"search for another title", "Another.Movie.2020.1080p.WEB-DL-GRP", "", 0, 0, -1, nil},
		{"search with wrong year, matching id", "Rocky.II.1985.1080p.BluRay-GRP", "", 1367, 1, 1, nil},
		{"search with wrong year", "Rocky.II.1985.1080p.BluRay-GRP", "", 0, 1, -1, nil},
		{"secondary year", "Crouching.Tiger.Hidden.Dragon.2001.1080p.BluRay-GRP", "", 0, -1, 5, nil},
		{"tmdb id with secondary year", "Whatever.2001.720p.WEB-DL-GRP", "", 146, -1, 5, nil},
		{"alternative title", "Wo.Hu.Cang.Long.2000.1080p.BluRay-GRP", "", 0, -1, 5, nil},
		{"alternative title with secondary year", "Wo.Hu.Cang.Long.2001.1080p.BluRay-GRP", "", 0, -1, 5, nil},
		{"alternative title with wrong year", "Wo.Hu.Cang.Long.2005.1080p.BluRay-GRP", "", 0, -1, -1, nil},
		{"translation", "Chihiros.Reise.ins.Zauberland.2001.German.1080p.BluRay-GRP", "", 0, -1, 6, nil},
		{"search by alternative title", "Wo.Hu.Cang.Long.2000.1080p.BluRay-GRP", "", 0, 5, 5, nil},
		{"search by translation", "Tiger.and.Dragon.2001.German.1080p.BluRay-GRP", "", 0, 5, 5, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := release.ParseMovieTitle(tt.title, false)
			if parsed == nil {
				t.Fatalf("ParseMovieTitle(%q) = nil", tt.title)
			}
			var searched *MovieRef
			if tt.searched >= 0 {
				searched = &movies[tt.searched]
			}
			got, err := MatchMovie(parsed, tt.imdb, tt.tmdb, searched, movies)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("MatchMovie() error = %v, want %v", err, tt.wantErr)
			}
			switch {
			case tt.want < 0 && got != nil:
				t.Errorf("MatchMovie() = %q, want nil", got.Title)
			case tt.want >= 0 && (got == nil || got.ID != movies[tt.want].ID):
				t.Errorf("MatchMovie() = %v, want %q", got, movies[tt.want].Title)
			}
		})
	}
}

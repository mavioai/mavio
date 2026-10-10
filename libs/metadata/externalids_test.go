package metadata

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestItemExternalURLs(t *testing.T) {
	kinds := MergeExternalIDKinds(BuiltinExternalIDKinds(),
		ExternalIDKind{Provider: "trakt", Name: "Trakt", Items: map[core.ItemKind]string{core.KindMovie: "https://trakt.tv/movies/{id}"}, Plugin: "org.example.trakt"},
		ExternalIDKind{Provider: core.ProviderIMDb, Name: "Not IMDb", Items: map[core.ItemKind]string{core.KindMovie: "https://example.org/{id}"}, Plugin: "org.example.imdb"},
	)
	tests := []struct {
		kind core.ItemKind
		ids  map[core.Provider]string
		want []ExternalURL
	}{
		{core.KindMovie, map[core.Provider]string{core.ProviderTMDB: "603", core.ProviderIMDb: "tt0133093", "trakt": "the matrix/1999", core.ProviderTVMaze: "1"}, []ExternalURL{
			{"TMDB", "https://www.themoviedb.org/movie/603"},
			{"IMDb", "https://www.imdb.com/title/tt0133093"},
			{"Trakt", "https://trakt.tv/movies/the%20matrix%2F1999"},
		}},
		{core.KindSeries, map[core.Provider]string{core.ProviderTMDB: "1399", core.ProviderTVDB: "121361"}, []ExternalURL{
			{"TMDB", "https://www.themoviedb.org/tv/1399"},
			{"TheTVDB", "https://www.thetvdb.com/dereferrer/series/121361"},
		}},
		{core.KindTrack, map[core.Provider]string{core.ProviderMusicBrainzTrack: "abc", core.ProviderMusicBrainzAlbum: "def"}, []ExternalURL{
			{"MusicBrainz Release", "https://musicbrainz.org/release/def"},
			{"MusicBrainz Track", "https://musicbrainz.org/track/abc"},
		}},
		{core.KindPhoto, map[core.Provider]string{core.ProviderTMDB: "1"}, nil},
	}
	for _, tt := range tests {
		if got := ItemExternalURLs(kinds, tt.kind, tt.ids); !slices.Equal(got, tt.want) {
			t.Errorf("ItemExternalURLs(%s) = %v, want = %v", tt.kind, got, tt.want)
		}
	}
	got := PersonExternalURLs(kinds, map[core.Provider]string{core.ProviderIMDb: "nm0000206", core.ProviderTVDB: "1"})
	if want := []ExternalURL{{"IMDb", "https://www.imdb.com/name/nm0000206"}}; !slices.Equal(got, want) {
		t.Errorf("PersonExternalURLs() = %v, want = %v", got, want)
	}
}

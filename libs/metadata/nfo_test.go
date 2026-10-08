package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

const localPoster = "/media/movies/Justice League (2017).jpg"

func parseFile(t *testing.T, name string, kind core.ItemKind) *Result {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "nfo", name))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseNFO(data, kind, NFOOptions{FileExists: func(p string) bool { return p == localPoster }})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func check[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got = %v, want = %v", what, got, want)
	}
}

func checkInt(t *testing.T, what string, got *int, want int) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s: got = %v, want = %d", what, got, want)
	}
}

func checkDate(t *testing.T, what string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil || !got.Equal(want) {
		t.Errorf("%s: got = %v, want = %v", what, got, want)
	}
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func peopleOf(r *Result, kind core.CreditKind) []Person {
	var out []Person
	for _, p := range r.People {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

func names(people []Person) []string {
	var out []string
	for _, p := range people {
		out = append(out, p.Name)
	}
	return out
}

func remoteImages(r *Result, kind core.ImageKind) []string {
	var out []string
	for _, i := range r.RemoteImages {
		if i.Kind == kind {
			out = append(out, i.URL)
		}
	}
	return out
}

// checkNoItem ports the facts that parse without an item or a file.
func checkNoItem(t *testing.T, kind core.ItemKind, file string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "nfo", file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseNFO(data, "", NFOOptions{}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("no item kind: got = %v, want = ErrInvalid", err)
	}
	if _, err := ParseNFO(nil, kind, NFOOptions{}); !errors.Is(err, ErrEmptyNFO) {
		t.Errorf("no file: got = %v, want = ErrEmptyNFO", err)
	}
}

func TestPortedMovieNfoParser(t *testing.T) {
	portedCases(t, "nfo/parsers/movie_nfo_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"Parse_UrlFile_Success": func(t *testing.T, a args) {
				r := parseFile(t, strings.TrimPrefix(a.str(t, "path"), "Test Data/"), core.KindMovie)
				check(t, "id", r.Item.ExternalIDs[core.Provider(strings.ToLower(a.str(t, "provider")))], a.str(t, "id"))
			},
		},
		facts: map[string]string{
			"Fetch_Valid_Success":                                        "TestParseMovieNFO",
			"Parse_GivenFileWithFanartTag_Success":                       "TestParseMovieNFOVariants",
			"Parse_RadarrUrlFile_Success":                                "TestParseMovieNFOVariants",
			"Fetch_WithNullItem_ThrowsArgumentException":                 "TestParseNFOWithoutItem",
			"Fetch_NullResult_ThrowsArgumentException":                   "TestParseNFOWithoutItem",
			"Parsing_Fields_With_Escaped_Xml_Special_Characters_Success": "TestParseMovieNFOVariants",
			"Parse_TmdbcolUniqueId_NormalizedToTmdbCollection":           "TestParseMovieNFOVariants",
			"Parse_CommunityRating_ValidRating_Success":                  "TestParseMovieNFOVariants",
			"Parse_CommunityRating_OutOfRange_Ignored":                   "TestParseMovieNFOVariants",
			"Parse_CommunityRating_Comma":                                "TestParseMovieNFOVariants",
		},
	})
}

func TestParseMovieNFO(t *testing.T) {
	r := parseFile(t, "Justice League.nfo", core.KindMovie)
	it := r.Item
	check(t, "original title", it.OriginalTitle, "Justice League")
	check(t, "tagline", it.Tagline, "Justice for all.")
	check(t, "imdb", it.ExternalIDs[core.ProviderIMDb], "tt0974015")
	check(t, "tmdb", it.ExternalIDs[core.ProviderTMDB], "141052")
	if !slices.Equal(it.Genres, []string{"Action", "Adventure", "Fantasy", "Sci-Fi"}) {
		t.Errorf("genres: got = %q", it.Genres)
	}
	checkDate(t, "premiere", it.PremiereDate, date(2017, 11, 15))
	checkDate(t, "end", it.EndDate, date(2017, 11, 16))
	if !slices.Equal(it.Studios, []string{"DC Comics"}) {
		t.Errorf("studios: got = %q", it.Studios)
	}
	check(t, "aspect ratio", it.AspectRatio, "1.777778")
	check(t, "3D", it.Video3DFormat, core.Video3DHalfSideBySide)
	check(t, "width", r.Video.Width, 1920)
	check(t, "height", r.Video.Height, 1080)
	check(t, "runtime", it.Runtime, 6268*time.Second)
	check(t, "subtitles", r.Video.HasSubtitles, true)
	check(t, "critic rating", it.CriticRating, 7.6)
	check(t, "custom rating", it.CustomRating, "8.7")
	check(t, "language", it.MetadataLanguage, "en")
	check(t, "country", it.MetadataCountry, "us")
	if !slices.Equal(it.RemoteTrailers, []string{"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}) {
		t.Errorf("trailers: got = %q", it.RemoteTrailers)
	}
	// Jellyfin keeps only the last <country>; all are kept.
	if !slices.Equal(it.ProductionLocations, []string{"USA", "Canada", "UK"}) {
		t.Errorf("production locations: got = %q", it.ProductionLocations)
	}

	check(t, "people", len(r.People), 20)
	writers := names(peopleOf(r, core.CreditWriter))
	if len(writers) != 3 || !slices.Contains(writers, "Jerry Siegel") || !slices.Contains(writers, "Joe Shuster") || !slices.Contains(writers, "Test") {
		t.Errorf("writers: got = %q", writers)
	}
	if d := names(peopleOf(r, core.CreditDirector)); !slices.Equal(d, []string{"Zack Snyder"}) {
		t.Errorf("directors: got = %q", d)
	}
	actors := peopleOf(r, core.CreditActor)
	check(t, "actors", len(actors), 15)
	i := slices.IndexFunc(actors, func(p Person) bool { return p.Role == "Aquaman" })
	if i < 0 {
		t.Fatal("Aquaman: got = missing")
	}
	check(t, "Aquaman", actors[i].Name, "Jason Momoa")
	checkInt(t, "Aquaman order", actors[i].Order, 5)
	check(t, "Aquaman image", actors[i].ImageURL, "https://m.media-amazon.com/images/M/MV5BMTI5MTU5NjM1MV5BMl5BanBnXkFtZTcwODc4MDk0Mw@@._V1_SX1024_SY1024_.jpg")
	if l := names(peopleOf(r, core.CreditLyricist)); !slices.Equal(l, []string{"Test Lyricist"}) {
		t.Errorf("lyricist: got = %q", l)
	}
	check(t, "date added", it.DateAdded, time.Date(2019, 8, 6, 9, 1, 18, 0, time.UTC))

	// Watched state.
	checkInt(t, "play count", r.UserData.PlayCount, 2)
	if r.UserData.Played == nil || !*r.UserData.Played {
		t.Errorf("played: got = %v", r.UserData.Played)
	}
	if lp := r.UserData.LastPlayedAt; lp == nil || !lp.Equal(time.Date(2021, 2, 11, 7, 47, 23, 0, time.UTC)) {
		t.Errorf("last played: got = %v", lp)
	}

	// Movie set.
	check(t, "collection id", it.ExternalIDs[core.ProviderTMDBCollection], "702342")
	check(t, "collection", it.CollectionName, "Justice League Collection")

	// Images: one of each kind; the local poster that exists.
	check(t, "remote images", len(r.RemoteImages), 7)
	for kind, url := range map[core.ImageKind]string{
		core.ImagePrimary:  "http://image.tmdb.org/t/p/original/9rtrRGeRnL0JKtu9IMBWsmlmmZz.jpg",
		core.ImageLogo:     "https://assets.fanart.tv/fanart/movies/141052/hdmovielogo/justice-league-5865bf95cbadb.png",
		core.ImageBanner:   "https://assets.fanart.tv/fanart/movies/141052/moviebanner/justice-league-586017e95adbd.jpg",
		core.ImageThumb:    "https://assets.fanart.tv/fanart/movies/141052/moviethumb/justice-league-585fb155c3743.jpg",
		core.ImageArt:      "https://assets.fanart.tv/fanart/movies/141052/hdmovieclearart/justice-league-5865c23193041.png",
		core.ImageDisc:     "https://assets.fanart.tv/fanart/movies/141052/moviedisc/justice-league-5a3af26360617.png",
		core.ImageBackdrop: "https://assets.fanart.tv/fanart/movies/141052/moviebackground/justice-league-5793f518c6d6e.jpg",
	} {
		if got := remoteImages(r, kind); !slices.Equal(got, []string{url}) {
			t.Errorf("%s images: got = %q, want = %q", kind, got, url)
		}
	}
	if len(r.LocalImages) != 1 || r.LocalImages[0].Path != localPoster {
		t.Errorf("local images: got = %+v", r.LocalImages)
	}
}

func TestParseMovieNFOVariants(t *testing.T) {
	// A <fanart> element holds backdrops; a relative path is not an image.
	r := parseFile(t, "Fanart.nfo", core.KindMovie)
	if got := remoteImages(r, core.ImageBackdrop); !slices.Equal(got, []string{"https://assets.fanart.tv/fanart/movies/141052/moviebackground/justice-league-5a5332c7b5e77.jpg"}) {
		t.Errorf("backdrops: got = %q", got)
	}
	// A file of Radarr's provider URLs.
	r = parseFile(t, "Radarr.nfo", core.KindMovie)
	check(t, "tmdb", r.Item.ExternalIDs[core.ProviderTMDB], "583689")
	check(t, "imdb", r.Item.ExternalIDs[core.ProviderIMDb], "tt4154796")
	// Escaped XML characters.
	r = parseFile(t, "Lilo & Stitch.nfo", core.KindMovie)
	check(t, "name", r.Item.Name, "Lilo & Stitch")
	check(t, "original title", r.Item.OriginalTitle, "Lilo & Stitch")
	check(t, "collection", r.Item.CollectionName, "Lilo & Stitch Collection")
	if !strings.HasPrefix(r.Item.Overview, ">>") || !strings.HasSuffix(r.Item.Overview, "<<") {
		t.Errorf("overview: got = %q", r.Item.Overview)
	}
	// <uniqueid type="tmdbcol"> is the TMDB collection.
	check(t, "collection id", r.Item.ExternalIDs[core.ProviderTMDBCollection], "97020")
	if _, ok := r.Item.ExternalIDs["tmdbcol"]; ok {
		t.Error("tmdbcol: got = present, want = normalized")
	}
	// Community ratings: in range, with a comma, out of range.
	check(t, "rating", parseFile(t, "CommunityRating.nfo", core.KindMovie).Item.CommunityRating, 7.5)
	check(t, "rating with comma", parseFile(t, "CommunityRating_Comma.nfo", core.KindMovie).Item.CommunityRating, 7.5)
	check(t, "rating out of range", parseFile(t, "CommunityRating_OutOfRange.nfo", core.KindMovie).Item.CommunityRating, 0.0)
}

func TestParseNFOWithoutItem(t *testing.T) {
	for kind, file := range map[core.ItemKind]string{
		core.KindMovie: "Justice League.nfo", core.KindEpisode: "The Bone Orchard.nfo", core.KindSeries: "American Gods.nfo",
		core.KindSeason: "Season 01.nfo", core.KindMusicAlbum: "The Best of 1980-1990.nfo",
		core.KindMusicArtist: "U2.nfo", core.KindMusicVideo: "Dancing Queen.nfo",
	} {
		checkNoItem(t, kind, file)
	}
}

func noItemFacts(m map[string]string) map[string]string {
	m["Fetch_WithNullItem_ThrowsArgumentException"] = "TestParseNFOWithoutItem"
	m["Fetch_NullResult_ThrowsArgumentException"] = "TestParseNFOWithoutItem"
	return m
}

func TestPortedEpisodeNfoProvider(t *testing.T) {
	portedCases(t, "nfo/parsers/episode_nfo_provider.json", ported{facts: noItemFacts(map[string]string{
		"Fetch_Valid_Success":                                "TestParseEpisodeNFO",
		"Fetch_Valid_MultiEpisode_Success":                   "TestParseMultiEpisodeNFO",
		"Fetch_Valid_MultiEpisode_Unordered_Success":         "TestParseMultiEpisodeNFO",
		"Fetch_Valid_MultiEpisode_With_Missing_Tags_Success": "TestParseMultiEpisodeNFO",
		"Parse_GivenFileWithThumbWithoutAspect_Success":      "TestParseEpisodeNFO",
	})})
}

func TestParseEpisodeNFO(t *testing.T) {
	r := parseFile(t, "The Bone Orchard.nfo", core.KindEpisode)
	it := r.Item
	check(t, "name", it.Name, "The Bone Orchard")
	check(t, "series", r.SeriesName, "American Gods")
	checkInt(t, "episode", it.IndexNumber, 1)
	checkInt(t, "season", it.ParentIndexNumber, 1)
	check(t, "overview", it.Overview, "When Shadow Moon is released from prison early after the death of his wife, he meets Mr. Wednesday and is recruited as his bodyguard. Shadow discovers that this may be more than he bargained for.")
	check(t, "runtime", it.Runtime, 0)
	check(t, "rating", it.OfficialRating, "16")
	for _, g := range []string{"Drama", "Mystery", "Sci-Fi & Fantasy"} {
		if !slices.Contains(it.Genres, g) {
			t.Errorf("genres: got = %q, want %q", it.Genres, g)
		}
	}
	checkDate(t, "premiere", it.PremiereDate, date(2017, 4, 30))
	check(t, "year", it.ProductionYear, 2017)
	if !slices.Equal(it.Studios, []string{"Starz"}) {
		t.Errorf("studios: got = %q", it.Studios)
	}
	checkInt(t, "episode end", it.IndexNumberEnd, 1)
	checkInt(t, "airs after season", it.AirsAfterSeasonNumber, 2)
	checkInt(t, "airs before season", it.AirsBeforeSeasonNumber, 3)
	checkInt(t, "airs before episode", it.AirsBeforeEpisodeNumber, 1)
	check(t, "imdb", it.ExternalIDs[core.ProviderIMDb], "tt5017734")
	check(t, "tmdb", it.ExternalIDs[core.ProviderTMDB], "1276153")
	writers := names(peopleOf(r, core.CreditWriter))
	if len(writers) != 2 || !slices.Contains(writers, "Bryan Fuller") || !slices.Contains(writers, "Michael Green") {
		t.Errorf("writers: got = %q", writers)
	}
	if d := names(peopleOf(r, core.CreditDirector)); !slices.Equal(d, []string{"David Slade"}) {
		t.Errorf("directors: got = %q", d)
	}
	actors := peopleOf(r, core.CreditActor)
	check(t, "actors", len(actors), 11)
	i := slices.IndexFunc(actors, func(p Person) bool { return p.Role == "Shadow Moon" })
	if i < 0 {
		t.Fatal("Shadow Moon: got = missing")
	}
	check(t, "Shadow Moon", actors[i].Name, "Ricky Whittle")
	checkInt(t, "order", actors[i].Order, 0)
	check(t, "image", actors[i].ImageURL, "http://image.tmdb.org/t/p/original/cjeDbVfBp6Qvb3C74Dfy7BKDTQN.jpg")
	check(t, "date added", it.DateAdded, time.Date(2017, 10, 7, 14, 25, 47, 0, time.UTC))

	// Sonarr writes posters as <thumb> without an aspect.
	r = parseFile(t, "Sonarr-Thumb.nfo", core.KindEpisode)
	if got := remoteImages(r, core.ImagePrimary); !slices.Equal(got, []string{"https://artworks.thetvdb.com/banners/episodes/359095/7081317.jpg"}) {
		t.Errorf("posters: got = %q", got)
	}
}

func TestParseMultiEpisodeNFO(t *testing.T) {
	const overview = "A new Stargate team embarks on a dangerous mission to a distant galaxy, where they discover a mythical lost city -- and a deadly new enemy."
	for _, file := range []string{"Rising.nfo", "Rising-Reversed.nfo"} {
		it := parseFile(t, file, core.KindEpisode).Item
		check(t, file+" name", it.Name, "Rising (1) / Rising (2)")
		checkInt(t, file+" episode", it.IndexNumber, 1)
		checkInt(t, file+" episode end", it.IndexNumberEnd, 2)
		checkInt(t, file+" season", it.ParentIndexNumber, 1)
		check(t, file+" overview", it.Overview, overview+" / Sheppard tries to convince Weir to mount a rescue mission to free Colonel Sumner, Teyla, and the others captured by the Wraith.")
		checkDate(t, file+" premiere", it.PremiereDate, date(2004, 7, 16))
		check(t, file+" year", it.ProductionYear, 2004)
	}
	it := parseFile(t, "Stargate Atlantis S01E01-E04.nfo", core.KindEpisode).Item
	check(t, "name", it.Name, "Rising / Hide and Seek / Thirty-Eight Minutes")
	check(t, "original title", it.OriginalTitle, "Rising (1) / Rising (2) / Hide and Seek / Thirty-Eight Minutes")
	checkInt(t, "episode", it.IndexNumber, 1)
	checkInt(t, "episode end", it.IndexNumberEnd, 4)
	checkInt(t, "season", it.ParentIndexNumber, 1)
	check(t, "overview", it.Overview, overview)
	checkDate(t, "premiere", it.PremiereDate, date(2004, 7, 16))
	check(t, "year", it.ProductionYear, 2004)
}

func TestPortedSeriesNfoParser(t *testing.T) {
	portedCases(t, "nfo/parsers/series_nfo_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"Parse_UrlFile_Success": func(t *testing.T, a args) {
				r := parseFile(t, strings.TrimPrefix(a.str(t, "path"), "Test Data/"), core.KindSeries)
				check(t, "id", r.Item.ExternalIDs[core.Provider(strings.ToLower(a.str(t, "provider")))], a.str(t, "id"))
			},
		},
		facts: noItemFacts(map[string]string{"Fetch_Valid_Success": "TestParseSeriesNFO"}),
	})
}

func TestParseSeriesNFO(t *testing.T) {
	r := parseFile(t, "American Gods.nfo", core.KindSeries)
	it := r.Item
	check(t, "original title", it.OriginalTitle, "American Gods")
	check(t, "tagline", it.Tagline, "")
	check(t, "runtime", it.Runtime, 0)
	check(t, "tmdb", it.ExternalIDs[core.ProviderTMDB], "46639")
	check(t, "tvdb", it.ExternalIDs[core.ProviderTVDB], "253573")
	check(t, "imdb", it.ExternalIDs[core.ProviderIMDb], "tt11111")
	if len(it.Genres) != 3 {
		t.Errorf("genres: got = %q", it.Genres)
	}
	checkDate(t, "premiere", it.PremiereDate, date(2017, 4, 30))
	if !slices.Equal(it.Studios, []string{"Starz"}) {
		t.Errorf("studios: got = %q", it.Studios)
	}
	check(t, "air time", it.AirTime, "9 PM")
	if !slices.Equal(it.AirDays, []time.Weekday{time.Friday}) {
		t.Errorf("air days: got = %v", it.AirDays)
	}
	check(t, "status", it.SeriesStatus, core.SeriesEnded)
	check(t, "people", len(r.People), 6)
	if len(peopleOf(r, core.CreditActor)) != 6 {
		t.Errorf("people: got = %+v, want all actors", r.People)
	}
	i := slices.IndexFunc(r.People, func(p Person) bool { return p.Role == "Mad Sweeney" })
	if i < 0 {
		t.Fatal("Mad Sweeney: got = missing")
	}
	check(t, "Mad Sweeney", r.People[i].Name, "Pablo Schreiber")
	checkInt(t, "order", r.People[i].Order, 3)
	check(t, "image", r.People[i].ImageURL, "http://image.tmdb.org/t/p/original/uo8YljeePz3pbj7gvWXdB4gOOW4.jpg")
	check(t, "date added", it.DateAdded, time.Date(2017, 10, 7, 14, 25, 47, 0, time.UTC))
}

func TestPortedSeasonNfoProvider(t *testing.T) {
	portedCases(t, "nfo/parsers/season_nfo_provider.json", ported{facts: noItemFacts(map[string]string{"Fetch_Valid_Success": "TestParseSeasonNFO"})})
}

func TestParseSeasonNFO(t *testing.T) {
	r := parseFile(t, "Season 01.nfo", core.KindSeason)
	it := r.Item
	check(t, "name", it.Name, "Season 1")
	checkInt(t, "season", it.IndexNumber, 1)
	check(t, "locked", it.Locked, false)
	check(t, "year", it.ProductionYear, 2019)
	checkDate(t, "premiere", it.PremiereDate, date(2019, 11, 8))
	check(t, "date added", it.DateAdded, time.Date(2020, 6, 14, 17, 26, 51, 0, time.UTC))
	check(t, "people", len(r.People), 10)
	if len(peopleOf(r, core.CreditActor)) != 10 {
		t.Errorf("people: got = %+v, want all actors", r.People)
	}
	i := slices.IndexFunc(r.People, func(p Person) bool { return p.Role == "Nini" })
	if i < 0 {
		t.Fatal("Nini: got = missing")
	}
	check(t, "Nini", r.People[i].Name, "Olivia Rodrigo")
	checkInt(t, "order", r.People[i].Order, 0)
	check(t, "image", r.People[i].ImageURL, "/config/metadata/People/O/Olivia Rodrigo/poster.jpg")
}

func TestPortedMusicNfo(t *testing.T) {
	portedCases(t, "nfo/parsers/music_album_nfo_provider.json", ported{facts: noItemFacts(map[string]string{"Fetch_Valid_Success": "TestParseMusicNFO"})})
	portedCases(t, "nfo/parsers/music_artist_nfo_parser.json", ported{facts: noItemFacts(map[string]string{"Fetch_Valid_Success": "TestParseMusicNFO"})})
	portedCases(t, "nfo/parsers/music_video_nfo_parser.json", ported{facts: noItemFacts(map[string]string{"Fetch_Valid_Succes": "TestParseMusicNFO"})})
}

func TestParseMusicNFO(t *testing.T) {
	album := parseFile(t, "The Best of 1980-1990.nfo", core.KindMusicAlbum).Item
	check(t, "album", album.Name, "The Best of 1980-1990")
	check(t, "year", album.ProductionYear, 1989)
	if !slices.Equal(album.Genres, []string{"Pop"}) || !slices.Contains(album.Tags, "Rock/Pop") {
		t.Errorf("genres and tags: got = %q %q", album.Genres, album.Tags)
	}
	if !strings.HasPrefix(album.Overview, "The Best of 1980-1990 is the first greatest hits compilation") ||
		!strings.Contains(album.Overview, "Billboard 200.\nThe boy on the cover") {
		t.Errorf("overview: got = %q", album.Overview)
	}

	artist := parseFile(t, "U2.nfo", core.KindMusicArtist).Item
	check(t, "artist", artist.Name, "U2")
	check(t, "sort name", artist.SortName, "U2")
	check(t, "musicbrainz", artist.ExternalIDs[core.ProviderMusicBrainzArtist], "a3cb23fc-acd3-4ce0-8f36-1e5aa6a18432")
	if !slices.Equal(artist.Genres, []string{"Rock"}) {
		t.Errorf("genres: got = %q", artist.Genres)
	}

	video := parseFile(t, "Dancing Queen.nfo", core.KindMusicVideo).Item
	check(t, "music video", video.Name, "Dancing Queen")
	if !slices.Equal(video.Artists, []string{"ABBA"}) {
		t.Errorf("artists: got = %q", video.Artists)
	}
	check(t, "album", video.Album, "Arrival")
}

func TestPortedMovieNfoLocation(t *testing.T) {
	portedCases(t, "nfo/location/movie_nfo_location.json", ported{facts: map[string]string{
		"Movie_MixedFolder_Success":    "TestMovieNFOPaths",
		"Movie_SeparateFolder_Success": "TestMovieNFOPaths",
		"Movie_DVD_Success":            "TestMovieNFOPaths",
	}})
}

func TestMovieNFOPaths(t *testing.T) {
	tests := []struct {
		path string
		o    MovieNFOOptions
		want []string
	}{
		{"/media/movies/Avengers Endgame.mp4", MovieNFOOptions{InMixedFolder: true, IsMovie: true}, []string{"/media/movies/Avengers Endgame.nfo"}},
		{
			"/media/movies/Avengers Endgame/Avengers Endgame.mp4",
			MovieNFOOptions{IsMovie: true},
			[]string{"/media/movies/Avengers Endgame/movie.nfo", "/media/movies/Avengers Endgame/Avengers Endgame.nfo"},
		},
		{
			`C:\media\movies\Avengers Endgame\Avengers Endgame.mp4`,
			MovieNFOOptions{IsMovie: true},
			[]string{`C:\media\movies\Avengers Endgame\movie.nfo`, `C:\media\movies\Avengers Endgame\Avengers Endgame.nfo`},
		},
		{"/media/movies/Avengers Endgame", MovieNFOOptions{Disc: DiscDVD, IsMovie: true}, []string{
			"/media/movies/Avengers Endgame/VIDEO_TS/VIDEO_TS.nfo", "/media/movies/Avengers Endgame/movie.nfo",
			"/media/movies/Avengers Endgame/Avengers Endgame.nfo",
		}},
	}
	for _, tt := range tests {
		if got := MovieNFOPaths(tt.path, tt.o); !slices.Equal(got, tt.want) {
			t.Errorf("MovieNFOPaths(%q): got = %q, want = %q", tt.path, got, tt.want)
		}
	}
}

package organize

import (
	"strings"
	"testing"
)

// Ported from Radarr's CustomFormatsFixture.
func TestCustomFormats(t *testing.T) {
	formats := []CustomFormat{
		{Name: "INTERNAL", IncludeWhenRenaming: true},
		{Name: "AMZN", IncludeWhenRenaming: true},
		{Name: "NAME WITH SPACES", IncludeWhenRenaming: true},
		{Name: "NotIncludedFormat", IncludeWhenRenaming: false},
	}
	withFormats := func(f *File) { f.CustomFormats = formats }
	none := func(f *File) { f.CustomFormats = nil }
	cases := []nameCase{
		{name: "custom formats", format: "{Custom Formats}", file: withFormats, want: "INTERNAL AMZN NAME WITH SPACES"},
		{name: "custom formats with no custom formats", format: "{Custom Formats}", file: none, want: ""},
		{name: "exclude INTERNAL", format: "{Custom Formats:-INTERNAL}", file: withFormats, want: "AMZN NAME WITH SPACES"},
		{name: "exclude NAME WITH SPACES", format: "{Custom Formats:-NAME WITH SPACES}", file: withFormats, want: "INTERNAL AMZN"},
		{name: "exclude two", format: "{Custom Formats:-INTERNAL,NAME WITH SPACES}", file: withFormats, want: "AMZN"},
		{name: "include INTERNAL", format: "{Custom Formats:INTERNAL}", file: withFormats, want: "INTERNAL"},
		{name: "include NAME WITH SPACES", format: "{Custom Formats:NAME WITH SPACES}", file: withFormats, want: "NAME WITH SPACES"},
		{name: "include two", format: "{Custom Formats:INTERNAL,NAME WITH SPACES}", file: withFormats, want: "INTERNAL NAME WITH SPACES"},
		{name: "invalid token dash", format: "{Custom Formats:-}", file: withFormats, want: "{Custom Formats:-}"},
		{name: "invalid token empty", format: "{Custom Formats:}", file: withFormats, want: "{Custom Formats:}"},
		{name: "custom format without name", format: "{Custom Format}", file: withFormats, want: ""},
		{name: "custom format INTERNAL", format: "{Custom Format:INTERNAL}", file: withFormats, want: "INTERNAL"},
		{name: "custom format AMZN", format: "{Custom Format:AMZN}", file: withFormats, want: "AMZN"},
		{name: "custom format NAME WITH SPACES", format: "{Custom Format:NAME WITH SPACES}", file: withFormats, want: "NAME WITH SPACES"},
		{name: "custom format DOESNOTEXIST", format: "{Custom Format:DOESNOTEXIST}", file: withFormats, want: ""},
		{name: "two custom formats", format: "{Custom Format:INTERNAL} - {Custom Format:AMZN}", file: withFormats, want: "INTERNAL - AMZN"},
		{name: "two custom formats reversed", format: "{Custom Format:AMZN} - {Custom Format:INTERNAL}", file: withFormats, want: "AMZN - INTERNAL"},
		{name: "custom format without name and formats", format: "{Custom Format}", file: none, want: ""},
		{name: "custom format INTERNAL without formats", format: "{Custom Format:INTERNAL}", file: none, want: ""},
		{name: "custom format AMZN without formats", format: "{Custom Format:AMZN}", file: none, want: ""},
	}
	runFileCases(t, fixture{naming: renaming(), movie: Movie{Title: "South Park"}, file: hdtv720p()}, cases)
}

// Ported from Radarr's EditionTagsFixture.
func TestEditionTags(t *testing.T) {
	edition := func(e string) func(f *File) { return func(f *File) { f.Edition = e } }
	cases := []nameCase{
		{name: "add edition tag", format: "{Movie Title} [{Edition Tags}]", want: "Movie Title [Uncut]"},
		{name: "edition curly brackets", format: "{Movie Title} {{Edition Tags}}", want: "Movie Title {Uncut}"},
		{name: "edition tag curly brackets", format: "{Movie Title} {{edition-{Edition Tags}}}", want: "Movie Title {{edition-Uncut}}"},
	}
	for _, format := range []string{
		"{Movie Title} {Edition Tags}", "{Movie Title} {{Edition Tags}}",
		"{Movie Title} {edition-{Edition Tags}}", "{Movie Title} {{edition-{Edition Tags}}}",
	} {
		cases = append(cases, nameCase{name: "conditional hide " + format, format: format, file: edition(""), want: "Movie Title"})
	}
	editions := []struct{ edition, format, want string }{
		// should_always_lowercase_ordinals
		{"1st anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [1st Anniversary Edition]"},
		{"2nd Anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [2nd Anniversary Edition]"},
		{"3rd anniversary Edition", "{Movie Title} [{Edition Tags}]", "Movie Title [3rd Anniversary Edition]"},
		{"4th anNiverSary eDitIOn", "{Movie Title} [{Edition Tags}]", "Movie Title [4th Anniversary Edition]"},
		{"5th anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [5th Anniversary Edition]"},
		{"6th anNiverSary EDITION", "{Movie Title} [{Edition Tags}]", "Movie Title [6th Anniversary Edition]"},
		{"7TH anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [7th Anniversary Edition]"},
		{"8Th anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [8th Anniversary Edition]"},
		{"9tH anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [9th Anniversary Edition]"},
		{"10th anniversary edition", "{Movie Title} [{edition tags}]", "Movie Title [10th anniversary edition]"},
		{"10TH anniversary edition", "{Movie Title} [{edition tags}]", "Movie Title [10th anniversary edition]"},
		{"10Th anniversary edition", "{Movie Title} [{edition tags}]", "Movie Title [10th anniversary edition]"},
		{"10th anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [10th Anniversary Edition]"},
		{"10TH anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [10th Anniversary Edition]"},
		{"10Th anniversary edition", "{Movie Title} [{Edition Tags}]", "Movie Title [10th Anniversary Edition]"},
		{"10th anniversary edition", "{Movie Title} [{EDITION TAGS}]", "Movie Title [10TH ANNIVERSARY EDITION]"},
		{"10TH anniversary edition", "{Movie Title} [{EDITION TAGS}]", "Movie Title [10TH ANNIVERSARY EDITION]"},
		{"10Th anniversary edition", "{Movie Title} [{EDITION TAGS}]", "Movie Title [10TH ANNIVERSARY EDITION]"},
		// should_always_uppercase_special_strings
		{"imax edition", "{Movie Title} [{edition tags}]", "Movie Title [imax edition]"},
		{"imax edition", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX Edition]"},
		{"Imax edition", "{Movie Title} [{EDITION TAGS}]", "Movie Title [IMAX EDITION]"},
		{"imax version", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX Version]"},
		{"IMAX-edition", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX-Edition]"},
		{"IMAX_edition", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX_Edition]"},
		{"IMAX.eDiTioN", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX.Edition]"},
		{"IMAX ed.", "{Movie Title} [{edition tags}]", "Movie Title [imax ed.]"},
		{"IMAX ed.", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX Ed.]"},
		{"Imax-ed.", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX-Ed.]"},
		{"imax.Ed", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX.Ed]"},
		{"Imax_ed", "{Movie Title} [{Edition Tags}]", "Movie Title [IMAX_Ed]"},
		{"THEATRICAL", "{Movie Title} [{Edition Tags}]", "Movie Title [Theatrical]"},
		{"director's CUt", "{Movie Title} [{Edition Tags}]", "Movie Title [Director's Cut]"},
	}
	// The 3 x 3 casings of each upper-case word of should_always_uppercase_special_strings.
	for _, word := range []string{"imax", "3d", "hdr", "dv", "sdr"} {
		upper := strings.ToUpper(word)
		for _, e := range distinct([]string{word, upper, strings.ToUpper(word[:1]) + word[1:]}) {
			editions = append(editions,
				struct{ edition, format, want string }{e, "{Movie Title} [{edition tags}]", "Movie Title [" + word + "]"},
				struct{ edition, format, want string }{e, "{Movie Title} [{Edition Tags}]", "Movie Title [" + upper + "]"},
				struct{ edition, format, want string }{e, "{Movie Title} [{EDITION TAGS}]", "Movie Title [" + upper + "]"},
			)
		}
	}
	for _, e := range editions {
		cases = append(cases, nameCase{name: e.edition + " " + e.format, format: e.format, file: edition(e.edition), want: e.want})
	}
	fx := fixture{naming: renaming(), movie: Movie{Title: "Movie Title"}, file: File{Quality: "Unknown", ReleaseGroup: "RadarrTest", Edition: "Uncut"}}
	runFileCases(t, fx, cases)
}

// Ported from Radarr's IdFixture.
func TestIDTokens(t *testing.T) {
	noImdb := func(m *Movie) { m.ImdbID = "" }
	cases := []nameCase{
		{name: "imdb id", format: "{Movie Title} ({ImdbId})", want: "Movie Title (tt12345)"},
		{name: "tmdb id", format: "{Movie Title} ({TmdbId})", want: "Movie Title (123456)"},
		{name: "imdb tag", format: "{Movie Title} {imdb-{ImdbId}}", want: "Movie Title {imdb-tt12345}"},
		{name: "imdb tag curly brackets", format: "{Movie Title} {{imdb-{ImdbId}}}", want: "Movie Title {{imdb-tt12345}}"},
		{name: "tmdb tag curly brackets", format: "{Movie Title} {{tmdb-{TmdbId}}}", want: "Movie Title {{tmdb-123456}}"},
	}
	for _, format := range []string{
		"{Movie Title} {imdb-{ImdbId}}", "{Movie Title} {imdbid-{ImdbId}}",
		"{Movie Title} {{imdb-{ImdbId}}}", "{Movie Title} {{imdbid-{ImdbId}}}",
	} {
		cases = append(cases, nameCase{name: "skip imdb tag if null " + format, format: format, movie: noImdb, want: "Movie Title"})
	}
	fx := fixture{naming: DefaultNaming(), movie: Movie{Title: "Movie Title", ImdbID: "tt12345", TmdbID: 123456, Year: 1}}
	runFolderCases(t, fx, cases)
}

// Ported from Radarr's OriginalTitleFixture.
func TestOriginalTitle(t *testing.T) {
	cases := []nameCase{
		{
			name: "not recursively include current filename", format: "{Movie Title} {[Original Title]}",
			file: func(f *File) { f.RelativePath = "My Movie" }, want: "My Movie",
		},
		{
			name: "include original title if not current file name", format: "{Movie Title} {[Original Title]}",
			file: func(f *File) { f.SceneName, f.RelativePath = "my.movie.2008", "My Movie" }, want: "My Movie [my.movie.2008]",
		},
		{
			name: "include current filename if not renaming files", naming: notRenaming,
			file: func(f *File) { f.SceneName = "my.movie.2008" }, want: "my.movie.2008",
		},
		{
			name: "include current filename if not including multiple naming tokens", format: "{Original Title}",
			file: func(f *File) { f.RelativePath = "My Movie" }, want: "My Movie",
		},
	}
	runFileCases(t, fixture{naming: renaming(), movie: Movie{Title: "My Movie"}, file: hdtv720p()}, cases)
}

// Ported from Radarr's GetMovieFolderFixture.
func TestMovieFolder(t *testing.T) {
	cases := []nameCase{
		{name: "Arrival", format: "{Movie Title} ({Release Year})", movie: func(m *Movie) { m.Title, m.Year = "Arrival", 2016 }, want: "Arrival (2016)"},
		{name: "Big Short the", format: "{Movie TitleThe} ({Release Year})", movie: func(m *Movie) { m.Title, m.Year = "The Big Short", 2015 }, want: "Big Short, The (2015)"},
		{name: "Big Short", format: "{Movie Title} ({Release Year})", movie: func(m *Movie) { m.Title, m.Year = "The Big Short", 2015 }, want: "The Big Short (2015)"},
		{
			name: "collection the", format: "{Movie CollectionThe}/{Movie TitleThe} ({Release Year})",
			movie: func(m *Movie) { m.CollectionTitle, m.Title, m.Year = "The Y-Women Collection", "The Y-Women 14", 2005 },
			want:  "Y-Women Collection, The/Y-Women 14, The (2005)",
		},
		{
			name: "clean collection the", format: "{Movie CleanCollectionThe}/{Movie TitleThe} ({Release Year})",
			movie: func(m *Movie) {
				m.CollectionTitle, m.Title, m.Year = "A Decade's Worth of Changes", "The First Year", 1980
			},
			want: "Decades Worth of Changes, A/First Year, The (1980)",
		},
		{
			name: "no collection", format: "{Movie Title} ({Release Year})",
			movie: func(m *Movie) { m.Title, m.Year = "Just a Movie", 1999 }, want: "Just a Movie (1999)",
		},
		{
			name: "collectionless", format: "{Movie CollectionThe}/{Movie TitleThe} ({Release Year})",
			movie: func(m *Movie) { m.Title, m.Year = "Collectionless Slop", 1949 }, want: "Collectionless Slop (1949)",
		},
	}
	runFolderCases(t, fixture{naming: DefaultNaming()}, cases)
}

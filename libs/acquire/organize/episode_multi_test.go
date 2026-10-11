package organize

import (
	"testing"
	"unicode/utf8"
)

const standardWithTitle = "{Series Title} - S{season:00}E{episode:00} - {Episode Title}"

// Ported from Sonarr's MultiEpisodeFixture.
func TestMultiEpisode(t *testing.T) {
	two, three := citySushiEpisodes(2), citySushiEpisodes(3)
	const animeFormat = "{Series Title} - {absolute:000} - {Episode Title}"
	cases := []episodeCase{
		{name: "replace Series space Title", format: "{Series Title}", want: "South Park"},
		{name: "extend", format: standardWithTitle, naming: withStyle(MultiEpisodeExtend), episodes: two, want: "South Park - S15E06-07 - City Sushi"},
		{name: "duplicate", format: standardWithTitle, naming: withStyle(MultiEpisodeDuplicate), episodes: two, want: "South Park - S15E06 - S15E07 - City Sushi"},
		{name: "repeat", format: standardWithTitle, naming: withStyle(MultiEpisodeRepeat), episodes: two, want: "South Park - S15E06E07 - City Sushi"},
		{name: "scene", format: standardWithTitle, naming: withStyle(MultiEpisodeScene), episodes: two, want: "South Park - S15E06-E07 - City Sushi"},
		{
			name: "dash as separator when style is extend for anime", format: animeFormat,
			naming: withStyle(MultiEpisodeExtend), series: anime, episodes: two, want: "South Park - 100-101 - City Sushi",
		},
		{
			name: "duplicate absolute pattern", format: animeFormat,
			naming: withStyle(MultiEpisodeDuplicate), series: anime, episodes: three, want: "South Park - 100 - 101 - 102 - City Sushi",
		},
		{
			name:   "duplicate with bracket following pattern",
			format: "{Series Title} - S{season:00}E{episode:00} - ({Quality Title}, {MediaInfo Full}, {Release Group}) - {Episode Title}",
			naming: withStyle(MultiEpisodeDuplicate), episodes: two,
			want: "South Park - S15E06 - S15E07 - (HDTV-720p, , SonarrTest) - City Sushi",
		},
		{name: "range", format: standardWithTitle, naming: withStyle(MultiEpisodeRange), episodes: three, want: "South Park - S15E06-08 - City Sushi"},
		{
			name: "range anime", format: animeFormat, naming: withStyle(MultiEpisodeRange), series: anime, episodes: three,
			want: "South Park - 100-102 - City Sushi",
		},
		{
			name: "repeat anime", format: animeFormat, naming: withStyle(MultiEpisodeRepeat), series: anime, episodes: three,
			want: "South Park - 100-101-102 - City Sushi",
		},
		{name: "single episode with range", format: standardWithTitle, naming: withStyle(MultiEpisodeRange), want: "South Park - S15E06 - City Sushi"},
		{
			name: "single anime episode with range", format: animeFormat, naming: withStyle(MultiEpisodeRange), series: anime,
			want: "South Park - 100 - City Sushi",
		},
		{
			name:   "dash when separator is not set for absolute number",
			format: "{Series Title} - {season}x{episode:00} - [{absolute:000}] - {Episode Title} - {Quality Title}",
			naming: withStyle(MultiEpisodeDuplicate), series: anime, episodes: two,
			want: "South Park - 15x06 - 15x07 - [100-101] - City Sushi - HDTV-720p",
		},
		{
			name: "prefixed range", format: standardWithTitle, naming: withStyle(MultiEpisodePrefixedRange), episodes: three,
			want: "South Park - S15E06-E08 - City Sushi",
		},
		{
			name: "prefixed range anime", format: animeFormat, naming: withStyle(MultiEpisodePrefixedRange), series: anime, episodes: three,
			want: "South Park - 100-102 - City Sushi",
		},
		{
			name: "single episode with prefixed range", format: standardWithTitle, naming: withStyle(MultiEpisodePrefixedRange),
			want: "South Park - S15E06 - City Sushi",
		},
		{
			name: "single anime episode with prefixed range", format: animeFormat, naming: withStyle(MultiEpisodePrefixedRange), series: anime,
			want: "South Park - 100 - City Sushi",
		},
		{
			name: "prefixed range using episode separator", format: "{Series Title} - {season:0}x{episode:00} - {Episode Title}",
			naming: withStyle(MultiEpisodePrefixedRange), episodes: three, want: "South Park - 15x06-x08 - City Sushi",
		},
		{
			name: "range wrapped in brackets", format: "{Series Title} (S{season:00}E{episode:00}) {Episode Title}",
			naming: withStyle(MultiEpisodeRange), episodes: three, want: "South Park (S15E06-08) City Sushi",
		},
	}
	runEpisodeCases(t, southParkEpisodeFixture(), cases)
}

// Ported from Sonarr's AbsoluteEpisodeFormatFixture.
func TestAbsoluteEpisodeFormat(t *testing.T) {
	noAbsolute := func(eps []Episode) []Episode {
		eps[0].AbsoluteEpisodeNumber = 0
		return eps
	}
	formats := func(standard, animeFormat string) func(n *SeriesNaming) {
		return func(n *SeriesNaming) { n.StandardEpisodeFormat, n.AnimeEpisodeFormat = standard, animeFormat }
	}
	cases := []episodeCase{
		{
			name:     "standard format if absolute format requires absolute number and it is missing",
			naming:   formats("{Series Title} S{season:00}E{episode:00}", "{Series Title} {absolute:00} [{ReleaseGroup}]"),
			episodes: noAbsolute, want: "Anime Series S15E06",
		},
		{
			name:   "absolute format if absolute format requires absolute number and it is available",
			naming: formats("{Series Title} S{season:00}E{episode:00}", "{Series Title} {absolute:00} [{ReleaseGroup}]"),
			want:   "Anime Series 100 [SonarrTest]",
		},
		{
			name:   "absolute format if absolute format does not require absolute number and it is not available",
			naming: formats("{Series Title} S{season:00}E{episode:00}", "{Series Title} S{season:00}E{episode:00} [{ReleaseGroup}]"),
			want:   "Anime Series S15E06 [SonarrTest]",
		},
		{
			name:     "standard format without absolute numbering if absolute format requires absolute number and it is missing",
			naming:   formats("{Series Title} S{season:00}E{episode:00} - {absolute:00}", "{Series Title} {absolute:00} [{ReleaseGroup}]"),
			episodes: noAbsolute, want: "Anime Series S15E06",
		},
	}
	f := sonarrHDTV720p()
	f.Imported = true
	runEpisodeCases(t, episodeFixture{
		naming:   seriesRenaming(),
		series:   Series{Title: "Anime Series", Type: SeriesAnime},
		episodes: []Episode{citySushi()},
		file:     f,
	}, cases)
}

// Ported from Sonarr's EpisodeTitleCollapseFixture.
func TestEpisodeTitleCollapse(t *testing.T) {
	titled := func(first, second string) func([]Episode) []Episode {
		return func(eps []Episode) []Episode {
			eps = citySushiEpisodes(2)(eps)
			eps[0].Title, eps[1].Title = first, second
			return eps
		}
	}
	var cases []episodeCase
	for _, c := range []struct{ first, second, want string }{
		{"Hey, Baby, What's Wrong (1)", "Hey, Baby, What's Wrong (2)", "Hey, Baby, What's Wrong"},
		{"Meet the Guys and Girls of Cycle 20 Part 1", "Meet the Guys and Girls of Cycle 20 Part 2", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 Part1", "Meet the Guys and Girls of Cycle 20 Part2", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 Part01", "Meet the Guys and Girls of Cycle 20 Part02", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 Part 01", "Meet the Guys and Girls of Cycle 20 Part 02", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 part 1", "Meet the Guys and Girls of Cycle 20 part 2", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 pt 1", "Meet the Guys and Girls of Cycle 20 pt 2", "Meet the Guys and Girls of Cycle 20"},
		{"Meet the Guys and Girls of Cycle 20 pt. 1", "Meet the Guys and Girls of Cycle 20 pt. 2", "Meet the Guys and Girls of Cycle 20"},
	} {
		cases = append(cases, episodeCase{name: "collapse " + c.first, format: "{Episode Title}", episodes: titled(c.first, c.second), want: c.want})
	}
	cases = append(cases,
		episodeCase{
			name: "not collapse when titles differ", format: standardWithTitle, naming: withStyle(MultiEpisodeScene),
			episodes: titled("Hello", "World"), want: "South Park - S15E06-E07 - Hello + World",
		},
		episodeCase{name: "not collapse when result is empty", format: "{Episode Title}", episodes: titled("Part 1", "Part 2"), want: "Part 1 + Part 2"},
	)
	runEpisodeCases(t, southParkEpisodeFixture(), cases)
}

// Ported from Sonarr's MultiEpisodeTitleFixture.
func TestMultiEpisodeTitle(t *testing.T) {
	var cases []episodeCase
	for _, c := range []struct{ first, second string }{
		{"Episode Title (1)", "Episode Title (2)"},
		{"Episode Title Part 1", "Episode Title Part 2"},
		{"Episode Title", "Episode Title: Part 2"},
	} {
		cases = append(cases, episodeCase{
			name: c.first + " and " + c.second, format: "{Episode Title} {Quality Full}",
			episodes: func(eps []Episode) []Episode {
				eps = citySushiEpisodes(2)(eps)
				eps[0].Title, eps[1].Title = c.first, c.second
				return eps
			},
			want: "Episode Title HDTV-720p",
		})
	}
	runEpisodeCases(t, southParkEpisodeFixture(), cases)
}

// truncationEpisodes are the episodes of Sonarr's
// TruncatedEpisodeTitlesFixture.
func truncationEpisodes() []Episode {
	titles := []string{
		"Episode Title 1", "Another Episode Title", "Yet Another Episode Title",
		"Yet Another Episode Title Take 2", "Yet Another Episode Title Take 3",
		"Yet Another Episode Title Take 4", "A Really Really Really Really Long Episode Title",
	}
	var eps []Episode
	for i, title := range titles {
		eps = append(eps, Episode{Title: title, SeasonNumber: 1, EpisodeNumber: i + 1})
	}
	return eps
}

// Ported from Sonarr's TruncatedEpisodeTitlesFixture.
func TestTruncatedEpisodeTitles(t *testing.T) {
	const format = "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}"
	const lorem = "Lorem ipsum dolor sit amet, consectetur adipiscing elit Maecenas et magna sem Morbi vitae volutpat quam, id porta arcu Orci varius natoque penatibus et magnis dis parturient montes"
	const loremLong = lorem + " nascetur ridiculus musu Cras vestibulum" //nolint:misspell // Latin placeholder from the fixture.
	first := func(title string) func([]Episode) []Episode {
		return func(eps []Episode) []Episode {
			eps = eps[:1]
			if title != "" {
				eps[0].Title = title
			}
			return eps
		}
	}
	cases := []struct {
		name      string
		series    string
		episodes  func([]Episode) []Episode
		quality   string
		extension string
		want      string
	}{
		{
			name: "truncate with extension", series: "The Fantastic Life of Mr. Sisko",
			episodes: func(eps []Episode) []Episode {
				eps[0].SeasonNumber, eps[0].EpisodeNumber = 2, 18
				eps[0].Title = "This title has to be 197 characters in length, combined with the series title, quality and episode number it becomes 254ish and the extension puts it above the 255 limit and triggers the truncation"
				return eps[:1]
			},
			quality: "Bluray-1080p", extension: ".mkv",
			want: "The Fantastic Life of Mr. Sisko - S02E18 - This title has to be 197 characters in length, combined with the series title, quality and episode number it becomes 254ish and the extension puts it above the 255 limit and triggers the trunc... Bluray-1080p.mkv",
		},
		{
			name: "ellipsis between first and last episode titles", series: "Series Title",
			want: "Series Title - S01E01-02-03-04-05-06-07 - Episode Title 1...A Really Really Really Really Long Episode Title HDTV-720p",
		},
		{
			name: "ellipsis if only first episode title fits", series: lorem,
			want: lorem + " - S01E01-02-03-04-05-06-07 - Episode Title 1... HDTV-720p",
		},
		{
			name: "first episode title partially fits", series: loremLong, episodes: first(""),
			want: loremLong + " - S01E01 - Episode Ti... HDTV-720p",
		},
		{
			name: "measuring series title bytes", series: "Lorém" + loremLong[5:], episodes: first(""),
			want: "Lorém" + loremLong[5:] + " - S01E01 - Episode T... HDTV-720p",
		},
		{
			name: "measuring episode title bytes", series: loremLong, episodes: first("Episodé Title"),
			want: loremLong + " - S01E01 - Episodé T... HDTV-720p",
		},
		{
			name: "measuring episode title bytes middle", series: loremLong, episodes: first("Episode Tétle"),
			want: loremLong + " - S01E01 - Episode T... HDTV-720p",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := seriesRenaming()
			n.MultiEpisodeStyle = MultiEpisodeExtend
			n.StandardEpisodeFormat = format
			eps := truncationEpisodes()
			if c.episodes != nil {
				eps = c.episodes(eps)
			}
			f := sonarrHDTV720p()
			if c.quality != "" {
				f.Quality = c.quality
			}
			var got string
			var err error
			if c.extension != "" {
				got, err = BuildEpisodeFilePath(n, Series{Title: c.series}, eps, f, c.extension)
			} else {
				got, err = BuildEpisodeFileName(n, Series{Title: c.series}, eps, f)
			}
			if err != nil {
				t.Fatalf("build error = %v", err)
			}
			if len(got) > maxFileNameBytes || utf8.RuneCountInString(got) > maxFileNameBytes {
				t.Errorf("len(%q) = %d, want <= %d", got, len(got), maxFileNameBytes)
			}
			if got != c.want {
				t.Errorf("got = %q, want = %q", got, c.want)
			}
		})
	}
}

// Ported from Sonarr's TruncatedReleaseGroupFixture.
func TestTruncatedEpisodeReleaseGroup(t *testing.T) {
	const group = "IWishIWasALittleBitTallerIWishIWasABallerIWishIHadAGirlWhoLookedGoodIWouldCallHerIWishIHadARabbitInAHatWithABatAndASixFourImpala"
	cases := []struct{ name, format, want string }{
		{
			"from beginning", "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}-{ReleaseGroup:12}",
			"The Fantastic Life of Mr. Sisko - S01E01 - Episode Title 1 Bluray-1080p-IWishIWas....mkv",
		},
		{
			"from end", "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}-{ReleaseGroup:-17}",
			"The Fantastic Life of Mr. Sisko - S01E01 - Episode Title 1 Bluray-1080p-...ASixFourImpala.mkv",
		},
	}
	for _, c := range cases {
		n := seriesRenaming()
		n.MultiEpisodeStyle = MultiEpisodeExtend
		n.StandardEpisodeFormat = c.format
		eps := []Episode{{Title: "Episode Title 1", SeasonNumber: 1, EpisodeNumber: 1}}
		f := EpisodeFile{Quality: "Bluray-1080p", ReleaseGroup: group}
		got, err := BuildEpisodeFilePath(n, Series{Title: "The Fantastic Life of Mr. Sisko"}, eps, f, ".mkv")
		if err != nil || got != c.want {
			t.Errorf("%s: BuildEpisodeFilePath() = %q, %v, want = %q", c.name, got, err, c.want)
		}
	}
}

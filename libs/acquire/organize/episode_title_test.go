package organize

import (
	"strconv"
	"testing"
)

// seriesTitleCase is a case of the series title fixtures: a title, a year
// and the expected value of the token.
type seriesTitleCase struct {
	title string
	year  int
	want  string
}

// runSeriesTitleCases names a file with format for each title.
func runSeriesTitleCases(t *testing.T, format string, cases []seriesTitleCase) {
	t.Helper()
	var named []episodeCase
	for _, c := range cases {
		named = append(named, episodeCase{
			name: c.title + " " + strconv.Itoa(c.year), format: format,
			series: withSeriesTitleYear(c.title, c.year), want: c.want,
		})
	}
	runEpisodeCases(t, southParkEpisodeFixture(), named)
}

// unchangedSeriesTitles are the should_not_change_title cases of Sonarr's
// title fixtures.
func unchangedSeriesTitles() []seriesTitleCase {
	var cases []seriesTitleCase
	for _, title := range unchangedTitles {
		cases = append(cases, seriesTitleCase{title, 0, title})
	}
	return cases
}

// Ported from Sonarr's CleanTitleFixture.
func TestSeriesCleanTitle(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitle}", []seriesTitleCase{
		{"Florence + the Machine", 0, "Florence + the Machine"},
		{"Beyoncé X10", 0, "Beyoncé X10"},
		{"Girlfriends' Guide to Divorce", 0, "Girlfriends Guide to Divorce"},
		{"Rule #23: Never Lie to the Kids", 0, "Rule #23 Never Lie to the Kids"},
		{"Anne Hathaway/Florence + The Machine", 0, "Anne Hathaway Florence + The Machine"},
		{"Chris Rock/Prince", 0, "Chris Rock Prince"},
		{"Karma's a B*tch!", 0, "Karmas a B-tch!"},
		{"Ke$ha: My Crazy Beautiful Life", 0, "Ke$ha My Crazy Beautiful Life"},
		{"$#*! My Dad Says", 0, "$#-! My Dad Says"},
		{"Free! - Iwatobi Swim Club", 0, "Free! Iwatobi Swim Club"},
		{"Tamara Ecclestone: Billion $$ Girl", 0, "Tamara Ecclestone Billion $$ Girl"},
		{"Marvel's Agents of S.H.I.E.L.D.", 0, "Marvels Agents of S.H.I.E.L.D"},
		{"Castle (2009)", 0, "Castle 2009"},
		{"Law & Order (UK)", 0, "Law and Order UK"},
		{"Is this okay?", 0, "Is this okay"},
		{"[a] title", 0, "a title"},
		{"backslash \\ backlash", 0, "backslash backlash"},
		{"I'm the Boss", 0, "Im the Boss"},
		{"The Title's", 0, "The Title's"},
		{"I'm after I'm", 0, "Im after I'm"},
		{"I've Been Caught", 0, "Ive Been Caught"},
		{"I'm Lost", 0, "Im Lost"},
		{"That'll Be The Day", 0, "Thatll Be The Day"},
		{"I'd Rather Be Alone", 0, "Id Rather Be Alone"},
		{"I Can't Die", 0, "I Cant Die"},
		{"Won`t Get Fooled Again", 0, "Wont Get Fooled Again"},
		{"Don’t Blink", 0, "Dont Blink"},
		{"The ` Legend of Kings", 0, "The Legend of Kings"},
	})
	t.Run("and as separator for multiple episodes", func(t *testing.T) {
		runEpisodeCases(t, southParkEpisodeFixture(), []episodeCase{{
			name: "episode clean title", format: "{Episode CleanTitle}",
			episodes: func(eps []Episode) []Episode {
				eps = citySushiEpisodes(2)(eps)
				eps[0].Title, eps[1].Title = "Surrender Benson", "Imprisoned Lives"
				return eps
			},
			want: "Surrender Benson and Imprisoned Lives",
		}})
	})
}

// Ported from Sonarr's CleanTitleTheFixture.
func TestSeriesCleanTitleThe(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitleThe}", append([]seriesTitleCase{
		{"The Mist", 0, "Mist, The"},
		{"A Place to Call Home", 0, "Place to Call Home, A"},
		{"An Adventure in Space and Time", 0, "Adventure in Space and Time, An"},
		{"The Flash (2010)", 0, "Flash, The 2010"},
		{"A League Of Their Own (AU)", 0, "League Of Their Own, A AU"},
		{"The Fixer (ZH) (2015)", 0, "Fixer, The ZH 2015"},
		{"The Sixth Sense 2 (Thai)", 0, "Sixth Sense 2, The Thai"},
		{"The Amazing Race (Latin America)", 0, "Amazing Race, The Latin America"},
		{"The Rat Pack (A&E)", 0, "Rat Pack, The AandE"},
		{"The Climax: I (Almost) Got Away With It (2016)", 0, "Climax I Almost Got Away With It, The 2016"},
	}, unchangedSeriesTitles()...))
}

// Ported from Sonarr's CleanTitleTheWithoutYearFixture.
func TestSeriesCleanTitleTheWithoutYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitleTheWithoutYear}", []seriesTitleCase{
		{"The Mist", 2018, "Mist, The"},
		{"The Rat Pack (A&E)", 1999, "Rat Pack, The AandE"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "Climax I Almost Got Away With It, The"},
		{"A", 2017, "A"},
		{"The Alienist", 0, "Alienist, The"},
	})
}

// Ported from Sonarr's CleanTitleTheYearFixture.
func TestSeriesCleanTitleTheYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitleTheYear}", []seriesTitleCase{
		{"The Mist", 2018, "Mist, The 2018"},
		{"The Rat Pack (A&E)", 1999, "Rat Pack, The AandE 1999"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "Climax I Almost Got Away With It, The 2016"},
		{"The Climax: I (Almost) Got Away With It (2016)", 0, "Climax I Almost Got Away With It, The 2016"},
		{"The Climax: I (Almost) Got Away With It", 0, "Climax I Almost Got Away With It, The"},
		{"A", 2017, "A 2017"},
	})
	// should_not_include_0_for_year uses {Series TitleTheYear}.
	runSeriesTitleCases(t, "{Series TitleTheYear}", []seriesTitleCase{{"The Alienist", 0, "Alienist, The"}})
}

// Ported from Sonarr's CleanTitleWithoutYearFixture.
func TestSeriesCleanTitleWithoutYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitleWithoutYear}", []seriesTitleCase{
		{"The Mist", 2018, "The Mist"},
		{"The Rat Pack (A&E)", 1999, "The Rat Pack AandE"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "The Climax I Almost Got Away With It"},
		{"The Series Title's (2016)", 2016, "The Series Titles"},
		{"The Series Title's", 2016, "The Series Title's"},
		{"The Alienist", 0, "The Alienist"},
	})
}

// Ported from Sonarr's CleanTitleYearFixture.
func TestSeriesCleanTitleYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series CleanTitleYear}", []seriesTitleCase{
		{"The Mist", 2018, "The Mist 2018"},
		{"The Rat Pack (A&E)", 1999, "The Rat Pack AandE 1999"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "The Climax I Almost Got Away With It 2016"},
		{"The Series Title's", 2016, "The Series Titles 2016"},
		{"The Alienist", 0, "The Alienist"},
	})
}

// Ported from Sonarr's TitleTheFixture.
func TestSeriesTitleThe(t *testing.T) {
	runSeriesTitleCases(t, "{Series TitleThe}", append([]seriesTitleCase{
		{"The Mist", 0, "Mist, The"},
		{"A Place to Call Home", 0, "Place to Call Home, A"},
		{"An Adventure in Space and Time", 0, "Adventure in Space and Time, An"},
		{"The Flash (2010)", 0, "Flash, The (2010)"},
		{"A League Of Their Own (AU)", 0, "League Of Their Own, A (AU)"},
		{"The Fixer (ZH) (2015)", 0, "Fixer, The (ZH) (2015)"},
		{"The Sixth Sense 2 (Thai)", 0, "Sixth Sense 2, The (Thai)"},
		{"The Amazing Race (Latin America)", 0, "Amazing Race, The (Latin America)"},
		{"The Rat Pack (A&E)", 0, "Rat Pack, The (A&E)"},
		{"The Climax: I (Almost) Got Away With It (2016)", 0, "Climax - I (Almost) Got Away With It, The (2016)"},
	}, unchangedSeriesTitles()...))
}

// Ported from Sonarr's TitleTheWithoutYearFixture.
func TestSeriesTitleTheWithoutYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series TitleTheWithoutYear}", []seriesTitleCase{
		{"The Mist", 2018, "Mist, The"},
		{"The Rat Pack (A&E)", 1999, "Rat Pack, The (A&E)"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "Climax - I (Almost) Got Away With It, The"},
		{"A", 2017, "A"},
		{"The Alienist", 0, "Alienist, The"},
	})
}

// Ported from Sonarr's TitleTheYearFixture.
func TestSeriesTitleTheYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series TitleTheYear}", []seriesTitleCase{
		{"The Mist", 2018, "Mist, The (2018)"},
		{"The Rat Pack (A&E)", 1999, "Rat Pack, The (A&E) (1999)"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "Climax - I (Almost) Got Away With It, The (2016)"},
		{"A", 2017, "A (2017)"},
		{"The Alienist", 0, "Alienist, The"},
	})
}

// Ported from Sonarr's TitleWithoutYearFixture.
func TestSeriesTitleWithoutYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series TitleWithoutYear}", []seriesTitleCase{
		{"The Mist", 2018, "The Mist"},
		{"The Rat Pack (A&E)", 1999, "The Rat Pack (A&E)"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "The Climax - I (Almost) Got Away With It"},
		{"The Alienist", 0, "The Alienist"},
	})
}

// Ported from Sonarr's TitleYearFixture.
func TestSeriesTitleYear(t *testing.T) {
	runSeriesTitleCases(t, "{Series TitleYear}", []seriesTitleCase{
		{"The Mist", 2018, "The Mist (2018)"},
		{"The Rat Pack (A&E)", 1999, "The Rat Pack (A&E) (1999)"},
		{"The Climax: I (Almost) Got Away With It (2016)", 2016, "The Climax - I (Almost) Got Away With It (2016)"},
		{"The Alienist", 0, "The Alienist"},
	})
}

// Ported from Sonarr's ColonReplacementFixture.
func TestSeriesColonReplacement(t *testing.T) {
	cases := []episodeCase{
		{name: "colon followed by space with space dash space by default", format: "{Series Title}", want: "CSI - Vegas"},
	}
	for _, c := range []struct {
		title       string
		replacement ColonReplacement
		custom      string
		want        string
	}{
		{"CSI: Vegas", ColonSmart, "", "CSI - Vegas"},
		{"CSI: Vegas", ColonDash, "", "CSI- Vegas"},
		{"CSI: Vegas", ColonDelete, "", "CSI Vegas"},
		{"CSI: Vegas", ColonSpaceDash, "", "CSI - Vegas"},
		{"CSI: Vegas", ColonSpaceDashSpace, "", "CSI - Vegas"},
		{"Series:Title", ColonSmart, "", "Series-Title"},
		{"Series:Title", ColonDash, "", "Series-Title"},
		{"Series:Title", ColonDelete, "", "SeriesTitle"},
		{"Series:Title", ColonSpaceDash, "", "Series -Title"},
		{"Series:Title", ColonSpaceDashSpace, "", "Series - Title"},
		{"Series: Title", ColonCustom, "꞉", "Series꞉ Title"},
		{"Series: Title", ColonCustom, "∶", "Series∶ Title"},
	} {
		cases = append(cases, episodeCase{
			name: c.title + " " + string(c.replacement) + " " + c.custom, format: "{Series Title}", series: withSeriesTitle(c.title),
			naming: func(n *SeriesNaming) { n.ColonReplacement, n.CustomColonReplacement = c.replacement, c.custom },
			want:   c.want,
		})
	}
	fx := southParkEpisodeFixture()
	fx.series.Title = "CSI: Vegas"
	fx.episodes = []Episode{{Title: "What Happens in Vegas", SeasonNumber: 1, EpisodeNumber: 6, AbsoluteEpisodeNumber: 100}}
	runEpisodeCases(t, fx, cases)
}

// Ported from Sonarr's ReplaceCharacterFixture.
func TestSeriesReplaceCharacters(t *testing.T) {
	runSeriesTitleCases(t, "{Series Title}", []seriesTitleCase{
		{"CSI: Crime Scene Investigation", 0, "CSI - Crime Scene Investigation"},
		{"Code:Breaker", 0, "Code-Breaker"},
		{"Back Slash\\", 0, "Back Slash+"},
		{"Forward Slash/", 0, "Forward Slash+"},
		{"Greater Than>", 0, "Greater Than"},
		{"Less Than<", 0, "Less Than"},
		{"Question Mark?", 0, "Question Mark!"},
		{"Aster*sk", 0, "Aster-sk"},
		{"Colon: Two Periods", 0, "Colon - Two Periods"},
		{"Pipe|", 0, "Pipe"},
		{"Quotes\"", 0, "Quotes"},
	})
}

// Ported from Sonarr's ReservedDeviceNameFixture.
func TestSeriesReservedDeviceNames(t *testing.T) {
	for _, c := range []struct{ title, want string }{{"Con Game", "Con_Game"}, {"Com1 Sat", "Com1_Sat"}} {
		n := seriesRenaming()
		n.SeriesFolderFormat = "{Series.Title}"
		n.SeasonFolderFormat = "{Series.Title} - Season {Season:00}"
		n.StandardEpisodeFormat = "{Series.Title} - S{Season:00}E{Episode:00}"
		s := Series{Title: c.title}
		if got := SeriesFolder(n, s); got != c.want {
			t.Errorf("SeriesFolder(%q) = %q, want = %q", c.title, got, c.want)
		}
		if got, want := SeasonFolder(n, s, 1), c.want+" - Season 01"; got != want {
			t.Errorf("SeasonFolder(%q) = %q, want = %q", c.title, got, want)
		}
		got, err := BuildEpisodeFileName(n, s, []Episode{citySushi()}, sonarrHDTV720p())
		if want := c.want + " - S15E06"; err != nil || got != want {
			t.Errorf("BuildEpisodeFileName(%q) = %q, %v, want = %q", c.title, got, err, want)
		}
	}
}

// Ported from Sonarr's SeriesTitleFirstCharacterFixture.
func TestSeriesTitleFirstCharacter(t *testing.T) {
	var cases []episodeCase
	for _, c := range []struct{ title, want string }{
		{"The Mist", "M/The Mist"},
		{"A", "A/A"},
		{"30 Rock", "3/30 Rock"},
		{"The '80s Greatest", "8/The '80s Greatest"},
		{"좀비버스", "좀/좀비버스"},
		{"¡Mucha Lucha!", "M/¡Mucha Lucha!"},
		{".hack", "H/hack"},
		{"Ütopya", "U/Ütopya"},
		{"Æon Flux", "A/Æon Flux"},
	} {
		cases = append(cases, episodeCase{
			name: c.title, format: `{Series TitleFirstCharacter}\{Series Title}`, series: withSeriesTitle(c.title), want: c.want,
		})
	}
	cases = append(cases, episodeCase{
		name: "lower case first character", format: `{series titlefirstcharacter}\{series title}`,
		series: withSeriesTitle("Westworld"), want: "w/westworld",
	})
	runSeriesFolderCases(t, southParkEpisodeFixture(), cases)
}

// Ported from Sonarr's SeriesYearFixture.
func TestSeriesYearToken(t *testing.T) {
	var cases []episodeCase
	for _, c := range []struct {
		title string
		year  int
		want  string
	}{
		{"The Mist", 2017, "2017/The Mist"},
		{"A", 2021, "2021/A"},
		{"30 Rock", 2006, "2006/30 Rock"},
	} {
		cases = append(cases, episodeCase{
			name: c.title, format: `{Series Year}\{Series Title}`, series: withSeriesTitleYear(c.title, c.year), want: c.want,
		})
	}
	runSeriesFolderCases(t, southParkEpisodeFixture(), cases)
}

// Ported from Sonarr's TruncatedSeriesTitleFixture.
func TestTruncatedSeriesTitle(t *testing.T) {
	title := withSeriesTitle("The Fantastic Life of Mr. Sisko")
	runSeriesFolderCases(t, southParkEpisodeFixture(), []episodeCase{
		{name: "title", format: "{Series Title:16}", series: title, want: "The Fantastic..."},
		{name: "title the", format: "{Series TitleThe:17}", series: title, want: "Fantastic Life..."},
		{name: "clean title from the end", format: "{Series CleanTitle:-13}", series: title, want: "...Mr. Sisko"},
	})
}

// Ported from Sonarr's IdFixture.
func TestSeriesIDTokens(t *testing.T) {
	ids := func(s *Series) { s.ImdbID, s.TvdbID, s.TvMazeID, s.TmdbID = "tt12345", 12345, 1, 1 }
	runSeriesFolderCases(t, episodeFixture{naming: DefaultSeriesNaming(), series: Series{Title: "Series Title"}}, []episodeCase{
		{name: "imdb id", format: "{Series Title} ({ImdbId})", series: ids, want: "Series Title (tt12345)"},
		{name: "tvdb id", format: "{Series Title} ({TvdbId})", series: ids, want: "Series Title (12345)"},
		{name: "tvmaze id", format: "{Series Title} ({TvMazeId})", series: ids, want: "Series Title (1)"},
		{name: "tmdb id", format: "{Series Title} ({TmdbId})", series: ids, want: "Series Title (1)"},
	})
}

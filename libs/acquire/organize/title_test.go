package organize

import "testing"

func movieTitleFixture() fixture {
	return fixture{naming: renaming(), movie: Movie{Title: "Movie Title", Year: 1}, file: hdtv720p()}
}

// unchangedTitles are the should_not_change_title cases of the title
// fixtures.
var unchangedTitles = []string{"A", "Anne", "Theodore", "3%"}

// Ported from Radarr's CleanCollectionTheFixture.
func TestCleanCollectionThe(t *testing.T) {
	cases := []struct{ collection, want string }{
		{"The Badger's Collection", "Badgers Collection, The"},
		{"@ The Movies Collection", "@ The Movies Collection"},
		{"A Stupid/Idiotic Collection", "Stupid Idiotic Collection, A"},
		{"An Astounding & Amazing Collection", "Astounding and Amazing Collection, An"},
		{"The Amazing Animal-Hero's Collection (2001)", "Amazing Animal-Heros Collection, The 2001"},
		{"A Different Movië (AU)", "Different Movie, A AU"},
		{"The Repairër (ZH) (2015)", "Repairer, The ZH 2015"},
		{"The Eighth Sensë 2 (Thai)", "Eighth Sense 2, The Thai"},
		{"The Astonishing Jæg (Latin America)", "Astonishing Jaeg, The Latin America"},
		{"The Hampster Pack (B&F)", "Hampster Pack, The BandF"},
		{"The Gásm: I (Almost) Got Away With It (1900)", "Gasm I Almost Got Away With It, The 1900"},
		{"", ""},
	}
	for _, title := range unchangedTitles {
		cases = append(cases, struct{ collection, want string }{title, title})
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{name: c.collection, format: "{Movie CleanCollectionThe}", movie: withCollection(c.collection), want: c.want})
	}
	runFileCases(t, movieTitleFixture(), named)
}

// Ported from Radarr's CleanTitleFixture.
func TestCleanTitleToken(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Florence + the Machine", "Florence + the Machine"},
		{"Beyoncé X10", "Beyonce X10"},
		{"Girlfriends' Guide to Divorce", "Girlfriends Guide to Divorce"},
		{"Rule #23: Never Lie to the Kids", "Rule #23 Never Lie to the Kids"},
		{"Anne Hathaway/Florence + The Machine", "Anne Hathaway Florence + The Machine"},
		{"Chris Rock/Prince", "Chris Rock Prince"},
		{"Karma's a B*tch!", "Karmas a B-tch!"},
		{"Ke$ha: My Crazy Beautiful Life", "Ke$ha My Crazy Beautiful Life"},
		{"$#*! My Dad Says", "$#-! My Dad Says"},
		{"Free! - Iwatobi Swim Club", "Free! Iwatobi Swim Club"},
		{"Tamara Ecclestone: Billion $$ Girl", "Tamara Ecclestone Billion $$ Girl"},
		{"Marvel's Agents of S.H.I.E.L.D.", "Marvels Agents of S.H.I.E.L.D"},
		{"Castle (2009)", "Castle 2009"},
		{"Law & Order (UK)", "Law and Order UK"},
		{"Is this okay?", "Is this okay"},
		{"[a] title", "a title"},
		{"backslash \\ backlash", "backslash backlash"},
		{"I'm the Boss", "Im the Boss"},
		{"I've Been Caught", "Ive Been Caught"},
		{"I'm Lost", "Im Lost"},
		{"That'll Be The Day", "Thatll Be The Day"},
		{"I'd Rather Be Alone", "Id Rather Be Alone"},
		{"I Can't Die", "I Cant Die"},
		{"Won`t Get Fooled Again", "Wont Get Fooled Again"},
		{"Don’t Blink", "Dont Blink"},
		{"The ` Legend of Kings", "The Legend of Kings"},
		{"Joker: Folie à deux", "Joker Folie a deux"},
	}
	fx := fixture{naming: renaming(), movie: Movie{Title: "South Park"}, file: File{Quality: "Unknown", ReleaseGroup: "SonarrTest"}}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{name: c.title, format: "{Movie CleanTitle}", movie: withTitle(c.title), want: c.want})
	}
	runFileCases(t, fx, named)
}

// Ported from Radarr's CleanTitleTheFixture.
func TestCleanTitleTheToken(t *testing.T) {
	cases := []struct{ title, want string }{
		{"The Mist", "Mist, The"},
		{"A Place to Call Home", "Place to Call Home, A"},
		{"An Adventure in Space and Time", "Adventure in Space and Time, An"},
		{"The Flash (2010)", "Flash, The 2010"},
		{"A League Of Their Own (AU)", "League Of Their Own, A AU"},
		{"The Fixer (ZH) (2015)", "Fixer, The ZH 2015"},
		{"The Sixth Sense 2 (Thai)", "Sixth Sense 2, The Thai"},
		{"The Amazing Race (Latin America)", "Amazing Race, The Latin America"},
		{"The Rat Pack (A&E)", "Rat Pack, The AandE"},
		{"The Climax: I (Almost) Got Away With It (2016)", "Climax I Almost Got Away With It, The 2016"},
		{"", ""},
	}
	for _, title := range unchangedTitles {
		cases = append(cases, struct{ title, want string }{title, title})
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{name: c.title, format: "{Movie CleanTitleThe}", movie: withTitle(c.title), want: c.want})
	}
	runFileCases(t, movieTitleFixture(), named)
}

// Ported from Radarr's CollectionTheFixture.
func TestCollectionThe(t *testing.T) {
	cases := []struct{ collection, want string }{
		{"The Badger Collection", "Badger Collection, The"},
		{"The Mover Collection", "Mover Collection, The"},
		{"A Stupid Collection", "Stupid Collection, A"},
		{"An Astounding Collection", "Astounding Collection, An"},
		{"The Amazing Animal-Hero Collection (2001)", "Amazing Animal-Hero Collection, The (2001)"},
		{"A Different Movie (AU)", "Different Movie, A (AU)"},
		{"The Repairer (ZH) (2015)", "Repairer, The (ZH) (2015)"},
		{"The Eighth Sense 2 (Thai)", "Eighth Sense 2, The (Thai)"},
		{"The Astonishing Jog (Latin America)", "Astonishing Jog, The (Latin America)"},
		{"The Hampster Pack (B&F)", "Hampster Pack, The (B&F)"},
		{"The Gasm: I (Almost) Got Away With It (1900)", "Gasm - I (Almost) Got Away With It, The (1900)"},
	}
	for _, title := range unchangedTitles {
		cases = append(cases, struct{ collection, want string }{title, title})
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{name: c.collection, format: "{Movie CollectionThe}", movie: withCollection(c.collection), want: c.want})
	}
	runFileCases(t, movieTitleFixture(), named)
}

// Ported from Radarr's TitleTheFixture.
func TestTitleTheToken(t *testing.T) {
	cases := []struct{ title, want string }{
		{"The Mist", "Mist, The"},
		{"A Place to Call Home", "Place to Call Home, A"},
		{"An Adventure in Space and Time", "Adventure in Space and Time, An"},
		{"The Flash (2010)", "Flash, The (2010)"},
		{"A League Of Their Own (AU)", "League Of Their Own, A (AU)"},
		{"The Fixer (ZH) (2015)", "Fixer, The (ZH) (2015)"},
		{"The Sixth Sense 2 (Thai)", "Sixth Sense 2, The (Thai)"},
		{"The Amazing Race (Latin America)", "Amazing Race, The (Latin America)"},
		{"The Rat Pack (A&E)", "Rat Pack, The (A&E)"},
		{"The Climax: I (Almost) Got Away With It (2016)", "Climax - I (Almost) Got Away With It, The (2016)"},
	}
	for _, title := range unchangedTitles {
		cases = append(cases, struct{ title, want string }{title, title})
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{name: c.title, format: "{Movie TitleThe}", movie: withTitle(c.title), want: c.want})
	}
	runFileCases(t, fixture{naming: renaming(), movie: Movie{Title: "Batman"}, file: hdtv720p()}, named)
}

// Ported from Radarr's MovieTitleFirstCharacterFixture.
func TestTitleFirstCharacterToken(t *testing.T) {
	cases := []struct{ title, parent, child string }{
		{"The Mist", "M", "The Mist"},
		{"A", "A", "A"},
		{"30 Rock", "3", "30 Rock"},
		{"The '80s Greatest", "8", "The '80s Greatest"},
		{"좀비버스", "좀", "좀비버스"},
		{"¡Mucha Lucha!", "M", "¡Mucha Lucha!"},
		{".hack", "H", "hack"},
		{"Ütopya", "U", "Ütopya"},
		{"Æon Flux", "A", "Æon Flux"},
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{
			name: c.title, format: `{Movie TitleFirstCharacter}\{Movie Title}`, movie: withTitle(c.title),
			want: c.parent + "/" + c.child,
		})
	}
	named = append(named, nameCase{
		name: "lower case first character", format: `{movie titlefirstcharacter}\{movie title}`,
		movie: withTitle("Westworld"), want: "w/westworld",
	})
	runFolderCases(t, fixture{naming: renaming(), movie: Movie{Title: "Title1"}}, named)
}

// Ported from Radarr's ColonReplacementFixture.
func TestColonReplacement(t *testing.T) {
	cases := []nameCase{
		{name: "colon followed by space with space dash space by default", format: "{Movie Title}", want: "CSI - Vegas"},
	}
	for _, c := range []struct {
		title       string
		replacement ColonReplacement
		want        string
	}{
		{"CSI: Vegas", ColonSmart, "CSI - Vegas"},
		{"CSI: Vegas", ColonDash, "CSI- Vegas"},
		{"CSI: Vegas", ColonDelete, "CSI Vegas"},
		{"CSI: Vegas", ColonSpaceDash, "CSI - Vegas"},
		{"CSI: Vegas", ColonSpaceDashSpace, "CSI - Vegas"},
		{"Movie:Title", ColonSmart, "Movie-Title"},
		{"Movie:Title", ColonDash, "Movie-Title"},
		{"Movie:Title", ColonDelete, "MovieTitle"},
		{"Movie:Title", ColonSpaceDash, "Movie -Title"},
		{"Movie:Title", ColonSpaceDashSpace, "Movie - Title"},
	} {
		cases = append(cases, nameCase{
			name: c.title + " " + string(c.replacement), format: "{Movie Title}", movie: withTitle(c.title),
			naming: func(n *Naming) { n.ColonReplacement = c.replacement }, want: c.want,
		})
	}
	runFileCases(t, fixture{naming: renaming(), movie: Movie{Title: "CSI: Vegas"}, file: hdtv720p()}, cases)
}

// Ported from Radarr's ReservedDeviceNameFixture.
func TestReservedDeviceNames(t *testing.T) {
	cases := []struct {
		title string
		year  int
		want  string
	}{
		{"Con Game", 2021, "Con_Game (2021)"},
		{"Com1 Sat", 2021, "Com1_Sat (2021)"},
	}
	var named []nameCase
	for _, c := range cases {
		named = append(named, nameCase{
			name: c.title, format: "{Movie.Title} ({Release Year})",
			movie: func(m *Movie) { m.Title, m.Year = c.title, c.year }, want: c.want,
		})
	}
	fx := fixture{naming: renaming(), movie: Movie{Title: "Title1"}, file: hdtv720p()}
	t.Run("folder", func(t *testing.T) { runFolderCases(t, fx, named) })
	t.Run("file", func(t *testing.T) { runFileCases(t, fx, named) })
}

// Ported from Radarr's TruncatedMovieTitleFixture.
func TestTruncatedMovieTitle(t *testing.T) {
	title := withTitle("The Fantastic Life of Mr. Sisko")
	runFolderCases(t, movieTitleFixture(), []nameCase{
		{name: "title", format: "{Movie Title:16}", movie: title, want: "The Fantastic..."},
		{name: "title the", format: "{Movie TitleThe:17}", movie: title, want: "Fantastic Life..."},
		{name: "clean title from the end", format: "{Movie CleanTitle:-13}", movie: title, want: "...Mr. Sisko"},
	})
}

// Ported from Radarr's TruncatedReleaseGroupFixture.
func TestTruncatedReleaseGroup(t *testing.T) {
	const group = "IWishIWasALittleBitTallerIWishIWasABallerIWishIHadAGirlWhoLookedGoodIWouldCallHerIWishIHadARabbitInAHatWithABatAndASixFourImpala"
	file := func(f *File) { f.Quality, f.ReleaseGroup = "Bluray-1080p", group }
	movie := func(m *Movie) { m.Title, m.Year = "The Fantastic Life of Mr. Sisko", 2024 }
	runFileCases(t, movieTitleFixture(), []nameCase{
		{
			name: "from beginning", format: "{Movie Title} ({Release Year}) {Quality Full}-{ReleaseGroup:12}",
			movie: movie, file: file, want: "The Fantastic Life of Mr. Sisko (2024) Bluray-1080p-IWishIWas...",
		},
		{
			name: "from end", format: "{Movie Title} ({Release Year}) {Quality Full}-{ReleaseGroup:-17}",
			movie: movie, file: file, want: "The Fantastic Life of Mr. Sisko (2024) Bluray-1080p-...ASixFourImpala",
		},
	})
}

// Ported from Radarr's CleanFilenameFixture.
func TestCleanFileName(t *testing.T) {
	cases := []struct{ name, want string }{
		{"Mission: Impossible - no [HDTV-720p]", "Mission - Impossible - no [HDTV-720p]"},
		{".45 (2006)", "45 (2006)"},
		{" The Movie Title", "The Movie Title"},
		{"The Movie Title ", "The Movie Title"},
		{" The Movie Title ", "The Movie Title"},
	}
	for _, c := range cases {
		if got := CleanFileName(c.name); got != c.want {
			t.Errorf("CleanFileName(%q) = %q, want = %q", c.name, got, c.want)
		}
	}
}

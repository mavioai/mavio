package naming

import "testing"

type wantStack struct {
	name  string // empty: only the file count is checked
	files int
}

var stackFacts = []struct {
	fact  string
	files []FileEntry
	want  []wantStack
}{
	{"TestSimpleStack", files("Bad Boys (2006) part1.mkv", "Bad Boys (2006) part2.mkv", "Bad Boys (2006) part3.mkv",
		"Bad Boys (2006) part4.mkv", "Bad Boys (2006)-trailer.mkv"), []wantStack{{"Bad Boys (2006)", 4}}},
	{"TestFalsePositives", files("Bad Boys (2006).mkv", "Bad Boys (2007).mkv"), nil},
	{"TestFalsePositives2", files("Bad Boys 2006.mkv", "Bad Boys 2007.mkv"), nil},
	{"TestFalsePositives3", files("300 (2006).mkv", "300 (2007).mkv"), nil},
	{"TestFalsePositives4", files("300 2006.mkv", "300 2007.mkv"), nil},
	{"TestFalsePositives5", files("Star Trek 1 - The motion picture.mkv", "Star Trek 2- The wrath of khan.mkv"), nil},
	{"TestFalsePositives6", files("Red Riding in the Year of Our Lord 1983 (2009).mkv",
		"Red Riding in the Year of Our Lord 1980 (2009).mkv", "Red Riding in the Year of Our Lord 1974 (2009).mkv"), nil},
	{"TestStackName", files("d:/movies/300 2006 part1.mkv", "d:/movies/300 2006 part2.mkv"), []wantStack{{"300 2006", 2}}},
	{"ResolveFiles_GivenPartInMiddleOfName_ReturnsNoStack", files(
		"Bad Boys (2006).part1.stv.unrated.multi.1080p.bluray.x264-rough.mkv",
		"Bad Boys (2006).part2.stv.unrated.multi.1080p.bluray.x264-rough.mkv",
		"Bad Boys (2006).part3.stv.unrated.multi.1080p.bluray.x264-rough.mkv",
		"Bad Boys (2006).part4.stv.unrated.multi.1080p.bluray.x264-rough.mkv",
		"Bad Boys (2006)-trailer.mkv"), nil},
	{"ResolveFiles_FileNamesWithMissingPartType_ReturnsNoStack", files("Bad Boys (2006).mkv", "Bad Boys (2006) 1.mkv",
		"Bad Boys (2006) 2.mkv", "Bad Boys (2006) 3.mkv", "Bad Boys (2006)-trailer.mkv"), nil},
	{"TestSimpleStackWithNumericName", files("300 (2006) part1.mkv", "300 (2006) part2.mkv", "300 (2006) part3.mkv",
		"300 (2006) part4.mkv", "300 (2006)-trailer.mkv"), []wantStack{{"300 (2006)", 4}}},
	{"TestMixedExpressionsNotAllowed", files("Bad Boys (2006) part1.mkv", "Bad Boys (2006) part2.mkv",
		"Bad Boys (2006) part3.mkv", "Bad Boys (2006) parta.mkv", "Bad Boys (2006)-trailer.mkv"), []wantStack{{"Bad Boys (2006)", 3}}},
	{"TestDualStacks", files("Bad Boys (2006) part1.mkv", "Bad Boys (2006) part2.mkv", "Bad Boys (2006) part3.mkv",
		"Bad Boys (2006) part4.mkv", "Bad Boys (2006)-trailer.mkv", "300 (2006) part1.mkv", "300 (2006) part2.mkv",
		"300 (2006) part3.mkv", "300 (2006)-trailer.mkv"), []wantStack{{"300 (2006)", 3}, {"Bad Boys (2006)", 4}}},
	{"TestDirectories", dirs("blah blah - cd 1", "blah blah - cd 2"), []wantStack{{"blah blah", 2}}},
	// All files are separate movies.
	{"TestMissingParttype", files("300a.mkv", "300b.mkv", "300c.mkv", "300-trailer.mkv"), nil},
	{"TestFailSequence", files("300 part1.mkv", "300 part2.mkv", "Avatar", "Avengers part1.mkv", "Avengers part2.mkv",
		"Avengers part3.mkv"), []wantStack{{"300", 2}, {"Avengers", 3}}},
	{"TestMixedExpressions", files("Bad Boys (2006) part1.mkv", "Bad Boys (2006) part2.mkv", "Bad Boys (2006) part3.mkv",
		"Bad Boys (2006) part4.mkv", "Bad Boys (2006)-trailer.mkv", "300 (2006) parta.mkv", "300 (2006) partb.mkv",
		"300 (2006) partc.mkv", "300 (2006) partd.mkv", "300 (2006)-trailer.mkv", "300a.mkv", "300b.mkv", "300c.mkv",
		"300-trailer.mkv"), []wantStack{{"300 (2006)", 4}, {"Bad Boys (2006)", 4}}},
	{"TestAlphaLimitOfFour", files("300 (2006) parta.mkv", "300 (2006) partb.mkv", "300 (2006) partc.mkv",
		"300 (2006) partd.mkv", "300 (2006) parte.mkv", "300 (2006) partf.mkv", "300 (2006) partg.mkv",
		"300 (2006)-trailer.mkv"), []wantStack{{"300 (2006)", 4}}},
	{"TestMixed", []FileEntry{
		{Path: "Bad Boys (2006) part1.mkv"},
		{Path: "Bad Boys (2006) part2.mkv"},
		{Path: "300 (2006) part2", IsDir: true},
		{Path: "300 (2006) part3", IsDir: true},
		{Path: "300 (2006) part1", IsDir: true},
	}, []wantStack{{"300 (2006)", 3}, {"Bad Boys (2006)", 2}}},
	// No part, disc or the like: no stack.
	{"TestNamesWithoutParts", files("Harry Potter and the Deathly Hallows.mkv", "Harry Potter and the Deathly Hallows 1.mkv",
		"Harry Potter and the Deathly Hallows 2.mkv", "Harry Potter and the Deathly Hallows 3.mkv",
		"Harry Potter and the Deathly Hallows 4.mkv"), nil},
	{"TestNumbersAppearingBeforePartNumber", files("Neverland (2011)[720p][PG][Voted 6.5][Family-Fantasy]part1.mkv",
		"Neverland (2011)[720p][PG][Voted 6.5][Family-Fantasy]part2.mkv"), []wantStack{{"", 2}}},
	{"TestMultiDiscs", dirs("M:/Movies (DVD)/Movies (Musical)/The Sound of Music/The Sound of Music (1965) (Disc 01)",
		"M:/Movies (DVD)/Movies (Musical)/The Sound of Music/The Sound of Music (1965) (Disc 02)"), []wantStack{{"", 2}}},
}

func files(paths ...string) []FileEntry { return entries(paths, false) }
func dirs(paths ...string) []FileEntry  { return entries(paths, true) }

func TestPortedStack(t *testing.T) {
	facts := map[string]string{}
	for _, f := range stackFacts {
		facts[f.fact] = "TestStacks"
	}
	portedCases(t, "video/stack.json", ported{facts: facts})
}

func TestStacks(t *testing.T) {
	for _, tt := range stackFacts {
		t.Run(tt.fact, func(t *testing.T) {
			got := parser.ResolveStacks(tt.files)
			if len(got) != len(tt.want) {
				t.Fatalf("stacks: got = %+v, want = %+v", got, tt.want)
			}
			for i, w := range tt.want {
				if (w.name != "" && got[i].Name != w.name) || len(got[i].Files) != w.files {
					t.Errorf("stack %d: got = %q with %d files, want = %q with %d", i, got[i].Name, len(got[i].Files), w.name, w.files)
				}
			}
		})
	}
}

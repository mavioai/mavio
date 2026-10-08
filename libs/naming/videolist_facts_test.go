package naming

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// resolveVideos resolves paths as files (or folders) and groups them.
func resolveVideos(paths []string, isDir bool, o VideoListOptions) []Video {
	var files []VideoFile
	for _, path := range paths {
		if f, ok := parser.ResolveVideo(path, isDir, true, ""); ok {
			files = append(files, f)
		}
	}
	return parser.ResolveVideos(files, o)
}

const tr = core.ExtraTrailer

var videoListFacts = []struct {
	fact  string
	paths []string
	isDir bool
	count int
	// extras lists the expected extra kind of the first results.
	extras []core.ExtraKind
	// stacks maps a video name to its expected number of files.
	stacks map[string]int
}{
	{
		fact: "TestStackAndExtras",
		paths: []string{
			"Harry Potter and the Deathly Hallows-trailer.mkv", "Harry Potter and the Deathly Hallows.trailer.mkv",
			"Harry Potter and the Deathly Hallows part1.mkv", "Harry Potter and the Deathly Hallows part2.mkv",
			"Harry Potter and the Deathly Hallows part3.mkv", "Harry Potter and the Deathly Hallows part4.mkv",
			"Batman-deleted.mkv", "Batman-sample.mkv", "Batman-trailer.mkv", "Batman part1.mkv", "Batman part2.mkv",
			"Batman part3.mkv", "Avengers.mkv", "Avengers-trailer.mkv",
			// Extra keywords without a video to attach to: standalone videos.
			"trailer.mkv", "WillyWonka-trailer.mkv",
		},
		count:  11,
		extras: []core.ExtraKind{2: "", 3: tr, 4: tr, 5: core.ExtraDeletedScene, 6: core.ExtraSample, 7: tr, 8: tr, 9: tr, 10: tr},
		stacks: map[string]int{"Batman": 3, "Harry Potter and the Deathly Hallows": 4},
	},
	{fact: "TestWithMetadata", paths: []string{"300.mkv", "300.nfo"}, count: 1},
	{fact: "TestWithExtra", paths: []string{"300.mkv", "300 - trailer.mkv"}, count: 2, extras: []core.ExtraKind{"", tr}},
	{
		fact: "TestVariationWithFolderName", paths: []string{"X-Men Days of Future Past - 1080p.mkv", "X-Men Days of Future Past-trailer.mp4"},
		count: 2, extras: []core.ExtraKind{"", tr},
	},
	{fact: "TestTrailer2", paths: []string{
		"X-Men Days of Future Past - 1080p.mkv", "X-Men Days of Future Past-trailer.mp4",
		"X-Men Days of Future Past-trailer2.mp4",
	}, count: 3, extras: []core.ExtraKind{"", tr, tr}},
	{fact: "Resolve_SameNameAndYear_ReturnsSingleItem", paths: []string{
		"Looper (2012)-trailer.mkv", "Looper 2012-trailer.mkv",
		"Looper.2012.bluray.720p.x264.mkv",
	}, count: 3, extras: []core.ExtraKind{"", tr, tr}},
	{fact: "Resolve_TrailerMatchesFolderName_ReturnsSingleItem", paths: []string{
		"/movies/Looper (2012)/Looper (2012)-trailer.mkv",
		"/movies/Looper (2012)/Looper.bluray.720p.x264.mkv",
	}, count: 2, extras: []core.ExtraKind{"", tr}},
	// Separate, unrelated videos.
	{fact: "TestSeparateFiles", paths: []string{"My video 1.mkv", "My video 2.mkv", "My video 3.mkv", "My video 4.mkv", "My video 5.mkv"}, count: 5},
	{fact: "TestMultiDisc", paths: []string{
		"M:/Movies (DVD)/Movies (Musical)/Sound of Music (1965)/Sound of Music Disc 1",
		"M:/Movies (DVD)/Movies (Musical)/Sound of Music (1965)/Sound of Music Disc 2",
	}, isDir: true, count: 1},
	{fact: "TestPoundSign", paths: []string{"My movie #1.mp4", "My movie #2.mp4"}, isDir: true, count: 2},
	{fact: "TestStackedWithTrailer", paths: []string{
		"No (2012) part1.mp4", "No (2012) part2.mp4", "No (2012) part1-trailer.mp4",
		"No (2012)-trailer.mp4",
	}, count: 3, extras: []core.ExtraKind{"", tr, tr}},
	{fact: "TestExtrasByFolderName", paths: []string{
		"/Movies/Top Gun (1984)/movie.mp4", "/Movies/Top Gun (1984)/Top Gun (1984)-trailer.mp4",
		"/Movies/Top Gun (1984)/Top Gun (1984)-trailer2.mp4", "/Movies/trailer.mp4",
	}, count: 4, extras: []core.ExtraKind{"", tr, tr, tr}},
	{fact: "TestDoubleTags", paths: []string{
		"/MCFAMILY-PC/Private3$/Heterosexual/Breast In Class 2 Counterfeit Racks (2011)/Breast In Class 2 Counterfeit Racks (2011) Disc 1 cd1.avi",
		"/MCFAMILY-PC/Private3$/Heterosexual/Breast In Class 2 Counterfeit Racks (2011)/Breast In Class 2 Counterfeit Racks (2011) Disc 1 cd2.avi",
		"/MCFAMILY-PC/Private3$/Heterosexual/Breast In Class 2 Counterfeit Racks (2011)/Breast In Class 2 Disc 2 cd1.avi",
		"/MCFAMILY-PC/Private3$/Heterosexual/Breast In Class 2 Counterfeit Racks (2011)/Breast In Class 2 Disc 2 cd2.avi",
	}, count: 2},
	{fact: "TestArgumentOutOfRangeException", paths: []string{"/nas-markrobbo78/Videos/INDEX HTPC/Movies/Watched/3 - ACTION/Argo (2012)/movie.mkv"}, count: 1},
	{fact: "TestColony", paths: []string{"The Colony.mkv"}, count: 1},
	// Not grouped as versions: the folder is not named after the movie.
	{fact: "TestFourSisters", paths: []string{"Four Sisters and a Wedding - A.avi", "Four Sisters and a Wedding - B.avi"}, count: 2},
	{fact: "TestFourRooms", paths: []string{"Four Rooms - A.avi", "Four Rooms - A.mp4"}, count: 2},
	{
		fact: "TestMovieTrailer", paths: []string{"/Server/Despicable Me/Despicable Me (2010).mkv", "/Server/Despicable Me/trailer.mkv"},
		count: 2, extras: []core.ExtraKind{"", tr},
	},
	{fact: "Resolve_TrailerInTrailersFolder_ReturnsCorrectExtraType", paths: []string{
		"/Server/Despicable Me/Despicable Me (2010).mkv",
		"/Server/Despicable Me/trailers/some title.mkv",
	}, count: 2, extras: []core.ExtraKind{"", tr}},
	{
		fact: "TestSubfolders", paths: []string{"/Movies/Despicable Me/Despicable Me.mkv", "/Movies/Despicable Me/trailers/trailer.mkv"},
		count: 2, extras: []core.ExtraKind{"", tr},
	},
}

func TestPortedVideoListResolver(t *testing.T) {
	facts := map[string]string{"TestDirectoryStack": "TestFileStackContains"}
	for _, f := range videoListFacts {
		facts[f.fact] = "TestVideoList"
	}
	portedCases(t, "video/video_list_resolver.json", ported{facts: facts})
}

func TestVideoList(t *testing.T) {
	for _, tt := range videoListFacts {
		t.Run(tt.fact, func(t *testing.T) {
			got := resolveVideos(tt.paths, tt.isDir, VideoListOptions{})
			if len(got) != tt.count {
				t.Fatalf("videos: got = %d, want = %d (%+v)", len(got), tt.count, names(got))
			}
			for i, want := range tt.extras {
				if got[i].Extra != want {
					t.Errorf("video %d (%s) extra: got = %q, want = %q", i, got[i].Name, got[i].Extra, want)
				}
			}
			for name, n := range tt.stacks {
				found := false
				for _, v := range got {
					if v.Name == name {
						found = true
						if len(v.Files) != n {
							t.Errorf("%s files: got = %d, want = %d", name, len(v.Files), n)
						}
						break
					}
				}
				if !found {
					t.Errorf("%s: got = missing", name)
				}
			}
		})
	}
}

func names(videos []Video) []string {
	out := make([]string, len(videos))
	for i, v := range videos {
		out[i] = v.Name
	}
	return out
}

func TestFileStackContains(t *testing.T) {
	if (FileStack{}).ContainsFile("XX", true) {
		t.Error("got = true, want = false")
	}
}

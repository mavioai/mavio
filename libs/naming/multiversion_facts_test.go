package naming

import (
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func movies(paths ...string) []Video { return resolveVideos(paths, false, VideoListOptions{}) }
func episodes(paths ...string) []Video {
	return resolveVideos(paths, false, VideoListOptions{TVShows: true})
}

func wantCount(t *testing.T, videos []Video, n int) {
	t.Helper()
	if len(videos) != n {
		t.Fatalf("videos: got = %d %q, want = %d", len(videos), names(videos), n)
	}
}

func wantAlternates(t *testing.T, v Video, n int) {
	t.Helper()
	if len(v.AlternateVersions) != n {
		t.Errorf("%s alternates: got = %d, want = %d", v.Name, len(v.AlternateVersions), n)
	}
}

func wantFiles(t *testing.T, v Video, n int) {
	t.Helper()
	if len(v.Files) != n {
		t.Errorf("%s files: got = %d, want = %d", v.Name, len(v.Files), n)
	}
}

func wantPath(t *testing.T, v Video, want string) {
	t.Helper()
	checkString(t, "path", v.Files[0].Path, want)
}

func wantPathContains(t *testing.T, v Video, sub string) {
	t.Helper()
	if !strings.Contains(v.Files[0].Path, sub) {
		t.Errorf("path: got = %q, want containing %q", v.Files[0].Path, sub)
	}
}

// episodeWith returns the first video whose first file contains sub.
func episodeWith(t *testing.T, videos []Video, sub string) Video {
	t.Helper()
	for _, v := range videos {
		if strings.Contains(v.Files[0].Path, sub) {
			return v
		}
	}
	t.Fatalf("no video with %q", sub)
	return Video{}
}

// wantNonExtras checks the number of videos that are and are not extras.
func wantNonExtras(t *testing.T, videos []Video, nonExtras, extras int) {
	t.Helper()
	n := 0
	for _, v := range videos {
		if v.Extra == "" {
			n++
		}
	}
	if n != nonExtras || len(videos)-n != extras {
		t.Errorf("videos: got = %d + %d extras, want = %d + %d", n, len(videos)-n, nonExtras, extras)
	}
}

func allAlternates(t *testing.T, videos []Video, n int) {
	t.Helper()
	for _, v := range videos {
		wantAlternates(t, v, n)
	}
}

const xmen = "/movies/X-Men Apocalypse (2016)/X-Men Apocalypse (2016)"

var multiVersionFacts = map[string]func(t *testing.T){
	"TestMultiEdition1": func(t *testing.T) {
		wantNonExtras(t, movies(
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past - 1080p.mkv",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past-trailer.mp4",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past - [hsbs].mkv",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past [hsbs].mkv"), 1, 1)
	},
	"TestMultiEdition2": func(t *testing.T) {
		got := movies(
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past - apple.mkv",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past-trailer.mp4",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past - banana.mkv",
			"/movies/X-Men Days of Future Past/X-Men Days of Future Past [banana].mp4")
		wantNonExtras(t, got, 1, 1)
		wantAlternates(t, got[0], 2)
	},
	"TestMultiEdition3": func(t *testing.T) {
		got := movies(
			"/movies/The Phantom of the Opera (1925)/The Phantom of the Opera (1925) - 1925 version.mkv",
			"/movies/The Phantom of the Opera (1925)/The Phantom of the Opera (1925) - 1929 version.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"TestLetterFolders": func(t *testing.T) {
		got := movies("/movies/M/Movie 1.mkv", "/movies/M/Movie 2.mkv", "/movies/M/Movie 3.mkv", "/movies/M/Movie 4.mkv",
			"/movies/M/Movie 5.mkv", "/movies/M/Movie 6.mkv", "/movies/M/Movie 7.mkv")
		wantCount(t, got, 7)
		wantAlternates(t, got[0], 0)
	},
	"TestMultiVersionLimit": func(t *testing.T) {
		got := movies("/movies/Movie/Movie.mkv", "/movies/Movie/Movie-2.mkv", "/movies/Movie/Movie-3.mkv", "/movies/Movie/Movie-4.mkv",
			"/movies/Movie/Movie-5.mkv", "/movies/Movie/Movie-6.mkv", "/movies/Movie/Movie-7.mkv", "/movies/Movie/Movie-8.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 7)
	},
	"TestMultiVersionLimit2": func(t *testing.T) {
		got := movies("/movies/Mo/Movie 1.mkv", "/movies/Mo/Movie 2.mkv", "/movies/Mo/Movie 3.mkv", "/movies/Mo/Movie 4.mkv",
			"/movies/Mo/Movie 5.mkv", "/movies/Mo/Movie 6.mkv", "/movies/Mo/Movie 7.mkv", "/movies/Mo/Movie 8.mkv", "/movies/Mo/Movie 9.mkv")
		wantCount(t, got, 9)
		wantAlternates(t, got[0], 0)
	},
	"TestMultiVersion3": func(t *testing.T) {
		got := movies("/movies/Movie/Movie 1.mkv", "/movies/Movie/Movie 2.mkv", "/movies/Movie/Movie 3.mkv",
			"/movies/Movie/Movie 4.mkv", "/movies/Movie/Movie 5.mkv")
		wantCount(t, got, 5)
		wantAlternates(t, got[0], 0)
	},
	// A false positive.
	"TestMultiVersion4": func(t *testing.T) {
		got := movies("/movies/Iron Man/Iron Man.mkv", "/movies/Iron Man/Iron Man (2008).mkv", "/movies/Iron Man/Iron Man (2009).mkv",
			"/movies/Iron Man/Iron Man (2010).mkv", "/movies/Iron Man/Iron Man (2011).mkv")
		wantCount(t, got, 5)
		wantAlternates(t, got[0], 0)
	},
	"TestMultiVersion5": func(t *testing.T) {
		checkIronMan(t, "-720p", "-test", "-bluray", "-3d", "-3d-hsbs", "[test]")
	},
	"TestMultiVersion6": func(t *testing.T) {
		checkIronMan(t, " - 720p", " - test", " - bluray", " - 3d", " - 3d-hsbs", " [test]")
	},
	"TestMultiVersion7": func(t *testing.T) {
		wantCount(t, movies("/movies/Iron Man/Iron Man - B (2006).mkv", "/movies/Iron Man/Iron Man - C (2007).mkv"), 2)
	},
	"TestMultiVersion8": func(t *testing.T) {
		got := movies("/movies/Iron Man/Iron Man.mkv", "/movies/Iron Man/Iron Man_720p.mkv", "/movies/Iron Man/Iron Man_test.mkv",
			"/movies/Iron Man/Iron Man_bluray.mkv", "/movies/Iron Man/Iron Man_3d.mkv", "/movies/Iron Man/Iron Man_3d-hsbs.mkv",
			"/movies/Iron Man/Iron Man_3d.hsbs.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 6)
		// 3D recognition is kept on alternate versions.
		hsbs := episodeWith(t, got[0].AlternateVersions, "3d-hsbs")
		if !hsbs.Files[0].Is3D || hsbs.Files[0].Format3D != "hsbs" {
			t.Errorf("3D: got = %+v", hsbs.Files[0])
		}
	},
	// A false positive.
	"TestMultiVersion9": func(t *testing.T) {
		got := movies("/movies/Iron Man/Iron Man (2007).mkv", "/movies/Iron Man/Iron Man (2008).mkv", "/movies/Iron Man/Iron Man (2009).mkv",
			"/movies/Iron Man/Iron Man (2010).mkv", "/movies/Iron Man/Iron Man (2011).mkv")
		wantCount(t, got, 5)
		wantAlternates(t, got[0], 0)
	},
	"TestMultiVersion10": func(t *testing.T) {
		got := movies("/movies/Blade Runner (1982)/Blade Runner (1982) [Final Cut] [1080p HEVC AAC].mkv",
			"/movies/Blade Runner (1982)/Blade Runner (1982) [EE by ADM] [480p HEVC AAC,AAC,AAC].mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"TestMultiVersion11": func(t *testing.T) {
		got := movies(xmen+" [1080p] Blu-ray.x264.DTS.mkv", xmen+" [2160p] Blu-ray.x265.AAC.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"TestMultiVersion12": func(t *testing.T) {
		got := movies(xmen+" - Theatrical Release.mkv", xmen+" - Directors Cut.mkv", xmen+" - 1080p.mkv", xmen+" - 2160p.mkv",
			xmen+" - 720p.mkv", xmen+".mkv")
		wantCount(t, got, 1)
		wantPath(t, got[0], xmen+".mkv")
		checkAlternateOrder(t, got[0], xmen, " - 2160p", " - 1080p", " - 720p", " - Directors Cut", " - Theatrical Release")
	},
	"TestMultiVersion13": func(t *testing.T) {
		got := movies(xmen+" - Theatrical Release.mkv", xmen+" - Directors Cut.mkv", xmen+" - 1080p.mkv", xmen+" - 2160p.mkv",
			xmen+" - 1080p Directors Cut.mkv", xmen+" - 2160p Remux.mkv", xmen+" - 1080p Theatrical Release.mkv", xmen+" - 720p.mkv",
			xmen+" - 1080p Remux.mkv", xmen+" - 720p Directors Cut.mkv", xmen+" - 1080p High Bitrate.mkv", xmen+".mkv")
		wantCount(t, got, 1)
		wantPath(t, got[0], xmen+".mkv")
		checkAlternateOrder(t, got[0], xmen, " - 2160p", " - 2160p Remux", " - 1080p", " - 1080p Directors Cut", " - 1080p High Bitrate",
			" - 1080p Remux", " - 1080p Theatrical Release", " - 720p", " - 720p Directors Cut", " - Directors Cut", " - Theatrical Release")
	},
	"Resolve_GivenFolderNameWithBracketsAndHyphens_GroupsBasedOnFolderName": func(t *testing.T) {
		const dir = "/movies/John Wick - Kapitel 3 (2019) [imdbid=tt6146586]/John Wick - Kapitel 3 (2019) [imdbid=tt6146586]"
		got := movies(dir+" - Version 1.mkv", dir+" - Version 2.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"Resolve_GivenUnclosedBrackets_DoesNotGroup": func(t *testing.T) {
		wantCount(t, movies("/movies/John Wick - Chapter 3 (2019)/John Wick - Chapter 3 (2019) [Version 1].mkv",
			"/movies/John Wick - Chapter 3 (2019)/John Wick - Chapter 3 (2019) [Version 2.mkv"), 2)
	},
	"TestEmptyList": func(t *testing.T) {
		wantCount(t, parser.ResolveVideos(nil, VideoListOptions{}), 0)
	},
	"Resolve_GivenUnderscoreSeparator_GroupsVersions": func(t *testing.T) {
		got := movies("/movies/Movie (2020)/Movie (2020)_4K.mkv", "/movies/Movie (2020)/Movie (2020)_1080p.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"Resolve_GivenDotSeparator_GroupsVersions": func(t *testing.T) {
		got := movies("/movies/Movie (2020)/Movie (2020).UHD.mkv", "/movies/Movie (2020)/Movie (2020).1080p.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	// Two versions of an episode in its own folder merge; the higher
	// resolution is the primary.
	"TestMultiVersionEpisodeInOwnFolder": func(t *testing.T) {
		got := episodes("/TV/Dexter/Dexter - S01E01/Dexter - S01E01 - 1080p.mkv", "/TV/Dexter/Dexter - S01E01/Dexter - S01E01 - 720p.mkv")
		checkPrimaryAndAlternate(t, got, "1080p", "720p")
	},
	"TestMultiVersionEpisodeMixedSeasonFolder": func(t *testing.T) {
		got := episodes("/TV/Dexter/Season 1/Dexter - S01E01 - 1080p.mkv", "/TV/Dexter/Season 1/Dexter - S01E01 - 720p.mkv",
			"/TV/Dexter/Season 1/Dexter - S01E02.mkv", "/TV/Dexter/Season 1/Dexter - S01E03 - 1080p.mkv",
			"/TV/Dexter/Season 1/Dexter - S01E03 - 720p.mkv")
		wantCount(t, got, 3)
		e01 := episodeWith(t, got, "S01E01")
		wantAlternates(t, e01, 1)
		wantPathContains(t, e01, "1080p")
		wantAlternates(t, episodeWith(t, got, "S01E02"), 0)
		wantAlternates(t, episodeWith(t, got, "S01E03"), 1)
	},
	// Different episodes do not collapse into versions.
	"TestMultiVersionEpisodeDontCollapse": func(t *testing.T) {
		got := episodes("/TV/Dexter/Season 1/Dexter - S01E01.mkv", "/TV/Dexter/Season 1/Dexter - S01E02.mkv",
			"/TV/Dexter/Season 1/Dexter - S01E03.mkv", "/TV/Dexter/Season 1/Dexter - S01E04.mkv", "/TV/Dexter/Season 1/Dexter - S01E05.mkv")
		wantCount(t, got, 5)
		allAlternates(t, got, 0)
	},
	"TestMultiVersionEpisodeWithVersionSuffix": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - Aired.mkv", "/TV/Show/Season 1/Show - S01E01 - Uncensored.mkv",
			"/TV/Show/Season 1/Show - S01E02 - Aired.mkv", "/TV/Show/Season 1/Show - S01E02 - Uncensored.mkv")
		wantCount(t, got, 2)
		allAlternates(t, got, 1)
	},
	"TestMultiVersionEpisodeFourVersions": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - VersionA.mkv", "/TV/Show/Season 1/Show - S01E01 - VersionB.mkv",
			"/TV/Show/Season 1/Show - S01E01 - VersionC.mkv", "/TV/Show/Season 1/Show - S01E01 - VersionD.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 3)
	},
	"TestMultiVersionEpisodeWithResolutions": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - 720p.mkv", "/TV/Show/Season 1/Show - S01E01 - 2160p.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 1080p.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 2)
		wantPathContains(t, got[0], "2160p")
		wantPathContains(t, got[0].AlternateVersions[0], "1080p")
		wantPathContains(t, got[0].AlternateVersions[1], "720p")
	},
	// The same episode number in different seasons does not group.
	"TestMultiVersionEpisodeDifferentSeasons": func(t *testing.T) {
		got := episodes("/TV/Show/Show - S01E01.mkv", "/TV/Show/Show - S02E01.mkv")
		wantCount(t, got, 2)
		allAlternates(t, got, 0)
	},
	// Outside TV libraries episodes take the movie path and do not group.
	"TestMultiVersionEpisodeDisabledByDefault": func(t *testing.T) {
		wantCount(t, movies("/TV/Show/Season 1/Show - S01E01 - 1080p.mkv", "/TV/Show/Season 1/Show - S01E01 - 720p.mkv"), 2)
	},
	// Grouping keys only on season and episode, so differently titled
	// files of the same number collapse.
	"TestMultiVersionEpisodeSameNumberDifferentTitle": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - Pilot.mkv", "/TV/Show/Season 1/Show - S01E01 - Completely Different Title.mkv")
		wantCount(t, got, 1)
		wantAlternates(t, got[0], 1)
	},
	"TestMultiVersionEpisodeWithTitle": func(t *testing.T) {
		got := episodes("/TV/Show/Show - S01E01/Show - S01E01 - Episode Title - 1080p.mkv",
			"/TV/Show/Show - S01E01/Show - S01E01 - Episode Title - 720p.mkv")
		checkPrimaryAndAlternate(t, got, "1080p", "720p")
	},
	"TestMultiVersionEpisodeWithTitleMixedFolder": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - Pilot - 1080p.mkv", "/TV/Show/Season 1/Show - S01E01 - Pilot - 720p.mkv",
			"/TV/Show/Season 1/Show - S01E02 - Second Episode - 1080p.mkv", "/TV/Show/Season 1/Show - S01E02 - Second Episode - 720p.mkv",
			"/TV/Show/Season 1/Show - S01E03 - Third Episode.mkv")
		wantCount(t, got, 3)
		wantAlternates(t, episodeWith(t, got, "S01E01"), 1)
		wantAlternates(t, episodeWith(t, got, "S01E02"), 1)
		wantAlternates(t, episodeWith(t, got, "S01E03"), 0)
	},
	"TestMultiVersionEpisodeInSeasonSubfolder": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01/Show - S01E01 - 1080p.mkv", "/TV/Show/Season 1/Show - S01E01/Show - S01E01 - 720p.mkv")
		checkPrimaryAndAlternate(t, got, "1080p", "720p")
	},
	"TestMultiVersionEpisodeWithTitleAndVersionSuffix": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - Pilot - Aired.mkv", "/TV/Show/Season 1/Show - S01E01 - Pilot - Uncensored.mkv",
			"/TV/Show/Season 1/Show - S01E02 - The Getaway - Aired.mkv", "/TV/Show/Season 1/Show - S01E02 - The Getaway - Uncensored.mkv")
		wantCount(t, got, 2)
		allAlternates(t, got, 1)
	},
	"TestMultiVersionEpisodeWithAdditionalPartsCd": func(t *testing.T) {
		checkStackedPrimary(t, episodes("/TV/Show/Season 1/Show - S01E01 - 1080p cd1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p cd2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p.mkv"))
	},
	"TestMultiVersionEpisodeWithAdditionalPartsDashPart": func(t *testing.T) {
		checkStackedPrimary(t, episodes("/TV/Show/Season 1/Show - S01E01 - 1080p - part1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p - part2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p.mkv"))
	},
	"TestMultiVersionEpisodeWithAdditionalPartsPt": func(t *testing.T) {
		checkStackedPrimary(t, episodes("/TV/Show/Season 1/Show - S01E01 - 1080p.pt1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p.pt2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p.mkv"))
	},
	"TestMultiVersionEpisodeWithAdditionalPartsAndTitle": func(t *testing.T) {
		checkStackedPrimary(t, episodes("/TV/Show/Season 1/Show - S01E01 - Pilot - 1080p part1.mkv",
			"/TV/Show/Season 1/Show - S01E01 - Pilot - 1080p part2.mkv", "/TV/Show/Season 1/Show - S01E01 - Pilot - 720p.mkv"))
	},
	"TestMultiVersionEpisodeWithAdditionalPartsAndTitleDashSeparator": func(t *testing.T) {
		checkStackedPrimary(t, episodes("/TV/Show/Season 1/Show - S01E01 - Pilot - 1080p - part1.mkv",
			"/TV/Show/Season 1/Show - S01E01 - Pilot - 1080p - part2.mkv", "/TV/Show/Season 1/Show - S01E01 - Pilot - 720p.mkv"))
	},
	"TestMultiVersionEpisodeWithAdditionalPartsAndMultipleEpisodes": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - 1080p cd1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p cd2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p.mkv", "/TV/Show/Season 1/Show - S01E02 - Other.mkv")
		wantCount(t, got, 2)
		e01 := episodeWith(t, got, "S01E01")
		wantFiles(t, e01, 2)
		wantAlternates(t, e01, 1)
		wantAlternates(t, episodeWith(t, got, "S01E02"), 0)
	},
	// A three-part stack without resolution is preferred as the primary.
	"TestMultiVersionEpisodePartStackAlongsideSingleFileResolutions": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/S01E01 - 720p.mkv", "/TV/Show/Season 1/S01E01 - 1080p.mkv", "/TV/Show/Season 1/S01E01 - Part 1.mkv",
			"/TV/Show/Season 1/S01E01 - Part 2.mkv", "/TV/Show/Season 1/S01E01 - Part 3.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 3)
		for _, f := range got[0].Files {
			if !strings.Contains(f.Path, "Part") {
				t.Errorf("primary file: got = %q, want a part", f.Path)
			}
		}
		wantAlternates(t, got[0], 2)
		episodeWith(t, got[0].AlternateVersions, "1080p")
		episodeWith(t, got[0].AlternateVersions, "720p")
	},
	// The 1080p stack is the primary; the 720p stack stays whole.
	"TestMultiVersionEpisodeTwoPartStacks": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - 1080p - part1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p - part2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p - part1.mkv", "/TV/Show/Season 1/Show - S01E01 - 720p - part2.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 2)
		wantPathContains(t, got[0], "1080p")
		wantAlternates(t, got[0], 1)
		alt := got[0].AlternateVersions[0]
		wantFiles(t, alt, 2)
		for _, f := range alt.Files {
			if !strings.Contains(f.Path, "720p") {
				t.Errorf("alternate file: got = %q, want 720p", f.Path)
			}
		}
	},
	// A trailer is not pulled into the versions.
	"TestMultiVersionEpisodePartStackWithTrailer": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - 1080p part1.mkv", "/TV/Show/Season 1/Show - S01E01 - 1080p part2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - 720p.mkv", "/TV/Show/Season 1/Show - S01E01-trailer.mp4")
		wantCount(t, got, 2)
		for _, v := range got {
			switch v.Extra {
			case "":
				wantFiles(t, v, 2)
				wantAlternates(t, v, 1)
				wantPathContains(t, v.AlternateVersions[0], "720p")
			case core.ExtraTrailer:
			default:
				t.Errorf("extra: got = %q, want = trailer", v.Extra)
			}
		}
	},
	"TestMovieStackingWithPartNaming": func(t *testing.T) {
		checkSingleStack(t, movies("/movies/Movie/Movie part1.mkv", "/movies/Movie/Movie part2.mkv"))
	},
	"TestMovieStackingWithDashPartNaming": func(t *testing.T) {
		checkSingleStack(t, movies("/movies/Movie/Movie - part1.mkv", "/movies/Movie/Movie - part2.mkv"))
	},
	"TestMovieStackingWithPtNaming": func(t *testing.T) {
		checkSingleStack(t, movies("/movies/Movie/Movie.pt1.mkv", "/movies/Movie/Movie.pt2.mkv"))
	},
	"TestMovieStackingWithHyphenNoSpaces": func(t *testing.T) {
		checkSingleStack(t, movies("/movies/Movie/Movie-part1.mkv", "/movies/Movie/Movie-part2.mkv"))
	},
	"TestMovieStackingWithHyphenNoSpacesAndVersion": func(t *testing.T) {
		got := movies("/movies/Movie/Movie-1080p-part1.mkv", "/movies/Movie/Movie-1080p-part2.mkv", "/movies/Movie/Movie-720p.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 2)
		wantAlternates(t, got[0], 1)
	},
	// The file named after the folder is the primary; a stacked alternate
	// keeps all its files.
	"TestMovieMultiVersionWithStackedAlternate": func(t *testing.T) {
		got := movies("/movies/Inception (2010)/Inception (2010).mkv", "/movies/Inception (2010)/Inception (2010) - 4k part1.mkv",
			"/movies/Inception (2010)/Inception (2010) - 4k part2.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 1)
		wantPath(t, got[0], "/movies/Inception (2010)/Inception (2010).mkv")
		wantAlternates(t, got[0], 1)
		alt := got[0].AlternateVersions[0]
		wantFiles(t, alt, 2)
		for _, f := range alt.Files {
			if !strings.Contains(f.Path, "4k part") {
				t.Errorf("alternate file: got = %q", f.Path)
			}
		}
	},
	"TestEpisodeStackingWithHyphenNoSpaces": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01-1080p-cd1.mkv", "/TV/Show/Season 1/Show - S01E01-1080p-cd2.mkv",
			"/TV/Show/Season 1/Show - S01E01-720p.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 2)
		wantAlternates(t, got[0], 1)
	},
	"TestEpisodeStackingWithHyphenNoSpacesAndTitle": func(t *testing.T) {
		got := episodes("/TV/Show/Season 1/Show - S01E01 - Pilot-1080p-part1.mkv", "/TV/Show/Season 1/Show - S01E01 - Pilot-1080p-part2.mkv",
			"/TV/Show/Season 1/Show - S01E01 - Pilot-720p.mkv")
		wantCount(t, got, 1)
		wantFiles(t, got[0], 2)
		wantAlternates(t, got[0], 1)
	},
	// All parse as episode 2, from the "2" of the series title, yet they
	// are distinct episodes.
	"TestMultiVersionEpisodeAbsoluteNumberingWithNumberInSeriesTitle": func(t *testing.T) {
		const dir = "/anime/IS Infinite Stratos 2/IS Infinite Stratos 2"
		got := episodes(dir+" - 01 - The Memory of a Summer (b6f40849).mkv", dir+" - 02 - Heart Pain Killer (d8c0896c).mkv",
			dir+" - 03 - Translucent Chord (4ecce3dd).mkv", dir+" - 04 - The Mysterious Lady (837a1909).mkv")
		wantCount(t, got, 4)
		allAlternates(t, got, 0)
	},
	// Without a season number absolute episodes stay separate.
	"TestMultiVersionEpisodeAbsoluteNumberingDontCollapse": func(t *testing.T) {
		got := episodes("/anime/Bleach/Bleach - 001 - The Day I Became a Shinigami.mkv", "/anime/Bleach/Bleach - 002 - The Shinigami's Work.mkv",
			"/anime/Bleach/Bleach - 003 - The Older Brother's Wish.mkv")
		wantCount(t, got, 3)
		allAlternates(t, got, 0)
	},
}

func checkIronMan(t *testing.T, suffixes ...string) {
	t.Helper()
	paths := []string{"/movies/Iron Man/Iron Man.mkv"}
	for _, s := range suffixes {
		paths = append(paths, "/movies/Iron Man/Iron Man"+s+".mkv")
	}
	got := movies(paths...)
	wantCount(t, got, 1)
	wantPath(t, got[0], "/movies/Iron Man/Iron Man.mkv")
	// Resolutions first, then by name.
	s := suffixes
	checkAlternateOrder(t, got[0], "/movies/Iron Man/Iron Man", s[0], s[3], s[4], s[2], s[1], s[5])
}

func checkAlternateOrder(t *testing.T, v Video, prefix string, suffixes ...string) {
	t.Helper()
	if len(v.AlternateVersions) != len(suffixes) {
		t.Fatalf("alternates: got = %d, want = %d", len(v.AlternateVersions), len(suffixes))
	}
	for i, s := range suffixes {
		wantPath(t, v.AlternateVersions[i], prefix+s+".mkv")
	}
}

func checkPrimaryAndAlternate(t *testing.T, got []Video, primary, alternate string) {
	t.Helper()
	wantCount(t, got, 1)
	wantAlternates(t, got[0], 1)
	wantPathContains(t, got[0], primary)
	wantPathContains(t, got[0].AlternateVersions[0], alternate)
}

// checkStackedPrimary checks a stacked 1080p primary with a 720p alternate.
func checkStackedPrimary(t *testing.T, got []Video) {
	t.Helper()
	wantCount(t, got, 1)
	wantFiles(t, got[0], 2)
	wantAlternates(t, got[0], 1)
	wantPathContains(t, got[0].AlternateVersions[0], "720p")
}

func checkSingleStack(t *testing.T, got []Video) {
	t.Helper()
	wantCount(t, got, 1)
	wantFiles(t, got[0], 2)
}

func TestPortedMultiVersion(t *testing.T) {
	facts := map[string]string{}
	for name := range multiVersionFacts {
		facts[name] = "TestMultiVersion"
	}
	portedCases(t, "video/multi_version.json", ported{facts: facts})
}

func TestMultiVersion(t *testing.T) {
	for name, check := range multiVersionFacts {
		t.Run(name, check)
	}
}

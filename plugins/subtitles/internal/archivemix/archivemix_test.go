package archivemix

import (
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	for _, c := range []struct {
		name     string
		season   int
		episodes []int
	}{
		{"Game.of.Thrones.S03E10.Mhysa.720p.srt", 3, []int{10}},
		{"Show.S01E02E03.srt", 1, []int{2, 3}},
		{"Show.S01E02-E05.srt", 1, []int{2, 3, 4, 5}},
		{"Show.S01E02-05.srt", 1, []int{2, 3, 4, 5}},
		{"La Femme Nikita - 01x13 - Recruit.srt", 1, []int{13}},
		{"Show 2x01-2x03.srt", 2, []int{1, 2, 3}},
		{"Show.Season.2.Complete", 2, nil},
		{"Show.S02.1080p", 2, nil},
		{"One Piece EP0264-0270", 0, []int{264, 265, 266, 267, 268, 269, 270}},
		{"Show - Episode 5.srt", 0, []int{5}},
		{"Movie.2021.1080p.WEBRip.x264.srt", 0, nil},
	} {
		g := Parse(c.name)
		if g.Season != c.season || !slices.Equal(g.Episodes, c.episodes) {
			t.Errorf("Parse(%q) = %d %v, want = %d %v", c.name, g.Season, g.Episodes, c.season, c.episodes)
		}
	}
	if _, ok := Single("Show.S01E02E03.srt"); ok {
		t.Error("Single of two episodes = true, want = false")
	}
	if e, ok := Single("Show.S01E07.srt"); !ok || e != 7 {
		t.Errorf("Single = %d %v, want = 7 true", e, ok)
	}
}

func TestPick(t *testing.T) {
	pack := []string{"readme.txt", "Show.S01E01.srt", "Show.S01E02.forced.srt", "Show.S01E02.srt", "Show.S02E02.srt"}
	for _, c := range []struct {
		names []string
		want  Want
		got   string
	}{
		{[]string{"readme.txt", "only.srt"}, Want{Season: 9, Episode: 9, AskedForEpisode: 9}, "only.srt"},
		{pack, Want{Season: 1, Episode: 2, AskedForEpisode: 2}, "Show.S01E02.srt"},
		{pack, Want{Season: 2, Episode: 2, AskedForEpisode: 2}, "Show.S02E02.srt"},
		{pack, Want{Season: 1, AskedForEpisode: 2, Pack: true}, "Show.S01E02.srt"},
		{pack, Want{Season: 1, Episode: 5, AskedForEpisode: 5}, ""},
		{[]string{"Movie.forced.srt", "Movie.srt"}, Want{}, "Movie.srt"},
		{[]string{"Movie.forced.srt", "Movie.srt"}, Want{Forced: true}, "Movie.forced.srt"},
		{[]string{"Movie-A.srt", "Movie-GRP.srt"}, Want{Pack: true, ReleaseGroup: "grp"}, "Movie-GRP.srt"},
	} {
		if got := Pick(c.names, c.want); got != c.got {
			t.Errorf("Pick(%v, %+v) = %q, want = %q", c.names, c.want, got, c.got)
		}
	}
}

package scrape

import (
	"math"
	"slices"
	"testing"

	"github.com/mavioai/mavio/plugins/subtitles/internal/site"
)

func TestSanitize(t *testing.T) {
	for _, c := range []struct{ in, ignore, want, patch string }{
		{"Marvel's Agents of S.H.I.E.L.D.", "", "marvels agents of s h i e l d", "marvels agents of s h i e l d"},
		{"Grey’s Anatomy: (2005)", "", "grey’s anatomy 2005", "greys anatomy 2005"},
		{"Grey's Anatomy", "'", "grey's anatomy", "grey's anatomy"},
	} {
		if got := Sanitize(c.in, c.ignore); got != c.want {
			t.Errorf("Sanitize(%q) got = %q, want = %q", c.in, got, c.want)
		}
		if got := SanitizePatch(c.in, c.ignore); got != c.patch {
			t.Errorf("SanitizePatch(%q) got = %q, want = %q", c.in, got, c.patch)
		}
	}
}

func TestFixNaming(t *testing.T) {
	renames := []string{"Back to the Future Part III", "Back to the Future 3", "Back to the Future Part II", "Back to the Future 2"}
	for in, want := range map[string]string{
		"Back to the Future Part III": "Back to the Future 3",
		"Back to the Future Part II":  "Back to the Future 2",
		"Dune":                        "Dune",
	} {
		if got := FixNaming(in, renames...); got != want {
			t.Errorf("FixNaming(%q) got = %q, want = %q", in, got, want)
		}
	}
}

func TestRatio(t *testing.T) {
	if got := Ratio("dune", "dune"); got != 1 {
		t.Errorf("Ratio got = %v, want = 1", got)
	}
	// difflib.SequenceMatcher(None, "cure", "cure (kyua)").ratio()
	if got := Ratio("cure", "cure (kyua)"); math.Abs(got-8.0/15) > 1e-9 {
		t.Errorf("Ratio got = %v, want = %v", got, 8.0/15)
	}
}

func TestSeasonsEpisodes(t *testing.T) {
	for _, c := range []struct {
		in                string
		seasons, episodes []int
	}{
		{"Breaking.Bad.S01E01-7.DSR-HDTV.eng", []int{1}, []int{1, 2, 3, 4, 5, 6, 7}},
		{"Breaking.Bad.S01E03.720p.BluRay.x264", []int{1}, []int{3}},
		{"Show.S01E01E02.720p", []int{1}, []int{1, 2}},
		{"Show 2x05 HDTV", []int{2}, []int{5}},
		{"Rick.and.Morty.S06.1080p.BluRay.x264-STORiES", []int{6}, nil},
		{"Season 3 complete.", []int{3}, nil},
		{"Dune.2021.1080p.WEBRip.x264-SHITBOX", nil, nil},
		{"Show.S01E01.720p-x264", []int{1}, []int{1}},
	} {
		s, e := SeasonsEpisodes(c.in)
		if !slices.Equal(s, c.seasons) || !slices.Equal(e, c.episodes) {
			t.Errorf("SeasonsEpisodes(%q) got = %v, %v, want = %v, %v", c.in, s, e, c.seasons, c.episodes)
		}
	}
}

func TestTitleYear(t *testing.T) {
	for _, c := range []struct {
		in, title string
		year      int
	}{
		{"Dune.2021.1080p.WEBRip.x264-SHITBOX", "Dune", 2021},
		{"The.Matrix.1999.720p.BluRay.mkv", "The Matrix", 1999},
		{"Game.of.Thrones.S01E01.720p.srt", "Game of Thrones", 0},
		{"2001 A Space Odyssey 1968 DVDRip", "2001 A Space Odyssey", 1968},
		{"Inception", "Inception", 0},
	} {
		if got := Title(c.in); got != c.title {
			t.Errorf("Title(%q) got = %q, want = %q", c.in, got, c.title)
		}
		if got := Year(c.in); got != c.year {
			t.Errorf("Year(%q) got = %d, want = %d", c.in, got, c.year)
		}
	}
}

func TestText(t *testing.T) {
	n, err := site.Parse([]byte(`<div id="d"><b>(2010)</b> x <i>Inception</i>
	  (2010)</div><a id="a">A</a>tail`), "text/html")
	if err != nil {
		t.Fatal(err)
	}
	d := n.First("#d")
	if got := Words(d); got != "(2010) x Inception (2010)" {
		t.Errorf("Words got = %q", got)
	}
	if got := Contents(d); len(got) != 4 || got[2] != "Inception" {
		t.Errorf("Contents got = %q, want = 4 nodes, the third Inception", got)
	}
	if got := NextText(n.First("#a")); got != "tail" {
		t.Errorf("NextText got = %q, want = tail", got)
	}
	if got := ASCII("Shōgun"); got != "Shogun" {
		t.Errorf("ASCII got = %q, want = Shogun", got)
	}
}

package library

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

func TestLocalImages(t *testing.T) {
	art := &fstest.MapFile{Data: []byte("img")}
	fsys := fstest.MapFS{
		// A movie in its own folder.
		"Up (2009)/Up (2009).mkv":     {},
		"Up (2009)/poster.jpg":        art,
		"Up (2009)/poster.png":        art, // preferred extension
		"Up (2009)/fanart.jpg":        art,
		"Up (2009)/fanart-1.jpg":      art,
		"Up (2009)/fanart-2.jpg":      art,
		"Up (2009)/clearlogo.png":     art,
		"Up (2009)/landscape.jpg":     art,
		"Up (2009)/disc.png":          art,
		"Up (2009)/banner.jpg":        {}, // empty: skipped
		"Up (2009)/extrafanart/x.jpg": art,
		"Up (2009)/backdrop1.jpg":     art,
		// The movie's own name wins over poster.
		"Heat (1995)/Heat (1995).mkv": {},
		"Heat (1995)/Heat (1995).jpg": art,
		"Heat (1995)/poster.jpg":      art,
		// Movies sharing a folder only take images named after them.
		"Mixed/A (2020).mkv":        {},
		"Mixed/B (2021).mkv":        {},
		"Mixed/poster.jpg":          art,
		"Mixed/A (2020)-poster.jpg": art,
		// A series, its seasons and an episode.
		"Show/show.jpg":                          art,
		"Show/season01-poster.jpg":               art,
		"Show/season-specials-poster.jpg":        art,
		"Show/Season 1/Show S01E01.mkv":          {},
		"Show/Season 1/Show S01E01-thumb.jpg":    art,
		"Show/Season 1/metadata/Show S01E02.jpg": art,
		"Show/Season 1/Show S01E02.mkv":          {},
		"Show/Specials/Show S00E01.mkv":          {},
		"Music/Album/01 Song.flac":               {},
		"Music/Album/01 Song.jpg":                art,
	}
	type img = metadata.LocalImage
	tests := []struct {
		name          string
		item          core.Item
		rel           string
		folder, mixed bool
		want          []img
	}{
		{"movie in its folder", core.Item{Kind: core.KindMovie}, "Up (2009)/Up (2009).mkv", false, false, []img{
			{Kind: core.ImagePrimary, Path: "Up (2009)/poster.png"},
			{Kind: core.ImageLogo, Path: "Up (2009)/clearlogo.png"},
			{Kind: core.ImageDisc, Path: "Up (2009)/disc.png"},
			{Kind: core.ImageThumb, Path: "Up (2009)/landscape.jpg"},
			{Kind: core.ImageBackdrop, Path: "Up (2009)/fanart.jpg"},
			{Kind: core.ImageBackdrop, Path: "Up (2009)/fanart-1.jpg"},
			{Kind: core.ImageBackdrop, Path: "Up (2009)/fanart-2.jpg"},
			{Kind: core.ImageBackdrop, Path: "Up (2009)/extrafanart/x.jpg"},
			{Kind: core.ImageBackdrop, Path: "Up (2009)/backdrop1.jpg"},
		}},
		{"own name first", core.Item{Kind: core.KindMovie}, "Heat (1995)/Heat (1995).mkv", false, false, []img{
			{Kind: core.ImagePrimary, Path: "Heat (1995)/Heat (1995).jpg"},
		}},
		{"mixed folder, named", core.Item{Kind: core.KindMovie}, "Mixed/A (2020).mkv", false, true, []img{
			{Kind: core.ImagePrimary, Path: "Mixed/A (2020)-poster.jpg"},
		}},
		{"mixed folder, unnamed", core.Item{Kind: core.KindMovie}, "Mixed/B (2021).mkv", false, true, nil},
		{"series", core.Item{Kind: core.KindSeries}, "Show", true, false, []img{
			{Kind: core.ImagePrimary, Path: "Show/show.jpg"},
		}},
		{"season from the series folder", core.Item{Kind: core.KindSeason, Name: "Season 1", IndexNumber: new(1)}, "Show/Season 1", true, false, []img{
			{Kind: core.ImagePrimary, Path: "Show/season01-poster.jpg"},
		}},
		{"specials", core.Item{Kind: core.KindSeason, Name: "Specials", IndexNumber: new(0)}, "Show/Specials", true, false, []img{
			{Kind: core.ImagePrimary, Path: "Show/season-specials-poster.jpg"},
		}},
		{"episode thumb", core.Item{Kind: core.KindEpisode}, "Show/Season 1/Show S01E01.mkv", false, false, []img{
			{Kind: core.ImagePrimary, Path: "Show/Season 1/Show S01E01-thumb.jpg"},
		}},
		{"episode in metadata", core.Item{Kind: core.KindEpisode}, "Show/Season 1/Show S01E02.mkv", false, false, []img{
			{Kind: core.ImagePrimary, Path: "Show/Season 1/metadata/Show S01E02.jpg"},
		}},
		{"track", core.Item{Kind: core.KindTrack}, "Music/Album/01 Song.flac", false, false, nil},
		{"photo", core.Item{Kind: core.KindPhoto}, "Trip/beach.png", false, false, []img{
			{Kind: core.ImagePrimary, Path: "Trip/beach.png"},
		}},
	}
	for _, tt := range tests {
		got := localImages(fsys, &tt.item, tt.rel, tt.folder, tt.mixed)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s:\ngot  = %v\nwant = %v", tt.name, got, tt.want)
		}
	}
}

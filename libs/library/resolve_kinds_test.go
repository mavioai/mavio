package library

import (
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestResolveMusic(t *testing.T) {
	fs := newFS(
		"/music/Artist/Album/01 Song.flac", "/music/Artist/Album/02 Song.flac", "/music/Artist/Album/cover.jpg",
		"/music/Artist/Double/CD1/01 A.mp3", "/music/Artist/Double/CD2/01 B.mp3",
		"/music/Loose.mp3",
	)
	root := resolve(t, core.LibraryMusic, "/music", "/music", InFolder, "", fs)
	if len(root.Items) != 1 || root.Items[0].Kind != core.KindTrack || len(root.Subfolders) != 1 {
		t.Fatalf("root: got = %+v", root)
	}
	artist := resolve(t, core.LibraryMusic, "/music", "/music/Artist", InFolder, "/music", fs)
	if artist.Item == nil || artist.Item.Kind != core.KindMusicArtist || len(artist.Subfolders) != 2 || artist.Subfolders[0].Scope.Parent != InMusicArtist {
		t.Fatalf("artist: got = %+v", artist)
	}
	album := resolve(t, core.LibraryMusic, "/music", "/music/Artist/Album", InMusicArtist, "/music/Artist", fs)
	if album.Item == nil || album.Item.Kind != core.KindMusicAlbum || len(album.Items) != 2 {
		t.Fatalf("album: got = %+v", album)
	}
	double := resolve(t, core.LibraryMusic, "/music", "/music/Artist/Double", InMusicArtist, "/music/Artist", fs)
	if double.Item == nil || len(double.Items) != 2 || *double.Items[1].ParentIndex != 2 || len(double.Subfolders) != 0 {
		t.Fatalf("multi-disc album: got = %+v", double)
	}
}

func TestResolveBooks(t *testing.T) {
	fs := newFS("/books/Dune (1965).epub", "/books/Sherlock Holmes #2 (1890).pdf", "/books/Audio/Story/story.mp3", "/books/notes.txt")
	root := resolve(t, core.LibraryBooks, "/books", "/books", InFolder, "", fs)
	if len(root.Items) != 2 || len(root.Subfolders) != 1 {
		t.Fatalf("root: got = %+v", root)
	}
	for _, n := range root.Items {
		if n.Kind != core.KindBook {
			t.Errorf("%s: got = %s", n.Path, n.Kind)
		}
	}
	story := resolve(t, core.LibraryBooks, "/books", "/books/Audio/Story", InFolder, "/books/Audio", fs)
	if story.Item == nil || story.Item.Kind != core.KindAudioBook || story.Item.Name != "Story" {
		t.Errorf("audiobook: got = %+v", story)
	}
}

func TestResolvePhotos(t *testing.T) {
	fs := newFS("/photos/Trip/a.jpg", "/photos/Trip/b.heic", "/photos/Trip/folder.jpg", "/photos/Trip/clip.mp4", "/photos/Trip/clip.jpg")
	trip := resolve(t, core.LibraryPhotos, "/photos", "/photos/Trip", InFolder, "/photos", fs)
	if trip.Item == nil || trip.Item.Kind != core.KindPhotoAlbum {
		t.Fatalf("got = %+v", trip.Item)
	}
	var photos, videos int
	for _, n := range trip.Items {
		switch n.Kind {
		case core.KindPhoto:
			photos++
		case core.KindVideo:
			videos++
		}
	}
	// folder.jpg is artwork, clip.jpg the video's thumbnail.
	if photos != 2 || videos != 1 {
		t.Errorf("got = %d photos, %d videos", photos, videos)
	}
}

func TestResolveMixed(t *testing.T) {
	fs := newFS("/media/Show/Season 1/Show S01E01.mkv", "/media/Movie (2020)/Movie (2020).mkv")
	show := resolve(t, core.LibraryMixed, "/media", "/media/Show", InFolder, "/media", fs)
	if show.Item == nil || show.Item.Kind != core.KindSeries || show.Subfolders[0].Scope.Kind != core.LibraryShows {
		t.Errorf("show: got = %+v", show)
	}
	movie := resolve(t, core.LibraryMixed, "/media", "/media/Movie (2020)", InFolder, "/media", fs)
	if movie.Item == nil || movie.Item.Kind != core.KindMovie || movie.Item.Name != "Movie" || *movie.Item.Year != 2020 {
		t.Errorf("movie: got = %+v", movie.Item)
	}
}

func TestResolveDiscs(t *testing.T) {
	fs := newFS("/movies/Rip (2001)/VIDEO_TS/VTS_01_1.VOB", "/movies/Box/Box Disc 1/BDMV/", "/movies/Box/Box Disc 2/BDMV/")
	rip := resolve(t, core.LibraryMovies, "/movies", "/movies/Rip (2001)", InFolder, "", fs)
	if rip.Item == nil || rip.Item.Disc != "dvd" || rip.Item.Path != "/movies/Rip (2001)" {
		t.Errorf("dvd: got = %+v", rip.Item)
	}
	box := resolve(t, core.LibraryMovies, "/movies", "/movies/Box", InFolder, "", fs)
	// Folders stack by name, as "Box Disc 1" and "Box Disc 2" do.
	if box.Item == nil || box.Item.Disc != "bluray" || len(box.Item.Parts) != 1 || box.Item.Name != "Box" {
		t.Errorf("multi-disc: got = %+v", box.Item)
	}
}

func TestResolveDailyEpisode(t *testing.T) {
	p := "/tv/News/News 2024-03-05.mkv"
	res := resolve(t, core.LibraryShows, "/tv", "/tv/News", InFolder, "/tv", newFS(p))
	if len(res.Items) != 1 || res.Items[0].Aired == nil || res.Items[0].ParentIndex == nil || *res.Items[0].ParentIndex != 1 {
		t.Errorf("got = %+v", res.Items)
	}
}

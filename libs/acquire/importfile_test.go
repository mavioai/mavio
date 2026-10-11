package acquire

import (
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

func TestBlocklisted(t *testing.T) {
	published := testNow.Add(-time.Hour)
	usenet := core.BlocklistEntry{SourceTitle: "Movie.2024.1080p-GRP", Protocol: core.ProtocolUsenet, Indexer: "NZBgeek", PublishedDate: published, Size: 1 << 30}
	torrent := core.BlocklistEntry{SourceTitle: "Movie.2024.1080p-GRP", Protocol: core.ProtocolTorrent, Indexer: "Tracker", TorrentInfoHash: "ABCDEF"}
	tests := []struct {
		name    string
		entries []core.BlocklistEntry
		release Release
		want    bool
	}{
		{"same nzb", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolUsenet, Indexer: "NZBgeek", PublishDate: published}, true},
		{"nzb of another date", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolUsenet, Indexer: "NZBgeek", PublishDate: published.Add(time.Hour)}, false},
		{"nzb of another indexer, same size and date", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolUsenet, Indexer: "Other", PublishDate: published.Add(time.Minute), Size: 1<<30 + 1<<20}, true},
		{"nzb of another indexer, other size", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolUsenet, Indexer: "Other", PublishDate: published.Add(time.Minute), Size: 2 << 30}, false},
		{"nzb of another title", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.720p-GRP", Protocol: ProtocolUsenet, PublishDate: published}, false},
		{"torrent by hash", []core.BlocklistEntry{torrent}, Release{Title: "Renamed", Protocol: ProtocolTorrent, InfoHash: "abcdef"}, true},
		{"torrent of another hash", []core.BlocklistEntry{torrent}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolTorrent, InfoHash: "123456"}, false},
		{"torrent by title and indexer", []core.BlocklistEntry{torrent}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolTorrent, Indexer: "tracker"}, true},
		{"torrent by title, other indexer", []core.BlocklistEntry{torrent}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolTorrent, Indexer: "Other"}, false},
		{"usenet entry for a torrent", []core.BlocklistEntry{usenet}, Release{Title: "Movie.2024.1080p-GRP", Protocol: ProtocolTorrent, Indexer: "NZBgeek"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Blocklisted(tt.entries, &tt.release); got != tt.want {
				t.Errorf("Blocklisted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterImportPaths(t *testing.T) {
	base := "/downloads/Movie.2024"
	paths := []string{
		"/downloads/Movie.2024/movie.mkv",
		"/downloads/Movie.2024/Sample/movie-sample.mkv",
		"/downloads/Movie.2024/Extras/making of.mkv",
		"/downloads/Movie.2024/movie-trailer.mkv",
		"/downloads/Movie.2024/.hidden/movie.mkv",
		"/downloads/Movie.2024/._movie.mkv",
		`C:\dl\Movie.2024\Featurettes\x.mkv`,
	}
	got := FilterImportPaths(base, paths)
	if want := []string{"/downloads/Movie.2024/movie.mkv"}; !slices.Equal(got, want) {
		t.Errorf("FilterImportPaths() = %q, want %q", got, want)
	}
}

func TestIsSample(t *testing.T) {
	d := func(v time.Duration) *time.Duration { return &v }
	tests := []struct {
		name    string
		file    string
		runtime *time.Duration
		movie   int
		want    SampleResult
	}{
		{"feature", "movie.mkv", d(100 * time.Minute), 100, SampleNo},
		{"short file", "movie.mkv", d(5 * time.Minute), 100, SampleYes},
		{"zero runtime", "movie.mkv", d(0), 100, SampleYes},
		{"unknown runtime", "movie.mkv", nil, 100, SampleIndeterminate},
		{"disc image", "movie.iso", nil, 100, SampleNo},
		{"short movie", "movie.mkv", d(2 * time.Minute), 10, SampleNo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSample(tt.file, tt.runtime, tt.movie); got != tt.want {
				t.Errorf("IsSample() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImportUpgrade(t *testing.T) {
	p := testProfile(release.Bluray1080p, release.HDTV720p, release.WEBDL1080p, release.Bluray1080p)
	file := func(q release.Quality, version int) *File {
		m := NewQualityModel(q)
		m.Revision.Version = version
		return &File{Quality: m}
	}
	model := func(q release.Quality, version int) QualityModel {
		m := NewQualityModel(q)
		m.Revision.Version = version
		return m
	}
	tests := []struct {
		name     string
		existing *File
		quality  QualityModel
		want     ImportRejection
	}{
		{"no file", nil, model(release.HDTV720p, 1), ""},
		{"better quality", file(release.HDTV720p, 1), model(release.WEBDL1080p, 1), ""},
		{"worse quality", file(release.WEBDL1080p, 1), model(release.HDTV720p, 1), ImportNotQualityUpgrade},
		{"same quality", file(release.WEBDL1080p, 1), model(release.WEBDL1080p, 1), ""},
		{"revision downgrade", file(release.WEBDL1080p, 2), model(release.WEBDL1080p, 1), ImportNotRevisionUpgrade},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := (Upgrades{}).ImportUpgrade(p, tt.existing, tt.quality, nil); got != tt.want {
				t.Errorf("ImportUpgrade() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecent(t *testing.T) {
	tests := []struct {
		name string
		a    Availability
		want bool
	}{
		{"in cinemas lately", Availability{InCinemas: testNow.AddDate(0, 0, -30)}, true},
		{"in cinemas long ago", Availability{InCinemas: testNow.AddDate(0, 0, -200)}, false},
		{"digital lately", Availability{InCinemas: testNow.AddDate(0, 0, -200), Digital: testNow.AddDate(0, 0, -7)}, true},
		{"nothing known", Availability{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Recent(testNow); got != tt.want {
				t.Errorf("Recent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemapRemoteToLocal(t *testing.T) {
	mappings := []RemotePathMapping{
		{Host: "nas", RemotePath: "/volume1/downloads/", LocalPath: "/mnt/downloads/"},
		{Host: "win", RemotePath: `D:\Downloads`, LocalPath: "/mnt/win"},
	}
	tests := []struct {
		name, host, remote, want string
	}{
		{"mapped", "NAS", "/volume1/downloads/movies/Movie.2024", "/mnt/downloads/movies/Movie.2024"},
		{"the folder itself", "nas", "/volume1/downloads", "/mnt/downloads/"},
		{"other host", "other", "/volume1/downloads/x", "/volume1/downloads/x"},
		{"not below", "nas", "/volume1/downloads2/x", "/volume1/downloads2/x"},
		{"windows path", "win", `d:\downloads\Movie.2024\movie.mkv`, "/mnt/win/Movie.2024/movie.mkv"},
		{"empty", "nas", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemapRemoteToLocal(mappings, tt.host, tt.remote); got != tt.want {
				t.Errorf("RemapRemoteToLocal() = %q, want %q", got, tt.want)
			}
		})
	}
}

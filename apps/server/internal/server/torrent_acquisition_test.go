package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/release"
)

// qbTorrent is a torrent of the fake qBittorrent.
type qbTorrent struct {
	hash, name, category, magnet string
	// ratioLimit and seedingTimeLimit are the seeding limits the torrent
	// was added with, as the form gave them; empty for qBittorrent's.
	ratioLimit, seedingTimeLimit string
	// state is qBittorrent's: "downloading", then "uploading" while it
	// seeds, "pausedUP" once paused at its seeding limits.
	state   string
	ratio   float64
	deleted bool
	// deletedFiles is set when the torrent was deleted with its files.
	deletedFiles bool
}

// fakeQBittorrent answers qBittorrent 5's Web API v2 without
// authentication, for the calls the download clients plugin makes: the
// torrents added download into a folder of their category below the save
// path, which is a path of qBittorrent's machine, until the test completes
// them; local is the folder the server sees that save path as.
type fakeQBittorrent struct {
	*httptest.Server
	savePath, local string

	mu       sync.Mutex
	torrents []*qbTorrent
}

func newFakeQBittorrent(t *testing.T, savePath, local string) *fakeQBittorrent {
	f := &fakeQBittorrent{savePath: savePath, local: local}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeQBittorrent) serve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	answer := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/api/v2/app/webapiVersion":
		_, _ = w.Write([]byte("2.11.2"))
	case "/api/v2/app/preferences":
		answer(map[string]any{
			"save_path": f.savePath, "dht": true, "queueing_enabled": false,
			"max_ratio_enabled": false, "max_ratio": -1, "max_seeding_time_enabled": false, "max_seeding_time": -1, "max_ratio_act": 0,
		})
	case "/api/v2/torrents/categories":
		answer(map[string]any{
			"movies": map[string]any{"name": "movies", "savePath": ""},
			"tv":     map[string]any{"name": "tv", "savePath": ""},
		})
	case "/api/v2/torrents/createCategory", "/api/v2/torrents/setCategory", "/api/v2/torrents/setShareLimits", "/api/v2/torrents/topPrio":
	case "/api/v2/torrents/add":
		link := r.Form.Get("urls")
		u, err := url.Parse(link)
		if err != nil || u.Scheme != "magnet" {
			_, _ = w.Write([]byte("Fails."))
			return
		}
		f.torrents = append(f.torrents, &qbTorrent{
			hash: strings.TrimPrefix(u.Query().Get("xt"), "urn:btih:"), name: u.Query().Get("dn"), category: r.Form.Get("category"), magnet: link,
			ratioLimit: r.Form.Get("ratioLimit"), seedingTimeLimit: r.Form.Get("seedingTimeLimit"), state: "downloading",
		})
		_, _ = w.Write([]byte("Ok."))
	case "/api/v2/torrents/info":
		var list []any
		for _, t := range f.torrents {
			if t.deleted || (r.Form.Get("category") != "" && r.Form.Get("category") != t.category) {
				continue
			}
			progress, eta := 0.5, 600
			if t.state != "downloading" {
				progress, eta = 1, 8640000
			}
			list = append(list, map[string]any{
				"hash": t.hash, "name": t.name, "size": 6 << 30, "progress": progress, "eta": eta, "state": t.state, "category": t.category,
				"save_path": f.savePath, "content_path": path.Join(f.savePath, t.category, t.name), "ratio": t.ratio,
				"ratio_limit": limit(t.ratioLimit), "seeding_time_limit": limit(t.seedingTimeLimit), "inactive_seeding_time_limit": -2,
				"seeding_time": 0, "last_activity": time.Now().Unix(),
			})
		}
		answer(list)
	case "/api/v2/torrents/properties":
		for _, t := range f.torrents {
			if t.hash == r.Form.Get("hash") && !t.deleted {
				answer(map[string]any{"save_path": f.savePath, "seeding_time": 0})
				return
			}
		}
		http.NotFound(w, r)
	case "/api/v2/torrents/delete":
		for _, t := range f.torrents {
			if t.hash == r.Form.Get("hashes") {
				t.deleted, t.deletedFiles = true, r.Form.Get("deleteFiles") == "true"
			}
		}
	default:
		http.NotFound(w, r)
	}
}

// limit is a seeding limit of the add form as the torrent list tells it:
// -2 for qBittorrent's global one.
func limit(s string) any {
	if s == "" {
		return -2
	}
	return json.Number(s)
}

// torrent returns a torrent by its hash, or nil.
func (f *fakeQBittorrent) torrent(hash string) *qbTorrent {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.torrents {
		if t.hash == hash {
			c := *t
			return &c
		}
	}
	return nil
}

// complete completes a torrent with a copy of a video file in its folder,
// which it then seeds.
func (f *fakeQBittorrent) complete(t *testing.T, hash, video string) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, tr := range f.torrents {
		if tr.hash == hash {
			file := filepath.Join(f.local, tr.category, tr.name, tr.name+".mkv")
			copyFile(t, video, file)
			tr.state = "uploading"
			return file
		}
	}
	t.Fatalf("no torrent %s", hash)
	return ""
}

// pause pauses a torrent at a ratio, as qBittorrent pauses those reaching
// their limits.
func (f *fakeQBittorrent) pause(hash string, ratio float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.torrents {
		if t.hash == hash {
			t.state, t.ratio = "pausedUP", ratio
		}
	}
}

// TestTorrentAcquisition runs the assembled server with the indexers and
// the download clients plugins against a Torznab indexer and qBittorrent
// faked on the loopback interface (P18's done criteria for torrents): a
// wanted movie is found and sent to qBittorrent as a magnet link with
// the indexer's seeding limits; qBittorrent's paths of another machine
// are mapped to the server's by a remote path mapping; the completed
// download, which keeps seeding, is imported by a hard link, and an
// upgrade, with hard links off, by a copy, leaving the files seeding in
// place, as Radarr's DownloadedMovieImportService with its
// TransferMode; a download is removed from qBittorrent with its files
// once paused at its seeding limits, as Radarr's DownloadEventHub.
func TestTorrentAcquisition(t *testing.T) {
	t.Parallel()
	s := startAcquisition(t, nil, true)
	ctx := t.Context()
	film, film1080 := s.video("1280x720", 610), s.video("1920x1080", 610)

	// qBittorrent runs on another machine, saving to /data/torrents, which
	// the server mounts elsewhere.
	movies, mounted := t.TempDir(), t.TempDir()
	indexer := newFakeTorznab(t)
	qb := newFakeQBittorrent(t, "/data/torrents", mounted)
	lib := s.library("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies)
	profile := s.profile(release.Movie, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)
	mapping, err := s.profiles.CreateRemotePathMapping(ctx, acquisitionv1.CreateRemotePathMappingRequest_builder{
		Mapping: acquisitionv1.RemotePathMapping_builder{Host: new("127.0.0.1"), RemotePath: new("/data/torrents/"), LocalPath: new(mounted)}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateRemotePathMapping: %v", err)
	}
	if m := mapping.GetMapping(); m.GetId() == "" || m.GetRemotePath() != "/data/torrents/" {
		t.Errorf("created mapping = %v", m)
	}
	s.plugins([]any{
		map[string]any{
			"id": "tz", "definition": "torznab", "settings": map[string]any{"base_url": indexer.URL, "api_key": "nzbkey"},
			"seed_ratio": 1.5, "seed_time": 90,
		},
	}, []any{
		map[string]any{"id": "qb", "name": "qBittorrent", "definition": "qbittorrent", "settings": map[string]any{
			"host": "127.0.0.1", "port": qb.Listener.Addr().(*net.TCPAddr).Port, "movie_category": "movies", "tv_category": "tv",
		}},
	})
	clients, err := s.downloads.ListDownloadClients(ctx, &acquisitionv1.ListDownloadClientsRequest{})
	if err != nil || clients.GetClients()[0].GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT {
		t.Fatalf("ListDownloadClients = %v, %v; want = qBittorrent, for torrents", clients, err)
	}

	added, err := s.wanted.AddMovie(ctx, acquisitionv1.AddMovieRequest_builder{
		LibraryId: new(lib), TmdbId: new(int32(603)), QualityProfileId: new(profile),
	}.Build())
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	movieID := added.GetItemId()
	folder := filepath.Join(movies, "The Matrix (1999)")
	day := time.Now().Add(-48 * time.Hour)

	// grabbed waits for a release to be sent to qBittorrent and tracked.
	grabbed := func(rel offered, quality string) {
		t.Helper()
		hash := infoHash(rel.id)
		var q *acquisitionv1.QueueItem
		waitFor(t, rel.title+" grabbed", func() bool {
			q = s.download(rel.title)
			return q != nil && qb.torrent(hash) != nil
		}, s.describeWanted(movieID))
		if q.GetQuality() != quality || q.GetClientId() != "qb" || q.GetDownloadId() != strings.ToUpper(hash) ||
			q.GetProtocol() != acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT || q.GetIndexer().GetIndexerId() != "tz" {
			t.Errorf("queued %s = %v, want = %s in qBittorrent, from the Torznab indexer", rel.title, q, quality)
		}
		// The magnet link is sent with the indexer's seeding limits, in the
		// movies' category.
		if tr := qb.torrent(hash); tr.magnet != magnet(rel) || tr.category != "movies" || tr.ratioLimit != "1.5" || tr.seedingTimeLimit != "90" {
			t.Errorf("torrent of %s = %+v, want = its magnet link in movies, seeding to 1.5 or 90 minutes", rel.title, tr)
		}
		s.waitForDownload(rel.title, acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING)
	}
	// imported completes a torrent, waits for its import, and returns the
	// file it seeds.
	imported := func(rel offered, video, want string) string {
		t.Helper()
		seeded := qb.complete(t, infoHash(rel.id), video)
		s.waitForDownload(rel.title, acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED)
		if it := s.item(movieID); it.GetVirtual() || it.GetPath() != filepath.ToSlash(want) {
			t.Errorf("movie after the import: virtual=%v path=%q, want = %q", it.GetVirtual(), it.GetPath(), filepath.ToSlash(want))
		}
		// The torrent keeps seeding: its files stay, and it stays in
		// qBittorrent.
		if _, err := os.Stat(seeded); err != nil {
			t.Errorf("seeded file after the import: %v", err)
		}
		if tr := qb.torrent(infoHash(rel.id)); tr.deleted {
			t.Errorf("torrent of %s deleted while seeding", rel.title)
		}
		return seeded
	}

	// 1. A search finds the HDTV torrent, imported by a hard link to the
	// file qBittorrent seeds.
	hdtv := offered{id: "tz01", title: "The.Matrix.1999.720p.HDTV.x264-GRPA", size: 4 << 30, published: day}
	indexer.offer(hdtv)
	if resp, err := s.wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{movieID}}.Build()); err != nil || resp.GetQueued() != 1 {
		t.Fatalf("SearchWanted = %v, %v; want = the movie queued", resp, err)
	}
	grabbed(hdtv, "HDTV-720p")
	first := filepath.Join(folder, "The Matrix (1999) HDTV-720p.mkv")
	seeded := imported(hdtv, film, first)
	if !same(t, first, seeded) {
		t.Errorf("%s is no hard link of the file seeding", first)
	}

	// 2. With hard links off, an RSS sync's WEB-DL is imported by a copy.
	s.settings(func(settings *systemv1.ServerSettings) { settings.GetAcquisition().SetCopyUsingHardlinks(false) })
	webdl := offered{id: "tz02", title: "The.Matrix.1999.1080p.WEB-DL.DD5.1.H.264-GRPB", size: 6 << 30, published: day}
	indexer.offer(webdl)
	if _, err := s.wanted.SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{}); err != nil {
		t.Fatalf("SyncRSS: %v", err)
	}
	grabbed(webdl, "WEBDL-1080p")
	upgraded := filepath.Join(folder, "The Matrix (1999) WEBDL-1080p.mkv")
	seeded2 := imported(webdl, film1080, upgraded)
	if same(t, upgraded, seeded2) {
		t.Errorf("%s is a hard link of the file seeding, want = a copy", upgraded)
	}
	if a, b := read(t, upgraded), read(t, seeded2); !bytes.Equal(a, b) {
		t.Errorf("copied %d bytes, want = the %d seeding", len(a), len(b))
	}
	// The hard link replaced goes to the recycle bin; the file seeding
	// stays.
	if _, err := os.Stat(first); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("replaced file: %v, want = gone", err)
	}
	if _, err := os.Stat(filepath.Join(s.recycleBin, "The Matrix (1999)", "The Matrix (1999) HDTV-720p.mkv")); err != nil {
		t.Errorf("replaced file in the recycle bin: %v", err)
	}
	if _, err := os.Stat(seeded); err != nil {
		t.Errorf("file seeding after the upgrade: %v", err)
	}

	// 3. Paused at its seeding limits, the first torrent is removed from
	// qBittorrent with its files.
	qb.pause(infoHash(hdtv.id), 1.5)
	waitFor(t, "the torrent seeded removed", func() bool {
		s.runTask("acquisition.queue")
		return qb.torrent(infoHash(hdtv.id)).deleted
	}, func() any { return qb.torrent(infoHash(hdtv.id)) })
	if tr := qb.torrent(infoHash(hdtv.id)); !tr.deletedFiles {
		t.Errorf("torrent of %s removed without its files", hdtv.title)
	}
	if tr := qb.torrent(infoHash(webdl.id)); tr.deleted {
		t.Errorf("torrent of %s, seeding, removed", webdl.title)
	}
}

// same reports whether two paths are the same file.
func same(t *testing.T, a, b string) bool {
	t.Helper()
	fa, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(fa, fb)
}

// read reads a file.
func read(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

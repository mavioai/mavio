package flood

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are Flood's settings. The tags take the place of categories:
// a download is of a kind when it has all of the kind's tags.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	// Directories are Radarr's Destination, by kind.
	client.Directories
	MovieTags           []string `json:"movie_tags"`
	TVTags              []string `json:"tv_tags"`
	MoviePostImportTags []string `json:"movie_post_import_tags"`
	TVPostImportTags    []string `json:"tv_post_import_tags"`
	StartOnAdd          bool     `json:"start_on_add"`
}

func (s *settings) tags(k client.Kind) []string {
	if k == client.Series {
		return s.TVTags
	}
	return s.MovieTags
}

func (s *settings) postImportTags(k client.Kind) []string {
	if k == client.Series {
		return s.TVPostImportTags
	}
	return s.MoviePostImportTags
}

func fields() []client.Field {
	f := client.ServerFields(3000, "")
	f[len(f)-1].HelpText = "Adds a prefix to the Flood API, such as [protocol]://[host]:[port]/[urlBase]/api"
	f[len(f)-1].Advanced = false
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.DirectoryFields("Flood")...)
	tags := "Initial tags of a download. To be recognized, a download must have all initial tags. This avoids conflicts with unrelated downloads."
	post := "Appends tags after a download is imported."
	return append(f,
		client.Field{Name: "movie_tags", Label: "Movie Tags", Type: client.FieldMultiSelect, Default: []string{client.DefaultMovieCategory}, HelpText: tags},
		client.Field{Name: "tv_tags", Label: "Series Tags", Type: client.FieldMultiSelect, Default: []string{client.DefaultTVCategory}, HelpText: tags},
		client.Field{Name: "movie_post_import_tags", Label: "Post-Import Movie Tags", Type: client.FieldMultiSelect, Advanced: true, HelpText: post},
		client.Field{Name: "tv_post_import_tags", Label: "Post-Import Series Tags", Type: client.FieldMultiSelect, Advanced: true, HelpText: post},
		client.Field{Name: "start_on_add", Label: "Start on Add", Type: client.FieldCheckbox, Default: true},
	)
}

// Definitions returns the definition of Flood.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "flood", Name: "Flood", Protocol: client.Torrent, Fields: fields(),
		Description: "Flood, over its API. Mavio removes torrents by the seed criteria of their indexers.",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{
				Server:     client.Server{Host: "localhost", Port: 3000},
				MovieTags:  []string{client.DefaultMovieCategory},
				TVTags:     []string{client.DefaultTVCategory},
				StartOnAdd: true,
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.Validate(); err != nil {
				return nil, err
			}
			f := &flood{settings: s, env: env, seeds: map[string]seed{}}
			f.proxy = &proxy{http: env.HTTP, settings: &f.settings}
			return f, nil
		},
	}}
}

// seed is the seeding a release asked for.
type seed struct {
	ratio *float64
	time  *time.Duration
}

// flood is the client of Radarr's Flood.
type flood struct {
	settings settings
	proxy    *proxy
	env      client.Env

	// seeds are the seeding of the releases added, by info hash, which
	// Radarr's DownloadSeedConfigProvider finds from the grab history.
	mu    sync.Mutex
	seeds map[string]seed
}

// Host implements client.Client.
func (f *flood) Host() string { return f.settings.Host }

// Add implements client.Client.
func (f *flood) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, f.env.Log, f, r)
}

// AddMagnet implements client.TorrentAdder.
func (f *flood) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	if err := f.proxy.addURL(ctx, link, f.addTags(r), f.settings.Directory(r.Kind)); err != nil {
		return "", err
	}
	f.remember(hash, r)
	return hash, nil
}

// AddTorrentFile implements client.TorrentAdder.
func (f *flood) AddTorrentFile(ctx context.Context, r *client.Release, hash, _ string, data []byte) (string, error) {
	if err := f.proxy.addFile(ctx, base64.StdEncoding.EncodeToString(data), f.addTags(r), f.settings.Directory(r.Kind)); err != nil {
		return "", err
	}
	f.remember(hash, r)
	return hash, nil
}

// addTags are the tags of a release's kind, as Radarr's HandleTags. The
// additional tags of the media and release are left out.
func (f *flood) addTags(r *client.Release) []string {
	return union(nil, f.settings.tags(r.Kind))
}

func (f *flood) remember(hash string, r *client.Release) {
	if r.SeedRatio == nil && r.SeedTime == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seeds[strings.ToUpper(hash)] = seed{ratio: r.SeedRatio, time: r.SeedTime}
}

// union returns tags with more appended, without blanks or duplicates.
func union(tags, more []string) []string {
	out := []string{}
	for _, t := range slices.Concat(tags, more) {
		if strings.TrimSpace(t) != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

// kindOf returns the kind whose tags a torrent has all of, and whether
// it is of either; zero when it is of both.
func (f *flood) kindOf(t *torrent) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		for _, tag := range f.settings.tags(k) {
			if !slices.Contains(t.Tags, tag) {
				return false
			}
		}
		return true
	}
	movie, series := of(client.Movie), of(client.Series)
	switch {
	case movie && series:
		return 0, true
	case movie:
		return client.Movie, true
	case series:
		return client.Series, true
	}
	return 0, false
}

// Items implements client.Client.
func (f *flood) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := f.proxy.torrents(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for _, hash := range slices.Sorted(maps.Keys(torrents)) {
		t := torrents[hash]
		kind, ok := f.kindOf(&t)
		if !ok {
			continue
		}
		ratio := t.Ratio
		item := client.Item{
			ID: strings.ToUpper(hash), Title: t.Name, Kind: kind, OutputPath: t.Directory,
			TotalSize: t.SizeBytes, RemainingSize: t.SizeBytes - t.BytesDone, SeedRatio: &ratio, Message: t.Message,
		}
		if len(t.Tags) > 0 {
			item.Category = t.Tags[0]
		}
		if t.ETA > 0 {
			eta := time.Duration(t.ETA) * time.Second
			item.RemainingTime = &eta
		}
		switch {
		case slices.Contains(t.Status, "seeding") || slices.Contains(t.Status, "complete"):
			item.Status = client.Completed
		case slices.Contains(t.Status, "stopped"):
			item.Status = client.Paused
		case slices.Contains(t.Status, "error"):
			item.Status = client.Warning
		case slices.Contains(t.Status, "downloading"):
			item.Status = client.Downloading
		default:
			item.Status = client.Queued
		}
		if opts.RemoveCompletedDownloads && item.Status == client.Completed {
			item.CanBeRemoved = f.doneSeeding(item.ID, &t)
			item.CanMoveFiles = item.CanBeRemoved
		}
		items = append(items, item)
	}
	return items, nil
}

// doneSeeding reports whether a torrent reached the ratio or, since it
// finished, the seed time its release asked for.
func (f *flood) doneSeeding(hash string, t *torrent) bool {
	f.mu.Lock()
	s, ok := f.seeds[hash]
	f.mu.Unlock()
	switch {
	case !ok:
		return false
	case s.ratio != nil && t.Ratio >= *s.ratio:
		return true
	case t.DateFinished != nil && *t.DateFinished > 0 && s.time != nil:
		return f.env.Now().Sub(time.Unix(*t.DateFinished, 0)) >= *s.time
	}
	return false
}

// ImportItem implements client.Importer: the output path becomes the file
// of a single-file torrent, or the folder all the files of a torrent
// share.
func (f *flood) ImportItem(ctx context.Context, item client.Item, _ client.Options) (client.Item, error) {
	paths, err := f.proxy.contentPaths(ctx, item.ID)
	if err != nil {
		return item, err
	}
	switch len(paths) {
	case 0:
		return item, &client.UnavailableError{Err: fmt.Errorf("failed to fetch list of contents of torrent: %s", item.ID)}
	case 1:
		item.OutputPath = joinPath(item.OutputPath, paths[0])
		return item, nil
	}
	first := func(p string) string {
		parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
		if len(parts) == 0 {
			return ""
		}
		return parts[0]
	}
	base := first(paths[0])
	for _, p := range paths[1:] {
		if first(p) != base {
			// The output path is the torrent's folder already.
			return item, nil
		}
	}
	item.OutputPath = joinPath(item.OutputPath, base)
	return item, nil
}

// MarkImported implements client.Marker: the post-import tags of the
// item's kind are added to the torrent's.
func (f *flood) MarkImported(ctx context.Context, item client.Item) error {
	kind := item.Kind
	if kind == 0 {
		kind = client.Movie
	}
	post := f.settings.postImportTags(kind)
	if len(post) == 0 {
		return nil
	}
	torrents, err := f.proxy.torrents(ctx)
	if err != nil {
		return err
	}
	for hash, t := range torrents {
		if strings.EqualFold(hash, item.ID) {
			return f.proxy.setTags(ctx, hash, union(t.Tags, post))
		}
	}
	return nil
}

// Remove implements client.Client.
func (f *flood) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if err := f.proxy.remove(ctx, item.ID, deleteData); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.seeds, item.ID)
	return nil
}

// Info implements client.Client.
func (f *flood) Info(ctx context.Context) (client.Info, error) {
	host := f.settings.Host
	info := client.Info{IsLocalhost: host == "127.0.0.1" || host == "::1" || host == "localhost"}
	var def *string
	for _, k := range client.Kinds {
		dir := f.settings.Directory(k)
		if strings.TrimSpace(dir) == "" {
			if def == nil {
				d, err := f.proxy.defaultDirectory(ctx)
				if err != nil {
					return info, err
				}
				def = &d
			}
			dir = *def
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Test implements client.Client.
func (f *flood) Test(ctx context.Context) error {
	return f.proxy.verify(ctx)
}

// joinPath joins a folder of the client's machine, Windows' or not, and
// a relative path.
func joinPath(dir, rel string) string {
	if dir == "" {
		return rel
	}
	sep := "/"
	if strings.Contains(dir, `\`) && !strings.Contains(dir, "/") {
		sep = `\`
	}
	return strings.TrimRight(dir, `/\`) + sep + strings.TrimLeft(rel, `/\`)
}

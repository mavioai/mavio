package rtorrent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// importedView is the view imported torrents are pushed to, as Radarr's
// "radarr_imported".
const importedView = "mavio_imported"

// settings are the settings of rTorrent.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Directories
	client.Priorities
	AddStopped bool `json:"add_stopped"`
}

func fields() []client.Field {
	f := client.ServerFields(8080, "RPC2")
	f[len(f)-1].Label = "URL Path"
	f[len(f)-1].Advanced = false
	f[len(f)-1].HelpText = "Path to the XMLRPC endpoint, see http(s)://[host]:[port]/[urlPath]. This is usually RPC2 or [path to ruTorrent]/plugins/rpc/rpc.php when using ruTorrent."
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, true)...)
	f = append(f, client.DirectoryFields("rTorrent")...)
	f = append(f, client.PriorityFields([]client.Option{
		{Value: priorityDoNotDownload, Label: "Do Not Download"},
		{Value: priorityLow, Label: "Low"},
		{Value: priorityNormal, Label: "Normal"},
		{Value: priorityHigh, Label: "High"},
	}, priorityNormal)...)
	return append(f, client.Field{
		Name: "add_stopped", Label: "Add Stopped", Type: client.FieldCheckbox, Default: false,
		HelpText: "Enabling will add torrents and magnets to rTorrent in a stopped state. This may break magnet files.",
	})
}

// Definitions returns the definition of rTorrent.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "rtorrent", Name: "rTorrent", Protocol: client.Torrent, Fields: fields(),
		Description: "rTorrent, over its XML-RPC; imported torrents are put in the " + importedView + " view",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{
				Server:     client.Server{Host: "localhost", Port: 8080, URLBase: "RPC2"},
				Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
				Priorities: client.Priorities{
					RecentMoviePriority: priorityNormal, OlderMoviePriority: priorityNormal,
					RecentTVPriority: priorityNormal, OlderTVPriority: priorityNormal,
				},
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.Validate(); err != nil {
				return nil, err
			}
			r := &rtorrent{settings: s, env: env, retryDelay: 500 * time.Millisecond, seeds: map[string]seedConfig{}}
			r.proxy = &proxy{http: env.HTTP, settings: &r.settings}
			return r, nil
		},
	}}
}

// seedConfig is the seeding a release's indexer asks for, as Radarr's
// TorrentSeedConfiguration.
type seedConfig struct {
	Ratio *float64
	Time  *time.Duration
}

// rtorrent is the client of Radarr's RTorrent.
type rtorrent struct {
	settings settings
	proxy    *proxy
	env      client.Env
	// retryDelay is how long to wait between checks for an added torrent.
	retryDelay time.Duration

	// seeds are the seed configurations of added torrents by hash, as
	// Radarr's DownloadSeedConfigProvider caches them.
	mu    sync.Mutex
	seeds map[string]seedConfig
}

// Host implements client.Client.
func (r *rtorrent) Host() string { return r.settings.Host }

// Add implements client.Client.
func (r *rtorrent) Add(ctx context.Context, rel *client.Release) (string, error) {
	return client.AddTorrent(ctx, r.env.Log, r, rel)
}

// AddMagnet implements client.TorrentAdder: the magnet is given a while
// to resolve, and kept track of even when it does not.
func (r *rtorrent) AddMagnet(ctx context.Context, rel *client.Release, hash, link string) (string, error) {
	priority := r.settings.Priority(rel.Kind, rel.Recent)
	if err := r.proxy.addURL(ctx, link, r.settings.Category(rel.Kind), priority, r.settings.Directory(rel.Kind)); err != nil {
		return "", err
	}
	r.remember(hash, rel)
	ok, err := r.waitForTorrent(ctx, hash)
	if err != nil {
		return "", err
	}
	if !ok {
		r.env.Log.WarnContext(ctx, "rTorrent could not resolve magnet within 5 seconds, download may remain stuck", "magnet", link)
	}
	return hash, nil
}

// AddTorrentFile implements client.TorrentAdder.
func (r *rtorrent) AddTorrentFile(ctx context.Context, rel *client.Release, hash, fileName string, data []byte) (string, error) {
	priority := r.settings.Priority(rel.Kind, rel.Recent)
	if err := r.proxy.addFile(ctx, fileName, data, r.settings.Category(rel.Kind), priority, r.settings.Directory(rel.Kind)); err != nil {
		return "", err
	}
	ok, err := r.waitForTorrent(ctx, hash)
	if err != nil {
		return "", err
	}
	if !ok {
		r.env.Log.DebugContext(ctx, "rTorrent didn't add the torrent within 5 seconds", "file", fileName)
		return "", errors.New("downloading torrent failed")
	}
	r.remember(hash, rel)
	return hash, nil
}

// remember keeps the seeding a release asks for.
func (r *rtorrent) remember(hash string, rel *client.Release) {
	if rel.SeedRatio == nil && rel.SeedTime == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seeds[strings.ToUpper(hash)] = seedConfig{Ratio: rel.SeedRatio, Time: rel.SeedTime}
}

// waitForTorrent checks 10 times, retryDelay apart, that rTorrent has a
// torrent.
func (r *rtorrent) waitForTorrent(ctx context.Context, hash string) (bool, error) {
	const tries = 10
	for i := range tries {
		ok, err := r.proxy.hasTorrent(ctx, hash)
		if err != nil || ok {
			return ok, err
		}
		if i == tries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(r.retryDelay):
		}
	}
	r.env.Log.DebugContext(ctx, "could not find the torrent", "hash", hash, "tries", tries, "interval", r.retryDelay)
	return false, nil
}

// Items implements client.Client.
func (r *rtorrent) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := r.proxy.torrents(ctx)
	if err != nil {
		return nil, err
	}
	r.env.Log.DebugContext(ctx, "retrieved metadata of torrents in client", "count", len(torrents))
	r.forget(torrents)
	var items []client.Item
	for i := range torrents {
		t := &torrents[i]
		kind, ok := r.settings.Match(func(c string) bool { return strings.TrimSpace(c) == "" || t.Category == c })
		if !ok {
			continue
		}
		if strings.TrimSpace(t.Path) == "" {
			r.env.Log.WarnContext(ctx, "torrent has an empty download path and will not be processed; adjust this to an absolute path in rTorrent", "torrent", t.Name)
			continue
		}
		if strings.HasPrefix(t.Path, ".") {
			r.env.Log.WarnContext(ctx, "torrent has a download path starting with '.' and will not be processed; adjust this to an absolute path in rTorrent", "torrent", t.Name)
			continue
		}
		ratio := float64(t.Ratio) / 1000
		remaining := time.Duration(0)
		if t.DownRate > 0 {
			remaining = time.Duration(t.RemainingSize/t.DownRate) * time.Second
		}
		item := client.Item{
			ID: strings.ToUpper(t.Hash), Title: t.Name, Category: t.Category, Kind: kind,
			OutputPath: t.Path, TotalSize: t.TotalSize, RemainingSize: t.RemainingSize,
			RemainingTime: &remaining, SeedRatio: &ratio,
			HasPostImportCategory: strings.TrimSpace(r.settings.ImportedCategory(kind)) != "",
		}
		switch {
		case t.IsFinished:
			item.Status = client.Completed
		case t.IsActive:
			item.Status = client.Downloading
		default:
			item.Status = client.Paused
		}
		if opts.RemoveCompletedDownloads && t.IsFinished {
			if seed, ok := r.seedConfig(t.Hash); ok {
				item.CanBeRemoved = r.seedingDone(t, ratio, seed)
				item.CanMoveFiles = item.CanBeRemoved
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// seedingDone reports whether a finished torrent met its seed ratio or
// seed time.
func (r *rtorrent) seedingDone(t *torrent, ratio float64, seed seedConfig) bool {
	if seed.Ratio != nil && ratio >= *seed.Ratio {
		return true
	}
	return seed.Time != nil && r.env.Now().Sub(time.Unix(t.FinishedTime, 0)) >= *seed.Time
}

func (r *rtorrent) seedConfig(hash string) (seedConfig, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seeds[strings.ToUpper(hash)]
	return s, ok
}

// forget drops the seed configurations of torrents no longer in rTorrent.
func (r *rtorrent) forget(torrents []torrent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for hash := range r.seeds {
		if !slices.ContainsFunc(torrents, func(t torrent) bool { return strings.EqualFold(t.Hash, hash) }) {
			delete(r.seeds, hash)
		}
	}
}

// Remove implements client.Client: the files are deleted by Mavio, then
// the torrent erased.
func (r *rtorrent) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if deleteData {
		client.DeleteItemData(ctx, r.env.Log, item)
	}
	return r.proxy.remove(ctx, item.ID)
}

// MarkImported implements client.Marker: the torrent's label becomes the
// post-import category, and it is pushed to the imported view.
func (r *rtorrent) MarkImported(ctx context.Context, item client.Item) error {
	kind := item.Kind
	if kind == 0 {
		kind = client.Movie
	}
	hash := strings.ToLower(item.ID)
	imported := r.settings.ImportedCategory(kind)
	if strings.TrimSpace(imported) != "" && imported != r.settings.Category(kind) {
		if err := r.proxy.setLabel(ctx, hash, imported); err != nil {
			r.env.Log.WarnContext(ctx, "failed to set the post-import torrent label in rTorrent; does the label exist?",
				"label", imported, "download", item.Title, "err", err)
		}
	}
	if err := r.proxy.pushUniqueView(ctx, hash, importedView); err != nil {
		r.env.Log.WarnContext(ctx, "failed to set the post-import torrent view in rTorrent",
			"view", importedView, "download", item.Title, "err", err)
	}
	return nil
}

// Info implements client.Client.
func (r *rtorrent) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: r.settings.IsLocalhost()}, nil
}

// Test implements client.Client.
func (r *rtorrent) Test(ctx context.Context) error {
	version, err := r.proxy.version(ctx)
	if err != nil {
		r.env.Log.ErrorContext(ctx, "failed to test rTorrent", "err", err)
		return fmt.Errorf("unable to connect to rTorrent: %w", err)
	}
	if v, ok := parseVersion(version); !ok || slices.Compare(v, []int{0, 9, 0}) < 0 {
		return fmt.Errorf("rTorrent version %s is not supported; 0.9.0 or later is required", version)
	}
	if _, err := r.proxy.torrents(ctx); err != nil {
		r.env.Log.ErrorContext(ctx, "failed to get torrents", "err", err)
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return r.testDirectories()
}

// parseVersion reads a version such as "0.9.8", as .NET's Version.
func parseVersion(s string) ([]int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, false
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

var windowsPath = regexp.MustCompile(`^[a-zA-Z]:[\\/]|^\\\\`)

// testDirectories checks the directories, as Radarr's
// RTorrentDirectoryValidator, when rTorrent runs on this machine: they
// are to be valid, absolute and existing.
func (r *rtorrent) testDirectories() error {
	if !r.settings.IsLocalhost() {
		return nil
	}
	for _, k := range client.Kinds {
		dir := r.settings.Directory(k)
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if !strings.HasPrefix(dir, "/") && !windowsPath.MatchString(dir) {
			return fmt.Errorf("directory: invalid path: %q", dir)
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return fmt.Errorf("directory: path %q does not exist", dir)
		}
	}
	return nil
}

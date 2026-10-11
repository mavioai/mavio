package utorrent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are uTorrent's settings.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Priorities
	InitialState int `json:"initial_state"`
}

// The priorities: last puts torrents at the end of the queue, first at
// its top.
const (
	priorityLast  = 0
	priorityFirst = 1
)

// The initial states, and the actions putting torrents in them.
const (
	stateStart = iota
	stateForceStart
	statePause
	stateStop
)

var stateActions = []string{"start", "forcestart", "pause", "stop"}

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := client.ValidateURLBase("url base", s.URLBase, "/gui"); err != nil {
		return err
	}
	if strings.TrimSpace(s.MovieCategory) == "" || strings.TrimSpace(s.TVCategory) == "" {
		return errors.New("the movie and series categories are required")
	}
	if s.InitialState < stateStart || s.InitialState > stateStop {
		return fmt.Errorf("initial state %d is out of range", s.InitialState)
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(8080, "")
	f[len(f)-1].HelpText = "Adds a prefix to the uTorrent url, such as http://[host]:[port]/[urlBase]/api"
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, true)...)
	f = append(f, client.PriorityFields([]client.Option{{Value: priorityLast, Label: "Last"}, {Value: priorityFirst, Label: "First"}}, priorityLast)...)
	return append(f, client.Field{
		Name: "initial_state", Label: "Initial State", Type: client.FieldSelect, Default: stateStart,
		HelpText: "Initial state for torrents added to uTorrent",
		Options: []client.Option{
			{Value: stateStart, Label: "Start"},
			{Value: stateForceStart, Label: "Force Start"},
			{Value: statePause, Label: "Pause"},
			{Value: stateStop, Label: "Stop"},
		},
	})
}

// Definitions returns the definition of uTorrent.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "utorrent", Name: "uTorrent", Protocol: client.Torrent, Fields: fields(),
		Description: "uTorrent, over its Web UI API. uTorrent has a history of including cryptominers, malware and ads; " +
			"choosing a different client is strongly encouraged.",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{
				Server:     client.Server{Host: "localhost", Port: 8080},
				Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.validate(); err != nil {
				return nil, err
			}
			u := &utorrent{settings: s, env: env}
			u.proxy = &proxy{http: env.HTTP, settings: &u.settings}
			return u, nil
		},
	}}
}

// cacheTTL is how long the torrents listed are kept for the next list to
// be incremental.
const cacheTTL = 15 * time.Minute

// utorrent is the client of Radarr's UTorrent.
type utorrent struct {
	settings settings
	proxy    *proxy
	env      client.Env

	// The torrents listed last and the cache ID uTorrent gave with them,
	// as Radarr's differential torrent cache.
	mu       sync.Mutex
	cacheID  string
	cached   []torrent
	cachedAt time.Time
}

// Host implements client.Client.
func (u *utorrent) Host() string { return u.settings.Host }

// Add implements client.Client.
func (u *utorrent) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, u.env.Log, u, r)
}

// AddMagnet implements client.TorrentAdder.
func (u *utorrent) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	if err := u.proxy.addURL(ctx, link); err != nil {
		return "", err
	}
	return u.added(ctx, r, hash)
}

// AddTorrentFile implements client.TorrentAdder.
func (u *utorrent) AddTorrentFile(ctx context.Context, r *client.Release, hash, fileName string, data []byte) (string, error) {
	if err := u.proxy.addFile(ctx, fileName, data); err != nil {
		return "", err
	}
	return u.added(ctx, r, hash)
}

// added sets an added torrent's seeding, label, place in the queue and
// state.
func (u *utorrent) added(ctx context.Context, r *client.Release, hash string) (string, error) {
	if err := u.proxy.setSeeding(ctx, hash, r); err != nil {
		return "", err
	}
	if c := u.settings.Category(r.Kind); strings.TrimSpace(c) != "" {
		if err := u.proxy.setLabel(ctx, hash, c); err != nil {
			return "", err
		}
	}
	if u.settings.Priority(r.Kind, r.Recent) == priorityFirst {
		if err := u.proxy.moveToTop(ctx, hash); err != nil {
			return "", err
		}
	}
	if err := u.proxy.setState(ctx, hash, u.settings.InitialState); err != nil {
		return "", err
	}
	return hash, nil
}

// torrents lists the torrents, incrementally from the last list while it
// is cached, as Radarr's GetTorrents.
func (u *utorrent) torrents(ctx context.Context) ([]torrent, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.env.Now()
	cached := u.cachedAt != (time.Time{}) && now.Sub(u.cachedAt) < cacheTTL
	cacheID := ""
	if cached {
		cacheID = u.cacheID
	}
	r, err := u.proxy.torrents(ctx, cacheID)
	if err != nil {
		return nil, err
	}
	torrents := r.Torrents
	if cached && r.Torrents == nil {
		gone := map[string]bool{}
		for _, t := range r.Changed {
			gone[t.Hash] = true
		}
		for _, h := range r.Removed {
			gone[h] = true
		}
		torrents = nil
		for _, t := range u.cached {
			if !gone[t.Hash] {
				torrents = append(torrents, t)
			}
		}
		torrents = append(torrents, r.Changed...)
	}
	u.cacheID, u.cached, u.cachedAt = r.CacheID, torrents, now
	return torrents, nil
}

// Items implements client.Client.
func (u *utorrent) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := u.torrents(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for _, t := range torrents {
		kind, ok := u.settings.Match(func(c string) bool { return t.Label == c })
		if !ok {
			continue
		}
		ratio := t.Ratio
		item := client.Item{
			ID: t.Hash, Title: t.Name, Category: t.Label, Kind: kind,
			TotalSize: t.Size, RemainingSize: t.Remaining, SeedRatio: &ratio,
			OutputPath:            outputPath(t.RootDownloadPath, t.Name),
			HasPostImportCategory: strings.TrimSpace(u.settings.ImportedCategory(kind)) != "",
		}
		if t.ETA != -1 {
			eta := time.Duration(t.ETA) * time.Second
			item.RemainingTime = &eta
		}
		switch {
		case t.Status&statusError != 0:
			item.Status = client.Warning
			item.Message = "uTorrent is reporting an error"
		case t.Status&statusLoaded != 0 && t.Status&statusChecked != 0 && t.Remaining == 0 && t.Progress == 1:
			item.Status = client.Completed
		case t.Status&statusPaused != 0:
			item.Status = client.Paused
		case t.Status&statusStarted != 0:
			item.Status = client.Downloading
		default:
			item.Status = client.Queued
		}
		// Started without queued is forced seeding.
		item.CanBeRemoved = opts.RemoveCompletedDownloads && t.Status&(statusQueued|statusStarted) == 0
		item.CanMoveFiles = item.CanBeRemoved
		items = append(items, item)
	}
	return items, nil
}

// outputPath is where a torrent's files are: its root download path when
// that is named after the torrent, else the torrent's name in it.
func outputPath(root, name string) string {
	if client.RemoteBase(root) == name {
		return root
	}
	return client.JoinRemote(root, name)
}

// Remove implements client.Client.
func (u *utorrent) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return u.proxy.remove(ctx, item.ID, deleteData)
}

// MarkImported implements client.Marker: the torrent's label becomes the
// post-import category.
func (u *utorrent) MarkImported(ctx context.Context, item client.Item) error {
	kind := item.Kind
	if kind == 0 {
		kind = client.Movie
	}
	imported, category := u.settings.ImportedCategory(kind), u.settings.Category(kind)
	if strings.TrimSpace(imported) == "" || imported == category {
		return nil
	}
	hash := strings.ToLower(item.ID)
	if err := u.proxy.setLabel(ctx, hash, imported); err != nil {
		return err
	}
	if strings.TrimSpace(category) != "" {
		return u.proxy.removeLabel(ctx, hash, category)
	}
	return nil
}

// Info implements client.Client.
func (u *utorrent) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: u.settings.IsLocalhost()}
	config, err := u.proxy.config(ctx)
	if err != nil {
		return info, err
	}
	var dir string
	if config["dir_active_download_flag"] == "true" {
		dir = config["dir_active_download"]
	}
	completed := config["dir_completed_download_flag"] == "true"
	if completed {
		dir = config["dir_completed_download"]
	}
	if dir == "" {
		return info, nil
	}
	for _, k := range client.Kinds {
		d := dir
		if completed && config["dir_add_label"] == "true" {
			d = client.JoinRemote(dir, u.settings.Category(k))
		}
		if !slices.Contains(info.OutputRootFolders, d) {
			info.OutputRootFolders = append(info.OutputRootFolders, d)
		}
	}
	return info, nil
}

// minBuild is the build of uTorrent 3.0.
const minBuild = 25406

// Test implements client.Client.
func (u *utorrent) Test(ctx context.Context) error {
	build, err := u.proxy.version(ctx)
	if err != nil {
		var auth *client.AuthError
		if errors.As(err, &auth) {
			return fmt.Errorf("authentication failure; verify the credentials, and that Mavio is not blocked from uTorrent by its whitelist: %w", err)
		}
		return fmt.Errorf("unable to connect to uTorrent: %w", err)
	}
	if build < minBuild {
		return fmt.Errorf("uTorrent version should be at least 3.0. Version reported is %d", build)
	}
	if _, err := u.proxy.torrents(ctx, ""); err != nil {
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return nil
}

package freebox

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the Freebox's settings, as Radarr's FreeboxDownloadSettings
// with a category and a directory for each kind.
type settings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	UseSSL   bool   `json:"use_ssl"`
	APIURL   string `json:"api_url"`
	AppID    string `json:"app_id"`
	AppToken string `json:"app_token"`
	client.Categories
	client.Directories
	client.Priorities
	AddPaused bool `json:"add_paused"`
}

// The priorities: last puts tasks at the end of the queue, first at its
// top.
const (
	priorityLast  = 0
	priorityFirst = 1
)

var (
	categoryPattern = regexp.MustCompile(`(?i)^\.?[-a-z]*$`)
	windowsPath     = regexp.MustCompile(`^[a-zA-Z]:[\\/]`)
)

func (s *settings) validate() error {
	server := client.Server{Host: s.Host, Port: s.Port}
	if err := server.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.APIURL) == "" {
		return errors.New("'API URL' must not be empty")
	}
	if err := client.ValidateURLBase("'API URL'", s.APIURL, "/api/v1/"); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(s.AppID) == "":
		return errors.New("'App ID' must not be empty")
	case strings.TrimSpace(s.AppToken) == "":
		return errors.New("'App Token' must not be empty")
	}
	for _, k := range client.Kinds {
		c, dir := s.Category(k), strings.TrimSpace(s.Directory(k))
		switch {
		case !categoryPattern.MatchString(c):
			return fmt.Errorf("category %q: allowed characters a-z and -", c)
		case dir != "" && !strings.HasPrefix(dir, "/") && !windowsPath.MatchString(dir) && !strings.HasPrefix(dir, `\\`):
			return fmt.Errorf("directory %q: must be an absolute path", dir)
		case strings.TrimSpace(c) != "" && dir != "":
			return errors.New("cannot use 'Category' and 'Destination Directory' at the same time")
		}
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(443, "-")
	f[0].Default = "mafreebox.freebox.fr"
	f[0].HelpText = "Hostname or host IP address of the Freebox, defaults to 'mafreebox.freebox.fr' (will only work if on same network)"
	f[1].HelpText = "Port used to access Freebox interface, defaults to '443'"
	f[2].Default = true
	f[2].HelpText = "Use secured connection when connecting to Freebox API"
	f = append(f,
		client.Field{
			Name: "api_url", Label: "API URL", Type: client.FieldText, Default: "/api/v1/", Advanced: true,
			HelpText: "Define Freebox API base URL with API version, eg 'http://[host]:[port]/[api_base_url]/[api_version]/', defaults to '/api/v1/'",
		},
		client.Field{Name: "app_id", Label: "App ID", Type: client.FieldText, HelpText: "App ID given when creating access to Freebox API (ie 'app_id')"},
		client.Field{
			Name: "app_token", Label: "App Token", Type: client.FieldText, Secret: true,
			HelpText: "App token retrieved when creating access to Freebox API (ie 'app_token')",
		},
	)
	dirs := client.DirectoryFields("Freebox")
	for i := range dirs {
		dirs[i].HelpText = "Optional location to put downloads in, leave blank to use the default Freebox location"
	}
	f = append(f, dirs...)
	categories := client.CategoryFields("Category", "", "", false)
	for i := range categories {
		categories[i].HelpText += "; creates a [category] subdirectory in the output directory"
	}
	f = append(f, categories...)
	f = append(f, client.PriorityFields([]client.Option{{Value: priorityLast, Label: "Last"}, {Value: priorityFirst, Label: "First"}}, priorityLast)...)
	return append(f, client.Field{Name: "add_paused", Label: "Add Paused", Type: client.FieldCheckbox, Default: false})
}

// Definitions returns the Freebox's definition.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "freebox", Name: "Freebox Download", Protocol: client.Torrent, Fields: fields(),
		Description: "The Freebox's downloader, over the Freebox OS API with an app token",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{Host: "mafreebox.freebox.fr", Port: 443, UseSSL: true, APIURL: "/api/v1/"}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.validate(); err != nil {
				return nil, err
			}
			f := &freebox{settings: s, env: env}
			f.proxy = &proxy{http: env.HTTP, s: &f.settings, log: env.Log}
			return f, nil
		},
	}}
}

// freebox is the Freebox's downloader, as Radarr's TorrentFreeboxDownload.
type freebox struct {
	settings settings
	proxy    *proxy
	env      client.Env
}

// Host implements client.Client.
func (f *freebox) Host() string { return f.settings.Host }

// kindOf returns the kind of a task by its folder, as Radarr's and
// Sonarr's GetItems filter tasks, and whether it is of either; zero when
// it is of both.
func (f *freebox) kindOf(dir string) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		if d := f.settings.Directory(k); strings.TrimSpace(d) != "" && !containsPath(d, dir) {
			return false
		}
		if c := f.settings.Category(k); strings.TrimSpace(c) != "" {
			return slices.Contains(strings.FieldsFunc(dir, func(r rune) bool { return r == '/' || r == '\\' }), c)
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

// category returns the category of a kind; for both, theirs when it is
// one.
func (f *freebox) category(k client.Kind) string {
	if k != 0 {
		return f.settings.Category(k)
	}
	if c := f.settings.Category(client.Movie); c == f.settings.Category(client.Series) {
		return c
	}
	return ""
}

// Items implements client.Client.
func (f *freebox) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	tasks, err := f.proxy.tasks(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for i := range tasks {
		t := &tasks[i]
		if !strings.EqualFold(t.Type, "bt") {
			continue
		}
		dir := t.downloadDir()
		kind, ok := f.kindOf(dir)
		if !ok {
			continue
		}
		var ratio float64
		if t.StopRatio > 0 {
			ratio = t.StopRatio / 100
		}
		item := client.Item{
			ID: string(t.ID), Title: t.Name, Category: f.category(kind), Kind: kind,
			TotalSize:     t.Size,
			RemainingSize: int64(float64(t.Size) * (1 - float64(t.ReceivedPct)/10000)),
			SeedRatio:     &ratio,
			OutputPath:    dir,
		}
		if t.ETA > 0 {
			d := time.Duration(t.ETA) * time.Second
			item.RemainingTime = &d
		}
		switch t.Status {
		case "stopped", "stopping":
			item.Status = client.Paused
		case "queued":
			item.Status = client.Queued
		case "starting", "downloading", "retry", "checking":
			item.Status = client.Downloading
		case "error":
			item.Status = client.Warning
			item.Message = t.errorDescription()
		case "done", "seeding":
			item.Status = client.Completed
		default:
			item.Status = client.Downloading
			item.Message = "Unknown download state: " + t.Status
			f.env.Log.InfoContext(ctx, "unknown Freebox download state", "state", t.Status, "download", t.Name)
		}
		item.CanBeRemoved = opts.RemoveCompletedDownloads && t.Status == "done"
		item.CanMoveFiles = item.CanBeRemoved
		items = append(items, item)
	}
	return items, nil
}

// downloadDirectory returns the folder of a kind's downloads, with the
// release's own subfolder when title is set, as Radarr's
// GetDownloadDirectory.
func (f *freebox) downloadDirectory(ctx context.Context, k client.Kind, title string) (string, error) {
	var dir string
	if d := f.settings.Directory(k); strings.TrimSpace(d) != "" {
		dir = strings.TrimRight(d, "/")
	} else {
		def, err := f.proxy.defaultDir(ctx)
		if err != nil {
			return "", err
		}
		dir = strings.TrimRight(def, "/")
		if c := f.settings.Category(k); strings.TrimSpace(c) != "" {
			dir += "/" + c
		}
	}
	if strings.TrimSpace(title) != "" {
		dir += "/" + client.CleanFileName(title)
	}
	return dir, nil
}

// Info implements client.Client.
func (f *freebox) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: client.IsLocalhost(f.settings.Host)}
	for _, k := range client.Kinds {
		dir, err := f.downloadDirectory(ctx, k, "")
		if err != nil {
			return client.Info{}, err
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Add implements client.Client.
func (f *freebox) Add(ctx context.Context, r *client.Release) (string, error) {
	// AddTorrent upper-cases what the adder returns, for info hashes; the
	// Freebox's IDs are its own.
	a := &rawID{TorrentAdder: f}
	if _, err := client.AddTorrent(ctx, f.env.Log, a, r); err != nil {
		return "", err
	}
	return a.id, nil
}

func (f *freebox) addOptions(r *client.Release) addOptions {
	opts := addOptions{
		paused: f.settings.AddPaused,
		first:  f.settings.Priority(r.Kind, r.Recent) == priorityFirst,
	}
	if r.SeedRatio != nil {
		ratio := *r.SeedRatio * 100
		opts.seedRatio = &ratio
	}
	return opts
}

// AddMagnet implements client.TorrentAdder.
func (f *freebox) AddMagnet(ctx context.Context, r *client.Release, _, link string) (string, error) {
	dir, err := f.downloadDirectory(ctx, r.Kind, r.Title)
	if err != nil {
		return "", err
	}
	return f.proxy.addURL(ctx, link, dir, f.addOptions(r))
}

// AddTorrentFile implements client.TorrentAdder.
func (f *freebox) AddTorrentFile(ctx context.Context, r *client.Release, _, fileName string, data []byte) (string, error) {
	dir, err := f.downloadDirectory(ctx, r.Kind, r.Title)
	if err != nil {
		return "", err
	}
	return f.proxy.addFile(ctx, fileName, data, dir, f.addOptions(r))
}

// rawID keeps the ID an adder returns as it is.
type rawID struct {
	client.TorrentAdder
	id string
}

func (a *rawID) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	id, err := a.TorrentAdder.AddMagnet(ctx, r, hash, link)
	a.id = id
	return "", err
}

func (a *rawID) AddTorrentFile(ctx context.Context, r *client.Release, hash, fileName string, data []byte) (string, error) {
	id, err := a.TorrentAdder.AddTorrentFile(ctx, r, hash, fileName, data)
	a.id = id
	return "", err
}

// Remove implements client.Client; the Freebox erases the files itself.
func (f *freebox) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return f.proxy.remove(ctx, item.ID, deleteData)
}

// Test implements client.Client, as Radarr's: the app token opens a
// logged-in session.
func (f *freebox) Test(ctx context.Context) error {
	err := f.proxy.authenticate(ctx)
	if errors.Is(err, errNotLoggedIn) {
		return &client.AuthError{Err: errors.New("the session is not logged in; verify the App ID and App Token")}
	}
	return err
}

// containsPath reports whether p is folder or inside it, as Radarr's
// OsPath.Contains.
func containsPath(folder, p string) bool {
	norm := func(s string) string { return path.Clean(strings.ReplaceAll(s, `\`, "/")) }
	f, q := norm(folder), norm(p)
	return q == f || strings.HasPrefix(q, strings.TrimRight(f, "/")+"/")
}

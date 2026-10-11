package deluge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are Deluge's settings: the download and completed directories
// are by kind, as Radarr's and Sonarr's own are.
type settings struct {
	client.Server
	Password string `json:"password"`
	client.Categories
	client.Priorities
	AddPaused bool `json:"add_paused"`
	client.Directories
	MovieCompletedDirectory string `json:"movie_completed_directory"`
	TVCompletedDirectory    string `json:"tv_completed_directory"`
}

// completedDirectory returns where completed torrents of a kind are moved
// to; empty for Deluge's default.
func (s *settings) completedDirectory(k client.Kind) string {
	if k == client.Series {
		return s.TVCompletedDirectory
	}
	return s.MovieCompletedDirectory
}

// The priorities: last leaves torrents at the end of the queue, first
// moves them to its top.
const (
	priorityLast  = 0
	priorityFirst = 1
)

var labelPattern = regexp.MustCompile(`^[-a-z0-9]*$`)

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	for _, c := range []string{s.MovieCategory, s.MovieImportedCategory, s.TVCategory, s.TVImportedCategory} {
		if !labelPattern.MatchString(c) {
			return fmt.Errorf("category %q: allowed characters a-z, 0-9 and -", c)
		}
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(8112, "")
	f[len(f)-1].HelpText = "Adds a prefix to the deluge json url, see http://[host]:[port]/[urlBase]/json"
	f = append(f, client.Field{Name: "password", Label: "Password", Type: client.FieldText, Default: "deluge", Secret: true})
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, true)...)
	f = append(f, client.PriorityFields([]client.Option{{Value: priorityLast, Label: "Last"}, {Value: priorityFirst, Label: "First"}}, priorityLast)...)
	f = append(f, client.Field{Name: "add_paused", Label: "Add Paused", Type: client.FieldCheckbox, Default: false})
	for _, d := range client.DirectoryFields("Deluge") {
		d.Label = strings.Replace(d.Label, "Directory", "Download Directory", 1)
		f = append(f, d)
	}
	help := "Optional location to move completed downloads to, leave blank to use the default Deluge location"
	return append(f,
		client.Field{Name: "movie_completed_directory", Label: "Movie Move When Completed Directory", Type: client.FieldText, Advanced: true, HelpText: help},
		client.Field{Name: "tv_completed_directory", Label: "Series Move When Completed Directory", Type: client.FieldText, Advanced: true, HelpText: help},
	)
}

// Definitions returns the definition of Deluge.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "deluge", Name: "Deluge", Protocol: client.Torrent, Fields: fields(),
		Description: "Deluge, over its web UI's JSON-RPC",
		New:         newDeluge,
	}}
}

func newDeluge(raw []byte, env client.Env) (client.Client, error) {
	s := settings{
		Server:     client.Server{Host: "localhost", Port: 8112},
		Password:   "deluge",
		Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
	}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	d := &deluge{settings: s, env: env}
	d.proxy = &proxy{http: env.HTTP, url: s.BaseURL() + "/json", password: s.Password, log: env.Log}
	return d, nil
}

// deluge is the client of Radarr's Deluge.
type deluge struct {
	settings settings
	proxy    *proxy
	env      client.Env

	mu sync.Mutex
	// reconnected is set once torrents without a hash or name had the web
	// UI reconnect to the daemon, until they are gone.
	reconnected bool
}

// Host implements client.Client.
func (d *deluge) Host() string { return d.settings.Host }

// Items implements client.Client: the torrents of each kind's label, or
// all of them for a kind without one.
func (d *deluge) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	type found struct {
		t             torrent
		movie, series bool
	}
	byHash := map[string]*found{}
	var order []string
	queried := map[string]bool{}
	ignored := 0
	for _, k := range client.Kinds {
		label := d.settings.Category(k)
		if queried[label] {
			continue
		}
		queried[label] = true
		torrents, err := d.proxy.torrents(ctx, label)
		if err != nil {
			return nil, err
		}
		for _, t := range torrents {
			// Torrents without a hash or name are left out; Deluge may
			// have lost its daemon.
			if strings.TrimSpace(t.Hash) == "" || strings.TrimSpace(t.Name) == "" {
				ignored++
				continue
			}
			f := byHash[t.Hash]
			if f == nil {
				f = &found{t: t}
				byHash[t.Hash] = f
				order = append(order, t.Hash)
			}
			for _, kind := range client.Kinds {
				if d.settings.Category(kind) != label {
					continue
				}
				if kind == client.Movie {
					f.movie = true
				} else {
					f.series = true
				}
			}
		}
	}
	d.checkIgnored(ctx, ignored)

	items := make([]client.Item, 0, len(order))
	for _, hash := range order {
		f := byHash[hash]
		var kind client.Kind
		switch {
		case f.movie && !f.series:
			kind = client.Movie
		case f.series && !f.movie:
			kind = client.Series
		}
		items = append(items, d.item(&f.t, kind, opts))
	}
	return items, nil
}

// checkIgnored has the web UI reconnect to the daemon the first time
// torrents come without a hash or name, and warns after, as Radarr means
// to.
func (d *deluge) checkIgnored(ctx context.Context, ignored int) {
	d.mu.Lock()
	reconnected := d.reconnected
	d.reconnected = ignored > 0
	d.mu.Unlock()
	switch {
	case ignored == 0:
	case reconnected:
		d.env.Log.WarnContext(ctx, "torrents were ignored because they did not have a hash or title; Deluge may have disconnected from its daemon. If you continue to see this error, check Deluge for invalid torrents",
			"count", ignored)
	default:
		if err := d.proxy.reconnect(ctx); err != nil {
			d.env.Log.WarnContext(ctx, "reconnect Deluge to its daemon", "err", err)
		}
	}
}

// category returns the category of a kind; for zero, the first set.
func (d *deluge) category(k client.Kind) string {
	if k == 0 && d.settings.MovieCategory == "" {
		return d.settings.TVCategory
	}
	return d.settings.Category(k)
}

func (d *deluge) item(t *torrent, kind client.Kind, opts client.Options) client.Item {
	ratio := t.Ratio
	eta := etaDuration(t.ETA)
	item := client.Item{
		ID: strings.ToUpper(t.Hash), Title: t.Name, Category: d.category(kind), Kind: kind,
		OutputPath: client.JoinRemote(t.SavePath, t.Name), TotalSize: t.TotalSize, RemainingSize: t.TotalSize - t.TotalDone,
		SeedRatio: &ratio, RemainingTime: &eta,
		HasPostImportCategory: d.settings.ImportedCategory(kind) != "" ||
			kind == 0 && d.settings.TVImportedCategory != "",
	}
	switch {
	case t.State == stateError:
		item.Status = client.Warning
		item.Message = "Deluge is reporting an error"
	case t.IsFinished && t.State != stateChecking:
		item.Status = client.Completed
	case t.State == stateQueued:
		item.Status = client.Queued
	case t.State == statePaused:
		item.Status = client.Paused
	default:
		item.Status = client.Downloading
	}
	// Deluge manages the torrent and is done seeding it.
	item.CanBeRemoved = opts.RemoveCompletedDownloads && t.IsAutoManaged && t.StopAtRatio &&
		t.Ratio >= t.StopRatio && t.State == statePaused
	item.CanMoveFiles = item.CanBeRemoved
	return item
}

// etaDuration converts an ETA in seconds, the longest duration where it
// overflows, as Radarr's TimeSpan.MaxValue.
func etaDuration(eta float64) time.Duration {
	if s := eta * float64(time.Second); s >= math.MaxInt64 {
		return time.Duration(math.MaxInt64)
	} else if s <= math.MinInt64 {
		return time.Duration(math.MinInt64)
	}
	return time.Duration(eta * float64(time.Second))
}

// Remove implements client.Client.
func (d *deluge) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return d.proxy.remove(ctx, strings.ToLower(item.ID), deleteData)
}

// Info implements client.Client: where each kind's torrents end up, by
// the settings' directories, the label's move-completed folder, or
// Deluge's own folders.
func (d *deluge) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: d.settings.IsLocalhost()}
	config := sync.OnceValues(func() (map[string]any, error) { return d.proxy.config(ctx) })
	for _, k := range client.Kinds {
		dir := d.settings.completedDirectory(k)
		if strings.TrimSpace(dir) == "" {
			dir = d.settings.Directory(k)
		}
		if strings.TrimSpace(dir) == "" {
			if label := d.settings.Category(k); label != "" {
				o, err := d.proxy.labelOptions(ctx, label)
				if err != nil {
					return info, err
				}
				if o.ApplyMoveCompleted && o.MoveCompleted {
					dir = o.MoveCompletedPath
				}
			}
		}
		if strings.TrimSpace(dir) == "" {
			c, err := config()
			if err != nil {
				return info, err
			}
			if c["move_completed"] == true {
				dir, _ = c["move_completed_path"].(string)
			} else {
				dir, _ = c["download_location"].(string)
			}
		}
		if strings.TrimSpace(dir) != "" && !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Add implements client.Client.
func (d *deluge) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, d.env.Log, d, r)
}

func (d *deluge) addOptions(k client.Kind) map[string]any {
	return addOptions(d.settings.AddPaused, d.settings.Directory(k), d.settings.completedDirectory(k))
}

// AddMagnet implements client.TorrentAdder.
func (d *deluge) AddMagnet(ctx context.Context, r *client.Release, _, link string) (string, error) {
	hash, err := d.proxy.addMagnet(ctx, link, d.addOptions(r.Kind))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(hash) == "" {
		return "", fmt.Errorf("deluge failed to add magnet %s", link)
	}
	return d.added(ctx, r, hash)
}

// AddTorrentFile implements client.TorrentAdder.
func (d *deluge) AddTorrentFile(ctx context.Context, r *client.Release, _, fileName string, data []byte) (string, error) {
	hash, err := d.proxy.addFile(ctx, fileName, data, d.addOptions(r.Kind))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(hash) == "" {
		return "", fmt.Errorf("deluge failed to add torrent %s", fileName)
	}
	return d.added(ctx, r, hash)
}

// added sets an added torrent's seeding, label and place in the queue.
func (d *deluge) added(ctx context.Context, r *client.Release, hash string) (string, error) {
	if err := d.proxy.setSeeding(ctx, hash, r); err != nil {
		return "", err
	}
	if label := d.settings.Category(r.Kind); label != "" {
		if err := d.proxy.setLabel(ctx, hash, label); err != nil {
			return "", err
		}
	}
	if d.settings.Priority(r.Kind, r.Recent) == priorityFirst {
		if err := d.proxy.queueTop(ctx, hash); err != nil {
			return "", err
		}
	}
	return strings.ToUpper(hash), nil
}

// MarkImported implements client.Marker: the torrent's label becomes the
// post-import category.
func (d *deluge) MarkImported(ctx context.Context, item client.Item) error {
	kinds := client.Kinds
	if item.Kind != 0 {
		kinds = []client.Kind{item.Kind}
	}
	for _, k := range kinds {
		if !d.settings.HasPostImportCategory(k) {
			continue
		}
		imported := d.settings.ImportedCategory(k)
		err := d.proxy.setLabel(ctx, strings.ToLower(item.ID), imported)
		var unavailable *client.UnavailableError
		if errors.As(err, &unavailable) {
			d.env.Log.WarnContext(ctx, "failed to set the post-import torrent label in Deluge; does the label exist?",
				"label", imported, "download", item.Title, "err", err)
			return nil
		}
		return err
	}
	return nil
}

// Test implements client.Client.
func (d *deluge) Test(ctx context.Context) error {
	if err := d.testConnection(ctx); err != nil {
		return err
	}
	if err := d.testCategories(ctx); err != nil {
		return err
	}
	if _, err := d.proxy.torrents(ctx, ""); err != nil {
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return nil
}

func (d *deluge) testConnection(ctx context.Context) error {
	_, err := d.proxy.version(ctx)
	var auth *client.AuthError
	var unavailable *client.UnavailableError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &auth):
		return fmt.Errorf("authentication failure: %w", err)
	case errors.As(err, &unavailable):
		return fmt.Errorf("unable to connect to Deluge, please verify the hostname and port: %w", err)
	}
	return fmt.Errorf("unable to connect to Deluge: %w", err)
}

// testCategories checks that the Label plugin is on and creates the
// labels of the categories that are missing.
func (d *deluge) testCategories(ctx context.Context) error {
	var wanted []string
	for _, c := range []string{d.settings.MovieCategory, d.settings.MovieImportedCategory, d.settings.TVCategory, d.settings.TVImportedCategory} {
		if c != "" && !slices.Contains(wanted, c) {
			wanted = append(wanted, c)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	methods, err := d.proxy.methods(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(methods, func(m string) bool { return strings.HasPrefix(m, "label.") }) {
		return errors.New("label plugin not activated: you must have the Label plugin enabled in Deluge to use categories")
	}
	labels, err := d.proxy.labels(ctx)
	if err != nil {
		return err
	}
	for _, label := range wanted {
		if slices.Contains(labels, label) {
			continue
		}
		if err := d.proxy.addLabel(ctx, label); err != nil {
			return fmt.Errorf("configuration of label failed: %w", err)
		}
		if labels, err = d.proxy.labels(ctx); err != nil {
			return err
		}
		if !slices.Contains(labels, label) {
			return fmt.Errorf("configuration of label failed: Mavio was unable to add the label %q to Deluge", label)
		}
	}
	return nil
}

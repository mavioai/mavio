package qbittorrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of qBittorrent.
type settings struct {
	client.Server
	// APIKey signs in to the Web API v2 instead of the username and
	// password.
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Priorities
	InitialState    int  `json:"initial_state"`
	SequentialOrder bool `json:"sequential_order"`
	FirstAndLast    bool `json:"first_and_last"`
	ContentLayout   int  `json:"content_layout"`
}

// The priorities: last leaves torrents at the end of the queue, first
// moves them to its top.
const (
	priorityLast  = 0
	priorityFirst = 1
)

// The initial states of added torrents.
const (
	stateStart      = 0
	stateForceStart = 1
	stateStop       = 2
)

// The content layouts of added torrents: qBittorrent's default, the
// torrent's own, or always a subfolder.
const (
	layoutDefault   = 0
	layoutOriginal  = 1
	layoutSubfolder = 2
)

var (
	categoryPattern = regexp.MustCompile(`^([^\\/](/?[^\\/])*)?$`)
	urlBasePattern  = regexp.MustCompile(`(?i)^/?https?://[-_a-z0-9.]+`)
)

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if !blank(s.URLBase) && urlBasePattern.MatchString(s.URLBase) {
		return errors.New("url base: must be a valid URL path (ie: '/qbittorrent')")
	}
	if !blank(s.APIKey) {
		switch {
		case s.Username != "":
			return errors.New("username must be empty when using API key")
		case s.Password != "":
			return errors.New("password must be empty when using API key")
		}
	}
	for _, c := range []string{s.MovieCategory, s.MovieImportedCategory, s.TVCategory, s.TVImportedCategory} {
		if !categoryPattern.MatchString(c) {
			return fmt.Errorf(`category %q: can not contain '\', '//', or start/end with '/'`, c)
		}
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(8080, "")
	f[2].HelpText = "Use a secure connection. See Options -> Web UI -> 'Use HTTPS instead of HTTP' in qBittorrent."
	f[3].HelpText = "Adds a prefix to the qBittorrent url, such as http://[host]:[port]/[urlBase]/api"
	f = append(f,
		client.Field{Name: "api_key", Label: "API Key", Type: client.FieldText, Secret: true},
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, true)...)
	f = append(f, client.PriorityFields([]client.Option{{Value: priorityLast, Label: "Last"}, {Value: priorityFirst, Label: "First"}}, priorityLast)...)
	return append(f,
		client.Field{
			Name: "initial_state", Label: "Initial State", Type: client.FieldSelect, Default: stateStart,
			Options: []client.Option{
				{Value: stateStart, Label: "Started"}, {Value: stateForceStart, Label: "Force Started"}, {Value: stateStop, Label: "Stopped"},
			},
			HelpText: "Initial state for torrents added to qBittorrent. Note that Forced Torrents do not abide by seed restrictions",
		},
		client.Field{
			Name: "sequential_order", Label: "Sequential Order", Type: client.FieldCheckbox, Default: false,
			HelpText: "Download in sequential order (qBittorrent 4.1.0+)",
		},
		client.Field{
			Name: "first_and_last", Label: "First and Last First", Type: client.FieldCheckbox, Default: false,
			HelpText: "Download first and last pieces first (qBittorrent 4.1.0+)",
		},
		client.Field{
			Name: "content_layout", Label: "Content Layout", Type: client.FieldSelect, Default: layoutDefault,
			Options: []client.Option{
				{Value: layoutDefault, Label: "Default"}, {Value: layoutOriginal, Label: "Original"}, {Value: layoutSubfolder, Label: "Subfolder"},
			},
			HelpText: "Whether to use qBittorrent's configured content layout, the original layout from the torrent or always create a subfolder (qBittorrent 4.3.2+)",
		},
	)
}

// Definitions returns the definition of qBittorrent.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "qbittorrent", Name: "qBittorrent", Protocol: client.Torrent, Fields: fields(),
		Description: "qBittorrent, over its Web API",
		New:         newQBittorrent,
	}}
}

func newQBittorrent(raw []byte, env client.Env) (client.Client, error) {
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
	q := &qbittorrent{settings: s, env: env, seeding: map[string]*seedingTime{}}
	hc := client.NoRedirect(env.HTTP)
	q.selector = &selector{
		apis: []api{v2{&conn{http: hc, settings: &q.settings, v2: true}}, v1{&conn{http: hc, settings: &q.settings}}},
		now:  env.Now,
	}
	return q, nil
}

// qbittorrent is the client of Radarr's QBittorrent.
type qbittorrent struct {
	settings settings
	env      client.Env
	selector *selector

	mu sync.Mutex
	// seeding are the seeding times last fetched, by hash, kept 5 minutes.
	seeding map[string]*seedingTime
}

// seedingTime is a torrent's seeding time when it was fetched.
type seedingTime struct {
	fetched time.Time
	seconds int64
	expires time.Time
}

// Host implements client.Client.
func (q *qbittorrent) Host() string { return q.settings.Host }

// atLeast reports whether version v is at least min, as .NET compares
// versions.
func atLeast(v []int, minimum ...int) bool { return slices.Compare(v, minimum) >= 0 }

// orMovie is the kind whose settings serve a download of both kinds.
func orMovie(k client.Kind) client.Kind {
	if k == 0 {
		return client.Movie
	}
	return k
}

// torrents lists the torrents of the kinds' categories, as Radarr's and
// Sonarr's GetTorrents ask for: all of them when a kind has none.
func (q *qbittorrent) torrents(ctx context.Context, p api) ([]torrent, error) {
	var categories []string
	for _, k := range client.Kinds {
		c := q.settings.Category(k)
		if blank(c) {
			categories = []string{""}
			break
		}
		if !slices.Contains(categories, c) {
			categories = append(categories, c)
		}
	}
	var out []torrent
	for _, c := range categories {
		ts, err := p.torrents(ctx, c)
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			if !slices.ContainsFunc(out, func(o torrent) bool { return strings.EqualFold(o.Hash, t.Hash) }) {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// Items implements client.Client.
func (q *qbittorrent) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	p, _, err := q.selector.get(ctx, false)
	if err != nil {
		return nil, err
	}
	version, err := p.apiVersion(ctx)
	if err != nil {
		return nil, err
	}
	config, err := p.config(ctx)
	if err != nil {
		return nil, err
	}
	torrents, err := q.torrents(ctx, p)
	if err != nil {
		return nil, err
	}
	q.pruneSeeding()
	var items []client.Item
	for i := range torrents {
		t := &torrents[i]
		category := t.Category
		if blank(category) {
			category = t.Label
		}
		kind, ok := q.settings.Match(func(c string) bool { return blank(c) || c == category })
		if !ok {
			continue
		}
		ratio := t.Ratio
		item := client.Item{
			ID: strings.ToUpper(t.Hash), Title: t.Name, Category: category, Kind: kind, TotalSize: t.Size,
			RemainingSize: int64(float64(t.Size) * (1.0 - t.Progress)), RemainingTime: remainingTime(t.ETA),
			SeedRatio:             &ratio,
			HasPostImportCategory: !blank(q.settings.ImportedCategory(orMovie(kind))),
		}
		// Torrents are removed once paused at their limits, which qBittorrent
		// may have set higher on the torrent than the API tells.
		if opts.RemoveCompletedDownloads && (t.State == "pausedUP" || t.State == "stoppedUP") {
			reached, err := q.hasReachedSeedLimit(ctx, p, t, config)
			if err != nil {
				return nil, err
			}
			item.CanMoveFiles, item.CanBeRemoved = reached, reached
		}
		switch t.State {
		case "error":
			item.Status, item.Message = client.Warning, "qBittorrent is reporting an error"
		case "stoppedDL", "pausedDL":
			item.Status = client.Paused
		case "queuedDL", "checkingDL", "checkingUP", "checkingResumeData":
			item.Status = client.Queued
		case "pausedUP", "stoppedUP", "uploading", "stalledUP", "queuedUP", "forcedUP":
			// qBittorrent sends an ETA of 8640000 for completed torrents.
			zero := time.Duration(0)
			item.Status, item.RemainingTime = client.Completed, &zero
		case "stalledDL":
			item.Status, item.Message = client.Warning, "The download is stalled with no connections"
		case "missingFiles":
			item.Status, item.Message = client.Warning, "qBittorrent is reporting missing files"
		case "metaDL", "forcedMetaDL":
			if config.DHT {
				item.Status, item.Message = client.Queued, "qBittorrent is downloading metadata"
			} else {
				item.Status, item.Message = client.Warning, "qBittorrent cannot resolve magnet link with DHT disabled"
			}
		case "forcedDL", "moving", "downloading":
			item.Status = client.Downloading
		default:
			q.env.Log.InfoContext(ctx, "unknown qBittorrent download state", "state", t.State)
			item.Status, item.Message = client.Downloading, "Unknown download state: "+t.State
		}
		if atLeast(version, 2, 6, 1) && item.Status == client.Completed {
			if !samePath(t.ContentPath, t.SavePath) {
				item.OutputPath = t.ContentPath
			} else {
				item.Status = client.Warning
				item.Message = "Unable to Import. Path matches client base download directory, it's possible 'Keep top-level folder' is disabled for this torrent or 'Torrent Content Layout' is NOT set to 'Original' or 'Create Subfolder'?"
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// remainingTime converts an ETA, nil when unknown: negative, over a
// year, or qBittorrent's 8640000.
func remainingTime(eta json.Number) *time.Duration {
	n, err := strconv.ParseInt(eta.String(), 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(eta.String(), 64)
		if ferr != nil || f < 0 || f > 365*24*3600 {
			return nil
		}
		n = int64(f)
	}
	if n < 0 || n > 365*24*3600 || n == 8640000 {
		return nil
	}
	d := time.Duration(n) * time.Second
	return &d
}

// hasReachedSeedLimit reports whether a torrent is done seeding, by its
// own limits or, where they are the global ones, qBittorrent's.
func (q *qbittorrent) hasReachedSeedLimit(ctx context.Context, p api, t *torrent, config *preferences) (bool, error) {
	switch {
	case t.RatioLimit >= 0:
		if t.RatioLimit-t.Ratio <= 0.001 {
			return true, nil
		}
	case t.RatioLimit == -2 && config.MaxRatioEnabled:
		if config.MaxRatio-t.Ratio <= 0.001 {
			return true, nil
		}
	}
	reached, err := q.hasReachedSeedingTimeLimit(ctx, p, t, config)
	if err != nil || reached {
		return reached, err
	}
	return q.hasReachedInactiveSeedingTimeLimit(t, config), nil
}

// hasReachedSeedingTimeLimit reports whether a torrent has seeded long
// enough. The list leaves out seeding times, which are fetched at most
// once until they may have reached the limit, as Radarr's seedingTime
// cache.
func (q *qbittorrent) hasReachedSeedingTimeLimit(ctx context.Context, p api, t *torrent, config *preferences) (bool, error) {
	var limit int64
	switch {
	case t.SeedingTimeLimit >= 0:
		limit = t.SeedingTimeLimit * 60
	case t.SeedingTimeLimit == -2 && config.MaxSeedingTimeEnabled:
		limit = config.MaxSeedingTime * 60
	default:
		return false, nil
	}
	if t.SeedingTime != nil {
		return *t.SeedingTime >= limit, nil
	}
	now := q.env.Now()
	q.mu.Lock()
	if e := q.seeding[t.Hash]; e != nil && now.Before(e.expires) {
		togo, elapsed := limit-e.seconds, now.Sub(e.fetched).Seconds()
		switch {
		case togo <= 0:
			e.expires = now.Add(5 * time.Minute)
			q.mu.Unlock()
			return true, nil
		case float64(togo) > elapsed:
			// The seeding time cannot have reached the limit since.
			e.expires = now.Add(5 * time.Minute)
			q.mu.Unlock()
			return false, nil
		}
	}
	q.mu.Unlock()
	props, err := p.properties(ctx, t.Hash)
	if err != nil {
		return false, err
	}
	q.mu.Lock()
	q.seeding[t.Hash] = &seedingTime{fetched: now, seconds: props.SeedingTime, expires: now.Add(5 * time.Minute)}
	q.mu.Unlock()
	return props.SeedingTime >= limit, nil
}

// pruneSeeding forgets the seeding times expired.
func (q *qbittorrent) pruneSeeding() {
	now := q.env.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	for h, e := range q.seeding {
		if !now.Before(e.expires) {
			delete(q.seeding, h)
		}
	}
}

// hasReachedInactiveSeedingTimeLimit reports whether a torrent has been
// inactive long enough.
func (q *qbittorrent) hasReachedInactiveSeedingTimeLimit(t *torrent, config *preferences) bool {
	var limit int64
	switch {
	case t.InactiveSeedingTimeLimit >= 0:
		limit = t.InactiveSeedingTimeLimit * 60
	case t.InactiveSeedingTimeLimit == -2 && config.MaxInactiveSeedingTimeEnabled:
		limit = config.MaxInactiveSeedingTime * 60
	default:
		return false
	}
	return q.env.Now().Unix()-t.LastActivity > limit
}

// Remove implements client.Client.
func (q *qbittorrent) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	p, _, err := q.selector.get(ctx, false)
	if err != nil {
		return err
	}
	return p.remove(ctx, strings.ToLower(item.ID), deleteData)
}

// ImportItem implements client.Importer: before API 2.6.1 the list has no
// content path, which is then the save path joined with the first
// folder, or the file, of the torrent's first file.
func (q *qbittorrent) ImportItem(ctx context.Context, item client.Item, _ client.Options) (client.Item, error) {
	if item.OutputPath != "" {
		return item, nil
	}
	p, _, err := q.selector.get(ctx, false)
	if err != nil {
		return item, err
	}
	hash := strings.ToLower(item.ID)
	files, err := p.files(ctx, hash)
	if err != nil {
		return item, err
	}
	if len(files) == 0 {
		q.env.Log.DebugContext(ctx, "no files found for torrent in qBittorrent", "download", item.Title)
		return item, nil
	}
	props, err := p.properties(ctx, hash)
	if err != nil {
		return item, err
	}
	first := strings.TrimLeft(files[0].Name, `/\`)
	if i := strings.IndexAny(first, `/\`); i >= 0 {
		first = first[:i]
	}
	item.OutputPath = joinPath(props.SavePath, first)
	return item, nil
}

// MarkImported implements client.Marker: the torrent moves to the
// post-import category.
func (q *qbittorrent) MarkImported(ctx context.Context, item client.Item) error {
	kind := orMovie(item.Kind)
	imported := q.settings.ImportedCategory(kind)
	if blank(imported) || imported == q.settings.Category(kind) {
		return nil
	}
	p, _, err := q.selector.get(ctx, false)
	if err != nil {
		return err
	}
	if err := p.setCategory(ctx, strings.ToLower(item.ID), imported); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		q.env.Log.WarnContext(ctx, "failed to set the post-import torrent category in qBittorrent; does the category exist?",
			"category", imported, "download", item.Title, "err", err)
	}
	return nil
}

// removesCompletedDownloads reports whether qBittorrent removes torrents
// at their share limits, sooner than 14 days for seeding times.
func removesCompletedDownloads(c *preferences) bool {
	const minimumRetention = 60 * 24 * 14
	return (c.MaxRatioEnabled || (c.MaxSeedingTimeEnabled && c.MaxSeedingTime < minimumRetention)) &&
		(c.MaxRatioAction == ratioActionRemove || c.MaxRatioAction == ratioActionDeleteFiles)
}

// Info implements client.Client: each kind's folder is qBittorrent's save
// path, or its category's.
func (q *qbittorrent) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: q.settings.IsLocalhost()}
	p, _, err := q.selector.get(ctx, false)
	if err != nil {
		return info, err
	}
	version, err := p.apiVersion(ctx)
	if err != nil {
		return info, err
	}
	config, err := p.config(ctx)
	if err != nil {
		return info, err
	}
	info.RemovesCompletedDownloads = removesCompletedDownloads(config)
	var categories map[string]category
	for _, k := range client.Kinds {
		dir := config.SavePath
		if c := q.settings.Category(k); !blank(c) && atLeast(version, 2, 0) {
			if categories == nil {
				if categories, err = p.categories(ctx); err != nil {
					return info, err
				}
			}
			if l, ok := categories[c]; ok && !blank(l.SavePath) {
				savePath := l.SavePath
				if strings.HasPrefix(savePath, "//") {
					q.env.Log.DebugContext(ctx, "replacing double forward slashes in a category's save path; if this is not meant to be a Windows UNC path fix the 'Save Path' of the qBittorrent category",
						"path", savePath, "category", c)
					savePath = strings.ReplaceAll(savePath, "/", `\`)
				}
				if isRooted(savePath) {
					dir = savePath
				} else {
					dir = joinPath(dir, savePath)
				}
			}
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Add implements client.Client.
func (q *qbittorrent) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, q.env.Log, q, r)
}

// AddMagnet implements client.TorrentAdder; without DHT qBittorrent needs
// the link's trackers.
func (q *qbittorrent) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	p, version, err := q.selector.get(ctx, false)
	if err != nil {
		return "", err
	}
	config, err := p.config(ctx)
	if err != nil {
		return "", err
	}
	if !config.DHT && !strings.Contains(link, "&tr=") {
		return "", fmt.Errorf("magnet links without trackers are not supported if DHT is disabled: %w", client.ErrMagnetNotSupported)
	}
	return q.add(ctx, p, version, r, hash, func(seed *client.Release) error {
		return p.addURL(ctx, link, q.settings.Category(r.Kind), seed)
	})
}

// AddTorrentFile implements client.TorrentAdder.
func (q *qbittorrent) AddTorrentFile(ctx context.Context, r *client.Release, hash, fileName string, data []byte) (string, error) {
	p, version, err := q.selector.get(ctx, false)
	if err != nil {
		return "", err
	}
	return q.add(ctx, p, version, r, hash, func(seed *client.Release) error {
		return p.addFile(ctx, fileName, data, q.settings.Category(r.Kind), seed)
	})
}

// add adds a torrent, with its seeding limits from API 2.8.1, and then
// sets what adding it could not: the seeding limits before, the top of
// the queue and force start. Failures of these are logged.
func (q *qbittorrent) add(ctx context.Context, p api, version []int, r *client.Release, hash string, add func(seed *client.Release) error) (string, error) {
	setShareLimits := r.SeedRatio != nil || r.SeedTime != nil
	addSetsShareLimits := setShareLimits && atLeast(version, 2, 8, 1)
	moveToTop := q.settings.Priority(r.Kind, r.Recent) == priorityFirst
	forceStart := q.settings.InitialState == stateForceStart
	var seed *client.Release
	if addSetsShareLimits {
		seed = r
	}
	if err := add(seed); err != nil {
		return "", err
	}
	if (addSetsShareLimits || !setShareLimits) && !moveToTop && !forceStart {
		return hash, nil
	}
	loaded, err := q.waitForTorrent(ctx, p, hash)
	if err != nil || !loaded {
		return hash, err
	}
	lower := strings.ToLower(hash)
	if !addSetsShareLimits && setShareLimits {
		if err := p.setShareLimits(ctx, lower, r); err != nil {
			q.env.Log.WarnContext(ctx, "failed to set the torrent seed criteria", "hash", hash, "err", err)
		}
	}
	if moveToTop {
		if err := p.moveToTop(ctx, lower); err != nil {
			q.env.Log.WarnContext(ctx, "failed to set the torrent priority", "hash", hash, "err", err)
		}
	}
	if forceStart {
		if err := p.setForceStart(ctx, lower, true); err != nil {
			q.env.Log.WarnContext(ctx, "failed to set force start", "hash", hash, "err", err)
		}
	}
	return hash, ctx.Err()
}

// waitForTorrent waits up to a second for qBittorrent to show an added
// torrent.
func (q *qbittorrent) waitForTorrent(ctx context.Context, p api, hash string) (bool, error) {
	for range 10 {
		if p.loaded(ctx, strings.ToLower(hash)) {
			return true, nil
		}
		q.env.Log.DebugContext(ctx, "torrent not yet visible in qBittorrent, waiting 100ms", "hash", hash)
		t := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return false, ctx.Err()
		case <-t.C:
		}
	}
	q.env.Log.WarnContext(ctx, "failed to load the torrent in time, skipping additional parameters", "hash", hash)
	return false, nil
}

// Test implements client.Client.
func (q *qbittorrent) Test(ctx context.Context) error {
	p, err := q.testConnection(ctx)
	if err != nil {
		return err
	}
	if err := q.testCategories(ctx, p); err != nil {
		return err
	}
	if err := q.testPrioritySupport(ctx, p); err != nil {
		return err
	}
	if _, err := q.torrents(ctx, p); err != nil {
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return nil
}

// testConnection selects the API again and checks its version, the
// categories it supports and that qBittorrent does not remove torrents.
func (q *qbittorrent) testConnection(ctx context.Context) (api, error) {
	wrap := func(err error) error {
		var auth *client.AuthError
		var unavailable *client.UnavailableError
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.As(err, &auth):
			return fmt.Errorf("authentication failure; please verify your username and password, or API key: %w", err)
		case errors.As(err, &unavailable):
			return fmt.Errorf("unable to connect to qBittorrent; please verify the hostname and port: %w", err)
		}
		return fmt.Errorf("unable to connect to qBittorrent: %w", err)
	}
	p, _, err := q.selector.get(ctx, true)
	if err != nil {
		return nil, wrap(err)
	}
	version, err := p.apiVersion(ctx)
	if err != nil {
		return nil, wrap(err)
	}
	switch {
	case !atLeast(version, 1, 5):
		// API 5 brought the save path of torrents.
		return nil, fmt.Errorf("qBittorrent version should be at least 3.2.4. Version reported is %s", formatVersion(version))
	case !atLeast(version, 1, 6):
		// API 6 brought labels.
		for _, k := range client.Kinds {
			if !blank(q.settings.Category(k)) {
				return nil, errors.New("category is not supported: categories are not supported until qBittorrent version 3.3.0. Please upgrade or try again with an empty category")
			}
		}
	default:
		for _, k := range client.Kinds {
			if blank(q.settings.Category(k)) {
				q.env.Log.WarnContext(ctx, "a category is recommended: completed downloads without a category are not imported", "kind", k)
			}
		}
	}
	config, err := p.config(ctx)
	if err != nil {
		return nil, wrap(err)
	}
	if removesCompletedDownloads(config) {
		return nil, errors.New("qBittorrent is configured to remove torrents when they reach their Share Ratio Limit; completed download handling will not work as configured. Change 'Options -> BitTorrent -> Share Ratio Limiting' from 'Remove them' to 'Pause them' in qBittorrent")
	}
	return p, nil
}

// testCategories creates the categories qBittorrent lacks, from API 2.0;
// before, setting a category creates it.
func (q *qbittorrent) testCategories(ctx context.Context, p api) error {
	var names []string
	for _, k := range client.Kinds {
		for _, c := range []string{q.settings.Category(k), q.settings.ImportedCategory(k)} {
			if !blank(c) && !slices.Contains(names, c) {
				names = append(names, c)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}
	version, err := p.apiVersion(ctx)
	if err != nil {
		return err
	}
	if !atLeast(version, 2, 0) {
		return nil
	}
	categories, err := p.categories(ctx)
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, ok := categories[name]; ok {
			continue
		}
		if err := p.addCategory(ctx, name); err != nil {
			return err
		}
		if categories, err = p.categories(ctx); err != nil {
			return err
		}
		if _, ok := categories[name]; !ok {
			return fmt.Errorf("configuration of category %q failed: Mavio was unable to add the category to qBittorrent", name)
		}
	}
	return nil
}

// testPrioritySupport checks that queueing is on where torrents are to
// move to the top of the queue.
func (q *qbittorrent) testPrioritySupport(ctx context.Context, p api) error {
	if !slices.ContainsFunc([]int{
		q.settings.RecentMoviePriority, q.settings.OlderMoviePriority, q.settings.RecentTVPriority, q.settings.OlderTVPriority,
	}, func(v int) bool { return v != priorityLast }) {
		return nil
	}
	config, err := p.config(ctx)
	if err != nil {
		return fmt.Errorf("unknown exception: %w", err)
	}
	if !config.QueueingEnabled {
		return errors.New("queueing not enabled: Torrent Queueing is not enabled in your qBittorrent settings. Enable it in qBittorrent or select 'Last' as priority")
	}
	return nil
}

// joinPath joins a folder of the client's machine, with either slashes,
// and a name.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	sep := "/"
	if strings.Contains(dir, `\`) && !strings.Contains(dir, "/") {
		sep = `\`
	}
	return strings.TrimRight(dir, `/\`) + sep + name
}

// isRooted reports whether a path of the client's machine is absolute:
// a Unix path, a UNC path or one with a drive letter.
func isRooted(p string) bool {
	return strings.HasPrefix(p, "/") || strings.HasPrefix(p, `\\`) ||
		len(p) >= 2 && p[1] == ':' && ('a' <= p[0]|0x20 && p[0]|0x20 <= 'z')
}

// samePath reports whether two paths of the client's machine are one, as
// Radarr's OsPath equality: trailing separators aside, and Windows paths
// in any case.
func samePath(a, b string) bool {
	a, b = strings.TrimRight(a, `/\`), strings.TrimRight(b, `/\`)
	if strings.Contains(a, `\`) || isRooted(a) && !strings.HasPrefix(a, "/") {
		return strings.EqualFold(strings.ReplaceAll(a, "/", `\`), strings.ReplaceAll(b, "/", `\`))
	}
	return a == b
}

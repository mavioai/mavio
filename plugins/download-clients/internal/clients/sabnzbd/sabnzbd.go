package sabnzbd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of SABnzbd.
type settings struct {
	client.Server
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Priorities
}

// validate checks the settings as Radarr's SabnzbdSettingsValidator.
func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := client.ValidateURLBase("url base", s.URLBase, "/sabnzbd"); err != nil {
		return err
	}
	noKey := strings.TrimSpace(s.APIKey) == ""
	switch {
	case noKey && strings.TrimSpace(s.Username) == "":
		return errors.New("API Key is required when username/password are not configured")
	case noKey && s.Password == "":
		return errors.New("password is required when API key is not configured")
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(8080, "")
	f[len(f)-1].HelpText = "Adds a prefix to the SABnzbd url, such as http://[host]:[port]/[urlBase]/api"
	f = append(f,
		client.Field{Name: "api_key", Label: "API Key", Type: client.FieldText, Secret: true},
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, false)...)
	options := []client.Option{
		{Value: int(priorityDefault), Label: "Default"},
		{Value: int(priorityPaused), Label: "Paused"},
		{Value: int(priorityLow), Label: "Low"},
		{Value: int(priorityNormal), Label: "Normal"},
		{Value: int(priorityHigh), Label: "High"},
		{Value: int(priorityForce), Label: "Force"},
	}
	return append(f, client.PriorityFields(options, int(priorityDefault))...)
}

// Definitions returns the definition of SABnzbd.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "sabnzbd", Name: "SABnzbd", Protocol: client.Usenet, Fields: fields(),
		Description: "SABnzbd, over its API",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			p := int(priorityDefault)
			s := settings{
				Server:     client.Server{Host: "localhost", Port: 8080},
				Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
				Priorities: client.Priorities{RecentMoviePriority: p, OlderMoviePriority: p, RecentTVPriority: p, OlderTVPriority: p},
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.validate(); err != nil {
				return nil, err
			}
			c := &sabnzbd{settings: s, env: env}
			c.proxy = &proxy{http: env.HTTP, settings: &c.settings}
			return c, nil
		},
	}}
}

// sabnzbd is the client of Radarr's Sabnzbd.
type sabnzbd struct {
	settings settings
	proxy    *proxy
	env      client.Env
}

// Host implements client.Client.
func (c *sabnzbd) Host() string { return c.settings.Host }

// Add implements client.Client: the NZB goes to the category of the
// release's kind with its priority.
func (c *sabnzbd) Add(ctx context.Context, r *client.Release) (string, error) {
	if len(r.Data) == 0 {
		return "", errors.New("the release has no NZB")
	}
	name := r.FileName
	if name == "" {
		name = client.CleanFileName(r.Title) + ".nzb"
	}
	ids, err := c.proxy.addFile(ctx, r.Data, name, c.settings.Category(r.Kind), c.settings.Priority(r.Kind, r.Recent))
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", client.Rejected("SABnzbd rejected the NZB for an unknown reason")
	}
	return ids[0], nil
}

// queryCategories are the categories the queue and history are asked
// for: each kind's, or all at once when a kind has none.
func (c *sabnzbd) queryCategories() []string {
	var out []string
	for _, k := range client.Kinds {
		cat := c.settings.Category(k)
		if strings.TrimSpace(cat) == "" {
			return []string{""}
		}
		if !slices.Contains(out, cat) {
			out = append(out, cat)
		}
	}
	return out
}

// kindOf returns the kind whose category a download is in, as Radarr's
// and Sonarr's GetItems filter them: the category itself, or SABnzbd's
// "*" for none.
func (c *sabnzbd) kindOf(category string) (client.Kind, bool) {
	return c.settings.Match(func(setting string) bool {
		return category == setting || (category == "*" && strings.TrimSpace(setting) == "")
	})
}

// queueItems lists the queue, as Radarr's GetQueue.
func (c *sabnzbd) queueItems(ctx context.Context) ([]client.Item, error) {
	var items []client.Item
	for _, cat := range c.queryCategories() {
		q, err := c.proxy.queue(ctx, 0, 0, cat)
		if err != nil {
			return nil, err
		}
		for _, s := range q.Items {
			if s.Status == statusDeleted {
				continue
			}
			remaining := time.Duration(s.TimeLeft)
			item := client.Item{
				ID: s.ID, Category: s.Category, Title: s.Title,
				TotalSize: int64(s.Size * 1024 * 1024), RemainingSize: int64(s.SizeLeft * 1024 * 1024),
				RemainingTime: &remaining,
				CanBeRemoved:  true, CanMoveFiles: true,
			}
			switch {
			case (bool(q.Paused) && s.Priority != priorityForce) || s.Status == statusPaused:
				item.Status = client.Paused
				item.RemainingTime = nil
			case s.Status == statusQueued || s.Status == statusGrabbing || s.Status == statusPropagating:
				item.Status = client.Queued
			default:
				item.Status = client.Downloading
			}
			if t, ok := strings.CutPrefix(item.Title, "ENCRYPTED /"); ok {
				item.Title, item.Encrypted = t, true
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// unpackDiskFull is the failure SABnzbd reports when unpacking runs out of
// space, a warning rather than a failure.
const unpackDiskFull = "Unpacking failed, write error or disk is full?"

// historyItems lists the history, as Radarr's GetHistory.
func (c *sabnzbd) historyItems(ctx context.Context, limit int) ([]client.Item, error) {
	var items []client.Item
	for _, cat := range c.queryCategories() {
		h, err := c.proxy.history(ctx, 0, limit, cat)
		if err != nil {
			return nil, err
		}
		for _, s := range h.Items {
			if s.Status == statusDeleted {
				continue
			}
			var zero time.Duration
			item := client.Item{
				ID: s.ID, Category: s.Category, Title: s.Title,
				TotalSize: int64(s.Size), RemainingTime: &zero, Message: s.FailMessage,
				OutputPath:   jobFolder(s.Storage, s.Title),
				CanBeRemoved: true, CanMoveFiles: true,
			}
			switch s.Status {
			case statusFailed:
				item.Status = client.Failed
				if strings.EqualFold(strings.TrimSpace(s.FailMessage), unpackDiskFull) {
					item.Status = client.Warning
				}
			case statusCompleted:
				item.Status = client.Completed
			default: // verifying, moving and so on
				item.Status = client.Downloading
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// Items implements client.Client.
func (c *sabnzbd) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	queued, err := c.queueItems(ctx)
	if err != nil {
		return nil, err
	}
	done, err := c.historyItems(ctx, opts.Limit())
	if err != nil {
		return nil, err
	}
	var items []client.Item
	seen := map[string]bool{}
	for _, item := range append(queued, done...) {
		kind, ok := c.kindOf(item.Category)
		if !ok || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		item.Kind = kind
		items = append(items, item)
	}
	return items, nil
}

// Remove implements client.Client: a queued download is deleted from the
// queue, else from the history, archived unless it failed.
func (c *sabnzbd) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	queued, err := c.queueItems(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(queued, func(q client.Item) bool { return q.ID == item.ID }) {
		return c.proxy.remove(ctx, "queue", item.ID, deleteData, false)
	}
	if deleteData && item.Status == client.Completed {
		client.DeleteItemData(ctx, c.env.Log, item)
	}
	return c.proxy.remove(ctx, "history", item.ID, deleteData, item.Status == client.Failed)
}

// categories returns SABnzbd's categories with their folders, as Radarr's
// GetCategories: complete_dir, resolved when relative, and each
// category's dir.
func (c *sabnzbd) categories(ctx context.Context, cfg *config) ([]category, error) {
	completeDir := cfg.Misc.CompleteDir
	if !isRooted(completeDir) {
		v2, err := c.hasVersion(ctx, 2, 0, 0)
		if err != nil {
			return nil, err
		}
		if v2 {
			if completeDir, err = c.proxy.completeDir(ctx); err != nil {
				return nil, err
			}
		} else {
			q, err := c.proxy.queue(ctx, 0, 1, c.queryCategories()[0])
			if err != nil {
				return nil, err
			}
			completeDir = combinePath(q.DefaultRootFolder, completeDir)
		}
	}
	out := make([]category, len(cfg.Categories))
	for i, cat := range cfg.Categories {
		cat.FullPath = combinePath(completeDir, strings.TrimRight(cat.Dir, "*"))
		out[i] = cat
	}
	return out, nil
}

// findCategory returns the category of a name.
func findCategory(cats []category, name string) *category {
	for i := range cats {
		if cats[i].Name == name {
			return &cats[i]
		}
	}
	return nil
}

// sortingMode returns the sorting SABnzbd applies to a category: "TV",
// "Movie", "Date" or none.
func sortingMode(cfg *config, cat string) string {
	switch m := &cfg.Misc; {
	case bool(m.EnableTVSorting) && containsCategory(m.TVCategories, cat):
		return "TV"
	case bool(m.EnableMovieSorting) && containsCategory(m.MovieCategories, cat):
		return "Movie"
	case bool(m.EnableDateSorting) && containsCategory(m.DateCategories, cat):
		return "Date"
	}
	return ""
}

// Info implements client.Client: the folders of both kinds' categories,
// SABnzbd's default category standing in for those it lacks.
func (c *sabnzbd) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: c.settings.IsLocalhost()}
	cfg, err := c.proxy.config(ctx)
	if err != nil {
		return info, err
	}
	cats, err := c.categories(ctx, cfg)
	if err != nil {
		return info, err
	}
	for _, k := range client.Kinds {
		name := c.settings.Category(k)
		cat := findCategory(cats, name)
		if cat == nil {
			cat = findCategory(cats, "*")
		}
		if cat == nil {
			continue
		}
		if info.SortingMode == "" {
			info.SortingMode = sortingMode(cfg, name)
		}
		if !slices.Contains(info.OutputRootFolders, cat.FullPath) {
			info.OutputRootFolders = append(info.OutputRootFolders, cat.FullPath)
		}
	}
	info.RemovesCompletedDownloads = removesCompletedDownloads(&cfg.Misc)
	return info, nil
}

// removesCompletedDownloads reports whether SABnzbd's history retention
// loses downloads before they are imported: kept fewer than 14 days, or
// by number.
func removesCompletedDownloads(m *misc) bool {
	switch m.HistoryRetentionOption {
	case "all":
		return false
	case "number-archive", "number-delete", "all-archive", "all-delete":
		return true
	case "days-archive", "days-delete":
		return m.HistoryRetentionNumber < 14
	}
	// Before SABnzbd 4.3.
	retention := string(m.HistoryRetention)
	if strings.TrimSpace(retention) == "" {
		return false
	}
	if days, ok := strings.CutSuffix(retention, "d"); ok {
		n, _ := strconv.Atoi(days)
		return n < 14
	}
	return retention != "0"
}

// containsCategory reports whether a list of categories holds one, as
// Radarr's ContainsCategory: an empty list holds all, and SABnzbd names no
// category "Default".
func containsCategory(categories []string, cat string) bool {
	if len(categories) == 0 {
		return true
	}
	if strings.TrimSpace(cat) == "" {
		cat = "Default"
	}
	return slices.Contains(categories, cat)
}

// The version SABnzbd reports: patch is a number for releases, or "x" for
// git.
var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+|x)`)

// parseVersion reads a version, "develop" being 3.0.0; false when it is
// none.
func parseVersion(s string) ([3]int, bool) {
	if strings.TrimSpace(s) == "" {
		return [3]int{}, false
	}
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		if strings.EqualFold(s, "develop") {
			return [3]int{3, 0, 0}, true
		}
		return [3]int{}, false
	}
	var v [3]int
	for i, p := range m[1:] {
		n, err := strconv.Atoi(strings.ReplaceAll(p, "x", "0"))
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

// hasVersion reports whether SABnzbd is major.minor.patch or later.
func (c *sabnzbd) hasVersion(ctx context.Context, major, minor, patch int) (bool, error) {
	raw, err := c.proxy.version(ctx)
	if err != nil {
		return false, err
	}
	v, ok := parseVersion(raw)
	return ok && slices.Compare(v[:], []int{major, minor, patch}) >= 0, nil
}

// Test implements client.Client as Radarr's Test: the connection and
// version, the API key, the global settings and the categories.
func (c *sabnzbd) Test(ctx context.Context) error {
	warning, err := c.testConnectionAndVersion(ctx)
	if err != nil {
		return err
	}
	cfg, err := c.proxy.config(ctx)
	if err != nil {
		if msg, ok := isAPIKeyError(err); ok {
			return &client.AuthError{Err: errors.New(msg)}
		}
		return err
	}
	if cfg.Misc.PreCheck {
		v, err := c.hasVersion(ctx, 1, 1, 0)
		if err != nil {
			return err
		}
		if !v {
			return fmt.Errorf("disable the 'Check before download' option in SABnzbd: it affects tracking new downloads, "+
				"and SABnzbd recommends 'Abort jobs that cannot be completed' instead (see %s)", c.proxy.baseURL("config/switches/"))
		}
	}
	cats, err := c.categories(ctx, cfg)
	if err != nil {
		return err
	}
	var tested []string
	for _, k := range client.Kinds {
		name := c.settings.Category(k)
		if slices.Contains(tested, name) {
			continue
		}
		tested = append(tested, name)
		if err := c.testCategory(cfg, cats, name); err != nil {
			return err
		}
	}
	if warning != nil {
		return warning
	}
	return nil
}

// testConnectionAndVersion checks the version; a develop version is a
// warning, as Radarr's IsWarning failure, which the other checks follow.
func (c *sabnzbd) testConnectionAndVersion(ctx context.Context) (*client.TestWarning, error) {
	raw, err := c.proxy.version(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to SABnzbd: %w", err)
	}
	v, ok := parseVersion(raw)
	switch {
	case !ok:
		return nil, fmt.Errorf("unknown SABnzbd version: %s", raw)
	case strings.EqualFold(raw, "develop"):
		return &client.TestWarning{Message: "SABnzbd develop version, assuming version 3.0.0 or higher; " +
			"Mavio may not be able to support new features added to SABnzbd when running develop versions"}, nil
	case v[0] >= 1 || v[1] >= 7:
		return nil, nil
	}
	return nil, fmt.Errorf("SABnzbd version should be at least 0.7.0; version reported is %d.%d.%d", v[0], v[1], v[2])
}

// testCategory checks a category as Radarr's TestCategory: that it exists
// with job folders, and that SABnzbd sorts none of its downloads.
func (c *sabnzbd) testCategory(cfg *config, cats []category, name string) error {
	categoriesURL, sortingURL := c.proxy.baseURL("config/categories/"), c.proxy.baseURL("config/sorting/")
	if cat := findCategory(cats, name); cat != nil {
		if strings.HasSuffix(cat.Dir, "*") {
			return fmt.Errorf("enable job folders for category %q: with * appended to the folder SABnzbd does not "+
				"create a separate folder for each download (see %s)", name, categoriesURL)
		}
	} else if strings.TrimSpace(name) != "" {
		return fmt.Errorf("category %q does not exist: create it in SABnzbd first (see %s)", name, categoriesURL)
	}
	sorting := func(kind string) error {
		return fmt.Errorf("disable %s sorting for category %q to prevent import issues (see %s)", kind, name, sortingURL)
	}
	// Sorters are new in SABnzbd 4.1; older versions have none.
	if slices.ContainsFunc(cfg.Sorters, func(s sorter) bool { return bool(s.IsActive) && containsCategory(s.SortCats, name) }) {
		return sorting("TV")
	}
	switch m := &cfg.Misc; {
	case bool(m.EnableTVSorting) && containsCategory(m.TVCategories, name):
		return sorting("TV")
	case bool(m.EnableMovieSorting) && containsCategory(m.MovieCategories, name):
		return sorting("Movie")
	case bool(m.EnableDateSorting) && containsCategory(m.DateCategories, name):
		return sorting("Date")
	}
	return nil
}

// pathKind is how a path of SABnzbd's machine is written, as Radarr's
// OsPath tells.
type pathKind int

const (
	pathUnknown pathKind = iota
	pathUnix
	pathWindows
)

func hasDriveLetter(p string) bool {
	return len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

func kindOfPath(p string) pathKind {
	switch {
	case strings.HasPrefix(p, "/"):
		return pathUnix
	case hasDriveLetter(p) || strings.Contains(p, `\`):
		return pathWindows
	case strings.Contains(p, "/"):
		return pathUnix
	}
	return pathUnknown
}

func isRooted(p string) bool {
	switch kindOfPath(p) {
	case pathWindows:
		return strings.HasPrefix(p, `\\`) || hasDriveLetter(p)
	case pathUnix:
		return strings.HasPrefix(p, "/")
	}
	return false
}

// normalizePath writes a path with its kind's separators.
func normalizePath(p string) (string, string) {
	if kindOfPath(p) == pathWindows {
		return strings.ReplaceAll(p, "/", `\`), `\`
	}
	return strings.ReplaceAll(p, `\`, "/"), "/"
}

// combinePath joins a folder and a path under it, as Radarr's OsPath +:
// a rooted path stands alone.
func combinePath(left, right string) string {
	switch {
	case right == "":
		return left
	case left == "" || isRooted(right):
		return right
	}
	if kindOfPath(left) == pathWindows || kindOfPath(right) == pathWindows {
		l, r := strings.ReplaceAll(left, "/", `\`), strings.ReplaceAll(right, "/", `\`)
		return strings.TrimRight(l, `\`) + `\` + strings.TrimLeft(r, `\`)
	}
	l, r := strings.ReplaceAll(left, `\`, "/"), strings.ReplaceAll(right, `\`, "/")
	return strings.TrimRight(l, "/") + "/" + strings.TrimLeft(r, "/")
}

// jobFolder returns where a download's files are: the outermost folder of
// its storage named as the download, else the storage itself, as Radarr's
// GetHistory finds the job folder of sorted downloads.
func jobFolder(storage, title string) string {
	if storage == "" || title == "" {
		return storage
	}
	p, sep := normalizePath(storage)
	parts := strings.Split(strings.TrimRight(p, sep), sep)
	for i := range len(parts) - 1 {
		if parts[i] == title {
			return strings.Join(parts[:i+1], sep)
		}
	}
	return p
}

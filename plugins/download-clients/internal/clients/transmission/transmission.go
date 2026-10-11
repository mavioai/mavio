package transmission

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of Transmission and Vuze.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Directories
	client.Priorities
	AddPaused bool `json:"add_paused"`
}

// The priorities: last puts torrents at the end of the queue, first at
// its top.
const (
	priorityLast  = 0
	priorityFirst = 1
)

var categoryPattern = regexp.MustCompile(`(?i)^\.?[-a-z]*$`)

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := client.ValidateURLBase("url base", s.URLBase, "/transmission/"); err != nil {
		return err
	}
	for _, k := range client.Kinds {
		switch c := s.Category(k); {
		case !categoryPattern.MatchString(c):
			return fmt.Errorf("category %q: allowed characters a-z and -", c)
		case c != "" && strings.TrimSpace(s.Directory(k)) != "":
			return errors.New("cannot use category and directory")
		}
	}
	return nil
}

func fields(name string) []client.Field {
	f := client.ServerFields(9091, "/transmission/")
	f[len(f)-1].HelpText = fmt.Sprintf("Adds a prefix to the %s rpc url, eg http://[host]:[port]/[urlBase]/rpc, defaults to '/transmission/'", name)
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, name == "Transmission")...)
	for i := range f {
		if strings.HasSuffix(f[i].Name, "_category") && !strings.Contains(f[i].Name, "imported") {
			f[i].HelpText += "; creates a [category] subdirectory in the output directory"
		}
	}
	f = append(f, client.DirectoryFields(name)...)
	f = append(f, client.PriorityFields([]client.Option{{Value: priorityLast, Label: "Last"}, {Value: priorityFirst, Label: "First"}}, priorityLast)...)
	return append(f, client.Field{Name: "add_paused", Label: "Add Paused", Type: client.FieldCheckbox, Default: false})
}

// Definitions returns the definitions of Transmission and Vuze.
func Definitions() []client.Definition {
	return []client.Definition{
		{
			ID: "transmission", Name: "Transmission", Protocol: client.Torrent, Fields: fields("Transmission"),
			Description: "Transmission, over its RPC",
			New: func(raw []byte, env client.Env) (client.Client, error) {
				b, err := newBase(raw, env, "Transmission")
				if err != nil {
					return nil, err
				}
				t := &transmission{base: b}
				b.supportsLabels = t.supportsLabels
				b.validateVersion = t.validateVersion
				b.outputPath = func(dir string, t *torrent) string {
					return client.JoinRemote(dir, strings.ReplaceAll(t.Name, ":", "_"))
				}
				return t, nil
			},
		},
		{
			ID: "vuze", Name: "Vuze", Protocol: client.Torrent, Fields: fields("Vuze"),
			Description: "Vuze, over its Transmission-compatible RPC",
			New: func(raw []byte, env client.Env) (client.Client, error) {
				b, err := newBase(raw, env, "Vuze")
				if err != nil {
					return nil, err
				}
				b.supportsLabels = func(context.Context) bool { return false }
				b.validateVersion = b.validateVuzeVersion
				b.outputPath = vuzeOutputPath
				b.removeID = func(id string) string { return id }
				return &vuze{base: b}, nil
			},
		},
	}
}

// base is the client of Radarr's TransmissionBase.
type base struct {
	name     string
	settings settings
	proxy    *proxy
	env      client.Env

	supportsLabels  func(ctx context.Context) bool
	validateVersion func(ctx context.Context) error
	outputPath      func(dir string, t *torrent) string
	removeID        func(id string) string
}

func newBase(raw []byte, env client.Env, name string) (*base, error) {
	s := settings{
		Server:     client.Server{Host: "localhost", Port: 9091, URLBase: "/transmission/"},
		Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
	}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	b := &base{name: name, settings: s, env: env, removeID: strings.ToLower}
	b.proxy = &proxy{http: client.NoRedirect(env.HTTP), settings: &b.settings}
	return b, nil
}

// transmission is Transmission, which labels torrents from 4.0.
type transmission struct {
	*base

	mu      sync.Mutex
	version string
	checked time.Time
}

// vuze is Vuze, which has no labels.
type vuze struct{ *base }

// Host implements client.Client.
func (b *base) Host() string { return b.settings.Host }

// kindOf returns the kind of a torrent by its labels or folder, as
// Radarr's and Sonarr's GetItems filter torrents, and whether it is of
// either; zero when it is of both.
func (b *base) kindOf(t *torrent, labels bool) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		category := b.settings.Category(k)
		if category != "" && labels && len(t.Labels) > 0 {
			return slices.ContainsFunc(t.Labels, func(l string) bool { return strings.EqualFold(l, category) })
		}
		if dir := b.settings.Directory(k); strings.TrimSpace(dir) != "" {
			return containsPath(dir, t.DownloadDir)
		}
		if category != "" {
			return slices.Contains(strings.FieldsFunc(t.DownloadDir, func(r rune) bool { return r == '/' || r == '\\' }), category)
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
func (b *base) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	torrents, err := b.proxy.torrents(ctx, nil)
	if err != nil {
		return nil, err
	}
	labels := b.supportsLabels(ctx)
	config := sync.OnceValues(func() (*sessionConfig, error) { return b.proxy.config(ctx) })
	var items []client.Item
	for i := range torrents {
		t := &torrents[i]
		kind, ok := b.kindOf(t, labels)
		if !ok {
			continue
		}
		var ratio float64
		if t.DownloadedEver > 0 {
			ratio = float64(t.UploadedEver) / float64(t.DownloadedEver)
		}
		item := client.Item{
			ID: strings.ToUpper(t.HashString), Title: t.Name, Category: b.settings.Category(kind), Kind: kind,
			OutputPath: b.outputPath(t.DownloadDir, t), TotalSize: t.TotalSize, RemainingSize: t.LeftUntilDone,
			SeedRatio:             &ratio,
			HasPostImportCategory: b.settings.ImportedCategory(kind) != "" && labels,
		}
		if t.ETA >= 0 {
			eta := etaDuration(t.ETA)
			item.RemainingTime = &eta
		}
		switch {
		case t.ErrorString != "":
			item.Status = client.Warning
			item.Message = fmt.Sprintf("%s is reporting an error: %s", b.name, t.ErrorString)
		case t.TotalSize == 0:
			item.Status = client.Queued
		case t.LeftUntilDone == 0 && (t.Status == statusStopped || t.Status == statusSeeding || t.Status == statusSeedingWait):
			item.Status = client.Completed
		case t.IsFinished && t.Status != statusCheck && t.Status != statusCheckWait:
			item.Status = client.Completed
		case t.Status == statusQueued:
			item.Status = client.Queued
		default:
			item.Status = client.Downloading
		}
		if opts.RemoveCompletedDownloads {
			reached, err := hasReachedSeedLimit(t, ratio, config)
			if err != nil {
				return nil, err
			}
			item.CanBeRemoved = reached
		}
		item.CanMoveFiles = item.CanBeRemoved && t.Status == statusStopped
		items = append(items, item)
	}
	return items, nil
}

// etaDuration converts an ETA in seconds, or in milliseconds where it is
// too large for seconds as Radarr falls back on overflow.
func etaDuration(eta int64) time.Duration {
	const radarrMaxSeconds = math.MaxInt64 / 10_000_000 // TimeSpan's limit
	switch {
	case eta <= math.MaxInt64/int64(time.Second):
		return time.Duration(eta) * time.Second
	case eta <= radarrMaxSeconds:
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(eta) * time.Millisecond
}

// hasReachedSeedLimit reports whether a torrent is done seeding, by its
// own limits or, without them, the session's.
func hasReachedSeedLimit(t *torrent, ratio float64, config func() (*sessionConfig, error)) (bool, error) {
	stopped, seeding := t.Status == statusStopped, t.Status == statusSeeding
	switch t.SeedRatioMode {
	case 1:
		if stopped && ratio >= t.SeedRatioLimit {
			return true, nil
		}
	case 0:
		if stopped {
			c, err := config()
			if err != nil {
				return false, err
			}
			if c.SeedRatioLimited && ratio >= c.SeedRatioLimit {
				return true, nil
			}
		}
	}
	switch t.SeedIdleMode {
	case 1:
		if (stopped || seeding) && t.SecondsSeeding > t.SeedIdleLimit*60 {
			return true, nil
		}
	case 0:
		if stopped {
			c, err := config()
			if err != nil {
				return false, err
			}
			if c.IdleSeedingLimitEnabled {
				return true, nil
			}
		}
	}
	return false, nil
}

// Remove implements client.Client.
func (b *base) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	return b.proxy.remove(ctx, b.removeID(item.ID), deleteData)
}

// Info implements client.Client.
func (b *base) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: b.settings.IsLocalhost()}
	var config *sessionConfig
	for _, k := range client.Kinds {
		dir := b.settings.Directory(k)
		if strings.TrimSpace(dir) == "" {
			if config == nil {
				c, err := b.proxy.config(ctx)
				if err != nil {
					return info, err
				}
				config = c
			}
			dir = config.DownloadDir
			if c := b.settings.Category(k); c != "" {
				dir = dir + "/" + c
			}
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// downloadDirectory is where torrents of a kind go: its directory, else
// its category's folder in the default one, else empty for the default.
func (b *base) downloadDirectory(ctx context.Context, k client.Kind) (string, error) {
	if dir := b.settings.Directory(k); strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	category := b.settings.Category(k)
	if category == "" {
		return "", nil
	}
	c, err := b.proxy.config(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(c.DownloadDir, "/") + "/" + category, nil
}

// Add implements client.Client.
func (b *base) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, b.env.Log, b, r)
}

// AddMagnet implements client.TorrentAdder.
func (b *base) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	return b.add(ctx, r, hash, func(dir string) error { return b.proxy.addURL(ctx, link, dir, b.settings.Category(r.Kind)) })
}

// AddTorrentFile implements client.TorrentAdder.
func (b *base) AddTorrentFile(ctx context.Context, r *client.Release, hash, _ string, data []byte) (string, error) {
	return b.add(ctx, r, hash, func(dir string) error { return b.proxy.addData(ctx, data, dir, b.settings.Category(r.Kind)) })
}

func (b *base) add(ctx context.Context, r *client.Release, hash string, add func(dir string) error) (string, error) {
	dir, err := b.downloadDirectory(ctx, r.Kind)
	if err != nil {
		return "", err
	}
	if err := add(dir); err != nil {
		return "", err
	}
	if err := b.proxy.setSeeding(ctx, hash, r); err != nil {
		return "", err
	}
	if b.settings.Priority(r.Kind, r.Recent) == priorityFirst {
		if err := b.proxy.moveToTop(ctx, hash); err != nil {
			return "", err
		}
	}
	return hash, nil
}

// Test implements client.Client.
func (b *base) Test(ctx context.Context) error {
	if err := b.validateVersion(ctx); err != nil {
		return err
	}
	if _, err := b.proxy.torrents(ctx, nil); err != nil {
		return fmt.Errorf("failed to get the list of torrents: %w", err)
	}
	return nil
}

var versionPattern = regexp.MustCompile(`[\d.]+`)

// parseVersion reads the version a client reports, such as "2.84+ (1)",
// leaving out what is in parentheses.
func parseVersion(s string) ([]int, error) {
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	m := strings.Trim(versionPattern.FindString(s), ".")
	if m == "" {
		return nil, fmt.Errorf("unable to parse version %q", s)
	}
	var out []int
	for p := range strings.SplitSeq(m, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("unable to parse version %q", s)
		}
		out = append(out, n)
	}
	return out, nil
}

// atLeast reports whether version v is major.minor or later.
func atLeast(v []int, major, minor int) bool {
	return slices.Compare(v, []int{major, minor}) >= 0
}

// clientVersion returns Transmission's version, kept for 6 hours unless
// force is set.
func (t *transmission) clientVersion(ctx context.Context, force bool) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.env.Now()
	if !force && t.version != "" && now.Sub(t.checked) < 6*time.Hour {
		return t.version, nil
	}
	c, err := t.proxy.config(ctx)
	if err != nil {
		return "", err
	}
	t.version, t.checked = c.Version, now
	return c.Version, nil
}

func (t *transmission) supportsLabels(ctx context.Context) bool {
	s, err := t.clientVersion(ctx, false)
	if err != nil {
		return false
	}
	v, err := parseVersion(s)
	return err == nil && atLeast(v, 4, 0)
}

func (t *transmission) validateVersion(ctx context.Context) error {
	s, err := t.clientVersion(ctx, true)
	if err != nil {
		return err
	}
	t.env.Log.DebugContext(ctx, "Transmission version information", "version", s)
	v, err := parseVersion(s)
	if err != nil {
		return err
	}
	if !atLeast(v, 2, 40) {
		return fmt.Errorf("transmission version %s is not supported; 2.40 or later is required", s)
	}
	return nil
}

// MarkImported implements client.Marker: the torrent's label becomes the
// post-import category, from Transmission 4.0.
func (t *transmission) MarkImported(ctx context.Context, item client.Item) error {
	if !t.supportsLabels(ctx) {
		return fmt.Errorf("%s does not support marking items as imported: %w", t.name, client.ErrNotSupported)
	}
	kind := item.Kind
	if kind == 0 {
		kind = client.Movie
	}
	imported, category := t.settings.ImportedCategory(kind), t.settings.Category(kind)
	if imported == "" || imported == category {
		return nil
	}
	hash := strings.ToLower(item.ID)
	torrents, err := t.proxy.torrents(ctx, []string{hash})
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		t.env.Log.WarnContext(ctx, "could not find torrent in Transmission", "hash", hash)
		return nil
	}
	var labels []string
	for _, l := range append(torrents[0].Labels, imported) {
		if category != "" && strings.EqualFold(l, category) ||
			slices.ContainsFunc(labels, func(x string) bool { return strings.EqualFold(x, l) }) {
			continue
		}
		labels = append(labels, l)
	}
	if err := t.proxy.setLabels(ctx, hash, labels); err != nil {
		t.env.Log.WarnContext(ctx, "failed to set the post-import torrent label",
			"label", imported, "download", item.Title, "err", err)
	}
	return nil
}

// validateVuzeVersion checks Vuze's RPC version, 14 or later.
func (b *base) validateVuzeVersion(ctx context.Context) error {
	c, err := b.proxy.config(ctx)
	if err != nil {
		return err
	}
	b.env.Log.DebugContext(ctx, "Vuze protocol version information", "version", c.RPCVersion)
	if v, err := strconv.Atoi(c.RPCVersion.String()); err != nil || v < 14 {
		return errors.New("the protocol version of Vuze is not supported; use Vuze 5.0.0.0 or later with the Vuze Web Remote plugin")
	}
	return nil
}

// vuzeOutputPath returns where a Vuze torrent's files are: Vuze puts a
// multi-file torrent in a folder of its own, which downloadDir names, and a
// single file in the folder.
func vuzeOutputPath(dir string, t *torrent) string {
	if dir == "" || client.RemoteBase(dir) == t.Name || t.fileCount() > 1 {
		return dir
	}
	return client.JoinRemote(dir, t.Name)
}

// containsPath reports whether folder holds p, as Radarr's
// OsPath.Contains, with either slashes.
func containsPath(folder, p string) bool {
	norm := func(s string) string {
		s = path.Clean(strings.ReplaceAll(s, `\`, "/"))
		if len(s) >= 2 && s[1] == ':' {
			s = strings.ToLower(s[:1]) + s[1:]
		}
		return s
	}
	f, q := norm(folder), norm(p)
	return q == f || strings.HasPrefix(q, strings.TrimRight(f, "/")+"/")
}

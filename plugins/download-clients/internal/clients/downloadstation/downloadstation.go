package downloadstation

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of Download Station, as Radarr's
// DownloadStationSettings by kind.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Directories
}

var categoryPattern = regexp.MustCompile(`(?i)^\.?[-a-z]*$`)

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	for _, k := range client.Kinds {
		dir, category := s.Directory(k), s.Category(k)
		switch {
		case strings.TrimSpace(dir) != "" && strings.HasPrefix(dir, "/"):
			return fmt.Errorf("directory %q: cannot start with /", dir)
		case !categoryPattern.MatchString(category):
			return fmt.Errorf("category %q: allowed characters a-z and -", category)
		case category != "" && strings.TrimSpace(dir) != "":
			return errors.New("cannot use category and directory")
		}
	}
	return nil
}

func fields() []client.Field {
	f := client.ServerFields(5000, "-")
	f[0].Default = "127.0.0.1"
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Secret: true},
	)
	categories := client.CategoryFields("Category", "", "", false)
	for i := range categories {
		categories[i].HelpText += "; creates a [category] subdirectory in the output directory"
	}
	f = append(f, categories...)
	dirs := client.DirectoryFields("Download Station")
	for i := range dirs {
		dirs[i].HelpText = "Optional shared folder to put downloads into, leave blank to use the default Download Station location"
	}
	return append(f, dirs...)
}

// description warns of what Radarr's provider message does.
const description = "Synology Download Station, over the DSM web API; it cannot be reached when 2-factor authentication is enabled on the DSM account"

// Definitions returns the definitions of Download Station for torrents and
// for NZBs.
func Definitions() []client.Definition {
	return []client.Definition{
		{
			ID: "torrent-download-station", Name: "Download Station", Protocol: client.Torrent, Fields: fields(),
			Description: description,
			New: func(raw []byte, env client.Env) (client.Client, error) {
				s, err := newStation(raw, env, false)
				if err != nil {
					return nil, err
				}
				return &torrentStation{station: s}, nil
			},
		},
		{
			ID: "usenet-download-station", Name: "Download Station", Protocol: client.Usenet, Fields: fields(),
			Description: description,
			New: func(raw []byte, env client.Env) (client.Client, error) {
				s, err := newStation(raw, env, true)
				if err != nil {
					return nil, err
				}
				return &usenetStation{station: s}, nil
			},
		},
	}
}

// How long the task API, serial number and shared folders are kept, as
// Radarr caches them.
const (
	taskAPITime = 10 * time.Minute
	serialTime  = 5 * time.Minute
	shareTime   = time.Hour
)

// station is the client of Radarr's TorrentDownloadStation and
// UsenetDownloadStation, which differ in their tasks' type, remaining
// times and removal.
type station struct {
	settings settings
	env      client.Env
	dsm      *dsm
	usenet   bool

	apiMu sync.Mutex
	api   taskAPI
	apiAt time.Time

	serialMu sync.Mutex
	serial   string
	serialAt time.Time

	sharesMu sync.Mutex
	shares   map[string]cachedShare
}

type cachedShare struct {
	physical string
	at       time.Time
}

func newStation(raw []byte, env client.Env, usenet bool) (*station, error) {
	s := settings{Server: client.Server{Host: "127.0.0.1", Port: 5000}}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	st := &station{settings: s, env: env, usenet: usenet}
	st.dsm = &dsm{http: client.NoRedirect(env.HTTP), s: &st.settings, now: env.Now, log: env.Log}
	return st, nil
}

// torrentStation is Download Station for torrents.
type torrentStation struct{ *station }

// usenetStation is Download Station for NZBs.
type usenetStation struct{ *station }

// Host implements client.Client.
func (s *station) Host() string { return s.settings.Host }

func (s *station) taskType() string {
	if s.usenet {
		return "NZB"
	}
	return "BT"
}

// taskAPI returns the task API, v2 where the Diskstation has it, as
// Radarr's DownloadStationTaskProxySelector.
func (s *station) taskAPI(ctx context.Context) (taskAPI, error) {
	s.apiMu.Lock()
	defer s.apiMu.Unlock()
	if s.api != nil && s.env.Now().Sub(s.apiAt) < taskAPITime {
		return s.api, nil
	}
	var chosen taskAPI
	if info, err := s.dsm.info(ctx, synoTask2); err != nil {
		return nil, err
	} else if info != nil {
		chosen = taskV2{s.dsm}
	} else {
		s.env.Log.WarnContext(ctx, "Download Station task API v2 not found", "host", s.settings.Host, "port", s.settings.Port)
		info, err := s.dsm.info(ctx, synoTask)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, errors.New("unable to determine Download Station's task API version")
		}
		chosen = taskV1{s.dsm}
	}
	s.api, s.apiAt = chosen, s.env.Now()
	return chosen, nil
}

// serialNumber returns the hash of the Diskstation's serial number, kept
// for 5 minutes, as Radarr's SerialNumberProvider.
func (s *station) serialNumber(ctx context.Context) (string, error) {
	s.serialMu.Lock()
	defer s.serialMu.Unlock()
	if s.serial != "" && s.env.Now().Sub(s.serialAt) < serialTime {
		return s.serial, nil
	}
	serial, err := s.dsm.serialNumber(ctx)
	if err != nil {
		s.env.Log.WarnContext(ctx, "could not get the serial number from Download Station",
			"host", s.settings.Host, "port", s.settings.Port, "err", err)
		return "", err
	}
	s.serial, s.serialAt = hashSerial(serial), s.env.Now()
	return s.serial, nil
}

// fullPath returns the physical path of a path in a shared folder, such
// as /video/movies, by its shared folder's path kept for an hour, as
// Radarr's SharedFolderResolver.
func (s *station) fullPath(ctx context.Context, p, serial string) (string, error) {
	share := p
	if i := strings.Index(p[min(1, len(p)):], "/"); i >= 0 {
		share = p[:i+1]
	}
	key := serial + ":" + share
	s.sharesMu.Lock()
	defer s.sharesMu.Unlock()
	if c, ok := s.shares[key]; ok && s.env.Now().Sub(c.at) < shareTime {
		return remap(p, share, c.physical), nil
	}
	physical, err := s.sharedFolder(ctx, share)
	if err != nil {
		s.env.Log.WarnContext(ctx, "failed to get shared folder from Disk Station",
			"folder", share, "host", s.settings.Host, "port", s.settings.Port, "err", err)
		return "", err
	}
	if s.shares == nil {
		s.shares = map[string]cachedShare{}
	}
	s.shares[key] = cachedShare{physical: physical, at: s.env.Now()}
	return remap(p, share, physical), nil
}

// sharedFolder returns a shared folder's physical path, as Radarr's
// FileStationProxy.GetSharedFolderMapping.
func (s *station) sharedFolder(ctx context.Context, share string) (string, error) {
	info, err := s.dsm.fileInfo(ctx, share)
	if err != nil {
		return "", err
	}
	real, ok := info.Additional["real_path"].(string)
	if !ok {
		return "", fmt.Errorf("there is no shared folder %s", share)
	}
	return real, nil
}

// tasks lists the tasks of the client's type.
func (s *station) tasks(ctx context.Context) ([]task, error) {
	api, err := s.taskAPI(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := api.tasks(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(tasks, func(t task) bool { return !strings.EqualFold(t.Type, s.taskType()) }), nil
}

// kindOf returns the kind of a task by its destination, as Radarr's and
// Sonarr's GetItems filter tasks, and whether it is of either; zero when
// it is of both.
func (s *station) kindOf(destination string) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		if dir := s.settings.Directory(k); strings.TrimSpace(dir) != "" {
			return containsPath("/"+dir, destination)
		}
		if category := s.settings.Category(k); strings.TrimSpace(category) != "" {
			return slices.Contains(strings.FieldsFunc(destination, func(r rune) bool { return r == '/' || r == '\\' }), category)
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
func (s *station) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	tasks, err := s.tasks(ctx)
	if err != nil {
		return nil, err
	}
	serial, err := s.serialNumber(ctx)
	if err != nil {
		return nil, err
	}
	// NZBs share the speed of those downloading, and wait for those ahead.
	var globalSpeed, totalRemaining int64
	for i := range tasks {
		if tasks[i].Status == statusDownloading {
			globalSpeed += s.speed(ctx, &tasks[i])
		}
	}
	var items []client.Item
	for i := range tasks {
		t := &tasks[i]
		destination := "/" + t.Additional.Detail["destination"]
		remaining := s.remainingSize(ctx, t)
		if t.Status != statusPaused {
			totalRemaining += remaining
		}
		kind, ok := s.kindOf(destination)
		if !ok {
			continue
		}
		item := client.Item{
			ID: serial + ":" + t.ID, Title: t.Title, Category: s.settings.Category(kind), Kind: kind,
			TotalSize: t.Size, RemainingSize: remaining, Status: s.status(ctx, t), Message: message(t),
		}
		if s.usenet {
			item.CanBeRemoved, item.CanMoveFiles = true, true
			if item.Status != client.Paused {
				item.RemainingTime = remainingTime(totalRemaining, globalSpeed)
			}
		} else {
			item.RemainingTime = remainingTime(remaining, s.speed(ctx, t))
			item.SeedRatio = seedRatio(t)
			item.CanBeRemoved = opts.RemoveCompletedDownloads && t.Status == statusFinished
			item.CanMoveFiles = item.CanBeRemoved
		}
		if item.Status == client.Completed || item.Status == client.Failed {
			full, err := s.fullPath(ctx, destination, serial)
			if err != nil {
				return nil, err
			}
			item.OutputPath = client.JoinRemote(full, t.Title)
		}
		items = append(items, item)
	}
	return items, nil
}

func message(t *task) string {
	if t.StatusExtra == nil {
		return ""
	}
	switch t.Status {
	case statusExtracting:
		return fmt.Sprintf("Extracting: %s%%", t.StatusExtra["unzip_progress"])
	case statusError:
		return t.StatusExtra["error_detail"]
	}
	return ""
}

func (s *station) status(ctx context.Context, t *task) client.Status {
	switch t.Status {
	case statusUnknown, statusWaiting, statusFilehostingWaiting:
		if t.Size == 0 || s.remainingSize(ctx, t) > 0 {
			return client.Queued
		}
		return client.Completed
	case statusPaused:
		return client.Paused
	case statusFinished, statusSeeding:
		return client.Completed
	case statusError:
		return client.Failed
	}
	return client.Downloading
}

// transferValue reads a number of a task's transfer, zero when invalid.
func (s *station) transferValue(ctx context.Context, t *task, key string) int64 {
	v := t.Additional.Transfer[key]
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		s.env.Log.DebugContext(ctx, "task has an invalid "+key, "task", t.Title, "value", v)
		return 0
	}
	return n
}

func (s *station) remainingSize(ctx context.Context, t *task) int64 {
	return t.Size - max(0, s.transferValue(ctx, t, "size_downloaded"))
}

func (s *station) speed(ctx context.Context, t *task) int64 {
	return max(0, s.transferValue(ctx, t, "speed_download"))
}

func remainingTime(remaining, speed int64) *time.Duration {
	if speed <= 0 {
		return nil
	}
	d := time.Duration(remaining/speed) * time.Second
	return &d
}

func seedRatio(t *task) *float64 {
	down, err1 := strconv.ParseInt(t.Additional.Transfer["size_downloaded"], 10, 64)
	up, err2 := strconv.ParseInt(t.Additional.Transfer["size_uploaded"], 10, 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	ratio := 0.0
	if down > 0 {
		ratio = float64(up) / float64(down)
	}
	return &ratio
}

// downloadDirectory is the shared folder downloads of a kind go to,
// without a leading slash: its directory, else the default destination
// with its category; empty when there is no default destination.
func (s *station) downloadDirectory(ctx context.Context, k client.Kind) (string, error) {
	if dir := s.settings.Directory(k); strings.TrimSpace(dir) != "" {
		return strings.TrimLeft(dir, "/"), nil
	}
	dest, err := s.dsm.defaultDir(ctx)
	if err != nil {
		return "", err
	}
	if category := s.settings.Category(k); strings.TrimSpace(dest) != "" && strings.TrimSpace(category) != "" {
		return strings.TrimRight(dest, "/") + "/" + category, nil
	}
	return strings.TrimRight(dest, "/"), nil
}

// Info implements client.Client.
func (s *station) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: s.settings.IsLocalhost()}
	serial, err := s.serialNumber(ctx)
	if err != nil {
		return info, err
	}
	for _, k := range client.Kinds {
		dir, err := s.downloadDirectory(ctx, k)
		if err != nil {
			return info, err
		}
		full, err := s.fullPath(ctx, "/"+dir, serial)
		if err != nil {
			return info, err
		}
		if !slices.Contains(info.OutputRootFolders, full) {
			info.OutputRootFolders = append(info.OutputRootFolders, full)
		}
	}
	return info, nil
}

// Remove implements client.Client.
func (s *station) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	_, id, ok := strings.Cut(item.ID, ":")
	if !ok {
		return fmt.Errorf("invalid Download Station download ID %q", item.ID)
	}
	if deleteData {
		client.DeleteItemData(ctx, s.env.Log, item)
	}
	api, err := s.taskAPI(ctx)
	if err != nil {
		return err
	}
	if err := api.remove(ctx, id); err != nil {
		return err
	}
	s.env.Log.DebugContext(ctx, "removed from Download Station", "download", item.ID)
	return nil
}

// added returns the ID of the task added with a URI, as Download Station
// gives none when adding.
func (s *station) added(ctx context.Context, serial, uri, what string) (string, error) {
	tasks, err := s.tasks(ctx)
	if err != nil {
		return "", err
	}
	for _, t := range tasks {
		if t.Additional.Detail["uri"] == uri {
			return serial + ":" + t.ID, nil
		}
	}
	s.env.Log.DebugContext(ctx, "no such task in Download Station", "uri", uri)
	return "", fmt.Errorf("failed to add %s task to Download Station", what)
}

// add adds a task, its serial number first so that none is added
// without an ID.
func (s *station) add(ctx context.Context, k client.Kind, uri, what string, add func(api taskAPI, dir string) error) (string, error) {
	serial, err := s.serialNumber(ctx)
	if err != nil {
		return "", err
	}
	dir, err := s.downloadDirectory(ctx, k)
	if err != nil {
		return "", err
	}
	api, err := s.taskAPI(ctx)
	if err != nil {
		return "", err
	}
	if err := add(api, dir); err != nil {
		return "", err
	}
	return s.added(ctx, serial, uri, what)
}

// Add implements client.Client.
func (t *torrentStation) Add(ctx context.Context, r *client.Release) (string, error) {
	// AddTorrent upper-cases what the adder returns, for info hashes;
	// Download Station's IDs are its own.
	a := &rawID{TorrentAdder: t}
	if _, err := client.AddTorrent(ctx, t.env.Log, a, r); err != nil {
		return "", err
	}
	return a.id, nil
}

// AddMagnet implements client.TorrentAdder.
func (t *torrentStation) AddMagnet(ctx context.Context, r *client.Release, _, link string) (string, error) {
	return t.add(ctx, r.Kind, link, "magnet", func(api taskAPI, dir string) error { return api.addURL(ctx, link, dir) })
}

// AddTorrentFile implements client.TorrentAdder; Download Station names
// the task by the file's name without its extension.
func (t *torrentStation) AddTorrentFile(ctx context.Context, r *client.Release, _, fileName string, data []byte) (string, error) {
	uri := strings.TrimSuffix(fileName, path.Ext(fileName))
	return t.add(ctx, r.Kind, uri, "torrent", func(api taskAPI, dir string) error { return api.addData(ctx, data, fileName, dir) })
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

// Add implements client.Client.
func (u *usenetStation) Add(ctx context.Context, r *client.Release) (string, error) {
	name := r.FileName
	if name == "" {
		name = client.CleanFileName(r.Title) + ".nzb"
	}
	return u.add(ctx, r.Kind, name, "NZB", func(api taskAPI, dir string) error { return api.addData(ctx, r.Data, name, dir) })
}

// Test implements client.Client, as Radarr's: the task API's version,
// then the download folders, then the tasks.
func (s *station) Test(ctx context.Context) error {
	err := s.test(ctx)
	var auth *client.AuthError
	var unavailable *client.UnavailableError
	switch {
	case errors.As(err, &auth):
		return &client.AuthError{Err: fmt.Errorf("verify the username and password, and that Download Station does not block Mavio's host (%w)", auth.Err)}
	case errors.As(err, &unavailable):
		return &client.UnavailableError{Err: fmt.Errorf("verify the host and port (%w)", unavailable.Err)}
	}
	return err
}

func (s *station) test(ctx context.Context) error {
	if err := s.testConnection(ctx); err != nil {
		return err
	}
	if err := s.testOutputPath(ctx); err != nil {
		return err
	}
	if _, err := s.Items(ctx, client.Options{}); err != nil {
		what := "torrents"
		if s.usenet {
			what = "NZBs"
		}
		return fmt.Errorf("failed to get the list of %s: %w", what, err)
	}
	return nil
}

func (s *station) testConnection(ctx context.Context) error {
	info, err := s.apiInfo(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to Download Station: %w", err)
	}
	s.env.Log.DebugContext(ctx, "Download Station api version information", "min", info.MinVersion, "max", info.MaxVersion)
	if info.MinVersion > 2 || info.MaxVersion < 2 {
		return fmt.Errorf("Download Station API version not supported, should be at least 2. It supports from %d to %d", info.MinVersion, info.MaxVersion) //nolint:staticcheck // Radarr's message.
	}
	return nil
}

func (s *station) apiInfo(ctx context.Context) (*apiInfo, error) {
	api, err := s.taskAPI(ctx)
	if err != nil {
		return nil, err
	}
	info, err := api.info(ctx)
	if err == nil && info == nil {
		err = errors.New("no info of Download Station's task API")
	}
	return info, err
}

// testOutputPath checks the shared folders of each kind's downloads.
func (s *station) testOutputPath(ctx context.Context) error {
	var tested []string
	for _, k := range client.Kinds {
		dir, err := s.downloadDirectory(ctx, k)
		if err == nil && slices.Contains(tested, dir) {
			continue
		}
		tested = append(tested, dir)
		if err == nil {
			err = s.testFolder(ctx, dir)
		}
		var auth *client.AuthError
		switch {
		case errors.As(err, &auth):
			return err
		case err != nil && !errors.Is(err, errTest):
			return fmt.Errorf("unknown exception: %w", err)
		case err != nil:
			return err
		}
	}
	return nil
}

// errTest marks the failures of testFolder.
var errTest = errors.New("download folder")

func (s *station) testFolder(ctx context.Context, dir string) error {
	if dir == "" {
		return fmt.Errorf("%w: no default destination; you must log in to your Diskstation as %s and set it up in Download Station's settings under BT/HTTP/FTP/NZB -> Location",
			errTest, s.settings.Username)
	}
	share := dir
	if parts := strings.FieldsFunc(dir, func(r rune) bool { return r == '/' || r == '\\' }); len(parts) > 0 {
		share = parts[0]
	}
	info, err := s.dsm.fileInfo(ctx, "/"+dir)
	switch {
	case err != nil:
		return err
	case info.Additional == nil:
		return fmt.Errorf("%w: shared folder does not exist; the Diskstation does not have a shared folder named %q", errTest, share)
	case !info.IsDir:
		return fmt.Errorf("%w: folder does not exist; the folder %q must be created manually inside the shared folder %q", errTest, dir, share)
	}
	return nil
}

// containsPath reports whether folder holds p, as Radarr's
// OsPath.Contains.
func containsPath(folder, p string) bool {
	f, q := path.Clean("/"+strings.ReplaceAll(folder, `\`, "/")), path.Clean("/"+strings.ReplaceAll(p, `\`, "/"))
	return q == f || strings.HasPrefix(q, strings.TrimRight(f, "/")+"/")
}

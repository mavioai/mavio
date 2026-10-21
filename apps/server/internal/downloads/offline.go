package downloads

import (
	"cmp"
	"context"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/rclone"
)

// OfflineID is the provider ID of the offline download clients, under
// which their configuration is kept as a plugin's is.
const OfflineID = "dev.mavio.rclone-offline"

// offlineDefinition is the one kind of offline download client.
const offlineDefinition = "offline"

// Categories of offline downloads, and the folders of a remote that movies
// and series are downloaded into unless its settings name others.
const (
	categoryMovies = "movies"
	categorySeries = "series"
)

// OfflineConfig is the configuration of the offline download clients.
type OfflineConfig struct {
	Clients []OfflineClient `json:"remotes"`
}

// OfflineClient is the offline downloads of a remote whose cloud drive
// downloads magnet links itself: a download client named after the remote,
// one at most per remote, downloading movies into MovieFolder and series
// into SeriesFolder, folders below the remote's root.
type OfflineClient struct {
	Remote   string   `json:"remote"`
	Enabled  bool     `json:"enabled"`
	Priority int32    `json:"priority"`
	Tags     []string `json:"tags,omitempty"`
	// MovieFolder and SeriesFolder default to "movies" and "series".
	MovieFolder  string `json:"movie_folder,omitempty"`
	SeriesFolder string `json:"series_folder,omitempty"`
	// RemoveCompleted and RemoveFailed remove imported and failed
	// downloads from the drive's list of offline downloads.
	RemoveCompleted bool `json:"remove_completed_downloads"`
	RemoveFailed    bool `json:"remove_failed_downloads"`
}

// validate checks the client's settings.
func (c OfflineClient) validate() error {
	if c.Remote == "" || strings.ContainsAny(c.Remote, `/\: `) {
		return fmt.Errorf("%w: offline downloads of remote %q", core.ErrInvalid, c.Remote)
	}
	for _, dir := range []string{c.MovieFolder, c.SeriesFolder} {
		if dir != "" && (path.Clean(dir) != dir || dir == "." || dir == ".." || strings.HasPrefix(dir, "../") || strings.HasPrefix(dir, "/") || strings.Contains(dir, `\`)) {
			return fmt.Errorf("%w: offline download folder %q: want a folder below the remote's root", core.ErrInvalid, dir)
		}
	}
	return nil
}

// folder returns the folder below the remote's root a category's downloads
// go to.
func (c OfflineClient) folder(category string) string {
	if category == categorySeries {
		return cmp.Or(c.SeriesFolder, categorySeries)
	}
	return cmp.Or(c.MovieFolder, categoryMovies)
}

// path returns the path of a folder below the remote's root, as library
// paths are written.
func (c OfflineClient) path(dir string) string {
	return core.RemoteScheme + c.Remote + ":" + dir
}

// Folders returns the folders movies and series are downloaded into.
func (c OfflineClient) Folders() (movies, series string) {
	return c.folder(categoryMovies), c.folder(categorySeries)
}

// Offline is the provider of the offline download clients, built into the
// server: cloud drives that download magnet links themselves through
// mavioai/rclone's operations/addurl. The downloads it sent are kept in the
// storage job ledger, which tells them apart from the drive's other
// offline downloads and keeps their title and category.
type Offline struct {
	// Remotes reaches the rcd; nil when the server reaches none.
	Remotes *rclone.Remotes
	Jobs    core.StorageJobRepository
	Configs core.PluginConfigRepository

	// mu serializes changes of the configuration.
	mu sync.Mutex
}

// Config returns the configuration of the clients.
func (o *Offline) Config(ctx context.Context) (OfflineConfig, error) {
	var cfg OfflineConfig
	c, err := o.Configs.Get(ctx, OfflineID)
	if errors.Is(err, core.ErrNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(c.JSON), &cfg); err != nil {
		return cfg, fmt.Errorf("offline download clients: %w", err)
	}
	return cfg, nil
}

// setConfig validates and stores the configuration of the clients.
func (o *Offline) setConfig(ctx context.Context, cfg OfflineConfig) error {
	seen := map[string]bool{}
	for _, c := range cfg.Clients {
		if err := c.validate(); err != nil {
			return err
		}
		if seen[c.Remote] {
			return fmt.Errorf("%w: two offline download clients of remote %s", core.ErrInvalid, c.Remote)
		}
		seen[c.Remote] = true
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return o.Configs.Put(ctx, &core.PluginConfig{PluginID: OfflineID, JSON: string(data)})
}

// Client returns the offline downloads of a remote, and whether it has them.
func (o *Offline) Client(ctx context.Context, remote string) (OfflineClient, bool, error) {
	cfg, err := o.Config(ctx)
	if err != nil {
		return OfflineClient{}, false, err
	}
	i := slices.IndexFunc(cfg.Clients, func(c OfflineClient) bool { return c.Remote == remote })
	if i < 0 {
		return OfflineClient{}, false, nil
	}
	return cfg.Clients[i], true, nil
}

// SetClient replaces the offline downloads of c's remote.
func (o *Offline) SetClient(ctx context.Context, c OfflineClient) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	cfg, err := o.Config(ctx)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(cfg.Clients, func(x OfflineClient) bool { return x.Remote == c.Remote }); i >= 0 {
		cfg.Clients[i] = c
	} else {
		cfg.Clients = append(cfg.Clients, c)
	}
	return o.setConfig(ctx, cfg)
}

// RemoveClient removes the offline downloads of a remote, and the
// downloads the ledger keeps for it, as when the remote is removed.
func (o *Offline) RemoveClient(ctx context.Context, remote string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	cfg, err := o.Config(ctx)
	if err != nil {
		return err
	}
	n := len(cfg.Clients)
	cfg.Clients = slices.DeleteFunc(cfg.Clients, func(c OfflineClient) bool { return c.Remote == remote })
	if len(cfg.Clients) != n {
		if err := o.setConfig(ctx, cfg); err != nil {
			return err
		}
	}
	jobs, err := o.Jobs.List(ctx, core.StorageJobOffline)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		if client, _, _ := strings.Cut(j.Key, "/"); client == remote {
			if err := o.Jobs.Delete(ctx, core.StorageJobOffline, j.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

// client returns a configured client, or a NotFound error.
func (o *Offline) client(ctx context.Context, id string) (OfflineClient, error) {
	c, ok, err := o.Client(ctx, id)
	if err != nil {
		return OfflineClient{}, err
	}
	if !ok {
		return OfflineClient{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("no offline downloads of remote %s", id))
	}
	return c, nil
}

// rc returns the rcd's client, or an Unavailable error.
func (o *Offline) rc() (*rclone.Client, error) {
	if o.Remotes == nil || o.Remotes.Client == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the server reaches no rclone"))
	}
	return o.Remotes.Client, nil
}

// ListDownloadClientDefinitions implements
// pluginv1connect.DownloadClientProviderServiceClient.
func (o *Offline) ListDownloadClientDefinitions(context.Context, *pluginv1.ListDownloadClientDefinitionsRequest) (*pluginv1.ListDownloadClientDefinitionsResponse, error) {
	def := pluginv1.DownloadClientDefinition_builder{
		Id:          new(offlineDefinition),
		Name:        new("Cloud drive offline download"),
		Description: new("A cloud drive downloads magnet links itself into folders of its remote; imports move the files within the remote. Set on the remote's page."),
		Protocol:    new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
	}.Build()
	return pluginv1.ListDownloadClientDefinitionsResponse_builder{Definitions: []*pluginv1.DownloadClientDefinition{def}}.Build(), nil
}

// ListDownloadClients implements
// pluginv1connect.DownloadClientProviderServiceClient.
func (o *Offline) ListDownloadClients(ctx context.Context, _ *pluginv1.ListDownloadClientsRequest) (*pluginv1.ListDownloadClientsResponse, error) {
	cfg, err := o.Config(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*pluginv1.DownloadClient, len(cfg.Clients))
	for i, c := range cfg.Clients {
		out[i] = pluginv1.DownloadClient_builder{
			Id: new(c.Remote), Name: new(c.Remote), DefinitionId: new(offlineDefinition),
			Protocol: new(pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT),
			Enabled:  new(c.Enabled), Priority: new(cmp.Or(c.Priority, 1)),
			RemoveCompletedDownloads: new(c.RemoveCompleted), RemoveFailedDownloads: new(c.RemoveFailed),
			Tags: c.Tags,
		}.Build()
	}
	return pluginv1.ListDownloadClientsResponse_builder{Clients: out}.Build(), nil
}

// TestDownloadClient implements
// pluginv1connect.DownloadClientProviderServiceClient: the remote must make
// offline downloads.
func (o *Offline) TestDownloadClient(ctx context.Context, req *pluginv1.TestDownloadClientRequest) (*pluginv1.TestDownloadClientResponse, error) {
	c, err := o.client(ctx, req.GetClientId())
	if err != nil {
		return nil, err
	}
	failure := func(msg string) (*pluginv1.TestDownloadClientResponse, error) {
		return pluginv1.TestDownloadClientResponse_builder{Failure: &msg}.Build(), nil
	}
	if err := c.validate(); err != nil {
		return failure(err.Error())
	}
	if _, err := o.rc(); err != nil {
		return failure(err.Error())
	}
	ok, err := o.Remotes.OfflineDownloads(ctx, c.Remote)
	switch {
	case err != nil:
		return failure(err.Error())
	case !ok:
		return failure(fmt.Sprintf("the backend of remote %s makes no offline downloads", c.Remote))
	}
	var warnings []string
	for _, dir := range []string{c.folder(categoryMovies), c.folder(categorySeries)} {
		st, err := o.Remotes.Open(ctx, c.Remote, dir)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			warnings = append(warnings, fmt.Sprintf("%s does not exist yet; the first download creates it", core.RemoteScheme+c.Remote+":"+dir))
		case err != nil:
			return failure(err.Error())
		default:
			_ = st.Close()
		}
	}
	return pluginv1.TestDownloadClientResponse_builder{Warnings: warnings}.Build(), nil
}

// AddDownload implements pluginv1connect.DownloadClientProviderServiceClient.
// Drives take magnet links only.
func (o *Offline) AddDownload(ctx context.Context, req *pluginv1.AddDownloadRequest) (*pluginv1.AddDownloadResponse, error) {
	c, err := o.client(ctx, req.GetClientId())
	if err != nil {
		return nil, err
	}
	rc, err := o.rc()
	if err != nil {
		return nil, err
	}
	link := req.GetMagnetUrl()
	if link == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cloud drives take magnet links only"))
	}
	hash := strings.ToUpper(cmp.Or(req.GetInfoHash(), magnetHash(link)))
	if hash != "" && slices.ContainsFunc(req.GetBlocklistedInfoHashes(), func(h string) bool { return strings.EqualFold(h, hash) }) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("torrent %s is blocklisted", hash))
	}
	if hash != "" {
		if _, err := o.Jobs.Get(ctx, core.StorageJobOffline, c.Remote+"/"+hash); err == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("torrent %s was sent to %s already", hash, c.Remote))
		} else if !errors.Is(err, core.ErrNotFound) {
			return nil, err
		}
	}
	category, kind := categoryMovies, "movie"
	if req.GetKind() == pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES {
		category, kind = categorySeries, "series"
	}
	remote, target := c.Remote, c.folder(category)
	if err := rc.Call(ctx, "operations/mkdir", map[string]any{"fs": remote + ":", "remote": target}, nil); err != nil {
		return nil, offlineError(err)
	}
	task, err := rc.AddOffline(ctx, remote+":"+target, link, "")
	if err != nil {
		return nil, offlineError(err)
	}
	if hash == "" {
		hash = strings.ToUpper(task.ID)
	}
	job := core.StorageJob{
		Kind: core.StorageJobOffline, Key: c.Remote + "/" + hash, Remote: remote,
		Data: map[string]string{
			"task": task.ID, "title": req.GetTitle(), "kind": kind, "category": category,
			"folder": target, "name": task.Name, "status": string(cmp.Or(task.Status, rclone.OfflineInit)),
		},
	}
	if err := o.Jobs.Put(ctx, &job); err != nil {
		return nil, err
	}
	return pluginv1.AddDownloadResponse_builder{DownloadId: &hash}.Build(), nil
}

// offlineError maps an error of the rcd: one it refused is a release the
// drive rejected.
func offlineError(err error) error {
	var e *rclone.Error
	if errors.As(err, &e) && e.Status < http.StatusInternalServerError {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeUnavailable, err)
}

// magnetHash returns the info hash of a magnet link in hex, or "".
func magnetHash(link string) string {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "magnet" {
		return ""
	}
	for _, xt := range u.Query()["xt"] {
		h, ok := strings.CutPrefix(strings.ToLower(xt), "urn:btih:")
		if !ok {
			continue
		}
		switch len(h) {
		case 40:
			if _, err := hex.DecodeString(h); err == nil {
				return h
			}
		case 32:
			if b, err := base32.StdEncoding.DecodeString(strings.ToUpper(h)); err == nil {
				return hex.EncodeToString(b)
			}
		}
	}
	return ""
}

// ListDownloads implements pluginv1connect.DownloadClientProviderServiceClient.
func (o *Offline) ListDownloads(ctx context.Context, req *pluginv1.ListDownloadsRequest) (*pluginv1.ListDownloadsResponse, error) {
	cfg, err := o.Config(ctx)
	if err != nil {
		return nil, err
	}
	out := pluginv1.ListDownloadsResponse_builder{}.Build()
	var downloads []*pluginv1.Download
	var statuses []*pluginv1.DownloadClientStatus
	var failures []*pluginv1.DownloadClientFailure
	for _, c := range cfg.Clients {
		if ids := req.GetClientIds(); len(ids) > 0 && !slices.Contains(ids, c.Remote) || len(req.GetClientIds()) == 0 && !c.Enabled {
			continue
		}
		list, err := o.downloads(ctx, c)
		if err != nil {
			failures = append(failures, pluginv1.DownloadClientFailure_builder{ClientId: new(c.Remote), Message: new(err.Error())}.Build())
			continue
		}
		downloads = append(downloads, list...)
		if req.GetIncludeStatuses() {
			statuses = append(statuses, pluginv1.DownloadClientStatus_builder{
				ClientId:          new(c.Remote),
				OutputRootFolders: []string{c.path(c.folder(categoryMovies)), c.path(c.folder(categorySeries))},
			}.Build())
		}
	}
	out.SetDownloads(downloads)
	out.SetStatuses(statuses)
	out.SetFailures(failures)
	return out, nil
}

// downloads lists a client's downloads: the drive's offline downloads the
// ledger keeps for it, recording what the drive tells of them, which tells
// of those the drive no longer lists.
func (o *Offline) downloads(ctx context.Context, c OfflineClient) ([]*pluginv1.Download, error) {
	rc, err := o.rc()
	if err != nil {
		return nil, err
	}
	tasks, err := rc.OfflineTasks(ctx, c.Remote)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]rclone.OfflineTask, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	jobs, err := o.Jobs.List(ctx, core.StorageJobOffline)
	if err != nil {
		return nil, err
	}
	var out []*pluginv1.Download
	for _, j := range jobs {
		client, id, _ := strings.Cut(j.Key, "/")
		if client != c.Remote {
			continue
		}
		t, listed := byID[j.Data["task"]]
		if listed {
			data := maps.Clone(j.Data)
			data["name"] = cmp.Or(t.Name, data["name"])
			data["status"] = string(t.Status)
			data["size"] = strconv.FormatInt(t.Size, 10)
			data["error"] = t.Error
			if !maps.Equal(data, j.Data) {
				j.Data = data
				if err := o.Jobs.Put(ctx, &j); err != nil {
					return nil, err
				}
			}
		}
		out = append(out, offlineDownload(c, id, j, t, listed))
	}
	return out, nil
}

// offlineDownload returns a download of a client.
func offlineDownload(c OfflineClient, id string, j core.StorageJob, t rclone.OfflineTask, listed bool) *pluginv1.Download {
	size, _ := strconv.ParseInt(j.Data["size"], 10, 64)
	d := pluginv1.Download_builder{
		ClientId: new(c.Remote), Id: new(id), Title: new(cmp.Or(j.Data["title"], j.Data["name"])),
		Category: new(j.Data["category"]), TotalSize: &size,
		Message: new(j.Data["error"]), CanMoveFiles: new(true),
	}.Build()
	if j.Data["kind"] == "series" {
		d.SetKind(pluginv1.DownloadKind_DOWNLOAD_KIND_SERIES)
	} else {
		d.SetKind(pluginv1.DownloadKind_DOWNLOAD_KIND_MOVIE)
	}
	if name := j.Data["name"]; name != "" {
		d.SetOutputPath(c.path(path.Join(cmp.Or(j.Data["folder"], c.folder(j.Data["category"])), name)))
	}
	status := rclone.OfflineStatus(j.Data["status"])
	switch {
	case !listed && status != rclone.OfflineComplete:
		d.SetStatus(pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED)
		d.SetMessage("the cloud drive no longer lists the download")
		d.SetCanBeRemoved(true)
	case status == rclone.OfflineComplete:
		d.SetStatus(pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED)
		d.SetCanBeRemoved(true)
	case status == rclone.OfflineError:
		d.SetStatus(pluginv1.DownloadStatus_DOWNLOAD_STATUS_FAILED)
		d.SetCanBeRemoved(true)
	case status == rclone.OfflineRunning:
		d.SetStatus(pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING)
	default:
		d.SetStatus(pluginv1.DownloadStatus_DOWNLOAD_STATUS_QUEUED)
	}
	switch d.GetStatus() {
	case pluginv1.DownloadStatus_DOWNLOAD_STATUS_COMPLETED:
		d.SetRemainingSize(0)
	case pluginv1.DownloadStatus_DOWNLOAD_STATUS_DOWNLOADING, pluginv1.DownloadStatus_DOWNLOAD_STATUS_QUEUED:
		left := size - int64(float64(size)*min(max(t.Progress, 0), 100)/100)
		d.SetRemainingSize(left)
		if t.Speed > 0 {
			d.SetRemainingTime(durationpb.New(time.Duration(left/t.Speed) * time.Second))
		}
	}
	return d
}

// GetDownload implements pluginv1connect.DownloadClientProviderServiceClient.
func (o *Offline) GetDownload(ctx context.Context, req *pluginv1.GetDownloadRequest) (*pluginv1.GetDownloadResponse, error) {
	c, err := o.client(ctx, req.GetClientId())
	if err != nil {
		return nil, err
	}
	list, err := o.downloads(ctx, c)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	for _, d := range list {
		if d.GetId() == req.GetDownloadId() {
			return pluginv1.GetDownloadResponse_builder{Download: d}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no download %s in %s", req.GetDownloadId(), c.Remote))
}

// RemoveDownload implements pluginv1connect.DownloadClientProviderServiceClient.
func (o *Offline) RemoveDownload(ctx context.Context, req *pluginv1.RemoveDownloadRequest) (*pluginv1.RemoveDownloadResponse, error) {
	c, err := o.client(ctx, req.GetClientId())
	if err != nil {
		return nil, err
	}
	rc, err := o.rc()
	if err != nil {
		return nil, err
	}
	key := c.Remote + "/" + req.GetDownloadId()
	j, err := o.Jobs.Get(ctx, core.StorageJobOffline, key)
	if errors.Is(err, core.ErrNotFound) {
		return &pluginv1.RemoveDownloadResponse{}, nil
	} else if err != nil {
		return nil, err
	}
	if err := rc.DeleteOffline(ctx, j.Remote, j.Data["task"]); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, offlineError(err)
	}
	if name := j.Data["name"]; req.GetDeleteData() && name != "" {
		st, err := o.Remotes.Open(ctx, j.Remote, cmp.Or(j.Data["folder"], c.folder(j.Data["category"])))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, offlineError(err)
		}
		if err == nil {
			defer func() { _ = st.Close() }()
			if err := st.(*rclone.Storage).RemoveAll(ctx, name); err != nil {
				return nil, offlineError(err)
			}
		}
	}
	if err := o.Jobs.Delete(ctx, core.StorageJobOffline, key); err != nil {
		return nil, err
	}
	return &pluginv1.RemoveDownloadResponse{}, nil
}

// MarkDownloadImported implements
// pluginv1connect.DownloadClientProviderServiceClient: drives have no
// category for imported downloads.
func (o *Offline) MarkDownloadImported(context.Context, *pluginv1.MarkDownloadImportedRequest) (*pluginv1.MarkDownloadImportedResponse, error) {
	return &pluginv1.MarkDownloadImportedResponse{}, nil
}

package nzbget

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of NZBGet.
type settings struct {
	client.Server
	Username string `json:"username"`
	Password string `json:"password"`
	client.Categories
	client.Priorities
	AddPaused bool `json:"add_paused"`
}

func (s *settings) validate() error {
	if err := s.Validate(); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(s.Username) == "" && strings.TrimSpace(s.Password) != "":
		return errors.New("username is required with a password")
	case strings.TrimSpace(s.Password) == "" && strings.TrimSpace(s.Username) != "":
		return errors.New("password is required with a username")
	}
	return nil
}

// The priorities of Radarr's NzbgetPriority.
var priorities = []client.Option{
	{Value: -100, Label: "Very Low"},
	{Value: -50, Label: "Low"},
	{Value: 0, Label: "Normal"},
	{Value: 50, Label: "High"},
	{Value: 100, Label: "Very High"},
	{Value: 900, Label: "Force"},
}

func fields() []client.Field {
	f := client.ServerFields(6789, "")
	f[len(f)-1].HelpText = "Adds a prefix to the NZBGet url, such as http://[host]:[port]/[urlBase]/jsonrpc"
	f = append(f,
		client.Field{Name: "username", Label: "Username", Type: client.FieldText, Default: "nzbget"},
		client.Field{Name: "password", Label: "Password", Type: client.FieldText, Default: "tegbzn6789", Secret: true},
	)
	f = append(f, client.CategoryFields("Category", client.DefaultMovieCategory, client.DefaultTVCategory, false)...)
	f = append(f, client.PriorityFields(priorities, 0)...)
	return append(f, client.Field{
		Name: "add_paused", Label: "Add Paused", Type: client.FieldCheckbox, Default: false,
		HelpText: "This option requires at least NZBGet version 16.0",
	})
}

// Definitions returns the definition of NZBGet.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "nzbget", Name: "NZBGet", Protocol: client.Usenet, Fields: fields(),
		Description: "NZBGet, over its JSON-RPC",
		New:         newNzbget,
	}}
}

func newNzbget(raw []byte, env client.Env) (client.Client, error) {
	s := settings{
		Server:     client.Server{Host: "localhost", Port: 6789},
		Username:   "nzbget",
		Password:   "tegbzn6789",
		Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
	}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	n := &nzbget{settings: s, env: env}
	n.proxy = &proxy{http: env.HTTP, settings: &n.settings, now: env.Now, log: env.Log}
	return n, nil
}

// historyLimit is how many downloads of the history are read, as
// Radarr's default DownloadClientHistoryLimit.
const historyLimit = 60

// The statuses of the history that are success.
var successStatus = []string{"SUCCESS", "NONE"}

// The delete statuses of failed downloads; other deletions are warnings.
var deleteFailedStatus = []string{"HEALTH", "DUPE", "SCAN", "COPY", "BAD"}

// nzbget is the client of Radarr's Nzbget.
type nzbget struct {
	settings settings
	proxy    *proxy
	env      client.Env
}

// Host implements client.Client.
func (n *nzbget) Host() string { return n.settings.Host }

// Add implements client.Client.
func (n *nzbget) Add(ctx context.Context, r *client.Release) (string, error) {
	name := r.FileName
	if name == "" {
		name = client.CleanFileName(r.Title) + ".nzb"
	}
	id, err := n.proxy.downloadNzb(ctx, r.Data, name, n.settings.Category(r.Kind), n.settings.Priority(r.Kind, r.Recent), n.settings.AddPaused)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("NZBGet rejected the NZB for an unknown reason: %s", r.Title)
	}
	return id, nil
}

// kindOf returns the kind of a download by its category, as Radarr's and
// Sonarr's GetItems keep only those in their category.
func (n *nzbget) kindOf(category string) (client.Kind, bool) {
	return n.settings.Match(func(c string) bool { return c == category })
}

// removable sets whether a download can be removed and its files moved:
// failed ones when failed downloads are removed, others when completed
// ones are.
func removable(item *client.Item, opts client.Options) {
	remove := opts.RemoveCompletedDownloads
	if item.Status == client.Failed {
		remove = opts.RemoveFailedDownloads
	}
	item.CanBeRemoved, item.CanMoveFiles = remove, remove
}

// Items implements client.Client.
func (n *nzbget) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	queue, err := n.queueItems(ctx, opts)
	if err != nil {
		return nil, err
	}
	history, err := n.historyItems(ctx, opts)
	if err != nil {
		return nil, err
	}
	return append(queue, history...), nil
}

func (n *nzbget) queueItems(ctx context.Context, opts client.Options) ([]client.Item, error) {
	status, err := n.proxy.globalStatus(ctx)
	if err != nil {
		return nil, err
	}
	queue, err := n.proxy.queue(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	var totalRemaining int64
	for _, q := range queue {
		total := makeInt64(q.FileSizeHi, q.FileSizeLo)
		paused := makeInt64(q.PausedSizeHi, q.PausedSizeLo)
		remaining := makeInt64(q.RemainingSizeHi, q.RemainingSizeLo)
		// Downloads are told by status even out of the categories, so
		// the remaining time of those in them counts all before.
		item := client.Item{Title: q.NzbName, TotalSize: total, Category: q.Category}
		item.ID = strconv.Itoa(q.NzbID)
		if d, ok := droneID(q.Parameters); ok {
			item.ID = d
		}
		if status.DownloadPaused || (remaining == paused && remaining != 0) {
			item.Status = client.Paused
			item.RemainingSize = remaining
		} else {
			item.Status = client.Downloading
			if q.ActiveDownloads == 0 && remaining != 0 {
				item.Status = client.Queued
			}
			item.RemainingSize = remaining - paused
			if status.DownloadRate != 0 {
				eta := time.Duration((totalRemaining+item.RemainingSize)/status.DownloadRate) * time.Second
				item.RemainingTime = &eta
				totalRemaining += item.RemainingSize
			}
		}
		kind, ok := n.kindOf(q.Category)
		if !ok {
			continue
		}
		item.Kind = kind
		removable(&item, opts)
		items = append(items, item)
	}
	return items, nil
}

func (n *nzbget) historyItems(ctx context.Context, opts client.Options) ([]client.Item, error) {
	history, err := n.proxy.history(ctx)
	if err != nil {
		return nil, err
	}
	history = history[:min(len(history), historyLimit)]
	var items []client.Item
	for _, h := range history {
		kind, ok := n.kindOf(h.Category)
		if !ok {
			continue
		}
		dir := h.FinalDir
		if strings.TrimSpace(dir) == "" {
			dir = h.DestDir
		}
		var zero time.Duration
		item := client.Item{
			ID: strconv.Itoa(h.ID), Title: h.Name, TotalSize: makeInt64(h.FileSizeHi, h.FileSizeLo),
			OutputPath: dir, Category: h.Category, Kind: kind, Status: client.Completed, RemainingTime: &zero,
			Message: fmt.Sprintf("PAR Status: %s - Unpack Status: %s - Move Status: %s - Script Status: %s - Delete Status: %s - Mark Status: %s",
				h.ParStatus, h.UnpackStatus, h.MoveStatus, h.ScriptStatus, h.DeleteStatus, h.MarkStatus),
		}
		if d, ok := droneID(h.Parameters); ok {
			item.ID = d
		}
		if h.DeleteStatus == "MANUAL" {
			// Deleted by the user: failed when marked bad, else left out.
			if h.MarkStatus == "BAD" {
				item.Status = client.Failed
				removable(&item, opts)
				items = append(items, item)
			}
			continue
		}
		item.Status = historyStatus(&h)
		removable(&item, opts)
		items = append(items, item)
	}
	return items, nil
}

// historyStatus maps the post-processing statuses of a download in the
// history, the last that applies winning.
func historyStatus(h *historyItem) client.Status {
	status := client.Completed
	if !slices.Contains(successStatus, h.ParStatus) {
		status = client.Failed
	}
	if h.UnpackStatus == "SPACE" {
		status = client.Warning
	} else if !slices.Contains(successStatus, h.UnpackStatus) {
		status = client.Failed
	}
	if !slices.Contains(successStatus, h.MoveStatus) {
		status = client.Warning
	}
	if !slices.Contains(successStatus, h.ScriptStatus) {
		status = client.Failed
	}
	if !slices.Contains(successStatus, h.DeleteStatus) && strings.TrimSpace(h.DeleteStatus) != "" {
		if slices.Contains(deleteFailedStatus, h.DeleteStatus) {
			status = client.Failed
		} else {
			status = client.Warning
		}
	}
	return status
}

// Remove implements client.Client.
func (n *nzbget) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if deleteData {
		client.DeleteItemData(ctx, n.env.Log, item)
	}
	return n.proxy.remove(ctx, item.ID)
}

// category is a category of NZBGet's configuration.
type category struct {
	Name    string
	DestDir string
}

// categories reads the categories of NZBGet's configuration, with the
// folder each puts downloads in.
func categories(config map[string]string) []category {
	var out []category
	for i := 1; i < 100; i++ {
		name, ok := config[fmt.Sprintf("Category%d.Name", i)]
		if !ok {
			break
		}
		dest := config[fmt.Sprintf("Category%d.DestDir", i)]
		if strings.TrimSpace(dest) == "" {
			dest = strings.ReplaceAll(config["DestDir"], "${MainDir}", config["MainDir"])
			if appendDir, ok := config["AppendCategoryDir"]; !ok || appendDir == "yes" {
				dest = joinPath(dest, name)
			}
		}
		out = append(out, category{Name: name, DestDir: dest})
	}
	return out
}

// Info implements client.Client.
func (n *nzbget) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: n.settings.IsLocalhost()}
	config, err := n.proxy.config(ctx)
	if err != nil {
		return info, err
	}
	cats := categories(config)
	for _, k := range client.Kinds {
		i := slices.IndexFunc(cats, func(c category) bool { return c.Name == n.settings.Category(k) })
		if i >= 0 && !slices.Contains(info.OutputRootFolders, cats[i].DestDir) {
			info.OutputRootFolders = append(info.OutputRootFolders, cats[i].DestDir)
		}
	}
	return info, nil
}

// Test implements client.Client.
func (n *nzbget) Test(ctx context.Context) error {
	if err := n.testConnection(ctx); err != nil {
		return err
	}
	config, err := n.proxy.config(ctx)
	if err != nil {
		return err
	}
	cats := categories(config)
	for _, k := range client.Kinds {
		c := n.settings.Category(k)
		if strings.TrimSpace(c) != "" && !slices.ContainsFunc(cats, func(x category) bool { return x.Name == c }) {
			return fmt.Errorf("category %q does not exist; the category you entered doesn't exist in NZBGet, create it in NZBGet first", c)
		}
	}
	keep, ok := config["KeepHistory"]
	if !ok {
		keep = "7"
	}
	v, err := parseDigits(keep)
	switch {
	case err != nil || v == 0:
		return errors.New("NZBGet setting KeepHistory should be greater than 0; set to 0, it prevents Mavio from seeing completed downloads")
	case v > 25000:
		return errors.New("NZBGet setting KeepHistory should be less than 25000; it is set too high")
	}
	return nil
}

// parseDigits parses a number of digits alone, as .NET's int.TryParse
// with NumberStyles.None.
func parseDigits(s string) (int, error) {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		return 0, fmt.Errorf("not a number: %q", s)
	}
	return strconv.Atoi(s)
}

func (n *nzbget) testConnection(ctx context.Context) error {
	s, err := n.proxy.getVersion(ctx)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "authentication failed") {
			return fmt.Errorf("authentication failure: %w", err)
		}
		n.env.Log.ErrorContext(ctx, "unable to connect to NZBGet", "err", err)
		return fmt.Errorf("unable to connect to NZBGet: %w", err)
	}
	version := strings.Split(s, "-")[0]
	v, err := parseVersion(version)
	if err != nil {
		n.env.Log.ErrorContext(ctx, "unable to connect to NZBGet", "err", err)
		return fmt.Errorf("unable to connect to NZBGet: %w", err)
	}
	if slices.Compare(v, []int{12, 0}) < 0 {
		return fmt.Errorf("NZBGet version should be at least 12.0; version reported is %s", version)
	}
	return nil
}

// parseVersion parses a version as .NET's Version.Parse: two to four
// numbers separated by dots.
func parseVersion(s string) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, fmt.Errorf("unable to parse version %q", s)
	}
	v := make([]int, len(parts))
	for i, p := range parts {
		n, err := parseDigits(p)
		if err != nil {
			return nil, fmt.Errorf("unable to parse version %q", s)
		}
		v[i] = n
	}
	return v, nil
}

// joinPath joins a folder of the client's machine, with either slashes,
// and a name, as Path.Combine.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, `\`) {
		return dir + name
	}
	sep := "/"
	if strings.Contains(dir, `\`) && !strings.Contains(dir, "/") {
		sep = `\`
	}
	return dir + sep + name
}

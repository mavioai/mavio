package nzbvortex

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

// settings are the settings of NZBVortex, which is always reached over
// HTTPS.
type settings struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	URLBase string `json:"url_base"`
	APIKey  string `json:"api_key"`
	client.Categories
	client.Priorities
}

func (s *settings) server() client.Server {
	return client.Server{Host: s.Host, Port: s.Port, UseSSL: true, URLBase: s.URLBase}
}

func (s *settings) validate() error {
	if err := s.server().Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("API key is required")
	}
	return nil
}

// The priorities of Radarr's NzbVortexPriority.
var priorities = []client.Option{
	{Value: -1, Label: "Low"},
	{Value: 0, Label: "Normal"},
	{Value: 1, Label: "High"},
}

func fields() []client.Field {
	f := slices.DeleteFunc(client.ServerFields(4321, ""), func(f client.Field) bool { return f.Name == "use_ssl" })
	f[len(f)-1].HelpText = "Adds a prefix to the NZBVortex url, such as https://[host]:[port]/[urlBase]/api"
	f = append(f, client.Field{Name: "api_key", Label: "API Key", Type: client.FieldText, Secret: true})
	f = append(f, client.CategoryFields("Group", client.DefaultMovieCategory, client.DefaultTVCategory, false)...)
	return append(f, client.PriorityFields(priorities, 0)...)
}

// Definitions returns the definition of NZBVortex.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "nzbvortex", Name: "NZBVortex", Protocol: client.Usenet, Fields: fields(),
		Description: "NZBVortex, over its API",
		New:         newNzbVortex,
	}}
}

func newNzbVortex(raw []byte, env client.Env) (client.Client, error) {
	s := settings{
		Host: "localhost", Port: 4321,
		Categories: client.Categories{MovieCategory: client.DefaultMovieCategory, TVCategory: client.DefaultTVCategory},
	}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	n := &nzbVortex{settings: s, env: env}
	n.proxy = &proxy{http: env.HTTP, settings: &n.settings}
	return n, nil
}

// doneLimit is how many finished downloads the queue lists.
const doneLimit = 30

// nzbVortex is the client of Radarr's NzbVortex.
type nzbVortex struct {
	settings settings
	proxy    *proxy
	env      client.Env
}

// Host implements client.Client.
func (n *nzbVortex) Host() string { return n.settings.Host }

// Add implements client.Client.
func (n *nzbVortex) Add(ctx context.Context, r *client.Release) (string, error) {
	name := r.FileName
	if name == "" {
		name = client.CleanFileName(r.Title) + ".nzb"
	}
	id, err := n.proxy.downloadNzb(ctx, r.Data, name, n.settings.Priority(r.Kind, r.Recent), n.settings.Category(r.Kind))
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("failed to add nzb %s", name)
	}
	return id, nil
}

// queue lists the downloads of the groups of both kinds, as Radarr and
// Sonarr list those of theirs: all of them when a kind has no group.
func (n *nzbVortex) queue(ctx context.Context, limit int) ([]queueItem, error) {
	var groups []string
	for _, k := range client.Kinds {
		g := n.settings.Category(k)
		if strings.TrimSpace(g) == "" {
			groups = []string{""}
			break
		}
		if !slices.Contains(groups, g) {
			groups = append(groups, g)
		}
	}
	var queue []queueItem
	for _, g := range groups {
		q, err := n.proxy.queue(ctx, limit, g)
		if err != nil {
			return nil, err
		}
		queue = append(queue, q...)
	}
	return queue, nil
}

// kindOf returns the kind of a download by its group: that of a kind
// with no group matches all.
func (n *nzbVortex) kindOf(group string) (client.Kind, bool) {
	return n.settings.Match(func(c string) bool { return strings.TrimSpace(c) == "" || c == group })
}

// Items implements client.Client.
func (n *nzbVortex) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	queue, err := n.queue(ctx, doneLimit)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for i := range queue {
		q := &queue[i]
		kind, ok := n.kindOf(q.GroupName)
		if !ok {
			continue
		}
		item := client.Item{
			ID: q.AddUUID, Category: q.GroupName, Kind: kind, Title: q.UITitle,
			TotalSize: q.TotalDownloadSize, RemainingSize: q.TotalDownloadSize - q.DownloadedSize,
			Encrypted: q.State == statePasswordRequest,
		}
		if item.ID == "" {
			item.ID = strconv.Itoa(q.ID)
		}
		switch {
		case q.IsPaused:
			item.Status = client.Paused
		case q.State == stateWaiting:
			item.Status = client.Queued
		case q.State == stateDone:
			item.Status = client.Completed
		case q.State == stateUncompressFailed || q.State == stateCheckFailedDataCorrupt || q.State == stateBadlyEncoded:
			item.Status = client.Failed
		default:
			item.Status = client.Downloading
		}
		if err := n.outputPath(ctx, q, &item); err != nil {
			return nil, err
		}
		if item.Status == client.Completed {
			var zero time.Duration
			item.RemainingTime = &zero
		}
		remove := opts.RemoveCompletedDownloads
		if item.Status == client.Failed {
			remove = opts.RemoveFailedDownloads
		}
		item.CanBeRemoved, item.CanMoveFiles = remove, remove
		items = append(items, item)
	}
	return items, nil
}

// outputPath sets where a download's files are: its destination when it
// is the download's job folder, else, once done, its file in the folder,
// warning when there are several.
func (n *nzbVortex) outputPath(ctx context.Context, q *queueItem, item *client.Item) error {
	dest := q.DestinationPath
	if dest != "" && pathBase(dest) == q.UITitle {
		item.OutputPath = dest
		return nil
	}
	if q.State != stateDone {
		return nil
	}
	files, err := n.proxy.files(ctx, q.ID)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		item.OutputPath = dest
		return nil
	}
	if len(files) > 1 {
		item.Status = client.Warning
		item.Message = "Download contains multiple files and is not in a job folder: " + dest
		n.env.Log.DebugContext(ctx, item.Message)
	}
	item.OutputPath = joinPath(dest, files[0].FileName)
	return nil
}

// Remove implements client.Client: by NZBVortex's ID, else by the add
// UUID.
func (n *nzbVortex) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if id, err := strconv.Atoi(item.ID); err == nil {
		return n.proxy.remove(ctx, id, deleteData)
	}
	queue, err := n.queue(ctx, doneLimit)
	if err != nil {
		return err
	}
	for _, q := range queue {
		if q.AddUUID == item.ID {
			return n.proxy.remove(ctx, q.ID, deleteData)
		}
	}
	return nil
}

// Info implements client.Client.
func (n *nzbVortex) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: n.settings.server().IsLocalhost()}, nil
}

// Test implements client.Client.
func (n *nzbVortex) Test(ctx context.Context) error {
	if _, err := n.proxy.version(ctx); err != nil {
		n.env.Log.ErrorContext(ctx, "unable to connect to NZBVortex", "err", err)
		return fmt.Errorf("unable to connect to NZBVortex: %w", err)
	}
	level, err := n.proxy.apiVersion(ctx)
	if err == nil {
		var v []int
		if v, err = parseVersion(level); err == nil && slices.Compare(v, []int{2, 3}) < 0 {
			return fmt.Errorf("NZBVortex version should be at least 2.3; version reported is %s", level)
		}
	}
	if err != nil {
		n.env.Log.ErrorContext(ctx, "unable to connect to NZBVortex", "err", err)
		return fmt.Errorf("unable to connect to NZBVortex: %w", err)
	}
	if _, err := n.proxy.queue(ctx, 1, n.settings.Category(client.Movie)); err != nil {
		if errors.Is(err, errAuth) {
			return fmt.Errorf("API key incorrect: %w", err)
		}
		return err
	}
	groups, err := n.proxy.groups(ctx)
	if err != nil {
		return err
	}
	for _, k := range client.Kinds {
		if g := n.settings.Category(k); strings.TrimSpace(g) != "" && !slices.Contains(groups, g) {
			return fmt.Errorf("group %q does not exist; the group you entered doesn't exist in NZBVortex, create it in NZBVortex first", g)
		}
	}
	return nil
}

// parseVersion parses a version as .NET's Version: two to four numbers
// separated by dots.
func parseVersion(s string) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, fmt.Errorf("unable to parse version %q", s)
	}
	v := make([]int, len(parts))
	for i, p := range parts {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return nil, fmt.Errorf("unable to parse version %q", s)
		}
		v[i] = x
	}
	return v, nil
}

// pathBase returns the last element of a path of the client's machine,
// with either slashes.
func pathBase(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
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

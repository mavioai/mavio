package aria2

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// settings are the settings of Aria2.
type settings struct {
	client.Server
	SecretToken string `json:"secret_token"`
	client.Directories
}

func fields() []client.Field {
	f := client.ServerFields(6800, "/rpc")
	f[len(f)-1].Label = "XML RPC Path"
	f[len(f)-1].Advanced = false
	f[len(f)-1].HelpText = "The path of aria2's XML-RPC, such as http://[host]:[port]/[rpcPath]"
	f = append(f, client.Field{Name: "secret_token", Label: "Secret Token", Type: client.FieldText, Default: "MySecretToken", Secret: true})
	return append(f, client.DirectoryFields("Aria2")...)
}

// Definitions returns the definition of Aria2.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "aria2", Name: "Aria2", Protocol: client.Torrent, Fields: fields(),
		Description: "aria2, over its XML-RPC",
		New: func(raw []byte, env client.Env) (client.Client, error) {
			s := settings{
				Server:      client.Server{Host: "localhost", Port: 6800, URLBase: "/rpc"},
				SecretToken: "MySecretToken",
			}
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := s.Validate(); err != nil {
				return nil, err
			}
			a := &aria2{settings: s, env: env, retryDelay: 500 * time.Millisecond}
			a.proxy = &proxy{http: env.HTTP, settings: &a.settings}
			return a, nil
		},
	}}
}

// aria2 is the client of Radarr's Aria2.
type aria2 struct {
	settings settings
	proxy    *proxy
	env      client.Env
	// retryDelay is how long to wait between checks for an added torrent.
	retryDelay time.Duration
}

// Host implements client.Client.
func (a *aria2) Host() string { return a.settings.Host }

// Add implements client.Client.
func (a *aria2) Add(ctx context.Context, r *client.Release) (string, error) {
	return client.AddTorrent(ctx, a.env.Log, a, r)
}

// AddMagnet implements client.TorrentAdder.
func (a *aria2) AddMagnet(ctx context.Context, r *client.Release, hash, link string) (string, error) {
	gid, err := a.proxy.addMagnet(ctx, link, a.settings.Directory(r.Kind))
	if err != nil {
		return "", err
	}
	ok, err := a.waitForTorrent(ctx, gid, hash)
	if err != nil {
		return "", err
	}
	if !ok {
		a.env.Log.WarnContext(ctx, "Aria2 could not add magnet within 5 seconds, download may remain stuck", "magnet", link)
		return hash, nil
	}
	a.env.Log.DebugContext(ctx, "Aria2 added magnet", "hash", hash, "gid", gid)
	return hash, nil
}

// AddTorrentFile implements client.TorrentAdder.
func (a *aria2) AddTorrentFile(ctx context.Context, r *client.Release, hash, fileName string, data []byte) (string, error) {
	gid, err := a.proxy.addTorrent(ctx, data, a.settings.Directory(r.Kind))
	if err != nil {
		return "", err
	}
	ok, err := a.waitForTorrent(ctx, gid, hash)
	if err != nil {
		return "", err
	}
	if !ok {
		a.env.Log.WarnContext(ctx, "Aria2 could not add torrent within 5 seconds, download may remain stuck", "file", fileName)
	}
	return hash, nil
}

// waitForTorrent checks 10 times, retryDelay apart, that the download of
// a GID is of the hash.
func (a *aria2) waitForTorrent(ctx context.Context, gid, hash string) (bool, error) {
	const tries = 10
	for i := range tries {
		s, err := a.proxy.status(ctx, gid)
		if err != nil {
			return false, err
		}
		if strings.EqualFold(s.InfoHash, hash) {
			return true, nil
		}
		if i == tries-1 {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(a.retryDelay):
		}
	}
	a.env.Log.DebugContext(ctx, "could not find the torrent", "hash", hash, "tries", tries, "interval", a.retryDelay)
	return false, nil
}

// kindOf returns the kind of a download by its directory, and whether it
// is of either; zero when it is of both. A kind without a directory takes
// every download.
func (a *aria2) kindOf(output string) (client.Kind, bool) {
	of := func(k client.Kind) bool {
		dir := a.settings.Directory(k)
		return strings.TrimSpace(dir) == "" || containsPath(dir, output)
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
func (a *aria2) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	downloads, err := a.proxy.downloads(ctx)
	if err != nil {
		return nil, err
	}
	var items []client.Item
	for i := range downloads {
		d := &downloads[i]
		// Skip the downloads of magnet links' metadata, and those not of
		// torrents.
		if len(d.Files) > 0 && strings.Contains(d.Files[0], "[METADATA]") || d.InfoHash == "" {
			continue
		}
		output := outputPath(d.Files)
		kind, ok := a.kindOf(output)
		if !ok {
			continue
		}
		st := client.Failed
		switch d.Status {
		case "active":
			st = client.Downloading
			if d.CompletedLength == d.TotalLength {
				st = client.Completed
			}
		case "waiting":
			st = client.Queued
		case "paused":
			st = client.Paused
		case "complete":
			st = client.Completed
		}
		a.env.Log.DebugContext(ctx, "aria2 status", "hash", d.InfoHash, "gid", d.GID, "status", st,
			"total", d.TotalLength, "completed", d.CompletedLength)
		var ratio float64
		if d.TotalLength > 0 {
			ratio = float64(d.UploadLength) / float64(d.TotalLength)
		}
		item := client.Item{
			ID: strings.ToUpper(d.InfoHash), Title: d.Name, Kind: kind, Message: d.ErrorMessage,
			OutputPath: output, TotalSize: d.TotalLength, RemainingSize: d.TotalLength - d.CompletedLength,
			SeedRatio: &ratio, Status: st,
			CanBeRemoved: opts.RemoveCompletedDownloads && d.Status == "complete",
		}
		if d.DownloadSpeed != 0 {
			remaining := time.Duration((d.TotalLength-d.CompletedLength)/d.DownloadSpeed) * time.Second
			item.RemainingTime = &remaining
		}
		items = append(items, item)
	}
	return items, nil
}

// outputPath is where a download's files are: its file, or the longest
// common folder of its files.
func outputPath(files []string) string {
	switch len(files) {
	case 0:
		return ""
	case 1:
		return files[0]
	}
	prefix := files[0]
	for _, f := range files[1:] {
		n := min(len(prefix), len(f))
		i := 0
		for i < n && prefix[i] == f[i] {
			i++
		}
		prefix = prefix[:i]
	}
	if i := strings.LastIndexAny(prefix, `/\`); i >= 0 {
		return prefix[:i]
	}
	return ""
}

// Remove implements client.Client: aria2 does not delete files, which
// Mavio deletes once the download is removed.
func (a *aria2) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	downloads, err := a.proxy.downloads(ctx)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(downloads, func(d status) bool { return strings.EqualFold(d.InfoHash, item.ID) })
	if i < 0 {
		a.env.Log.ErrorContext(ctx, "Aria2 could not find the download for deletion", "hash", item.ID)
		return nil
	}
	d := downloads[i]
	a.env.Log.DebugContext(ctx, "Aria2 removing download", "hash", item.ID, "gid", d.GID)
	var ok bool
	switch d.Status {
	case "complete", "error", "removed":
		ok, err = a.proxy.removeResult(ctx, d.GID)
	default:
		ok, err = a.proxy.remove(ctx, d.GID)
	}
	if err != nil {
		return err
	}
	if !ok {
		a.env.Log.ErrorContext(ctx, "Aria2 error while deleting the download", "hash", item.ID)
		return nil
	}
	if deleteData {
		client.DeleteItemData(ctx, a.env.Log, item)
	}
	return nil
}

// Info implements client.Client.
func (a *aria2) Info(ctx context.Context) (client.Info, error) {
	info := client.Info{IsLocalhost: strings.Contains(a.settings.Host, "127.0.0.1") || strings.Contains(a.settings.Host, "localhost")}
	var globalDir *string
	for _, k := range client.Kinds {
		dir := a.settings.Directory(k)
		if strings.TrimSpace(dir) == "" {
			if globalDir == nil {
				g, err := a.proxy.globals(ctx)
				if err != nil {
					return info, err
				}
				d := g["dir"]
				globalDir = &d
			}
			dir = *globalDir
		}
		if !slices.Contains(info.OutputRootFolders, dir) {
			info.OutputRootFolders = append(info.OutputRootFolders, dir)
		}
	}
	return info, nil
}

// Test implements client.Client.
func (a *aria2) Test(ctx context.Context) error {
	version, err := a.proxy.version(ctx)
	if err != nil {
		a.env.Log.ErrorContext(ctx, "failed to test Aria2", "err", err)
		return fmt.Errorf("unable to connect to Aria2: %w", err)
	}
	v, ok := parseVersion(version)
	if !ok {
		return fmt.Errorf("unable to connect to Aria2: unable to parse version %q", version)
	}
	if slices.Compare(v, []int{1, 34, 0}) < 0 {
		return fmt.Errorf("the Aria2 version %s is not supported; 1.34.0 or later is required", version)
	}
	return nil
}

// parseVersion reads a version such as "1.37.0", as .NET's Version.
func parseVersion(s string) ([]int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, false
	}
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
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

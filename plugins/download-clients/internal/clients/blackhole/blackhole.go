package blackhole

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// scanGracePeriod is how long a download is left unchanged before it is
// taken as completed, as Radarr's ScanGracePeriod.
const scanGracePeriod = 30 * time.Second

// category is the category of every download: the watch folder is
// shared by movies and series, and Radarr needs one to track them.
const category = "mavio"

// torrentSettings are Radarr's TorrentBlackholeSettings.
type torrentSettings struct {
	TorrentFolder       string `json:"torrent_folder"`
	WatchFolder         string `json:"watch_folder"`
	SaveMagnetFiles     bool   `json:"save_magnet_files"`
	MagnetFileExtension string `json:"magnet_file_extension"`
	ReadOnly            bool   `json:"read_only"`
}

// usenetSettings are Radarr's UsenetBlackholeSettings.
type usenetSettings struct {
	NzbFolder   string `json:"nzb_folder"`
	WatchFolder string `json:"watch_folder"`
}

// validPath checks a folder setting as Radarr's IsValidPath: a rooted
// path.
func validPath(setting, folder string) error {
	if strings.TrimSpace(folder) == "" || !filepath.IsAbs(folder) {
		return fmt.Errorf("%s: invalid path %q", setting, folder)
	}
	return nil
}

func watchFolderField() client.Field {
	return client.Field{
		Name: "watch_folder", Label: "Watch Folder", Type: client.FieldText,
		HelpText: "Folder from which completed downloads of movies and series are imported",
	}
}

// Definitions returns the definitions of Torrent Blackhole and Usenet
// Blackhole.
func Definitions() []client.Definition {
	return []client.Definition{
		{
			ID: "torrent-blackhole", Name: "Torrent Blackhole", Protocol: client.Torrent, PrefersTorrentFile: true,
			Description: "Saves torrent files to a folder another client watches, and imports what it finishes in a watch folder",
			Fields: []client.Field{
				{
					Name: "torrent_folder", Label: "Torrent Folder", Type: client.FieldText,
					HelpText: "Folder in which Mavio will store the .torrent file",
				},
				watchFolderField(),
				{
					Name: "save_magnet_files", Label: "Save Magnet Files", Type: client.FieldCheckbox, Default: false,
					HelpText: "Save the magnet link if no .torrent file is available (only useful if the download client supports magnets saved to a file)",
				},
				{
					Name: "magnet_file_extension", Label: "Save Magnet Files Extension", Type: client.FieldText, Default: ".magnet",
					HelpText: "Extension to use for magnet links, defaults to '.magnet'",
				},
				{
					Name: "read_only", Label: "Read Only", Type: client.FieldCheckbox, Default: true,
					HelpText: "Instead of moving files this will instruct Mavio to copy or hardlink (depending on settings/system configuration)",
				},
			},
			New: newTorrent,
		},
		{
			ID: "usenet-blackhole", Name: "Usenet Blackhole", Protocol: client.Usenet,
			Description: "Saves NZB files to a folder another client watches, and imports what it finishes in a watch folder",
			Fields: []client.Field{
				{
					Name: "nzb_folder", Label: "Nzb Folder", Type: client.FieldText,
					HelpText: "Folder in which Mavio will store the .nzb file",
				},
				watchFolderField(),
			},
			New: newUsenet,
		},
	}
}

// base is what both blackholes share: the watch folder they list.
type base struct {
	env         client.Env
	watchFolder string
	scanner     *scanner
	grace       time.Duration
}

func newBase(env client.Env, watchFolder string) base {
	return base{env: env, watchFolder: watchFolder, scanner: newScanner(env), grace: scanGracePeriod}
}

// torrent is Radarr's TorrentBlackhole.
type torrent struct {
	base
	settings torrentSettings
}

func newTorrent(raw []byte, env client.Env) (client.Client, error) {
	s := torrentSettings{MagnetFileExtension: ".magnet", ReadOnly: true}
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := validPath("torrent folder", s.TorrentFolder); err != nil {
		return nil, err
	}
	if err := validPath("watch folder", s.WatchFolder); err != nil {
		return nil, err
	}
	if strings.TrimSpace(s.MagnetFileExtension) == "" {
		return nil, errors.New("magnet file extension is required")
	}
	return &torrent{base: newBase(env, s.WatchFolder), settings: s}, nil
}

// usenet is Radarr's UsenetBlackhole.
type usenet struct {
	base
	settings usenetSettings
}

func newUsenet(raw []byte, env client.Env) (client.Client, error) {
	var s usenetSettings
	if err := client.Decode(raw, &s); err != nil {
		return nil, err
	}
	if err := validPath("nzb folder", s.NzbFolder); err != nil {
		return nil, err
	}
	if err := validPath("watch folder", s.WatchFolder); err != nil {
		return nil, err
	}
	return &usenet{base: newBase(env, s.WatchFolder), settings: s}, nil
}

// writeFile writes a file into a folder, confined to it.
func writeFile(folder, name string, data []byte) (string, error) {
	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", fmt.Errorf("open folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.WriteFile(name, data, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", name, err)
	}
	return filepath.Join(folder, name), nil
}

// Add implements client.Client. The download has no ID until it shows up
// in the watch folder, so none is returned.
func (t *torrent) Add(ctx context.Context, r *client.Release) (string, error) {
	if _, err := client.AddTorrent(ctx, t.env.Log, t, r); err != nil {
		return "", err
	}
	return "", nil
}

// AddMagnet implements client.TorrentAdder: the link is saved to a file
// when SaveMagnetFiles is set.
func (t *torrent) AddMagnet(ctx context.Context, r *client.Release, _, link string) (string, error) {
	if !t.settings.SaveMagnetFiles {
		return "", fmt.Errorf("blackhole does not support magnet links: %w", client.ErrMagnetNotSupported)
	}
	name := client.CleanFileName(r.Title) + "." + strings.Trim(t.settings.MagnetFileExtension, ".")
	p, err := writeFile(t.settings.TorrentFolder, name, []byte(link))
	if err != nil {
		return "", err
	}
	t.env.Log.DebugContext(ctx, "saving magnet link succeeded", "path", p)
	return "", nil
}

// AddTorrentFile implements client.TorrentAdder.
func (t *torrent) AddTorrentFile(ctx context.Context, r *client.Release, _, _ string, data []byte) (string, error) {
	p, err := writeFile(t.settings.TorrentFolder, client.CleanFileName(r.Title)+".torrent", data)
	if err != nil {
		return "", err
	}
	t.env.Log.DebugContext(ctx, "torrent download succeeded", "path", p)
	return "", nil
}

// Add implements client.Client. The download has no ID until it shows up
// in the watch folder, so none is returned.
func (u *usenet) Add(ctx context.Context, r *client.Release) (string, error) {
	if len(r.Data) == 0 {
		return "", errors.New("the release has no NZB")
	}
	p, err := writeFile(u.settings.NzbFolder, client.CleanFileName(r.Title)+".nzb", r.Data)
	if err != nil {
		return "", err
	}
	u.env.Log.DebugContext(ctx, "NZB download succeeded", "path", p)
	return "", nil
}

// items lists the watch folder's downloads; canMoveFiles is whether
// their files may be moved rather than copied.
func (b *base) items(ctx context.Context, opts client.Options, canMoveFiles bool) ([]client.Item, error) {
	found, err := b.scanner.items(ctx, b.watchFolder, b.grace)
	if err != nil {
		return nil, err
	}
	items := make([]client.Item, 0, len(found))
	for _, w := range found {
		items = append(items, client.Item{
			ID: w.ID, Title: w.Title, Category: category,
			TotalSize: w.TotalSize, RemainingTime: w.RemainingTime,
			OutputPath: w.OutputPath, Status: w.Status,
			CanMoveFiles: canMoveFiles, CanBeRemoved: opts.RemoveCompletedDownloads,
		})
	}
	return items, nil
}

// Items implements client.Client.
func (t *torrent) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	return t.items(ctx, opts, !t.settings.ReadOnly)
}

// Items implements client.Client.
func (u *usenet) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	return u.items(ctx, opts, true)
}

// Remove implements client.Client: the download's files are deleted, if
// in the watch folder.
func (b *base) Remove(ctx context.Context, item client.Item, deleteData bool) error {
	if !deleteData {
		return fmt.Errorf("blackhole cannot remove a download without deleting its data as well: %w", client.ErrNotSupported)
	}
	if item.OutputPath == "" {
		return nil
	}
	rel, err := filepath.Rel(b.watchFolder, item.OutputPath)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return fmt.Errorf("%q is not in the watch folder %q", item.OutputPath, b.watchFolder)
	}
	root, err := os.OpenRoot(b.watchFolder)
	if err != nil {
		return fmt.Errorf("open watch folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.RemoveAll(rel); err != nil {
		b.env.Log.WarnContext(ctx, "delete download data", "download", item.Title, "path", item.OutputPath, "err", err)
	}
	return nil
}

// Info implements client.Client.
func (b *base) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: true, OutputRootFolders: []string{b.watchFolder}}, nil
}

// Host implements client.Client: blackholes are folders.
func (b *base) Host() string { return "" }

// Test implements client.Client.
func (t *torrent) Test(context.Context) error {
	if err := client.TestFolder(t.settings.TorrentFolder, "torrent folder", true); err != nil {
		return err
	}
	return client.TestFolder(t.settings.WatchFolder, "watch folder", true)
}

// Test implements client.Client.
func (u *usenet) Test(context.Context) error {
	if err := client.TestFolder(u.settings.NzbFolder, "nzb folder", true); err != nil {
		return err
	}
	return client.TestFolder(u.settings.WatchFolder, "watch folder", true)
}

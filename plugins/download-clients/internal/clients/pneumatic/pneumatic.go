package pneumatic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/blackhole/disk"
)

// settings are Radarr's PneumaticSettings.
type settings struct {
	NzbFolder  string `json:"nzb_folder"`
	StrmFolder string `json:"strm_folder"`
}

// validPath checks a folder setting as Radarr's IsValidPath: a rooted
// path.
func validPath(setting, folder string) error {
	if strings.TrimSpace(folder) == "" || !filepath.IsAbs(folder) {
		return fmt.Errorf("%s: invalid path %q", setting, folder)
	}
	return nil
}

// Definitions returns the definition of Pneumatic.
func Definitions() []client.Definition {
	return []client.Definition{{
		ID: "pneumatic", Name: "Pneumatic", Protocol: client.Usenet,
		Description: "Saves NZB files and .strm files for Kodi's Pneumatic add-on",
		Fields: []client.Field{
			{
				Name: "nzb_folder", Label: "Nzb Folder", Type: client.FieldText,
				HelpText: "This folder will need to be reachable from XBMC",
			},
			{
				Name: "strm_folder", Label: "Strm Folder", Type: client.FieldText,
				HelpText: ".strm files in this folder will be imported by the drone",
			},
		},
		New: func(raw []byte, env client.Env) (client.Client, error) {
			var s settings
			if err := client.Decode(raw, &s); err != nil {
				return nil, err
			}
			if err := validPath("nzb folder", s.NzbFolder); err != nil {
				return nil, err
			}
			if err := validPath("strm folder", s.StrmFolder); err != nil {
				return nil, err
			}
			return &pneumatic{settings: s, env: env, locked: disk.IsLocked}, nil
		},
	}}
}

// pneumatic is Radarr's Pneumatic.
type pneumatic struct {
	settings settings
	env      client.Env
	// locked reports whether another process holds a file.
	locked func(path string) bool
}

// writeFile writes a file into a folder, confined to it, and returns its
// path and details.
func writeFile(folder, name string, data []byte) (string, fs.FileInfo, error) {
	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", nil, fmt.Errorf("open folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.WriteFile(name, data, 0o644); err != nil {
		return "", nil, fmt.Errorf("write %s: %w", name, err)
	}
	fi, err := root.Stat(name)
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(folder, name), fi, nil
}

// downloadID is a .strm file's ID: its name and last write as Radarr's
// ticks.
func downloadID(fi fs.FileInfo) string {
	return fi.Name() + "_" + strconv.FormatInt(disk.Ticks(fi.ModTime()), 10)
}

// Add implements client.Client: the NZB goes to the NZB folder and a
// .strm file of it to the strm folder, which is the download.
func (p *pneumatic) Add(ctx context.Context, r *client.Release) (string, error) {
	if r.FullSeason {
		// Sonarr's Pneumatic.Download.
		return "", fmt.Errorf("full season releases are not supported with Pneumatic: %w", client.ErrNotSupported)
	}
	if len(r.Data) == 0 {
		return "", errors.New("the release has no NZB")
	}
	title := client.CleanFileName(r.Title)
	nzbFile, _, err := writeFile(p.settings.NzbFolder, title+".nzb", r.Data)
	if err != nil {
		return "", err
	}
	p.env.Log.DebugContext(ctx, "NZB download succeeded", "path", nzbFile)
	contents := fmt.Sprintf("plugin://plugin.program.pneumatic/?mode=strm&type=add_file&nzb=%s&nzbname=%s", nzbFile, title)
	_, fi, err := writeFile(p.settings.StrmFolder, title+".strm", []byte(contents))
	if err != nil {
		return "", err
	}
	return downloadID(fi), nil
}

// Items implements client.Client: the .strm files of the strm folder.
func (p *pneumatic) Items(ctx context.Context, opts client.Options) ([]client.Item, error) {
	root, err := os.OpenRoot(p.settings.StrmFolder)
	if err != nil {
		return nil, fmt.Errorf("open strm folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("read strm folder: %w", err)
	}
	var items []client.Item
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if filepath.Ext(e.Name()) != ".strm" {
			continue
		}
		fi, err := root.Stat(e.Name())
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(p.settings.StrmFolder, e.Name())
		item := client.Item{
			ID: downloadID(fi), Title: client.CleanFileName(e.Name()),
			TotalSize: fi.Size(), OutputPath: path, Status: client.Completed,
			CanMoveFiles: true, CanBeRemoved: opts.RemoveCompletedDownloads,
		}
		if p.locked(path) {
			item.Status = client.Downloading
		}
		items = append(items, item)
	}
	return items, nil
}

// Remove implements client.Client; Pneumatic cannot remove downloads.
func (p *pneumatic) Remove(context.Context, client.Item, bool) error {
	return fmt.Errorf("pneumatic cannot remove downloads: %w", client.ErrNotSupported)
}

// Info implements client.Client.
func (p *pneumatic) Info(context.Context) (client.Info, error) {
	return client.Info{IsLocalhost: true}, nil
}

// Test implements client.Client.
func (p *pneumatic) Test(context.Context) error {
	if err := client.TestFolder(p.settings.NzbFolder, "nzb folder", true); err != nil {
		return err
	}
	return client.TestFolder(p.settings.StrmFolder, "strm folder", true)
}

// Host implements client.Client: Pneumatic is folders.
func (p *pneumatic) Host() string { return "" }

package acquisition

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// ManualImportFile is a video file of a tracked download that can be
// imported by hand, as Radarr's ManualImportItem.
type ManualImportFile struct {
	// Path is the server's path of the file; RelativePath is that below
	// the download's.
	Path, RelativePath string
	Size               int64
	// Quality is what an import would give the file.
	Quality acquire.QualityModel
	// Rejections are why the import specifications reject the file,
	// which a manual import overrides.
	Rejections []string
}

// manualDownload is a tracked movie download to import by hand.
type manualDownload struct {
	d        core.Download
	p        polled
	local    string
	mappings []acquire.RemotePathMapping
	rn       *run
	m        *movie
	files    []importFile
}

// ListManualImport lists the video files of a tracked movie download with
// the quality an import would give them and why the import specifications
// reject them, as Radarr's ManualImportService.GetMediaFiles for a
// download.
func (r *Runtime) ListManualImport(ctx context.Context, id core.ID) ([]ManualImportFile, error) {
	r.importing.Lock()
	defer r.importing.Unlock()
	md, err := r.manualDownload(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]ManualImportFile, 0, len(md.files))
	for _, f := range md.files {
		plan, reasons, err := r.planImport(ctx, md.rn, md.m, &md.d, f, nil, true)
		if err != nil {
			return nil, err
		}
		out = append(out, ManualImportFile{Path: f.path, RelativePath: f.rel, Size: f.size, Quality: plan.q, Rejections: reasons})
	}
	return out, nil
}

// ManualImport imports a video file of a tracked movie download for its
// movie whatever the import specifications reject, as Radarr's
// ManualImportService.Execute; quality, when set, is the file's quality
// rather than that read from it and its names. The download is then
// complete, as an automatic import leaves it.
func (r *Runtime) ManualImport(ctx context.Context, id core.ID, path string, quality *acquire.QualityModel) error {
	r.importing.Lock()
	defer r.importing.Unlock()
	md, err := r.manualDownload(ctx, id)
	if err != nil {
		return err
	}
	path = filepath.Clean(path)
	var file *importFile
	for i := range md.files {
		if md.files[i].path == path {
			file = &md.files[i]
		}
	}
	if file == nil {
		return fmt.Errorf("%w: %s is not a video file of download %s", core.ErrNotFound, path, md.d.Title)
	}
	plan, reasons, err := r.planImport(ctx, md.rn, md.m, &md.d, *file, quality, true)
	if err != nil {
		return err
	}
	if plan.libPath == "" {
		return fmt.Errorf("%w: %s", core.ErrConflict, strings.Join(reasons, "; "))
	}
	if len(reasons) > 0 {
		r.logger().InfoContext(ctx, "importing by hand despite rejections", "download", md.d.Title, "path", path, "rejections", reasons)
	}
	if err := r.importPlanned(ctx, md.rn, md.m, &md.d, md.p, *file, plan); err != nil {
		return err
	}
	if md.p.item.GetCanMoveFiles() {
		r.cleanDownloadFolder(ctx, md.m, md.local, md.mappings)
	}
	return r.completeImport(ctx, &md.d, md.p)
}

// manualDownload looks up a tracked movie download, its client's listing
// and its video files for a manual import.
func (r *Runtime) manualDownload(ctx context.Context, id core.ID) (*manualDownload, error) {
	d, err := r.Store.Downloads().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if isSeriesDownload(d) {
		return nil, fmt.Errorf("%w: download %s is of episodes", core.ErrInvalid, d.Title)
	}
	switch d.State {
	case core.DownloadImported, core.DownloadFailed, core.DownloadFailedPending, core.DownloadIgnored:
		return nil, fmt.Errorf("%w: download %s is %s", core.ErrConflict, d.Title, d.State)
	}
	p, err := r.pollDownload(ctx, d)
	if err != nil {
		return nil, err
	}
	out := p.item.GetOutputPath()
	if strings.TrimSpace(out) == "" {
		return nil, fmt.Errorf("%w: %s", core.ErrConflict, msgNoOutputPath)
	}
	mappings, err := r.remotePathMappings(ctx)
	if err != nil {
		return nil, err
	}
	local := acquire.RemapRemoteToLocal(mappings, p.client.GetHost(), out)
	if !filepath.IsAbs(local) {
		return nil, fmt.Errorf("%w: %s", core.ErrConflict, fmt.Sprintf(msgNotLocalPath, local))
	}
	local = filepath.Clean(local)
	rn, err := r.newRun(ctx)
	if err != nil {
		return nil, err
	}
	m, err := rn.movie(ctx, d.ItemID)
	if errors.Is(err, core.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", core.ErrConflict, msgNoMovie)
	}
	if err != nil {
		return nil, err
	}
	files, err := importFiles(ctx, local)
	if err != nil {
		return nil, fmt.Errorf("read download %s: %w", local, err)
	}
	return &manualDownload{d: d, p: p, local: local, mappings: mappings, rn: rn, m: m, files: files}, nil
}

// pollDownload asks a tracked download's client for it.
func (r *Runtime) pollDownload(ctx context.Context, d core.Download) (polled, error) {
	clients, _ := r.ListClients(ctx)
	for _, prov := range r.clients() {
		if prov.ID != d.PluginID {
			continue
		}
		resp, err := prov.Client.ListDownloads(ctx, pluginv1.ListDownloadsRequest_builder{IncludeStatuses: new(true)}.Build())
		if err != nil {
			return polled{}, fmt.Errorf("list downloads of %s: %w", prov.ID, err)
		}
		for _, item := range resp.GetDownloads() {
			if item.GetClientId() != d.ClientID || item.GetId() != d.DownloadID {
				continue
			}
			c := Client{PluginID: prov.ID, DownloadClient: pluginv1.DownloadClient_builder{Id: new(item.GetClientId())}.Build()}
			for _, known := range clients {
				if known.Ref() == c.PluginID+"/"+item.GetClientId() {
					c = known
				}
			}
			return polled{client: c, item: item}, nil
		}
	}
	return polled{}, fmt.Errorf("%w: download %s is not in its client", core.ErrNotFound, d.Title)
}

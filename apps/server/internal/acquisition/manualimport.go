package acquisition

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// msgUnknownMovie is the rejection of files of a manual import no movie
// matches, Radarr's.
const msgUnknownMovie = "Unknown Movie"

// ImportMode is how a manual import transfers a file, as Radarr's
// ImportMode.
type ImportMode string

// The import modes.
const (
	// ImportAuto moves the files of downloads whose client lets them be
	// moved and of folders, and copies the others.
	ImportAuto ImportMode = ""
	// ImportMove moves the file.
	ImportMove ImportMode = "move"
	// ImportCopy copies the file, or hard links it with
	// CopyUsingHardlinks.
	ImportCopy ImportMode = "copy"
)

// ManualImportQuery selects the files of a manual import, as the
// parameters of Radarr's GET /manualimport: those of a tracked download,
// or of a folder or file that is not one.
type ManualImportQuery struct {
	// DownloadID is the tracked download; zero for Folder.
	DownloadID core.ID
	// Folder is the absolute path of a folder or file to import from.
	Folder string
	// MovieID is the movie the files are for; zero for the download's
	// movie, or for that each file's names match among the wanted movies.
	MovieID core.ID
}

// ManualImportFile is a video file that can be imported by hand, as
// Radarr's ManualImportItem.
type ManualImportFile struct {
	// Path is the server's path of the file; RelativePath is that below
	// the download's or the folder's.
	Path, RelativePath string
	Size               int64
	// MovieID is the movie it is for; zero when no movie matches.
	MovieID core.ID
	// Quality, Languages and ReleaseGroup are what an import would give
	// the file.
	Quality      acquire.QualityModel
	Languages    []release.Language
	ReleaseGroup string
	// Rejections are why the import specifications reject the file,
	// which a manual import overrides.
	Rejections []string
}

// ManualImportRequest is a file to import by hand, as an item of Radarr's
// ManualImport command.
type ManualImportRequest struct {
	// DownloadID, Folder and Path name the file as ListManualImport lists
	// it.
	DownloadID core.ID
	Folder     string
	Path       string
	// MovieID is the movie to import the file for; zero for that
	// ListManualImport lists.
	MovieID core.ID
	// Quality, Languages and ReleaseGroup, when set, are the file's
	// rather than those ListManualImport lists.
	Quality      *acquire.QualityModel
	Languages    []release.Language
	ReleaseGroup *string
	Mode         ImportMode
}

// manualSource is a tracked download, or a folder that is not one, to
// import from by hand.
type manualSource struct {
	// d is the tracked download; for a folder, one standing for it, named
	// as the folder.
	d       core.Download
	tracked bool
	p       polled
	// local is the server's path of the download or folder.
	local    string
	mappings []acquire.RemotePathMapping
	rn       *run
	files    []importFile
}

// ListManualImport lists the video files of a tracked download, or of a
// folder that is not one, with the movie each is for, the quality,
// languages and release group an import would give it and why the import
// specifications reject it, as Radarr's ManualImportService.GetMediaFiles.
// A file is for the query's movie, else for the download's, else for the
// wanted movie its name, or its folder's, matches; files no movie matches
// are rejected as unknown.
func (r *Runtime) ListManualImport(ctx context.Context, q ManualImportQuery) ([]ManualImportFile, error) {
	r.importing.Lock()
	defer r.importing.Unlock()
	src, err := r.manualSource(ctx, q.DownloadID, q.Folder)
	if err != nil {
		return nil, err
	}
	out := make([]ManualImportFile, 0, len(src.files))
	for _, f := range src.files {
		item := ManualImportFile{Path: f.path, RelativePath: f.rel, Size: f.size}
		m, err := r.manualMovie(ctx, src, f, q.MovieID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			item.Quality = acquire.ModelOf(release.ParseQuality(filepath.Base(f.path), release.Movie))
			item.Rejections = []string{msgUnknownMovie}
			out = append(out, item)
			continue
		}
		plan, reasons, err := r.planImport(ctx, src.rn, m, &src.d, f, nil, true)
		if err != nil {
			return nil, err
		}
		item.MovieID, item.Quality, item.Languages, item.ReleaseGroup, item.Rejections = m.item.ID, plan.q, plan.langs, plan.releaseGroup, reasons
		out = append(out, item)
	}
	return out, nil
}

// ManualImport imports a video file of a tracked download, or of a folder
// that is not one, whatever the import specifications reject, as Radarr's
// ManualImportService.Execute: for the movie chosen, else that
// ListManualImport lists, with the quality, languages and release group
// chosen, else those read from the file and its names, transferred as
// mode says. A tracked download is then complete, as an automatic import
// leaves it, even when the file was for another movie.
func (r *Runtime) ManualImport(ctx context.Context, req ManualImportRequest) error {
	r.importing.Lock()
	defer r.importing.Unlock()
	src, err := r.manualSource(ctx, req.DownloadID, req.Folder)
	if err != nil {
		return err
	}
	path := filepath.Clean(req.Path)
	var file *importFile
	for i := range src.files {
		if src.files[i].path == path {
			file = &src.files[i]
		}
	}
	if file == nil {
		return fmt.Errorf("%w: %s is not a video file of %s", core.ErrNotFound, req.Path, src.local)
	}
	m, err := r.manualMovie(ctx, src, *file, req.MovieID)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("%w: %s: %s", core.ErrInvalid, msgUnknownMovie, req.Path)
	}
	plan, reasons, err := r.planImport(ctx, src.rn, m, &src.d, *file, req.Quality, true)
	if err != nil {
		return err
	}
	if plan.libPath == "" {
		return fmt.Errorf("%w: %s", core.ErrConflict, strings.Join(reasons, "; "))
	}
	if req.Languages != nil {
		plan.langs = req.Languages
	}
	if req.ReleaseGroup != nil {
		plan.releaseGroup = strings.TrimSpace(*req.ReleaseGroup)
	}
	plan.formats = src.rn.formats(src.d.Title, plan.q, plan.langs, file.size, nil)
	mode, err := r.manualMode(req.Mode, src)
	if err != nil {
		return err
	}
	plan.mode = &mode
	if len(reasons) > 0 {
		r.logger().InfoContext(ctx, "importing by hand despite rejections", "from", src.local, "path", path, "rejections", reasons)
	}
	if err := r.importPlanned(ctx, src.rn, m, &src.d, src.p, *file, plan); err != nil {
		return err
	}
	if !src.tracked {
		return nil
	}
	if mode == transferMove && src.p.item.GetCanMoveFiles() {
		r.cleanDownloadFolder(ctx, m, src.local, src.mappings)
	}
	return r.completeImport(ctx, &src.d, src.p)
}

// manualMode returns how a manual import transfers a file, as Radarr's
// ImportApprovedMovie and MovieFileMovingService: files are moved unless
// copied, as asked or because their download's client does not let them
// be moved, and copies are hard links with CopyUsingHardlinks.
func (r *Runtime) manualMode(mode ImportMode, src *manualSource) (transferMode, error) {
	copyOnly := false
	switch mode {
	case ImportAuto:
		copyOnly = src.tracked && !src.p.item.GetCanMoveFiles()
	case ImportMove:
	case ImportCopy:
		copyOnly = true
	default:
		return 0, fmt.Errorf("%w: unknown import mode %q", core.ErrInvalid, mode)
	}
	switch {
	case !copyOnly:
		return transferMove, nil
	case r.settings().CopyUsingHardlinks:
		return transferHardLinkOrCopy, nil
	}
	return transferCopy, nil
}

// manualMovie returns the movie a file of a manual import is for: the one
// asked for, else the download's, else the wanted movie the file's name,
// or else its folder's, matches, as Radarr's ParsingService.GetMovie; nil
// when none does.
func (r *Runtime) manualMovie(ctx context.Context, src *manualSource, f importFile, movieID core.ID) (*movie, error) {
	switch {
	case !movieID.IsZero():
		m, err := src.rn.movie(ctx, movieID)
		if errors.Is(err, core.ErrNotFound) {
			return nil, fmt.Errorf("%w: movie %s is not wanted", core.ErrNotFound, movieID)
		}
		return m, err
	case src.tracked:
		m, err := src.rn.movie(ctx, src.d.ItemID)
		if errors.Is(err, core.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", core.ErrConflict, msgNoMovie)
		}
		return m, err
	}
	refs, err := src.rn.movieRefs(ctx)
	if err != nil {
		return nil, err
	}
	names := []string{strings.TrimSuffix(filepath.Base(f.path), filepath.Ext(f.path))}
	if f.inFolder {
		names = append(names, filepath.Base(filepath.Dir(f.path)), filepath.Base(src.local))
	}
	for _, name := range names {
		parsed := release.ParseMovieTitle(name, false)
		if parsed == nil || parsed.PrimaryMovieTitle() == "" {
			continue
		}
		ref, err := acquire.MatchMovie(parsed, parsed.ImdbID, parsed.TmdbID, nil, refs)
		if err != nil {
			return nil, err
		}
		if ref != nil {
			return src.rn.movie(ctx, ref.ID)
		}
	}
	return nil, nil
}

// manualSource looks up a tracked movie download, its client's listing
// and its video files, or the video files of a folder that is not a
// tracked download, for a manual import.
func (r *Runtime) manualSource(ctx context.Context, downloadID core.ID, folder string) (*manualSource, error) {
	rn, err := r.newRun(ctx)
	if err != nil {
		return nil, err
	}
	src := &manualSource{rn: rn}
	if downloadID.IsZero() {
		local, ok := downloadPath(folder)
		if !ok {
			return nil, fmt.Errorf("%w: folder %q is not absolute", core.ErrInvalid, folder)
		}
		src.local = local
		fi, err := r.statDownload(ctx, src.local)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", core.ErrNotFound, src.local)
		}
		if err != nil {
			return nil, err
		}
		name := fi.Name()
		if !fi.IsDir() {
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
		src.d = core.Download{Title: name}
	} else {
		d, err := r.Store.Downloads().Get(ctx, downloadID)
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
		local, ok := downloadPath(acquire.RemapRemoteToLocal(mappings, p.client.GetHost(), out))
		if !ok {
			return nil, fmt.Errorf("%w: %s", core.ErrConflict, fmt.Sprintf(msgNotLocalPath, local))
		}
		src.d, src.tracked, src.p, src.local, src.mappings = d, true, p, local, mappings
	}
	files, err := r.importFiles(ctx, src.local)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", src.local, err)
	}
	src.files = files
	return src, nil
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

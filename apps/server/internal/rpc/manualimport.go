package rpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/release"
)

var importModes = map[acquisition.ImportMode]acquisitionv1.ImportMode{
	acquisition.ImportAuto: acquisitionv1.ImportMode_IMPORT_MODE_UNSPECIFIED,
	acquisition.ImportMove: acquisitionv1.ImportMode_IMPORT_MODE_MOVE,
	acquisition.ImportCopy: acquisitionv1.ImportMode_IMPORT_MODE_COPY,
}

// ListManualImport lists the video files of a tracked movie download, or
// of a folder that is not one.
func (s *DownloadService) ListManualImport(ctx context.Context, req *acquisitionv1.ListManualImportRequest) (*acquisitionv1.ListManualImportResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := parseOptionalID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	movieID, err := parseOptionalID(req.GetMovieId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	files, err := s.rt.ListManualImport(ctx, acquisition.ManualImportQuery{DownloadID: id, Folder: req.GetFolder(), MovieID: movieID})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.ManualImportFile, len(files))
	for i, f := range files {
		out[i] = acquisitionv1.ManualImportFile_builder{
			Path: new(f.Path), RelativePath: new(f.RelativePath), Size: new(f.Size),
			Quality: new(f.Quality.Quality.String()), Revision: new(int32(f.Quality.Revision.Version)), Rejections: f.Rejections,
			MovieId: optionalID(f.MovieID), Languages: languageNames(f.Languages), ReleaseGroup: new(f.ReleaseGroup),
		}.Build()
	}
	return acquisitionv1.ListManualImportResponse_builder{Files: out}.Build(), nil
}

// ManualImport imports a video file of a tracked movie download, or of a
// folder that is not one.
func (s *DownloadService) ManualImport(ctx context.Context, req *acquisitionv1.ManualImportRequest) (*acquisitionv1.ManualImportResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := parseOptionalID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	movieID, err := parseOptionalID(req.GetMovieId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	in := acquisition.ManualImportRequest{DownloadID: id, Folder: req.GetFolder(), Path: req.GetPath(), MovieID: movieID}
	if name := strings.TrimSpace(req.GetQuality()); name != "" {
		q, ok := qualityNamed(name)
		if !ok {
			return nil, connectError(ctx, fmt.Errorf("%w: unknown quality %q", core.ErrInvalid, name))
		}
		in.Quality = &acquire.QualityModel{Quality: q, Revision: release.Revision{Version: 1}}
	}
	for _, name := range req.GetLanguages() {
		l, ok := release.LanguageByName(name)
		if !ok {
			return nil, connectError(ctx, fmt.Errorf("%w: unknown language %q", core.ErrInvalid, name))
		}
		in.Languages = append(in.Languages, l)
	}
	if req.HasReleaseGroup() {
		in.ReleaseGroup = new(req.GetReleaseGroup())
	}
	// Unknown modes are automatic.
	in.Mode = keyOf(importModes, req.GetMode())
	if err := s.rt.ManualImport(ctx, in); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.ManualImportResponse{}, nil
}

// qualityNamed returns the quality of Radarr's name, ignoring case.
func qualityNamed(name string) (release.Quality, bool) {
	for _, q := range acquire.Qualities {
		if strings.EqualFold(q.String(), name) {
			return q, true
		}
	}
	return release.Unknown, false
}

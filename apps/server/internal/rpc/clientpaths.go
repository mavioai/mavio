package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

// GetDownloadClientPaths returns the folders of a download client with
// where the server reads each.
func (s *DownloadService) GetDownloadClientPaths(ctx context.Context, req *acquisitionv1.GetDownloadClientPathsRequest) (*acquisitionv1.GetDownloadClientPathsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	paths, failure, err := s.rt.ClientPaths(ctx, req.GetPluginId(), req.GetClientId())
	if err != nil {
		return nil, clientPathsError(ctx, err)
	}
	return acquisitionv1.GetDownloadClientPathsResponse_builder{Paths: clientPathsToProto(paths), Failure: new(failure)}.Build(), nil
}

// UpdateDownloadClientPaths replaces where the server reads the folders
// of a download client.
func (s *DownloadService) UpdateDownloadClientPaths(ctx context.Context, req *acquisitionv1.UpdateDownloadClientPathsRequest) (*acquisitionv1.UpdateDownloadClientPathsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	in := make([]acquisition.ClientPath, len(req.GetPaths()))
	for i, p := range req.GetPaths() {
		in[i] = acquisition.ClientPath{RemotePath: p.GetRemotePath(), LocalPath: p.GetLocalPath()}
	}
	paths, failure, err := s.rt.SetClientPaths(ctx, req.GetPluginId(), req.GetClientId(), in)
	if err != nil {
		return nil, clientPathsError(ctx, err)
	}
	return acquisitionv1.UpdateDownloadClientPathsResponse_builder{Paths: clientPathsToProto(paths), Failure: new(failure)}.Build(), nil
}

func clientPathsError(ctx context.Context, err error) error {
	if errors.Is(err, acquisition.ErrClientNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connectError(ctx, err)
}

func clientPathsToProto(paths []acquisition.ClientPath) []*acquisitionv1.DownloadClientPath {
	out := make([]*acquisitionv1.DownloadClientPath, len(paths))
	for i, p := range paths {
		out[i] = acquisitionv1.DownloadClientPath_builder{
			RemotePath: new(p.RemotePath), LocalPath: new(p.LocalPath), RootFolder: new(p.RootFolder),
			SuggestedPath: new(p.SuggestedPath), Readable: new(p.Readable), Error: new(p.Error),
			LinkLibraryPaths: p.LinkLibraryPaths,
		}.Build()
	}
	return out
}

package app

import (
	"context"
	"maps"
	"slices"

	"connectrpc.com/connect"

	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
)

func (s *fakeStorage) ListDisks(context.Context, *storagev1.ListDisksRequest) (*storagev1.ListDisksResponse, error) {
	return storagev1.ListDisksResponse_builder{Disks: []*storagev1.Disk{
		storagev1.Disk_builder{
			Path: new("/"), Name: new("Macintosh HD"), FileSystem: new("apfs"), Kind: new(storagev1.DiskKind_DISK_KIND_SSD),
			TotalBytes: new(int64(1e12)), UsedBytes: new(int64(9e11)), AvailableBytes: new(int64(1e11)),
		}.Build(),
		storagev1.Disk_builder{Path: new("/Volumes/nas"), FileSystem: new("smbfs"), Kind: new(storagev1.DiskKind_DISK_KIND_NETWORK)}.Build(),
	}}.Build(), nil
}

// fakeProviders are the kinds of remotes: Google Drive, whose
// configuration asks a question, and SFTP, of a required host.
func fakeProviders() []*storagev1.Provider {
	return []*storagev1.Provider{
		storagev1.Provider_builder{Name: new("drive"), Description: new("Google Drive"), Options: []*storagev1.Option{
			storagev1.Option_builder{
				Name: new("scope"), Help: new("Scope of access."), DefaultValue: new("drive"), Type: new("string"), Exclusive: new(true),
				Examples: []*storagev1.Example{
					storagev1.Example_builder{Value: new("drive"), Help: new("Full access")}.Build(),
					storagev1.Example_builder{Value: new("drive.readonly"), Help: new("Read only")}.Build(),
				},
			}.Build(),
			storagev1.Option_builder{Name: new("token"), Help: new("OAuth token."), Type: new("string"), Sensitive: new(true)}.Build(),
			storagev1.Option_builder{Name: new("chunk_size"), Help: new("Upload chunk size."), DefaultValue: new("8Mi"), Type: new("SizeSuffix"), Advanced: new(true)}.Build(),
		}}.Build(),
		storagev1.Provider_builder{Name: new("sftp"), Description: new("SSH/SFTP"), Options: []*storagev1.Option{
			storagev1.Option_builder{Name: new("host"), Help: new("SSH host to connect to."), Type: new("string"), Required: new(true)}.Build(),
			storagev1.Option_builder{Name: new("pass"), Help: new("SSH password."), Type: new("string"), Password: new(true)}.Build(),
		}}.Build(),
	}
}

func (s *fakeStorage) ListProviders(context.Context, *storagev1.ListProvidersRequest) (*storagev1.ListProvidersResponse, error) {
	return storagev1.ListProvidersResponse_builder{Providers: fakeProviders()}.Build(), nil
}

// authQuestion is the question a fake Google Drive asks.
func authQuestion() *storagev1.ConfigQuestion {
	return storagev1.ConfigQuestion_builder{State: new("*oauth"), Option: storagev1.Option_builder{
		Name: new("config_is_local"), Help: new("Use web browser to automatically authenticate rclone with remote?"),
		DefaultValue: new("true"), Type: new("bool"),
	}.Build()}.Build()
}

func (s *fakeStorage) CreateRemote(_ context.Context, r *storagev1.CreateRemoteRequest) (*storagev1.CreateRemoteResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	remote := storagev1.Remote_builder{Name: new(r.GetName()), Type: new(r.GetType()), Parameters: maps.Clone(r.GetParameters())}.Build()
	s.f.remotes = append(s.f.remotes, remote)
	res := storagev1.CreateRemoteResponse_builder{Remote: remote}.Build()
	if r.GetType() == "drive" {
		res.SetQuestion(authQuestion())
	}
	return res, nil
}

func (s *fakeStorage) UpdateRemote(_ context.Context, r *storagev1.UpdateRemoteRequest) (*storagev1.UpdateRemoteResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	i := slices.IndexFunc(s.f.remotes, func(x *storagev1.Remote) bool { return x.GetName() == r.GetName() })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	params := maps.Clone(s.f.remotes[i].GetParameters())
	if params == nil {
		params = map[string]string{}
	}
	maps.Copy(params, r.GetParameters())
	s.f.remotes[i].SetParameters(params)
	res := storagev1.UpdateRemoteResponse_builder{Remote: s.f.remotes[i]}.Build()
	if len(r.GetParameters()) == 0 {
		res.SetQuestion(authQuestion())
	}
	return res, nil
}

func (s *fakeStorage) ContinueRemote(_ context.Context, r *storagev1.ContinueRemoteRequest) (*storagev1.ContinueRemoteResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.answers = append(s.f.answers, r.GetAnswer())
	return &storagev1.ContinueRemoteResponse{}, nil
}

func (s *fakeStorage) DeleteRemote(_ context.Context, r *storagev1.DeleteRemoteRequest) (*storagev1.DeleteRemoteResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.remotes = slices.DeleteFunc(s.f.remotes, func(x *storagev1.Remote) bool { return x.GetName() == r.GetName() })
	return &storagev1.DeleteRemoteResponse{}, nil
}

func (s *fakeStorage) GetRemoteUsage(_ context.Context, r *storagev1.GetRemoteUsageRequest) (*storagev1.GetRemoteUsageResponse, error) {
	if r.GetRemote() != "drive" {
		return &storagev1.GetRemoteUsageResponse{}, nil
	}
	return storagev1.GetRemoteUsageResponse_builder{TotalBytes: new(int64(2e12)), UsedBytes: new(int64(5e11))}.Build(), nil
}

func (s *fakeStorage) ListOfflineClients(context.Context, *storagev1.ListOfflineClientsRequest) (*storagev1.ListOfflineClientsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return storagev1.ListOfflineClientsResponse_builder{Clients: slices.Clone(s.f.offline)}.Build(), nil
}

func (s *fakeStorage) SetOfflineClients(_ context.Context, r *storagev1.SetOfflineClientsRequest) (*storagev1.SetOfflineClientsResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.offline = slices.Clone(r.GetClients())
	return &storagev1.SetOfflineClientsResponse{}, nil
}

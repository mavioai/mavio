package rpc

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"path"
	"slices"
	"strings"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/libs/core"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/rclone"
)

// StorageService implements mavio.storage.v1.StorageService.
type StorageService struct {
	// Client reaches the rcd; nil when the server reaches none.
	Client *rclone.Client
	// Managed is set when the server runs the rcd itself.
	Managed bool
	// Changed is told of remotes whose configuration changed; nil tells
	// no one.
	Changed func(remote string)
	// Offline keeps the offline download clients; nil keeps none.
	Offline *downloads.Offline
	// Down is why the server reaches no rcd, when Client is nil.
	Down string
}

// client returns the rcd's client, after checking the caller is an
// administrator.
func (s *StorageService) client(ctx context.Context) (*rclone.Client, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Client == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the server reaches no rclone"))
	}
	return s.Client, nil
}

// rcloneError maps an error of the rcd to a Connect error.
func rcloneError(ctx context.Context, err error) error {
	var e *rclone.Error
	if errors.As(err, &e) {
		switch e.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err)
		}
		return connect.NewError(connect.CodeUnavailable, err)
	}
	return connectError(ctx, err)
}

// GetStatus tells whether the server reaches an rcd.
func (s *StorageService) GetStatus(ctx context.Context, _ *storagev1.GetStatusRequest) (*storagev1.GetStatusResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := storagev1.GetStatusResponse_builder{Managed: &s.Managed}.Build()
	if s.Client == nil {
		out.SetError(cmp.Or(s.Down, "the server reaches no rclone"))
		return out, nil
	}
	v, err := s.Client.Version(ctx)
	if err != nil {
		out.SetError(err.Error())
		return out, nil
	}
	out.SetAvailable(true)
	out.SetVersion(v)
	return out, nil
}

// ListProviders lists the kinds of remotes.
func (s *StorageService) ListProviders(ctx context.Context, _ *storagev1.ListProvidersRequest) (*storagev1.ListProvidersResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	providers, err := c.Providers(ctx)
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	out := make([]*storagev1.Provider, len(providers))
	for i, p := range providers {
		opts := make([]*storagev1.Option, len(p.Options))
		for j, o := range p.Options {
			opts[j] = optionToProto(o)
		}
		out[i] = storagev1.Provider_builder{Name: &p.Name, Description: &p.Description, Options: opts}.Build()
	}
	return storagev1.ListProvidersResponse_builder{Providers: out}.Build(), nil
}

func optionToProto(o rclone.Option) *storagev1.Option {
	examples := make([]*storagev1.Example, len(o.Examples))
	for i, e := range o.Examples {
		examples[i] = storagev1.Example_builder{Value: &e.Value, Help: &e.Help, Provider: &e.Provider}.Build()
	}
	return storagev1.Option_builder{
		Name: &o.Name, Help: &o.Help, Provider: &o.Provider, DefaultValue: &o.DefaultStr, Type: &o.Type,
		Required: &o.Required, Password: &o.IsPassword, Advanced: &o.Advanced, Sensitive: &o.Sensitive,
		Exclusive: &o.Exclusive, Examples: examples,
	}.Build()
}

// ListRemotes lists the configured remotes, leaving out their secrets.
func (s *StorageService) ListRemotes(ctx context.Context, _ *storagev1.ListRemotesRequest) (*storagev1.ListRemotesResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	remotes, err := c.Remotes(ctx)
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	secrets, err := s.secrets(ctx, c)
	if err != nil {
		return nil, err
	}
	out := make([]*storagev1.Remote, len(remotes))
	for i, r := range remotes {
		out[i] = remoteToProto(r, secrets)
	}
	return storagev1.ListRemotesResponse_builder{Remotes: out}.Build(), nil
}

// secrets returns, by provider, the options whose values are left out of
// listings.
func (s *StorageService) secrets(ctx context.Context, c *rclone.Client) (map[string][]string, error) {
	providers, err := c.Providers(ctx)
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	out := map[string][]string{}
	for _, p := range providers {
		for _, o := range p.Options {
			if o.IsPassword || o.Sensitive {
				out[p.Name] = append(out[p.Name], o.Name)
			}
		}
	}
	return out, nil
}

func remoteToProto(r rclone.Remote, secrets map[string][]string) *storagev1.Remote {
	params := map[string]string{}
	for k, v := range r.Parameters {
		if k == "token" || slices.Contains(secrets[r.Type], k) {
			continue
		}
		params[k] = v
	}
	return storagev1.Remote_builder{Name: &r.Name, Type: &r.Type, Parameters: params}.Build()
}

// step answers with the remote as configured, or the question its
// configuration asks.
func (s *StorageService) step(ctx context.Context, c *rclone.Client, name string, step rclone.ConfigStep) (*storagev1.Remote, *storagev1.ConfigQuestion, error) {
	if s.Changed != nil {
		s.Changed(name)
	}
	var question *storagev1.ConfigQuestion
	if !step.Done() {
		question = storagev1.ConfigQuestion_builder{State: &step.State, Error: &step.Error}.Build()
		if step.Option != nil {
			question.SetOption(optionToProto(*step.Option))
		}
	}
	remotes, err := c.Remotes(ctx)
	if err != nil {
		return nil, nil, rcloneError(ctx, err)
	}
	i := slices.IndexFunc(remotes, func(r rclone.Remote) bool { return r.Name == name })
	if i < 0 {
		return nil, question, nil
	}
	secrets, err := s.secrets(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	return remoteToProto(remotes[i], secrets), question, nil
}

// CreateRemote configures a remote.
func (s *StorageService) CreateRemote(ctx context.Context, req *storagev1.CreateRemoteRequest) (*storagev1.CreateRemoteResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	remotes, err := c.Remotes(ctx)
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	if slices.ContainsFunc(remotes, func(r rclone.Remote) bool { return r.Name == req.GetName() }) {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("a remote of that name exists"))
	}
	st, err := c.CreateRemote(ctx, req.GetName(), req.GetType(), req.GetParameters())
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	remote, question, err := s.step(ctx, c, req.GetName(), st)
	if err != nil {
		return nil, err
	}
	return storagev1.CreateRemoteResponse_builder{Remote: remote, Question: question}.Build(), nil
}

// UpdateRemote changes settings of a remote.
func (s *StorageService) UpdateRemote(ctx context.Context, req *storagev1.UpdateRemoteRequest) (*storagev1.UpdateRemoteResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.exists(ctx, c, req.GetName()); err != nil {
		return nil, err
	}
	st, err := c.UpdateRemote(ctx, req.GetName(), req.GetParameters())
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	remote, question, err := s.step(ctx, c, req.GetName(), st)
	if err != nil {
		return nil, err
	}
	return storagev1.UpdateRemoteResponse_builder{Remote: remote, Question: question}.Build(), nil
}

func (s *StorageService) exists(ctx context.Context, c *rclone.Client, name string) error {
	remotes, err := c.Remotes(ctx)
	if err != nil {
		return rcloneError(ctx, err)
	}
	if !slices.ContainsFunc(remotes, func(r rclone.Remote) bool { return r.Name == name }) {
		return connect.NewError(connect.CodeNotFound, errors.New("no remote of that name"))
	}
	return nil
}

// ContinueRemote answers the question of a remote's configuration.
func (s *StorageService) ContinueRemote(ctx context.Context, req *storagev1.ContinueRemoteRequest) (*storagev1.ContinueRemoteResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.exists(ctx, c, req.GetName()); err != nil {
		return nil, err
	}
	st, err := c.Continue(ctx, req.GetName(), req.GetState(), req.GetAnswer())
	if err != nil {
		return nil, rcloneError(ctx, err)
	}
	remote, question, err := s.step(ctx, c, req.GetName(), st)
	if err != nil {
		return nil, err
	}
	return storagev1.ContinueRemoteResponse_builder{Remote: remote, Question: question}.Build(), nil
}

// DeleteRemote removes a remote's configuration.
func (s *StorageService) DeleteRemote(ctx context.Context, req *storagev1.DeleteRemoteRequest) (*storagev1.DeleteRemoteResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.exists(ctx, c, req.GetName()); err != nil {
		return nil, err
	}
	if err := c.DeleteRemote(ctx, req.GetName()); err != nil {
		return nil, rcloneError(ctx, err)
	}
	if s.Changed != nil {
		s.Changed(req.GetName())
	}
	return &storagev1.DeleteRemoteResponse{}, nil
}

// ListFolders lists the folders of a folder of a remote.
func (s *StorageService) ListFolders(ctx context.Context, req *storagev1.ListFoldersRequest) (*storagev1.ListFoldersResponse, error) {
	c, err := s.client(ctx)
	if err != nil {
		return nil, err
	}
	remote, dir, ok := core.SplitRemotePath(req.GetPath())
	dir = strings.Trim(dir, "/")
	if !ok || strings.ContainsAny(remote, `/\`) || (dir != "" && (path.Clean(dir) != dir || dir == ".." || strings.HasPrefix(dir, "../"))) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("want rclone:<remote>: or rclone:<remote>:<path>"))
	}
	var out struct {
		List []struct {
			Name string `json:"Name"`
		} `json:"list"`
	}
	in := map[string]any{"fs": remote + ":" + dir, "remote": "", "opt": map[string]any{"dirsOnly": true, "noModTime": true, "noMimeType": true}}
	if err := c.Call(ctx, "operations/list", in, &out); err != nil {
		return nil, rcloneError(ctx, err)
	}
	folders := make([]string, len(out.List))
	for i, e := range out.List {
		folders[i] = core.RemoteScheme + remote + ":" + path.Join(dir, e.Name)
	}
	slices.Sort(folders)
	return storagev1.ListFoldersResponse_builder{Folders: folders}.Build(), nil
}

// ListOfflineClients lists the download clients of offline downloads.
func (s *StorageService) ListOfflineClients(ctx context.Context, _ *storagev1.ListOfflineClientsRequest) (*storagev1.ListOfflineClientsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Offline == nil {
		return &storagev1.ListOfflineClientsResponse{}, nil
	}
	cfg, err := s.Offline.Config(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*storagev1.OfflineClient, len(cfg.Clients))
	for i, c := range cfg.Clients {
		out[i] = storagev1.OfflineClient_builder{
			Id: &c.ID, Name: &c.Name, Enabled: &c.Enabled, Priority: &c.Priority, Folder: &c.Folder, Tags: c.Tags,
			RemoveCompletedDownloads: &c.RemoveCompleted, RemoveFailedDownloads: &c.RemoveFailed,
		}.Build()
	}
	return storagev1.ListOfflineClientsResponse_builder{Clients: out}.Build(), nil
}

// SetOfflineClients replaces the download clients of offline downloads.
func (s *StorageService) SetOfflineClients(ctx context.Context, req *storagev1.SetOfflineClientsRequest) (*storagev1.SetOfflineClientsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Offline == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the server keeps no offline download clients"))
	}
	var cfg downloads.OfflineConfig
	for _, c := range req.GetClients() {
		cfg.Clients = append(cfg.Clients, downloads.OfflineClient{
			ID: c.GetId(), Name: c.GetName(), Enabled: c.GetEnabled(), Priority: c.GetPriority(), Folder: c.GetFolder(),
			Tags: c.GetTags(), RemoveCompleted: c.GetRemoveCompletedDownloads(), RemoveFailed: c.GetRemoveFailedDownloads(),
		})
	}
	if err := s.Offline.SetConfig(ctx, cfg); err != nil {
		return nil, connectError(ctx, err)
	}
	return &storagev1.SetOfflineClientsResponse{}, nil
}

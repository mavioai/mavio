package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// HostService implements mavio.plugin.v1.HostService for plugins' host
// API requests.
type HostService struct {
	devices    DeviceController
	info       func() ServerInfo
	containers ContainerDependencies
	// Proxies gives plugins the server's proxies and solves challenges;
	// nil keeps none.
	Proxies *proxies.Service
	// Tasks keeps how far the plugins' tasks have come; nil keeps nothing.
	Tasks TaskProgress
}

// TaskProgress keeps how far the runs of the plugins' tasks have come, and
// starts the runs plugins ask of them.
type TaskProgress interface {
	// ReportTaskProgress keeps a running task's progress and status,
	// reporting whether it runs.
	ReportTaskProgress(plugin, task string, progress float64, status string) bool
	// JobProgress returns the progress of the run a job is.
	JobProgress(id core.ID) (float64, bool)
	// JobStatus returns what the run a job is is doing.
	JobStatus(id core.ID) (string, bool)
	// StartTask asks for a run of a plugin's task, without waiting;
	// core.ErrNotFound when the plugin declares no such task.
	StartTask(plugin, task string) error
}

// ServerInfo is how the server is reached.
type ServerInfo struct {
	ID, Name, Version, BaseURL string
	HTTPPort, HTTPSPort        int
}

var _ pluginv1connect.HostServiceHandler = (*HostService)(nil)

// NewHostService returns a HostService keeping devices in d, telling
// plugins how the server is reached with info and how their container
// dependencies are reached with c; d and c may be nil.
func NewHostService(d DeviceController, info func() ServerInfo, c ContainerDependencies) *HostService {
	return &HostService{devices: d, info: info, containers: c}
}

// GetServerInfo tells how the server is reached.
func (s *HostService) GetServerInfo(ctx context.Context, _ *pluginv1.GetServerInfoRequest) (*pluginv1.GetServerInfoResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins ask for the server info"))
	}
	var i ServerInfo
	if s.info != nil {
		i = s.info()
	}
	return pluginv1.GetServerInfoResponse_builder{
		ServerName: &i.Name, Version: &i.Version, HttpPort: new(int32(i.HTTPPort)), HttpsPort: new(int32(i.HTTPSPort)),
		BaseUrl: &i.BaseURL,
	}.Build(), nil
}

// SetDevices replaces the calling plugin's remote devices.
func (s *HostService) SetDevices(ctx context.Context, req *pluginv1.SetDevicesRequest) (*pluginv1.SetDevicesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins have devices"))
	}
	if s.devices == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server keeps no devices"))
	}
	if err := s.devices.SetDevices(ctx, p.Plugin, req.GetDevices()); err != nil {
		return nil, err
	}
	return &pluginv1.SetDevicesResponse{}, nil
}

// ListContainers tells the calling plugin how its container dependencies
// are reached.
func (s *HostService) ListContainers(ctx context.Context, _ *pluginv1.ListContainersRequest) (*pluginv1.ListContainersResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins have container dependencies"))
	}
	if s.containers == nil {
		return &pluginv1.ListContainersResponse{}, nil
	}
	deps, err := s.containers.List(ctx, p.Plugin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*pluginv1.ContainerStatus, len(deps))
	for i, d := range deps {
		out[i] = d.Status
	}
	return pluginv1.ListContainersResponse_builder{Containers: out}.Build(), nil
}

// ListProxies returns the server's proxies to the calling plugin, whose
// configuration must take them.
func (s *HostService) ListProxies(ctx context.Context, req *pluginv1.ListProxiesRequest) (*pluginv1.ListProxiesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" || s.Proxies == nil || !s.Proxies.UsesProxies(p.Plugin) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins whose configuration takes proxies list them"))
	}
	out, err := s.Proxies.ForPlugin(ctx, req.GetIds())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return pluginv1.ListProxiesResponse_builder{Proxies: out}.Build(), nil
}

// SolveChallenge solves a site's challenge with the challenge solver
// plugins, through the server's proxy the request names.
func (s *HostService) SolveChallenge(ctx context.Context, req *pluginv1.SolveChallengeRequest) (*pluginv1.SolveChallengeResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins solve challenges"))
	}
	if s.Proxies == nil {
		return nil, connect.NewError(connect.CodeUnavailable, proxies.ErrNoSolver)
	}
	sol, err := s.Proxies.Solve(ctx, req.GetChallenge(), req.GetProxyId())
	if errors.Is(err, proxies.ErrNoSolver) {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return pluginv1.SolveChallengeResponse_builder{Solution: sol}.Build(), nil
}

// ReportTaskProgress keeps how far the run of one of the calling plugin's
// tasks has come; a report of a task not running is ignored.
func (s *HostService) ReportTaskProgress(ctx context.Context, req *pluginv1.ReportTaskProgressRequest) (*pluginv1.ReportTaskProgressResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins have tasks"))
	}
	if s.Tasks != nil {
		s.Tasks.ReportTaskProgress(p.Plugin, req.GetTaskId(), req.GetProgress(), req.GetStatus())
	}
	return &pluginv1.ReportTaskProgressResponse{}, nil
}

// StartTask queues a run of one of the calling plugin's tasks, once the
// plugin is ready.
func (s *HostService) StartTask(ctx context.Context, req *pluginv1.StartTaskRequest) (*pluginv1.StartTaskResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if p.Plugin == "" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("only plugins have tasks"))
	}
	if s.Tasks == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("the server runs no plugin tasks"))
	}
	if err := s.Tasks.StartTask(p.Plugin, req.GetTaskId()); err != nil {
		return nil, connectError(ctx, err)
	}
	return &pluginv1.StartTaskResponse{}, nil
}

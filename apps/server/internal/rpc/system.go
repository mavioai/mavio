// Package rpc implements the Connect services defined in libs/proto.
package rpc

import (
	"context"
	"runtime"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/load"
	"github.com/mavioai/mavio/apps/server/internal/logs"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// SystemService implements mavio.system.v1.SystemService.
type SystemService struct {
	Version   string
	StartTime time.Time
	// Database is "sqlite" or "postgres"; empty until storage is wired.
	Database string
	// FFmpegVersion is the first line of `ffmpeg -version`, if available.
	FFmpegVersion string
	// Settings keeps the server settings; nil serves the defaults and
	// changes none.
	Settings *settings.Manager
	// Accelerations lists the hardware accelerations available; nil means
	// auto and none.
	Accelerations func() []core.HardwareAcceleration
	// Logs keeps the recent log records; nil keeps none.
	Logs *logs.Ring
	// Activity records administrators' changes; nil records none.
	Activity *activity.Log
	// Load samples the load of the server's computer; nil tells none.
	Load *load.Sampler
}

var _ systemv1connect.SystemServiceHandler = (*SystemService)(nil)

// GetHealth reports the server as serving.
func (s *SystemService) GetHealth(context.Context, *systemv1.GetHealthRequest) (*systemv1.GetHealthResponse, error) {
	resp := &systemv1.GetHealthResponse{}
	resp.SetStatus(systemv1.GetHealthResponse_STATUS_SERVING)
	resp.SetVersion(s.Version)
	return resp, nil
}

// GetSystemInfo describes the running server.
func (s *SystemService) GetSystemInfo(context.Context, *systemv1.GetSystemInfoRequest) (*systemv1.GetSystemInfoResponse, error) {
	return systemv1.GetSystemInfoResponse_builder{
		Version:       &s.Version,
		Os:            new(runtime.GOOS),
		Arch:          new(runtime.GOARCH),
		Database:      &s.Database,
		FfmpegVersion: &s.FFmpegVersion,
		StartTime:     timestamppb.New(s.StartTime),
	}.Build(), nil
}

// GetSystemLoad returns the load of the server's computer over the last
// minutes. Administrators only.
func (s *SystemService) GetSystemLoad(ctx context.Context, _ *systemv1.GetSystemLoadRequest) (*systemv1.GetSystemLoadResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	resp := &systemv1.GetSystemLoadResponse{}
	if s.Load == nil {
		return resp, nil
	}
	samples := s.Load.Samples()
	out := make([]*systemv1.LoadSample, 0, len(samples))
	for _, l := range samples {
		b := systemv1.LoadSample_builder{Time: timestamppb.New(l.Time)}
		if c := l.CPU; c != nil {
			b.Cpu = systemv1.CpuLoad_builder{User: &c.User, System: &c.System, Server: &c.Server}.Build()
		}
		if m := l.Memory; m != nil {
			b.Memory = systemv1.MemoryLoad_builder{
				TotalBytes: new(bytesOf(m.Total)), UsedBytes: new(bytesOf(m.Used)),
				SwapTotalBytes: new(bytesOf(m.SwapTotal)), SwapUsedBytes: new(bytesOf(m.SwapUsed)),
				ServerBytes: new(bytesOf(m.Server)),
			}.Build()
		}
		if d := l.Disk; d != nil {
			b.Disk = systemv1.DiskLoad_builder{
				ReadRate: &d.In, WriteRate: &d.Out, ReadBytes: new(bytesOf(d.InTotal)), WrittenBytes: new(bytesOf(d.OutTotal)),
			}.Build()
		}
		if n := l.Network; n != nil {
			b.Network = systemv1.NetworkLoad_builder{
				ReceiveRate: &n.In, SendRate: &n.Out, ReceivedBytes: new(bytesOf(n.InTotal)), SentBytes: new(bytesOf(n.OutTotal)),
			}.Build()
		}
		out = append(out, b.Build())
	}
	resp.SetSamples(out)
	return resp, nil
}

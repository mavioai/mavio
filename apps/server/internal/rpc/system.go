// Package rpc implements the Connect services defined in libs/proto.
package rpc

import (
	"context"
	"runtime"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
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

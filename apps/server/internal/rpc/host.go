package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// HostService implements mavio.plugin.v1.HostService for plugins' host
// API requests.
type HostService struct {
	devices DeviceController
}

var _ pluginv1connect.HostServiceHandler = (*HostService)(nil)

// NewHostService returns a HostService keeping devices in d, which may be
// nil.
func NewHostService(d DeviceController) *HostService { return &HostService{devices: d} }

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

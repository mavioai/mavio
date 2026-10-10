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
	info    func() ServerInfo
}

// ServerInfo is how the server is reached.
type ServerInfo struct {
	Name, Version, BaseURL string
	HTTPPort, HTTPSPort    int
}

var _ pluginv1connect.HostServiceHandler = (*HostService)(nil)

// NewHostService returns a HostService keeping devices in d, which may be
// nil, and telling plugins how the server is reached with info.
func NewHostService(d DeviceController, info func() ServerInfo) *HostService {
	return &HostService{devices: d, info: info}
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

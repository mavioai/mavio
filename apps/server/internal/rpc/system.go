// Package rpc implements the Connect services defined in libs/proto.
package rpc

import (
	"context"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// SystemService implements mavio.system.v1.SystemService.
type SystemService struct {
	Version string
}

var _ systemv1connect.SystemServiceHandler = (*SystemService)(nil)

// GetHealth reports the server as serving.
func (s *SystemService) GetHealth(context.Context, *systemv1.GetHealthRequest) (*systemv1.GetHealthResponse, error) {
	resp := &systemv1.GetHealthResponse{}
	resp.SetStatus(systemv1.GetHealthResponse_STATUS_SERVING)
	resp.SetVersion(s.Version)
	return resp, nil
}

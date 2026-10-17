package rpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// ProxyService implements mavio.system.v1.ProxyService.
type ProxyService struct {
	// Proxies keeps the proxies; nil keeps none.
	Proxies *proxies.Service
	// Activity records administrators' changes; nil records none.
	Activity *activity.Log
}

var _ systemv1connect.ProxyServiceHandler = (*ProxyService)(nil)

func (s *ProxyService) service() (*proxies.Service, error) {
	if s.Proxies == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("the server keeps no proxies"))
	}
	return s.Proxies, nil
}

// ListProxies lists the proxies.
func (s *ProxyService) ListProxies(ctx context.Context, _ *systemv1.ListProxiesRequest) (*systemv1.ListProxiesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	list, err := svc.List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*systemv1.Proxy, len(list))
	for i := range list {
		out[i] = proxyToProto(&list[i])
	}
	return systemv1.ListProxiesResponse_builder{Proxies: out}.Build(), nil
}

// CreateProxy adds a proxy.
func (s *ProxyService) CreateProxy(ctx context.Context, req *systemv1.CreateProxyRequest) (*systemv1.CreateProxyResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	out, err := svc.Create(ctx, proxyFromProto(req.GetProxy()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.record(ctx, p, "proxy.created", "%s added proxy %s", &out)
	return systemv1.CreateProxyResponse_builder{Proxy: proxyToProto(&out)}.Build(), nil
}

// UpdateProxy replaces a proxy.
func (s *ProxyService) UpdateProxy(ctx context.Context, req *systemv1.UpdateProxyRequest) (*systemv1.UpdateProxyResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	in := proxyFromProto(req.GetProxy())
	if in.ID, err = core.ParseID(req.GetProxy().GetId()); err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := svc.Update(ctx, in, req.GetClearPassword())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.record(ctx, p, "proxy.updated", "%s changed proxy %s", &out)
	return systemv1.UpdateProxyResponse_builder{Proxy: proxyToProto(&out)}.Build(), nil
}

// DeleteProxy deletes a proxy.
func (s *ProxyService) DeleteProxy(ctx context.Context, req *systemv1.DeleteProxyRequest) (*systemv1.DeleteProxyResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	old, err := svc.Get(ctx, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := svc.Delete(ctx, id); err != nil {
		return nil, connectError(ctx, err)
	}
	s.record(ctx, p, "proxy.deleted", "%s deleted proxy %s", &old)
	return &systemv1.DeleteProxyResponse{}, nil
}

// TestProxy fetches a URL through a stored or an unsaved proxy.
func (s *ProxyService) TestProxy(ctx context.Context, req *systemv1.TestProxyRequest) (*systemv1.TestProxyResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	var px core.Proxy
	switch {
	case req.HasId():
		id, err := core.ParseID(req.GetId())
		if err != nil {
			return nil, connectError(ctx, err)
		}
		if px, err = svc.Get(ctx, id); err != nil {
			return nil, connectError(ctx, err)
		}
	case req.HasProxy():
		px = proxyFromProto(req.GetProxy())
		if req.GetProxy().GetId() != "" {
			if px.ID, err = core.ParseID(req.GetProxy().GetId()); err != nil {
				return nil, connectError(ctx, err)
			}
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a proxy or its ID is required"))
	}
	r, err := svc.Test(ctx, px, req.GetUrl())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return systemv1.TestProxyResponse_builder{
		Ok: &r.OK, Error: &r.Err, Status: new(int32(r.Status)), Latency: durationpb.New(r.Latency),
		ExitIp: &r.ExitIP, Country: &r.Country,
	}.Build(), nil
}

func (s *ProxyService) record(ctx context.Context, p auth.Principal, kind, title string, px *core.Proxy) {
	if s.Activity == nil {
		return
	}
	s.Activity.Record(ctx, core.Activity{
		Type: kind, UserID: p.User.ID, Title: fmt.Sprintf(title, p.User.Name, px.Name),
		Attributes: map[string]string{"proxy": px.ID.String()},
	})
}

var proxyKinds = map[systemv1.ProxyKind]core.ProxyKind{
	systemv1.ProxyKind_PROXY_KIND_HTTP:   core.ProxyHTTP,
	systemv1.ProxyKind_PROXY_KIND_SOCKS4: core.ProxySOCKS4,
	systemv1.ProxyKind_PROXY_KIND_SOCKS5: core.ProxySOCKS5,
}

func proxyFromProto(p *systemv1.Proxy) core.Proxy {
	return core.Proxy{
		Name: p.GetName(), Kind: proxyKinds[p.GetKind()], Host: p.GetHost(), Port: int(p.GetPort()),
		Username: p.GetUsername(), Password: p.GetPassword(),
	}
}

func proxyToProto(p *core.Proxy) *systemv1.Proxy {
	var kind systemv1.ProxyKind
	for k, v := range proxyKinds {
		if v == p.Kind {
			kind = k
		}
	}
	return systemv1.Proxy_builder{
		Id: new(p.ID.String()), Name: &p.Name, Kind: &kind, Host: &p.Host, Port: new(int32(p.Port)),
		Username: &p.Username, HasPassword: new(p.Password != ""),
	}.Build()
}

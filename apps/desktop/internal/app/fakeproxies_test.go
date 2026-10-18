package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeProxies serves the server's proxies, whose tests always pass.
type fakeProxies struct {
	systemv1connect.UnimplementedProxyServiceHandler
	f *fakeCore
}

// fakeProxyList are the proxies a fake server starts with: the system
// proxy and one of its own.
func fakeProxyList() []*systemv1.Proxy {
	return []*systemv1.Proxy{
		systemv1.Proxy_builder{
			Id: new(systemProxyID), Name: new("System"), Kind: new(systemv1.ProxyKind_PROXY_KIND_HTTP),
			Host: new("10.0.0.1"), Port: new(int32(3128)), System: new(true), Bypass: []string{"localhost"},
		}.Build(),
		systemv1.Proxy_builder{
			Id: new("p1"), Name: new("Office"), Kind: new(systemv1.ProxyKind_PROXY_KIND_SOCKS5),
			Host: new("proxy.example.com"), Port: new(int32(1080)), Username: new("anna"), HasPassword: new(true),
		}.Build(),
	}
}

func (s *fakeProxies) ListProxies(context.Context, *systemv1.ListProxiesRequest) (*systemv1.ListProxiesResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return systemv1.ListProxiesResponse_builder{Proxies: slices.Clone(s.f.proxies)}.Build(), nil
}

func (s *fakeProxies) CreateProxy(_ context.Context, r *systemv1.CreateProxyRequest) (*systemv1.CreateProxyResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	p := proto.CloneOf(r.GetProxy())
	p.SetId(fmt.Sprintf("p%d", len(s.f.proxies)+1))
	p.SetHasPassword(p.GetPassword() != "")
	p.ClearPassword()
	s.f.proxies = append(s.f.proxies, p)
	return systemv1.CreateProxyResponse_builder{Proxy: p}.Build(), nil
}

func (s *fakeProxies) UpdateProxy(_ context.Context, r *systemv1.UpdateProxyRequest) (*systemv1.UpdateProxyResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	i := slices.IndexFunc(s.f.proxies, func(p *systemv1.Proxy) bool { return p.GetId() == r.GetProxy().GetId() })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no proxy %q", r.GetProxy().GetId()))
	}
	p := proto.CloneOf(r.GetProxy())
	p.SetHasPassword(p.GetPassword() != "" || s.f.proxies[i].GetHasPassword() && !r.GetClearPassword())
	p.ClearPassword()
	s.f.proxies[i] = p
	return systemv1.UpdateProxyResponse_builder{Proxy: p}.Build(), nil
}

func (s *fakeProxies) DeleteProxy(_ context.Context, r *systemv1.DeleteProxyRequest) (*systemv1.DeleteProxyResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.proxies = slices.DeleteFunc(s.f.proxies, func(p *systemv1.Proxy) bool { return p.GetId() == r.GetId() })
	return &systemv1.DeleteProxyResponse{}, nil
}

func (s *fakeProxies) TestProxy(context.Context, *systemv1.TestProxyRequest) (*systemv1.TestProxyResponse, error) {
	return systemv1.TestProxyResponse_builder{
		Ok: new(true), Status: new(int32(200)), Latency: durationpb.New(142 * time.Millisecond),
		ExitIp: new("203.0.113.7"), Country: new("JP"),
	}.Build(), nil
}

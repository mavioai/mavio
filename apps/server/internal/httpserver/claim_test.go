package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

func TestClaimServer(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	h, _ := newHandler(t, nil, func(o *httpserver.Options) {
		c := &cloud.Claimer{Store: o.Store, Client: cloud.NewClient(dev.URL, identity.Generate(), nil)}
		o.Claim = c.Claim
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL)
	claim := func(code string) (*authv1.ClaimServerResponse, error) {
		return public.ClaimServer(ctx, authv1.ClaimServerRequest_builder{ClaimCode: &code}.Build())
	}

	if _, err := claim("wrong"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ClaimServer with an unknown code: %v, want INVALID_ARGUMENT", err)
	}
	dev.AddClaimCode("code", cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	resp, err := claim("code")
	if err != nil || resp.GetUser().GetName() != "alice" || !resp.GetUser().GetAdmin() {
		t.Fatalf("ClaimServer = %v, %v; want alice as the administrator", resp, err)
	}
	info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
	if err != nil || info.GetSetupRequired() || info.GetMode() != authv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV {
		t.Errorf("GetAuthInfo after claiming = %v, %v; want mavio.dev mode, set up", info, err)
	}
	if _, err := claim("code"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("ClaimServer of a claimed server: %v, want FAILED_PRECONDITION", err)
	}
}

func TestClaimServerUnavailable(t *testing.T) {
	h, _ := newHandler(t, nil, func(o *httpserver.Options) {
		c := &cloud.Claimer{Store: o.Store, Client: cloud.NewClient("http://127.0.0.1:1/api", identity.Generate(), nil)}
		o.Claim = c.Claim
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	_, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL).ClaimServer(t.Context(),
		authv1.ClaimServerRequest_builder{ClaimCode: new("code")}.Build())
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("ClaimServer with mavio.dev unreachable: %v, want UNAVAILABLE", err)
	}
}

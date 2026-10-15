package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// claimer returns a claimer of the handler's store at the simulated
// mavio.dev, asking for its requests' outcomes until the test ends.
func claimer(t *testing.T, o *httpserver.Options, dev *cloudtest.Server, key *identity.Key) *cloud.Claimer {
	c := &cloud.Claimer{Store: o.Store, Client: cloud.NewClient(dev.URL, key, nil), PollInterval: 10 * time.Millisecond}
	go c.Run(t.Context())
	o.StartClaim = c.Start
	return c
}

// claim claims the server through StartClaim, confirming its claim
// request for account at the simulated mavio.dev, and waits until the
// server is claimed.
func claim(t *testing.T, public authv1connect.AuthServiceClient, dev *cloudtest.Server, account *cloudv1.Account) {
	t.Helper()
	ctx := t.Context()
	resp, err := public.StartClaim(ctx, authv1.StartClaimRequest_builder{
		RedirectUri: new("http://192.168.1.10:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(), State: new("state"),
	}.Build())
	if err != nil {
		t.Fatalf("StartClaim: %v", err)
	}
	u, err := url.Parse(resp.GetUrl())
	if err != nil {
		t.Fatal(err)
	}
	request := u.Query().Get("request")
	if r, ok := dev.ClaimRequest(request); !ok || r.GetRedirectUri() != "http://192.168.1.10:8686/auth/callback" || r.GetState() != "state" {
		t.Fatalf("claim request %q = %v, %v; want the web UI's redirect URI and state", request, r, ok)
	}
	dev.ConfirmClaimRequest(request, account)
	for range 500 {
		info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if !info.GetSetupRequired() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the server was not claimed once its claim request was confirmed")
}

func TestStartClaim(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	h, _ := newHandler(t, nil, func(o *httpserver.Options) { claimer(t, o, dev, identity.Generate()) })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL)

	claim(t, public, dev, cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	info, err := public.GetAuthInfo(ctx, &authv1.GetAuthInfoRequest{})
	if err != nil || info.GetSetupRequired() || info.GetMode() != authv1.AccountMode_ACCOUNT_MODE_MAVIO_DEV {
		t.Errorf("GetAuthInfo after claiming = %v, %v; want mavio.dev mode, set up", info, err)
	}
	_, err = public.StartClaim(ctx, authv1.StartClaimRequest_builder{
		RedirectUri: new("http://localhost:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(),
	}.Build())
	wantCode(t, "StartClaim of a claimed server", err, connect.CodeFailedPrecondition)
}

func TestStartClaimUnavailable(t *testing.T) {
	h, _ := newHandler(t, nil, func(o *httpserver.Options) {
		o.StartClaim = (&cloud.Claimer{Store: o.Store, Client: cloud.NewClient("http://127.0.0.1:1/api", identity.Generate(), nil)}).Start
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	_, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.URL).StartClaim(t.Context(), authv1.StartClaimRequest_builder{
		RedirectUri: new("http://localhost:8686/auth/callback"), CodeChallenge: new(strings.Repeat("c", 43)),
		DeviceKey: device("browser").GetPublicKey(),
	}.Build())
	wantCode(t, "StartClaim with mavio.dev unreachable", err, connect.CodeUnavailable)
}

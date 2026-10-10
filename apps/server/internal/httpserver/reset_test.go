package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/libs/core"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

func TestPasswordReset(t *testing.T) {
	ctx := t.Context()
	plugin := ""
	pins := make(chan string, 4)
	h, _ := newHandler(t, nil, func(o *httpserver.Options) {
		o.ResetPlugin = func() string { return plugin }
		o.StartReset = func(_ context.Context, pluginID string, user core.User, pin string, _ time.Time) error {
			pins <- pluginID + " " + user.Name + " " + pin
			return nil
		}
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	url := srv.URL
	public := authv1connect.NewAuthServiceClient(http.DefaultClient, url)
	admin := signUp(t, url)
	users := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(admin))
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("ann"), Password: new("old")}.Build()); err != nil {
		t.Fatal(err)
	}
	annToken := login(t, url, "ann", "old", "phone")

	forgot := func(name string) error {
		_, err := public.ForgotPassword(ctx, authv1.ForgotPasswordRequest_builder{Name: &name}.Build())
		return err
	}
	reset := func(name, pin, password string) error {
		_, err := public.ResetPassword(ctx, authv1.ResetPasswordRequest_builder{Name: &name, Pin: &pin, NewPassword: &password}.Build())
		return err
	}

	// Without a password reset plugin, passwords are not reset.
	wantCode(t, "forgot without a plugin", forgot("ann"), connect.CodeFailedPrecondition)

	plugin = "org.example.mail"
	// Unknown users get the same answer, and no PIN.
	if err := forgot("nobody"); err != nil {
		t.Fatalf("ForgotPassword(nobody) = %v", err)
	}
	if err := forgot("ann"); err != nil {
		t.Fatalf("ForgotPassword(ann) = %v", err)
	}
	var pin string
	select {
	case got := <-pins:
		const prefix = "org.example.mail ann "
		if len(got) != len(prefix)+8 || got[:len(prefix)] != prefix {
			t.Fatalf("delivered %q", got)
		}
		pin = got[len(prefix):]
	case <-time.After(10 * time.Second):
		t.Fatal("no PIN delivered")
	}
	wantCode(t, "reset with a wrong PIN", reset("ann", "00000000x", "new"), connect.CodeUnauthenticated)
	wantCode(t, "reset of nobody", reset("nobody", pin, "new"), connect.CodeUnauthenticated)
	if err := reset("ann", pin, "new"); err != nil {
		t.Fatalf("ResetPassword = %v", err)
	}
	wantCode(t, "reset with a used PIN", reset("ann", pin, "newer"), connect.CodeUnauthenticated)

	// The new password signs in; the old sessions ended.
	login(t, url, "ann", "new", "tablet")
	_, err := userv1connect.NewUserServiceClient(http.DefaultClient, url, withToken(annToken)).GetCurrentUser(ctx, &userv1.GetCurrentUserRequest{})
	wantCode(t, "old session", err, connect.CodeUnauthenticated)
	if len(pins) != 0 {
		t.Errorf("%d PINs more delivered", len(pins))
	}
}

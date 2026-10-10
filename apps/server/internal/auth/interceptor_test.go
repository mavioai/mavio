package auth

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestInterceptor(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	newSession := func(name string, disabled bool) string {
		u := core.User{Name: name, PasswordHash: HashPassword("pw"), Disabled: disabled}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		token, hash := NewToken()
		sess := core.AuthSession{UserID: u.ID, TokenHash: hash, DeviceID: "d", LastSeenAt: t0}
		if err := s.AuthSessions().Create(ctx, &sess); err != nil {
			t.Fatal(err)
		}
		return token
	}
	alice := newSession("alice", false)
	bob := newSession("bob", true)

	i := NewInterceptor(s, "/test.v1.Service/Public")
	call := func(procedure, header string, now time.Time) (Principal, bool, error) {
		i.now = func() time.Time { return now }
		h := http.Header{}
		if header != "" {
			h.Set("Authorization", header)
		}
		ctx, err := i.authenticate(ctx, connect.Spec{Procedure: procedure}, h)
		p, ok := FromContext(ctx)
		return p, ok, err
	}
	const private = "/test.v1.Service/Private"

	tests := []struct {
		name, procedure, header string
		want                    connect.Code // 0 for success
		wantUser                string
	}{
		{"public without token", "/test.v1.Service/Public", "", 0, ""},
		{"private without token", private, "", connect.CodeUnauthenticated, ""},
		{"unknown token", private, "Bearer nope", connect.CodeUnauthenticated, ""},
		{"disabled user", private, "Bearer " + bob, connect.CodeUnauthenticated, ""},
		{"valid", private, "Bearer " + alice, 0, "alice"},
	}
	for _, tt := range tests {
		p, ok, err := call(tt.procedure, tt.header, t0)
		if got := connect.CodeOf(err); err != nil && got != tt.want || err == nil && tt.want != 0 {
			t.Errorf("%s: err = %v, want code %v", tt.name, err, tt.want)
		}
		if ok != (tt.wantUser != "") || p.User.Name != tt.wantUser {
			t.Errorf("%s: principal = %q, %v; want %q", tt.name, p.User.Name, ok, tt.wantUser)
		}
	}

	// Activity is recorded at most once per touchInterval.
	lastSeen := func() time.Time {
		sess, err := s.AuthSessions().GetByTokenHash(context.WithoutCancel(ctx), HashToken(alice))
		if err != nil {
			t.Fatal(err)
		}
		return sess.LastSeenAt
	}
	if _, _, err := call(private, "Bearer "+alice, t0.Add(touchInterval/2)); err != nil {
		t.Fatal(err)
	}
	if got := lastSeen(); !got.Equal(t0) {
		t.Errorf("last seen within the interval = %v, want %v", got, t0)
	}
	later := t0.Add(touchInterval)
	p, _, err := call(private, "Bearer "+alice, later)
	if err != nil {
		t.Fatal(err)
	}
	if got := lastSeen(); !got.Equal(later) || !p.Session.LastSeenAt.Equal(later) {
		t.Errorf("last seen after the interval = %v (principal %v), want %v", got, p.Session.LastSeenAt, later)
	}
}

package auth

import (
	"errors"
	"regexp"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestQuickConnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewQuickConnect()
		tv := Device{ID: "tv", Name: "Living room", Client: "Mavio TV", ClientVersion: "1.0"}
		r := q.Start(tv)
		if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(r.Code) || len(r.Secret) != 64 || time.Until(r.Expires) != QuickConnectTimeout {
			t.Fatalf("request = %+v", r)
		}
		if st, err := q.State(r.Secret); err != nil || st.Authorized {
			t.Errorf("state = %+v, %v; want pending", st, err)
		}
		if _, err := q.Claim(r.Secret); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("claim before authorizing: %v, want ErrNotFound", err)
		}

		signedIn := 0
		signIn := func(d Device) (core.AuthSession, string, error) {
			signedIn++
			return core.AuthSession{ID: core.NewID(), DeviceID: d.ID}, "token", nil
		}
		if d, err := q.Authorize(r.Code, signIn); err != nil || d != tv {
			t.Fatalf("Authorize = %+v, %v", d, err)
		}
		if _, err := q.Authorize(r.Code, signIn); !errors.Is(err, core.ErrConflict) || signedIn != 1 {
			t.Errorf("second Authorize: %v, %d sign-ins; want ErrConflict", err, signedIn)
		}
		if st, err := q.State(r.Secret); err != nil || !st.Authorized {
			t.Errorf("state = %+v, %v; want authorized", st, err)
		}
		// The device claims its token once.
		got, err := q.Claim(r.Secret)
		if err != nil || got.Token != "token" || got.Session.DeviceID != "tv" {
			t.Errorf("Claim = %+v, %v", got, err)
		}
		if _, err := q.Claim(r.Secret); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Claim: %v, want ErrNotFound", err)
		}

		// A request expires after the timeout, an authorized one a minute
		// after authorizing.
		pending := q.Start(tv)
		time.Sleep(QuickConnectTimeout)
		if _, err := q.State(pending.Secret); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("expired request: %v, want ErrNotFound", err)
		}
		if _, err := q.Authorize(pending.Code, signIn); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("authorize an expired request: %v, want ErrNotFound", err)
		}
		late := q.Start(tv)
		time.Sleep(QuickConnectTimeout - time.Second)
		if _, err := q.Authorize(late.Code, signIn); err != nil {
			t.Fatal(err)
		}
		time.Sleep(QuickConnectClaimTime - time.Second)
		if _, err := q.State(late.Secret); err != nil {
			t.Errorf("authorized request before its claim time: %v", err)
		}
		time.Sleep(time.Second)
		if _, err := q.Claim(late.Secret); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("claim after the claim time: %v, want ErrNotFound", err)
		}
	})
}

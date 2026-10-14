package auth_test

import (
	"errors"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/auth/passkeytest"
	"github.com/mavioai/mavio/libs/core"
)

func TestRelyingPartyOf(t *testing.T) {
	tests := []struct {
		origin string
		want   auth.RelyingParty
		ok     bool
	}{
		{"https://media.example.com", auth.RelyingParty{ID: "media.example.com", Origin: "https://media.example.com"}, true},
		{"https://media.example.com:8443", auth.RelyingParty{ID: "media.example.com", Origin: "https://media.example.com:8443"}, true},
		{"http://localhost:8096", auth.RelyingParty{ID: "localhost", Origin: "http://localhost:8096"}, true},
		{"http://media.example.com", auth.RelyingParty{}, false},
		{"https://192.168.1.10", auth.RelyingParty{}, false},
		{"https://[::1]:8096", auth.RelyingParty{}, false},
		{"", auth.RelyingParty{}, false},
		{"null", auth.RelyingParty{}, false},
	}
	for _, tt := range tests {
		got, err := auth.RelyingPartyOf(tt.origin)
		if (err == nil) != tt.ok || got != tt.want || err != nil && !errors.Is(err, core.ErrInvalid) {
			t.Errorf("%q: got = %+v, %v, want = %+v", tt.origin, got, err, tt.want)
		}
	}
}

func TestPasskeys(t *testing.T) {
	rp := auth.RelyingParty{ID: "media.example.com", Origin: "https://media.example.com"}
	other := auth.RelyingParty{ID: "other.example.com", Origin: "https://other.example.com"}
	p := auth.NewPasskeys()
	alice := core.User{ID: core.NewID(), Name: "alice", Kind: core.UserMember}
	device := passkeytest.New(rp.Origin)

	register := func(rp auth.RelyingParty, creds []core.Credential) core.Credential {
		t.Helper()
		id, options, err := p.StartRegistration(rp, alice, creds)
		if err != nil {
			t.Fatal(err)
		}
		device.Origin(rp.Origin)
		response, err := device.Create(options)
		if err != nil {
			t.Fatal(err)
		}
		data, err := p.FinishRegistration(id, alice, response)
		if err != nil {
			t.Fatalf("FinishRegistration: %v", err)
		}
		if _, err := p.FinishRegistration(id, alice, response); !errors.Is(err, auth.ErrPasskey) {
			t.Errorf("finishing a registration twice: %v, want ErrPasskey", err)
		}
		return core.Credential{ID: core.NewID(), UserID: alice.ID, Kind: core.CredentialPasskey, Data: data}
	}
	creds := []core.Credential{register(rp, nil)}
	creds = append(creds, register(other, creds))

	lookup := func(id core.ID) (core.User, []core.Credential, error) {
		if id != alice.ID {
			return core.User{}, nil, core.ErrNotFound
		}
		return alice, creds, nil
	}
	signIn := func(rp auth.RelyingParty, origin string) (core.User, core.Credential, error) {
		t.Helper()
		id, options, err := p.StartSignIn(rp)
		if err != nil {
			t.Fatal(err)
		}
		device.Origin(origin)
		response, err := device.Get(options)
		if err != nil {
			t.Fatal(err)
		}
		return p.FinishSignIn(id, response, lookup)
	}
	user, used, err := signIn(rp, rp.Origin)
	if err != nil || user.ID != alice.ID || used.ID != creds[0].ID || used.Data == creds[0].Data {
		t.Fatalf("FinishSignIn = %+v, %+v, %v; want the first passkey with its counter updated", user, used, err)
	}
	creds[0] = used
	if user, used, err := signIn(other, other.Origin); err != nil || user.ID != alice.ID || used.ID != creds[1].ID {
		t.Errorf("FinishSignIn at the other origin = %+v, %+v, %v", user, used, err)
	}
	if _, _, err := signIn(rp, "https://evil.example.com"); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("sign-in from another origin: %v, want ErrPasskey", err)
	}
	if _, _, err := p.FinishSignIn("unknown", []byte("{}"), lookup); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("unknown ceremony: %v, want ErrPasskey", err)
	}
	creds = creds[1:]
	if _, _, err := signIn(rp, rp.Origin); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("sign-in with a removed passkey: %v, want ErrPasskey", err)
	}
}

func TestPasskeyEnrollment(t *testing.T) {
	rp := auth.RelyingParty{ID: "media.example.com", Origin: "https://media.example.com"}
	p := auth.NewPasskeys()
	device := passkeytest.New(rp.Origin)
	invitation := core.NewID()
	bob := core.User{ID: core.NewID(), Name: "bob"}

	enroll := func() (string, []byte) {
		t.Helper()
		id, options, err := p.StartEnrollment(rp, bob, invitation)
		if err != nil {
			t.Fatal(err)
		}
		response, err := device.Create(options)
		if err != nil {
			t.Fatal(err)
		}
		return id, response
	}
	id, response := enroll()
	if _, _, err := p.FinishEnrollment(id, core.NewID(), response); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("enrollment through another invitation: %v, want ErrPasskey", err)
	}
	id, response = enroll()
	if _, err := p.FinishRegistration(id, bob, response); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("enrollment finished as a registration: %v, want ErrPasskey", err)
	}
	id, response = enroll()
	user, data, err := p.FinishEnrollment(id, invitation, response)
	if err != nil || user != bob.ID || data == "" {
		t.Fatalf("FinishEnrollment = %v, %q, %v; want bob's passkey", user, data, err)
	}

	// A registration of an existing user cannot enroll.
	id, options, err := p.StartRegistration(rp, bob, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response, err = device.Create(options); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.FinishEnrollment(id, invitation, response); !errors.Is(err, auth.ErrPasskey) {
		t.Errorf("registration finished as an enrollment: %v, want ErrPasskey", err)
	}
}

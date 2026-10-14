package cloud_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
)

func TestVerifier(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	const serverID = "server-1"
	v := cloud.NewVerifier(dev.URL, serverID, cloud.NewKeySet(dev.URL, nil))
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	sign := func(token string) []byte { return ed25519.Sign(private, cloud.AssertionMessage(token)) }

	token := dev.Assertion(serverID, "acct-1", public)
	a, err := v.Verify(ctx, token, sign(token))
	if err != nil || a.AccountID != "acct-1" || !a.DeviceKey.Equal(public) {
		t.Fatalf("Verify = %+v, %v", a, err)
	}
	if _, err := v.Verify(ctx, token, sign(token)); !errors.Is(err, cloud.ErrInvalidAssertion) {
		t.Errorf("replayed assertion: %v, want ErrInvalidAssertion", err)
	}

	now := time.Now()
	claims := func(change func(*cloud.AssertionClaims)) string {
		c := cloud.AssertionClaims{
			Issuer: dev.URL, Audience: serverID, Subject: "acct-1", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
			DeviceKey: base64.RawURLEncoding.EncodeToString(public), Nonce: rand.Text(),
		}
		change(&c)
		return dev.Sign(c)
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	fresh := dev.Assertion(serverID, "acct-1", public)
	for _, tc := range []struct {
		name      string
		token     string
		signature []byte
	}{
		{"another server", dev.Assertion("server-2", "acct-1", public), nil},
		{"another issuer", claims(func(c *cloud.AssertionClaims) { c.Issuer = "https://example.com" }), nil},
		{"expired", claims(func(c *cloud.AssertionClaims) {
			c.IssuedAt, c.ExpiresAt = now.Add(-5*time.Minute).Unix(), now.Add(-3*time.Minute).Unix()
		}), nil},
		{"valid for an hour", claims(func(c *cloud.AssertionClaims) { c.ExpiresAt = now.Add(time.Hour).Unix() }), nil},
		{"no nonce", claims(func(c *cloud.AssertionClaims) { c.Nonce = "" }), nil},
		{"signed by another device", fresh, ed25519.Sign(other, cloud.AssertionMessage(fresh))},
		{"tampered", token[:len(token)-4] + "AAAA", nil},
		{"not a JWT", "assertion", nil},
	} {
		sig := tc.signature
		if sig == nil {
			sig = sign(tc.token)
		}
		if _, err := v.Verify(ctx, tc.token, sig); !errors.Is(err, cloud.ErrInvalidAssertion) {
			t.Errorf("%s: %v, want ErrInvalidAssertion", tc.name, err)
		}
	}
}

func TestVerifierUnreachable(t *testing.T) {
	dev := cloudtest.New(t)
	v := cloud.NewVerifier("http://127.0.0.1:1/api", "server-1", cloud.NewKeySet("http://127.0.0.1:1/api", nil))
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	token := dev.Assertion("server-1", "acct-1", public)
	_, err := v.Verify(t.Context(), token, ed25519.Sign(private, cloud.AssertionMessage(token)))
	if err == nil || errors.Is(err, cloud.ErrInvalidAssertion) {
		t.Errorf("Verify with mavio.dev unreachable: %v, want a connection error", err)
	}
}

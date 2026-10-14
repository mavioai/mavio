package certs_test

import (
	"context"
	"crypto/x509"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/certs"
	"github.com/mavioai/mavio/apps/server/internal/certs/certstest"
	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// TestThroughMavioDev obtains a claimed server's certificate with
// mavio.dev publishing the challenge and giving the CA's binding.
func TestThroughMavioDev(t *testing.T) {
	ctx := t.Context()
	dev := cloudtest.New(t)
	key := identity.Generate()
	client := cloud.NewClient(dev.URL, key, nil)
	dev.AddClaimCode("code", cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	if _, err := client.ClaimServer(ctx, cloudv1.ClaimServerRequest_builder{ClaimCode: new("code"), PublicKey: key.Public()}.Build()); err != nil {
		t.Fatal(err)
	}
	ca := certstest.New()
	ca.TXT, ca.BindingKeyID = dev.TXT, cloudtest.BindingKeyID
	defer ca.Close()

	m := certs.NewManager(filepath.ToSlash(t.TempDir()), certs.Name(key.ID()), client, func(context.Context) (bool, error) { return true, nil }, nil)
	m.CAs, m.PropagationDelay = []string{ca.Directory()}, time.Millisecond
	ready := make(chan struct{})
	m.Changed = func() { close(ready) }
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.Run(ctx)
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("no certificate")
	}
	cert, _ := m.Certificate(nil)
	if _, err := cert.Leaf.Verify(x509.VerifyOptions{DNSName: "10-0-0-2." + key.ID() + ".mavio.site", Roots: ca.Roots()}); err != nil {
		t.Errorf("Verify() = %v", err)
	}
	if txt := dev.TXT("_acme-challenge." + key.ID() + ".mavio.site"); len(txt) != 0 {
		t.Errorf("TXT after the challenge = %q, want none", txt)
	}
}

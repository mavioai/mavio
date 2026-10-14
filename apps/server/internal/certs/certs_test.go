package certs

import (
	"context"
	"crypto/x509"
	"math/big"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/certs/certstest"
)

const id = "abcdefghijklmnopqrstuvwxyz234567"

func newManager(t *testing.T, dir string, ch *certstest.Challenges, cas ...*certstest.CA) *Manager {
	t.Helper()
	m := NewManager(dir, Name(id), ch, func(context.Context) (bool, error) { return true, nil }, nil)
	m.CAs = nil
	for _, ca := range cas {
		m.CAs = append(m.CAs, ca.Directory())
	}
	m.PropagationDelay = time.Millisecond
	return m
}

func TestObtainFallsBackAndBinds(t *testing.T) {
	ch := &certstest.Challenges{ID: id, KeyID: "kid-1"}
	broken := certstest.New()
	broken.TXT = func(string) []string { return nil } // never sees the records
	defer broken.Close()
	ca := certstest.New()
	ca.TXT, ca.BindingKeyID = ch.TXT, "kid-1"
	defer ca.Close()
	dir := filepath.ToSlash(t.TempDir())
	m := newManager(t, dir, ch, broken, ca)
	changed := make(chan struct{}, 4)
	m.Changed = func() { changed <- struct{}{} }

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(done)
	}()
	select {
	case <-changed:
	case <-time.After(10 * time.Second):
		t.Fatal("no certificate")
	}
	cancel()
	<-done

	cert, err := m.Certificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cert.Leaf.Verify(x509.VerifyOptions{DNSName: "192-168-1-10." + id + ".mavio.site", Roots: ca.Roots()}); err != nil {
		t.Errorf("Verify() = %v", err)
	}
	if got, want := []int{ch.Set, ch.Cleared, ch.Bindings, ca.Issued, broken.Issued}, []int{2, 2, 1, 1, 0}; !slices.Equal(got, want) {
		t.Errorf("set, cleared, bindings, issued, broken issued = %v, want = %v", got, want)
	}

	// A new manager serves the stored certificate, and renews it only
	// when due.
	again := newManager(t, dir, ch, ca)
	if err := again.load(); err != nil || !again.Ready() {
		t.Fatalf("load() = %v, ready = %v", err, again.Ready())
	}
	if at, want := again.renewAt(t.Context()), cert.Leaf.NotBefore.Add(60*24*time.Hour); at.Sub(want).Abs() > time.Minute {
		t.Errorf("renewAt() = %v, want = %v", at, want)
	}
}

func TestRenewalInformation(t *testing.T) {
	ch := &certstest.Challenges{ID: id}
	ca := certstest.New()
	ca.TXT = ch.TXT
	defer ca.Close()
	m := newManager(t, filepath.ToSlash(t.TempDir()), ch, ca)
	if err := m.obtain(t.Context()); err != nil {
		t.Fatal(err)
	}
	// The CA suggests renewing now, long before two thirds of the lifetime.
	now := time.Now()
	ca.Window = func() (time.Time, time.Time) { return now.Add(-time.Hour), now.Add(-time.Minute) }
	if at := m.renewAt(t.Context()); !at.Before(now) {
		t.Errorf("renewAt() = %v, want before %v", at, now)
	}
	// Accounts are registered once.
	if err := m.obtain(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := []int{ca.Issued, ca.Accounts()}; !slices.Equal(got, []int{2, 1}) {
		t.Errorf("issued, accounts = %v, want = [2 1]", got)
	}
}

func TestCertID(t *testing.T) {
	// RFC 9773 §4.1's example.
	leaf := &x509.Certificate{
		AuthorityKeyId: []byte{0x69, 0x88, 0x5b, 0x6b, 0x87, 0x46, 0x40, 0x41, 0xe1, 0xb3, 0x7b, 0x84, 0x7b, 0xa0, 0xae, 0x2c, 0xde, 0x01, 0xc8, 0xd4},
	}
	leaf.SerialNumber, _ = new(big.Int).SetString("0087654321", 16)
	if got, want := certID(leaf), "aYhba4dGQEHhs3uEe6CuLN4ByNQ.AIdlQyE"; got != want {
		t.Errorf("certID() = %q, want = %q", got, want)
	}
}

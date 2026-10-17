package app

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// shots is where the screens tests write their frames, when set.
var shots = os.Getenv("MAVIO_SHOTS")

func TestSignIn(t *testing.T) {
	h := newHarness(t, false, 1280, 800)
	if h.app.phase != phaseSignIn {
		t.Fatalf("phase = %v, want = sign-in", h.app.phase)
	}
	h.save(shots, "sign-in")
	h.tt.SetDark(true)
	h.settle()
	h.save(shots, "sign-in-dark")
	h.tt.SetDark(false)
	if err := h.tt.Click("Connect to a Server"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.save(shots, "sign-in-address")
	h.tt.Type("mavio.test")
	if err := h.tt.Click("Continue"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.app.signIn.step != stepCredentials {
		t.Fatalf("step = %v, want = credentials", h.app.signIn.step)
	}
	h.save(shots, "sign-in-credentials")
}

// A server in the Mavio account mode sends its address back to the Mavio
// account's sign-in, at its mavio.dev.
func TestSignInMavioServer(t *testing.T) {
	h := newHarness(t, false, 1280, 800)
	if err := h.tt.Click("Connect to a Server"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.tt.Type("account.test")
	if err := h.tt.Click("Continue"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.app.signIn.step != stepMavio {
		t.Fatalf("step = %v, want = the Mavio account's", h.app.signIn.step)
	}
	if h.tt.HasText("Connect to a Server") {
		t.Errorf("texts = %q, want no Connect to a Server", h.tt.Texts())
	}
	if got, want := h.app.mavioAPI(), "https://mavio.dev/api"; got != want {
		t.Errorf("mavioAPI() = %q, want = %q", got, want)
	}
	h.save(shots, "sign-in-mavio-server")
}

func TestSignInLink(t *testing.T) {
	h := newHarness(t, false, 1280, 900)
	if err := h.tt.Click("Use a Link Code"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.app.signIn.link.GetUserCode() != "BDWP-HQPK" || h.app.signIn.linkQR == nil {
		t.Fatalf("link = %v, want = the code and its QR code", h.app.signIn.link)
	}
	h.save(shots, "sign-in-link")
}

func TestSignInTV(t *testing.T) {
	h := newHarness(t, false, 1920, 1080)
	h.app.prefs.TV = true
	h.tt.Frame()
	h.settle()
	if h.app.signIn.step != stepLink || h.app.signIn.link == nil {
		t.Fatalf("step = %v, want = the link code", h.app.signIn.step)
	}
	h.tt.SetDark(true)
	h.settle()
	h.save(shots, "sign-in-tv")
}

func TestHome(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	if h.app.phase != phaseReady {
		t.Fatalf("phase = %v, want = ready", h.app.phase)
	}
	if !h.tt.HasText("Continue Watching") {
		t.Errorf("texts = %q, want Continue Watching", h.tt.Texts())
	}
	h.save(shots, "home")
	h.tt.SetDark(true)
	h.settle()
	h.save(shots, "home-dark")
}

func TestScreens(t *testing.T) {
	for _, tc := range []struct{ name, route string }{
		{"library", "/library/%s/lib-movies"},
		{"details", "/item/%s/orbit"},
		{"search", "/search"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, true, 1280, 800)
			route := tc.route
			if strings.Contains(route, "%s") {
				route = fmt.Sprintf(route, h.core.s.Server.ID)
			}
			h.app.router.Push(route)
			h.settle()
			h.save(shots, tc.name)
		})
	}
}

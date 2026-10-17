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
	if err := h.tt.Click("Server"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("mavio.test")
	if err := h.tt.Click("Continue"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.save(shots, "sign-in-credentials")
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

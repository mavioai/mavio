package app

import (
	"slices"
	"testing"
)

func TestAssistant(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	server := h.core.s.Server.ID
	send := func(text string) {
		t.Helper()
		h.app.assistant.text = text
		h.settle()
		if err := h.tt.Click("Send"); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	has := func(when string, texts ...string) {
		t.Helper()
		for _, want := range texts {
			if !h.tt.HasText(want) {
				t.Errorf("%s: no %q; texts = %q", when, want, h.tt.Texts())
			}
		}
	}

	if err := h.tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	has("the panel opened", "Try saying", "Play The Long Orbit", "Space films")
	h.save(shots, "assistant")

	// The server's answer plays the title, its tools shown as they ran.
	send("Play The Long Orbit")
	p := h.app.playerOf(nil)
	if p == nil || p.item != "orbit" {
		t.Fatalf("the answer played %v, want = orbit", p)
	}
	has("playing", "You", "Searched the library", "Playing The Long Orbit.")
	h.save(shots, "assistant-play")
	// Each message has a bar copying it.
	if err := h.tt.Click("Copy"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	has("copied", "Copied")

	// The fast path pauses the player with no request to the server.
	asked := len(h.core.assistant.requests())
	send("pause")
	if got := len(h.core.assistant.requests()); got != asked {
		t.Errorf("pause asked the server")
	}
	h.video.mu.Lock()
	paused := slices.Contains(h.video.commands, "set pause yes")
	h.video.mu.Unlock()
	if !paused {
		t.Errorf("pause did not pause the video; commands = %q", h.video.commands)
	}
	has("pausing", "Paused.")
	a := h.core.assistant.requests()[asked-1]
	if got := a.GetContext().GetScreen(); got != "home" {
		t.Errorf("screen = %q, want = home", got)
	}
	h.app.closePlayer(p, true)
	h.settle()

	// Results to choose from, the one chosen opened.
	send("Space films")
	has("results", "Which one?", "The Lantern Keeper")
	h.save(shots, "assistant-results")
	if err := h.tt.Click("2. The Lantern Keeper"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.app.router.Path(), "/item/"+server+"/lantern"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
	if got := h.core.assistant.requests(); got[len(got)-1].GetChoice() != "lantern" {
		t.Errorf("the choice was not sent")
	}

	// A confirmation, then the answer streamed.
	send("request dune")
	has("confirming", "Request Dune?")
	h.save(shots, "assistant-confirm")
	if err := h.tt.Click("Confirm"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	has("confirmed", "Requested Dune.")

	if err := h.tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.app.assistant.open {
		t.Errorf("the panel stayed open")
	}
}

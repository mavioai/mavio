package app

import (
	"fmt"
	"testing"
	"time"
)

func TestDetailsFromServer(t *testing.T) {
	// Tall enough to show the whole page.
	h := newHarness(t, true, 1280, 2600)
	server := h.core.s.Server.ID
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	for _, want := range []string{
		"Commander Hale", "Director", "Writers", "Trailers & Extras", "Official Trailer", "The Orbit Collection",
		"Northlight Pictures", "Audience", "7.8", "91%", "space station", "1080p · HEVC", "orbit.mkv\n4.2 GB", "IMDb", "View Full Crew",
	} {
		if !h.tt.HasText(want) {
			t.Errorf("details have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-full")

	if err := h.tt.Click("Official Trailer"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := h.app.playerOf(nil); got == nil || got.item != "orbit-trailer" {
		t.Errorf("player = %+v, want = the trailer playing", got)
	}
	h.app.closePlayer(h.app.playerOf(nil), true)
	h.settle()

	if err := h.tt.Click("View Full Crew"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.app.router.Path(), "/crew/"+server+"/orbit"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
	if h.tt.HasText("Commander Hale") || !h.tt.HasText("Inès Varga") {
		t.Errorf("crew = %q, want = the director and writer alone", h.tt.Texts())
	}
	h.app.router.Back()
	h.settle()

	if err := h.tt.Click("See All"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"Cast", "Commander Hale", "Theo", "Director", "Inès Varga", "Writers"} {
		if !h.tt.HasText(want) {
			t.Errorf("credits have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "credits")

	if err := h.tt.Click("Mara Ellison"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.app.router.Path(), "/person/"+server+"/mara"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
	for _, want := range []string{"Mara Ellison", fmt.Sprintf("Born March 12, 1984 (age %d)", yearsBetween(time.Date(1984, 3, 12, 0, 0, 0, 0, time.UTC), time.Now())), "Halifax, Nova Scotia, Canada", "TMDB", "Movies", "TV Shows", "Episodes", "Dead Reckoning"} {
		if !h.tt.HasText(want) {
			t.Errorf("person has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "person")
}

func TestDetailsOffline(t *testing.T) {
	h := newHarness(t, true, 1280, 2000)
	// The server's item is not to be had: the core's details show alone.
	h.core.api = nil
	h.app.router.Push("/item/" + h.core.s.Server.ID + "/orbit")
	h.settle()
	if !h.tt.HasText("The Long Orbit") || h.tt.HasText("Trailers & Extras") {
		t.Errorf("texts = %q, want = the core's details without extras", h.tt.Texts())
	}
}

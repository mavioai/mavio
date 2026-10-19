package app

import (
	"fmt"
	"slices"
	"strings"
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

func TestDetailsMore(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	server := h.core.s.Server.ID
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	if err := h.tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := h.tt.Menu(), []string{
		"Play On…", "Report a Problem…", "Add to Playlist", "Add to Collection", "Edit Metadata", "Identify", "Refresh Metadata", "Edit Images",
	}; !slices.Equal(got, want) {
		t.Errorf("menu = %q, want = %q", got, want)
	}
	if err := h.tt.ChooseMenuItem("Add to Playlist"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.settle()
	if err := h.tt.Click("Night Drives"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Added The Long Orbit to Night Drives") {
		t.Errorf("dialog = %q, want = the item added", h.tt.Texts())
	}
	h.save(shots, "details-playlist")
	if err := h.tt.Click("New playlist"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("Space Operas")
	h.settle()
	if err := h.tt.Click("Create"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.settle()
	h.core.mu.Lock()
	var got []string
	for _, p := range h.core.playlists {
		got = append(got, p.name+":"+strings.Join(p.items, ","))
	}
	h.core.mu.Unlock()
	if want := []string{"Night Drives:orbit", "Space Operas:orbit"}; !slices.Equal(got, want) {
		t.Errorf("playlists = %q, want = %q", got, want)
	}
	if !h.tt.HasText("Space Operas") {
		t.Errorf("dialog = %q, want = the new playlist listed", h.tt.Texts())
	}
	if err := h.tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	h.settle()

	if err := h.tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ChooseMenuItem("Add to Collection"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.settle()
	if err := h.tt.Click("Space Classics"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.core.mu.Lock()
	members := h.core.collections["space-classics"].items
	h.core.mu.Unlock()
	if !slices.Equal(members, []string{"orbit"}) {
		t.Errorf("collection = %q, want = the item added", members)
	}
}

func TestPlayOn(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	server := h.core.s.Server.ID
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	if err := h.tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ChooseMenuItem("Play On…"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.settle()
	if h.tt.HasText("MacBook Pro") || !h.tt.HasText("Living Room TV") {
		t.Errorf("devices = %q, want = the television alone", h.tt.Texts())
	}
	if err := h.tt.Click("Living Room TV"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.settle()
	for _, want := range []string{"The Long Orbit", "47:12", "2:04:00", "Stop"} {
		if !h.tt.HasText(want) {
			t.Errorf("remote control has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "remote")
	if err := h.tt.Click("Pause"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.Click("Message to show"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("Dinner is ready")
	h.settle()
	if err := h.tt.Click("Send"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Sent") {
		t.Errorf("remote control = %q, want = the message sent", h.tt.Texts())
	}
	h.core.mu.Lock()
	var got []string
	for _, c := range h.core.commands {
		cmd := c.GetCommand()
		var s string
		switch {
		case cmd.HasPlay():
			s = "play " + strings.Join(cmd.GetPlay().GetItemIds(), ",")
		case cmd.HasPlayState():
			s = cmd.GetPlayState().GetCommand().String()
		case cmd.HasMessage():
			s = "message " + cmd.GetMessage().GetText()
		}
		got = append(got, c.GetSessionId()[len(c.GetSessionId())-1:]+" "+s)
	}
	h.core.mu.Unlock()
	if want := []string{"2 play orbit", "2 PLAY_STATE_COMMAND_PAUSE", "2 message Dinner is ready"}; !slices.Equal(got, want) {
		t.Errorf("commands = %q, want = %q", got, want)
	}
}

func TestDetailsMetadata(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	server := h.core.s.Server.ID
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	choose := func(item string) {
		t.Helper()
		if err := h.tt.Click("More"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if err := h.tt.ChooseMenuItem(item); err != nil {
			t.Fatal(err)
		}
		h.settle()
		h.settle()
	}

	choose("Edit Metadata")
	for _, want := range []string{"Edit The Long Orbit", "Original Title", "Tagline", "Runtime (minutes)", "TMDB", "IMDb", "Lock All Metadata", "Cast and Crew"} {
		if !h.tt.HasText(want) {
			t.Errorf("edit dialog has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "metadata-edit")
	// The field is below its label.
	r, ok := h.tt.Find("Tagline")
	if !ok {
		t.Fatal("no Tagline")
	}
	h.tt.ClickAt(r.X+20, r.Y+r.H+16)
	h.settle()
	h.tt.Type("Nothing orbits forever")
	h.settle()
	if err := h.tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.core.mu.Lock()
	updates := h.core.metadata.updates
	h.core.mu.Unlock()
	if len(updates) != 1 || !slices.Equal(updates[0].GetUpdateMask().GetPaths(), []string{"tagline"}) || !strings.HasSuffix(updates[0].GetMetadata().GetTagline(), "Nothing orbits forever") {
		t.Errorf("updates = %v, want = the tagline changed", updates)
	}
	if h.tt.HasText("Edit The Long Orbit") {
		t.Errorf("edit dialog still shows after saving")
	}

	choose("Identify")
	h.settle()
	if !h.tt.HasText("The Long Orbit (2024)") || !h.tt.HasText("TMDB · tmdb 4242") {
		t.Errorf("identify dialog = %q, want = the provider's result", h.tt.Texts())
	}
	h.save(shots, "metadata-identify")
	if err := h.tt.Click("Identify"); err != nil {
		t.Fatal(err)
	}
	h.settle()

	choose("Refresh Metadata")
	// The switch is at the end of its label's row, 400 points wide.
	r, ok = h.tt.Find("Replace All Metadata")
	if !ok {
		t.Fatal("no Replace All Metadata")
	}
	h.tt.ClickAt(r.X+400-16, r.Y+r.H/2)
	h.settle()
	if err := h.tt.Click("Refresh Metadata"); err != nil {
		t.Fatal(err)
	}
	h.settle()

	choose("Edit Images")
	h.settle()
	if !h.tt.HasText("TMDB · 2000×3000 · en") {
		t.Errorf("images dialog = %q, want = the provider's image", h.tt.Texts())
	}
	h.save(shots, "metadata-images")
	if err := h.tt.Click("TMDB"); err != nil {
		t.Fatal(err)
	}
	h.settle()

	h.core.mu.Lock()
	defer h.core.mu.Unlock()
	md := h.core.metadata
	if len(md.identified) != 1 || md.identified[0].GetExternalIds()["tmdb"] != "4242" {
		t.Errorf("identified = %v, want = tmdb 4242", md.identified)
	}
	if len(md.refreshed) != 1 || !md.refreshed[0].GetReplaceMetadata() {
		t.Errorf("refreshed = %v, want = replacing all", md.refreshed)
	}
	if len(md.images) != 1 || !strings.HasSuffix(md.images[0].GetUrl(), "/remote-poster.jpg") || md.images[0].GetKind().String() != "IMAGE_KIND_PRIMARY" {
		t.Errorf("images = %v, want = the provider's poster", md.images)
	}
}

package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

func TestTVMode(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	focused := func(label, after string) {
		t.Helper()
		if !h.tt.Focused(label) {
			t.Fatalf("after %s: %q has no focus; texts = %q", after, label, h.tt.Texts())
		}
	}
	for _, tab := range []string{"Home", "Libraries", "Search"} {
		if _, ok := h.tt.Find(tab); !ok {
			t.Fatalf("no tab %q; texts = %q", tab, h.tt.Texts())
		}
	}
	if _, ok := h.tt.Find("Back"); ok {
		t.Errorf("the TV mode shows Back")
	}
	h.save(shots, "tv-home")

	// The page opens with focus on its first element, the hero's.
	focused("Play", "opening Home")
	key(ui.KeyRight)
	focused("Details", "right from Play")
	// Up from the page's first row is the current tab.
	key(ui.KeyUp)
	focused("Home", "up from the hero")
	// On a tab's page, focus on another tab opens it in its place.
	key(ui.KeyRight)
	focused("Libraries", "right from Home")
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("path = %q, want = /libraries", got)
	}
	h.save(shots, "tv-libraries")
	// Back from a tab goes to Home's.
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from a tab: path = %q, want = /home", got)
	}

	// Down into the rows, and a card opened and left with back.
	key(ui.KeyDown)
	key(ui.KeyDown)
	h.save(shots, "tv-home-row")
	key(ui.KeyEnter)
	if got := h.app.router.Path(); got == "/home" {
		t.Fatalf("Return on a card opened nothing")
	}
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from the card's page: path = %q, want = /home", got)
	}

	// Search returns to where it was opened from.
	h.app.router.Push("/libraries")
	h.settle()
	h.app.router.Push("/search")
	h.settle()
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("back from Search: path = %q, want = /libraries", got)
	}
	h.app.router.Push("/home")
	h.settle()

	// The user menu opens with focus on Settings.
	h.app.userMenuOpen = true
	h.settle()
	focused("Settings", "opening the menu")
	h.save(shots, "tv-menu")
	key(ui.KeyEscape)
	if h.app.userMenuOpen {
		t.Errorf("back left the menu open")
	}

	// Videos play in the app's window, which a remote stays in.
	h.app.opts.NewWindow = func(_ func(*ui.Context), closed func()) Window { return &fakeWindow{closed: closed} }
	h.app.play(h.core.s.Server.ID, "orbit", nil)
	h.settle()
	if h.app.playerOf(nil) == nil {
		t.Errorf("the TV mode played in a window of the player's")
	}
}

func TestTVDetailsAndLibrary(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	server := h.core.s.Server.ID

	// A title's details hide the bar, and open with focus on Play.
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	if _, ok := h.tt.Find("Libraries"); ok {
		t.Errorf("the details show the tabs")
	}
	if !h.tt.Focused("Play") && !h.tt.Focused("Resume") {
		t.Fatalf("the details opened without focus on their primary action")
	}
	h.save(shots, "tv-details")
	for range 3 {
		key(ui.KeyDown)
	}
	h.save(shots, "tv-details-down")
	// Down into the details at the foot, a region in the page's flow, at
	// its first column's first row (docs/tv.md §6.2).
	for range 8 {
		if h.tt.Focused("Inès Varga") {
			break
		}
		key(ui.KeyDown)
	}
	if !h.tt.Focused("Inès Varga") {
		t.Fatalf("down the details reached no leading crew")
	}
	h.save(shots, "tv-details-about")
	// View Full Crew is reached by down only from the last of a row.
	key(ui.KeyDown)
	if !h.tt.Focused("Inès Varga") {
		t.Errorf("down from the first of the crew left it")
	}
	key(ui.KeyRight)
	if !h.tt.Focused("Jonah Pike") {
		t.Fatalf("right from the first of the crew: Jonah Pike has no focus")
	}
	key(ui.KeyDown)
	if !h.tt.Focused("View Full Crew") {
		t.Fatalf("down from the last of the crew: View Full Crew has no focus")
	}
	key(ui.KeyUp)
	key(ui.KeyUp)
	if h.tt.Focused("Inès Varga") || h.tt.Focused("Jonah Pike") || h.tt.Focused("View Full Crew") {
		t.Errorf("up from the region's first row stayed in it")
	}
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/home" {
		t.Fatalf("back from the details: path = %q, want = /home", got)
	}

	// A library opens with focus on Recommended; focus on Library shows
	// the grid of all its items.
	h.app.openPlace("/libraries")
	h.settle()
	key(ui.KeyDown)
	key(ui.KeyEnter)
	if !h.tt.Focused("Recommended") {
		t.Fatalf("the library opened without focus on Recommended; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-library")
	key(ui.KeyRight)
	if !h.tt.Focused("Library") {
		t.Fatalf("right from Recommended: Library has no focus; texts = %q", h.tt.Texts())
	}
	for k, st := range h.app.libraryStates {
		if st.tab != 1 {
			t.Errorf("%s: focus on Library left Recommended shown", k)
		}
	}
	h.save(shots, "tv-library-all")

	// The order is a menu of the app's own, opening with focus on the
	// current one; choosing returns focus to its button, as back does
	// (docs/tv.md §5).
	for range 6 {
		if h.tt.Focused("Sort: Name") {
			break
		}
		key(ui.KeyRight)
	}
	if !h.tt.Focused("Sort: Name") {
		t.Fatalf("right along the filters: the order has no focus; texts = %q", h.tt.Texts())
	}
	key(ui.KeyEnter)
	if !h.tt.Focused("Name") {
		t.Fatalf("the order's menu opened without focus on Name; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-library-sort")
	key(ui.KeyDown)
	key(ui.KeyEnter)
	if !h.tt.Focused("Sort: Date Added") {
		t.Errorf("choosing Date Added left focus off the order; texts = %q", h.tt.Texts())
	}
	for k, st := range h.app.libraryStates {
		if st.sort != capiv1.Sort_SORT_DATE_ADDED {
			t.Errorf("%s: sort = %v, want = %v", k, st.sort, capiv1.Sort_SORT_DATE_ADDED)
		}
	}
	key(ui.KeyEnter)
	if !h.tt.Focused("Date Added") {
		t.Fatalf("the order's menu opened without focus on Date Added")
	}
	key(ui.KeyEscape)
	if !h.tt.Focused("Sort: Date Added") || h.app.menuOpen() {
		t.Errorf("back left the menu open or focus off its button")
	}
	if got := h.app.router.Path(); !strings.HasPrefix(got, "/library/") {
		t.Fatalf("back closing the menu left the library: path = %q", got)
	}
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("back from a library: path = %q, want = /libraries", got)
	}
}

func TestTVSettings(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	path := func(want, after string) {
		t.Helper()
		if got := h.app.router.Location(); got != want {
			t.Fatalf("after %s: location = %q, want = %q", after, got, want)
		}
	}

	// The settings are a list, opening with focus on the account.
	h.app.router.Push("/settings")
	h.settle()
	if _, ok := h.tt.Find("Libraries"); ok {
		t.Errorf("the settings show the tabs")
	}
	if h.app.focus.focused() != 0 {
		t.Fatalf("the settings opened without focus on the account; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-settings")
	key(ui.KeyDown)
	key(ui.KeyEnter)
	section := h.app.router.Location()
	if settingsDepth(section) < 1 {
		t.Fatalf("Return on the second row opened %q", section)
	}
	h.save(shots, "tv-settings-section")
	key(ui.KeyEscape)
	path("/settings", "back from a section")
	key(ui.KeyEscape)
	path("/home", "back from the settings")
}

func TestSettingsDepth(t *testing.T) {
	for _, tc := range []struct {
		loc    string
		depth  int
		parent string
	}{
		{"/settings", 0, "/settings"},
		{"/settings/media", 1, "/settings"},
		{"/settings/appearance", 2, "/settings"},
		{"/settings/libraries", 2, "/settings/media"},
		{"/settings/users?user=u1", 3, "/settings/users"},
		{"/settings/acquisition?acquisition=formats", 3, "/settings/acquisition"},
		{"/settings/acquisition?acquisition=format&id=7", 4, "/settings/acquisition?acquisition=formats"},
		{"/home", -1, "/settings"},
	} {
		if got := settingsDepth(tc.loc); got != tc.depth {
			t.Errorf("settingsDepth(%q) = %d, want = %d", tc.loc, got, tc.depth)
		}
		if tc.depth < 0 {
			continue
		}
		if got := settingsParent(tc.loc); got != tc.parent {
			t.Errorf("settingsParent(%q) = %q, want = %q", tc.loc, got, tc.parent)
		}
	}
}

func TestTVKeyboard(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.app.router.Push("/settings/general")
	for range 40 {
		if h.tt.HasText("Server name") && h.tt.HasText("Metadata language") {
			break
		}
		h.settle()
	}
	// The section opens on its first element, the server name, though it
	// may load after the selects below it.
	for range 10 {
		if h.tt.Focused("Server name") {
			break
		}
		h.settle()
	}
	if !h.tt.Focused("Server name") {
		t.Fatalf("the settings did not open with focus on the server name")
	}
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	// The section opens with focus on its first field, and Return opens
	// the keyboard on it.
	key(ui.KeyEnter)
	k := &h.app.tvKeys
	if !k.open {
		t.Fatalf("Return on the field opened no keyboard; texts = %q", h.tt.Texts())
	}
	field, original := k.field.Label, k.original
	if !h.tt.Focused("1") {
		t.Fatalf("the keyboard opened without focus on its first key")
	}
	h.save(shots, "tv-keyboard")
	// A key types into the field as it is typed.
	key(ui.KeyEnter)
	if got, want := k.text, original+"1"; got != want {
		t.Fatalf("after a key: text = %q, want = %q", got, want)
	}
	// A computer's keyboard types into it as well, Space a space rather
	// than pressing the key with focus.
	h.tt.Type("Ab")
	h.tt.TypeKey(0, ui.KeySpace, " ")
	h.settle()
	if got, want := k.text, original+"1Ab "; got != want {
		t.Fatalf("after typing: text = %q, want = %q", got, want)
	}
	for range 3 {
		key(ui.KeyBackspace)
	}
	// Back deletes a character, and closes the keyboard once there is
	// none, putting back what the field held.
	key(ui.KeyBackspace)
	if got := k.text; got != original {
		t.Fatalf("after Back: text = %q, want = %q", got, original)
	}
	for range len([]rune(original)) + 1 {
		key(ui.KeyEscape)
	}
	if k.open {
		t.Fatalf("Back on no text left the keyboard open")
	}
	if !h.tt.Focused(field) {
		t.Errorf("closing the keyboard left focus off %q", field)
	}
	if got := h.app.router.Path(); got != "/settings/general" {
		t.Errorf("Back in the keyboard left the section for %q", got)
	}
	// Back on the section, focus returns to the element it was on.
	first := h.app.focus.focused()
	key(ui.KeyDown)
	was := h.app.focus.focused()
	if was == first {
		t.Fatalf("Down from the field kept focus on it")
	}
	h.app.router.Push("/settings/appearance")
	h.settle()
	h.app.router.Go(-1)
	h.settle()
	if got := h.app.focus.focused(); got != was {
		t.Errorf("back on the section: focused = %d, want = %d", got, was)
	}
}

// Discover's title page moves as the web UI's (docs/tv.md §6.3): its
// genres, actions and overview are regions of the page's flow, down from
// one to the next one's first element, and from the overview to the cast.
func TestTVDiscoveredTitle(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	focused := func(label string) {
		t.Helper()
		if !h.tt.Focused(label) {
			got := ""
			for _, s := range h.tt.Texts() {
				if h.tt.Focused(s) {
					got = s
				}
			}
			t.Fatalf("focus = %q, want = %q; texts = %q", got, label, h.tt.Texts())
		}
	}
	server := h.app.servers[0].GetId()
	h.app.router.Push("/title/" + server + "/movie/101")
	for range 20 {
		if h.tt.HasText("91%") && h.tt.HasText("Starfall") {
			break
		}
		h.settle()
	}
	focused("Watch")
	h.save(shots, "tv-discover-title")
	key(ui.KeyUp)
	focused("Science Fiction")
	key(ui.KeyRight)
	focused("Drama")
	key(ui.KeyDown)
	focused("Watch")
	key(ui.KeyDown)
	focused("Iris Okafor")
	h.save(shots, "tv-discover-title-overview")
	key(ui.KeyDown)
	focused("View Full Crew")
	key(ui.KeyDown)
	focused("space")
	key(ui.KeyDown)
	focused("Mara Quinn")
	key(ui.KeyUp)
	focused("space")
	// Up from the first of a row passes View Full Crew.
	key(ui.KeyUp)
	focused("Iris Okafor")
	// Right crosses to the box of facts beside, and left back to what is
	// marked beside.
	key(ui.KeyRight)
	if h.tt.Focused("Iris Okafor") || h.tt.Focused("View Full Crew") {
		t.Fatalf("right from the crew stayed in its column")
	}
	key(ui.KeyLeft)
	if !h.tt.Focused("Iris Okafor") && !h.tt.Focused("View Full Crew") {
		t.Fatalf("left from the facts did not cross back to the crew")
	}
	key(ui.KeyUp)
	key(ui.KeyUp)
	focused("Watch")
	key(ui.KeyUp)
	focused("Science Fiction")
	key(ui.KeyEnter)
	last := h.core.discovers[len(h.core.discovers)-1]
	if !slices.Equal(last.GetGenres(), []string{"878"}) {
		t.Errorf("genres = %v, want = [878]", last.GetGenres())
	}
}

func TestTVDiscoveredSeasons(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	server := h.app.servers[0].GetId()
	h.app.router.Push("/title/" + server + "/series/201")
	for range 20 {
		if h.tt.HasText("Season 1") && h.tt.HasText("Mara Quinn") {
			break
		}
		h.settle()
	}
	for range 12 {
		if h.tt.Focused("Season 1") {
			break
		}
		key(ui.KeyDown)
	}
	if !h.tt.Focused("Season 1") {
		t.Fatalf("down never reached the last season; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-discover-seasons")
	// Opening it shows a spinner while its episodes load, then the episodes.
	gate := make(chan struct{})
	h.core.seasonGate = gate
	key(ui.KeyEnter)
	time.Sleep(500 * time.Millisecond)
	h.settle()
	h.save(shots, "tv-discover-season-loading")
	close(gate)
	for range 20 {
		if h.tt.HasText("Episode 3 · Aurora 3") || h.tt.HasText("Aurora 3") {
			break
		}
		h.settle()
	}
	h.save(shots, "tv-discover-season-open")
	key(ui.KeyEnter)
	h.core.seasonGate = nil
	// Down from the seasons lands on the cast's first, and up goes back.
	key(ui.KeyDown)
	if !h.tt.Focused("Mara Quinn") {
		t.Fatalf("down from the seasons did not land on the cast's first")
	}
	key(ui.KeyUp)
	if !h.tt.Focused("Season 1") {
		t.Fatalf("up from the cast did not return to the season")
	}
	// The cast's row ends with See All, opening the whole cast and crew.
	key(ui.KeyDown)
	for range 4 {
		if h.tt.Focused("See All") {
			break
		}
		key(ui.KeyRight)
	}
	key(ui.KeyEnter)
	h.settle()
	if loc := h.app.router.Location(); !strings.HasSuffix(loc, "/series/201/credits") {
		t.Errorf("See All opened %s, want = the title's credits", loc)
	}
}

// Discover's Requests lies aside above the first row: down from the tabs
// passes it, and up reaches it only from the row's cards past its first.
func TestTVDiscoverRequests(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	focused := func(label, after string) {
		t.Helper()
		if !h.tt.Focused(label) {
			t.Fatalf("after %s: %q has no focus; texts = %q", after, label, h.tt.Texts())
		}
	}
	h.app.router.Push("/discover")
	for range 20 {
		if h.tt.HasText("Starfall") {
			break
		}
		h.settle()
	}
	if _, ok := h.tt.Find("Requests"); !ok {
		t.Fatalf("no Requests; texts = %q", h.tt.Texts())
	}
	// Focus lands once the rows are laid out.
	for range 10 {
		if h.tt.Focused("The Long Orbit") {
			break
		}
		h.settle()
	}
	focused("The Long Orbit", "opening Discover")
	for range 4 {
		if h.tt.Focused("Discover") {
			break
		}
		key(ui.KeyUp)
	}
	focused("Discover", "up to the tabs")
	key(ui.KeyDown)
	focused("The Long Orbit", "down from the tabs")
	h.save(shots, "tv-discover")
	key(ui.KeyUp)
	focused("Discover", "up from the first card")
	key(ui.KeyDown)
	key(ui.KeyRight)
	key(ui.KeyUp)
	focused("Requests", "up from the second card")
	key(ui.KeyDown)
	if h.tt.Focused("Requests") || h.tt.Focused("Discover") {
		t.Fatalf("down from Requests stayed above the row")
	}
	key(ui.KeyUp)
	focused("Requests", "up from the second card again")
	key(ui.KeyUp)
	focused("Discover", "up from Requests")
	key(ui.KeyDown)
	key(ui.KeyRight)
	key(ui.KeyUp)
	key(ui.KeyEnter)
	h.settle()
	if got := h.app.router.Path(); !strings.HasPrefix(got, "/requests/") {
		t.Errorf("Requests opened %s", got)
	}
}

// A select on the television moves focus on with up and down while it is
// closed, and select opens its options on the chosen one, which up and
// down move along (docs/tv.md §5).
func TestTVSelect(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.app.router.Push("/settings/general")
	for range 40 {
		if h.tt.Focused("Server name") {
			break
		}
		h.settle()
	}
	if !h.tt.Focused("Server name") {
		f := &h.app.focus
		t.Fatalf("the settings did not open with focus on the server name; at=%d want=%d frames=%d items=%d first=%d initial=%d path=%s items=%+v", f.at, f.want, f.frames, len(f.items), f.first, f.initial, h.app.router.Path(), f.items)
	}
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	key(ui.KeyDown)
	if !h.tt.Focused("Metadata language") {
		t.Fatalf("down from the server name did not reach the select")
	}
	h.save(shots, "tv-select")
	key(ui.KeyDown)
	if !h.tt.Focused("Country") {
		t.Fatalf("down from the closed select did not move focus on; metadata=%v texts=%q", h.tt.Focused("Metadata language"), h.tt.Texts()[:16])
	}
	key(ui.KeyUp)
	if !h.tt.Focused("Metadata language") {
		t.Fatalf("up from the next select did not return")
	}
	key(ui.KeyEnter)
	if !h.tt.HasText("Japanese") {
		t.Fatalf("select opened no options; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-select-open")
	key(ui.KeyDown)
	key(ui.KeyEnter)
	if h.tt.HasText("Japanese") {
		t.Fatalf("choosing an option left the options open")
	}
	if !h.tt.Focused("Metadata language") {
		t.Errorf("choosing an option left focus off the select")
	}
	if h.tt.HasText("English (default)") {
		t.Errorf("the option below the chosen one was not chosen")
	}
}

// TestTVSearch checks the TV mode's search (docs/tv.md §6.4): focus starts
// on the keyboard, a keyboard types too, left and right cross between the
// keys and the results, and back deletes before it leaves.
func TestTVSearch(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	h.app.router.Push("/libraries")
	h.settle()
	h.app.router.Push("/search")
	for range 20 {
		if h.tt.Focused("A") {
			break
		}
		h.settle()
	}
	if !h.tt.Focused("A") {
		t.Fatalf("Search did not open with focus on the keyboard")
	}
	h.save(shots, "tv-search")
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	for _, k := range []ui.Key{ui.KeyO, ui.KeyR, ui.KeyB} {
		key(k)
	}
	if got := h.app.searchState.query; got != "orb" {
		t.Fatalf("typing: query = %q, want = orb", got)
	}
	for range 20 {
		if h.tt.HasText("From Discover") {
			break
		}
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	if !h.tt.HasText("The Long Orbit") || !h.tt.HasText("From Discover") {
		t.Fatalf("searching found nothing; texts = %q", h.tt.Texts())
	}
	h.save(shots, "tv-search-results")
	// From the keyboard's right edge into the results, and back.
	for range 5 {
		key(ui.KeyRight)
	}
	if !h.tt.Focused("F") {
		t.Fatalf("right did not reach the row's last key")
	}
	key(ui.KeyRight)
	if !h.tt.Focused("The Long Orbit") {
		t.Fatalf("right from the keyboard's edge did not reach the results")
	}
	key(ui.KeyLeft)
	if !h.tt.Focused("F") && !h.tt.Focused("L") {
		t.Fatalf("left from the results did not return to the keyboard")
	}
	// Back deletes, and leaves once there is nothing to delete.
	for range 3 {
		key(ui.KeyEscape)
	}
	if got := h.app.searchState.query; got != "" {
		t.Fatalf("back: query = %q, want = empty", got)
	}
	if got := h.app.router.Path(); got != "/search" {
		t.Fatalf("back deleting left Search for %q", got)
	}
	key(ui.KeyEscape)
	if got := h.app.router.Path(); got != "/libraries" {
		t.Fatalf("back from the empty search: path = %q, want = /libraries", got)
	}
}

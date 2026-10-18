package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

func TestUsers(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	user := func(name string) *userv1.User {
		t.Helper()
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		i := slices.IndexFunc(h.core.users, func(u *userv1.User) bool { return u.GetName() == name })
		if i < 0 {
			t.Fatalf("no user %q", name)
		}
		return h.core.users[i]
	}
	people := &h.app.settingsState.admin.people

	h.app.router.Push("/settings/users")
	h.settle()
	want("Anna", "Guest", "Kids", "Add User", "Invitation Links", "No invitation links.")

	// An invitation link, shown once and revoked.
	h.click("Invite")
	want("Uses", "Valid for days", "All libraries", "Allow downloads")
	h.click("Create Link")
	want("New link", "http://mavio.test/invite#tok-i1", "Copy Link", "Used 0 of 1")
	h.save(shots, "settings-users-invitations")
	h.click("Revoke")
	want("No invitation links.")

	// A managed user's page: its restriction, access and sign-ins.
	h.click("Kids")
	want("Restriction", "Younger child", "Access", "All libraries", "Highest rating", "Playback", "Requests", "Sign-Ins", "A PIN is set",
		"Make a member", "Invitation link")
	h.click("All libraries")
	want("Movies", "Shows", "Music")
	h.click("Shows")
	h.click("Request movies")
	want("Movie quota", "The server's")
	h.click("Own quota")
	h.save(shots, "settings-user")
	h.click("Save")
	want("Saved.")
	p := user("Kids").GetPolicy()
	if p.GetAllLibraries() || !slices.Equal(p.GetLibraryIds(), []string{"shows"}) || !p.GetRequestMovies() ||
		p.GetMovieQuota().GetLimit() != 10 || p.GetMovieQuota().GetDays() != 7 {
		t.Errorf("policy = %v, want = shows, movie requests of 10 every 7 days", p)
	}
	h.click("Clear")
	want("No PIN", "The PIN is cleared.")
	people.form.password = "secret"
	h.settle()
	h.click("Set")
	want("The user is now a member.")
	if h.core.passwords["u3"] != "secret" {
		t.Errorf("password = %q, want = secret", h.core.passwords["u3"])
	}

	// Deleting the user, once confirmed.
	h.click("Delete")
	want("Delete Kids?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if !h.tt.HasText("Add User") || h.tt.HasText("Kids") {
		t.Errorf("Kids was not deleted; texts = %q", h.tt.Texts())
	}

	// Adding a managed user.
	h.click("Add User")
	h.click("Kind")
	people.fresh.managed, people.fresh.name, people.fresh.pin = true, "Leo", "1234"
	h.settle()
	want("PIN", "Restriction", "Managed users have no password")
	h.save(shots, "settings-user-new")
	h.click("Add")
	want("Leo", "A PIN is set")
	if u := user("Leo"); !u.GetManaged() || !u.GetHasPin() {
		t.Errorf("Leo = %v, want = managed with a PIN", u)
	}
}

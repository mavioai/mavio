package app

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
)

func TestStorage(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}
	core := func() (remotes []*storagev1.Remote, offline map[string]*storagev1.OfflineDownloads, answers []string) {
		h.core.mu.Lock()
		defer h.core.mu.Unlock()
		return h.core.remotes, h.core.offline, h.core.answers
	}
	remote := func(name string) *storagev1.Remote {
		remotes, _, _ := core()
		i := slices.IndexFunc(remotes, func(r *storagev1.Remote) bool { return r.GetName() == name })
		if i < 0 {
			return nil
		}
		return remotes[i]
	}
	ss := &h.app.settingsState.admin.storage

	// The disks and the remotes.
	h.app.router.Push("/settings/storage")
	h.settle()
	want("Macintosh HD", "SSD · apfs", "900 GB of 1 TB used", "/Volumes/nas", "Network share · smbfs", "Did not answer",
		"drive", "Google Drive", "500 GB of 2 TB used")
	h.save(shots, "settings-storage")

	// A remote's offline downloads: set up, then into another folder.
	h.app.router.Push(storagePath("drive"))
	h.settle()
	h.click("Use for Offline Downloads")
	if _, offline, _ := core(); !offline["drive"].GetEnabled() || offline["drive"].GetPriority() != 1 {
		t.Errorf("offline = %v, want drive's on, of priority 1", offline)
	}
	want("Download Folder", "Remove Offline Downloads")
	ss.offline.draft.folder = "media/incoming"
	h.settle()
	h.save(shots, "settings-storage-offline")
	// The first Save is the remote's: the offline downloads' is left of
	// their Remove.
	r, _ := h.tt.Find("Remove Offline Downloads")
	h.tt.ClickAt(r.X-40, r.Y+r.H/2)
	h.settle()
	if _, offline, _ := core(); offline["drive"].GetFolder() != "media/incoming" {
		t.Errorf("offline = %v, want drive's into media/incoming", offline)
	}
	h.click("Remove Offline Downloads")
	h.click("Remove for Good")
	if _, offline, _ := core(); offline["drive"] != nil {
		t.Errorf("offline = %v, want drive's removed", offline)
	}
	h.app.router.Push("/settings/storage")
	h.settle()

	// A new remote of a required setting.
	h.click("Add Remote")
	want("New Remote", "Choose…")
	ss.form.name, ss.form.kind = "nas", "sftp"
	h.settle()
	want("SSH host to connect to.")
	ss.form.values["host"] = "nas.lan"
	h.settle()
	h.click("Add")
	if r := remote("nas"); r.GetParameters()["host"] != "nas.lan" {
		t.Errorf("nas = %v, want its host", r)
	}
	if got, want := h.app.router.Location(), "/settings/storage?remote=nas"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
	want("SSH/SFTP", "Folders")

	// One whose configuration asks a question.
	h.app.router.Push(storagePath("new"))
	h.settle()
	ss.form.name, ss.form.kind = "gd", "drive"
	h.settle()
	h.click("Add")
	want("Use web browser to automatically authenticate rclone with remote?", "Yes", "No")
	h.save(shots, "settings-storage-question")
	h.click("Yes")
	if _, _, answers := core(); !slices.Equal(answers, []string{"true"}) {
		t.Errorf("answers = %q, want = [true]", answers)
	}
	if got, want := h.app.router.Location(), "/settings/storage?remote=gd"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}

	// Its settings, saved, and the remote deleted.
	want("Google Drive", "scope", "Show advanced settings")
	ss.form.values["scope"] = "drive.readonly"
	h.settle()
	h.click("Save")
	want("Saved.")
	if r := remote("gd"); r.GetParameters()["scope"] != "drive.readonly" {
		t.Errorf("gd = %v, want scope drive.readonly", r)
	}
	h.click("Delete Remote")
	want("Delete gd?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if remote("gd") != nil {
		t.Errorf("gd was not deleted")
	}
	if got, want := h.app.router.Location(), "/settings/storage"; got != want {
		t.Errorf("path = %q, want = %q", got, want)
	}
}

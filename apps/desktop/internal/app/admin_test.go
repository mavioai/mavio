package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAdmin(t *testing.T) {
	h := newHarness(t, true, 1280, 1400)
	h.tt.Key(ui.Cmd, ui.KeyComma)
	h.settle()
	open := func(label string) {
		t.Helper()
		if err := h.tt.Click(label); err != nil {
			t.Fatalf("click %q: %v; texts = %q", label, err, h.tt.Texts())
		}
		h.settle()
	}
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	open("Overview")
	want("Running", "0.9.0", "linux/amd64", "SQLite", "7.1.2-Jellyfin", "1 day 2 hr",
		"Docker 27.3.1 · API 1.47", "unix:///var/run/docker.sock",
		"25", "15%", "10%", "5%", "15", "GB", "of 16 GB", "High", "1.2", "available of 2 TB", "2.5", "MB/s", "120 kB/s",
		"cache", "For TMDB · Ready")
	open("Start Command")
	want("docker run -d --name mavio-cache redis:7")
	h.save(shots, "settings-overview")

	open("Users")
	want("Anna", "Guest", "Kids", "Admin", "Managed", "Disabled", "Never signed in")
	h.save(shots, "settings-users")

	open("Libraries")
	want("Movies", "Movies · 2 folders", "TV Shows · 1 folder")
	open("Scan")
	if h.core.scans != 1 {
		t.Errorf("scans = %d, want = 1", h.core.scans)
	}
	want("Scan started")
	h.save(shots, "settings-libraries")

	open("Activity")
	want("Now Playing", "The Long Orbit", "Anna · Living Room TV · Mavio for Android TV", "Direct Play", "47:12 / 2:04:00",
		"Metadata could not be fetched", "40 minutes ago", "Show More")
	h.save(shots, "settings-activity")

	open("Tasks")
	want("Scan Media Libraries", "every 12 hr", "ffmpeg exited with status 1", "on demand", "Search for Missing Subtitles", "library.scan", "42%")
	open("Run")
	if h.core.taskRuns != 1 {
		t.Errorf("task runs = %d, want = 1", h.core.taskRuns)
	}
	// A running task tells what it is doing, a run that succeeded what it
	// did; a job's progress shows only while it runs.
	want("3 items added", "Scanning Movies", "12 items refreshed")
	for range 10 {
		if h.tt.HasText("Running · 25% · Extracting Orbit") {
			break
		}
		h.settle()
	}
	want("Running · 25% · Extracting Orbit")
	if h.tt.HasText("30%") {
		t.Errorf("a failed job shows its progress")
	}
	h.save(shots, "settings-tasks")

	open("Logs")
	want("scan finished", "probe slow", "4 records")
	h.app.settingsState.admin.logFilter = "tmdb"
	h.tt.Frame()
	want("fetch metadata", "1 record")
	if h.tt.HasText("scan finished") {
		t.Errorf("the filter kept %q", "scan finished")
	}
	h.app.settingsState.admin.logFilter = ""
	h.tt.Frame()
	h.save(shots, "settings-logs")

	open("Issues")
	// Open issues by default, each naming its item.
	want("Issues", "The Long Orbit · Subtitles", "1 issue")
	h.save(shots, "settings-issues")

	// Backups lie below the sidebar's fold.
	h.app.router.Push("/settings/backups")
	h.settle()
	want("mavio-20261019-120000.zip", "1 day ago · 3 MB · Mavio 0.9.0")
	open("Back Up")
	want("mavio-20261020-090000.zip")
	if err := h.tt.Click("Delete"); err != nil {
		t.Fatalf("click Delete: %v; texts = %q", err, h.tt.Texts())
	}
	h.settle()
	want("Delete mavio-20261020-090000.zip?")
	h.save(shots, "settings-backups")
	h.tt.Key(0, ui.KeyEscape)
	h.settle()

	// The catalog's plugins, TMDB installed with an update.
	open("Plugins")
	want("OpenSubtitles", "Subtitles from OpenSubtitles.com.", "2.0.0", "TMDB", "Metadata · 1.0.0", "Update available")
	h.save(shots, "settings-plugins")
	open("TMDB")
	want("By Mavio", "Update to 1.1.0", "Configuration", "API key", "Language", "Default (en)",
		"Use cache", "Set up the container dependency cache below to turn this on.", "cache", "Caches responses", "Not Set",
		"Information", "https://plugins.mavio.test/catalog.json", "GPL-3.0", "Versions", "1.1.0")
	h.save(shots, "settings-plugin")
	open("Update to 1.1.0")
	if got := h.core.plugins.installed[1].GetManifest().GetVersion(); got != "1.1.0" {
		t.Errorf("TMDB's version = %q, want = 1.1.0", got)
	}
	open("Uninstall")
	want("Uninstall TMDB?")
}

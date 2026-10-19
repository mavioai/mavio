package app

import (
	"slices"
	"testing"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func TestDetailsValueLinks(t *testing.T) {
	h := newHarness(t, true, 1280, 2600)
	server := h.core.s.Server.ID
	library := h.core.s.Items["orbit"].Library
	for _, tc := range []struct {
		click, path, pill string
	}{
		{"Northlight Pictures", "/library/" + server + "/" + library + "?studio=Northlight+Pictures", "Studio: Northlight Pictures"},
		{"space station", "/library/" + server + "/" + library + "?tag=space+station", "Keyword: space station"},
	} {
		h.app.router.Push("/item/" + server + "/orbit")
		h.settle()
		if err := h.tt.Click(tc.click); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if got := h.app.router.Location(); got != tc.path {
			t.Errorf("location = %q, want = %q", got, tc.path)
		}
		h.app.libraryStates[key(server, library)].tab = 1
		h.settle()
		if !h.tt.HasText(tc.pill) {
			t.Errorf("library has no %q; texts = %q", tc.pill, h.tt.Texts())
		}
		h.core.mu.Lock()
		var tag, studio string
		for _, r := range h.core.requests {
			if r.HasListLibraryItems() {
				tag, studio = r.GetListLibraryItems().GetFilter().GetTag(), r.GetListLibraryItems().GetFilter().GetStudio()
			}
		}
		h.core.mu.Unlock()
		if got := tag + studio; got != tc.click {
			t.Errorf("filter = %q, want = %q", got, tc.click)
		}
		if err := h.tt.Click(tc.pill); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if got, want := h.app.router.Location(), "/library/"+server+"/"+library; got != want {
			t.Errorf("location = %q, want = %q", got, want)
		}
	}
}

func TestDetailsAcquisition(t *testing.T) {
	h := newHarness(t, true, 1280, 3200)
	server := h.core.s.Server.ID
	w := h.core.wanted
	w.movies = append(w.movies, acquisitionv1.WantedMovie_builder{
		ItemId: new("orbit"), Name: new("Orbit"), Wanted: acquisitionv1.Wanted_builder{ItemId: new("orbit"), Monitored: new(true), QualityProfileId: new("hd")}.Build(),
	}.Build())
	d := h.core.downloads
	d.queue = append(d.queue, acquisitionv1.QueueItem_builder{
		Id: new("q-orbit"), ItemId: new("orbit"), ItemName: new("Orbit"), Title: new("Orbit.2024.2160p.WEB-DL"), Size: new(int64(8_000_000_000)),
		RemainingSize: new(int64(6_000_000_000)), State: new(acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING),
	}.Build())
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	for _, want := range []string{"Downloading 25%", "Acquisition", "Monitored", "Quality Profile", "HD-1080p", "Orbit.2024.2160p.WEB-DL"} {
		if !h.tt.HasText(want) {
			t.Errorf("details have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	if h.tt.HasText("Dune.2021.1080p.BluRay.x265-GRP") {
		t.Errorf("details show another item's download")
	}
	h.save(shots, "details-acquisition")

	if err := h.tt.Click("Automatic Search"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !slices.Contains(w.searched, "orbit") {
		t.Errorf("searched = %q, want = orbit", w.searched)
	}

	if err := h.tt.Click("Interactive Search"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"Releases of The Long Orbit", "Orbit.2024.1080p.BluRay.x265-GRP", "CAM is not wanted in profile", "x265 · Score 10", "2 failed: timed out"} {
		if !h.tt.HasText(want) {
			t.Errorf("releases have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-releases")
	if err := h.tt.Click("Grab"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := d.grabbed, []string{"orbit Orbit.2024.1080p.BluRay.x265-GRP"}; !slices.Equal(got, want) {
		t.Errorf("grabbed = %q, want = %q", got, want)
	}
	if !h.tt.HasText("Grabbed") {
		t.Errorf("the release grabbed shows no Grabbed")
	}
	if err := h.tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	h.settle()

	if err := h.tt.Click("Monitored"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if n := len(w.updated); n == 0 || w.updated[n-1].GetMonitored() || w.updated[n-1].GetQualityProfileId() != "hd" {
		t.Errorf("updated = %v, want = orbit unmonitored, its profile kept", w.updated)
	}
	if !h.tt.HasText("Not wanted: the server doesn't acquire it.") {
		t.Errorf("details do not tell orbit is not wanted; texts = %q", h.tt.Texts())
	}
}

func TestDetailsSubtitles(t *testing.T) {
	h := newHarness(t, true, 1280, 3600)
	server := h.core.s.Server.ID
	subs := h.core.subtitles
	calls := func() []string {
		subs.mu.Lock()
		defer subs.mu.Unlock()
		return slices.Clone(subs.calls)
	}
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	for _, want := range []string{"Subtitles", "Subtitle Profile", "French", "French (Forced)", "Present", "Orbit.en.srt · SRT", "Embedded #3 · SUBRIP · Not counted", "Extract"} {
		if !h.tt.HasText(want) {
			t.Errorf("details have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-subtitles")

	for _, step := range []struct {
		clicks []string
		want   string
	}{
		{[]string{"Extract"}, "extract 3"},
		{[]string{"Sync", "Sync, English"}, "sync /media/Orbit/Orbit.en.srt "},
		{[]string{"Delete", "Delete"}, "delete /media/Orbit/Orbit.en.srt"},
	} {
		for _, label := range step.clicks {
			if err := h.tt.Click(label); err != nil {
				t.Fatal(err)
			}
			h.settle()
		}
		if got := calls(); !slices.Contains(got, step.want) {
			t.Errorf("calls = %q, want %q", got, step.want)
		}
	}
	if h.tt.HasText("Orbit.en.srt · SRT") {
		t.Errorf("the subtitle deleted still shows")
	}

	// The sidebar's Search comes first.
	h.app.detailStates[key(server, "orbit")].subs.searching = true
	h.settle()
	for _, want := range []string{"Subtitles for The Long Orbit", "Orbit.2024.1080p.BluRay", "French (Forced) · 92% match · opensubtitles · ana · SRT", "Not matched: source", "podnapisi failed"} {
		if !h.tt.HasText(want) {
			t.Errorf("search has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-subtitle-search")
	if err := h.tt.Click("Download"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got, want := "download orbit opensubtitles/os-1 fr", calls(); !slices.Contains(want, got) {
		t.Errorf("calls = %q, want %q", want, got)
	}
	if !h.tt.HasText("Downloaded") {
		t.Errorf("the subtitle downloaded shows no Downloaded")
	}
}

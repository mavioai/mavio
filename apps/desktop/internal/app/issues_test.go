package app

import (
	"slices"
	"testing"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
)

func TestDetailsIssues(t *testing.T) {
	h := newHarness(t, true, 1280, 3800)
	server := h.core.s.Server.ID
	issues := h.core.issues
	// The subtitles' Delete would come before the issue's.
	h.core.subtitles.mu.Lock()
	h.core.subtitles.items = map[string][]*acquisitionv1.ItemSubtitle{}
	h.core.subtitles.mu.Unlock()
	h.app.router.Push("/item/" + server + "/orbit")
	h.settle()
	for _, want := range []string{"Open Issues", "The subtitles run late. · Kids · 2 hours ago"} {
		if !h.tt.HasText(want) {
			t.Errorf("details have no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-issues")

	if err := h.tt.Click("The subtitles run late. · Kids · 2 hours ago"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"Subtitles · The Long Orbit", "Reported by Kids", "Kids · 2 hours ago", "Resolve"} {
		if !h.tt.HasText(want) {
			t.Errorf("issue has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.app.issues.comment = "Fixed in the new release."
	h.settle()
	for _, label := range []string{"Comment", "Resolve"} {
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	for _, want := range []string{"Fixed in the new release.", "Resolved", "Reopen"} {
		if !h.tt.HasText(want) {
			t.Errorf("issue has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "details-issue")
	issues.mu.Lock()
	i := issues.issues[0]
	if got := i.GetStatus(); got != acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED {
		t.Errorf("status = %v, want = resolved", got)
	}
	if n := len(i.GetComments()); n != 2 {
		t.Errorf("comments = %d, want = 2", n)
	}
	issues.mu.Unlock()
	if err := h.tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.tt.HasText("Open Issues") {
		t.Errorf("the issue resolved still shows among the open ones")
	}

	if err := h.tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ChooseMenuItem("Report a Problem"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Report a Problem with The Long Orbit") {
		t.Fatalf("no report dialog; texts = %q", h.tt.Texts())
	}
	// The details' Audio comes first.
	h.app.issues.kind = 1
	h.app.issues.message = "No sound after an hour."
	h.settle()
	h.save(shots, "details-report")
	if err := h.tt.Click("Report"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Audio · The Long Orbit") || !h.tt.HasText("No sound after an hour.") {
		t.Errorf("the issue reported does not open; texts = %q", h.tt.Texts())
	}
	if err := h.tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	issues.mu.Lock()
	ids := make([]string, 0, len(issues.issues))
	for _, i := range issues.issues {
		ids = append(ids, i.GetId())
	}
	issues.mu.Unlock()
	if want := []string{"i1"}; !slices.Equal(ids, want) {
		t.Errorf("issues = %q, want = %q", ids, want)
	}
	if h.app.issues.open {
		t.Errorf("the issue deleted is still open")
	}
}

func TestTitleManage(t *testing.T) {
	h := newHarness(t, true, 1280, 1000)
	server := h.core.s.Server.ID
	issues := h.core.issues
	issues.mu.Lock()
	issues.issues[0].SetItemId("m1")
	issues.mu.Unlock()
	h.app.router.Push("/title/" + server + "/movie/101")
	h.settle()
	for _, label := range []string{"Report Issue", "Manage"} {
		if !h.tt.HasText(label) {
			t.Errorf("title has no %q; texts = %q", label, h.tt.Texts())
		}
	}
	if err := h.tt.Click("Manage"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"Manage The Long Orbit", "No active requests.", "Open Issues", "The subtitles run late.", "Open in Library"} {
		if !h.tt.HasText(want) {
			t.Errorf("manage has no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.save(shots, "title-manage")
	if err := h.tt.Click("The subtitles run late. · Kids · 2 hours ago"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Subtitles · The Long Orbit") {
		t.Errorf("the issue does not open; texts = %q", h.tt.Texts())
	}
}

package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSelectSearch checks that a select of many options searches them by
// what is typed, as the web UI's.
func TestSelectSearch(t *testing.T) {
	var labels []string
	for i := range selectSearchFrom + 4 {
		labels = append(labels, fmt.Sprintf("Option %02d", i+1))
	}
	chosen, changes := labels[0], 0
	tt := ui.NewTester(func(c *ui.Context) {
		if selectOf(c, &chosen, labels).Label("Options").Changed() {
			changes++
		}
	}, 600, 800)
	tt.Frame()
	if err := tt.Click("Option 01"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("17")
	tt.Frame()
	if !tt.HasText("Option 17") || tt.HasText("Option 03") {
		t.Fatalf("texts = %q, want = Option 17 alone", tt.Texts())
	}
	if err := tt.Click("Option 17"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if chosen != "Option 17" || changes != 1 {
		t.Errorf("chosen = %q after %d changes, want = Option 17 after 1", chosen, changes)
	}
	if err := tt.Click("Option 17"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Option 03") {
		t.Errorf("reopened, texts = %q, want = every option", tt.Texts())
	}
}

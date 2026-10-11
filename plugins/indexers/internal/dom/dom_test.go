package dom

import (
	"testing"

	"github.com/andybalholm/cascadia"
)

func TestDOM(t *testing.T) {
	doc, err := Parse(`<table><tr><td class="a" HREF="x">one <b>two</b></td><td>three</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	row := cascadia.MustCompile("tr").MatchFirst(doc)
	cells := Children(row)
	tests := []struct {
		name      string
		got, want any
	}{
		{"len(Children)", len(cells), 2},
		{"Text", Text(cells[0]), "one two"},
		{"Attr", Attr(cells[0], "href"), "x"},
		{"Attr missing", Attr(cells[1], "class"), ""},
		{"Attr nil", Attr(nil, "class"), ""},
		{"Text nil", Text(nil), ""},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %v, want = %v", tt.name, tt.got, tt.want)
		}
	}
}

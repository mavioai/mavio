package site

import (
	"slices"
	"testing"
)

func TestNodeAll(t *testing.T) {
	doc, err := Parse([]byte(`<p><a class="x" href="1">a</a><b class="y">b</b><a href="2">c</a></p>`), "text/html")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		selector string
		want     []string
	}{
		{"a", []string{"a", "c"}},
		{"a.x, b.y", []string{"a", "b"}},
		{"a[", nil},
	} {
		var got []string
		for _, n := range doc.All(c.selector) {
			got = append(got, n.Text())
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("All(%q) = %q, want = %q", c.selector, got, c.want)
		}
	}
}

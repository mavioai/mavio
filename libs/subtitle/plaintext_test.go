package subtitle

import "testing"

func TestPlainText(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"<i>Hello</i>\nthere", "Hello\nthere"},
		{`{\an8}{\i1}Top{\i0}\hline`, "Top line"},
		{`<font color="#ff0000">Red</font>`, "Red"},
		{"Plain", "Plain"},
	} {
		if got := PlainText(tc.in); got != tc.want {
			t.Errorf("PlainText(%q) = %q, want = %q", tc.in, got, tc.want)
		}
	}
}

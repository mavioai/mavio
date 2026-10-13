package python

import (
	"encoding/json"
	"testing"
)

func TestEval(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`"a\x41\u00e9\n"`, `"aAé\n"`},
		{`r"a\n"`, `"a\\n"`},
		{`"""x"""`, `"x"`},
		{`b"x"`, `{"$expr":"b\"x\""}`},
		{`f"{k}"`, `{"$expr":"f\"{k}\""}`},
		{`0x1F`, `31`},
		{`-0o17`, `-15`},
		{`1_000`, `1000`},
		{`.5`, `0.5`},
		{`1e3`, `1e3`},
		{`2j`, `{"$expr":"2j"}`},
		{`(1,)`, `[1]`},
		{`()`, `[]`},
		{`[None, True]`, `[null,true]`},
		{`{"a": 1, "b": (2, 3)}`, `{"a":1,"b":[2,3]}`},
		{`{1: 2}`, `{"$expr":"{1: 2}"}`},
		{`{1, 2}`, `[1,2]`},
		{`K`, `3`},
		{`S`, `{"$symbol":"S"}`},
		{`os.sep`, `{"$symbol":"os.sep"}`},
		{`f(1)`, `{"$expr":"f(1)"}`},
	}
	for _, tt := range tests {
		src := "K = 3\nS = 'a' + b\nX = " + tt.src + "\n"
		m, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %v", src, err)
		}
		lines, err := Lines(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(m.Eval(lines[2].Tokens[2:]))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("Eval(%s): got = %s, want = %s", tt.src, got, tt.want)
		}
	}
}

func TestLines(t *testing.T) {
	src := "a = [1,\n  2]  # c\n\nif x:\n\ty = \\\n 3\n"
	lines, err := Lines(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []struct{ Indent, Tokens, Line int }
	for _, l := range lines {
		got = append(got, struct{ Indent, Tokens, Line int }{l.Indent, len(l.Tokens), l.Tokens[0].Line})
	}
	want := []struct{ Indent, Tokens, Line int }{{0, 7, 1}, {0, 3, 4}, {8, 3, 5}}
	if len(got) != len(want) {
		t.Fatalf("got = %v, want = %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got = %v, want = %v", i, got[i], want[i])
		}
	}
}

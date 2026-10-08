package csharp

import (
	"encoding/json"
	"testing"
)

func TestLexLiterals(t *testing.T) {
	tests := []struct {
		src  string
		kind Kind
		want string
	}{
		{`"a\"b\\c\n\t\u00e9"`, String, "a\"b\\c\n\té"},
		{`@"C:\path\""x"""`, String, `C:\path\"x"`},
		{`'\''`, Char, "'"},
		{`'x'`, Char, "x"},
		{"\"\"\"\n    line1\n      line2\n    \"\"\"", String, "line1\n  line2"},
		{`"""inline "quoted" raw"""`, String, `inline "quoted" raw`},
		{`1_000UL`, Number, "1_000UL"},
		{`1.5e-3f`, Number, "1.5e-3f"},
		{`0x1F`, Number, "0x1F"},
	}
	for _, tt := range tests {
		toks, err := Lex(tt.src)
		if err != nil {
			t.Errorf("Lex(%q): %v", tt.src, err)
			continue
		}
		if len(toks) != 1 || toks[0].Kind != tt.kind || toks[0].Value != tt.want {
			t.Errorf("Lex(%q) = %+v, want one token kind %v value %q", tt.src, toks, tt.kind, tt.want)
		}
	}
}

func TestLexSkipsTrivia(t *testing.T) {
	src := "\ufeff// comment\n#if DEBUG\n/* block\n */ a /* x */ . b // tail\n"
	toks, err := Lex(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tok := range toks {
		got = append(got, tok.Text)
	}
	if want := []string{"a", ".", "b"}; !equal(got, want) {
		t.Errorf("tokens = %v, want %v", got, want)
	}
	if toks[0].Line != 4 {
		t.Errorf("line of a = %d, want 4", toks[0].Line)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const sample = `
using Xunit;

namespace Jellyfin.Naming.Tests.Video
{
    public class SampleTests
    {
        private readonly NamingOptions _namingOptions = new NamingOptions();

        public static TheoryData<VideoFileInfo> Data()
        {
            var data = new TheoryData<VideoFileInfo>();
            data.Add(new VideoFileInfo(path: "/a.mkv", container: "mkv", year: 2005));
            return data;
        }

        [Theory]
        [InlineData("Season 1/S01E02.avi", 2, null)]
        [InlineData(@"C:\x.avi", -1, ExtraType.Trailer, Skip = "flaky")]
        public void Parse(string path, int? expected, ExtraType? extra = null)
        {
            Assert.Equal(expected, Parse(path));
        }

        [Theory]
        [MemberData(nameof(Data))]
        public void Resolve(VideoFileInfo expected) => Assert.NotNull(expected);

        [Fact]
        public void Plain()
        {
        }

        public string Name { get; set; } = "x";

        private class Nested
        {
            [Fact]
            public void Inner() { }
        }
    }
}
`

func TestParse(t *testing.T) {
	f, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Classes) != 1 || f.Classes[0].Name != "SampleTests" {
		t.Fatalf("classes = %+v", f.Classes)
	}
	cls := f.Classes[0]
	if len(cls.Nested) != 1 || cls.Nested[0].Name != "Nested" || len(cls.Nested[0].Members) != 1 {
		t.Errorf("nested = %+v", cls.Nested)
	}

	parse := cls.Find("Parse")
	if parse == nil || !parse.IsMethod {
		t.Fatalf("Parse not found as a method: %+v", parse)
	}
	if got := len(parse.Attributes); got != 3 {
		t.Fatalf("Parse has %d attributes, want 3", got)
	}
	wantParams := []Param{{Name: "path", Type: "string"}, {Name: "expected", Type: "int?"}, {Name: "extra", Type: "ExtraType?"}}
	for i, w := range wantParams {
		if i >= len(parse.Params) || parse.Params[i].Name != w.Name || parse.Params[i].Type != w.Type {
			t.Errorf("param %d = %+v, want %+v", i, parse.Params, w)
		}
	}
	if parse.Params[2].Default == nil {
		t.Error("default value of extra not captured")
	}

	inline := parse.Attributes[2]
	if inline.Name != "InlineData" || len(inline.Args) != 4 || inline.Args[3].Name != "Skip" {
		t.Errorf("second InlineData = %+v", inline)
	}
	got, _ := json.Marshal([]Value{f.Eval(inline.Args[0].Expr), f.Eval(inline.Args[1].Expr), f.Eval(inline.Args[2].Expr)})
	if want := `["C:\\x.avi",-1,{"$symbol":"ExtraType.Trailer"}]`; string(got) != want {
		t.Errorf("args = %s, want %s", got, want)
	}

	if m := cls.Find("Name"); m == nil || m.IsMethod {
		t.Errorf("property Name = %+v", m)
	}
	if m := cls.Find("Resolve"); m == nil || len(m.Params) != 1 || len(m.Body) == 0 {
		t.Errorf("expression-bodied Resolve = %+v", m)
	}
}

func TestEval(t *testing.T) {
	tests := []struct{ expr, want string }{
		{`"a" + "b"`, `"ab"`},
		{`string.Empty`, `""`},
		{`(int)3`, `3`},
		{`1.5f`, `1.5`},
		{`0x10`, `16`},
		{`true`, `true`},
		{`null`, `null`},
		{`nameof(Foo.Bar)`, `"Bar"`},
		{`typeof(Foo)`, `{"$type":"Foo"}`},
		{`new[] { 1, 2 }`, `[1,2]`},
		{`new string[] { "a" }`, `["a"]`},
		{`new VideoFileInfo("/a", "mkv", year: 2005)`, `{"$new":"VideoFileInfo","args":["/a","mkv"],"named":{"year":2005}}`},
		{`new MediaStream { Type = MediaStreamType.Audio, Index = 1 }`, `{"$new":"MediaStream","init":{"Type":{"$symbol":"MediaStreamType.Audio"},"Index":1}}`},
		{`new Dictionary<string, int> { { "a", 1 } }`, `{"$new":"Dictionary<string, int>","items":[{"$expr":"{ \"a\", 1 }"}]}`},
		{`TranscodeReason.A | TranscodeReason.B`, `{"$flags":["TranscodeReason.A","TranscodeReason.B"]}`},
		{`[1, "a"]`, `[1,"a"]`},
		{`Array.Empty<byte>()`, `[]`},
		{`new long[0]`, `[]`},
		{`04`, `4`},
		{`007.50`, `7.50`},
		{`TimeSpan.FromSeconds(1)`, `{"$expr":"TimeSpan.FromSeconds(1)"}`},
		{`$"x{y}"`, `{"$expr":"$\"x{y}\""}`},
	}
	for _, tt := range tests {
		f := &File{Src: tt.expr}
		toks, err := Lex(tt.expr)
		if err != nil {
			t.Errorf("Lex(%q): %v", tt.expr, err)
			continue
		}
		got, err := marshal(f.Eval(toks))
		if err != nil {
			t.Errorf("marshal %q: %v", tt.expr, err)
			continue
		}
		if string(got) != tt.want {
			t.Errorf("Eval(%s) = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

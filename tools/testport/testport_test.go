package testport

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleTests = `
public class ParserTests
{
    public static TheoryData<string, int> AddData()
    {
        var data = new TheoryData<string, int>();
        data.Add("a", 1);
        data.Add("b", 2);
        return data;
    }

    public static IEnumerable<object[]> YieldData()
    {
        yield return new object[] { "c", 3 };
    }

    public static TheoryData<string, int> InitData => new TheoryData<string, int> { { "d", 4 } };

    public static TheoryData<int> LoopData()
    {
        var data = new TheoryData<int>();
        foreach (var x in Enum.GetValues<Foo>()) { data.Add((int)x); }
        return data;
    }

    [Theory]
    [InlineData("x", 1)]
    // TODO: [InlineData("broken", 9)]
    [InlineData("y", 2, Skip = "flaky")]
    public void Parse(string input, int expected, bool strict = true)
    {
    }

    [Theory]
    [MemberData(nameof(AddData))]
    [MemberData(nameof(YieldData))]
    [MemberData(nameof(InitData))]
    public void Members(string input, int expected)
    {
    }

    [Theory]
    [MemberData(nameof(LoopData))]
    public void Loop(int value)
    {
    }

    [Theory]
    [InlineData("v", 1, 2, 3)]
    public void Variadic(string name, params int[] values)
    {
    }

    [Fact(Skip = "slow")]
    public void Plain()
    {
    }
}
`

func TestExtract(t *testing.T) {
	classes, err := Extract(sampleTests)
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 {
		t.Fatalf("got %d classes, want 1", len(classes))
	}
	got, err := json.Marshal(classes[0])
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Theories []struct {
			Method string
			Cases  []struct {
				ID   string
				Args map[string]any
				Skip string
				From string
			}
		}
		Facts       []Fact
		Unsupported []Unsupported
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}

	byMethod := map[string]int{}
	for i, th := range doc.Theories {
		byMethod[th.Method] = i
	}
	parse := doc.Theories[byMethod["Parse"]]
	if len(parse.Cases) != 3 {
		t.Fatalf("Parse cases = %+v, want 3 (two inline, one commented out)", parse.Cases)
	}
	wantParse := []struct {
		id, input, skip string
	}{
		{"Parse/1", "x", ""},
		{"Parse/2", "broken", "commented out in Jellyfin (TODO)"},
		{"Parse/3", "y", "flaky"},
	}
	for i, w := range wantParse {
		c := parse.Cases[i]
		if c.ID != w.id || c.Args["input"] != w.input || c.Skip != w.skip || c.Args["strict"] != true {
			t.Errorf("Parse case %d = %+v, want id %s input %s skip %q strict true", i, c, w.id, w.input, w.skip)
		}
	}

	members := doc.Theories[byMethod["Members"]]
	var inputs []string
	for _, c := range members.Cases {
		inputs = append(inputs, c.Args["input"].(string)+":"+c.From)
	}
	if got, want := strings.Join(inputs, ","), "a:AddData,b:AddData,c:YieldData,d:InitData"; got != want {
		t.Errorf("Members cases = %s, want %s", got, want)
	}

	variadic := doc.Theories[byMethod["Variadic"]].Cases[0].Args["values"]
	if got, _ := json.Marshal(variadic); string(got) != "[1,2,3]" {
		t.Errorf("params values = %s, want [1,2,3]", got)
	}

	if len(doc.Facts) != 1 || doc.Facts[0].Method != "Plain" || doc.Facts[0].Skip != "slow" {
		t.Errorf("facts = %+v", doc.Facts)
	}
	if len(doc.Unsupported) != 1 || doc.Unsupported[0].Method != "Loop" {
		t.Errorf("unsupported = %+v, want Loop", doc.Unsupported)
	}
}

func TestOutputPath(t *testing.T) {
	cm := CaseMapping{Source: "tests/Jellyfin.Naming.Tests", Target: "libs/naming/testdata/cases"}
	tests := []struct{ rel, class, want string }{
		{"tests/Jellyfin.Naming.Tests/TV/EpisodeNumberTests.cs", "EpisodeNumberTests", "libs/naming/testdata/cases/tv/episode_number.json"},
		{"tests/Jellyfin.Naming.Tests/AudioBook/AudioBookResolverTests.cs", "AudioBookResolverTests", "libs/naming/testdata/cases/audio_book/audio_book_resolver.json"},
		{"tests/Jellyfin.Naming.Tests/Video/Format3DTests.cs", "Format3DTests", "libs/naming/testdata/cases/video/format_3d.json"},
		{"tests/Jellyfin.Naming.Tests/Video/StubTests.cs", "StubTests.Inner", "libs/naming/testdata/cases/video/stub_tests_inner.json"},
	}
	for _, tt := range tests {
		if got := outputPath(cm, tt.rel, tt.class); got != tt.want {
			t.Errorf("outputPath(%s, %s) = %s, want %s", tt.rel, tt.class, got, tt.want)
		}
	}
}

func TestPort(t *testing.T) {
	jellyfin, mavio := t.TempDir(), t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(jellyfin, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tests/Lib.Tests/Sub/ParserTests.cs", sampleTests)
	write("tests/Lib.Tests/Test Data/a.nfo", "<movie/>")
	write("tests/Lib.Tests/Test Data/Nested/b.nfo", "<tvshow/>")
	write("tests/Lib.Tests/Test Data/c.json", "{}")
	write("tests/Lib.Tests/OutputData.cs", `namespace Lib.Tests
{
    internal static class OutputData
    {
        public const string Banner = @"line ""1""
line 2";
        public const int Answer = 42;
        public static string NotConst => "x";

        public static class Inner
        {
            internal const bool Flag = true;
        }
    }
}`)

	m := Mapping{
		Repository: "https://example.com/jellyfin",
		Cases:      []CaseMapping{{Source: "tests/Lib.Tests", Target: "libs/x/testdata/cases"}},
		Assets:     []AssetMapping{{Source: "tests/Lib.Tests/Test Data", Target: "libs/x/testdata/nfo", Include: "*.nfo", Recursive: true}},
		Constants:  []ConstMapping{{Source: "tests/Lib.Tests/OutputData.cs", Target: "libs/x/testdata/outputs.json"}},
	}
	sum, err := Port(t.Context(), Options{Jellyfin: jellyfin, Mavio: mavio, Mapping: m})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Files != 1 || sum.Assets != 2 || sum.Cases != 8 || sum.Constants != 3 {
		t.Errorf("summary = %+v, want 1 file, 8 cases, 2 assets, 3 constants", sum)
	}

	data, err := os.ReadFile(filepath.Join(mavio, "libs/x/testdata/outputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var consts struct{ Constants map[string]any }
	if err := json.Unmarshal(data, &consts); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"OutputData.Banner": "line \"1\"\nline 2", "OutputData.Answer": 42.0, "OutputData.Inner.Flag": true}
	if !maps.Equal(consts.Constants, want) {
		t.Errorf("constants: got = %v, want = %v", consts.Constants, want)
	}

	data, err = os.ReadFile(filepath.Join(mavio, "libs/x/testdata/cases/sub/parser.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Source Source }
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Source.File != "tests/Lib.Tests/Sub/ParserTests.cs" || doc.Source.Repository != m.Repository {
		t.Errorf("source = %+v", doc.Source)
	}

	for _, f := range []string{"a.nfo", "Nested/b.nfo", "SOURCES.json"} {
		if _, err := os.Stat(filepath.Join(mavio, "libs/x/testdata/nfo", f)); err != nil {
			t.Errorf("asset %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(mavio, "libs/x/testdata/nfo/c.json")); err == nil {
		t.Error("c.json copied despite the include pattern")
	}
}

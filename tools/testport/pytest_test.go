package testport

import (
	"encoding/json"
	"testing"
)

const samplePytest = `import pytest
from pytest import mark

LIMIT = 0x10
NAMES = ("a", "b")

pytestmark = [pytest.mark.slow]


def _helper():
    pass


@pytest.mark.parametrize("value,expected", [
    ("x", 1),  # a comment
    ('y' "z", -2.5),
    pytest.param("w", None, marks=pytest.mark.xfail(reason="broken")),
])
def test_values(value, expected):
    assert value


@mark.parametrize("n", [LIMIT, {"k": [True, False]}])
@pytest.mark.parametrize("s", ["p", "q"])
def test_product(n, s):
    def test_nested():
        pass


@pytest.mark.parametrize("item", list_items())
def test_computed(item):
    pass


@pytest.mark.skipif(not ENABLED, reason="disabled")
def test_plain():
    """A docstring with (unbalanced brackets."""


class TestThing:
    @pytest.mark.skip("later")
    def test_method(self):
        pass

    def helper(self):
        pass


class Helper:
    def test_not_collected(self):
        pass
`

func TestExtractPytest(t *testing.T) {
	tc, err := ExtractPytest(samplePytest, "test_sample", "Bazarr")
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(tc)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"class":"test_sample","line":1,` +
		`"theories":[` +
		`{"method":"test_values","line":19,"params":[{"name":"value"},{"name":"expected"}],"cases":[` +
		`{"id":"test_values/1","line":15,"args":{"value":"x","expected":1}},` +
		`{"id":"test_values/2","line":16,"args":{"value":"yz","expected":-2.5}},` +
		`{"id":"test_values/3","line":17,"args":{"value":"w","expected":null},"skip":"expected to fail in Bazarr: broken"}]},` +
		`{"method":"test_product","line":25,"params":[{"name":"s"},{"name":"n"}],"cases":[` +
		`{"id":"test_product/1","line":23,"args":{"s":"p","n":16}},` +
		`{"id":"test_product/2","line":23,"args":{"s":"p","n":{"k":[true,false]}}},` +
		`{"id":"test_product/3","line":23,"args":{"s":"q","n":16}},` +
		`{"id":"test_product/4","line":23,"args":{"s":"q","n":{"k":[true,false]}}}]}],` +
		`"facts":[` +
		`{"method":"test_plain","line":36,"skip":"skipped in Bazarr when not ENABLED: disabled"},` +
		`{"method":"TestThing.test_method","line":42,"skip":"later"}],` +
		`"unsupported":[{"method":"test_computed","line":31,"reason":"argument values computed at run time: list_items()"}]}`
	if string(got) != want {
		t.Errorf("ExtractPytest:\ngot  = %s\nwant = %s", got, want)
	}
}

func TestExtractPytestModuleSkip(t *testing.T) {
	src := `import pytest
pytestmark = pytest.mark.skipif(True, reason="no network")

def test_a():
    pass
`
	tc, err := ExtractPytest(src, "test_m", "Bazarr")
	if err != nil {
		t.Fatal(err)
	}
	if len(tc.Facts) != 1 || tc.Facts[0].Skip != "skipped in Bazarr when True: no network" {
		t.Errorf("facts: got = %+v, want = one fact skipped when True", tc.Facts)
	}
}

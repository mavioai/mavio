package cardigann

import (
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSolidusEscapes(t *testing.T) {
	tests := []struct {
		name, yml string
		want      map[string]any
	}{
		{"double quoted", `a: "torrent-category-(\\d+)\/"`, map[string]any{"a": `torrent-category-(\d+)/`}},
		{"flow sequence", `a: ["N\/A", ""]`, map[string]any{"a": []any{"N/A", ""}}},
		{"escaped quote", `a: "a[onclick=\"post('\/login')\"]"`, map[string]any{"a": `a[onclick="post('/login')"]`}},
		{"other escapes", `a: "\t\\/\/"`, map[string]any{"a": "\t\\//"}},
		{"plain", `a: N\/A`, map[string]any{"a": `N\/A`}},
		{"single quoted", `a: 'N\/A "x" it''s'`, map[string]any{"a": `N\/A "x" it's`}},
		{"apostrophe in plain", "a: it's \"\\/\"\nb: \"\\/\"", map[string]any{"a": `it's "\/"`, "b": "/"}},
		{"comment", "a: \"\\/\" # \"\\/\nb: 1", map[string]any{"a": "/", "b": 1}},
		{"block", "a: |\n  \"N\\/A\"\n\n  x\nb: \"\\/\"", map[string]any{"a": "\"N\\/A\"\n\nx\n", "b": "/"}},
		{"folded item", "a:\n  - >-\n    \"\\/\"\n  - \"\\/\"", map[string]any{"a": []any{`"\/"`, "/"}}},
		{"quotes in plain", `a: td a[href*="/x-"] "\/"`, map[string]any{"a": `td a[href*="/x-"] "\/"`}},
		{"key", "a: {b: \"\\/\"}", map[string]any{"a": map[string]any{"b": "/"}}},
		{"multiline", "a: \"x\n  \\/\"", map[string]any{"a": "x /"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]any
			if err := yaml.Unmarshal(solidusEscapes([]byte(tt.yml)), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %#v, want = %#v", got, tt.want)
			}
		})
	}
}

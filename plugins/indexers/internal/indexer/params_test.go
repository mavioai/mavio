package indexer

import (
	"maps"
	"testing"
)

func TestParseCookieHeader(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   map[string]string
	}{
		{"good", "__cfduid=d6237f041586694295; __cf_bm=TlOng/xyqckk-TMen38z+0RFYA7YA=", map[string]string{"__cfduid": "d6237f041586694295", "__cf_bm": "TlOng/xyqckk-TMen38z+0RFYA7YA="}},
		{"duplicate keys", "__cfduid=d6237f041586694295; __cf_bm=TlOng/xyqckk-TMen38z+0RFYA7YA= __cf_bm=test", map[string]string{"__cfduid": "d6237f041586694295", "__cf_bm": "test"}},
		{"malformed", "__cfduidd6237f041586694295; __cf_;bm TlOng; good_cookie=value", map[string]string{"good_cookie": "value"}},
		{"empty", "", map[string]string{}},
		{"attributes", "a=1; Path=/; HttpOnly; max-age=3", map[string]string{"a": "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseCookieHeader(tt.header); !maps.Equal(got, tt.want) {
				t.Errorf("ParseCookieHeader(%q) = %v, want = %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestParams(t *testing.T) {
	var p Params
	p.Set("action", "browse")
	p.Set("searchstr", "a b&c")
	p.Add("cat", "1")
	p.Add("cat", "2")
	p.Set("action", "search")
	p.Set("filter_cat[1]", "1")
	if got, want := p.Encode(), "action=search&searchstr=a+b%26c&cat=1%2C2&filter_cat[1]=1"; got != want {
		t.Errorf("Encode() = %q, want = %q", got, want)
	}
	p.Del("searchstr")
	if got, want := p.Encode(), "action=search&cat=1%2C2&filter_cat[1]=1"; got != want {
		t.Errorf("Encode() after Del = %q, want = %q", got, want)
	}
}

func TestURLHelpers(t *testing.T) {
	tests := []struct {
		name, got, want string
	}{
		{"URLEncode", URLEncode("ü ~'!*()-_."), "%C3%BC+%7E%27!*()-_."},
		{"EscapeDataString", EscapeDataString("a b+c~"), "a%20b%2Bc~"},
		{"AddQueryParam", AddQueryParam("https://x/torrents.php", "id", "1"), "https://x/torrents.php?id=1"},
		{"AddQueryParam again", AddQueryParam("https://x/torrents.php?id=1", "torrentid", "2"), "https://x/torrents.php?id=1&torrentid=2"},
		{"CombinePath", CombinePath("https://x/", "/torrents.php"), "https://x/torrents.php"},
		{"RemoveQueryParam", RemoveQueryParam("https://x/t.php?action=download&id=1&usetoken=1", "usetoken"), "https://x/t.php?action=download&id=1"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want = %q", tt.name, tt.got, tt.want)
		}
	}
}

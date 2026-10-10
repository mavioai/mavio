package indexer

import (
	"net/http"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

func TestRSSCases(t *testing.T) {
	ported.Run(t, "basic_rss_parser.json", ported.Cases{Facts: map[string]string{
		"should_handle_relative_url": "TestRSSRelativeURL",
	}, Run: map[string]func(*testing.T, ported.Args){
		"should_not_parse_size": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "sizeString")
			if got := ParseSize(s, true); got != 0 {
				t.Errorf("ParseSize(%q) got = %d, want = 0", s, got)
			}
		},
		"should_parse_size": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "sizeString")
			want, _ := a.Int(t, "expectedSize")
			if got := ParseSize(s, true); got != want {
				t.Errorf("ParseSize(%q) got = %d, want = %d", s, got, want)
			}
		},
	}})
	ported.Run(t, "x_element_extensions.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"should_parse_date": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "dateString")
			got, err := ParseDate(s)
			if err != nil || got.Year() != 2013 || got.Location() != time.UTC {
				t.Errorf("ParseDate(%q) got = %v, %v, want = 2013 in UTC", s, got, err)
			}
		},
	}})
}

func TestRSSRelativeURL(t *testing.T) {
	const u = "http://my.indexer.com/api?q=My+Favourite+Show"
	r := &Response{Request: NewRequest(u, ""), URL: u, StatusCode: 200, Header: http.Header{}, Body: ported.Response(t, "relative_urls.xml")}
	rs, err := (&RSSParser{}).Parse(r)
	if err != nil || len(rs) != 1 {
		t.Fatalf("Parse() got = %d releases, %v, want = 1", len(rs), err)
	}
	if got, want := rs[0].CommentsURL, "http://my.indexer.com/details/123#comments"; got != want {
		t.Errorf("comments got = %q, want = %q", got, want)
	}
	if got, want := rs[0].DownloadURL, "http://my.indexer.com/getnzb/123.nzb&i=782&r=123"; got != want {
		t.Errorf("download got = %q, want = %q", got, want)
	}
}

func TestLoadXML(t *testing.T) {
	doc, err := LoadXML("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n<rss xmlns:t=\"urn:t\"><channel><item><title>A &amp; B&nbsp;&eacute;\x01</title><t:attr name=\"x\" value=\"1\"/></item></channel></rss>")
	if err != nil {
		t.Fatal(err)
	}
	items := Items(doc)
	if len(items) != 1 {
		t.Fatalf("items got = %d, want = 1", len(items))
	}
	if got, want := items[0].ChildValue("title", ""), "A & B é"; got != want {
		t.Errorf("title got = %q, want = %q", got, want)
	}
	if v, _ := items[0].ElementNS("urn:t", "attr").Attr("value"); v != "1" {
		t.Errorf("namespaced attribute got = %q, want = \"1\"", v)
	}
}

func TestMagnetInfoHash(t *testing.T) {
	for _, tt := range []struct{ link, want string }{
		{"magnet:?xt=urn:btih:96cd620beda3efd7c4d7746ef94549d03a2eb13b&dn=x", "96CD620BEDA3EFD7C4D7746EF94549D03A2EB13B"},
		{"magnet:?dn=x&xt=urn:btih:ED6E7P5IQJZCOSWGOH4FPTAVCRHJVKB6", "20FC4FBFA88272274AC671F857CC15144E9AA83E"},
		{"magnet:?dn=x", ""},
	} {
		if got := MagnetInfoHash(tt.link); got != tt.want {
			t.Errorf("MagnetInfoHash(%q) got = %q, want = %q", tt.link, got, tt.want)
		}
	}
}

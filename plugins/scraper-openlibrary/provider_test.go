package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

var responses = map[string]string{
	"/search.json?author=Tolkien&fields=key%2Ctitle%2Cauthor_name%2Cfirst_publish_year%2Ccover_i%2Cisbn%2Cpublisher%2Csubject&limit=1&title=The+Hobbit": `{"docs":[
		{"key":"/works/OL262758W","title":"The Hobbit","author_name":["J.R.R. Tolkien"],"first_publish_year":1937,"cover_i":14627509}]}`,
	"/works/OL262758W.json": `{"key":"/works/OL262758W","title":"The Hobbit","subtitle":"There and Back Again",
		"description":{"type":"/type/text","value":"A hobbit goes on an adventure.\r\n\r\n([source][1])\r\n\r\n[1]: https://example.org"},
		"subjects":["Fantasy","Dragons"],"first_publish_date":"September 21, 1937","covers":[14627509,-1],
		"authors":[{"author":{"key":"/authors/OL26320A"}}]}`,
	"/isbn/9780261103344.json": `{"title":"The Hobbit","works":[{"key":"/works/OL262758W"}]}`,
	"/authors/OL26320A.json":   `{"name":"J.R.R. Tolkien"}`,
}

func newTestPlugin(t *testing.T) *plugin {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}
		body, ok := responses[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &plugin{baseURL: srv.URL, coversURL: "https://covers.example"}
}

func TestBook(t *testing.T) {
	p := newTestPlugin(t)
	for _, l := range []*pluginv1.Lookup{
		pluginv1.Lookup_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_BOOK.Enum(), Name: proto.String("The Hobbit"), Artists: []string{"Tolkien"}}.Build(),
		pluginv1.Lookup_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_AUDIOBOOK.Enum(), ExternalIds: map[string]string{keyISBN: "978-0-261-10334-4"}}.Build(),
	} {
		resp, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: l}.Build())
		if err != nil || !resp.GetFound() {
			t.Fatalf("GetMetadata(%v) = %v, %v", l, resp, err)
		}
		md := resp.GetMetadata()
		if md.GetName() != "The Hobbit: There and Back Again" || md.GetOverview() != "A hobbit goes on an adventure." ||
			md.GetPremiereDate() != "1937-09-21" || md.GetProductionYear() != 1937 || !slices.Equal(md.GetGenres(), []string{"Fantasy", "Dragons"}) {
			t.Errorf("book: %v", md)
		}
		if len(md.GetPeople()) != 1 || md.GetPeople()[0].GetName() != "J.R.R. Tolkien" || md.GetPeople()[0].GetKind() != pluginv1.CreditKind_CREDIT_KIND_AUTHOR {
			t.Errorf("people = %v", md.GetPeople())
		}
		if len(md.GetImages()) != 1 || md.GetImages()[0].GetUrl() != "https://covers.example/b/id/14627509-L.jpg" {
			t.Errorf("images = %v", md.GetImages())
		}
		if md.GetExternalIds()[keyWork] != "OL262758W" {
			t.Errorf("IDs = %v", md.GetExternalIds())
		}
	}
	resp, err := p.Search(t.Context(), pluginv1.SearchRequest_builder{Lookup: pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_BOOK.Enum(), Name: proto.String("The Hobbit"), Artists: []string{"Tolkien"},
	}.Build(), Limit: proto.Int32(1)}.Build())
	if err != nil || len(resp.GetResults()) != 1 || resp.GetResults()[0].GetName() != "J.R.R. Tolkien – The Hobbit" || resp.GetResults()[0].GetYear() != 1937 {
		t.Errorf("Search = %v, %v", resp, err)
	}
	// Movies are not books.
	resp2, err := p.GetMetadata(t.Context(), pluginv1.GetMetadataRequest_builder{Lookup: pluginv1.Lookup_builder{
		Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), Name: proto.String("The Hobbit"),
	}.Build()}.Build())
	if err != nil || resp2.GetFound() {
		t.Errorf("movie = %v, %v", resp2, err)
	}
}

func TestHelpers(t *testing.T) {
	for in, want := range map[string]string{"0-261-10334-X": "026110334X", "978 0 261 10334 4": "9780261103344", "12": ""} {
		if got := normalizeISBN(in); got != want {
			t.Errorf("normalizeISBN(%q) = %q, want = %q", in, got, want)
		}
	}
	for in, want := range map[string]int32{"1954": 1954, "July 29, 1954": 1954, "": 0, "n.d.": 0} {
		if got := year(in); got != want {
			t.Errorf("year(%q) = %d, want = %d", in, got, want)
		}
	}
	if got := cleanDescription("Text.\n----------\nContains: x"); got != "Text." {
		t.Errorf("cleanDescription = %q", got)
	}
}

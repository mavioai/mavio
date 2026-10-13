package ported

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCassette(t *testing.T) {
	c := &http.Client{Transport: Cassette(t, "test_napiprojekt/test_list_subtitles_movie.yaml")}
	// The query in another order.
	resp, err := c.Get("https://napiprojekt.pl/unit_napisy/dl.php?l=PL&f=444563eef63f83d47cabb888f7a45113&t=a6f09&v=dreambox&kolejka=false&nick=&pass=&napios=Linux")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || len(data) == 0 || strings.HasPrefix(string(data), "\x1f\x8b") {
		t.Errorf("got = %d, %d bytes, want = 200 and the body decompressed", resp.StatusCode, len(data))
	}
}

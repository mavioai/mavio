package app

import (
	"slices"
	"testing"
)

func TestDiscoveredPagesAndFilters(t *testing.T) {
	h := newHarness(t, true, 1280, 900)
	h.settle()
	server := h.app.servers[0].GetId()
	click := h.click
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	h.app.router.Push("/title/" + server + "/movie/101")
	h.settle()
	want("Director", "Iris Okafor", "Writers", "Jonah Reyes", "View Full Crew", "space", "homecoming",
		"The Orbit Collection", "View", "91%", "7.8", "76%", "Released", "Original Language", "English",
		"Meridian Pictures", "$90,000,000", "Open in Library", "Letterboxd")
	h.save(shots, "discover-title-facts")

	click("View Full Crew")
	want("Full Crew", "Iris Okafor", "Jonah Reyes")
	h.app.router.Back()
	h.settle()

	click("Mara Quinn")
	if got, want := h.app.router.Path(), "/discover/person/"+server+"/301"; got != want {
		t.Fatalf("path = %q, want = %q", got, want)
	}
	want("Mara Quinn", "Wellington, New Zealand", "Also known as M. Quinn", "Appearances", "Commander Vale", "Crew", "Producer", "Starfall")
	h.save(shots, "discover-person")
	click("TV Shows")
	if h.tt.HasText("The Long Orbit") || !h.tt.HasText("Northern Lights") {
		t.Errorf("series alone: texts = %q", h.tt.Texts())
	}

	h.app.router.Push("/title/" + server + "/movie/101")
	h.settle()
	click("The Orbit Collection")
	want("The Orbit Collection", "2 movies", "Two journeys to the edge and back.", "Request Collection", "Blocklist", "Movies", "Starfall")
	h.save(shots, "discover-collection")
	click("Request Collection")
	want("Request The Orbit Collection", "All Movies", "Request 1 Movie")
	click("Request 1 Movie")
	if n := len(h.core.titleRequests); n != 1 || h.core.titleRequests[0].GetTmdbId() != 102 {
		t.Fatalf("requests = %v, want = Starfall's", h.core.titleRequests)
	}
	click("Blocklist")
	if !slices.Equal(h.core.blocked, []int32{500}) {
		t.Errorf("blocked = %v, want = [500]", h.core.blocked)
	}
	want("Remove from Blocklist")

	h.app.router.Push("/title/" + server + "/movie/101")
	h.settle()
	click("space")
	want("Popular Movies", "Filters (1)")
	last := h.core.discovers[len(h.core.discovers)-1]
	if !slices.Equal(last.GetKeywords(), []string{"k1"}) {
		t.Errorf("keywords = %v, want = [k1]", last.GetKeywords())
	}
	click("Filters (1)")
	want("Sort By", "Genres", "Keywords", "Studios", "Original Language", "Released", "Minimum Rating", "Show Titles")
	h.save(shots, "discover-filters")
	click("Drama")
	click("Show Titles")
	want("Filters (2)")
	last = h.core.discovers[len(h.core.discovers)-1]
	if !slices.Equal(last.GetGenres(), []string{"18"}) || !slices.Equal(last.GetKeywords(), []string{"k1"}) {
		t.Errorf("genres = %v, keywords = %v, want = [18], [k1]", last.GetGenres(), last.GetKeywords())
	}
}

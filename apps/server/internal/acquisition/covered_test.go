package acquisition

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/acquire/series"
)

// TestCoveredTitles checks the name of an item holding a file of several
// episodes joins their titles as Jellyfin's EpisodeNfoParser, and splits
// back into one title per episode, so that renaming the file again names
// every episode.
func TestCoveredTitles(t *testing.T) {
	eps := func(titles ...string) []*series.Episode {
		var out []*series.Episode
		for _, title := range titles {
			out = append(out, &series.Episode{Title: title})
		}
		return out
	}
	names := []struct {
		name     string
		episodes []*series.Episode
		want     string
	}{
		{"two titles", eps("Pilot", "Cat's in the Bag"), "Pilot / Cat's in the Bag"},
		{"three titles", eps("A", "B", "C"), "A / B / C"},
		{"untitled episode", eps("Pilot", ""), ""},
		{"title with the separator", eps("Pilot", "This / That"), ""},
	}
	for _, tt := range names {
		if got := coveredName(tt.episodes); got != tt.want {
			t.Errorf("%s: coveredName = %q, want = %q", tt.name, got, tt.want)
		}
	}

	titles := []struct {
		name        string
		item        string
		first, last int
		want        []string
	}{
		{"split", "Pilot / Cat's in the Bag", 1, 2, []string{"Pilot", "Cat's in the Bag"}},
		{"one episode", "This / That", 3, 3, []string{"This / That"}},
		{"not one title per episode", "Pilot", 1, 2, []string{"Pilot"}},
		{"more titles than episodes", "A / B / C", 1, 2, []string{"A / B / C"}},
	}
	for _, tt := range titles {
		if got := coveredTitles(tt.item, tt.first, tt.last); !slices.Equal(got, tt.want) {
			t.Errorf("%s: coveredTitles = %q, want = %q", tt.name, got, tt.want)
		}
	}
}

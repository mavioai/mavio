package indexer

import (
	"slices"
	"testing"

	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

func TestCategoriesCases(t *testing.T) {
	ported.Run(t, "indexer_capabilities_categories.json", ported.Cases{Facts: map[string]string{
		"should_support_parent_if_child_mapping":    "TestCategories",
		"should_support_category_if_mapped":         "TestCategories",
		"should_not_support_category_if_not_mapped": "TestCategories",
		"should_get_tracker_category_list":          "TestCategories",
		"should_get_category_by_tracker_id":         "TestCategories",
		"should_get_category_by_tracker_desc":       "TestCategories",
	}})
}

func TestCategories(t *testing.T) {
	var one, two Categories
	one.Add("1", MoviesSD, "Filme SD")
	two.Add("1", MoviesSD, "Filme SD")
	two.Add("2", MoviesHD, "Filme HD")

	for _, tt := range []struct {
		query []int
		want  int
	}{{[]int{2000}, 1}, {[]int{2030}, 1}, {[]int{2040}, 0}} {
		if got := one.Supported(tt.query); len(got) != tt.want {
			t.Errorf("Supported(%v) got = %v, want = %d categories", tt.query, got, tt.want)
		}
	}
	if got := two.TrackerCategories(); !slices.Equal(got, []string{"1", "2"}) {
		t.Errorf("TrackerCategories() got = %v, want = [1 2]", got)
	}
	for name, got := range map[string][]*Category{"MapTracker": two.MapTracker("2"), "MapTrackerDesc": two.MapTrackerDesc("filme hd")} {
		if len(got) != 2 || got[0].ID != MoviesHD.ID || got[1].ID != 100002 {
			t.Errorf("%s got = %v, want = [2040 100002]", name, got)
		}
	}
}

func TestCategoriesTree(t *testing.T) {
	var cs Categories
	cs.Add("b", TVHD, "Series HD")
	cs.Add("7", Movies, "")
	cs.Add("8", &Category{ID: 2999, Name: "Movies/Custom"}, "")
	cs.Add("a", MoviesSD, "")
	tree := cs.Tree()
	var ids []int
	for _, c := range tree {
		ids = append(ids, c.ID)
	}
	// "b" has no number: its SHA-1 starts e9 d7, so its custom category is
	// 100000 + 0xd7e9.
	want := []int{2000, 5000, 155_273}
	if !slices.Equal(ids, want) {
		t.Fatalf("Tree() got = %v, want = %v", ids, want)
	}
	if got := tree[0].Sub; len(got) != 2 || got[0].ID != 2030 || got[1].ID != 2999 {
		t.Errorf("Tree()[0].Sub got = %v, want = [2030 2999]", got)
	}
	if got := cs.MapToTracker([]int{2000}, false); !slices.Equal(got, []string{"7", "8", "a"}) {
		t.Errorf("MapToTracker(2000) got = %v, want = [7 8 a]", got)
	}
	if got := cs.MapToTracker([]int{5040}, true); !slices.Equal(got, []string{"b"}) {
		t.Errorf("MapToTracker(5040, childToParent) got = %v, want = [b]", got)
	}
}

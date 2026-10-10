package indexer

import (
	"cmp"
	"crypto/sha1"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
)

// customCategoryBase is added to a tracker's own category ID to make the
// ID of the custom category standing for it.
const customCategoryBase = 100000

// Category is a Newznab category: a standard one, such as 2000 for movies
// with its subcategories, or a custom one standing for a tracker's own
// category, from 100000 up.
type Category struct {
	ID   int
	Name string
	Sub  []*Category
}

// contains reports whether c is the category with the ID or has it as a
// subcategory.
func (c *Category) contains(id int) bool {
	return c.ID == id || slices.ContainsFunc(c.Sub, func(s *Category) bool { return s.ID == id })
}

func (c *Category) withoutSub() *Category {
	return &Category{ID: c.ID, Name: c.Name}
}

// StandardCategory returns the standard category with the ID, or a
// nameless one when there is none.
func StandardCategory(id int) *Category {
	if i := slices.IndexFunc(standardCategories, func(c *Category) bool { return c.ID == id }); i >= 0 {
		return standardCategories[i]
	}
	return &Category{ID: id}
}

// categoryMapping maps a tracker's category to a Newznab category.
type categoryMapping struct {
	tracker, trackerDesc string
	newznab              int
}

// Categories maps an indexer's own categories to Newznab categories and
// back, and keeps the tree of Newznab categories the indexer has, as
// Prowlarr's IndexerCapabilitiesCategories does.
type Categories struct {
	mappings []categoryMapping
	tree     []*Category
}

// Add maps the tracker's category, given by ID, to a Newznab category.
// With a description, the tracker's category also gets a custom category
// of its own: 100000 plus its ID, or plus a hash of it when it is not a
// number.
func (cs *Categories) Add(tracker string, c *Category, desc string) {
	cs.mappings = append(cs.mappings, categoryMapping{tracker, desc, c.ID})
	cs.addToTree(c)
	if desc == "" {
		return
	}
	id, err := strconv.Atoi(tracker)
	if err != nil {
		sum := sha1.Sum([]byte(tracker))
		id = int(binary.LittleEndian.Uint16(sum[:2]))
	}
	custom := &Category{ID: id + customCategoryBase, Name: desc}
	cs.mappings = append(cs.mappings, categoryMapping{tracker, desc, custom.ID})
	cs.addToTree(custom)
}

// TrackerCategories lists the tracker's categories mapped to standard
// categories.
func (cs *Categories) TrackerCategories() []string {
	var out []string
	for _, m := range cs.mappings {
		if m.newznab < customCategoryBase && !slices.Contains(out, m.tracker) {
			out = append(out, m.tracker)
		}
	}
	return out
}

// Tree returns the indexer's Newznab categories with their
// subcategories: standard ones by ID, then custom ones by name.
func (cs *Categories) Tree() []*Category {
	tree := make([]*Category, 0, len(cs.tree))
	for _, c := range cs.tree {
		n := c.withoutSub()
		n.Sub = slices.SortedStableFunc(slices.Values(c.Sub), func(a, b *Category) int { return cmp.Compare(a.ID, b.ID) })
		tree = append(tree, n)
	}
	key := func(c *Category) string {
		if c.ID >= customCategoryBase {
			return "zzz" + c.Name
		}
		return strconv.Itoa(c.ID)
	}
	slices.SortStableFunc(tree, func(a, b *Category) int { return strings.Compare(key(a), key(b)) })
	return tree
}

// List returns the indexer's Newznab categories as a flat list, each
// followed by its subcategories, in the order of Tree.
func (cs *Categories) List() []*Category {
	var out []*Category
	for _, c := range cs.Tree() {
		out = append(out, c.withoutSub())
		out = append(out, c.Sub...)
	}
	return out
}

// MapToTracker returns the tracker's categories for Newznab categories:
// those mapped to them or, for parent categories, to their subcategories;
// with childToParent, also those mapped to the parents of subcategories.
func (cs *Categories) MapToTracker(categories []int, childToParent bool) []string {
	expanded := cs.Expand(categories, childToParent)
	var out []string
	for _, m := range cs.mappings {
		if slices.Contains(expanded, m.newznab) && !slices.Contains(out, m.tracker) {
			out = append(out, m.tracker)
		}
	}
	return out
}

// MapTracker returns the Newznab categories of a tracker's category,
// given by ID, ignoring case.
func (cs *Categories) MapTracker(tracker string) []*Category {
	return cs.mapBy(tracker, func(m categoryMapping) string { return m.tracker })
}

// MapTrackerDesc returns the Newznab categories of a tracker's category,
// given by description, ignoring case.
func (cs *Categories) MapTrackerDesc(desc string) []*Category {
	return cs.mapBy(desc, func(m categoryMapping) string { return m.trackerDesc })
}

func (cs *Categories) mapBy(v string, key func(categoryMapping) string) []*Category {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var out []*Category
	for _, m := range cs.mappings {
		if k := key(m); strings.TrimSpace(k) != "" && strings.EqualFold(k, v) {
			out = append(out, StandardCategory(m.newznab))
		}
	}
	return out
}

// Supported returns those of the Newznab categories the indexer has.
func (cs *Categories) Supported(categories []int) []int {
	var out []int
	for _, c := range cs.tree {
		if slices.Contains(categories, c.ID) {
			out = append(out, c.ID)
		}
	}
	for _, c := range cs.tree {
		for _, s := range c.Sub {
			if slices.Contains(categories, s.ID) {
				out = append(out, s.ID)
			}
		}
	}
	return out
}

// Merge adds the standard categories of another indexer, for searching
// several as one.
func (cs *Categories) Merge(other *Categories) {
	for _, c := range other.List() {
		if c.ID < customCategoryBase {
			cs.addToTree(c)
		}
	}
}

// Expand adds to Newznab categories the subcategories of the parents
// among them, and, with childToParent, the parents of the subcategories.
func (cs *Categories) Expand(categories []int, childToParent bool) []int {
	var out []int
	add := func(id int) {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	for _, id := range categories {
		add(id)
		if id >= customCategoryBase {
			continue
		}
		if i := slices.IndexFunc(cs.tree, func(c *Category) bool { return c.ID == id }); i >= 0 {
			for _, s := range cs.tree[i].Sub {
				add(s.ID)
			}
		} else if childToParent {
			if i := slices.IndexFunc(cs.tree, func(c *Category) bool { return c.contains(id) }); i >= 0 {
				add(cs.tree[i].ID)
			}
		}
	}
	return out
}

// addToTree adds a category to the tree, under its standard parent when
// it has one.
func (cs *Categories) addToTree(c *Category) {
	node := func(id int) *Category {
		if i := slices.IndexFunc(cs.tree, func(n *Category) bool { return n.ID == id }); i >= 0 {
			return cs.tree[i]
		}
		return nil
	}
	addUnder := func(parent *Category) {
		n := node(parent.ID)
		if n == nil {
			n = parent.withoutSub()
			cs.tree = append(cs.tree, n)
		}
		if !n.contains(c.ID) {
			n.Sub = append(n.Sub, c)
		}
	}
	if slices.ContainsFunc(parentCategories, func(p *Category) bool { return p.ID == c.ID }) {
		if node(c.ID) == nil {
			cs.tree = append(cs.tree, c.withoutSub())
		}
		return
	}
	if i := slices.IndexFunc(parentCategories, func(p *Category) bool { return p.contains(c.ID) }); i >= 0 {
		addUnder(parentCategories[i])
		return
	}
	// A custom category numbered like a standard one goes under its
	// parent.
	if c.ID > 1000 && c.ID < 10000 {
		if i := slices.IndexFunc(parentCategories, func(p *Category) bool { return p.ID/1000 == c.ID/1000 }); i >= 0 {
			addUnder(parentCategories[i])
			return
		}
	}
	cs.tree = append(cs.tree, c)
}

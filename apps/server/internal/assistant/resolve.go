package assistant

import (
	"slices"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
	"golang.org/x/text/unicode/norm"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

// titleKinds are the kinds a spoken title names.
var titleKinds = []libraryv1.ItemKind{
	libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_VIDEO,
	libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST, libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM, libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO,
	libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK, libraryv1.ItemKind_ITEM_KIND_COLLECTION, libraryv1.ItemKind_ITEM_KIND_PLAYLIST,
}

// normalize folds a name for comparison: compatibility forms, case,
// punctuation and spaces.
func normalize(s string) string {
	s = norm.NFKC.String(s)
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(unicode.ToLower(r))
		default:
			space = true
		}
	}
	return b.String()
}

// pinyinKey spells a name's Han characters in pinyin without tones and
// keeps its other letters and digits, so that homophones a recognizer
// chose compare equal.
func pinyinKey(s string) string {
	a := pinyin.NewArgs()
	var b strings.Builder
	for _, r := range norm.NFKC.String(s) {
		switch {
		case unicode.Is(unicode.Han, r):
			if p := pinyin.SinglePinyin(r, a); len(p) > 0 {
				b.WriteString(p[0])
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func hasHan(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}

// matches reports whether an item is named text exactly.
func matches(it *libraryv1.Item, want string) bool {
	return normalize(it.GetName()) == want || normalize(it.GetOriginalTitle()) == want || normalize(it.GetSortName()) == want
}

// resolveTitle finds the items a spoken title names among kinds: those
// named exactly, else what a search finds, else for Chinese the titles
// that sound the same. Several are returned for the user to choose.
func (c *call) resolveTitle(text string, ks []libraryv1.ItemKind) ([]*libraryv1.Item, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	resp, err := c.s.cfg.Services.Items.ListItems(c.ctx, libraryv1.ListItemsRequest_builder{
		Search: &text, Recursive: new(true), Kinds: ks, Limit: new(int32(20)),
	}.Build())
	if err != nil {
		return nil, err
	}
	found := resp.GetItems()
	want := normalize(text)
	if exact := slices.DeleteFunc(slices.Clone(found), func(it *libraryv1.Item) bool { return !matches(it, want) }); len(exact) > 0 {
		return exact, nil
	}
	if len(found) > 0 || !hasHan(text) {
		return found, nil
	}
	return c.soundsLike(text, ks)
}

// soundsLike finds the titles whose pinyin is the text's, else contains
// it.
func (c *call) soundsLike(text string, ks []libraryv1.ItemKind) ([]*libraryv1.Item, error) {
	resp, err := c.s.cfg.Services.Items.ListItems(c.ctx, libraryv1.ListItemsRequest_builder{
		Recursive: new(true), Kinds: ks, Limit: new(int32(1000)),
	}.Build())
	if err != nil {
		return nil, err
	}
	key := pinyinKey(text)
	if key == "" {
		return nil, nil
	}
	var exact, partial []*libraryv1.Item
	for _, it := range resp.GetItems() {
		for _, name := range []string{it.GetName(), it.GetOriginalTitle()} {
			if !hasHan(name) {
				continue
			}
			k := pinyinKey(name)
			if k == key {
				exact = append(exact, it)
				break
			}
			if strings.Contains(k, key) {
				partial = append(partial, it)
				break
			}
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}
	return partial[:min(len(partial), 10)], nil
}

// episode finds a series' episode by its numbers; episode 0 is the
// season's first unwatched, else its first.
func (c *call) episode(series *libraryv1.Item, season, episode int32) (*libraryv1.Item, error) {
	eps, err := c.children(series.GetId(), []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE}, 1000)
	if err != nil {
		return nil, err
	}
	var first *libraryv1.Item
	for _, e := range eps {
		if season > 0 && e.GetParentIndexNumber() != season {
			continue
		}
		if episode > 0 {
			if e.GetIndexNumber() == episode || (e.GetIndexNumber() < episode && e.GetIndexNumberEnd() >= episode) {
				return e, nil
			}
			continue
		}
		if !e.GetUserData().GetPlayed() {
			return e, nil
		}
		if first == nil {
			first = e
		}
	}
	return first, nil
}

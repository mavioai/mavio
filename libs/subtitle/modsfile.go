package subtitle

import (
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// Helpers of the whole-file mods: pysubs2's tag handling, emoji removal
// as the emoji package does it and the public suffix lookup of the tld
// package, which the common mod uses to leave domain names alone.

var (
	srtStyleTags   = regexp.MustCompile(`< *(/)? *([isub]) *>`)
	htmlTags       = regexp.MustCompile(`< */? *[a-zA-Z][^>]*>`)
	overrideBlocks = regexp.MustCompile(`\{[^}]*\}`)
)

// plaintext is pysubs2's SSAEvent.plaintext, on text with \n line breaks:
// override blocks removed and hard spaces made spaces.
func plaintext(text string) string {
	text = overrideBlocks.ReplaceAllString(text, "")
	return strings.NewReplacer(`\h`, " ", `\n`, "\n", `\N`, "\n").Replace(text)
}

// removeTags is the remove_tags mod: Subzero reads subtitles with it
// without keeping HTML tags, so pysubs2 turns <i>, <s>, <u> and <b> into
// override tags and drops the others, and then plaintext removes the
// override blocks.
func removeTags(text string) string {
	text = srtStyleTags.ReplaceAllStringFunc(text, func(tag string) string {
		m := srtStyleTags.FindStringSubmatch(tag)
		if m[1] != "" {
			return `{\` + m[2] + "0}"
		}
		return `{\` + m[2] + "1}"
	})
	return plaintext(htmlTags.ReplaceAllString(text, ""))
}

// PlainText returns a cue's text without its markup, as pysubs2's
// plaintext of a subtitle read by Subzero: SubRip's and WebVTT's tags and
// SSA's override blocks removed, hard spaces made spaces.
func PlainText(text string) string { return removeTags(text) }

// uppercaseSplit is Subzero's split_upper_re, with Python's Unicode \s.
var uppercaseSplit = regexp.MustCompile(`[\t\n\v\f\r\x1c-\x1f\p{Z}\x{85}]*[.!?♪\-][\t\n\v\f\r\x1c-\x1f\p{Z}\x{85}]*`)

// fixUppercase is FixUppercase.capitalize: every piece between sentence
// punctuation, and every separator, capitalized.
func fixUppercase(text string) string {
	var b strings.Builder
	last := 0
	for _, loc := range uppercaseSplit.FindAllStringIndex(text, -1) {
		b.WriteString(pyCapitalize(text[last:loc[0]]))
		b.WriteString(pyCapitalize(text[loc[0]:loc[1]]))
		last = loc[1]
	}
	b.WriteString(pyCapitalize(text[last:]))
	return b.String()
}

// emojiNode is a node of the emoji package's search tree.
type emojiNode struct {
	children map[rune]*emojiNode
	emoji    bool
}

// emojiData is the emoji package's EMOJI_DATA: its search tree and the
// emoji, by status component or not.
type emojiData struct {
	root      *emojiNode
	component map[string]bool
	all       map[string]bool
}

var emojiTable = sync.OnceValue(func() *emojiData {
	file := loadData[struct {
		Emoji      []string `json:"emoji"`
		Components []string `json:"components"`
	}]("emoji.json")
	d := &emojiData{root: &emojiNode{}, component: map[string]bool{}, all: map[string]bool{}}
	for _, e := range file.Emoji {
		d.all[e] = true
		node := d.root
		for _, r := range e {
			if node.children == nil {
				node.children = map[rune]*emojiNode{}
			}
			next := node.children[r]
			if next == nil {
				next = &emojiNode{}
				node.children[r] = next
			}
			node = next
		}
		node.emoji = true
	}
	for _, c := range file.Components {
		d.component[c] = true
	}
	return d
})

const (
	zwj           = '\u200d'
	textSelector  = '\ufe0e'
	emojiSelector = '\ufe0f'
)

// removeEmoji is emoji.replace_emoji(text): the emoji package's tokenize,
// without zero-width joiners between emoji, with the emoji removed.
func removeEmoji(text string) string {
	d := emojiTable()
	rs := []rune(text)
	type token struct {
		chars string
		emoji bool
	}
	var out strings.Builder
	var result []token
	flush := func() {
		for _, t := range result {
			if !t.emoji {
				out.WriteString(t.chars)
			}
		}
		result = result[:0]
	}
	var ignore []int
	for i := 0; i < len(rs); {
		consumed := false
		char := rs[i]
		if slices.Contains(ignore, i) {
			i++
			continue
		}
		if node := d.root.children[char]; node != nil {
			j := i + 1
			for j < len(rs) && node.children[rs[j]] != nil {
				if slices.Contains(ignore, j) {
					break
				}
				node = node.children[rs[j]]
				j++
			}
			if node.emoji {
				result = append(result, token{chars: string(rs[i:j]), emoji: true})
				i = j - 1
				consumed = true
			}
		} else if char == zwj && len(result) > 0 && d.all[result[len(result)-1].chars] &&
			i > 0 && d.root.children[rs[i-1]] != nil {
			// A joiner after an emoji: Subzero's emoji package goes back
			// and matches again, skipping it.
			ignore = append(ignore, i)
			lastChars := result[len(result)-1].chars
			if d.component[lastChars] {
				back := 0
				for _, t := range result[max(0, len(result)-2):] {
					back += len([]rune(t.chars))
				}
				i -= back
				if rs[i] == zwj {
					i++
					result = result[:len(result)-1]
				} else {
					result = result[:max(0, len(result)-2)]
				}
			} else {
				i -= len([]rune(lastChars))
				result = result[:len(result)-1]
			}
			continue
		} else if len(result) > 0 {
			flush()
		}
		if !consumed && char != textSelector && char != emojiSelector {
			result = append(result, token{chars: string(char)})
		}
		i++
	}
	flush()
	return out.String()
}

// tldNode is a node of the tld package's trie of public suffixes.
type tldNode struct {
	children  map[string]*tldNode
	exception string
	leaf      bool
}

// tldTrie is the trie of the public suffix list, built as the tld
// package's Trie.add builds it.
var tldTrie = sync.OnceValue(func() *tldNode {
	file := loadData[struct {
		Rules []string `json:"rules"`
	}]("tld.json")
	root := &tldNode{}
	for _, rule := range file.Rules {
		node := root
		parts := strings.Split(rule, ".")
		slices.Reverse(parts)
		for _, part := range parts {
			if exception, ok := strings.CutPrefix(part, "!"); ok {
				node.exception = exception
				break
			}
			if node.children == nil {
				node.children = map[string]*tldNode{}
			}
			child := node.children[part]
			if child == nil {
				child = &tldNode{}
				node.children[part] = child
			}
			node = child
		}
		node.leaf = true
	}
	return root
})

var urlProtocol = regexp.MustCompile(`^[a-z0-9.+:-]*//`)

// hasTLD reports whether get_tld(text, fail_silently=True,
// fix_protocol=True) of the tld package finds a public suffix.
func hasTLD(text string) bool {
	if !urlProtocol.MatchString(strings.ToLower(text)) {
		text = "https://" + text
	}
	host, ok := urlHostname(text)
	if !ok || host == "" {
		return false
	}
	host = strings.TrimRight(strings.ToLower(host), ".")
	parts := strings.Split(host, ".")
	node := tldTrie()
	found := false
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if node.children == nil || part == node.exception {
			break
		}
		child := node.children[part]
		if child == nil {
			child = node.children["*"]
		}
		if child == nil {
			break
		}
		node = child
		if node.leaf {
			found = true
		}
	}
	return found
}

// urlHostname is the hostname of Python's urllib.parse.urlsplit, false
// where urlsplit fails.
func urlHostname(u string) (string, bool) {
	if i := strings.IndexByte(u, ':'); i > 0 && isASCIILetter(rune(u[0])) &&
		strings.IndexFunc(u[:i], func(r rune) bool { return !isSchemeChar(r) }) < 0 {
		u = u[i+1:]
	}
	rest, ok := strings.CutPrefix(u, "//")
	if !ok {
		return "", true
	}
	netloc := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		netloc = rest[:i]
	}
	if strings.Contains(netloc, "[") != strings.Contains(netloc, "]") {
		return "", false
	}
	host := netloc
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	if _, bracketed, ok := strings.Cut(host, "["); ok {
		// An IPv6 address, which has no public suffix.
		host, _, _ = strings.Cut(bracketed, "]")
		return host, false
	}
	host, _, _ = strings.Cut(host, ":")
	return host, true
}

func isASCIILetter(r rune) bool { return r < unicode.MaxASCII && unicode.IsLetter(r) }

func isSchemeChar(r rune) bool {
	return isASCIILetter(r) || r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'
}

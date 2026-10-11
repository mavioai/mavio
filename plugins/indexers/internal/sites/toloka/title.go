package toloka

import (
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

func mustCompile(expr string, opts regexp2.RegexOptions) *regexp2.Regexp {
	return regexp2.MustCompile(expr, opts)
}

const ignoreCase = regexp2.IgnoreCase

// The expressions of Prowlarr's TolokaTitleParser, in .NET's syntax;
// \p{IsCyrillic}, which regexp2 lacks, is written as its block's range.
var (
	// tagExprs find the outermost groups in parentheses and in brackets.
	tagExprs = []*regexp2.Regexp{
		mustCompile(`\((?>\((?<c>)|[^()]+|\)(?<-c>))*(?(c)(?!))\)`, 0),
		mustCompile(`\[(?>\[(?<c>)|[^\[\]]+|\](?<-c>))*(?(c)(?!))\]`, 0),
	}

	dashes = mustCompile(`\p{Pd}`, ignoreCase)

	tvReplacements = []struct {
		re   *regexp2.Regexp
		with string
	}{
		{mustCompile(`\s(\d+),(\d+)`, 0), " $1-$2"},
		{mustCompile(`([\s-])Х+([\)\]])`, ignoreCase), "$1XX$2"},
		// Several seasons.
		{mustCompile(`(?:Сезон|Seasons?)\s*[:]*\s+(\d+-\d+)`, ignoreCase), "S$1"},
		{mustCompile(`Сезон\s*[:]*\s+(\d+).+(?:Серії|Серія|Серій|Епізоди?)+\s*[:]*\s+(\d+(?:-\d+)?)\s*з\s*([\w?])`, ignoreCase), "S$1E$2 of $3"},
		{mustCompile(`Сезон\s*[:]*\s+(\d+).+(?:Серії|Серія|Серій|Епізоди?)+\s*[:]*\s+(\d+(?:-\d+)?)`, ignoreCase), "S$1E$2"},
		{mustCompile(`Сезон\s*[:]*\s+(\d+)`, ignoreCase), "S$1"},
		{mustCompile(`(?:Серії|Серія|Серій|Епізоди?)+\s*[:]*\s+(\d+(?:-\d+)?)\s*з\s*([\w?])`, ignoreCase), "E$1 of $2"},
		{mustCompile(`(?:Серії|Серія|Серій|Епізоди?)+\s*[:]*\s+(\d+(?:-\d+)?)`, ignoreCase), "E$1"},
		{mustCompile(`Season\s*[:]*\s+(\d+).+(?:Episodes?)+\s*[:]*\s+(\d+(?:-\d+)?)\s*of\s*([\w?])`, ignoreCase), "S$1E$2 of $3"},
		{mustCompile(`Season\s*[:]*\s+(\d+).+(?:Episodes?)+\s*[:]*\s+(\d+(?:-\d+)?)`, ignoreCase), "S$1E$2"},
		{mustCompile(`Season\s*[:]*\s+(\d+(?:-\d+)?)`, ignoreCase), "S$1"},
		{mustCompile(`(?:Episodes?)+\s*[:]*\s+(\d+(?:-\d+)?)\s*of\s*([\w?])`, ignoreCase), "E$1 of $2"},
		{mustCompile(`(?:Episodes?)+\s*[:]+\s*[:]*\s+(\d+(?:-\d+)?)`, ignoreCase), "E$1"},
	}

	cyrillic = mustCompile(`(\([Ѐ-ӿ\W]+\))|(^[Ѐ-ӿ\W\d]+\/ )|([Ѐ-ӿ \-]+,+)|([Ѐ-ӿ]+)`, ignoreCase)

	ripReplacements = []struct {
		re   *regexp2.Regexp
		with string
	}{
		{mustCompile(`\b-Rip\b`, ignoreCase), "Rip"},
		{mustCompile(`\bHDTVRip\b`, ignoreCase), "HDTV"},
		{mustCompile(`\bWEB-DLRip\b`, ignoreCase), "WEB-DL"},
		{mustCompile(`\bWEBDLRip\b`, ignoreCase), "WEB-DL"},
		{mustCompile(`\bWEBDL\b`, ignoreCase), "WEB-DL"},
	}

	openSlash  = mustCompile(`\(\s*\/\s*`, 0)
	closeSlash = mustCompile(`\s*\/\s*\)`, 0)
	emptyTag   = mustCompile(`[\[\(]\s*[\)\]]`, 0)
	spaces     = mustCompile(`\s+`, 0)
)

// trimmed are the characters trimmed off titles.
const trimmed = " &,.!?+-_|/\\:;ʼ`"

func replace(re *regexp2.Regexp, s, with string) string {
	out, err := re.Replace(s, with, -1, -1)
	if err != nil {
		// Only a timeout fails, and none is set.
		return s
	}
	return out
}

// parseTitle makes a title of the site's, in Ukrainian and English, one
// Sonarr and Radarr parse, as Prowlarr's TolokaTitleParser does.
func parseTitle(title string, categories []int, stripCyrillic bool) string {
	title = replace(dashes, title, "-")
	if isTV(categories) {
		for _, r := range tvReplacements {
			title = replace(r.re, title, r.with)
		}
	}
	if stripCyrillic {
		title = strings.Trim(replace(cyrillic, title, ""), " -")
	}
	for _, r := range ripReplacements {
		title = replace(r.re, title, r.with)
	}
	title = moveFirstTagsToEnd(title)
	title = replace(openSlash, title, "(")
	title = replace(closeSlash, title, ")")
	title = replace(emptyTag, title, "")
	title = strings.Trim(title, trimmed)
	title = replace(spaces, title, " ")
	return strings.TrimSpace(title)
}

// isTV reports whether categories include TV or one of its
// subcategories.
func isTV(categories []int) bool {
	for _, c := range categories {
		if c >= 5000 && c < 6000 {
			return true
		}
	}
	return false
}

// moveFirstTagsToEnd moves the tags, in parentheses then in brackets,
// leading a title to its end, as Prowlarr's
// MoveFirstTagsToEndOfReleaseTitle does: the tags are those of the title
// before the moves, the text between them that of the title being moved.
func moveFirstTagsToEnd(input string) string {
	output := strings.Trim(input, trimmed)
	for _, re := range tagExprs {
		snapshot := output
		expected := 0
		m, _ := re.FindStringMatch(snapshot)
		for ; m != nil; m, _ = re.FindNextMatch(m) {
			if m.Index > expected {
				runes := []rune(output)
				// .NET's Substring would throw past the end.
				if m.Index > len(runes) {
					break
				}
				if strings.TrimSpace(string(runes[expected:m.Index])) != "" {
					break
				}
				expected = m.Index
			}
			tag := m.String()
			output = strings.TrimSpace(strings.Replace(output, tag, "", 1) + " " + tag)
			expected += utf8.RuneCountInString(tag)
		}
	}
	return strings.TrimSpace(output)
}

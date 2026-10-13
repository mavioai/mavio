package subtitle

import "sync"

// ChineseScript is the script of a Chinese text, as hanzidentifier tells
// it from the characters of CC-CEDICT.
type ChineseScript int

const (
	// ChineseUnknown is a text without Chinese characters.
	ChineseUnknown ChineseScript = iota
	// ChineseBoth is a text whose characters are all the same in both
	// scripts.
	ChineseBoth
	// ChineseTraditional is a text in Traditional Chinese.
	ChineseTraditional
	// ChineseSimplified is a text in Simplified Chinese.
	ChineseSimplified
	// ChineseMixed is a text with characters of only one script each from
	// both scripts.
	ChineseMixed
)

// hanziSets are the character sets of hanzidentifier.
type hanziSets struct {
	simplified, traditional, all map[rune]bool
}

var hanziData = sync.OnceValue(func() *hanziSets {
	file := loadData[struct {
		Simplified  string `json:"simplified"`
		Traditional string `json:"traditional"`
		All         string `json:"all"`
	}]("hanzi.json")
	set := func(s string) map[rune]bool {
		m := map[rune]bool{}
		for _, r := range s {
			m[r] = true
		}
		return m
	}
	return &hanziSets{simplified: set(file.Simplified), traditional: set(file.Traditional), all: set(file.All)}
})

// DetectChineseScript tells Simplified from Traditional Chinese as
// hanzidentifier.identify does: by whether the Chinese characters of the
// text all belong to one script's character set.
func DetectChineseScript(text string) ChineseScript {
	script, _, _ := chineseScript(text)
	return script
}

// chineseScript returns the script of a text and how many of its
// characters belong to only the simplified or the traditional script.
func chineseScript(text string) (script ChineseScript, simplifiedOnly, traditionalOnly int) {
	d := hanziData()
	found, allShared, allTraditional, allSimplified := false, true, true, true
	for _, r := range text {
		if !d.all[r] {
			continue
		}
		found = true
		simplified, traditional := d.simplified[r], d.traditional[r]
		allShared = allShared && simplified && traditional
		allTraditional = allTraditional && traditional
		allSimplified = allSimplified && simplified
		switch {
		case simplified && !traditional:
			simplifiedOnly++
		case traditional && !simplified:
			traditionalOnly++
		}
	}
	switch {
	case !found:
		return ChineseUnknown, 0, 0
	case allShared:
		return ChineseBoth, 0, 0
	case allTraditional:
		return ChineseTraditional, 0, traditionalOnly
	case allSimplified:
		return ChineseSimplified, simplifiedOnly, 0
	}
	return ChineseMixed, simplifiedOnly, traditionalOnly
}

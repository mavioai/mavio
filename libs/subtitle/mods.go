package subtitle

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/language"
)

// Subtitle modifications ported from Subzero, the mod engine of Bazarr
// (custom_libs/subzero/modification): mods are named by Subzero's
// identifiers, with arguments as in "shift_offset(h=0,m=0,s=1,ms=500)", and
// applied as SubtitleModifications.modify applies them.

// ErrInvalidMod is returned for a mod identifier ApplyMods does not accept.
var ErrInvalidMod = errors.New("invalid subtitle mod")

// modSpec describes a mod as Subzero's mod classes do.
type modSpec struct {
	// order sorts the mods applied line by line.
	order int
	// exclusive mods keep only their last occurrence.
	exclusive bool
	// argsMergeable mods add up the arguments of their occurrences.
	argsMergeable bool
	// wholeFile mods modify all cues at once, before the line mods or,
	// when applyLast is set, after them.
	wholeFile, applyLast bool
	// mostlyUppercase mods apply only to mostly upper-case subtitles.
	mostlyUppercase bool
	// languages limits a mod to languages, by ISO 639-2/T code.
	languages []string
	// check validates the arguments.
	check func(args modArgs) error
}

// modSpecs are Subzero's registered mods.
var modSpecs = map[string]modSpec{
	"common":        {order: 40, exclusive: true},
	"remove_tags":   {exclusive: true, wholeFile: true},
	"reverse_rtl":   {order: 50, exclusive: true, languages: []string{"heb", "ara", "fas"}},
	"fix_uppercase": {order: 41, exclusive: true, wholeFile: true, applyLast: true, mostlyUppercase: true},
	"remove_HI":     {order: 20, exclusive: true},
	"OCR_fixes":     {order: 10, exclusive: true},
	"color":         {exclusive: true, wholeFile: true, applyLast: true},
	"emoji":         {exclusive: true, wholeFile: true},
	"change_FPS":    {exclusive: true, wholeFile: true, check: checkFPSArgs},
	"shift_offset":  {argsMergeable: true, wholeFile: true, check: checkIntArgs},
	"two_point_fit": {wholeFile: true, check: checkTwoPointFitArgs},
}

// ModNames returns the identifiers of the mods ApplyMods knows, in
// Subzero's order of registration.
func ModNames() []string {
	return []string{
		"OCR_fixes", "remove_HI", "common", "remove_tags", "reverse_rtl", "fix_uppercase",
		"color", "emoji", "change_FPS", "shift_offset", "two_point_fit",
	}
}

// modColors are the colors of the color mod, in Subzero's order.
var modColors = [][2]string{
	{"white", "#FFFFFF"},
	{"light-grey", "#C0C0C0"},
	{"red", "#FF0000"},
	{"green", "#00FF00"},
	{"yellow", "#FFFF00"},
	{"blue", "#0000FF"},
	{"magenta", "#FF00FF"},
	{"cyan", "#00FFFF"},
	{"black", "#000000"},
	{"dark-red", "#800000"},
	{"dark-green", "#008000"},
	{"dark-yellow", "#808000"},
	{"dark-blue", "#000080"},
	{"dark-magenta", "#800080"},
	{"dark-cyan", "#008080"},
	{"dark-grey", "#808080"},
}

// ModColors returns the color names of the mod "color(name=...)".
func ModColors() []string {
	names := make([]string, len(modColors))
	for i, c := range modColors {
		names[i] = c[0]
	}
	return names
}

// modArgs are the arguments of a mod identifier, in order.
type modArgs [][2]string

func (a modArgs) get(key string) (string, bool) {
	for i := len(a) - 1; i >= 0; i-- {
		if a[i][0] == key {
			return a[i][1], true
		}
	}
	return "", false
}

// pyInt is Python's int() of a string.
func pyInt(s string) (int, error) {
	return strconv.Atoi(strings.ReplaceAll(strings.TrimPrefix(pyStrip(s), "+"), "_", ""))
}

func (a modArgs) int(key string) int {
	v, ok := a.get(key)
	if !ok {
		return 0
	}
	n, _ := pyInt(v)
	return n
}

func (a modArgs) float(key string) float64 {
	v, _ := a.get(key)
	f, _ := strconv.ParseFloat(pyStrip(v), 64)
	return f
}

func checkIntArgs(args modArgs) error {
	for _, kv := range args {
		if _, err := pyInt(kv[1]); err != nil {
			return fmt.Errorf("%s=%s is not an integer", kv[0], kv[1])
		}
	}
	return nil
}

func checkFPSArgs(args modArgs) error {
	for _, key := range []string{"from", "to"} {
		v, ok := args.get(key)
		if !ok {
			return fmt.Errorf("missing %s", key)
		}
		f, err := strconv.ParseFloat(pyStrip(v), 64)
		if err != nil || !(f > 0) || math.IsInf(f, 0) {
			return fmt.Errorf("%s=%s is not a positive frame rate", key, v)
		}
	}
	return nil
}

func checkTwoPointFitArgs(args modArgs) error {
	if err := checkFPSArgs(args); err != nil {
		return err
	}
	var ints modArgs
	for _, kv := range args {
		if kv[0] != "from" && kv[0] != "to" {
			ints = append(ints, kv)
		}
	}
	return checkIntArgs(ints)
}

// parsedMod is a mod identifier split as Subzero's parse_identifier does.
type parsedMod struct {
	identifier string
	name       string
	args       modArgs
	spec       modSpec
}

func parseMod(identifier string) (parsedMod, error) {
	m := parsedMod{identifier: identifier, name: identifier}
	if _, known := modSpecs[identifier]; !known && strings.Contains(identifier, "=") {
		open := strings.IndexByte(identifier, '(')
		if open < 0 || !strings.HasSuffix(identifier, ")") {
			return m, fmt.Errorf("%w: %q", ErrInvalidMod, identifier)
		}
		m.name = identifier[:open]
		for _, arg := range strings.Split(identifier[open+1:len(identifier)-1], ",") {
			k, v, ok := strings.Cut(arg, "=")
			if !ok || strings.Contains(v, "=") {
				return m, fmt.Errorf("%w: %q", ErrInvalidMod, identifier)
			}
			m.args = append(m.args, [2]string{k, v})
		}
	}
	spec, ok := modSpecs[m.name]
	if !ok {
		return m, fmt.Errorf("%w: unknown mod %q", ErrInvalidMod, m.name)
	}
	m.spec = spec
	if spec.check != nil {
		if err := spec.check(m.args); err != nil {
			return m, fmt.Errorf("%w: %s: %w", ErrInvalidMod, identifier, err)
		}
	}
	return m, nil
}

// CheckMod reports whether ApplyMods accepts a mod identifier, such as
// "remove_HI", "color(name=yellow)" or "change_FPS(from=23.976,to=25)".
func CheckMod(identifier string) error {
	_, err := parseMod(identifier)
	return err
}

// modContext is what Subzero's processors see of the subtitle.
type modContext struct {
	// lang is the ISO 639-2/T code of the subtitle's language, or "".
	lang string
	// region is the language tag's explicit region, such as "MX".
	region          string
	mostlyUppercase bool
}

// newModContext returns the context of a subtitle in the language of a
// BCP 47 tag.
func newModContext(tag string) *modContext {
	ctx := &modContext{}
	if tag == "" {
		return ctx
	}
	t, err := language.Parse(tag)
	if err != nil {
		return ctx
	}
	if base, conf := t.Base(); conf != language.No {
		ctx.lang = base.ISO3()
	}
	if region, conf := t.Region(); conf == language.Exact {
		ctx.region = region.String()
	}
	return ctx
}

// ApplyMods applies Subzero's mods to s, as Bazarr does before saving
// subtitles: line mods (OCR_fixes, remove_HI, common, reverse_rtl) in
// Subzero's order, line by line, with whole-file mods before them
// (remove_tags, emoji, change_FPS, shift_offset, two_point_fit) or after
// them (fix_uppercase, color). language is the BCP 47 tag of the
// subtitles, which selects the OCR fix dictionary and language-specific
// fixes; "" when unknown. A mod identifier ApplyMods does not accept
// fails it without modifying s.
//
// It returns the identifiers of the mods used, as Subzero reports them:
// mods for another language or for upper-case subtitles left out, and
// repeated time offsets merged.
func (s *Subtitle) ApplyMods(lang string, mods []string) ([]string, error) {
	parsed := make([]parsedMod, 0, len(mods))
	for _, id := range mods {
		m, err := parseMod(id)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, m)
	}
	ctx := newModContext(lang)
	ctx.mostlyUppercase = s.mostlyUppercase()

	final, used := prepareMods(parsed, ctx)
	var lineMods, fileMods []finalMod
	for _, m := range final {
		if m.spec.wholeFile {
			fileMods = append(fileMods, m)
		} else {
			lineMods = append(lineMods, m)
		}
	}
	for _, m := range fileMods {
		if !m.spec.applyLast {
			s.applyFileMod(m)
		}
	}
	slices.SortStableFunc(lineMods, func(a, b finalMod) int { return a.spec.order - b.spec.order })
	if len(lineMods) > 0 {
		s.applyLineMods(lineMods, ctx)
	}
	for _, m := range fileMods {
		if m.spec.applyLast {
			s.applyFileMod(m)
		}
	}
	return used, nil
}

// finalMod is a mod to apply, with its final arguments.
type finalMod struct {
	name string
	args modArgs
	spec modSpec
}

// prepareMods is Subzero's prepare_mods: it drops mods that do not apply,
// keeps the last occurrence of exclusive mods and merges the arguments of
// mergeable ones. It returns the mods to apply in order and the
// identifiers used.
func prepareMods(mods []parsedMod, ctx *modContext) ([]finalMod, []string) {
	var final []finalMod
	index := func(name string) int {
		return slices.IndexFunc(final, func(m finalMod) bool { return m.name == name })
	}
	var used []string
	type merged struct {
		name string
		args []mergedArg
	}
	var mergedMods []*merged
	findMerged := func(name string) *merged {
		for _, m := range mergedMods {
			if m.name == name {
				return m
			}
		}
		return nil
	}
	for _, m := range mods {
		if i := index(m.name); i >= 0 && m.spec.exclusive {
			final = slices.Delete(final, i, i+1)
		}
		if len(m.spec.languages) > 0 && !slices.Contains(m.spec.languages, ctx.lang) {
			continue
		}
		if m.spec.mostlyUppercase && !ctx.mostlyUppercase {
			continue
		}
		if m.spec.argsMergeable {
			if prev := findMerged(m.name); prev != nil {
				prev.args = mergeArgs(prev.args, m.args)
				continue
			}
			mergedMods = append(mergedMods, &merged{name: m.name, args: mergeArgs(nil, m.args)})
			used = append(used, m.name+"_ORIG_POSITION")
			continue
		}
		if i := index(m.name); i >= 0 {
			final[i].args = m.args
		} else {
			final = append(final, finalMod{name: m.name, args: m.args, spec: m.spec})
		}
		used = append(used, m.identifier)
	}
	for _, m := range mergedMods {
		pos := slices.Index(used, m.name+"_ORIG_POSITION")
		if len(m.args) == 0 {
			used = slices.Delete(used, pos, pos+1)
			continue
		}
		parts := make([]string, len(m.args))
		args := make(modArgs, len(m.args))
		for i, a := range m.args {
			parts[i] = fmt.Sprintf("%s=%d", a.key, a.value)
			args[i] = [2]string{a.key, strconv.Itoa(a.value)}
		}
		used[pos] = m.name + "(" + strings.Join(parts, ",") + ")"
		final = append(final, finalMod{name: m.name, args: args, spec: modSpecs[m.name]})
	}
	return final, used
}

type mergedArg struct {
	key   string
	value int
}

// mergeArgs is ShiftOffset.merge_args: the sums of the arguments by key,
// without zeros.
func mergeArgs(prev []mergedArg, args modArgs) []mergedArg {
	out := slices.Clone(prev)
	for _, kv := range args {
		v, _ := pyInt(kv[1])
		if v == 0 {
			continue
		}
		if i := slices.IndexFunc(out, func(a mergedArg) bool { return a.key == kv[0] }); i >= 0 {
			out[i].value += v
		} else {
			out = append(out, mergedArg{kv[0], v})
		}
	}
	return slices.DeleteFunc(out, func(a mergedArg) bool { return a.value == 0 })
}

// entryText is a cue's text as pysubs2 holds it, with \N line breaks.
func entryText(text string) string { return strings.ReplaceAll(text, "\n", `\N`) }

// mostlyUppercase is Subzero's detect_uppercase: whether at least 90% of
// the cased letters of the first 50 non-empty cues, without what looks
// like hearing-impaired annotations, are upper case, counting more than
// 100 of them.
func (s *Subtitle) mostlyUppercase() bool {
	const (
		maxEntries   = 50
		minPercent   = 90
		minUppercase = 100
	)
	hi := hearingImpairedProcessors()[:4]
	entries, upper, lower := 0, 0, 0
	ctx := &lineContext{mod: &modContext{}}
	for _, c := range s.Cues {
		text := pyStrip(entryText(c.Text))
		for _, p := range hi {
			text, _ = p.apply(text, ctx)
		}
		if pyStrip(text) != "" {
			for _, r := range text {
				switch {
				case unicode.IsUpper(r):
					upper++
				case unicode.IsLower(r):
					lower++
				}
			}
			entries++
		}
		if entries >= maxEntries {
			break
		}
	}
	total := upper + lower
	return total > 0 && upper > minUppercase && float64(upper)*100/float64(total) >= minPercent
}

// applyFileMod applies a mod that modifies the whole file.
func (s *Subtitle) applyFileMod(m finalMod) {
	switch m.name {
	case "remove_tags":
		for i := range s.Cues {
			s.Cues[i].Text = removeTags(s.Cues[i].Text)
		}
	case "fix_uppercase":
		for i := range s.Cues {
			s.Cues[i].Text = fixUppercase(plaintext(s.Cues[i].Text))
		}
	case "color":
		name, _ := m.args.get("name")
		for _, c := range modColors {
			if c[0] == name {
				for i := range s.Cues {
					s.Cues[i].Text = `<font color="` + c[1] + `">` + s.Cues[i].Text + "</font>"
				}
			}
		}
	case "emoji":
		for i := range s.Cues {
			s.Cues[i].Text = removeEmoji(s.Cues[i].Text)
		}
	case "change_FPS":
		s.transformFramerate(m.args.float("from"), m.args.float("to"))
	case "shift_offset":
		s.shift(makeTime(m.args.int("h"), m.args.int("m"), m.args.int("s"), m.args.int("ms")))
	case "two_point_fit":
		s.shift(-makeTime(m.args.int("rh"), m.args.int("rm"), m.args.int("rs"), m.args.int("rms")))
		s.transformFramerate(m.args.float("from"), m.args.float("to"))
		s.shift(makeTime(m.args.int("oh"), m.args.int("om"), m.args.int("os"), m.args.int("oms")))
	}
}

// makeTime is pysubs2's make_time, in milliseconds.
func makeTime(h, m, s, ms int) time.Duration {
	return time.Duration(h*3_600_000+m*60_000+s*1000+ms) * time.Millisecond
}

// shift is pysubs2's SSAFile.shift.
func (s *Subtitle) shift(d time.Duration) {
	for i := range s.Cues {
		s.Cues[i].Start += d
		s.Cues[i].End += d
	}
}

// transformFramerate is pysubs2's SSAFile.transform_framerate: times in
// milliseconds scaled by from/to, rounded half to even.
func (s *Subtitle) transformFramerate(from, to float64) {
	if !(from > 0) || !(to > 0) {
		return
	}
	ratio := from / to
	scale := func(d time.Duration) time.Duration {
		return time.Duration(math.RoundToEven(float64(d.Milliseconds())*ratio)) * time.Millisecond
	}
	for i := range s.Cues {
		s.Cues[i].Start = scale(s.Cues[i].Start)
		s.Cues[i].End = scale(s.Cues[i].End)
	}
}

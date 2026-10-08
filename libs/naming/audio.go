package naming

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

var albumNameSeparators = mustCompile(`[-\.\(\)\s]+`, false)

// IsMultiPartAlbum reports whether path is a disc folder of a multi-disc
// album, such as "CD 1" or "Disc-2".
func (p *Parser) IsMultiPartAlbum(path string) bool {
	name := fileName(path)
	if name == "" {
		return false
	}
	name = strings.TrimLeftFunc(albumNameSeparators.replaceAll(name, " "), unicode.IsSpace)
	for _, prefix := range p.opts.AlbumStackingPrefixes {
		if !hasPrefixFold(name, prefix) {
			continue
		}
		rest := strings.TrimSpace(name[len(prefix):])
		number, _, _ := strings.Cut(rest, " ")
		if _, ok := parseInt(number); ok {
			return true
		}
	}
	return false
}

// AudioBookFile is a file of an audiobook.
type AudioBookFile struct {
	Path      string
	Container string
	// PartNumber and ChapterNumber are nil when the name has none.
	PartNumber    *int
	ChapterNumber *int
}

// Compare orders files by chapter, then part, then path; missing numbers
// sort first.
func (f AudioBookFile) Compare(o AudioBookFile) int {
	if c := compareOptional(f.ChapterNumber, o.ChapterNumber); c != 0 {
		return c
	}
	if c := compareOptional(f.PartNumber, o.PartNumber); c != 0 {
		return c
	}
	return strings.Compare(f.Path, o.Path)
}

func compareOptional(a, b *int) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	return cmp.Compare(*a, *b)
}

// ResolveAudioBookFile resolves an audiobook file; it returns false for a
// file without a name or an audio extension.
func (p *Parser) ResolveAudioBookFile(path string) (AudioBookFile, bool) {
	if path == "" || fileNameWithoutExtension(path) == "" {
		return AudioBookFile{}, false
	}
	ext := extension(path)
	if !containsFold(p.opts.AudioFileExtensions, ext) {
		return AudioBookFile{}, false
	}
	part, chapter := p.ParseAudioBookPath(path)
	return AudioBookFile{Path: path, Container: strings.TrimLeft(ext, "."), PartNumber: part, ChapterNumber: chapter}, true
}

// ParseAudioBookPath finds the part and chapter numbers in an audiobook
// file name; each comes from the first expression that captures it.
func (p *Parser) ParseAudioBookPath(path string) (part, chapter *int) {
	name := fileNameWithoutExtension(path)
	for _, re := range p.audioBookParts {
		m := re.match(name)
		if m == nil {
			continue
		}
		if chapter == nil {
			if s, ok := m.group("chapter"); ok {
				if n, ok := parseInt(s); ok {
					chapter = &n
				}
			}
		}
		if part == nil {
			if s, ok := m.group("part"); ok {
				if n, ok := parseInt(s); ok {
					part = &n
				}
			}
		}
	}
	return part, chapter
}

// ParseAudioBookName splits an audiobook name into title and year.
func (p *Parser) ParseAudioBookName(name string) (title string, year *int) {
	titleSet := false
	for _, re := range p.audioBookNames {
		m := re.match(name)
		if m == nil {
			continue
		}
		if !titleSet {
			if s, ok := m.group("name"); ok {
				title, titleSet = s, true
			}
		}
		if year == nil {
			if s, ok := m.group("year"); ok {
				if n, ok := parseInt(s); ok {
					year = &n
				}
			}
		}
	}
	if title == "" {
		title = name
	}
	return title, year
}

// AudioBook is an audiobook made of the files of one folder.
type AudioBook struct {
	Name              string
	Year              *int
	Files             []AudioBookFile
	Extras            []AudioBookFile
	AlternateVersions []AudioBookFile
}

// ResolveAudioBookStacks groups audiobook files by folder; files outside
// any folder stand alone.
func ResolveAudioBookStacks(files []AudioBookFile) []FileStack {
	var dirs []string
	byDir := map[string][]string{}
	for _, f := range files {
		dir := dirName(f.Path)
		if _, ok := byDir[dir]; !ok {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], f.Path)
	}
	var out []FileStack
	for _, dir := range dirs {
		if dir == "" {
			for _, path := range byDir[dir] {
				out = append(out, FileStack{Name: fileNameWithoutExtension(path), Files: []string{path}})
			}
			continue
		}
		out = append(out, FileStack{Name: fileName(dir), Files: byDir[dir]})
	}
	return out
}

// ResolveAudioBooks groups audiobook files into audiobooks, separating
// extras and alternate versions.
func (p *Parser) ResolveAudioBooks(paths []string) []AudioBook {
	var files []AudioBookFile
	for _, path := range paths {
		if f, ok := p.ResolveAudioBookFile(path); ok {
			files = append(files, f)
		}
	}
	var out []AudioBook
	for _, stack := range ResolveAudioBookStacks(files) {
		var stackFiles []*AudioBookFile
		for _, path := range stack.Files {
			if f, ok := p.ResolveAudioBookFile(path); ok {
				stackFiles = append(stackFiles, &f)
			}
		}
		slices.SortStableFunc(stackFiles, func(a, b *AudioBookFile) int { return a.Compare(*b) })
		name, year := p.ParseAudioBookName(stack.Name)
		main, extras, alternates := splitAudioBookFiles(stackFiles, name)
		out = append(out, AudioBook{Name: name, Year: year, Files: deref(main), Extras: deref(extras), AlternateVersions: deref(alternates)})
	}
	return out
}

// splitAudioBookFiles separates extras and alternate versions from the
// files of an audiobook.
func splitAudioBookFiles(files []*AudioBookFile, name string) (main, extras, alternates []*AudioBookFile) {
	main = files
	hasNumbers := slices.ContainsFunc(files, func(f *AudioBookFile) bool { return f.ChapterNumber != nil || f.PartNumber != nil })
	byContainerAndPath := func(a, b *AudioBookFile) int {
		if c := compareCulture(a.Container, b.Container); c != 0 {
			return c
		}
		return compareCulture(a.Path, b.Path)
	}
	without := func(list, remove []*AudioBookFile) []*AudioBookFile {
		return slices.DeleteFunc(slices.Clone(list), func(f *AudioBookFile) bool { return slices.Contains(remove, f) })
	}
	dotted := strings.ReplaceAll(name, " ", ".")

	for _, group := range groupByNumbers(files) {
		numbered := group[0].ChapterNumber != nil || group[0].PartNumber != nil
		switch {
		case !numbered && (len(group) > 1 || hasNumbers):
			var ex, alt []*AudioBookFile
			for _, f := range group {
				n := fileNameWithoutExtension(f.Path)
				if strings.EqualFold(n, "audiobook") || containsStringFold(n, name) || containsStringFold(n, dotted) {
					alt = append(alt, f)
				} else {
					ex = append(ex, f)
				}
			}
			if ex != nil {
				slices.SortStableFunc(ex, byContainerAndPath)
				main = without(main, ex)
				extras = append(extras, ex...)
			}
			if alt != nil {
				slices.SortStableFunc(alt, byContainerAndPath)
				primary := mainAudioBookFile(alt, name, byContainerAndPath)
				alt = slices.DeleteFunc(alt, func(f *AudioBookFile) bool { return f == primary })
				main = without(main, alt)
				alternates = append(alternates, alt...)
			}
		case numbered && len(group) > 1:
			sorted := slices.Clone(group)
			slices.SortStableFunc(sorted, byContainerAndPath)
			main = without(main, sorted[1:])
			alternates = append(alternates, sorted[1:]...)
		}
	}
	return main, extras, alternates
}

// groupByNumbers groups files by chapter and part number, in order of
// first appearance.
func groupByNumbers(files []*AudioBookFile) [][]*AudioBookFile {
	type numbers struct {
		hasChapter, hasPart bool
		chapter, part       int
	}
	key := func(f *AudioBookFile) numbers {
		var k numbers
		if f.ChapterNumber != nil {
			k.hasChapter, k.chapter = true, *f.ChapterNumber
		}
		if f.PartNumber != nil {
			k.hasPart, k.part = true, *f.PartNumber
		}
		return k
	}
	var groups [][]*AudioBookFile
	index := map[numbers]int{}
	for _, f := range files {
		k := key(f)
		i, ok := index[k]
		if !ok {
			i = len(groups)
			index[k] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], f)
	}
	return groups
}

func mainAudioBookFile(files []*AudioBookFile, name string, order func(a, b *AudioBookFile) int) *AudioBookFile {
	for _, f := range files {
		if strings.EqualFold(fileNameWithoutExtension(f.Path), name) {
			return f
		}
	}
	for _, f := range files {
		if strings.EqualFold(fileNameWithoutExtension(f.Path), "audiobook") {
			return f
		}
	}
	return slices.MinFunc(files, order)
}

func deref(files []*AudioBookFile) []AudioBookFile {
	out := make([]AudioBookFile, len(files))
	for i, f := range files {
		out[i] = *f
	}
	return out
}

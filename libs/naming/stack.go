package naming

import (
	"slices"
	"strings"
)

// FileEntry is a file or folder handed to the resolvers.
type FileEntry struct {
	Path  string
	IsDir bool
}

// FileStack is a video split into parts ("movie cd1.avi", "movie cd2.avi").
type FileStack struct {
	Name  string
	Files []string
	IsDir bool
}

// ContainsFile reports whether the stack holds file.
func (s FileStack) ContainsFile(file string, isDir bool) bool {
	return file != "" && s.IsDir == isDir && containsFold(s.Files, file)
}

// ResolveFileStacks groups video files into stacks.
func (p *Parser) ResolveFileStacks(paths []string) []FileStack {
	return p.ResolveStacks(entries(paths, false))
}

// ResolveDirectoryStacks groups video folders into stacks.
func (p *Parser) ResolveDirectoryStacks(paths []string) []FileStack {
	return p.ResolveStacks(entries(paths, true))
}

func entries(paths []string, isDir bool) []FileEntry {
	out := make([]FileEntry, len(paths))
	for i, path := range paths {
		out[i] = FileEntry{Path: path, IsDir: isDir}
	}
	return out
}

type stackParts struct {
	name      string
	isDir     bool
	numerical bool
	partType  string
	// parts maps the lower-cased part number to the path, in order.
	numbers []string
	paths   []string
}

func (s *stackParts) contains(number string) bool {
	return slices.Contains(s.numbers, strings.ToLower(number))
}

// ResolveStacks groups video files and folders whose names differ only in
// a part number into stacks of at least two parts.
func (p *Parser) ResolveStacks(files []FileEntry) []FileStack {
	var candidates []FileEntry
	for _, f := range files {
		if f.IsDir || p.IsVideoFile(f.Path) || p.IsStubFile(f.Path) {
			candidates = append(candidates, f)
		}
	}
	slices.SortStableFunc(candidates, func(a, b FileEntry) int { return compareCulture(a.Path, b.Path) })

	var stacks []*stackParts
	byName := map[string]*stackParts{}
	for _, f := range candidates {
		name := fileName(f.Path)
		for _, rule := range p.videoStackRules {
			m := rule.re.match(name)
			if m == nil {
				continue
			}
			stackName, _ := m.group("filename")
			number, _ := m.group("number")
			partType, ok := m.group("parttype")
			if !ok {
				partType = "unknown"
			}
			stack, ok := byName[stackName]
			if !ok {
				stack = &stackParts{name: stackName, isDir: f.IsDir, numerical: rule.Numerical, partType: partType}
				byName[stackName] = stack
				stacks = append(stacks, stack)
			}
			if len(stack.paths) > 0 {
				if stack.isDir != f.IsDir || !strings.EqualFold(partType, stack.partType) || stack.contains(number) {
					continue
				}
				if rule.Numerical != stack.numerical {
					break
				}
			}
			stack.numbers = append(stack.numbers, strings.ToLower(number))
			stack.paths = append(stack.paths, f.Path)
			break
		}
	}

	var out []FileStack
	for _, s := range stacks {
		if len(s.paths) >= 2 {
			out = append(out, FileStack{Name: s.name, Files: s.paths, IsDir: s.isDir})
		}
	}
	return out
}

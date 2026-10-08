package naming

import (
	"strings"
	"unicode"

	"github.com/mavioai/mavio/libs/core"
)

// VideoFile is what a video file or folder name says about the video.
type VideoFile struct {
	Path string
	// Container is the extension without the dot; empty for folders.
	Container string
	// Name is the cleaned title.
	Name string
	Year *int
	// Extra is set for trailers and other extras, with the rule that
	// matched.
	Extra     core.ExtraKind
	ExtraRule *ExtraRule
	Is3D      bool
	Format3D  string
	// IsStub marks placeholder files for offline discs ("movie.dvd.disc").
	IsStub   bool
	StubType string
	IsDir    bool
}

// FileNameWithoutExtension returns the file name without extension, or the
// folder name.
func (f VideoFile) FileNameWithoutExtension() string {
	if f.IsDir {
		return fileName(f.Path)
	}
	return fileNameWithoutExtension(f.Path)
}

// ResolveVideo parses a video file or folder path. It returns false for a
// file that is neither a video nor a stub. Unless parseName is false, the
// name is cleaned and the year extracted. A folder named like an extras
// folder is not an extra when it is libraryRoot itself.
func (p *Parser) ResolveVideo(path string, isDir, parseName bool, libraryRoot string) (VideoFile, bool) {
	if path == "" {
		return VideoFile{}, false
	}
	var (
		isStub    bool
		container string
		stubType  string
	)
	if !isDir {
		ext := extension(path)
		if !containsFold(p.opts.VideoFileExtensions, ext) {
			var ok bool
			if stubType, ok = p.ResolveStub(path); !ok {
				return VideoFile{}, false
			}
			isStub = true
		}
		container = strings.TrimLeft(ext, ".")
	}

	is3D, format3D := p.Parse3D(path)
	extra, rule := p.ExtraInfo(path, libraryRoot)
	name := fileNameWithoutExtension(path)
	var year *int
	if parseName {
		name, year = p.CleanDateTime(name)
		if cleaned, ok := p.CleanString(name); ok {
			name = cleaned
		}
	}
	return VideoFile{
		Path:      path,
		Container: container,
		Name:      name,
		Year:      year,
		Extra:     extra,
		ExtraRule: rule,
		Is3D:      is3D,
		Format3D:  format3D,
		IsStub:    isStub,
		StubType:  stubType,
		IsDir:     isDir,
	}, true
}

// IsVideoFile reports whether path has a video extension.
func (p *Parser) IsVideoFile(path string) bool {
	return containsFold(p.opts.VideoFileExtensions, extension(path))
}

// IsStubFile reports whether path has a stub extension.
func (p *Parser) IsStubFile(path string) bool {
	return containsFold(p.opts.StubFileExtensions, extension(path))
}

// IsAudioFile reports whether path has an audio extension.
func (p *Parser) IsAudioFile(path string) bool {
	return containsFold(p.opts.AudioFileExtensions, extension(path))
}

// ResolveStub reports whether path is a stub file and returns its type,
// taken from the token before the extension ("movie.bluray.disc"); the type
// is empty when the token is unknown.
func (p *Parser) ResolveStub(path string) (stubType string, ok bool) {
	if path == "" || !p.IsStubFile(path) {
		return "", false
	}
	token := strings.TrimLeft(extension(fileNameWithoutExtension(path)), ".")
	for _, rule := range p.opts.StubTypes {
		if strings.EqualFold(token, rule.Token) {
			return rule.StubType, true
		}
	}
	return "", true
}

// CleanDateTime splits a video name into the title and the release year,
// as in "Movie (2010)" or "Movie.2010.1080p".
func (p *Parser) CleanDateTime(name string) (string, *int) {
	if name == "" {
		return name, nil
	}
	for _, re := range p.cleanDateTimes {
		m := re.match(name)
		if m == nil || m.groupCount() != 5 {
			continue
		}
		title, ok1 := m.groupN(1)
		yearText, ok2 := m.groupN(2)
		if !ok1 || !ok2 {
			continue
		}
		if year, ok := parseInt(yearText); ok {
			return strings.TrimRightFunc(title, unicode.IsSpace), &year
		}
	}
	return name, nil
}

// CleanString strips release tags such as resolutions, codecs and
// bracketed groups from a video name. It reports false when nothing was
// stripped.
func (p *Parser) CleanString(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	cleaned := false
	for _, re := range p.cleanStrings {
		m := re.match(name)
		if m == nil {
			continue
		}
		// Like .NET, an expression with a "cleaned" group cleans even when
		// the group did not participate.
		if g := m.m.GroupByName("cleaned"); g != nil {
			v, _ := groupValue(g)
			name = strings.TrimSpace(v)
			cleaned = true
		}
	}
	if !cleaned {
		return "", false
	}
	return name, true
}

// Parse3D finds a 3D format token in a video path.
func (p *Parser) Parse3D(path string) (is3D bool, format string) {
	delimiters := append(append([]rune{}, p.opts.VideoFlagDelimiters...), ' ')
	isDelimiter := func(r rune) bool {
		for _, d := range delimiters {
			if r == d {
				return true
			}
		}
		return false
	}
	tokens := strings.FieldsFunc(path, isDelimiter)
	// FieldsFunc drops empty tokens, which can never equal a rule token.
	for _, rule := range p.opts.Format3DRules {
		foundPrefix := rule.PrecedingToken == ""
		for _, token := range tokens {
			if !foundPrefix {
				foundPrefix = strings.EqualFold(token, rule.PrecedingToken)
				continue
			}
			if strings.EqualFold(token, rule.Token) {
				return true, rule.Token
			}
		}
	}
	return false, ""
}

// ExtraInfo classifies path as an extra by the video extra rules; the kind
// is empty when no rule matches. A rule on the folder name does not apply
// when the folder is libraryRoot itself.
func (p *Parser) ExtraInfo(path, libraryRoot string) (core.ExtraKind, *ExtraRule) {
	isAudio := p.IsAudioFile(path)
	isVideo := p.IsVideoFile(path)
	name := fileName(path)
	// Digits are trimmed from the end to recognize "-trailer2" or "sample1".
	trimmed := strings.TrimRight(fileNameWithoutExtension(path), "0123456789")
	dir := dirName(path)
	dirNameOnly := fileName(dir)
	for i, rule := range p.opts.VideoExtraRules {
		if (rule.Media == MediaAudio && !isAudio) || (rule.Media == MediaVideo && !isVideo) {
			continue
		}
		var ok bool
		switch rule.Type {
		case ExtraFilename:
			ok = strings.EqualFold(trimmed, rule.Token)
		case ExtraSuffix:
			ok = hasSuffixFold(trimmed, rule.Token)
		case ExtraRegex:
			ok = p.extraRuleRegexes[i].isMatch(name)
		case ExtraDirectoryName:
			ok = strings.EqualFold(dirNameOnly, rule.Token) && !strings.EqualFold(dir, libraryRoot)
		}
		if ok {
			return rule.Kind, &p.opts.VideoExtraRules[i]
		}
	}
	return "", nil
}

package naming

import "strings"

var (
	bookNames = []*regex{
		// seriesName (seriesYear) #index (of count) (year); only the series
		// name and index are required.
		mustCompile(`^(?<seriesName>.+?)((\s\((?<seriesYear>[0-9]{4})\))?)\s#(?<index>[0-9]+)(?:\.0)?((\s\(of\s(?<count>[0-9]+)\))?)((\s\((?<year>[0-9]{4})\))?)$`, false),
		mustCompile(`^(?<name>.+?)\s\((?<seriesName>.+?),\s#(?<index>[0-9]+)\)(?:\.0)?((\s\((?<year>[0-9]{4})\))?)$`, false),
		mustCompile(`^(?<index>[0-9]+)(?:\.0)?\s\-\s(?<name>.+?)((\s\((?<year>[0-9]{4})\))?)$`, false),
		mustCompile(`(?<name>.*)\((?<year>[0-9]{4})\)`, false),
		// The whole string as the name, as a last resort.
		mustCompile(`(?<name>.*)`, false),
	}
	comicName = mustCompile(`^(?<name>.+?)(\sv(?<volume>[0-9]+))?(\sc(?<chapter>[0-9]+))?$`, false)
)

// BookFileName is what a book file name says about the book.
type BookFileName struct {
	Name       string
	SeriesName string
	// Index is the number in the series, or the chapter of a comic;
	// ParentIndex is the volume of a comic.
	Index       *int
	ParentIndex *int
	Year        *int
}

// ParseBookFileName parses a book file name without extension, such as
// "Sherlock Holmes #2 (1890)" or "Captain Marvel Adventures v01 c120".
func ParseBookFileName(name string) BookFileName {
	var r BookFileName
	for _, re := range bookNames {
		m := re.match(name)
		if m == nil {
			continue
		}
		if s, ok := m.group("name"); ok {
			if cm := comicName.match(strings.TrimSpace(s)); cm != nil {
				if v, ok := cm.group("volume"); ok {
					if n, ok := parseInt(v); ok {
						r.ParentIndex = &n
					}
				}
				if v, ok := cm.group("chapter"); ok {
					if n, ok := parseInt(v); ok {
						r.Index = &n
					}
				}
			}
			r.Name = strings.TrimSpace(s)
		}
		if s, ok := m.group("index"); ok {
			if n, ok := parseInt(s); ok {
				r.Index = &n
			}
		}
		if s, ok := m.group("year"); ok {
			if n, ok := parseInt(s); ok {
				r.Year = &n
			}
		}
		if s, ok := m.group("seriesName"); ok {
			r.SeriesName = strings.TrimSpace(s)
		}
		break
	}
	return r
}

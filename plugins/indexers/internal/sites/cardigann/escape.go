package cardigann

import (
	"bytes"
	"regexp"
)

// blockIndicator ends a line whose value is a literal or folded block
// scalar, such as "key: |" or "- >-".
var blockIndicator = regexp.MustCompile(`(^|[\s:-])[|>][-+0-9]*$`)

// solidusEscapes returns a YAML document with the escape \/ of its double
// quoted scalars written as /. YAML 1.2 has the escape, for JSON, and
// Prowlarr's YamlDotNet reads it, but go-yaml knows YAML 1.1's escapes
// alone; elsewhere \/ is two characters, and is kept.
func solidusEscapes(data []byte) []byte {
	if !bytes.Contains(data, []byte(`\/`)) {
		return data
	}
	out := make([]byte, 0, len(data))
	var (
		quote byte // the quote of the scalar the scanner is in, 0 for none
		block = -1 // the indent of the line opening a block scalar, -1 for none
	)
	for line := range bytes.Lines(data) {
		indent := len(line) - len(bytes.TrimLeft(line, " "))
		if block >= 0 {
			// A block scalar's lines are its own, blank or more indented.
			if len(bytes.TrimSpace(line)) == 0 || indent > block {
				out = append(out, line...)
				continue
			}
			block = -1
		}
		var value []byte // the line outside quotes and comments
		for i := 0; i < len(line); i++ {
			c := line[i]
			switch {
			case quote == '"' && c == '\\' && i+1 < len(line):
				if line[i+1] == '/' {
					out = append(out, '/')
				} else {
					out = append(out, c, line[i+1])
				}
				i++
				continue
			case quote == '"' && c == '"':
				quote = 0
			case quote == '\'' && c == '\'':
				if i+1 < len(line) && line[i+1] == '\'' {
					out = append(out, c, c)
					i++
					continue
				}
				quote = 0
			case quote != 0:
			case c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
				// A comment runs to the end of the line.
				out = append(out, line[i:]...)
				i = len(line)
				continue
			case (c == '"' || c == '\'') && startsScalar(value):
				quote = c
			default:
				value = append(value, c)
			}
			out = append(out, c)
		}
		if quote == 0 && blockIndicator.Match(bytes.TrimSpace(value)) {
			block = indent
		}
	}
	return out
}

// startsScalar reports whether a quote after the text before it on its
// line opens a quoted scalar: at the start of a value, a key, an item of
// a sequence or of a flow collection. The indicators ":", "-" and "?" are
// followed by a space; "-" and "?" also start the line or follow one.
func startsScalar(before []byte) bool {
	t := bytes.TrimRight(before, " \t")
	if len(t) == 0 {
		return true
	}
	switch c, spaced := t[len(t)-1], len(t) < len(before); c {
	case '[', '{', ',':
		return true
	case ':':
		return spaced
	case '-', '?':
		return spaced && (len(t) == 1 || t[len(t)-2] == ' ' || t[len(t)-2] == '\t')
	}
	return false
}

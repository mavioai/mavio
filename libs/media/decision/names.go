package decision

import "strings"

// Names is a list of containers, codecs or languages as clients declare
// them: comma-separated and case-insensitive. The empty list matches
// anything; a leading "-" turns the list into "anything but these".
type Names string

// Contains reports whether any of the comma-separated input names is in the
// list, or, for a "-" list, whether none is. An empty input is only
// contained in a "-" list.
func (n Names) Contains(input string) bool {
	list, negative := strings.CutPrefix(string(n), "-")
	return containsAny(list, negative, input)
}

// containsSpan is Contains except that an empty input is matched by an empty
// list, as Jellyfin's span overloads do.
func (n Names) containsSpan(input string) bool {
	list, negative := strings.CutPrefix(string(n), "-")
	if input == "" && list == "" {
		return true
	}
	return containsAny(list, negative, input)
}

// List returns the names, without empty entries and the "-" prefix.
func (n Names) List() []string {
	return split(strings.TrimPrefix(string(n), "-"))
}

func containsAny(list string, negative bool, input string) bool {
	if input == "" {
		return negative
	}
	if list == "" {
		return true
	}
	for in := range strings.SplitSeq(input, ",") {
		if in == "" {
			continue
		}
		for name := range strings.SplitSeq(list, ",") {
			if name != "" && strings.EqualFold(in, name) {
				return !negative
			}
		}
	}
	return negative
}

// containsIn reports whether any name of the comma-separated input is in
// list, ignoring case.
func containsIn(list []string, input string) bool { return listContains(list, false, input) }

// listContains is containsIn, inverted for a negative list.
func listContains(list []string, negative bool, input string) bool {
	for _, in := range split(input) {
		for _, name := range list {
			if strings.EqualFold(in, name) {
				return !negative
			}
		}
	}
	return negative
}

// split splits a comma-separated list, dropping empty entries.
func split(s string) []string {
	var out []string
	for f := range strings.SplitSeq(s, ",") {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

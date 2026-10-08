package decision

import "testing"

func TestContainerHelperCases(t *testing.T) {
	str := func(t *testing.T, a args, name string) string {
		if a.null(name) {
			return ""
		}
		return a.str(t, name)
	}
	portedCases(t, "container_helper.json", ported{
		run: map[string]func(t *testing.T, a args){
			"ContainsContainer_EmptyContainerProfile_ReturnsTrue": func(t *testing.T, a args) {
				var p ContainerProfile
				if !p.containsContainer(str(t, a, "containers")) {
					t.Error("got = false, want = true")
				}
			},
			"ContainsContainer_InList_ReturnsTrue": func(t *testing.T, a args) {
				if !Names(a.str(t, "container")).Contains(str(t, a, "extension")) {
					t.Error("got = false, want = true")
				}
			},
			"ContainsContainer_NotInList_ReturnsFalse": func(t *testing.T, a args) {
				n, ext := Names(a.str(t, "container")), str(t, a, "extension")
				if n.Contains(ext) {
					t.Error("got = true, want = false")
				}
				if !a.null("extension") && n.containsSpan(ext) {
					t.Error("span: got = true, want = false")
				}
			},
			"ContainsContainer_InList_ReturnsTrue_SpanVersion": func(t *testing.T, a args) {
				if !Names(a.str(t, "container")).containsSpan(str(t, a, "extension")) {
					t.Error("got = false, want = true")
				}
			},
			"ContainsContainer_ThreeArgs_InList_ReturnsTrue": func(t *testing.T, a args) {
				if !listContains(a.strs(t, "containers"), a.boolean(t, "isNegativeList"), a.str(t, "inputContainer")) {
					t.Error("got = false, want = true")
				}
			},
			"ContainsContainer_ThreeArgs_InList_ReturnsFalse": func(t *testing.T, a args) {
				if listContains(a.strs(t, "containers"), a.boolean(t, "isNegativeList"), a.str(t, "inputContainer")) {
					t.Error("got = true, want = false")
				}
			},
		},
	})
}

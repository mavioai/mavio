package library

import (
	"slices"
	"testing"
)

type namedProvider string

func (p namedProvider) Name() string { return string(p) }

func TestOrdered(t *testing.T) {
	all := []namedProvider{"a", "b", "c"}
	tests := []struct {
		name  string
		order []string
		want  []namedProvider
	}{
		{"unset", nil, all},
		{"none", []string{}, []namedProvider{}},
		{"reordered", []string{"c", "a"}, []namedProvider{"c", "a"}},
		{"unknown", []string{"x", "b"}, []namedProvider{"b"}},
	}
	for _, tt := range tests {
		if got := ordered(all, tt.order); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

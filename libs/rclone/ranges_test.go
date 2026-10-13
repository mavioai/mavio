package rclone

import (
	"fmt"
	"testing"
)

func TestRanges(t *testing.T) {
	for _, tt := range []struct {
		name  string
		adds  [][2]int64
		want  string
		added int64 // by the last add
	}{
		{"one", [][2]int64{{0, 10}}, "[{0 10}]", 10},
		{"empty", [][2]int64{{5, 5}}, "[]", 0},
		{"apart", [][2]int64{{20, 30}, {0, 10}}, "[{0 10} {20 30}]", 10},
		{"touching", [][2]int64{{0, 10}, {10, 20}}, "[{0 20}]", 10},
		{"overlapping", [][2]int64{{0, 10}, {5, 15}}, "[{0 15}]", 5},
		{"bridging", [][2]int64{{0, 10}, {20, 30}, {40, 50}, {5, 45}}, "[{0 50}]", 20},
		{"inside", [][2]int64{{0, 100}, {10, 20}}, "[{0 100}]", 0},
		{"covering", [][2]int64{{10, 20}, {30, 40}, {0, 50}}, "[{0 50}]", 30},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var r ranges
			var added int64
			for _, a := range tt.adds {
				added = r.add(a[0], a[1])
			}
			if got := fmt.Sprint([]span(r)); got != tt.want || added != tt.added {
				t.Errorf("ranges = %s, added %d; want = %s, %d", got, added, tt.want, tt.added)
			}
		})
	}
}

func TestRangesMissing(t *testing.T) {
	r := ranges{{10, 20}, {30, 40}}
	for _, tt := range []struct {
		start, end int64
		want       span
		ok         bool
	}{
		{0, 5, span{0, 5}, true},
		{0, 15, span{0, 10}, true},
		{10, 20, span{}, false},
		{12, 35, span{20, 30}, true},
		{30, 40, span{}, false},
		{35, 50, span{40, 50}, true},
		{50, 60, span{50, 60}, true},
	} {
		got, ok := r.missing(tt.start, tt.end)
		if got != tt.want || ok != tt.ok {
			t.Errorf("missing(%d, %d) = %v, %t; want = %v, %t", tt.start, tt.end, got, ok, tt.want, tt.ok)
		}
	}
	if r.size() != 20 || !r.present(32, 38) || r.present(15, 32) {
		t.Errorf("size() = %d, present(32, 38) = %t, present(15, 32) = %t", r.size(), r.present(32, 38), r.present(15, 32))
	}
}

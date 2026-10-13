package rclone

import "slices"

// span is the bytes [Start, End) of a file.
type span struct {
	Start, End int64
}

// ranges is a set of byte spans, sorted, coalesced and not empty.
type ranges []span

// add adds [start, end) and returns how many bytes were not present.
func (r *ranges) add(start, end int64) int64 {
	if start >= end {
		return 0
	}
	rs := *r
	// The spans that touch or overlap [start, end) merge with it.
	i, _ := slices.BinarySearchFunc(rs, start, func(s span, v int64) int {
		switch {
		case s.End < v:
			return -1
		case s.Start > v:
			return 1
		}
		return 0
	})
	j := i
	added := end - start
	for j < len(rs) && rs[j].Start <= end {
		added -= overlap(rs[j], start, end)
		start, end = min(start, rs[j].Start), max(end, rs[j].End)
		j++
	}
	*r = slices.Replace(rs, i, j, span{start, end})
	return added
}

// overlap returns how many bytes of [start, end) s holds.
func overlap(s span, start, end int64) int64 {
	return max(0, min(s.End, end)-max(s.Start, start))
}

// missing returns the first span of [start, end) not present; false when
// all of it is.
func (r ranges) missing(start, end int64) (span, bool) {
	for _, s := range r {
		if s.End <= start {
			continue
		}
		if s.Start > start {
			return span{start, min(end, s.Start)}, start < end
		}
		start = s.End
		if start >= end {
			return span{}, false
		}
	}
	return span{start, end}, start < end
}

// present reports whether all of [start, end) is present.
func (r ranges) present(start, end int64) bool {
	_, ok := r.missing(start, end)
	return !ok
}

// size returns the bytes present.
func (r ranges) size() int64 {
	var n int64
	for _, s := range r {
		n += s.End - s.Start
	}
	return n
}

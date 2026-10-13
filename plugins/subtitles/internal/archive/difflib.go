package archive

// ratio is Python's difflib.SequenceMatcher(None, a, b).ratio(): twice
// the matched characters over the characters of both, matched as
// Ratcliff and Obershelp's algorithm with difflib's automatic junk.
func ratio(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra)+len(rb) == 0 {
		return 1
	}
	m := newMatcher(ra, rb)
	matched := 0
	for _, blk := range m.matchingBlocks() {
		matched += blk.size
	}
	return 2 * float64(matched) / float64(len(ra)+len(rb))
}

type block struct{ i, j, size int }

type matcher struct {
	a, b []rune
	b2j  map[rune][]int
}

func newMatcher(a, b []rune) *matcher {
	m := &matcher{a: a, b: b, b2j: map[rune][]int{}}
	for j, r := range b {
		m.b2j[r] = append(m.b2j[r], j)
	}
	// difflib's autojunk: in sequences of 200 or more, elements making
	// up more than 1% of them are popular and not matched from.
	if n := len(b); n >= 200 {
		limit := n/100 + 1
		for r, js := range m.b2j {
			if len(js) > limit {
				delete(m.b2j, r)
			}
		}
	}
	return m
}

// longest is difflib's find_longest_match without junk elements.
func (m *matcher) longest(alo, ahi, blo, bhi int) block {
	besti, bestj, bestsize := alo, blo, 0
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		next := map[int]int{}
		for _, j := range m.b2j[m.a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			next[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = next
	}
	// Extend the match with the elements autojunk left out.
	for besti > alo && bestj > blo && m.a[besti-1] == m.b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.a[besti+bestsize] == m.b[bestj+bestsize] {
		bestsize++
	}
	return block{besti, bestj, bestsize}
}

func (m *matcher) matchingBlocks() []block {
	var out []block
	queue := [][4]int{{0, len(m.a), 0, len(m.b)}}
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		alo, ahi, blo, bhi := q[0], q[1], q[2], q[3]
		x := m.longest(alo, ahi, blo, bhi)
		if x.size == 0 {
			continue
		}
		out = append(out, x)
		if alo < x.i && blo < x.j {
			queue = append(queue, [4]int{alo, x.i, blo, x.j})
		}
		if x.i+x.size < ahi && x.j+x.size < bhi {
			queue = append(queue, [4]int{x.i + x.size, ahi, x.j + x.size, bhi})
		}
	}
	return out
}

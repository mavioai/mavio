package subsync

import (
	"math"
	"math/bits"
	"math/cmplx"
)

// fft transforms x in place, whose length is a power of two; inverse runs
// the unscaled inverse transform.
func fft(x []complex128, inverse bool) {
	n := len(x)
	shift := 64 - bits.TrailingZeros(uint(n))
	for i := range x {
		if j := int(bits.Reverse64(uint64(i)) >> shift); i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	sign := -1.0
	if inverse {
		sign = 1
	}
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		step := cmplx.Rect(1, sign*2*math.Pi/float64(size))
		// Twiddles of this size, computed by halves to bound the error.
		tw := make([]complex128, half)
		for k := range tw {
			if k%64 == 0 {
				tw[k] = cmplx.Rect(1, sign*2*math.Pi*float64(k)/float64(size))
			} else {
				tw[k] = tw[k-1] * step
			}
		}
		for start := 0; start < n; start += size {
			a, b := x[start:start+half], x[start+half:start+size]
			for k := range a {
				t := tw[k] * b[k]
				a[k], b[k] = a[k]+t, a[k]-t
			}
		}
	}
}

// correlation holds the cross-correlation of a reference with a subtitle,
// both mapped from [0, 1] to [-1, 1] and zero outside them, by offset of
// the subtitle.
type correlation struct {
	c        []complex128
	ref, sub int
}

// correlate computes the correlation of ref and sub with one forward and
// one inverse transform: both go in one complex signal, ref in the real part
// and sub in the imaginary one.
func correlate(ref, sub Speech) correlation {
	n := 1 << bits.Len(uint(len(ref)+len(sub)-1))
	z := make([]complex128, n)
	for i, v := range ref {
		z[i] = complex(2*float64(v)-1, 0)
	}
	for i, v := range sub {
		z[i] += complex(0, 2*float64(v)-1)
	}
	fft(z, false)
	// R = (Z[k] + conj(Z[-k]))/2, S = (Z[k] - conj(Z[-k]))/2i, and the
	// correlation's transform is R × conj(S); pairs k and n-k are done
	// together since each needs the other's input.
	for k := 0; k <= n/2; k++ {
		zk, zm := z[k], z[(n-k)%n]
		r := (zk + cmplx.Conj(zm)) / 2
		s := (zk - cmplx.Conj(zm)) / complex(0, 2)
		rm := (zm + cmplx.Conj(zk)) / 2
		sm := (zm - cmplx.Conj(zk)) / complex(0, 2)
		z[k], z[(n-k)%n] = r*cmplx.Conj(s), rm*cmplx.Conj(sm)
	}
	fft(z, true)
	return correlation{c: z, ref: len(ref), sub: len(sub)}
}

// at returns the correlation when the subtitle is shifted by offset
// samples: the sum over t of ref[t] × sub[t-offset].
func (c correlation) at(offset int) float64 {
	n := len(c.c)
	return real(c.c[(offset%n+n)%n]) / float64(n)
}

// best returns the offset with the highest correlation within
// (-maxOffset, maxOffset] (any offset when maxOffset is negative), the
// largest offset among equals as ffsubsync's argmax picks.
func (c correlation) best(maxOffset int) (offset int, score float64, ok bool) {
	lo, hi := -(c.sub - 1), c.ref-1
	if maxOffset >= 0 {
		lo, hi = max(lo, -maxOffset+1), min(hi, maxOffset)
	}
	score = math.Inf(-1)
	for o := hi; o >= lo; o-- {
		if s := c.at(o); s > score {
			offset, score, ok = o, s, true
		}
	}
	return offset, score, ok
}

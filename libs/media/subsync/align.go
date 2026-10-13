package subsync

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mavioai/mavio/libs/subtitle"
)

// ErrNoAlignment is returned when no alignment lies within the options'
// bounds, or a side has no speech.
var ErrNoAlignment = errors.New("subsync: no alignment found")

// framerateRatios are the ratios between 23.976, 24 and 25 frames a second
// tried when fixing frame rates, each also inverted.
var framerateRatios = [...]float64{24 / 23.976, 25 / 23.976, 25 / 24.0}

// The range and tolerance of the golden-section search of ratios.
const (
	minRatio     = 0.9
	maxRatio     = 1.1
	gssTolerance = 1e-4
)

// Options says which alignments Align tries.
type Options struct {
	// MaxOffset bounds the offset; zero leaves it unbounded. Bazarr's
	// default is a minute.
	MaxOffset time.Duration
	// FixFramerate also tries the subtitle scaled by the ratios between
	// 23.976, 24 and 25 frames a second.
	FixFramerate bool
	// GoldenSection, with FixFramerate, also tries the ratio from 0.9 to
	// 1.1 a golden-section search finds.
	GoldenSection bool
	// InferFramerate also tries the ratio of the reference's speech span to
	// the subtitle's, which ffsubsync does when the reference is a
	// subtitle, whose speech has clear ends.
	InferFramerate bool
}

// Result is an alignment: the subtitle's times are multiplied by Scale,
// then shifted by Offset.
type Result struct {
	Offset time.Duration
	Scale  float64
	// Score is the correlation of the aligned speech, the number of
	// samples agreeing less those disagreeing; below zero the subtitle
	// matches the reference worse than its opposite would.
	Score float64
}

// Align returns the best alignment of cues to the reference's speech.
func Align(ctx context.Context, ref Speech, cues []subtitle.Cue, opt Options) (Result, error) {
	if len(ref) == 0 || len(cues) == 0 {
		return Result{}, ErrNoAlignment
	}
	maxOffset := -1
	if opt.MaxOffset > 0 {
		maxOffset = int(opt.MaxOffset.Seconds() * SampleRate)
	}
	var (
		best  Result
		found bool
	)
	// try scores the subtitle at ratio, keeping it when it is the best yet
	// and keep is set; ties keep the earlier, as ffsubsync's max.
	try := func(ratio float64, keep bool) (float64, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		offset, score, ok := correlate(ref, FromCues(cues, ratio)).best(maxOffset)
		if !ok {
			return math.Inf(-1), nil
		}
		if keep && (!found || score > best.Score) {
			best = Result{Offset: time.Duration(offset) * time.Second / SampleRate, Scale: ratio, Score: score}
			found = true
		}
		return score, nil
	}
	ratios := []float64{1}
	if opt.FixFramerate {
		ratios = append(ratios, framerateRatios[:]...)
		for _, r := range framerateRatios {
			ratios = append(ratios, 1/r)
		}
	}
	for _, r := range ratios {
		if _, err := try(r, true); err != nil {
			return Result{}, err
		}
	}
	if opt.FixFramerate && opt.GoldenSection {
		if err := goldenSection(try); err != nil {
			return Result{}, err
		}
	}
	if opt.InferFramerate {
		refSpan, ok1 := ref.span()
		subSpan, ok2 := FromCues(cues, 1).span()
		if ok1 && ok2 && subSpan > 0 {
			if _, err := try(float64(refSpan)/float64(subSpan), true); err != nil {
				return Result{}, err
			}
		}
	}
	if !found {
		return Result{}, ErrNoAlignment
	}
	return best, nil
}

// goldenSection searches the ratio with the highest score, as ffsubsync's
// gss: only the score of the last ratio it evaluates is kept.
func goldenSection(score func(ratio float64, keep bool) (float64, error)) error {
	invphi := (math.Sqrt(5) - 1) / 2
	invphi2 := (3 - math.Sqrt(5)) / 2
	a, b := minRatio, maxRatio
	h := b - a
	n := int(math.Ceil(math.Log(gssTolerance/h) / math.Log(invphi)))
	f := func(x float64, last bool) (float64, error) {
		s, err := score(x, last)
		return -s, err
	}
	c, d := a+invphi2*h, a+invphi*h
	yc, err := f(c, n == 1)
	if err != nil {
		return err
	}
	yd, err := f(d, n == 1)
	if err != nil {
		return err
	}
	for k := range n - 1 {
		last := k == n-2
		if yc < yd {
			d, yd = c, yc
			h *= invphi
			c = a + invphi2*h
			if yc, err = f(c, last); err != nil {
				return err
			}
		} else {
			a, c, yc = c, d, yd
			h *= invphi
			d = a + invphi*h
			if yd, err = f(d, last); err != nil {
				return err
			}
		}
	}
	return nil
}

// Quality bounds the alignments worth applying; a zero bound is not
// checked except MinScore.
type Quality struct {
	MinScore          float64
	MaxOffset         time.Duration
	MaxScaleDeviation float64
}

// DefaultQuality is Bazarr's: a score of at least zero, at most half a
// minute of offset and a tenth of scale.
var DefaultQuality = Quality{MinScore: 0, MaxOffset: 30 * time.Second, MaxScaleDeviation: 0.1}

// Problems returns why r looks wrong by q, as ffsubsync's
// --skip-sync-on-low-quality; none when it may be applied.
func (r Result) Problems(q Quality) []string {
	var out []string
	if r.Score < q.MinScore {
		out = append(out, fmt.Sprintf("score %.1f < %.1f", r.Score, q.MinScore))
	}
	if q.MaxOffset > 0 && r.Offset.Abs() > q.MaxOffset {
		out = append(out, fmt.Sprintf("|offset| %.1fs > %.1fs", r.Offset.Abs().Seconds(), q.MaxOffset.Seconds()))
	}
	if dev := math.Abs(r.Scale - 1); q.MaxScaleDeviation > 0 && dev > q.MaxScaleDeviation {
		out = append(out, fmt.Sprintf("framerate deviation %.3f > %.3f", dev, q.MaxScaleDeviation))
	}
	return out
}

// Apply returns cues retimed by r, times clamped at zero.
func (r Result) Apply(cues []subtitle.Cue) []subtitle.Cue {
	out := make([]subtitle.Cue, len(cues))
	at := func(d time.Duration) time.Duration {
		us := math.RoundToEven(float64(d.Microseconds()) * r.Scale)
		return max(time.Duration(us)*time.Microsecond+r.Offset, 0)
	}
	for i, c := range cues {
		c.Start, c.End = at(c.Start), at(c.End)
		out[i] = c
	}
	return out
}

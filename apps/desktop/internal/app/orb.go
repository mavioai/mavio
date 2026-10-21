package app

import (
	_ "embed"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// orbMode is a state of the orb, from the voice session's
// (libs/design/components.md, Assistant).
type orbMode int

const (
	orbOpening orbMode = iota
	orbIdle
	orbHearing
	orbPausing
	orbThinking
	orbSpeaking
	orbResting
	orbError
	orbEnded
)

// orbLook is how the orb looks in a state: its size, how far its lights
// orbit apart, how fast, how bright, whether it is a ring, and how much
// the audio's level swells, spreads, speeds and brightens it.
type orbLook struct {
	size, spread, speed, bright, ring, swing float64
}

var orbLooks = [...]orbLook{
	orbOpening:  {size: 0.55, spread: 0.12, speed: 0.35, bright: 0.45},
	orbIdle:     {size: 0.78, spread: 0.2, speed: 0.6, bright: 0.85, swing: 0.15},
	orbHearing:  {size: 0.82, spread: 0.24, speed: 1.3, bright: 1, swing: 1},
	orbPausing:  {size: 0.7, spread: 0.16, speed: 0.5, bright: 0.8},
	orbThinking: {size: 0.62, spread: 0.4, speed: 3.4, bright: 0.95, ring: 1},
	orbSpeaking: {size: 0.8, spread: 0.2, speed: 1.2, bright: 1, swing: 0.55},
	orbResting:  {size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85},
	orbError:    {size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85},
	orbEnded:    {size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85},
}

// orbSize is the orb's size, in DIPs; it is drawn half again as large,
// centered over its box, for its swell and glow.
const (
	orbSize  = 84
	orbBleed = 1.5
)

// The lights, magenta, cyan and blue: their colors on a dark page and on
// a light one, how fast they orbit and where they start.
var orbLights = [3]struct {
	dark, light [3]float64
	speed, at   float64
}{
	{[3]float64{255, 70, 190}, [3]float64{235, 40, 160}, 0.9, 0},
	{[3]float64{40, 220, 255}, [3]float64{0, 170, 235}, -1.15, 2.1},
	{[3]float64{100, 90, 255}, [3]float64{90, 70, 245}, 0.7, 4.2},
}

var (
	//go:embed orb.kage
	orbSource string
	orbShader = shader.MustCompile("package main\n" + orbSource)
)

// orbState is the orb's motion, eased toward its state's look frame by
// frame.
type orbState struct {
	started            bool
	cur                orbLook
	lv, phase          float64
	last, since, flash time.Time
	quiet              time.Time
	mode               orbMode
	interrupts         int
}

// orb builds the orb in mode; level reads the audio's level and
// interrupts counts the session's interrupts, each flashing it.
func orb(c *ui.Context, o *orbState, mode orbMode, level func() float32, interrupts int) {
	still := c.Preferences().ReduceMotion
	light := !c.Theme().Dark
	ui.Box(c.Key("orb")).Size(orbSize, orbSize).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		now := p.Now()
		if !o.started {
			o.started = true
			o.cur = orbLooks[orbOpening]
			o.last, o.since, o.quiet, o.mode, o.interrupts = now, now, now, mode, interrupts
			o.flash = time.Time{}
		}
		if mode != o.mode {
			o.mode, o.since = mode, now
		}
		if interrupts != o.interrupts {
			o.interrupts, o.flash = interrupts, now
		}
		dt := min(0.05, now.Sub(o.last).Seconds())
		o.last = now
		t := now.Sub(o.since).Seconds()
		motion := !still
		raw := 0.0
		if motion && (mode == orbHearing || mode == orbSpeaking || mode == orbIdle) {
			raw = float64(level())
		}
		// The user falls silent but the turn goes on: the orb draws in.
		if raw > 0.25 {
			o.quiet = now
		}
		look := orbLooks[mode]
		if mode == orbHearing && now.Sub(o.quiet) > 400*time.Millisecond {
			look = orbLooks[orbPausing]
		}
		k := 1.0
		if motion {
			k = 1 - math.Exp(-dt*6)
		}
		o.cur.size += (look.size - o.cur.size) * k
		o.cur.spread += (look.spread - o.cur.spread) * k
		o.cur.speed += (look.speed - o.cur.speed) * k
		o.cur.bright += (look.bright - o.cur.bright) * k
		o.cur.ring += (look.ring - o.cur.ring) * k
		o.cur.swing += (look.swing - o.cur.swing) * k
		// A syllable swells it at once; it settles back more slowly.
		target := raw * o.cur.swing
		rate := 8.0
		if target > o.lv {
			rate = 22
		}
		o.lv += (target - o.lv) * (1 - math.Exp(-dt*rate))
		flash := 0.0
		if motion {
			o.phase += dt * (o.cur.speed + o.lv*2.6)
			if !o.flash.IsZero() {
				flash = max(0, 1-now.Sub(o.flash).Seconds()/0.45)
			}
		}

		W := float64(orbSize * orbBleed)
		R := W / 2
		base := R / orbBleed * 0.58
		// An error shakes the orb once and flushes it red.
		shake := 0.0
		if motion && mode == orbError && t < 0.6 {
			shake = math.Sin(t*40) * 6 * (1 - t/0.6)
		}
		red := 0.0
		if mode == orbError {
			red = max(0, 1-t/1.4)
		}
		shrink := 1 - flash*0.35
		edge := base * (o.cur.size + o.lv*0.9) * shrink * 0.92
		box := ui.Rect{X: r.X + (r.W-float32(W))/2 + float32(shake), Y: r.Y + (r.H-float32(W))/2, W: float32(W), H: float32(W)}
		u := map[string]any{
			"Center": [2]float32{float32(R), float32(R)},
			"Edge":   float32(edge),
			"Bright": float32(o.cur.bright + o.lv*0.1),
			"Glow":   float32(0.3 + o.lv*0.2),
			"Flash":  float32(flash),
			"Base":   float32(base),
		}
		if light {
			u["Light"] = float32(1)
			u["Body"] = ui.RGBA(90, 70, 245, float32(0.55*o.cur.bright))
		} else {
			u["Body"] = ui.RGBA(18, 14, 58, float32(0.85*o.cur.bright))
		}
		if o.cur.ring > 0.02 {
			u["Hole"] = float32(edge * 0.5 * o.cur.ring)
		}
		for i, l := range orbLights {
			a := o.phase*l.speed*2 + l.at
			// Each light pulses on its own beat with the level, so that the
			// colors surge against each other as the user speaks.
			pulse := 1 + o.lv*0.35*math.Sin(o.phase*5.3+float64(i)*2.1)
			orbit := base * (o.cur.spread*(1-o.cur.ring) + o.cur.ring*0.55 + o.lv*0.4*pulse) * shrink
			x := R + math.Cos(a)*orbit
			y := R + math.Sin(a*(1+0.3*(1-o.cur.ring)))*orbit
			rad := base * (o.cur.size + o.lv*0.9*pulse) * shrink
			col := l.dark
			if light {
				col = l.light
			}
			var c [3]float32
			for j, v := range col {
				c[j] = float32((v + ([3]float64{255, 70, 80}[j]-v)*red) / 255)
			}
			n := string(rune('0' + i))
			u["B"+n] = [3]float32{float32(x), float32(y), float32(rad)}
			u["C"+n] = c
		}
		orbShader.Draw(p, box, shader.Options{Uniforms: u})
		if motion {
			p.AnimationFrame()
		}
	})
}

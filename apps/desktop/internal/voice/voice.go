package voice

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Rate is the rate of the microphone's PCM as sessions take it.
const Rate = 16000

// FrameSamples are the samples of one of the microphone's 20 ms frames.
const FrameSamples = Rate / 50

// outputRate is the rate speech is played at, converted to from the
// session's, unless the device plays at its own.
const outputRate = 48000

// ErrDenied tells that the user refused the app the microphone.
var ErrDenied = errors.New("voice: the microphone is not allowed")

// Microphone is an input device, by its ID, which stays the same across
// restarts, and its name.
type Microphone struct {
	ID, Name string
}

// Audio opens the audio of voice sessions.
type Audio interface {
	// Microphones lists the inputs.
	Microphones() ([]Microphone, error)
	// Open captures the microphone with id, the system's default input
	// when empty or gone, giving each 20 ms frame to frame from the audio
	// thread, and plays speech. It asks the user for the microphone the
	// first time, which it waits for.
	Open(id string, frame func([]int16)) (Device, error)
}

// Device is a voice session's audio, open until Close.
type Device interface {
	// Microphone names the microphone captured.
	Microphone() string
	// Play plays 16-bit mono PCM at rate after what is queued.
	Play(pcm []int16, rate int)
	// Run starts counting what plays from here, as an answer starts.
	Run()
	// Progress tells how much of the answer's speech has played and has
	// come.
	Progress() (played, total time.Duration)
	// Drop drops what was not played, returning how much of the answer's
	// speech was.
	Drop() time.Duration
	// InputLevel and OutputLevel tell how loud the microphone and the
	// speech are now, from 0 to about 1.
	InputLevel() float32
	OutputLevel() float32
	// Close stops the audio.
	Close()
}

// meter keeps the loudness of the audio last seen.
type meter struct{ bits atomic.Uint32 }

// measure takes the loudness of samples: speech's RMS spans roughly 0.01
// to 0.3, whose square root spreads it out.
func (m *meter) measure(samples []float32) {
	if len(samples) == 0 {
		return
	}
	var sum float64
	for _, v := range samples {
		sum += float64(v) * float64(v)
	}
	l := min(1, math.Sqrt(math.Sqrt(sum/float64(len(samples))))*1.5)
	m.bits.Store(math.Float32bits(float32(l)))
}

func (m *meter) level() float32 { return math.Float32frombits(m.bits.Load()) }

// capture converts the microphone's float samples at its rate to 16 kHz
// 16-bit frames, averaging the input over each output sample's span,
// which keeps what is above 8 kHz from folding into speech.
type capture struct {
	ratio    float64
	acc, sum float64
	count    int
	out      []int16
	frame    func([]int16)
	meter    meter
}

func newCapture(rate float64, frame func([]int16)) *capture {
	return &capture{ratio: rate / Rate, out: make([]int16, 0, FrameSamples), frame: frame}
}

// write takes samples of the microphone's.
func (c *capture) write(in []float32) {
	c.meter.measure(in)
	for _, v := range in {
		c.sum += float64(v)
		c.count++
		c.acc++
		if c.acc < c.ratio {
			continue
		}
		c.acc -= c.ratio
		s := max(-1, min(1, c.sum/float64(c.count)))
		c.sum, c.count = 0, 0
		if s < 0 {
			c.out = append(c.out, int16(s*0x8000))
		} else {
			c.out = append(c.out, int16(s*0x7fff))
		}
		if len(c.out) == FrameSamples {
			c.frame(c.out)
			c.out = make([]int16, 0, FrameSamples)
		}
	}
}

// playback holds the speech to play at its rate, converting it from the
// session's rate, and counts what of the answer has played.
type playback struct {
	// rate is the rate it plays at, outputRate unless set.
	rate  int
	mu    sync.Mutex
	queue []float32
	// played and total are the samples of the answer played and come.
	played, total int
	// pos is where the next output sample falls in the input, after prev,
	// the input's last sample.
	pos   float64
	prev  float32
	meter meter
}

func (p *playback) push(pcm []int16, rate int) {
	if len(pcm) == 0 || rate <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.queue)
	if rate == p.outRate() {
		for _, s := range pcm {
			p.queue = append(p.queue, float32(s)/0x8000)
		}
	} else {
		if p.pos == 0 {
			p.pos = 1
		}
		step := float64(rate) / float64(p.outRate())
		at := func(i int) float32 {
			if i == 0 {
				return p.prev
			}
			return float32(pcm[min(i, len(pcm))-1]) / 0x8000
		}
		last := float64(len(pcm))
		for ; p.pos <= last; p.pos += step {
			i := int(p.pos)
			f := float32(p.pos - float64(i))
			p.queue = append(p.queue, at(i)*(1-f)+at(i+1)*f)
		}
		p.pos -= last
		p.prev = float32(pcm[len(pcm)-1]) / 0x8000
	}
	p.total += len(p.queue) - n
}

// read fills out with what plays next, silence once nothing is queued.
func (p *playback) read(out []float32) {
	p.mu.Lock()
	n := copy(out, p.queue)
	p.queue = p.queue[n:]
	p.played += n
	if len(p.queue) == 0 {
		p.queue = nil
	}
	p.mu.Unlock()
	clear(out[n:])
	p.meter.measure(out)
}

func (p *playback) run() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.played, p.total = 0, len(p.queue)
}

func (p *playback) progress() (played, total time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.samples(p.played), p.samples(p.total)
}

func (p *playback) drop() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	played := p.samples(p.played)
	p.queue, p.played, p.total, p.pos, p.prev = nil, 0, 0, 0, 0
	return played
}

func (p *playback) outRate() int {
	if p.rate > 0 {
		return p.rate
	}
	return outputRate
}

// samples is how long n samples at the playback's rate play.
func (p *playback) samples(n int) time.Duration {
	return time.Duration(n) * time.Second / time.Duration(p.outRate())
}

package app

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"

	"github.com/mavioai/mavio/apps/desktop/internal/voice"
)

// fakeAudio is the platform's audio: its microphones, of which it opens
// one as a fakeDevice, or fails with err.
type fakeAudio struct {
	err error

	mu     sync.Mutex
	opened []string
	dev    *fakeDevice
}

func (a *fakeAudio) Microphones() ([]voice.Microphone, error) {
	return []voice.Microphone{{ID: "usb", Name: "USB Mic"}}, nil
}

func (a *fakeAudio) Open(id string, frame func([]int16)) (voice.Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opened = append(a.opened, id)
	if a.err != nil {
		return nil, a.err
	}
	a.dev = &fakeDevice{frame: frame}
	return a.dev, nil
}

func (a *fakeAudio) device() *fakeDevice {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dev
}

// fakeDevice is an open microphone and speaker: frame takes what the test
// says, and it keeps what it was told to play.
type fakeDevice struct {
	frame func([]int16)

	mu     sync.Mutex
	played int
	runs   int
	drops  int
	closed bool
}

func (d *fakeDevice) Microphone() string { return "Fake Mic" }

func (d *fakeDevice) Play(pcm []int16, _ int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.played += len(pcm)
}

func (d *fakeDevice) Run() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.runs++
}

func (d *fakeDevice) Progress() (time.Duration, time.Duration) { return 0, time.Second }

func (d *fakeDevice) Drop() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.drops++
	return 300 * time.Millisecond
}

func (d *fakeDevice) InputLevel() float32  { return 0.5 }
func (d *fakeDevice) OutputLevel() float32 { return 0.5 }

func (d *fakeDevice) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
}

func (d *fakeDevice) state() (played, runs, drops int, closed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.played, d.runs, d.drops, d.closed
}

// say gives n frames of the microphone's, of sound or of digital silence.
func (d *fakeDevice) say(n int, sound bool) {
	for range n {
		pcm := make([]int16, voice.FrameSamples)
		if sound {
			for i := range pcm {
				pcm[i] = int16(1000 * (i%2*2 - 1))
			}
		}
		d.frame(pcm)
	}
}

// fakeVoiceServer is the server's voice session: the client's frames and
// events come in on in, and what out has is sent to it.
type fakeVoiceServer struct {
	url string
	in  chan outgoing
	out chan outgoing
}

func newFakeVoiceServer(t *testing.T) *fakeVoiceServer {
	s := &fakeVoiceServer{in: make(chan outgoing, 4096), out: make(chan outgoing, 64)}
	srv := httptest.NewServer(websocket.Server{Handler: func(ws *websocket.Conn) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				var o outgoing
				if voiceFrames.Receive(ws, &o) != nil {
					return
				}
				s.in <- o
			}
		}()
		for {
			select {
			case <-done:
				return
			case o := <-s.out:
				if voiceFrames.Send(ws, o) != nil {
					return
				}
			}
		}
	}})
	t.Cleanup(srv.Close)
	s.url = srv.URL + "/assistant/voice/token"
	return s
}

// event sends the client an event.
func (s *fakeVoiceServer) event(e voiceEvent) {
	b, _ := json.Marshal(e)
	s.out <- outgoing{typ: websocket.TextFrame, data: b}
}

// speak sends the client n samples of speech.
func (s *fakeVoiceServer) speak(n int) {
	s.out <- outgoing{typ: websocket.BinaryFrame, data: make([]byte, 2*n)}
}

// next waits for the client's next event of type typ, skipping the
// microphone's frames and other events.
func (s *fakeVoiceServer) next(t *testing.T, typ string) voiceEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	var seen []string
	for {
		select {
		case o := <-s.in:
			if o.typ != websocket.TextFrame {
				continue
			}
			var e voiceEvent
			if json.Unmarshal(o.data, &e) == nil {
				if e.str("type") == typ {
					return e
				}
				seen = append(seen, e.str("type"))
			}
		case <-deadline:
			t.Fatalf("no %q from the client; got = %q", typ, seen)
			return nil
		}
	}
}

// frames counts the microphone's frames the client sent so far.
func (s *fakeVoiceServer) frames() int {
	n := 0
	for {
		select {
		case o := <-s.in:
			if o.typ == websocket.BinaryFrame {
				n++
			}
		default:
			return n
		}
	}
}

// until runs what the goroutines send the main thread, drawing frames,
// until ok holds, failing after a few seconds.
func (h *harness) until(what string, ok func() bool) {
	h.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		h.tt.Frame()
		if ok() {
			return
		}
		select {
		case fn := <-h.updates:
			fn()
		case <-time.After(20 * time.Millisecond):
		case <-deadline:
			h.t.Fatalf("%s: timed out; texts = %q", what, h.tt.Texts())
		}
	}
}

// shows reports whether the last frame shows every text.
func (h *harness) shows(texts ...string) func() bool {
	return func() bool {
		return !slices.ContainsFunc(texts, func(s string) bool { return !h.tt.HasText(s) })
	}
}

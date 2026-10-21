package app

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"golang.org/x/net/websocket"

	"github.com/mavioai/mavio/apps/desktop/internal/voice"
)

// silentFrames is how many of the microphone's 20 ms frames of digital
// silence tell it is silent: a real microphone's noise is never all zeros
// for 2 s.
const silentFrames = 100

// pendingFrames is how much is kept while the session's WebSocket opens,
// which takes seconds while the plugin makes the session's services: 20 s
// of the microphone's frames.
const pendingFrames = 1000

// voiceHandlers tell what a session does, on the main thread.
type voiceHandlers struct {
	// event is every event the session sends.
	event func(voiceEvent)
	// closed tells that the session ended; err says why when it failed.
	closed func(err string)
	// capturing tells that the microphone's first audio other than
	// digital silence came: what is said from now on is heard.
	capturing func()
	// silent tells that the microphone gave nothing but digital silence,
	// as a muted or virtual input does; device names it.
	silent func(device string)
}

// voiceEvent is an event of the session's protocol (docs/assistant.md
// §8.6), as its JSON object.
type voiceEvent map[string]any

func (e voiceEvent) str(k string) string {
	s, _ := e[k].(string)
	return s
}

func (e voiceEvent) num(k string) float64 {
	n, _ := e[k].(float64)
	return n
}

func (e voiceEvent) flag(k string) bool {
	b, _ := e[k].(bool)
	return b
}

// outgoing is what the session sends: a frame of the microphone's, or an
// event.
type outgoing struct {
	typ  byte
	data []byte
	// last closes the socket once sent.
	last bool
}

// voiceFrames reads and writes the WebSocket's frames as they are, which
// tells the speech's binary frames from the events' text ones.
var voiceFrames = websocket.Codec{
	Marshal: func(v any) ([]byte, byte, error) {
		o := v.(outgoing)
		return o.data, o.typ, nil
	},
	Unmarshal: func(data []byte, typ byte, v any) error {
		o := v.(*outgoing)
		o.typ, o.data = typ, data
		return nil
	},
}

// voiceSession is a voice session from the client's side (docs/assistant.md
// §8, §9.5): the microphone captured with the platform's echo
// cancellation, sent in 20 ms frames of 16 kHz mono 16-bit PCM on the
// session's WebSocket, and the answer's speech played as it comes, which
// an interrupt drops. Capture starts at once; what is said before the
// WebSocket opens is kept and sent then.
type voiceSession struct {
	dev    voice.Device
	h      voiceHandlers
	update func(func())
	out    chan outgoing
	ctx    context.Context
	cancel context.CancelFunc

	// On the audio thread: whether audio came, and the frames of digital
	// silence until one was not.
	captured bool
	zeros    int

	// On the main thread.
	ended      bool
	connecting bool
	// pushToTalk: the session listens only while the user holds to talk;
	// talking is whether the user does, told what the socket was told.
	pushToTalk, talking, told bool

	// mu guards ready, which the session said: one closed before failed.
	mu    sync.Mutex
	ready bool
}

// openVoice opens the microphone of audio, the one with id or the
// system's default; the session's events go to h on the main thread,
// through update.
func openVoice(ctx context.Context, audio voice.Audio, id string, update func(func()), h voiceHandlers) (*voiceSession, error) {
	v := &voiceSession{h: h, update: update, out: make(chan outgoing, pendingFrames)}
	v.ctx, v.cancel = context.WithCancel(ctx)
	dev, err := audio.Open(id, v.frame)
	if err != nil {
		v.cancel()
		return nil, err
	}
	v.dev = dev
	return v, nil
}

// frame takes a frame of the microphone's, on the audio thread.
func (v *voiceSession) frame(pcm []int16) {
	heard := false
	for _, s := range pcm {
		if s != 0 {
			heard = true
			break
		}
	}
	if !v.captured && heard {
		v.captured = true
		v.update(func() {
			if !v.ended {
				v.h.capturing()
			}
		})
	}
	if v.zeros >= 0 {
		if heard {
			v.zeros = -1
		} else if v.zeros++; v.zeros == silentFrames {
			v.zeros = -1
			name := v.dev.Microphone()
			v.update(func() {
				if !v.ended {
					v.h.silent(name)
				}
			})
		}
	}
	b := make([]byte, 2*len(pcm))
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(s))
	}
	select {
	case v.out <- outgoing{typ: websocket.BinaryFrame, data: b}:
	default:
		// The socket is slower than the microphone: the frame is lost.
	}
}

// connect connects to the session's WebSocket once start, which starts
// the session on the server, gives its URL.
func (v *voiceSession) connect(pushToTalk bool, start func(context.Context) (string, error)) {
	if v.ended || v.connecting {
		return
	}
	v.connecting = true
	v.pushToTalk = pushToTalk
	v.tell()
	go func() {
		url, err := start(v.ctx)
		if err != nil {
			v.fail(err.Error())
			return
		}
		cfg, err := websocket.NewConfig(url, strings.Replace(url, "ws", "http", 1))
		if err != nil {
			v.fail(err.Error())
			return
		}
		ws, err := cfg.DialContext(v.ctx)
		if err != nil {
			v.fail("")
			return
		}
		go func() {
			<-v.ctx.Done()
			ws.Close()
		}()
		go v.read(ws)
		for {
			select {
			case <-v.ctx.Done():
				return
			case o := <-v.out:
				if err := voiceFrames.Send(ws, o); err != nil {
					v.fail("")
					return
				}
				if o.last {
					v.cancel()
					return
				}
			}
		}
	}()
}

// read reads the session's speech and events until it closes.
func (v *voiceSession) read(ws *websocket.Conn) {
	rate := 24000
	for {
		var in outgoing
		if err := voiceFrames.Receive(ws, &in); err != nil {
			v.fail("")
			return
		}
		if in.typ == websocket.BinaryFrame {
			pcm := make([]int16, len(in.data)/2)
			for i := range pcm {
				pcm[i] = int16(binary.LittleEndian.Uint16(in.data[2*i:]))
			}
			v.dev.Play(pcm, rate)
			continue
		}
		var e voiceEvent
		if json.Unmarshal(in.data, &e) != nil {
			continue
		}
		switch e.str("type") {
		case "ready":
			v.mu.Lock()
			v.ready = true
			v.mu.Unlock()
			if r := int(e.num("output_sample_rate")); r > 0 {
				rate = r
			}
		case "run":
			v.dev.Run()
		case "interrupt":
			// The relay keeps the answer's speech as far as it was played
			// (§8.8).
			v.send(voiceEvent{"type": "played", "ms": v.dev.Drop().Milliseconds()})
		}
		v.update(func() {
			if !v.ended {
				v.h.event(e)
			}
		})
	}
}

// fail ends the session from its goroutines: unreachable when it closed
// before it was ready, with why when given.
func (v *voiceSession) fail(why string) {
	if v.ctx.Err() != nil {
		return
	}
	v.mu.Lock()
	ready := v.ready
	v.mu.Unlock()
	if why == "" && !ready {
		why = errUnreachable.Error()
	}
	v.update(func() { v.end(why) })
}

// errUnreachable tells that the session's WebSocket closed before it was
// ready.
var errUnreachable = errors.New("the voice session could not be reached")

// talk tells that the user holds to talk, or lets go.
func (v *voiceSession) talk(on bool) {
	v.talking = on
	if on {
		v.dev.Drop()
	}
	v.tell()
}

func (v *voiceSession) tell() {
	if !v.pushToTalk || v.talking == v.told {
		return
	}
	v.told = v.talking
	typ := "ptt_end"
	if v.talking {
		typ = "ptt_start"
	}
	v.send(voiceEvent{"type": typ})
}

// send sends an event after what is queued.
func (v *voiceSession) send(e voiceEvent) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	select {
	case v.out <- outgoing{typ: websocket.TextFrame, data: b}:
	case <-v.ctx.Done():
	}
}

// inputLevel and outputLevel tell how loud the microphone and the answer
// are now, from 0 to about 1.
func (v *voiceSession) inputLevel() float32 {
	if v.ended {
		return 0
	}
	return v.dev.InputLevel()
}

func (v *voiceSession) outputLevel() float32 {
	if v.ended {
		return 0
	}
	return v.dev.OutputLevel()
}

// close ends the session, telling the server once connected.
func (v *voiceSession) close() {
	if v.ended {
		return
	}
	if v.connecting {
		// The writer closes once it has sent it.
		select {
		case v.out <- outgoing{typ: websocket.TextFrame, data: []byte(`{"type":"close"}`), last: true}:
		default:
			v.cancel()
		}
	} else {
		v.cancel()
	}
	v.ended = true
	v.dev.Close()
	v.h.closed("")
}

// end ends the session on the main thread, err saying why it failed.
func (v *voiceSession) end(err string) {
	if v.ended {
		return
	}
	v.ended = true
	v.cancel()
	v.dev.Close()
	v.h.closed(err)
}

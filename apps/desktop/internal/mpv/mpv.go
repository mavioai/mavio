package mpv

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Library is libmpv, loaded.
type Library struct {
	create            func() uintptr
	initialize        func(h uintptr) int32
	setOptionString   func(h uintptr, name, value string) int32
	setPropertyString func(h uintptr, name, value string) int32
	getPropertyString func(h uintptr, name string) *byte
	getProperty       func(h uintptr, name string, format int32, data unsafe.Pointer) int32
	observeProperty   func(h uintptr, id uint64, name string, format int32) int32
	commandAsync      func(h uintptr, userdata uint64, args **byte) int32
	setPropertyAsync  func(h uintptr, userdata uint64, name string, format int32, data unsafe.Pointer) int32
	waitEvent         func(h uintptr, timeout float64) *event
	wakeup            func(h uintptr)
	terminateDestroy  func(h uintptr)
	free              func(p *byte)
	errorString       func(code int32) string
}

// Load loads libmpv at path, after the libraries it links that the app
// ships beside it, as MoltenVK on macOS.
func Load(path string, deps ...string) (*Library, error) {
	for _, d := range deps {
		if _, err := openLibrary(d); err != nil {
			return nil, fmt.Errorf("load %s: %w", d, err)
		}
	}
	h, err := openLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("load libmpv %s: %w", path, err)
	}
	l := &Library{}
	for fn, name := range map[any]string{
		&l.create:            "mpv_create",
		&l.initialize:        "mpv_initialize",
		&l.setOptionString:   "mpv_set_option_string",
		&l.setPropertyString: "mpv_set_property_string",
		&l.getPropertyString: "mpv_get_property_string",
		&l.getProperty:       "mpv_get_property",
		&l.observeProperty:   "mpv_observe_property",
		&l.commandAsync:      "mpv_command_async",
		&l.setPropertyAsync:  "mpv_set_property_async",
		&l.waitEvent:         "mpv_wait_event",
		&l.wakeup:            "mpv_wakeup",
		&l.terminateDestroy:  "mpv_terminate_destroy",
		&l.free:              "mpv_free",
		&l.errorString:       "mpv_error_string",
	} {
		purego.RegisterLibFunc(fn, h, name)
	}
	return l, nil
}

// The parts of libmpv's client API the player uses (mpv/client.h).
const (
	formatNone   = 0
	formatString = 1
	formatFlag   = 3
	formatInt64  = 4
	formatDouble = 5

	eventNone           = 0
	eventShutdown       = 1
	eventSetReply       = 4
	eventCommandReply   = 5
	eventEndFile        = 7
	eventFileLoaded     = 8
	eventPropertyChange = 22

	endFileEOF   = 0
	endFileError = 4
)

// event is an mpv_event.
type event struct {
	id            int32
	err           int32
	replyUserdata uint64
	data          unsafe.Pointer
}

// eventProperty is an mpv_event_property.
type eventProperty struct {
	name   *byte
	format int32
	data   unsafe.Pointer
}

// endFile is an mpv_event_end_file.
type endFile struct {
	reason int32
	err    int32
}

// State is what the player plays and where.
type State struct {
	// Loaded tells that a file plays, from when it opened until it ends.
	Loaded   bool
	Position time.Duration
	Duration time.Duration
	Paused   bool
	// Buffering tells that playback waits for the network or a seek.
	Buffering bool
	Speed     float64
	Muted     bool
	// SubtitleDelay shifts the subtitles later, or earlier when negative.
	SubtitleDelay time.Duration
	// Ended tells that the file played to its end.
	Ended bool
	// Err is why the file stopped, when it failed.
	Err error
	// Tracks lists the tracks of what plays.
	Tracks []Track
	// Volume is the sound's level, from 0 to 100.
	Volume float64
	// AudioDelay shifts the sound later, or earlier when negative.
	AudioDelay time.Duration
	// AudioDevice is the device the sound plays to, "auto" for the
	// system's, and AudioDevices those it can play to.
	AudioDevice  string
	AudioDevices []AudioDevice
	// Width and Height are the picture's as it shows, 0 without one.
	Width, Height int64
}

// AudioDevice is a device the sound can play to.
type AudioDevice struct {
	// Name is what the audio-device property takes.
	Name        string
	Description string
}

// Player is an mpv instance. Its methods are safe from any goroutine;
// those that change it queue the change and return at once, so that they
// never wait for mpv's core, which waits for the main thread as mpv's
// video output starts on macOS.
type Player struct {
	l       *Library
	h       uintptr
	changed func(State)

	mu       sync.Mutex
	state    State
	closed   bool
	done     chan struct{}
	requests map[uint64]string
	next     uint64
}

// The properties the player observes, by their reply IDs.
var observed = []struct {
	name   string
	format int32
}{
	{"time-pos", formatDouble},
	{"duration", formatDouble},
	{"pause", formatFlag},
	{"paused-for-cache", formatFlag},
	{"seeking", formatFlag},
	{"speed", formatDouble},
	{"mute", formatFlag},
	{"sub-delay", formatDouble},
	{"track-list", formatNone},
	{"volume", formatDouble},
	{"audio-delay", formatDouble},
	{"audio-device", formatString},
	{"audio-device-list", formatNone},
	{"dwidth", formatInt64},
	{"dheight", formatInt64},
}

// New starts a player with options, in order, as mpv's command line takes
// them without their dashes, such as {"wid", "1234"}. It calls changed
// from its own goroutine whenever its state changes.
func (l *Library) New(options [][2]string, changed func(State)) (*Player, error) {
	h := l.create()
	if h == 0 {
		return nil, errors.New("mpv: cannot create a player")
	}
	p := &Player{l: l, h: h, changed: changed, state: State{Speed: 1, Volume: 100}, done: make(chan struct{}), requests: map[uint64]string{}}
	for _, o := range options {
		if err := p.check(l.setOptionString(h, o[0], o[1])); err != nil {
			l.terminateDestroy(h)
			return nil, fmt.Errorf("mpv: set %s=%s: %w", o[0], o[1], err)
		}
	}
	if err := p.check(l.initialize(h)); err != nil {
		l.terminateDestroy(h)
		return nil, fmt.Errorf("mpv: initialize: %w", err)
	}
	for i, o := range observed {
		l.observeProperty(h, uint64(i), o.name, o.format)
	}
	go p.loop()
	return p, nil
}

func (p *Player) check(code int32) error {
	if code >= 0 {
		return nil
	}
	return errors.New(p.l.errorString(code))
}

// loop reads the player's events until it closes.
func (p *Player) loop() {
	defer close(p.done)
	for {
		e := p.l.waitEvent(p.h, -1)
		if e.id == eventPropertyChange && int(e.replyUserdata) < len(observed) {
			// Lists are read here, as the core is never waited for here.
			switch observed[e.replyUserdata].name {
			case "track-list":
				tracks := p.tracks()
				p.mu.Lock()
				p.state.Tracks = tracks
				p.mu.Unlock()
			case "audio-device-list":
				devices := p.audioDevices()
				p.mu.Lock()
				p.state.AudioDevices = devices
				p.mu.Unlock()
			}
		}
		p.mu.Lock()
		if p.closed || e.id == eventShutdown {
			p.mu.Unlock()
			return
		}
		ok := p.apply(e)
		s := p.state
		p.mu.Unlock()
		if ok && p.changed != nil {
			p.changed(s)
		}
	}
}

// apply takes an event into the state, reporting whether it changed it.
func (p *Player) apply(e *event) bool {
	s := &p.state
	switch e.id {
	case eventSetReply, eventCommandReply:
		what := p.requests[e.replyUserdata]
		delete(p.requests, e.replyUserdata)
		if e.err < 0 {
			slog.Warn("mpv request failed", "request", what, "err", p.l.errorString(e.err))
		}
		return false
	case eventFileLoaded:
		s.Loaded, s.Ended, s.Err = true, false, nil
	case eventEndFile:
		end := (*endFile)(e.data)
		s.Loaded = false
		switch end.reason {
		case endFileEOF:
			s.Ended = true
		case endFileError:
			s.Err = errors.New(p.l.errorString(end.err))
		}
	case eventPropertyChange:
		if int(e.replyUserdata) >= len(observed) {
			return false
		}
		prop := (*eventProperty)(e.data)
		var f float64
		var n int64
		var flag bool
		var str string
		switch prop.format {
		case formatDouble:
			f = *(*float64)(prop.data)
		case formatInt64:
			n = *(*int64)(prop.data)
		case formatFlag:
			flag = *(*int32)(prop.data) != 0
		case formatString:
			str = cString(*(**byte)(prop.data))
		}
		switch observed[e.replyUserdata].name {
		case "time-pos":
			s.Position = seconds(f)
		case "duration":
			s.Duration = seconds(f)
		case "pause":
			s.Paused = flag
		case "paused-for-cache", "seeking":
			s.Buffering = flag
		case "speed":
			s.Speed = f
		case "mute":
			s.Muted = flag
		case "sub-delay":
			s.SubtitleDelay = seconds(f)
		case "volume":
			s.Volume = f
		case "audio-delay":
			s.AudioDelay = seconds(f)
		case "audio-device":
			s.AudioDevice = str
		case "dwidth":
			s.Width = n
		case "dheight":
			s.Height = n
		case "track-list", "audio-device-list":
		}
	default:
		return false
	}
	return true
}

func seconds(f float64) time.Duration {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

// State returns the player's state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// request numbers a request, remembering what it was for its reply.
func (p *Player) request(what string) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	p.requests[p.next] = what
	return p.next
}

// Command queues a command of mpv's, such as {"seek", "10"}; a command
// that fails as it runs is logged.
func (p *Player) Command(args ...string) error {
	cs := make([][]byte, len(args))
	ptrs := make([]*byte, len(args)+1)
	for i, a := range args {
		cs[i] = append([]byte(a), 0)
		ptrs[i] = &cs[i][0]
	}
	err := p.check(p.l.commandAsync(p.h, p.request(strings.Join(args, " ")), &ptrs[0]))
	runtime.KeepAlive(cs)
	if err != nil {
		return fmt.Errorf("mpv: %s: %w", args[0], err)
	}
	return nil
}

// Set queues setting a property; a setting that fails is logged.
func (p *Player) Set(name, value string) error {
	v := append([]byte(value), 0)
	ptr := &v[0]
	err := p.check(p.l.setPropertyAsync(p.h, p.request("set "+name+"="+value), name, formatString, unsafe.Pointer(&ptr)))
	runtime.KeepAlive(v)
	if err != nil {
		return fmt.Errorf("mpv: set %s: %w", name, err)
	}
	return nil
}

// String returns a property as a string. It waits for mpv's core, so it
// must not run on the main thread.
func (p *Player) String(name string) (string, bool) {
	b := p.l.getPropertyString(p.h, name)
	if b == nil {
		return "", false
	}
	defer p.l.free(b)
	return cString(b), true
}

// cString copies a C string.
func cString(b *byte) string {
	if b == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(b), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(b, n))
}

// Int returns an integer property. It waits for mpv's core, so it must
// not run on the main thread.
func (p *Player) Int(name string) (int64, bool) {
	var v int64
	if p.l.getProperty(p.h, name, formatInt64, unsafe.Pointer(&v)) < 0 {
		return 0, false
	}
	return v, true
}

// Load plays url from start, replacing what played.
func (p *Player) Load(url string, start time.Duration) error {
	if err := p.Set("start", strconv.FormatFloat(start.Seconds(), 'f', 3, 64)); err != nil {
		return err
	}
	p.mu.Lock()
	p.state.Ended, p.state.Err, p.state.Position = false, nil, start
	p.mu.Unlock()
	return p.Command("loadfile", url, "replace")
}

// Track is an audio, video or subtitle track of what plays.
type Track struct {
	ID int64
	// Type is "audio", "video" or "sub".
	Type string
	// Index is the stream's index in the file, -1 for a file of its own.
	Index    int64
	Title    string
	Language string
	// External is the URL of a track of its own.
	External string
	Selected bool
}

// tracks reads the tracks of what plays.
func (p *Player) tracks() []Track {
	n, _ := p.Int("track-list/count")
	tracks := make([]Track, 0, n)
	for i := range n {
		key := func(k string) string { return fmt.Sprintf("track-list/%d/%s", i, k) }
		t := Track{Index: -1}
		t.ID, _ = p.Int(key("id"))
		t.Type, _ = p.String(key("type"))
		if v, ok := p.Int(key("ff-index")); ok {
			t.Index = v
		}
		t.Title, _ = p.String(key("title"))
		t.Language, _ = p.String(key("lang"))
		if t.External, _ = p.String(key("external-filename")); t.External != "" {
			t.Index = -1
		}
		sel, _ := p.String(key("selected"))
		t.Selected = sel == "yes"
		tracks = append(tracks, t)
	}
	return tracks
}

// audioDevices reads the devices the sound can play to.
func (p *Player) audioDevices() []AudioDevice {
	n, _ := p.Int("audio-device-list/count")
	devices := make([]AudioDevice, 0, n)
	for i := range n {
		var d AudioDevice
		d.Name, _ = p.String(fmt.Sprintf("audio-device-list/%d/name", i))
		d.Description, _ = p.String(fmt.Sprintf("audio-device-list/%d/description", i))
		devices = append(devices, d)
	}
	return devices
}

// Close stops the player and frees it, waiting for its renderer to stop
// drawing; it must not run on the main thread, which mpv's video output
// may wait for as it shuts down.
func (p *Player) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	p.l.wakeup(p.h)
	<-p.done
	p.l.terminateDestroy(p.h)
}

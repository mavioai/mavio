package voice

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// On macOS the voice processing I/O unit captures the default input and
// plays through the default output, cancelling the echo of what it plays;
// a microphone chosen is captured by a HAL unit of its own, the speech
// played by the default output unit.

// componentDescription is AudioComponentDescription, whose flags and
// flags mask are zero.
type componentDescription struct {
	typ, subType, manufacturer uint32
	_                          [2]uint32
}

type streamFormat struct {
	sampleRate       float64
	formatID         uint32
	formatFlags      uint32
	bytesPerPacket   uint32
	framesPerPacket  uint32
	bytesPerFrame    uint32
	channelsPerFrame uint32
	bitsPerChannel   uint32
	reserved         uint32
}

type audioBuffer struct {
	channels, size uint32
	data           unsafe.Pointer
}

type bufferList struct {
	n   uint32
	_   uint32
	buf [1]audioBuffer
}

type renderCallback struct{ proc, ref uintptr }

type propertyAddress struct{ selector, scope, element uint32 }

const (
	typeOutput          = 0x61756f75 // 'auou'
	subTypeVoice        = 0x7670696f // 'vpio'
	subTypeHAL          = 0x6168616c // 'ahal'
	subTypeDefault      = 0x64656620 // 'def '
	manufacturerApple   = 0x6170706c // 'appl'
	formatLinearPCM     = 0x6C70636D // 'lpcm'
	formatFloatPacked   = 1 | 8      // float, packed
	propStreamFormat    = 8
	propRenderCallback  = 23
	propCurrentDevice   = 2000
	propEnableIO        = 2003
	propInputCallback   = 2005
	scopeGlobal         = 0
	scopeInput          = 1
	scopeOutput         = 2
	busOutput           = 0
	busInput            = 1
	systemObject        = 1
	propDevices         = 0x64657623 // 'dev#'
	propDefaultInput    = 0x64496e20 // 'dIn '
	propStreams         = 0x73746d23 // 'stm#'
	propName            = 0x6c6e616d // 'lnam'
	propUID             = 0x75696420 // 'uid '
	objectScopeGlobal   = 0x676c6f62 // 'glob'
	objectScopeInput    = 0x696e7074 // 'inpt'
	cfStringEncodingUTF = 0x08000100
	maxFrames           = 8192
)

var (
	loadOnce sync.Once
	loadErr  error

	componentFindNext     func(after uintptr, desc *componentDescription) uintptr
	componentInstanceNew  func(comp uintptr, out *uintptr) int32
	componentDispose      func(u uintptr) int32
	unitSetProperty       func(u uintptr, id, scope, elem uint32, data unsafe.Pointer, size uint32) int32
	unitGetProperty       func(u uintptr, id, scope, elem uint32, data unsafe.Pointer, size *uint32) int32
	unitInitialize        func(u uintptr) int32
	unitUninitialize      func(u uintptr) int32
	outputUnitStart       func(u uintptr) int32
	outputUnitStop        func(u uintptr) int32
	unitRender            func(u uintptr, flags *uint32, ts uintptr, bus, frames uint32, data *bufferList) int32
	objectGetPropertySize func(obj uint32, addr *propertyAddress, qsize uint32, q uintptr, size *uint32) int32
	objectGetProperty     func(obj uint32, addr *propertyAddress, qsize uint32, q uintptr, size *uint32, data unsafe.Pointer) int32
	cfStringGetCString    func(s uintptr, buf *byte, size int, enc uint32) bool
	cfRelease             func(uintptr)

	// The callbacks are made once and find the device open through
	// current, by the reference they are given.
	inputProc, renderProc uintptr
	current               atomic.Pointer[device]
	opened                atomic.Uintptr
)

func load() error {
	loadOnce.Do(func() {
		at, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		ca, err := purego.Dlopen("/System/Library/Frameworks/CoreAudio.framework/CoreAudio", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&componentFindNext, at, "AudioComponentFindNext")
		purego.RegisterLibFunc(&componentInstanceNew, at, "AudioComponentInstanceNew")
		purego.RegisterLibFunc(&componentDispose, at, "AudioComponentInstanceDispose")
		purego.RegisterLibFunc(&unitSetProperty, at, "AudioUnitSetProperty")
		purego.RegisterLibFunc(&unitGetProperty, at, "AudioUnitGetProperty")
		purego.RegisterLibFunc(&unitInitialize, at, "AudioUnitInitialize")
		purego.RegisterLibFunc(&unitUninitialize, at, "AudioUnitUninitialize")
		purego.RegisterLibFunc(&outputUnitStart, at, "AudioOutputUnitStart")
		purego.RegisterLibFunc(&outputUnitStop, at, "AudioOutputUnitStop")
		purego.RegisterLibFunc(&unitRender, at, "AudioUnitRender")
		purego.RegisterLibFunc(&objectGetPropertySize, ca, "AudioObjectGetPropertyDataSize")
		purego.RegisterLibFunc(&objectGetProperty, ca, "AudioObjectGetPropertyData")
		purego.RegisterLibFunc(&cfStringGetCString, cf, "CFStringGetCString")
		purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
		// The units' I/O threads call them.
		inputProc = purego.NewCallback(func(ref uintptr, flags *uint32, ts uintptr, _, frames uint32, _ *bufferList) uintptr {
			if d := current.Load(); d != nil && d.ref == ref {
				d.input(flags, ts, frames)
			}
			return 0
		})
		renderProc = purego.NewCallback(func(ref uintptr, _ *uint32, _ uintptr, _, _ uint32, data *bufferList) uintptr {
			d := current.Load()
			for i := range data.n {
				b := (*audioBuffer)(unsafe.Add(unsafe.Pointer(&data.buf[0]), uintptr(i)*unsafe.Sizeof(audioBuffer{})))
				out := unsafe.Slice((*float32)(b.data), b.size/4)
				if d != nil && d.ref == ref && i == 0 {
					d.play.read(out)
				} else {
					clear(out)
				}
			}
			return 0
		})
	})
	return loadErr
}

// System returns the platform's audio.
func System() Audio { return system{} }

type system struct{}

func (system) Microphones() ([]Microphone, error) {
	if err := load(); err != nil {
		return nil, err
	}
	var mics []Microphone
	for _, id := range devices() {
		if !hasInput(id) {
			continue
		}
		mics = append(mics, Microphone{ID: deviceString(id, propUID), Name: deviceString(id, propName)})
	}
	return mics, nil
}

func (system) Open(id string, frame func([]int16)) (Device, error) {
	if err := load(); err != nil {
		return nil, err
	}
	if err := allow(); err != nil {
		return nil, err
	}
	d := &device{ref: opened.Add(1)}
	chosen := uint32(0)
	if id != "" {
		for _, dev := range devices() {
			if hasInput(dev) && deviceString(dev, propUID) == id {
				chosen = dev
				break
			}
		}
	}
	var err error
	if chosen == 0 {
		err = d.openVoice(frame)
	} else {
		err = d.openChosen(chosen, frame)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// device is the units of a session: in captures, out plays, one unit for
// voice processing.
type device struct {
	ref     uintptr
	in, out uintptr
	name    string
	capture *capture
	play    playback
	buf     []float32
	list    bufferList
	pinner  runtime.Pinner
	closed  bool
}

// mono is a format of mono float samples at rate.
func mono(rate float64) streamFormat {
	return streamFormat{
		sampleRate: rate, formatID: formatLinearPCM, formatFlags: formatFloatPacked,
		bytesPerPacket: 4, framesPerPacket: 1, bytesPerFrame: 4, channelsPerFrame: 1, bitsPerChannel: 32,
	}
}

func newUnit(subType uint32) (uintptr, error) {
	comp := componentFindNext(0, &componentDescription{typ: typeOutput, subType: subType, manufacturer: manufacturerApple})
	if comp == 0 {
		return 0, fmt.Errorf("voice: no audio unit %#x", subType)
	}
	var u uintptr
	if st := componentInstanceNew(comp, &u); st != 0 {
		return 0, fmt.Errorf("voice: AudioComponentInstanceNew: %d", st)
	}
	return u, nil
}

func set[T any](u uintptr, id, scope, elem uint32, v *T) error {
	if st := unitSetProperty(u, id, scope, elem, unsafe.Pointer(v), uint32(unsafe.Sizeof(*v))); st != 0 {
		return fmt.Errorf("voice: AudioUnitSetProperty %d: %d", id, st)
	}
	return nil
}

func start(u uintptr) error {
	if st := unitInitialize(u); st != 0 {
		return fmt.Errorf("voice: AudioUnitInitialize: %d", st)
	}
	if st := outputUnitStart(u); st != 0 {
		return fmt.Errorf("voice: AudioOutputUnitStart: %d", st)
	}
	return nil
}

// prepare readies the capture at rate and the buffer the input renders
// into, and makes the device the callbacks' current one.
func (d *device) prepare(rate float64, frame func([]int16)) {
	d.capture = newCapture(rate, frame)
	d.buf = make([]float32, maxFrames)
	d.pinner.Pin(&d.buf[0])
	current.Store(d)
}

// openVoice opens the voice processing unit, capturing and playing.
func (d *device) openVoice(frame func([]int16)) error {
	u, err := newUnit(subTypeVoice)
	if err != nil {
		return err
	}
	d.in = u
	d.name = deviceString(defaultInput(), propName)
	one := uint32(1)
	capture, speech := mono(Rate), mono(outputRate)
	if err := set(u, propEnableIO, scopeInput, busInput, &one); err != nil {
		return err
	}
	if err := set(u, propEnableIO, scopeOutput, busOutput, &one); err != nil {
		return err
	}
	if err := set(u, propStreamFormat, scopeOutput, busInput, &capture); err != nil {
		return err
	}
	if err := set(u, propStreamFormat, scopeInput, busOutput, &speech); err != nil {
		return err
	}
	d.prepare(Rate, frame)
	if err := set(u, propInputCallback, scopeGlobal, busInput, &renderCallback{inputProc, d.ref}); err != nil {
		return err
	}
	if err := set(u, propRenderCallback, scopeInput, busOutput, &renderCallback{renderProc, d.ref}); err != nil {
		return err
	}
	return start(u)
}

// openChosen captures the input dev with a HAL unit, and plays through
// the default output unit.
func (d *device) openChosen(dev uint32, frame func([]int16)) error {
	u, err := newUnit(subTypeHAL)
	if err != nil {
		return err
	}
	d.in = u
	d.name = deviceString(dev, propName)
	one, zero := uint32(1), uint32(0)
	if err := set(u, propEnableIO, scopeInput, busInput, &one); err != nil {
		return err
	}
	if err := set(u, propEnableIO, scopeOutput, busOutput, &zero); err != nil {
		return err
	}
	if err := set(u, propCurrentDevice, scopeGlobal, busOutput, &dev); err != nil {
		return err
	}
	// The HAL unit converts no rate on input: it captures at the device's.
	var hw streamFormat
	size := uint32(unsafe.Sizeof(hw))
	if st := unitGetProperty(u, propStreamFormat, scopeInput, busInput, unsafe.Pointer(&hw), &size); st != 0 || hw.sampleRate <= 0 {
		return fmt.Errorf("voice: the microphone's format: %d", st)
	}
	capture := mono(hw.sampleRate)
	if err := set(u, propStreamFormat, scopeOutput, busInput, &capture); err != nil {
		return err
	}
	d.prepare(hw.sampleRate, frame)
	if err := set(u, propInputCallback, scopeGlobal, busInput, &renderCallback{inputProc, d.ref}); err != nil {
		return err
	}
	if err := start(u); err != nil {
		return err
	}
	out, err := newUnit(subTypeDefault)
	if err != nil {
		return err
	}
	d.out = out
	speech := mono(outputRate)
	if err := set(out, propStreamFormat, scopeInput, busOutput, &speech); err != nil {
		return err
	}
	if err := set(out, propRenderCallback, scopeInput, busOutput, &renderCallback{renderProc, d.ref}); err != nil {
		return err
	}
	return start(out)
}

// input renders the microphone's frames into the buffer and captures
// them, on the input unit's thread.
func (d *device) input(flags *uint32, ts uintptr, frames uint32) {
	n := min(frames, maxFrames)
	d.list = bufferList{n: 1, buf: [1]audioBuffer{{channels: 1, size: n * 4, data: unsafe.Pointer(&d.buf[0])}}}
	if unitRender(d.in, flags, ts, busInput, n, &d.list) != 0 {
		return
	}
	d.capture.write(d.buf[:n])
}

func (d *device) Microphone() string             { return d.name }
func (d *device) Play(pcm []int16, rate int)     { d.play.push(pcm, rate) }
func (d *device) Run()                           { d.play.run() }
func (d *device) Progress() (_, _ time.Duration) { return d.play.progress() }
func (d *device) Drop() time.Duration            { return d.play.drop() }
func (d *device) OutputLevel() float32           { return d.play.meter.level() }

func (d *device) InputLevel() float32 {
	if d.capture == nil {
		return 0
	}
	return d.capture.meter.level()
}

func (d *device) Close() {
	if d.closed {
		return
	}
	d.closed = true
	for _, u := range []uintptr{d.in, d.out} {
		if u != 0 {
			outputUnitStop(u)
			unitUninitialize(u)
			componentDispose(u)
		}
	}
	current.CompareAndSwap(d, nil)
	d.pinner.Unpin()
}

// allow asks the user for the microphone the first time, waiting for the
// answer, and tells when it is refused.
func allow() error {
	cls := objc.ID(objc.GetClass("AVCaptureDevice"))
	if cls == 0 {
		return nil
	}
	audio := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), "soun")
	switch objc.Send[int](cls, objc.RegisterName("authorizationStatusForMediaType:"), audio) {
	case 3: // authorized
		return nil
	case 1, 2: // restricted, denied
		return ErrDenied
	}
	answer := make(chan bool, 1)
	block := objc.NewBlock(func(_ objc.Block, granted bool) { answer <- granted })
	defer block.Release()
	cls.Send(objc.RegisterName("requestAccessForMediaType:completionHandler:"), audio, block)
	if !<-answer {
		return ErrDenied
	}
	return nil
}

// devices lists the audio devices.
func devices() []uint32 {
	addr := propertyAddress{propDevices, objectScopeGlobal, 0}
	var size uint32
	if objectGetPropertySize(systemObject, &addr, 0, 0, &size) != 0 || size == 0 {
		return nil
	}
	ids := make([]uint32, size/4)
	if objectGetProperty(systemObject, &addr, 0, 0, &size, unsafe.Pointer(&ids[0])) != 0 {
		return nil
	}
	return ids[:size/4]
}

func defaultInput() uint32 {
	addr := propertyAddress{propDefaultInput, objectScopeGlobal, 0}
	var id uint32
	size := uint32(4)
	if objectGetProperty(systemObject, &addr, 0, 0, &size, unsafe.Pointer(&id)) != 0 {
		return 0
	}
	return id
}

// hasInput reports whether a device has input streams.
func hasInput(id uint32) bool {
	addr := propertyAddress{propStreams, objectScopeInput, 0}
	var size uint32
	return objectGetPropertySize(id, &addr, 0, 0, &size) == 0 && size > 0
}

// deviceString reads a string property of a device.
func deviceString(id, prop uint32) string {
	if id == 0 {
		return ""
	}
	addr := propertyAddress{prop, objectScopeGlobal, 0}
	var s uintptr
	size := uint32(unsafe.Sizeof(s))
	if objectGetProperty(id, &addr, 0, 0, &size, unsafe.Pointer(&s)) != 0 || s == 0 {
		return ""
	}
	defer cfRelease(s)
	buf := make([]byte, 512)
	if !cfStringGetCString(s, &buf[0], len(buf), cfStringEncodingUTF) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

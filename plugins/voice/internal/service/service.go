package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SettingType is the kind of a setting's field.
type SettingType int

// The kinds of fields.
const (
	Text SettingType = iota
	Password
	Number
	Checkbox
)

// Setting is a setting a recognizer or synthesizer takes.
type Setting struct {
	Name, Label, Help string
	Type              SettingType
	Default           any
	Advanced          bool
}

// Env is what a recognizer or synthesizer is made with.
type Env struct {
	// HTTP reaches the services.
	HTTP *http.Client
	Log  *slog.Logger
}

// Audio is audio to recognize: 16 kHz mono 16-bit little-endian PCM.
type Audio struct {
	// Open returns the audio; it may be called more than once.
	Open func(ctx context.Context) (io.ReadCloser, error)
	// Name is what the audio is of, for the service's logs.
	Name string
}

// SampleRate and BytesPerSecond describe the audio.
const (
	SampleRate     = 16000
	BytesPerSecond = SampleRate * 2
)

// Request is what to recognize.
type Request struct {
	Audio Audio
	// Language is the language spoken, BCP 47; empty to detect it.
	Language   string
	Translate  bool
	Hints      []string
	Timestamps bool
}

// Segment is something said, with its times.
type Segment struct {
	Start, End time.Duration
	Text       string
}

// Result is what was recognized.
type Result struct {
	Text string
	// Language is the language recognized, BCP 47.
	Language string
	Segments []Segment
}

// Caps are what a recognizer can do.
type Caps struct {
	Languages                              []string
	Timestamps, Detects, Translates, Hints bool
	MaxDuration                            time.Duration
}

// Recognizer is a configured recognition service.
type Recognizer interface {
	// Caps tells what it can do with its settings.
	Caps() Caps
	Recognize(ctx context.Context, req Request) (Result, error)
	// DetectLanguage tells the language spoken in audio, BCP 47; "" when
	// it cannot be told.
	DetectLanguage(ctx context.Context, audio Audio) (string, error)
}

// RecognizerDef is a recognition service the plugin implements.
type RecognizerDef struct {
	Name, DisplayName string
	Settings          []Setting
	// Caps are what it can do with its default settings.
	Caps Caps
	New  func(env Env, settings json.RawMessage) (Recognizer, error)
}

// Voice is a voice of a synthesizer.
type Voice struct {
	// ID is unique within the synthesizer.
	ID, Name  string
	Languages []string
}

// Format is the encoding of synthesized audio.
type Format int

// The encodings.
const (
	OggOpus Format = iota
	MP3
)

// Speech is synthesized audio.
type Speech struct {
	Audio       []byte
	ContentType string
}

// Synthesizer is a configured synthesis service.
type Synthesizer interface {
	Voices() []Voice
	// Synthesize speaks text with a voice; speed 0 is 1.
	Synthesize(ctx context.Context, voice, text, language string, format Format, speed float64) (Speech, error)
}

// SynthesizerDef is a synthesis service the plugin implements.
type SynthesizerDef struct {
	Name, DisplayName string
	Settings          []Setting
	New               func(env Env, settings json.RawMessage) (Synthesizer, error)
}

// ErrConfig is a setting that cannot be used.
var ErrConfig = errors.New("invalid settings")

// Decode decodes settings into v, failing with ErrConfig.
func Decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return nil
}

// Answer reads a service's answer, failing with its body when its status
// is not a success.
func Answer(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := string(body)
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, fmt.Errorf("%w: %s: %s", ErrService, resp.Status, msg)
	}
	return body, nil
}

// ErrService is a service that failed to answer.
var ErrService = errors.New("the service failed")

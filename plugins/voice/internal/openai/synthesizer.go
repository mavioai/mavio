package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

const (
	defaultSpeechModel = "gpt-4o-mini-tts"
	// defaultVoices are OpenAI's voices.
	defaultVoices = "alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse"
)

// SynthesizerDefinition returns the synthesizer's definition.
func SynthesizerDefinition() service.SynthesizerDef {
	s := connectionSettings(defaultSpeechModel, "Such as gpt-4o-mini-tts, tts-1, or speaches-ai/Kokoro-82M-v1.0-ONNX.")
	s = append(s,
		service.Setting{
			Name: "voices", Label: "Voices", Help: "The voices the model speaks with, separated by commas, such as af_heart for Kokoro.",
			Type: service.Text, Default: defaultVoices,
		},
		service.Setting{
			Name: "format", Label: "Audio format", Help: "None leaves it to the service, MP3 for OpenAI's; Opus is smaller, for the services that make it. The server converts it for clients that cannot play it.",
			Type: service.Select, Options: []service.Option{{Value: "opus", Label: "Opus"}, {Value: "mp3", Label: "MP3"}},
		},
		service.Setting{
			Name: "instructions", Label: "Instructions", Help: "How to speak, such as \"warm and unhurried\", for models that take them, such as gpt-4o-mini-tts.",
			Type: service.Text,
		},
	)
	return service.SynthesizerDef{Name: Name, DisplayName: "OpenAI-compatible speech", Settings: s, New: NewSynthesizer}
}

type synthesizerSettings struct {
	connection
	Voices       *string `json:"voices"`
	Format       string  `json:"format"`
	Instructions string  `json:"instructions"`
}

// NewSynthesizer makes the synthesizer from its settings.
func NewSynthesizer(env service.Env, raw json.RawMessage) (service.Synthesizer, error) {
	var s synthesizerSettings
	if err := service.Decode(raw, &s); err != nil {
		return nil, err
	}
	c, err := newClient(env, s.connection, defaultSpeechModel)
	if err != nil {
		return nil, err
	}
	list := defaultVoices
	if s.Voices != nil {
		list = *s.Voices
	}
	if s.Format != "" && s.Format != "opus" && s.Format != "mp3" {
		return nil, fmt.Errorf("%w: audio format %q", service.ErrConfig, s.Format)
	}
	out := &Synthesizer{c: c, format: s.Format, instructions: strings.TrimSpace(s.Instructions)}
	for v := range strings.SplitSeq(list, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out.voices = append(out.voices, service.Voice{ID: v, Name: title(v)})
		}
	}
	if len(out.voices) == 0 {
		return nil, fmt.Errorf("%w: no voices", service.ErrConfig)
	}
	return out, nil
}

func title(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Synthesizer is a server of the OpenAI-compatible speech API.
type Synthesizer struct {
	c      *client
	voices []service.Voice
	// format is the encoding asked for, opus or mp3; empty asks for none,
	// leaving it to the service.
	format       string
	instructions string
}

// Voices implements service.Synthesizer; they speak any language.
func (s *Synthesizer) Voices() []service.Voice { return s.voices }

// Synthesize implements service.Synthesizer: in the format of the
// settings, else the service's, MP3 for OpenAI's.
func (s *Synthesizer) Synthesize(ctx context.Context, voice, text, _ string, speed float64) (service.Speech, error) {
	req := map[string]any{"model": s.c.model, "input": text, "voice": voice}
	if s.format != "" {
		req["response_format"] = s.format
	}
	if s.instructions != "" {
		req["instructions"] = s.instructions
	}
	if speed > 0 {
		req["speed"] = speed
	}
	body, err := json.Marshal(req)
	if err != nil {
		return service.Speech{}, err
	}
	audio, err := s.c.do(ctx, "/audio/speech", "application/json", body)
	if err != nil {
		return service.Speech{}, err
	}
	return service.Speech{Audio: audio, ContentType: contentType(audio)}, nil
}

// contentType tells the encodings services answer in by their first bytes:
// Opus comes in Ogg, whose pages start "OggS", AAC in ADTS frames; anything
// else is taken for MP3.
func contentType(audio []byte) string {
	switch {
	case bytes.HasPrefix(audio, []byte("OggS")):
		return "audio/ogg"
	case bytes.HasPrefix(audio, []byte("fLaC")):
		return "audio/flac"
	case bytes.HasPrefix(audio, []byte("RIFF")):
		return "audio/wav"
	case len(audio) > 1 && audio[0] == 0xff && audio[1]&0xf6 == 0xf0:
		return "audio/aac"
	}
	return "audio/mpeg"
}

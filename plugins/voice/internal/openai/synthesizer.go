package openai

import (
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
	s = append(s, service.Setting{
		Name: "voices", Label: "Voices", Help: "The voices the model speaks with, separated by commas, such as af_heart for Kokoro.",
		Type: service.Text, Default: defaultVoices,
	})
	return service.SynthesizerDef{Name: Name, DisplayName: "OpenAI-compatible speech", Settings: s, New: NewSynthesizer}
}

type synthesizerSettings struct {
	connection
	Voices *string `json:"voices"`
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
	out := &Synthesizer{c: c}
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
}

// Voices implements service.Synthesizer; they speak any language.
func (s *Synthesizer) Voices() []service.Voice { return s.voices }

// Synthesize implements service.Synthesizer.
func (s *Synthesizer) Synthesize(ctx context.Context, voice, text, _ string, format service.Format, speed float64) (service.Speech, error) {
	req := map[string]any{"model": s.c.model, "input": text, "voice": voice, "response_format": "opus"}
	contentType := "audio/ogg"
	if format == service.MP3 {
		req["response_format"], contentType = "mp3", "audio/mpeg"
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
	return service.Speech{Audio: audio, ContentType: contentType}, nil
}

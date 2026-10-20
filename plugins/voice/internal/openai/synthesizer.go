package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

const (
	defaultSpeechModel = "gpt-4o-mini-tts"
	// defaultVoices are OpenAI's voices.
	defaultVoices = "alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse"
	// pcmRate is the sample rate of the API's PCM.
	pcmRate = 24000
)

// formats are the API's audio formats.
var formats = []service.Option{
	{Value: "mp3", Label: "MP3"},
	{Value: "opus", Label: "Opus"},
	{Value: "aac", Label: "AAC"},
	{Value: "flac", Label: "FLAC"},
	{Value: "wav", Label: "WAV"},
	{Value: "pcm", Label: "PCM"},
}

// SynthesizerDefinition returns the synthesizer's definition.
func SynthesizerDefinition() service.SynthesizerDef {
	s := connectionSettings(defaultSpeechModel, "Such as gpt-4o-mini-tts, tts-1, or speaches-ai/Kokoro-82M-v1.0-ONNX.")
	s = append(s,
		service.Setting{
			Name: "voices", Label: "Voices", Help: "The voices the model speaks with, separated by commas, such as af_heart for Kokoro.",
			Type: service.Text, Default: defaultVoices,
		},
		service.Setting{
			Name: "format", Label: "Audio format", Help: "One the service makes; none leaves it to the service, MP3 for OpenAI's. Opus is the smallest; WAV and PCM are the quickest to make, and large. The server converts it for clients that cannot play it.",
			Type: service.Select, Options: formats,
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
	if s.Format != "" && !slices.ContainsFunc(formats, func(o service.Option) bool { return o.Value == s.Format }) {
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
	// format is the audio format asked for; empty asks for none, leaving
	// it to the service.
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
	audio, header, err := s.c.answer(ctx, "/audio/speech", "application/json", body)
	if err != nil {
		return service.Speech{}, err
	}
	ct, rate := contentType(header, audio, s.format)
	if ct == "" {
		// Raw samples go in a WAV file, which says what they are.
		return service.Speech{Audio: service.WAVAt(audio, rate), ContentType: "audio/wav"}, nil
	}
	return service.Speech{Audio: audio, ContentType: ct}, nil
}

// contentType tells the encoding of speech by the content type the service
// gives, else by its first bytes: Opus comes in Ogg, whose pages start
// "OggS", AAC in ADTS frames, MP3 after an ID3 tag or in frames. Raw PCM,
// asked for or not told by either, returns "" with its sample rate, the
// API's 24 kHz unless the content type says another.
func contentType(header string, audio []byte, format string) (string, int) {
	media, params, _ := mime.ParseMediaType(header)
	switch media {
	case "audio/ogg", "audio/opus", "application/ogg":
		return "audio/ogg", 0
	case "audio/mpeg", "audio/mp3":
		return "audio/mpeg", 0
	case "audio/aac", "audio/x-aac":
		return "audio/aac", 0
	case "audio/flac", "audio/x-flac":
		return "audio/flac", 0
	case "audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave":
		return "audio/wav", 0
	case "audio/pcm", "audio/l16", "audio/raw":
		if rate, err := strconv.Atoi(params["rate"]); err == nil && rate > 0 {
			return "", rate
		}
		return "", pcmRate
	}
	switch {
	case format == "pcm":
	case bytes.HasPrefix(audio, []byte("OggS")):
		return "audio/ogg", 0
	case bytes.HasPrefix(audio, []byte("fLaC")):
		return "audio/flac", 0
	case bytes.HasPrefix(audio, []byte("RIFF")):
		return "audio/wav", 0
	case bytes.HasPrefix(audio, []byte("ID3")):
		return "audio/mpeg", 0
	case len(audio) > 2 && audio[0] == 0xff && audio[1]&0xf6 == 0xf0:
		return "audio/aac", 0
	case len(audio) > 2 && audio[0] == 0xff && audio[1]&0xe0 == 0xe0 && audio[1]&0x06 != 0 &&
		audio[2]>>4 != 0 && audio[2]>>4 != 15 && audio[2]&0x0c != 0x0c:
		// An MP3 frame's header: its layer, bitrate and sample rate are valid.
		return "audio/mpeg", 0
	}
	return "", pcmRate
}

package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
	"github.com/mavioai/mavio/plugins/voice/internal/whisper"
)

const (
	defaultRecognitionModel = "whisper-1"
	// defaultChunk is the longest audio sent at once: ten minutes are
	// 19 MB of WAV, within the API's 25 MB.
	defaultChunk = 10 * time.Minute
	// detectLength is how much audio languages are detected from.
	detectLength = 30 * time.Second
)

// RecognizerDefinition returns the recognizer's definition.
func RecognizerDefinition() service.RecognizerDef {
	s := connectionSettings(defaultRecognitionModel, "Such as whisper-1, gpt-4o-transcribe, whisper-large-v3 or Systran/faster-whisper-large-v3.")
	s = append(s,
		service.Setting{Name: "timestamps", Label: "Times of what is said", Help: "The model answers verbose_json with segments, as whisper-1 and Whisper servers do; needed to transcribe subtitles.", Type: service.Checkbox, Default: true},
		service.Setting{Name: "translates", Label: "Translation to English", Help: "The server translates speech to English, as OpenAI's whisper-1 does.", Type: service.Checkbox, Default: true},
	)
	return service.RecognizerDef{
		Name: Name, DisplayName: "OpenAI-compatible transcription",
		Settings: s,
		Caps:     recognizerCaps(true, true),
		New:      NewRecognizer,
	}
}

func recognizerCaps(timestamps, translates bool) service.Caps {
	// The language is told only with verbose_json.
	return service.Caps{Languages: whisper.Tags(), Timestamps: timestamps, Detects: timestamps, Translates: translates, Hints: true}
}

type recognizerSettings struct {
	connection
	Timestamps *bool `json:"timestamps"`
	Translates *bool `json:"translates"`
}

// NewRecognizer makes the recognizer from its settings.
func NewRecognizer(env service.Env, raw json.RawMessage) (service.Recognizer, error) {
	var s recognizerSettings
	if err := service.Decode(raw, &s); err != nil {
		return nil, err
	}
	c, err := newClient(env, s.connection, defaultRecognitionModel)
	if err != nil {
		return nil, err
	}
	r := &Recognizer{c: c, timestamps: true, translates: true, chunk: defaultChunk}
	if s.Timestamps != nil {
		r.timestamps = *s.Timestamps
	}
	if s.Translates != nil {
		r.translates = *s.Translates
	}
	return r, nil
}

// Recognizer is a server of the OpenAI-compatible transcription API.
type Recognizer struct {
	c                      *client
	timestamps, translates bool
	// chunk is the longest audio sent at once.
	chunk time.Duration
}

// Caps implements service.Recognizer.
func (r *Recognizer) Caps() service.Caps { return recognizerCaps(r.timestamps, r.translates) }

type transcription struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
}

// Recognize implements service.Recognizer: the audio is sent ten minutes
// at a time, whose times are put together.
func (r *Recognizer) Recognize(ctx context.Context, req service.Request) (service.Result, error) {
	if req.Translate && !r.translates {
		return service.Result{}, fmt.Errorf("%w: the recognizer does not translate", service.ErrConfig)
	}
	pcm, err := req.Audio.Read(ctx)
	if err != nil {
		return service.Result{}, fmt.Errorf("audio: %w", err)
	}
	size := int(r.chunk/time.Second) * service.BytesPerSecond
	var out service.Result
	var texts []string
	for at := 0; at < len(pcm); at += size {
		part := pcm[at:min(at+size, len(pcm))]
		t, err := r.send(ctx, part, req)
		if err != nil {
			return service.Result{}, err
		}
		texts = append(texts, strings.TrimSpace(t.Text))
		if out.Language == "" {
			out.Language = whisper.Tag(t.Language)
		}
		offset := service.Duration(pcm[:at])
		for _, s := range t.Segments {
			out.Segments = append(out.Segments, service.Segment{
				Start: offset + seconds(s.Start), End: offset + seconds(s.End), Text: strings.TrimSpace(s.Text),
			})
		}
	}
	out.Text = strings.TrimSpace(strings.Join(texts, " "))
	switch {
	case req.Translate:
		out.Language = "en"
	case out.Language == "":
		out.Language = req.Language
	}
	if !req.Timestamps {
		out.Segments = nil
	}
	return out, nil
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second)).Round(time.Millisecond)
}

// send recognizes a piece of the audio.
func (r *Recognizer) send(ctx context.Context, pcm []byte, req service.Request) (transcription, error) {
	f := newForm()
	f.file("file", "audio.wav", service.WAV(pcm))
	f.field("model", r.c.model)
	path := "/audio/transcriptions"
	if req.Translate {
		path = "/audio/translations"
	} else if req.Language != "" {
		code := whisper.Code(req.Language)
		if code == "" {
			code, _, _ = strings.Cut(req.Language, "-")
		}
		f.field("language", code)
	}
	f.field("prompt", strings.Join(req.Hints, ", "))
	if r.timestamps {
		f.field("response_format", "verbose_json")
		if !req.Translate {
			f.field("timestamp_granularities[]", "segment")
		}
	} else {
		f.field("response_format", "json")
	}
	body, contentType := f.close()
	data, err := r.c.do(ctx, path, contentType, body)
	if err != nil {
		return transcription{}, err
	}
	var t transcription
	if err := json.Unmarshal(data, &t); err != nil {
		return transcription{}, fmt.Errorf("%w: %w", service.ErrService, err)
	}
	return t, nil
}

// DetectLanguage implements service.Recognizer, recognizing the audio's
// first 30 seconds; "" without verbose_json, which alone tells it.
func (r *Recognizer) DetectLanguage(ctx context.Context, audio service.Audio) (string, error) {
	if !r.timestamps {
		return "", nil
	}
	pcm, err := audio.First(detectLength).Read(ctx)
	if err != nil {
		return "", fmt.Errorf("audio: %w", err)
	}
	t, err := r.send(ctx, pcm, service.Request{})
	if err != nil {
		return "", err
	}
	return whisper.Tag(t.Language), nil
}

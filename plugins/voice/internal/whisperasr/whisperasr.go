package whisperasr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/voice/internal/service"
	"github.com/mavioai/mavio/plugins/voice/internal/whisper"
)

// Name is the recognizer's name.
const Name = "whisper-asr"

const (
	defaultEndpoint = "http://127.0.0.1:9000"
	defaultResponse = 5
	defaultTimeout  = 3600
	// detectLength is how much audio languages are detected from:
	// Whisper detects them from the first 30 seconds.
	detectLength = 30 * time.Second
)

func caps() service.Caps {
	return service.Caps{Languages: whisper.Tags(), Timestamps: true, Detects: true, Translates: true, Hints: true}
}

// Definition returns the recognizer's definition.
func Definition() service.RecognizerDef {
	return service.RecognizerDef{
		Name: Name, DisplayName: "whisper-asr-webservice",
		Settings: []service.Setting{
			{Name: "endpoint", Label: "Endpoint", Help: "The whisper-asr-webservice server.", Type: service.Text, Default: defaultEndpoint},
			{Name: "response", Label: "Connection timeout in seconds", Type: service.Number, Default: defaultResponse, Advanced: true},
			{Name: "timeout", Label: "Recognition timeout in seconds", Type: service.Number, Default: defaultTimeout, Advanced: true},
			{Name: "pass_name", Label: "Pass the video's file name (for the server's logs)", Type: service.Checkbox, Default: false, Advanced: true},
		},
		Caps: caps(),
		New:  New,
	}
}

type settings struct {
	Endpoint string `json:"endpoint"`
	Response *int   `json:"response"`
	Timeout  *int   `json:"timeout"`
	PassName bool   `json:"pass_name"`
}

// New makes the recognizer from its settings.
func New(_ service.Env, raw json.RawMessage) (service.Recognizer, error) {
	var s settings
	if err := service.Decode(raw, &s); err != nil {
		return nil, err
	}
	if s.Endpoint = strings.TrimSuffix(strings.TrimSpace(s.Endpoint), "/"); s.Endpoint == "" {
		s.Endpoint = defaultEndpoint
	}
	if u, err := url.Parse(s.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: endpoint %q", service.ErrConfig, s.Endpoint)
	}
	response, timeout := defaultResponse, defaultTimeout
	if s.Response != nil {
		response = *s.Response
	}
	if s.Timeout != nil {
		timeout = *s.Timeout
	}
	if response <= 0 || timeout <= 0 {
		return nil, fmt.Errorf("%w: the timeouts must be positive", service.ErrConfig)
	}
	// The server is reached directly, connecting within the response
	// timeout and answering within the recognition timeout.
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialContext = (&net.Dialer{Timeout: time.Duration(response) * time.Second}).DialContext
	return &Recognizer{s: s, http: &http.Client{Transport: t, Timeout: time.Duration(response+timeout) * time.Second}}, nil
}

// Recognizer is a whisper-asr-webservice server.
type Recognizer struct {
	s    settings
	http *http.Client
}

// Caps implements service.Recognizer.
func (r *Recognizer) Caps() service.Caps { return caps() }

// answer is the JSON whisper-asr-webservice answers recognitions with,
// Whisper's result.
type answer struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
}

// Recognize implements service.Recognizer.
func (r *Recognizer) Recognize(ctx context.Context, req service.Request) (service.Result, error) {
	q := url.Values{"output": {"json"}}
	q.Set("task", "transcribe")
	if req.Translate {
		q.Set("task", "translate")
	}
	if req.Language != "" {
		code := whisper.Code(req.Language)
		if code == "" {
			return service.Result{}, fmt.Errorf("%w: Whisper does not recognize %q", service.ErrConfig, req.Language)
		}
		q.Set("language", code)
	}
	if len(req.Hints) > 0 {
		q.Set("initial_prompt", strings.Join(req.Hints, ", "))
	}
	body, err := r.post(ctx, "/asr", q, req.Audio)
	if err != nil {
		return service.Result{}, err
	}
	var a answer
	if err := json.Unmarshal(body, &a); err != nil {
		return service.Result{}, fmt.Errorf("%w: %w", service.ErrService, err)
	}
	out := service.Result{Text: strings.TrimSpace(a.Text), Language: whisper.Tag(a.Language)}
	if out.Language == "" {
		out.Language = req.Language
	}
	if req.Translate {
		out.Language = "en"
	}
	if req.Timestamps {
		for _, s := range a.Segments {
			out.Segments = append(out.Segments, service.Segment{Start: seconds(s.Start), End: seconds(s.End), Text: strings.TrimSpace(s.Text)})
		}
	}
	return out, nil
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second)).Round(time.Millisecond)
}

// DetectLanguage implements service.Recognizer, from the audio's first 30
// seconds.
func (r *Recognizer) DetectLanguage(ctx context.Context, audio service.Audio) (string, error) {
	body, err := r.post(ctx, "/detect-language", url.Values{}, audio.First(detectLength))
	if err != nil {
		return "", err
	}
	var a struct {
		LanguageCode     string `json:"language_code"`
		DetectedLanguage string `json:"detected_language"`
	}
	if err := json.Unmarshal(body, &a); err != nil {
		return "", fmt.Errorf("%w: %w", service.ErrService, err)
	}
	if t := whisper.Tag(a.LanguageCode); t != "" {
		return t, nil
	}
	return whisper.Tag(a.DetectedLanguage), nil
}

// post sends the audio as raw PCM to the server's path as audio_file,
// streaming it, and returns the answer.
func (r *Recognizer) post(ctx context.Context, path string, q url.Values, audio service.Audio) ([]byte, error) {
	rc, err := audio.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}
	defer rc.Close()
	q.Set("encode", "false")
	if r.s.PassName && audio.Name != "" {
		q.Set("video_file", audio.Name)
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("audio_file", "audio_file")
		if err == nil {
			_, err = io.Copy(part, rc)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.s.Endpoint+path+"?"+q.Encode(), pr)
	if err != nil {
		pr.CloseWithError(err)
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := r.http.Do(req)
	if err != nil {
		pr.CloseWithError(err)
		return nil, fmt.Errorf("%w: %w", service.ErrService, err)
	}
	return service.Answer(resp)
}

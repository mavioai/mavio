// Package whisperai is the provider of Whisper, ported from Bazarr's
// whisperai: subtitles are transcribed from a video's audio by a
// whisper-asr-webservice server, in the audio's language or translated
// to English. The audio comes from the server's host API, which serves it
// only for videos on local and network storage.
package whisperai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// Name is the provider's name.
const Name = "whisperai"

const (
	defaultEndpoint = "http://127.0.0.1:9000"
	defaultResponse = 5
	defaultTimeout  = 3600
	// userAgent is subliminal's, as Bazarr's provider sends it.
	userAgent = "Subliminal/2.1"
	// detectSeconds is how much audio languages are detected from: Whisper
	// detects them from the first 30 seconds.
	detectSeconds = 30
	// detectedSize bounds the languages detected kept, as Bazarr's
	// lru_cache(2048).
	detectedSize = 2048
)

// Definition returns the provider's definition.
func Definition() provider.Definition {
	var tags []string
	for _, l := range languages {
		if t := tag(l.alpha3); t != "" && !slices.Contains(tags, t) {
			tags = append(tags, t)
		}
	}
	return provider.Definition{
		Name: Name, DisplayName: "Whisper",
		Settings: []provider.Setting{
			{Name: "endpoint", Label: "Whisper ASR Docker Endpoint", Help: "The whisper-asr-webservice server.", Type: provider.Text, Default: defaultEndpoint},
			{Name: "response", Label: "Connection/response timeout in seconds", Type: provider.Number, Default: defaultResponse},
			{Name: "timeout", Label: "Transcription/translation timeout in seconds", Type: provider.Number, Default: defaultTimeout},
			{Name: "pass_video_name", Label: "Pass video filename to Whisper (for logging)", Type: provider.Checkbox, Default: false},
		},
		Languages: tags,
		Movies:    true, Episodes: true,
		New: New,
	}
}

// tag returns the BCP 47 tag of a Whisper language.
func tag(alpha3 string) string { return provider.FromCode(alpha3) }

type settings struct {
	Endpoint      string `json:"endpoint"`
	Response      *int   `json:"response"`
	Timeout       *int   `json:"timeout"`
	PassVideoName bool   `json:"pass_video_name"`
}

// New makes the provider from its settings.
func New(env provider.Env, raw json.RawMessage) (provider.Provider, error) {
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, provider.Errorf(provider.ConfigurationError, "settings: %w", err)
		}
	}
	if s.Endpoint = strings.TrimSuffix(strings.TrimSpace(s.Endpoint), "/"); s.Endpoint == "" {
		s.Endpoint = defaultEndpoint
	}
	if u, err := url.Parse(s.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, provider.Errorf(provider.ConfigurationError, "invalid Whisper endpoint %q", s.Endpoint)
	}
	response, timeout := defaultResponse, defaultTimeout
	if s.Response != nil {
		response = *s.Response
	}
	if s.Timeout != nil {
		timeout = *s.Timeout
	}
	if response <= 0 || timeout <= 0 {
		return nil, provider.Errorf(provider.ConfigurationError, "the timeouts must be positive")
	}
	// The server is reached directly, connecting within the response
	// timeout and answering within the transcription timeout.
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	t.DialContext = (&net.Dialer{Timeout: time.Duration(response) * time.Second}).DialContext
	return &Provider{
		env: env, s: s, http: &http.Client{Transport: t, Timeout: time.Duration(response+timeout) * time.Second},
		detected: map[string]string{},
	}, nil
}

// Provider is Whisper.
type Provider struct {
	env  provider.Env
	s    settings
	http *http.Client

	mu sync.Mutex
	// detected are the languages detected by audio URL, as ISO 639-3
	// codes; "" for those that could not be.
	detected map[string]string
}

// task is a subtitle Search found, as its ID holds it.
type task struct {
	// Task is "transcribe" or "translate".
	Task string `json:"task"`
	// Audio is the audio's language, as an ISO 639-3 code.
	Audio string `json:"audio"`
	// Stream is the audio stream transcribed.
	Stream   int               `json:"stream"`
	Language provider.Language `json:"language"`
}

// Search implements provider.Provider: a subtitle per language Whisper
// can transcribe or translate the audio to.
func (p *Provider) Search(ctx context.Context, v *provider.Video, langs []provider.Language) ([]provider.Subtitle, error) {
	if v.AudioURL == "" || p.env.Host == nil {
		p.env.Log.DebugContext(ctx, "no audio to transcribe", "file", v.FileName)
		return nil, nil
	}
	var out []provider.Subtitle
	for _, lang := range langs {
		t, err := p.query(ctx, v, lang)
		if err != nil {
			return nil, err
		}
		if t == nil {
			continue
		}
		id, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		s := provider.Subtitle{
			ID: string(id), Language: lang, Format: "srt",
			ReleaseNames: []string{fmt.Sprintf("%s %s audio -> %s SRT", t.Task, name(t.Audio), name(provider.Alpha3(lang.Tag)))},
		}
		if v.Episode {
			s.Matches = []string{"series", "season", "episode"}
		} else {
			s.Matches = []string{"title"}
		}
		out = append(out, s)
	}
	return out, nil
}

// query decides how a language is transcribed, as Bazarr's query; nil
// when it is not.
func (p *Provider) query(ctx context.Context, v *provider.Video, lang provider.Language) (*task, error) {
	target := provider.Alpha3(lang.Tag)
	if byAlpha3(target) == nil {
		return nil, nil
	}
	t := &task{Task: "transcribe", Language: lang}
	var tagged []provider.AudioStream
	for _, s := range v.AudioStreams {
		if s.Language != "" {
			tagged = append(tagged, s)
		}
	}
	detect := len(tagged) == 0
	if !detect {
		matched := false
		for _, s := range tagged {
			if mapped(code(s.Language)) == target {
				t.Audio = target
				if len(tagged) > 1 {
					t.Stream = s.Index
				}
				matched = true
				break
			}
		}
		if !matched {
			t.Task, t.Audio = "translate", mapped(code(tagged[0].Language))
		}
		detect = slices.ContainsFunc(tagged, func(s provider.AudioStream) bool {
			return slices.Contains(ambiguousCodes, strings.ToLower(s.Language))
		})
	}
	if detect {
		detected, err := p.detect(ctx, v)
		if err != nil {
			return nil, err
		}
		if detected == "" {
			p.env.Log.InfoContext(ctx, "whisper language detection failed", "file", v.FileName)
			return nil, nil
		}
		t.Audio = mapped(detected)
		t.Task = "transcribe"
		if t.Audio != target {
			t.Task = "translate"
		}
	}
	if t.Task == "translate" && target != "eng" {
		p.env.Log.DebugContext(ctx, "whisper translates only to English", "audio", t.Audio, "language", target)
		return nil, nil
	}
	return t, nil
}

// code returns the ISO 639-3 code of an audio stream's tag; ambiguous
// and unknown tags as they are.
func code(s string) string {
	s = strings.ToLower(s)
	if slices.Contains(ambiguousCodes, s) {
		return s
	}
	if t := provider.FromCode(s); t != "" {
		return provider.Alpha3(t)
	}
	return s
}

// mapped returns the language a language is transcribed as.
func mapped(alpha3 string) string {
	if m, ok := languageMapping[alpha3]; ok {
		return m
	}
	return alpha3
}

func byAlpha3(alpha3 string) *whisperLanguage {
	for i := range languages {
		if languages[i].alpha3 == alpha3 {
			return &languages[i]
		}
	}
	return nil
}

// name returns a language's name, as Bazarr's get_name.
func name(alpha3 string) string {
	if l := byAlpha3(alpha3); l != nil {
		return l.name
	}
	return "None"
}

// detect returns the language of the video's first audio stream as
// Whisper detects it, as an ISO 639-3 code; "" when it cannot be.
func (p *Provider) detect(ctx context.Context, v *provider.Video) (string, error) {
	p.mu.Lock()
	lang, ok := p.detected[v.AudioURL]
	p.mu.Unlock()
	if ok {
		return lang, nil
	}
	body, err := p.post(ctx, v, "/detect-language", url.Values{}, 0, detectSeconds)
	if err != nil {
		if errors.Is(err, errNoAudio) {
			p.remember(v.AudioURL, "")
			return "", nil
		}
		return "", err
	}
	var r struct {
		LanguageCode     string `json:"language_code"`
		DetectedLanguage string `json:"detected_language"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		p.env.Log.ErrorContext(ctx, "invalid JSON response in language detection", "err", err)
		return "", nil
	}
	switch r.LanguageCode {
	case "":
		p.env.Log.InfoContext(ctx, "whisper returned an empty language code")
	case "und":
		p.env.Log.InfoContext(ctx, "whisper detected an undefined language")
	default:
		lang = detected(r.LanguageCode, r.DetectedLanguage)
	}
	p.remember(v.AudioURL, lang)
	return lang, nil
}

func (p *Provider) remember(audioURL, lang string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.detected) >= detectedSize {
		clear(p.detected)
	}
	p.detected[audioURL] = lang
}

// detected returns the ISO 639-3 code of a language Whisper detected, by
// its code or else its name, as Bazarr's _get_language.
func detected(code, langName string) string {
	for _, l := range languages {
		if l.alpha2 == code {
			return l.alpha3
		}
	}
	if t := provider.FromCode(code); t != "" {
		return provider.Alpha3(t)
	}
	if t := provider.FromName(langName); t != "" {
		return provider.Alpha3(t)
	}
	return ""
}

// Download implements provider.Provider: Whisper transcribes the audio
// stream, as SubRip.
func (p *Provider) Download(ctx context.Context, v *provider.Video, id string) (provider.File, error) {
	var t task
	if err := json.Unmarshal([]byte(id), &t); err != nil {
		return provider.File{}, fmt.Errorf("invalid subtitle ID %q: %w", id, err)
	}
	if v.AudioURL == "" || p.env.Host == nil {
		return provider.File{}, errors.New("the video's audio is not served")
	}
	output := t.Audio
	if t.Task != "transcribe" {
		output = "eng"
	}
	input := ""
	if l := byAlpha3(t.Audio); l != nil {
		input = l.alpha2
	}
	if input == "" {
		if output != "eng" {
			return provider.File{}, fmt.Errorf("unsupported audio language %q", t.Audio)
		}
		p.env.Log.InfoContext(ctx, "whisper treating an unsupported audio language as English", "audio", t.Audio)
		input, t.Task = "en", "transcribe"
	}
	p.env.Log.InfoContext(ctx, "whisper starting", "task", t.Task, "to", name(output), "file", v.FileName)
	start := p.env.Now()
	data, err := p.post(ctx, v, "/asr", url.Values{"task": {t.Task}, "language": {input}, "output": {"srt"}}, t.Stream, 0)
	if err != nil {
		return provider.File{}, err
	}
	p.env.Log.InfoContext(ctx, "whisper completed", "task", t.Task, "to", name(output), "took", p.env.Now().Sub(start).Round(time.Second), "file", v.FileName)
	return provider.File{Data: data, Format: "srt", Language: t.Language}, nil
}

// errNoAudio is a video whose audio stream cannot be served.
var errNoAudio = errors.New("missing or bad audio stream")

// post sends an audio stream, or its first seconds when seconds is
// positive, to the server's path as audio_file, streaming it from the
// host API, and returns the answer.
func (p *Provider) post(ctx context.Context, v *provider.Video, path string, q url.Values, stream, seconds int) ([]byte, error) {
	au, err := url.Parse(v.AudioURL)
	if err != nil {
		return nil, fmt.Errorf("audio URL: %w", err)
	}
	aq := au.Query()
	aq.Set("stream", strconv.Itoa(stream))
	if seconds > 0 {
		aq.Set("seconds", strconv.Itoa(seconds))
	}
	au.RawQuery = aq.Encode()
	areq, err := http.NewRequestWithContext(ctx, http.MethodGet, au.String(), nil)
	if err != nil {
		return nil, err
	}
	audio, err := p.env.Host.Do(areq)
	if err != nil {
		return nil, fmt.Errorf("audio: %w", err)
	}
	defer audio.Body.Close()
	if audio.StatusCode != http.StatusOK {
		p.env.Log.InfoContext(ctx, "whisper cannot process the video because of a missing or bad audio stream", "status", audio.Status, "file", v.FileName)
		return nil, fmt.Errorf("%w: %s", errNoAudio, audio.Status)
	}

	q.Set("encode", "false")
	if p.s.PassVideoName {
		q.Set("video_file", v.FileName)
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreateFormFile("audio_file", "audio_file")
		if err == nil {
			_, err = io.Copy(part, audio.Body)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.s.Endpoint+path+"?"+q.Encode(), pr)
	if err != nil {
		pr.CloseWithError(err)
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", userAgent)
	resp, err := p.http.Do(req)
	if err != nil {
		pr.CloseWithError(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, provider.Errorf(provider.ServiceUnavailable, "whisper answered %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

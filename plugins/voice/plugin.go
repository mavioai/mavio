package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/voice/internal/openai"
	"github.com/mavioai/mavio/plugins/voice/internal/service"
	"github.com/mavioai/mavio/plugins/voice/internal/whisperasr"
)

// config is the plugin's configuration.
type config struct {
	Recognizers  []entry `json:"recognizers"`
	Synthesizers []entry `json:"synthesizers"`
}

// entry is a recognizer's or synthesizer's configuration.
type entry struct {
	Name     string          `json:"name"`
	Enabled  bool            `json:"enabled"`
	Settings json.RawMessage `json:"settings"`
}

// subtitlesSetting lets a recognizer with timestamps transcribe subtitles.
var subtitlesSetting = service.Setting{
	Name: "subtitles", Label: "Transcribe subtitles", Type: service.Checkbox, Default: false,
	Help: "Transcribe videos into the subtitles they are missing, after the subtitle providers.",
}

// recognizer is an enabled recognizer.
type recognizer struct {
	def       *service.RecognizerDef
	r         service.Recognizer
	subtitles bool
	// err is why it could not be made from its settings.
	err error
}

// synthesizer is an enabled synthesizer.
type synthesizer struct {
	def *service.SynthesizerDef
	s   service.Synthesizer
	err error
}

// plugin implements the plugin's services.
type plugin struct {
	log     *slog.Logger
	version string
	// http reaches the services.
	http *http.Client
	// host returns the client reaching the server's host API.
	host         func() *http.Client
	recognizers  []service.RecognizerDef
	synthesizers []service.SynthesizerDef

	mu          sync.RWMutex
	recognizing map[string]*recognizer
	speaking    map[string]*synthesizer
}

func newPlugin(log *slog.Logger, version string, hc *http.Client, host func() *http.Client) *plugin {
	return &plugin{
		log: log, version: version, http: hc, host: host,
		recognizers:  []service.RecognizerDef{whisperasr.Definition(), openai.RecognizerDefinition()},
		synthesizers: []service.SynthesizerDef{openai.SynthesizerDefinition()},
		recognizing:  map[string]*recognizer{}, speaking: map[string]*synthesizer{},
	}
}

// Describe implements PluginService.
func (p *plugin) Describe(context.Context, *pluginv1.DescribeRequest) (*pluginv1.DescribeResponse, error) {
	m := pluginv1.Manifest_builder{Id: proto.String(ID), Version: proto.String(p.version)}.Build()
	return pluginv1.DescribeResponse_builder{Manifest: m}.Build(), nil
}

// Health implements PluginService.
func (p *plugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true)}.Build(), nil
}

// Shutdown implements PluginService.
func (p *plugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	return &pluginv1.ShutdownResponse{}, nil
}

// Configure implements PluginService: the recognizers and synthesizers
// it enables replace those enabled.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	var c config
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &c); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	env := func(kind, name string) service.Env {
		return service.Env{HTTP: p.http, Log: p.log.With(kind, name)}
	}
	recognizing, seen := map[string]*recognizer{}, map[string]bool{}
	for i, e := range c.Recognizers {
		def := find(p.recognizers, func(d service.RecognizerDef) string { return d.Name }, e.Name)
		if err := check(def == nil, seen[e.Name], "recognizers", i, e.Name); err != nil {
			return nil, err
		}
		seen[e.Name] = true
		if !e.Enabled {
			continue
		}
		var s struct {
			Subtitles bool `json:"subtitles"`
		}
		_ = service.Decode(e.Settings, &s)
		r := &recognizer{def: def, subtitles: s.Subtitles}
		r.r, r.err = def.New(env("recognizer", def.Name), e.Settings)
		if r.err != nil {
			p.log.Warn("configure recognizer", "recognizer", def.Name, "err", r.err)
		}
		recognizing[e.Name] = r
	}
	speaking := map[string]*synthesizer{}
	clear(seen)
	for i, e := range c.Synthesizers {
		def := find(p.synthesizers, func(d service.SynthesizerDef) string { return d.Name }, e.Name)
		if err := check(def == nil, seen[e.Name], "synthesizers", i, e.Name); err != nil {
			return nil, err
		}
		seen[e.Name] = true
		if !e.Enabled {
			continue
		}
		s := &synthesizer{def: def}
		s.s, s.err = def.New(env("synthesizer", def.Name), e.Settings)
		if s.err != nil {
			p.log.Warn("configure synthesizer", "synthesizer", def.Name, "err", s.err)
		}
		speaking[e.Name] = s
	}
	p.mu.Lock()
	p.recognizing, p.speaking = recognizing, speaking
	p.mu.Unlock()
	return &pluginv1.ConfigureResponse{}, nil
}

func find[T any](defs []T, name func(T) string, n string) *T {
	for i := range defs {
		if name(defs[i]) == n {
			return &defs[i]
		}
	}
	return nil
}

func check(unknown, repeated bool, list string, i int, name string) error {
	switch {
	case unknown:
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s[%d]: unknown %q", list, i, name))
	case repeated:
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s[%d]: %q is repeated", list, i, name))
	}
	return nil
}

// ListRecognizers implements SpeechToTextService.
func (p *plugin) ListRecognizers(context.Context, *pluginv1.ListRecognizersRequest) (*pluginv1.ListRecognizersResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*pluginv1.RecognizerInfo
	for i := range p.recognizers {
		def := &p.recognizers[i]
		caps := def.Caps
		r := p.recognizing[def.Name]
		enabled := r != nil && r.err == nil
		if enabled {
			caps = r.r.Caps()
		}
		settings := def.Settings
		if def.Caps.Timestamps {
			settings = append(settings[:len(settings):len(settings)], subtitlesSetting)
		}
		b := pluginv1.RecognizerInfo_builder{
			Name: proto.String(def.Name), DisplayName: proto.String(def.DisplayName), Settings: settingFields(settings),
			Enabled: proto.Bool(enabled), Languages: caps.Languages, Timestamps: proto.Bool(caps.Timestamps),
			DetectsLanguage: proto.Bool(caps.Detects), Translates: proto.Bool(caps.Translates), Hints: proto.Bool(caps.Hints),
			Subtitles: proto.Bool(enabled && caps.Timestamps && r.subtitles),
		}
		if caps.MaxDuration > 0 {
			b.MaxDuration = durationpb.New(caps.MaxDuration)
		}
		out = append(out, b.Build())
	}
	return pluginv1.ListRecognizersResponse_builder{Recognizers: out}.Build(), nil
}

// lookupRecognizer returns an enabled recognizer.
func (p *plugin) lookupRecognizer(name string) (service.Recognizer, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	r := p.recognizing[name]
	switch {
	case r == nil:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("recognizer %q is not enabled", name))
	case r.err != nil:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("recognizer %q: %w", name, r.err))
	}
	return r.r, nil
}

// audio returns the audio of a request.
func (p *plugin) audio(a *pluginv1.SpeechAudio, name string) (service.Audio, error) {
	out := service.Audio{Name: name}
	switch {
	case a.HasPcm():
		pcm := a.GetPcm()
		out.Open = func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(pcm)), nil }
	case a.HasUrl():
		var host *http.Client
		if p.host != nil {
			host = p.host()
		}
		if host == nil {
			return out, connect.NewError(connect.CodeFailedPrecondition, errors.New("the host API is not reachable"))
		}
		u := a.GetUrl()
		out.Open = func(ctx context.Context) (io.ReadCloser, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				return nil, err
			}
			resp, err := host.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, fmt.Errorf("the host API answered %s", resp.Status)
			}
			return resp.Body, nil
		}
	default:
		return out, connect.NewError(connect.CodeInvalidArgument, errors.New("no audio"))
	}
	if a.HasLimit() {
		out = out.First(a.GetLimit().AsDuration())
	}
	return out, nil
}

// RecognizeSpeech implements SpeechToTextService.
func (p *plugin) RecognizeSpeech(ctx context.Context, req *pluginv1.RecognizeSpeechRequest) (*pluginv1.RecognizeSpeechResponse, error) {
	r, err := p.lookupRecognizer(req.GetRecognizer())
	if err != nil {
		return nil, err
	}
	audio, err := p.audio(req.GetAudio(), req.GetName())
	if err != nil {
		return nil, err
	}
	res, err := r.Recognize(ctx, service.Request{
		Audio: audio, Language: req.GetLanguage(), Translate: req.GetTranslate(), Hints: req.GetHints(), Timestamps: req.GetTimestamps(),
	})
	if err != nil {
		return nil, serviceError(err)
	}
	segments := make([]*pluginv1.SpeechSegment, len(res.Segments))
	for i, s := range res.Segments {
		segments[i] = pluginv1.SpeechSegment_builder{
			Start: durationpb.New(s.Start), End: durationpb.New(s.End), Text: proto.String(s.Text),
		}.Build()
	}
	return pluginv1.RecognizeSpeechResponse_builder{
		Text: proto.String(res.Text), Language: proto.String(res.Language), Segments: segments,
	}.Build(), nil
}

// DetectSpeechLanguage implements SpeechToTextService.
func (p *plugin) DetectSpeechLanguage(ctx context.Context, req *pluginv1.DetectSpeechLanguageRequest) (*pluginv1.DetectSpeechLanguageResponse, error) {
	r, err := p.lookupRecognizer(req.GetRecognizer())
	if err != nil {
		return nil, err
	}
	if !r.Caps().Detects {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("recognizer %q does not detect languages", req.GetRecognizer()))
	}
	audio, err := p.audio(req.GetAudio(), "")
	if err != nil {
		return nil, err
	}
	lang, err := r.DetectLanguage(ctx, audio)
	if err != nil {
		return nil, serviceError(err)
	}
	return pluginv1.DetectSpeechLanguageResponse_builder{Language: proto.String(lang)}.Build(), nil
}

// serviceError returns a service's error as a Connect error.
func serviceError(err error) error {
	switch {
	case errors.Is(err, service.ErrConfig):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, service.ErrService):
		return connect.NewError(connect.CodeUnavailable, err)
	}
	return err
}

// ListVoices implements TextToSpeechService.
func (p *plugin) ListVoices(context.Context, *pluginv1.ListVoicesRequest) (*pluginv1.ListVoicesResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var voices []*pluginv1.VoiceInfo
	var synths []*pluginv1.SynthesizerInfo
	for i := range p.synthesizers {
		def := &p.synthesizers[i]
		s := p.speaking[def.Name]
		enabled := s != nil && s.err == nil
		synths = append(synths, pluginv1.SynthesizerInfo_builder{
			Name: proto.String(def.Name), DisplayName: proto.String(def.DisplayName),
			Settings: settingFields(def.Settings), Enabled: proto.Bool(enabled),
		}.Build())
		if !enabled {
			continue
		}
		for _, v := range s.s.Voices() {
			voices = append(voices, pluginv1.VoiceInfo_builder{
				Id: proto.String(def.Name + "/" + v.ID), Name: proto.String(v.Name), Languages: v.Languages,
			}.Build())
		}
	}
	return pluginv1.ListVoicesResponse_builder{Voices: voices, Synthesizers: synths}.Build(), nil
}

// SynthesizeSpeech implements TextToSpeechService.
func (p *plugin) SynthesizeSpeech(ctx context.Context, req *pluginv1.SynthesizeSpeechRequest) (*pluginv1.SynthesizeSpeechResponse, error) {
	name, voice, ok := strings.Cut(req.GetVoice(), "/")
	if !ok || name == "" || voice == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("voice %q names no synthesizer", req.GetVoice()))
	}
	p.mu.RLock()
	s := p.speaking[name]
	p.mu.RUnlock()
	switch {
	case s == nil:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("synthesizer %q is not enabled", name))
	case s.err != nil:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("synthesizer %q: %w", name, s.err))
	}
	format := service.OggOpus
	if req.GetFormat() == pluginv1.SpeechFormat_SPEECH_FORMAT_MP3 {
		format = service.MP3
	}
	speech, err := s.s.Synthesize(ctx, voice, req.GetText(), req.GetLanguage(), format, req.GetSpeed())
	if err != nil {
		return nil, serviceError(err)
	}
	return pluginv1.SynthesizeSpeechResponse_builder{Audio: speech.Audio, ContentType: proto.String(speech.ContentType)}.Build(), nil
}

// settingFields describes settings.
func settingFields(settings []service.Setting) []*pluginv1.SettingField {
	var out []*pluginv1.SettingField
	for _, s := range settings {
		f := pluginv1.SettingField_builder{
			Name: proto.String(s.Name), Label: proto.String(s.Label), HelpText: proto.String(s.Help),
			Advanced: proto.Bool(s.Advanced), Secret: proto.Bool(s.Type == service.Password),
		}.Build()
		switch s.Type {
		case service.Text, service.Password:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TEXT)
		case service.Number:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER)
		case service.Checkbox:
			f.SetType(pluginv1.SettingFieldType_SETTING_FIELD_TYPE_CHECKBOX)
		}
		if s.Default != nil {
			if data, err := json.Marshal(s.Default); err == nil {
				f.SetDefaultJson(string(data))
			}
		}
		out = append(out, f)
	}
	return out
}

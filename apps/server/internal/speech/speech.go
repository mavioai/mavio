package speech

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// RecognizerPlugin is a started SPEECH_TO_TEXT plugin.
type RecognizerPlugin struct {
	ID     string
	Client pluginv1connect.SpeechToTextServiceClient
}

// VoicePlugin is a started TEXT_TO_SPEECH plugin.
type VoicePlugin struct {
	ID     string
	Client pluginv1connect.TextToSpeechServiceClient
}

// Recognizer is an enabled recognizer of a plugin.
type Recognizer struct {
	// ID is "<plugin>/<name>".
	ID     string
	Plugin RecognizerPlugin
	Info   *pluginv1.RecognizerInfo
}

// Voice is a voice of an enabled synthesizer.
type Voice struct {
	// ID is "<plugin>/<synthesizer>/<voice>".
	ID     string
	Plugin VoicePlugin
	Info   *pluginv1.VoiceInfo
}

// ErrNone is a request no recognizer or voice can serve.
var ErrNone = errors.New("no recognizer or voice can serve it")

// Recognizers lists the enabled recognizers of plugins, in their order;
// plugins that fail to list them are logged and left out.
func Recognizers(ctx context.Context, plugins []RecognizerPlugin, log *slog.Logger) []Recognizer {
	var out []Recognizer
	for _, p := range plugins {
		resp, err := p.Client.ListRecognizers(ctx, &pluginv1.ListRecognizersRequest{})
		if err != nil {
			log.WarnContext(ctx, "list recognizers", "plugin", p.ID, "err", err)
			continue
		}
		for _, r := range resp.GetRecognizers() {
			if r.GetEnabled() {
				out = append(out, Recognizer{ID: p.ID + "/" + r.GetName(), Plugin: p, Info: r})
			}
		}
	}
	return out
}

// Voices lists the voices of plugins' enabled synthesizers.
func Voices(ctx context.Context, plugins []VoicePlugin, log *slog.Logger) []Voice {
	var out []Voice
	for _, p := range plugins {
		resp, err := p.Client.ListVoices(ctx, &pluginv1.ListVoicesRequest{})
		if err != nil {
			log.WarnContext(ctx, "list voices", "plugin", p.ID, "err", err)
			continue
		}
		enabled := map[string]bool{}
		for _, s := range resp.GetSynthesizers() {
			enabled[s.GetName()] = s.GetEnabled()
		}
		for _, v := range resp.GetVoices() {
			synth, _, _ := strings.Cut(v.GetId(), "/")
			if on, known := enabled[synth]; known && !on {
				continue
			}
			out = append(out, Voice{ID: p.ID + "/" + v.GetId(), Plugin: p, Info: v})
		}
	}
	return out
}

// Speaks reports whether a recognizer or voice of languages takes a BCP 47
// language: any when it lists none, else one of the same base language.
func Speaks(languages []string, language string) bool {
	if len(languages) == 0 || language == "" {
		return true
	}
	b := Base(language)
	for _, l := range languages {
		if Base(l) == b {
			return true
		}
	}
	return false
}

// Base returns the language of a BCP 47 tag without its script and region,
// in lower case.
func Base(tag string) string {
	b, _, _ := strings.Cut(strings.ReplaceAll(tag, "_", "-"), "-")
	return strings.ToLower(b)
}

// Pick returns the recognizer of id, or with id empty the first for the
// language that suits need, or ErrNone.
func Pick(list []Recognizer, id, language string, need func(*pluginv1.RecognizerInfo) bool) (Recognizer, error) {
	for _, r := range list {
		if id != "" && r.ID != id {
			continue
		}
		if !Speaks(r.Info.GetLanguages(), language) || (need != nil && !need(r.Info)) {
			if id != "" {
				return Recognizer{}, fmt.Errorf("%w: recognizer %s does not take %q", ErrNone, id, language)
			}
			continue
		}
		return r, nil
	}
	if id != "" {
		return Recognizer{}, fmt.Errorf("%w: no recognizer %s", ErrNone, id)
	}
	return Recognizer{}, ErrNone
}

// Recognize recognizes audio with a recognizer.
func (r Recognizer) Recognize(ctx context.Context, req *pluginv1.RecognizeSpeechRequest) (*pluginv1.RecognizeSpeechResponse, error) {
	_, name, _ := strings.Cut(r.ID, "/")
	req.SetRecognizer(name)
	return r.Plugin.Client.RecognizeSpeech(ctx, req)
}

// DetectLanguage tells the language spoken in audio.
func (r Recognizer) DetectLanguage(ctx context.Context, audio *pluginv1.SpeechAudio) (string, error) {
	_, name, _ := strings.Cut(r.ID, "/")
	resp, err := r.Plugin.Client.DetectSpeechLanguage(ctx, pluginv1.DetectSpeechLanguageRequest_builder{Recognizer: &name, Audio: audio}.Build())
	return resp.GetLanguage(), err
}

// Synthesize speaks text with a voice.
func (v Voice) Synthesize(ctx context.Context, text, language string, format pluginv1.SpeechFormat) (*pluginv1.SynthesizeSpeechResponse, error) {
	_, id, _ := strings.Cut(v.ID, "/")
	return v.Plugin.Client.SynthesizeSpeech(ctx, pluginv1.SynthesizeSpeechRequest_builder{
		Voice: &id, Text: &text, Language: &language, Format: &format,
	}.Build())
}

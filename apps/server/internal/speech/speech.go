package speech

import (
	"context"
	"log/slog"
	"strings"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Plugin is a started SPEECH_TO_TEXT plugin.
type Plugin struct {
	ID     string
	Client pluginv1connect.SpeechToTextServiceClient
}

// Transcriber is an enabled and ready transcriber of a plugin.
type Transcriber struct {
	// ID is "<plugin>/<transcriber>".
	ID     string
	Plugin Plugin
	Info   *pluginv1.TranscriberInfo
}

// Transcribers lists the enabled and ready transcribers of plugins, in
// their order; plugins that fail to list them are logged and left out.
func Transcribers(ctx context.Context, plugins []Plugin, log *slog.Logger) []Transcriber {
	var out []Transcriber
	for _, p := range plugins {
		resp, err := p.Client.ListTranscribers(ctx, &pluginv1.ListTranscribersRequest{})
		if err != nil {
			log.WarnContext(ctx, "list transcribers", "plugin", p.ID, "err", err)
			continue
		}
		for _, t := range resp.GetTranscribers() {
			if t.GetEnabled() && t.GetReady() {
				out = append(out, Transcriber{ID: p.ID + "/" + t.GetId(), Plugin: p, Info: t})
			}
		}
	}
	return out
}

// Speaks reports whether a transcriber of languages takes a BCP 47
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

// Transcribe transcribes audio with a transcriber.
func (t Transcriber) Transcribe(ctx context.Context, req *pluginv1.TranscribeSpeechRequest) (*pluginv1.TranscribeSpeechResponse, error) {
	req.SetTranscriber(t.Info.GetId())
	return t.Plugin.Client.TranscribeSpeech(ctx, req)
}

// DetectLanguage tells the language spoken in audio.
func (t Transcriber) DetectLanguage(ctx context.Context, audio *pluginv1.SpeechAudio) (string, error) {
	resp, err := t.Plugin.Client.DetectSpeechLanguage(ctx, pluginv1.DetectSpeechLanguageRequest_builder{
		Transcriber: new(t.Info.GetId()), Audio: audio,
	}.Build())
	return resp.GetLanguage(), err
}

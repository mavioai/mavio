package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/translator"
)

func TestTranslators(t *testing.T) {
	h := newHarness(t)
	list, err := h.p.ListSubtitleTranslators(t.Context(), &pluginv1.ListSubtitleTranslatorsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tr := range list.GetTranslators() {
		names = append(names, tr.GetName())
		if tr.GetEnabled() {
			t.Errorf("translator %s enabled before being configured", tr.GetName())
		}
	}
	if strings.Join(names, ",") != "google_translate,gemini,lingarr" {
		t.Errorf("translators = %v, want Bazarr's", names)
	}

	h.configure(t, `{"translators":[
		{"name":"google_translate","enabled":true},
		{"name":"gemini","enabled":true,"settings":{"api_keys":"k","batch_size":50}},
		{"name":"lingarr","enabled":false}]}`)
	list, err = h.p.ListSubtitleTranslators(t.Context(), &pluginv1.ListSubtitleTranslatorsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	g, gem, l := list.GetTranslators()[0], list.GetTranslators()[1], list.GetTranslators()[2]
	if !g.GetEnabled() || g.GetMarkup() || g.GetBatchSize() != 0 || !gem.GetEnabled() || !gem.GetMarkup() || gem.GetBatchSize() != 50 ||
		l.GetEnabled() || len(gem.GetSettings()) != 3 || !gem.GetSettings()[0].GetSecret() {
		t.Errorf("translators = %v, want Google and Gemini enabled", list.GetTranslators())
	}

	// A translator not enabled, or whose settings are wrong, fails.
	h.configure(t, `{"translators":[{"name":"gemini","enabled":true}]}`)
	for _, name := range []string{"lingarr", "gemini"} {
		_, err := h.p.TranslateSubtitleLines(t.Context(), pluginv1.TranslateSubtitleLinesRequest_builder{
			Translator: proto.String(name), SourceLanguage: proto.String("en"), TargetLanguage: proto.String("fr"), Lines: []string{"a"},
		}.Build())
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("TranslateSubtitleLines(%s): err = %v, want failed precondition", name, err)
		}
	}
	for _, cfg := range []string{`{"translators":[{"name":"deepl"}]}`, `{"translators":[{"name":"gemini"},{"name":"gemini"}]}`} {
		if _, err := h.p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(cfg)}.Build()); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("Configure(%s): err = %v, want invalid argument", cfg, err)
		}
	}
}

// fakeTranslator upper-cases lines, or fails.
type fakeTranslator struct{ err error }

func (f *fakeTranslator) Translate(_ context.Context, req *translator.Request) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, l := range req.Lines {
		out = append(out, strings.ToUpper(l)+" "+req.Media.Title)
	}
	return out, nil
}

func (*fakeTranslator) Credit() string { return "Fake" }

func (*fakeTranslator) BatchSize() int { return 0 }

func TestTranslateSubtitleLines(t *testing.T) {
	h := newHarness(t)
	f := &fakeTranslator{}
	h.p.translatorDefs = []translator.Definition{{Name: "fake", DisplayName: "Fake", New: func(provider.Env, json.RawMessage) (translator.Translator, error) { return f, nil }}}
	h.p.translatorByName = map[string]*translator.Definition{"fake": &h.p.translatorDefs[0]}
	h.configure(t, `{"translators":[{"name":"fake","enabled":true}]}`)
	req := pluginv1.TranslateSubtitleLinesRequest_builder{
		Translator: proto.String("fake"), SourceLanguage: proto.String("en"), TargetLanguage: proto.String("fr"), Lines: []string{"a", "b"},
		Media: pluginv1.SubtitleTranslationMedia_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_MOVIE.Enum(), Title: proto.String("Up")}.Build(),
	}.Build()
	resp, err := h.p.TranslateSubtitleLines(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(resp.GetLines(), "|") != "A Up|B Up" || resp.GetCredit() != "Fake" {
		t.Errorf("response = %v, want the lines translated and credited", resp)
	}
	f.err = provider.Errorf(provider.TooManyRequests, "slow down")
	if _, err := h.p.TranslateSubtitleLines(t.Context(), req); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("TranslateSubtitleLines rate limited: err = %v, want resource exhausted", err)
	}
}

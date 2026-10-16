package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/plugins/voice/internal/service"
)

// services is a whisper-asr-webservice and an OpenAI speech server, and
// the host API serving ten seconds of audio.
func services(t *testing.T) (*httptest.Server, *[]int) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/subtitles/audio/tok":
			_, _ = w.Write(make([]byte, 10*service.BytesPerSecond))
		case "/asr", "/detect-language":
			file, _, err := r.FormFile("audio_file")
			if err != nil {
				t.Fatalf("audio_file: %v", err)
			}
			n, _ := io.Copy(io.Discard, file)
			sizes = append(sizes, int(n))
			if r.URL.Path == "/detect-language" {
				_, _ = io.WriteString(w, `{"language_code":"fr"}`)
				return
			}
			_, _ = io.WriteString(w, `{"text":"Bonjour.","language":"fr","segments":[{"start":1,"end":2,"text":"Bonjour."}]}`)
		case "/v1/audio/speech":
			_, _ = io.WriteString(w, "OggS")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &sizes
}

func TestPlugin(t *testing.T) {
	srv, sizes := services(t)
	p := newPlugin(slog.New(slog.DiscardHandler), "0.1.0", srv.Client(), srv.Client)
	ctx := t.Context()
	configure := func(c string) error {
		_, err := p.Configure(ctx, pluginv1.ConfigureRequest_builder{ConfigJson: proto.String(c)}.Build())
		return err
	}
	if err := configure(`{"recognizers":[{"name":"whisper-asr","enabled":true,"settings":{"endpoint":"` + srv.URL + `","subtitles":true}},
		{"name":"openai","enabled":false}],
		"synthesizers":[{"name":"openai","enabled":true,"settings":{"base_url":"` + srv.URL + `/v1","voices":"nova"}}]}`); err != nil {
		t.Fatal(err)
	}

	list, err := p.ListRecognizers(ctx, &pluginv1.ListRecognizersRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range list.GetRecognizers() {
		names := []string{}
		for _, s := range r.GetSettings() {
			names = append(names, s.GetName())
		}
		got = append(got, r.GetName())
		if r.GetName() == "whisper-asr" && (!r.GetEnabled() || !r.GetSubtitles() || !r.GetTimestamps() || !strings.Contains(strings.Join(names, ","), "subtitles")) {
			t.Errorf("whisper-asr = %v, want enabled for subtitles", r)
		}
		if r.GetName() == "openai" && (r.GetEnabled() || r.GetSubtitles()) {
			t.Errorf("openai = %v, want disabled", r)
		}
	}
	if strings.Join(got, ",") != "whisper-asr,openai" {
		t.Errorf("recognizers = %q", got)
	}

	// The host API's audio is fetched, its first four seconds only.
	resp, err := p.RecognizeSpeech(ctx, pluginv1.RecognizeSpeechRequest_builder{
		Recognizer: proto.String("whisper-asr"), Timestamps: proto.Bool(true),
		Audio: pluginv1.SpeechAudio_builder{Url: proto.String(srv.URL + "/subtitles/audio/tok"), Limit: durationpb.New(4 * time.Second)}.Build(),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if s := resp.GetSegments(); resp.GetLanguage() != "fr" || len(s) != 1 || s[0].GetStart().AsDuration() != time.Second || s[0].GetText() != "Bonjour." {
		t.Errorf("RecognizeSpeech = %v", resp)
	}
	detected, err := p.DetectSpeechLanguage(ctx, pluginv1.DetectSpeechLanguageRequest_builder{
		Recognizer: proto.String("whisper-asr"), Audio: pluginv1.SpeechAudio_builder{Pcm: make([]byte, 100)}.Build(),
	}.Build())
	if err != nil || detected.GetLanguage() != "fr" {
		t.Errorf("DetectSpeechLanguage = %v, %v; want fr", detected, err)
	}
	if want := []int{4 * service.BytesPerSecond, 100}; len(*sizes) != 2 || (*sizes)[0] != want[0] || (*sizes)[1] != want[1] {
		t.Errorf("audio sent = %v, want = %v", *sizes, want)
	}
	_, err = p.RecognizeSpeech(ctx, pluginv1.RecognizeSpeechRequest_builder{
		Recognizer: proto.String("openai"), Audio: pluginv1.SpeechAudio_builder{Pcm: []byte{0, 0}}.Build(),
	}.Build())
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("RecognizeSpeech with a disabled recognizer: err = %v, want FailedPrecondition", err)
	}

	voices, err := p.ListVoices(ctx, &pluginv1.ListVoicesRequest{})
	if err != nil || len(voices.GetVoices()) != 1 || voices.GetVoices()[0].GetId() != "openai/nova" {
		t.Fatalf("ListVoices = %v, %v; want openai/nova", voices, err)
	}
	speech, err := p.SynthesizeSpeech(ctx, pluginv1.SynthesizeSpeechRequest_builder{Voice: proto.String("openai/nova"), Text: proto.String("Bonjour.")}.Build())
	if err != nil || speech.GetContentType() != "audio/ogg" || string(speech.GetAudio()) != "OggS" {
		t.Errorf("SynthesizeSpeech = %v, %v", speech, err)
	}

	for _, c := range []string{`{"recognizers":[{"name":"vosk"}]}`, `{"recognizers":[{"name":"openai"},{"name":"openai"}]}`, `{`} {
		if err := configure(c); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("Configure(%s): err = %v, want InvalidArgument", c, err)
		}
	}
}

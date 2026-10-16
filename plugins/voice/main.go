// Command voice is Mavio's voice plugin, a process plugin connecting to
// the speech services the administrator configures: it recognizes speech
// with whisper-asr-webservice and servers of the OpenAI-compatible
// transcription API, as the SPEECH_TO_TEXT capability, for the assistant
// and for subtitles transcribed from videos, and speaks with servers of
// the OpenAI-compatible speech API, as the TEXT_TO_SPEECH capability.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/guest/process"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID is the plugin's ID, as its manifest gives it.
const ID = "dev.mavio.voice"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newPlugin(log, Version, &http.Client{}, guest.HostClient)
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewSpeechToTextServiceHandler(p))
	guest.Handle(pluginv1connect.NewTextToSpeechServiceHandler(p))
	if err := process.Serve(ctx, ID); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

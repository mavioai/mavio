package app

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
	"github.com/mavioai/mavio/apps/desktop/internal/voice"
)

// newVoiceHarness is a harness whose app has voice sessions, on srv.
func newVoiceHarness(t *testing.T, audio *fakeAudio) (*harness, *fakeVoiceServer) {
	h := newHarness(t, true, 1280, 800)
	srv := newFakeVoiceServer(t)
	h.core.assistant.voiceURL = srv.url
	h.app.opts.Voice = audio
	h.settle()
	return h, srv
}

func TestAssistantVoice(t *testing.T) {
	audio := &fakeAudio{}
	h, srv := newVoiceHarness(t, audio)

	// A tap starts a conversation: loading until the microphone hears.
	if err := h.tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	h.until("the microphone opened", func() bool { return audio.device() != nil })
	h.until("loading", h.shows("Loading…"))
	dev := audio.device()
	dev.say(1, true)
	h.until("listening", h.shows("Listening…"))
	h.until("connected", func() bool { return len(h.core.assistant.voiceRequests()) == 1 })
	req := h.core.assistant.voiceRequests()[0]
	if got := req.GetMode(); got != pluginv1.VoiceMode_VOICE_MODE_CONVERSATION {
		t.Errorf("mode = %v, want = conversation", got)
	}
	srv.event(voiceEvent{"type": "ready", "output_sample_rate": 24000.0})
	// What was said before the socket opened is sent.
	deadline := time.Now().Add(5 * time.Second)
	for n := 0; n < 1; n += srv.frames() {
		if time.Now().After(deadline) {
			t.Fatal("the microphone's frames were not sent")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A turn: what is heard, the tools run, the action carried out and the
	// answer spoken.
	srv.event(voiceEvent{"type": "user_started"})
	srv.event(voiceEvent{"type": "transcript", "text": "play the long", "final": false})
	h.until("heard", h.shows("play the long"))
	srv.event(voiceEvent{"type": "transcript", "text": "play the long orbit", "final": true})
	srv.event(voiceEvent{"type": "run", "text": "Play The Long Orbit"})
	srv.event(voiceEvent{"type": "tool", "id": "t1", "summary": "Searched the library"})
	h.until("ran", h.shows("Play The Long Orbit", "Searched the library"))
	srv.event(voiceEvent{"type": "tool", "id": "t1", "summary": "Searched the library", "done": true})
	srv.event(voiceEvent{"type": "action", "action": map[string]any{"id": "a1", "play": map[string]any{"itemIds": []string{"orbit"}}}})
	h.until("played", func() bool { p := h.app.playerOf(nil); return p != nil && p.item == "orbit" })
	srv.event(voiceEvent{"type": "bot_started"})
	srv.speak(2400)
	srv.event(voiceEvent{"type": "answer", "text": "Playing The Long Orbit."})
	h.until("answered", h.shows("Playing The Long Orbit."))
	h.save(shots, "assistant-voice")
	h.until("spoken", func() bool { played, _, _, _ := dev.state(); return played == 2400 })
	if _, runs, _, _ := dev.state(); runs != 1 {
		t.Errorf("runs = %d, want = 1", runs)
	}

	// An interrupt drops the answer's speech and tells how much played.
	srv.event(voiceEvent{"type": "interrupt"})
	if got := srv.next(t, "played").num("ms"); got != 300 {
		t.Errorf("played = %v ms, want = 300", got)
	}
	h.until("interrupted", func() bool { return h.app.assistant.interrupts == 1 })
	srv.event(voiceEvent{"type": "bot_stopped"})
	srv.event(voiceEvent{"type": "end", "answer": "Playing The Long Orbit.", "status": "completed"})
	h.until("ended", func() bool { return !h.app.assistant.working && !h.app.assistant.speaking })

	// A confirmation is answered by the cards' buttons.
	srv.event(voiceEvent{"type": "confirm", "id": "c1", "text": "Request Dune?"})
	h.until("confirm", h.shows("Request Dune?", "Confirm"))
	h.save(shots, "assistant-voice-confirm")
	asked := len(h.core.assistant.requests())
	if err := h.tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	h.until("cancelled", func() bool { return len(h.core.assistant.requests()) > asked })
	if a := h.core.assistant.requests()[asked].GetAnswer(); a.GetId() != "c1" || a.GetConfirmed() {
		t.Errorf("answer = %v, want = c1 not confirmed", a)
	}

	// Escape ends the session, telling the server, and the orb goes.
	h.tt.Key(0, ui.KeyEscape)
	srv.next(t, "close")
	h.until("closed", func() bool { _, _, _, closed := dev.state(); return closed })
	h.until("gone", func() bool { return !h.app.assistant.open })
	h.settle()
	if h.tt.HasText("Playing The Long Orbit.") {
		t.Errorf("the cards stayed after Escape")
	}
}

// The player's microphone starts a conversation as the assistant's button does.
func TestAssistantVoicePlayer(t *testing.T) {
	audio := &fakeAudio{}
	h, _ := newVoiceHarness(t, audio)
	h.app.play(h.core.s.Server.ID, "atlas-s2e3", nil)
	h.settle()
	h.video.emit(mpv.State{Loaded: true, Position: time.Minute, Duration: 42 * time.Minute, Speed: 1, Volume: 100, Restarts: 1})
	h.settle()
	if err := h.tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	h.until("the microphone opened", func() bool { return audio.device() != nil })
	audio.device().say(1, true)
	h.until("connected", func() bool { return len(h.core.assistant.voiceRequests()) == 1 })
	// Tapped again, it ends the conversation rather than starting another.
	if err := h.tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	h.until("ended", func() bool { return h.app.assistant.voice == nil && !h.app.assistant.listening })
	h.settle()
	if got := len(h.core.assistant.voiceRequests()); got != 1 {
		t.Errorf("sessions = %d, want = 1", got)
	}
}

func TestAssistantVoiceHold(t *testing.T) {
	audio := &fakeAudio{}
	h, srv := newVoiceHarness(t, audio)
	lang := "en"

	// Held past holdTime, the session listens while held.
	h.app.pressAssistant(nil, lang)
	h.until("held", func() bool { return len(h.core.assistant.voiceRequests()) == 1 })
	if got := h.core.assistant.voiceRequests()[0].GetMode(); got != pluginv1.VoiceMode_VOICE_MODE_PUSH_TO_TALK {
		t.Errorf("mode = %v, want = push to talk", got)
	}
	srv.next(t, "ptt_start")
	h.app.releaseAssistant(lang)
	srv.next(t, "ptt_end")
	if !h.app.assistant.pushToTalk || h.app.assistant.talking {
		t.Errorf("pushToTalk, talking = %v, %v, want = true, false", h.app.assistant.pushToTalk, h.app.assistant.talking)
	}
	// Held again, it talks again; tapped, it ends.
	h.app.pressAssistant(nil, lang)
	h.until("held again", func() bool { return h.app.assistant.talking })
	srv.next(t, "ptt_start")
	h.app.releaseAssistant(lang)
	srv.next(t, "ptt_end")
	h.app.tapAssistant(nil, lang)
	srv.next(t, "close")
	if h.app.assistant.voice != nil {
		t.Errorf("the session stayed after a tap")
	}
}

func TestAssistantVoiceErrors(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		sound bool
		want  string
	}{
		{"denied", voice.ErrDenied, false, "Allow this site to use the microphone"},
		{"silent", nil, false, "Fake Mic hears nothing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio := &fakeAudio{err: tt.err}
			h, _ := newVoiceHarness(t, audio)
			if err := h.tt.Click("Assistant"); err != nil {
				t.Fatal(err)
			}
			if tt.err == nil {
				h.until("opened", func() bool { return audio.device() != nil })
				audio.device().say(silentFrames, tt.sound)
			}
			h.until(tt.name, h.shows(tt.want))
			h.save(shots, "assistant-voice-"+tt.name)
			if got := audio.opened; !slices.Equal(got, []string{""}) {
				t.Errorf("opened = %q, want = the system's default", got)
			}
		})
	}
}

// The microphone chosen in the settings is the one opened.
func TestAssistantVoiceMicrophone(t *testing.T) {
	audio := &fakeAudio{}
	h, _ := newVoiceHarness(t, audio)
	h.app.prefs.Microphone = "usb"
	h.app.tapAssistant(nil, "en")
	h.until("opened", func() bool { return audio.device() != nil })
	if got := audio.opened; !slices.Equal(got, []string{"usb"}) {
		t.Errorf("opened = %q, want = usb", got)
	}
}

// V starts a conversation, but types a V in a text field.
func TestAssistantVoiceKey(t *testing.T) {
	audio := &fakeAudio{}
	h, _ := newVoiceHarness(t, audio)
	if err := h.tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.Focused("Search") {
		t.Fatal("the search field is not focused")
	}
	h.tt.TypeKey(0, ui.KeyV, "v")
	h.settle()
	if len(audio.opened) != 0 {
		t.Fatalf("V in the search field opened the microphone")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	h.tt.ClickAt(640, 500)
	h.settle()
	h.tt.TypeKey(0, ui.KeyV, "v")
	h.until("opened", func() bool { return audio.device() != nil })
}

// The Assistant settings choose the microphone among the system's.
func TestAssistantVoiceMicrophonePref(t *testing.T) {
	audio := &fakeAudio{}
	h, _ := newVoiceHarness(t, audio)
	h.app.router.Push("/settings/assistant")
	h.settle()
	for _, want := range []string{"Microphone", "System default", "whose echo and noise are removed"} {
		if !h.tt.HasText(want) {
			t.Errorf("no %q; texts = %q", want, h.tt.Texts())
		}
	}
	h.click("System default")
	h.click("USB Mic")
	h.settle()
	if got := h.app.prefs.Microphone; got != "usb" {
		t.Errorf("microphone = %q, want = usb", got)
	}
}

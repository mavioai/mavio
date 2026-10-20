package assistant

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

var opus, aac, mp3, wavFormat = audioFormats[0], audioFormats[1], audioFormats[2], audioFormats[4]

func TestRecorderKeepsSpeechWithItsAnswer(t *testing.T) {
	dir := t.TempDir()
	// Without ffmpeg, speech is kept as WAV.
	r := newRecorder(dir, "", slog.New(slog.DiscardHandler))
	const answer = "天气预报我可管不着啦。现在是凌晨 1 点多。"
	pcm := bytes.Repeat([]byte{1, 0}, 2400)
	if err := r.keepPCM(t.Context(), "52", answer, pcm, 24000); err != nil {
		t.Fatal(err)
	}
	if !r.has("52", answer) || r.has("52", "another answer") {
		t.Errorf("has = %v, %v, want = true, false", r.has("52", answer), r.has("52", "another answer"))
	}
	key := answerKey(answer)
	if got := names(r.encodings("52", key)); !slices.Equal(got, []string{"wav"}) {
		t.Errorf("encodings = %v, want = [wav]", got)
	}
	b, err := r.read("52", key, wavFormat)
	if err != nil || !bytes.Equal(b, wav(pcm, 24000)) || len(b) != 44+len(pcm) {
		t.Errorf("read = %d bytes, %v, want the PCM as WAV", len(b), err)
	}
	if f, _ := formatOf("", b); f.name != "wav" {
		t.Errorf("kept as %s, want = wav", f.name)
	}
	// Speech converted when it is played again is kept too.
	r.keep("52", key, mp3, []byte("converted"))
	if got := names(r.encodings("52", key)); !slices.Equal(got, []string{"mp3", "wav"}) {
		t.Errorf("encodings after keep = %v, want = [mp3 wav]", got)
	}

	// The same answer spoken again replaces its speech in every encoding.
	if err := r.keepPCM(t.Context(), "52", answer, pcm[:100], 24000); err != nil {
		t.Fatal(err)
	}
	if got := names(r.encodings("52", key)); !slices.Equal(got, []string{"wav"}) {
		t.Errorf("encodings spoken again = %v, want = [wav]", got)
	}

	r.remove("52")
	if _, err := os.Stat(filepath.Join(dir, "52")); !os.IsNotExist(err) {
		t.Errorf("conversation's folder after remove: %v, want = gone", err)
	}
}

// TestRecorderEncodesOpus keeps speech as Opus with ffmpeg.
func TestRecorderEncodesOpus(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	r := newRecorder(t.TempDir(), ffmpeg, slog.New(slog.DiscardHandler))
	if err := r.keepPCM(t.Context(), "52", "answer", make([]byte, 48000), 24000); err != nil {
		t.Fatal(err)
	}
	if got := names(r.encodings("52", answerKey("answer"))); !slices.Equal(got, []string{"opus"}) {
		t.Errorf("encodings = %v, want = [opus]", got)
	}
}

func TestRecorderRefuses(t *testing.T) {
	r := newRecorder(t.TempDir(), "", slog.New(slog.DiscardHandler))
	if err := r.keepPCM(t.Context(), "../up", "answer", []byte{0, 0}, 24000); err != nil || r.has("../up", "answer") {
		t.Errorf("keepPCM outside the folder = %v, kept %v; want refused", err, r.has("../up", "answer"))
	}
	for _, c := range []struct{ conv, key string }{
		{"..", answerKey("a")},
		{"52", "../52"},
	} {
		if _, err := r.file(c.conv, c.key, mp3); err == nil {
			t.Errorf("file(%q, %q) opened, want = refused", c.conv, c.key)
		}
	}
	none := newRecorder("", "", slog.New(slog.DiscardHandler))
	if err := none.keepPCM(t.Context(), "52", "a", []byte{0, 0}, 24000); err != nil || none.has("52", "a") {
		t.Error("a recorder without a folder keeps speech")
	}
}

func TestRecorderPrunes(t *testing.T) {
	dir := t.TempDir()
	r := newRecorder(dir, "", slog.New(slog.DiscardHandler))
	now := time.Now()
	for _, conv := range []string{"1", "2", "3"} {
		if err := r.keepPCM(t.Context(), conv, "answer", []byte{0, 0}, 24000); err != nil {
			t.Fatal(err)
		}
	}
	old := now.Add(-2 * time.Hour)
	for _, conv := range []string{"1", "2"} {
		if err := os.Chtimes(filepath.Join(dir, conv), old, old); err != nil {
			t.Fatal(err)
		}
	}
	// 1 is live, 2 is gone, 3 is gone but new.
	r.prune(map[string]bool{"1": true}, now)
	for conv, want := range map[string]bool{"1": true, "2": false, "3": true} {
		if _, err := os.Stat(filepath.Join(dir, conv)); (err == nil) != want {
			t.Errorf("conversation %s kept = %v, want = %v", conv, err == nil, want)
		}
	}
}

func names(fs []audioFormat) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.name
	}
	return out
}

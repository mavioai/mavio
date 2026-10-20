package assistant

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

var opus, aac, mp3 = audioFormats[0], audioFormats[1], audioFormats[2]

func TestRecorderKeepsSpeechWithItsAnswer(t *testing.T) {
	dir := t.TempDir()
	r := newRecorder(dir, slog.New(slog.DiscardHandler))
	user, other := core.NewID(), core.NewID()
	now := time.Now()
	const id, answer = "speech-0001", "天气预报我可管不着啦。现在是凌晨 1 点多。"
	// Sentences come before the answer's conversation is known, and after.
	r.add(user, id, 1, mp3, []byte("two"), now)
	r.add(other, id, 0, mp3, []byte("not theirs"), now)
	r.bind(user, id, "52", answer, now)
	r.add(user, id, 0, mp3, []byte("one"), now)
	r.add(user, id, 10, opus, []byte("eleven"), now)
	r.add(user, id, 10, aac, []byte("eleven converted"), now)
	if got, want := r.parts("52", answer), []int{0, 1, 10}; !slices.Equal(got, want) {
		t.Fatalf("parts = %v, want = %v", got, want)
	}
	if got := r.parts("52", "another answer"); got != nil {
		t.Errorf("parts of another answer = %v, want = none", got)
	}
	if got := names(r.encodings("52", answerKey(answer), 10)); !slices.Equal(got, []string{"opus", "aac"}) {
		t.Errorf("encodings = %v, want = [opus aac]", got)
	}
	if b, err := r.read("52", answerKey(answer), 0, mp3); err != nil || string(b) != "one" {
		t.Errorf("read = %q, %v, want = one", b, err)
	}
	// Speech converted when it is played again is kept too.
	r.keep("52", answerKey(answer), 0, opus, []byte("one converted"))
	if got := names(r.encodings("52", answerKey(answer), 0)); !slices.Equal(got, []string{"opus", "mp3"}) {
		t.Errorf("encodings after keep = %v, want = [opus mp3]", got)
	}

	// The same answer spoken again replaces its speech.
	r.add(user, "speech-0002", 0, opus, []byte("again"), now)
	r.bind(user, "speech-0002", "52", answer, now)
	if got, want := r.parts("52", answer), []int{0}; !slices.Equal(got, want) {
		t.Errorf("parts spoken again = %v, want = %v", got, want)
	}

	r.remove("52")
	if _, err := os.Stat(filepath.Join(dir, "52")); !os.IsNotExist(err) {
		t.Errorf("conversation's folder after remove: %v, want = gone", err)
	}
}

func TestRecorderRefuses(t *testing.T) {
	r := newRecorder(t.TempDir(), slog.New(slog.DiscardHandler))
	user, now := core.NewID(), time.Now()
	r.add(user, "short", 0, mp3, []byte("a"), now)
	r.add(user, "speech-0001", maxParts, mp3, []byte("a"), now)
	r.bind(user, "speech-0001", "../up", "answer", now)
	if len(r.open) != 0 {
		t.Errorf("open = %+v, want = none", r.open)
	}
	for _, c := range []struct{ conv, answer string }{
		{"..", answerKey("a")},
		{"52", "../52"},
	} {
		if _, err := r.file(c.conv, c.answer, 0, mp3); err == nil {
			t.Errorf("file(%q, %q) opened, want = refused", c.conv, c.answer)
		}
	}
	if none := newRecorder("", slog.New(slog.DiscardHandler)); none.parts("52", "a") != nil {
		t.Error("a recorder without a folder keeps speech")
	}
}

func TestRecorderForgetsAndPrunes(t *testing.T) {
	dir := t.TempDir()
	r := newRecorder(dir, slog.New(slog.DiscardHandler))
	user, now := core.NewID(), time.Now()
	r.add(user, "speech-0001", 0, mp3, []byte("a"), now)
	r.forget(now.Add(recordTTL + time.Second))
	if len(r.open) != 0 {
		t.Errorf("open after the TTL = %d, want = 0", len(r.open))
	}
	for _, conv := range []string{"1", "2", "3"} {
		r.add(user, "speech-"+conv+"000", 0, mp3, []byte("a"), now)
		r.bind(user, "speech-"+conv+"000", conv, "answer", now)
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

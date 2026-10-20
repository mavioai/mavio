package assistant

import (
	"os/exec"
	"testing"
)

func TestFormatOf(t *testing.T) {
	for _, c := range []struct {
		contentType string
		audio       string
		want        string
		known       bool
	}{
		{"audio/ogg", "", "opus", true},
		{"audio/ogg; codecs=opus", "", "opus", true},
		{"audio/opus", "", "opus", true},
		{"audio/MPEG", "", "mp3", true},
		{"audio/aac", "", "aac", true},
		{"audio/x-wav", "", "wav", true},
		{"", "OggS", "opus", true},
		{"", "\xff\xf1P", "aac", true},
		{"", "ID3", "mp3", true},
		{"audio/basic", "", "", false},
	} {
		f, ok := formatOf(c.contentType, []byte(c.audio))
		if f.name != c.want || ok != c.known {
			t.Errorf("formatOf(%q, %q) = %s, %v, want = %s, %v", c.contentType, c.audio, f.name, ok, c.want, c.known)
		}
	}
}

func TestSpeechTarget(t *testing.T) {
	for _, c := range []struct {
		have    audioFormat
		formats []string
		want    string
	}{
		{mp3, nil, "mp3"},
		{mp3, []string{"mp3", "opus"}, "mp3"},
		{opus, []string{"mp3", "aac"}, "aac"},
		{opus, []string{"wav", "mp3"}, "mp3"},
		{mp3, []string{"wav", "flac"}, "flac"},
		{opus, []string{"speex"}, "opus"},
	} {
		if got := speechTarget(c.have, c.formats); got.name != c.want {
			t.Errorf("speechTarget(%s, %v) = %s, want = %s", c.have.name, c.formats, got.name, c.want)
		}
	}
}

func TestFormatList(t *testing.T) {
	if got := formatList(" Opus, ,mp3 "); len(got) != 2 || got[0] != "opus" || got[1] != "mp3" {
		t.Errorf("formatList = %v, want = [opus mp3]", got)
	}
}

// TestConvert converts speech with ffmpeg to each encoding.
func TestConvert(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	src, err := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "libmp3lame", "-f", "mp3", "pipe:1").Output()
	if err != nil {
		t.Skipf("ffmpeg makes no MP3: %v", err)
	}
	for _, to := range audioFormats {
		out, err := convert(t.Context(), ffmpeg, src, to)
		if err != nil {
			t.Errorf("convert to %s: %v", to.name, err)
			continue
		}
		if got, _ := formatOf("", out); got.name != to.name {
			t.Errorf("convert to %s gave %s", to.name, got.name)
		}
	}
	if _, err := convert(t.Context(), "", src, opus); err == nil {
		t.Error("convert without ffmpeg: err = nil, want one")
	}
}

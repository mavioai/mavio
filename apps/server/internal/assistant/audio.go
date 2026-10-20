package assistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// audioFormat is an encoding of speech that clients play (§8.3).
type audioFormat struct {
	// name is how clients name it in the formats they play.
	name, ext, contentType string
	// args encode to it with ffmpeg.
	args []string
}

// audioFormats are the encodings speech is kept and converted to, the most
// balanced first: Opus is the smallest for speech of its quality, AAC next,
// MP3 plays everywhere, and the lossless ones are only for clients playing
// nothing else.
var audioFormats = []audioFormat{
	{"opus", "ogg", "audio/ogg", []string{"-c:a", "libopus", "-b:a", "32k", "-f", "ogg"}},
	{"aac", "aac", "audio/aac", []string{"-c:a", "aac", "-b:a", "64k", "-f", "adts"}},
	{"mp3", "mp3", "audio/mpeg", []string{"-c:a", "libmp3lame", "-b:a", "64k", "-f", "mp3"}},
	{"flac", "flac", "audio/flac", []string{"-c:a", "flac", "-f", "flac"}},
	{"wav", "wav", "audio/wav", []string{"-c:a", "pcm_s16le", "-f", "wav"}},
}

// errNoFFmpeg is converting without ffmpeg.
var errNoFFmpeg = errors.New("no ffmpeg to convert speech")

// formatOf returns the encoding of a content type, else of the audio's
// first bytes.
func formatOf(contentType string, audio []byte) (audioFormat, bool) {
	media, _, _ := strings.Cut(contentType, ";")
	media = strings.ToLower(strings.TrimSpace(media))
	if alias, ok := map[string]string{
		"audio/opus": "audio/ogg", "application/ogg": "audio/ogg", "audio/mp3": "audio/mpeg",
		"audio/x-flac": "audio/flac", "audio/x-wav": "audio/wav", "audio/wave": "audio/wav",
	}[media]; ok {
		media = alias
	}
	if media == "" {
		switch {
		case bytes.HasPrefix(audio, []byte("OggS")):
			media = "audio/ogg"
		case bytes.HasPrefix(audio, []byte("fLaC")):
			media = "audio/flac"
		case bytes.HasPrefix(audio, []byte("RIFF")):
			media = "audio/wav"
		case len(audio) > 1 && audio[0] == 0xff && audio[1]&0xf6 == 0xf0:
			media = "audio/aac"
		default:
			media = "audio/mpeg"
		}
	}
	return formatBy(func(f audioFormat) string { return f.contentType }, media)
}

func formatBy(key func(audioFormat) string, v string) (audioFormat, bool) {
	i := slices.IndexFunc(audioFormats, func(f audioFormat) bool { return key(f) == v })
	if i < 0 {
		return audioFormat{}, false
	}
	return audioFormats[i], true
}

// speechTarget returns the encoding a client playing formats gets speech in
// whose encoding is have: have itself when the client plays it or names
// none Mavio knows, else the most balanced it plays.
func speechTarget(have audioFormat, formats []string) audioFormat {
	if plays(formats, have) {
		return have
	}
	for _, f := range audioFormats {
		if slices.Contains(formats, f.name) {
			return f
		}
	}
	return have
}

// plays tells whether a client playing formats plays f; one naming none
// is taken to play anything.
func plays(formats []string, f audioFormat) bool {
	return len(formats) == 0 || slices.Contains(formats, f.name)
}

// convert encodes audio to f with ffmpeg.
func convert(ctx context.Context, ffmpeg string, audio []byte, f audioFormat) ([]byte, error) {
	if ffmpeg == "" {
		return nil, errNoFFmpeg
	}
	args := append([]string{"-hide_banner", "-loglevel", "error", "-i", "pipe:0", "-vn", "-ac", "1"}, f.args...)
	cmd := exec.CommandContext(ctx, ffmpeg, append(args, "pipe:1")...)
	var out, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(audio), &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convert speech to %s: %w: %s", f.name, err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

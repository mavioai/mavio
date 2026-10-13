package subsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/mavioai/mavio/libs/media/vad"
)

// AudioArgs returns the ffmpeg arguments writing an audio stream of input
// as 16-bit mono samples at Silero's rate: the stream'th audio stream, or
// ffmpeg's choice when stream is negative.
func AudioArgs(input string, stream int) []string {
	args := []string{"-v", "error", "-nostdin", "-i", input}
	if stream >= 0 {
		args = append(args, "-map", "0:a:"+strconv.Itoa(stream))
	}
	return append(args, "-vn", "-sn", "-dn",
		"-ac", "1", "-ar", strconv.Itoa(vad.SampleRate), "-af", "aresample=async=1",
		"-c:a", "pcm_s16le", "-f", "s16le", "-")
}

// AudioSpeech returns the speech of an audio stream of input, the URL
// ffmpeg reads (see AudioArgs), detected by m with up to workers
// goroutines. It reads the whole stream.
func AudioSpeech(ctx context.Context, ffmpeg, input string, stream int, m *vad.Model, workers int) (Speech, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffmpeg, AudioArgs(input, stream)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("subsync: start ffmpeg: %w", err)
	}
	p, detectErr := m.Detect(ctx, out, workers)
	if detectErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if detectErr != nil {
		return nil, fmt.Errorf("subsync: detect speech: %w", detectErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("subsync: ffmpeg: %w: %s", waitErr, bytes.TrimSpace(stderr.Bytes()))
	}
	if len(p) == 0 {
		return nil, errors.New("subsync: no audio")
	}
	return FromProbabilities(p), nil
}

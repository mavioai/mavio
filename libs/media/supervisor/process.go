package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// Process is a running ffmpeg.
type Process interface {
	// Stdin receives ffmpeg's interactive keys.
	Stdin() io.Writer
	// Progress yields the output of -progress.
	Progress() io.Reader
	// Wait waits for the exit.
	Wait() error
	// Kill ends the process at once.
	Kill() error
}

// Starter starts ffmpeg with the given arguments.
type Starter func(ctx context.Context, args []string) (Process, error)

// Exec returns a Starter running the ffmpeg binary at path, reporting
// progress on standard output.
func Exec(path string) Starter {
	return func(ctx context.Context, args []string) (Process, error) {
		full := append([]string{"-progress", "pipe:1", "-nostats"}, args...)
		// The supervisor ends the process itself, gracefully first; ctx
		// only bounds the start.
		cmd := exec.CommandContext(context.WithoutCancel(ctx), path, full...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, fmt.Errorf("ffmpeg stdin: %w", err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("ffmpeg stdout: %w", err)
		}
		var stderr limitedBuffer
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("start ffmpeg: %w", err)
		}
		return &execProcess{cmd: cmd, stdin: stdin, stdout: stdout, stderr: &stderr}, nil
	}
}

type execProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr *limitedBuffer
}

func (p *execProcess) Stdin() io.Writer    { return p.stdin }
func (p *execProcess) Progress() io.Reader { return p.stdout }
func (p *execProcess) Kill() error         { return p.cmd.Process.Kill() }

func (p *execProcess) Wait() error {
	err := p.cmd.Wait()
	if err != nil && p.stderr.Len() > 0 {
		return fmt.Errorf("%w: %s", err, p.stderr.String())
	}
	return err
}

// limitedBuffer keeps the last 4 KiB written, for error messages.
type limitedBuffer struct{ b []byte }

const stderrLimit = 4096

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.b = append(l.b, p...)
	if len(l.b) > stderrLimit {
		l.b = l.b[len(l.b)-stderrLimit:]
	}
	return len(p), nil
}

func (l *limitedBuffer) Len() int       { return len(l.b) }
func (l *limitedBuffer) String() string { return string(l.b) }

// ErrIdle is the cause of sessions reaped for lack of client activity.
var ErrIdle = errors.New("transcode idle")

// ErrStopped is the cause of sessions stopped by their owner.
var ErrStopped = errors.New("transcode stopped")

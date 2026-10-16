package service

import (
	"context"
	"encoding/binary"
	"io"
	"time"
)

// First returns the first d of the audio; all of it when d is 0.
func (a Audio) First(d time.Duration) Audio {
	if d <= 0 {
		return a
	}
	n := int64(d/time.Second)*BytesPerSecond + int64(d%time.Second)*BytesPerSecond/int64(time.Second)&^1
	open := a.Open
	a.Open = func(ctx context.Context) (io.ReadCloser, error) {
		rc, err := open(ctx)
		if err != nil {
			return nil, err
		}
		return struct {
			io.Reader
			io.Closer
		}{io.LimitReader(rc, n), rc}, nil
	}
	return a
}

// Read reads all of the audio.
func (a Audio) Read(ctx context.Context) ([]byte, error) {
	rc, err := a.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Duration returns how long PCM lasts.
func Duration(pcm []byte) time.Duration {
	return time.Duration(len(pcm)/2) * time.Second / SampleRate
}

// WAV wraps PCM in a WAV file.
func WAV(pcm []byte) []byte {
	out := make([]byte, 44, 44+len(pcm))
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1) // PCM
	binary.LittleEndian.PutUint16(out[22:], 1) // mono
	binary.LittleEndian.PutUint32(out[24:], SampleRate)
	binary.LittleEndian.PutUint32(out[28:], BytesPerSecond)
	binary.LittleEndian.PutUint16(out[32:], 2)
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(len(pcm)))
	return append(out, pcm...)
}

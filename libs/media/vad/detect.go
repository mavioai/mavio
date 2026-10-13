package vad

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"

	"golang.org/x/sync/errgroup"
)

// Detector detects speech in one stream, window by window, keeping the
// model's state between them as Silero's OnnxWrapper does.
type Detector struct {
	m       *Model
	h, c    [hidden]float32
	input   [contextSize + WindowSize]float32
	feat    [hidden]float32
	scratch [scratchSize]float32
}

// NewDetector returns a detector at the start of a stream.
func (m *Model) NewDetector() *Detector {
	return &Detector{m: m}
}

// Reset returns the detector to the start of a stream.
func (d *Detector) Reset() {
	d.h, d.c = [hidden]float32{}, [hidden]float32{}
	clear(d.input[:contextSize])
}

// Process returns the probability that the next window of the stream,
// WindowSize samples from -1 to 1, holds speech.
func (d *Detector) Process(window []float32) (float32, error) {
	if len(window) != WindowSize {
		return 0, fmt.Errorf("window of %d samples, want %d", len(window), WindowSize)
	}
	copy(d.input[contextSize:], window)
	d.m.features(d.input[:], d.feat[:], d.scratch[:])
	p := d.m.step(d.feat[:], &d.h, &d.c)
	copy(d.input[:contextSize], d.input[WindowSize:])
	return p, nil
}

// blockWindows is the number of windows Detect computes the features of at
// once.
const blockWindows = 1024

// Detect reads a stream of 16-bit little-endian mono samples at SampleRate,
// as ffmpeg's s16le output, and returns the probability of speech of each
// window of WindowSize samples, the last one padded with silence. The
// convolutions of a block of windows run on up to workers goroutines
// (GOMAXPROCS when 0), the LSTM after them in order.
func (m *Model) Detect(ctx context.Context, r io.Reader, workers int) ([]float32, error) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	br := bufio.NewReaderSize(r, 1<<16)
	// samples holds the context of the block's first window and the block.
	samples := make([]float32, contextSize+blockWindows*WindowSize)
	feats := make([]float32, blockWindows*hidden)
	raw := make([]byte, blockWindows*WindowSize*2)
	scratch := make([][scratchSize]float32, workers)
	var h, c [hidden]float32
	var out []float32
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := io.ReadFull(br, raw)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read audio: %w", err)
		}
		if n == 0 {
			return out, nil
		}
		count := n / 2
		windows := (count + WindowSize - 1) / WindowSize
		block := samples[contextSize:]
		for i := range count {
			block[i] = float32(int16(binary.LittleEndian.Uint16(raw[2*i:]))) / 32768
		}
		clear(block[count : windows*WindowSize])

		g, gctx := errgroup.WithContext(ctx)
		per := (windows + workers - 1) / workers
		for w := range workers {
			lo, hi := w*per, min((w+1)*per, windows)
			if lo >= hi {
				break
			}
			g.Go(func() error {
				for i := lo; i < hi; i++ {
					if i%64 == 0 {
						if err := gctx.Err(); err != nil {
							return err
						}
					}
					m.features(samples[i*WindowSize:i*WindowSize+contextSize+WindowSize], feats[i*hidden:(i+1)*hidden], scratch[w][:])
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		for i := range windows {
			out = append(out, m.step(feats[i*hidden:(i+1)*hidden], &h, &c))
		}
		if count < blockWindows*WindowSize {
			return out, nil
		}
		copy(samples[:contextSize], samples[len(samples)-contextSize:])
	}
}

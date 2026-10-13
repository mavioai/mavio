package vad

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
)

// ModelSource is the ONNX file the embedded weights were read from.
const ModelSource = "https://github.com/snakers4/silero-vad/raw/1e261b036686cd0017d500ee96acd1c4ba572a9d/src/silero_vad/data/silero_vad_16k_op15.onnx " +
	"(sha256 7ed98ddbad84ccac4cd0aeb3099049280713df825c610a8ed34543318f1b2c49)"

//go:embed weights.bin
var weights []byte

const (
	// SampleRate is the sample rate the model takes.
	SampleRate = 16000
	// WindowSize is the number of samples of one window.
	WindowSize = 512
	// contextSize is the number of samples before a window the model also
	// reads.
	contextSize = 64

	nFFT     = 256
	hop      = 128
	bins     = nFFT/2 + 1 // 129
	padRight = 64
	frames   = (contextSize+WindowSize+padRight-nFFT)/hop + 1 // 4
	hidden   = 128
)

// conv is a one-dimensional convolution with a kernel of 3 and padding 1.
type conv struct {
	in, out, stride int
	w               []float32 // [out][in][3]
	b               []float32 // [out]
}

// Model holds the weights of Silero VAD's 16 kHz model. It is read-only and
// may be shared by any number of detectors.
type Model struct {
	basis     []float32 // [2*bins][nFFT]: the real and imaginary STFT bases
	encoder   [4]conv
	wIn, wHid []float32 // [4*hidden][hidden], gates i, f, g, o
	bias      []float32 // [4*hidden]: bias_ih + bias_hh
	dec       []float32 // [hidden]
	decBias   float32
}

// expected lists the tensors of the weights with their dimensions.
var expected = []struct {
	name string
	dims []uint32
}{
	{"model.stft.forward_basis_buffer", []uint32{2 * bins, 1, nFFT}},
	{"model.encoder.0.reparam_conv.weight", []uint32{128, bins, 3}},
	{"model.encoder.0.reparam_conv.bias", []uint32{128}},
	{"model.encoder.1.reparam_conv.weight", []uint32{64, 128, 3}},
	{"model.encoder.1.reparam_conv.bias", []uint32{64}},
	{"model.encoder.2.reparam_conv.weight", []uint32{64, 64, 3}},
	{"model.encoder.2.reparam_conv.bias", []uint32{64}},
	{"model.encoder.3.reparam_conv.weight", []uint32{128, 64, 3}},
	{"model.encoder.3.reparam_conv.bias", []uint32{128}},
	{"model.decoder.rnn.weight_ih", []uint32{4 * hidden, hidden}},
	{"model.decoder.rnn.weight_hh", []uint32{4 * hidden, hidden}},
	{"model.decoder.rnn.bias_ih", []uint32{4 * hidden}},
	{"model.decoder.rnn.bias_hh", []uint32{4 * hidden}},
	{"model.decoder.decoder.2.weight", []uint32{1, hidden, 1}},
	{"model.decoder.decoder.2.bias", []uint32{1}},
}

// NewModel reads the embedded weights.
func NewModel() (*Model, error) {
	t, err := parseWeights(weights)
	if err != nil {
		return nil, fmt.Errorf("vad weights: %w", err)
	}
	m := &Model{
		basis: t[0],
		wIn:   t[9],
		wHid:  t[10],
		bias:  make([]float32, 4*hidden),
		dec:   t[13],
	}
	strides := [4]int{1, 2, 2, 1}
	for i := range m.encoder {
		d := expected[1+2*i].dims
		m.encoder[i] = conv{in: int(d[1]), out: int(d[0]), stride: strides[i], w: t[1+2*i], b: t[2+2*i]}
	}
	for i := range m.bias {
		m.bias[i] = t[11][i] + t[12][i]
	}
	m.decBias = t[14][0]
	return m, nil
}

// parseWeights reads the tensors silerogen writes, checking them against
// expected.
func parseWeights(b []byte) ([][]float32, error) {
	le := binary.LittleEndian
	if len(b) < 8 || string(b[:4]) != "SVAD" {
		return nil, errors.New("not Silero VAD weights")
	}
	n := int(le.Uint32(b[4:]))
	if n != len(expected) {
		return nil, fmt.Errorf("%d tensors, want %d", n, len(expected))
	}
	b = b[8:]
	out := make([][]float32, n)
	short := errors.New("truncated")
	for i := range n {
		if len(b) < 2 {
			return nil, short
		}
		l := int(le.Uint16(b))
		if len(b) < 3+l {
			return nil, short
		}
		name := string(b[2 : 2+l])
		nd := int(b[2+l])
		b = b[3+l:]
		if len(b) < 4*nd {
			return nil, short
		}
		dims := make([]uint32, nd)
		size := 1
		for j := range dims {
			dims[j] = le.Uint32(b[4*j:])
			size *= int(dims[j])
		}
		b = b[4*nd:]
		if name != expected[i].name || !slices.Equal(dims, expected[i].dims) {
			return nil, fmt.Errorf("tensor %d is %s %v, want %s %v", i, name, dims, expected[i].name, expected[i].dims)
		}
		if len(b) < 4*size {
			return nil, short
		}
		v := make([]float32, size)
		for j := range v {
			v[j] = math.Float32frombits(le.Uint32(b[4*j:]))
		}
		out[i] = v
		b = b[4*size:]
	}
	if len(b) != 0 {
		return nil, errors.New("trailing data")
	}
	return out, nil
}

// features computes the encoder's output for one window: x holds the
// context and the window (contextSize+WindowSize samples), out receives
// hidden values. scratch must hold scratchSize values.
func (m *Model) features(x []float32, out []float32, scratch []float32) {
	// Reflection padding on the right, as ONNX's Pad in reflect mode.
	padded := scratch[:contextSize+WindowSize+padRight]
	copy(padded, x)
	n := contextSize + WindowSize
	for i := range padRight {
		padded[n+i] = x[n-2-i]
	}
	// STFT magnitudes, laid out [bins][frames+2] with a zero column on each
	// side as the first convolution's padding.
	mag := scratch[len(padded) : len(padded)+bins*(frames+2)]
	clear(mag)
	for f := range bins {
		re := m.basis[f*nFFT : (f+1)*nFFT]
		im := m.basis[(f+bins)*nFFT : (f+bins+1)*nFFT]
		for t := range frames {
			seg := padded[t*hop : t*hop+nFFT]
			r, i := dot(re, seg), dot(im, seg)
			mag[f*(frames+2)+1+t] = float32(math.Sqrt(float64(r*r + i*i)))
		}
	}
	in, length := mag, frames
	rest := scratch[len(padded)+len(mag):]
	for li := range m.encoder {
		c := &m.encoder[li]
		outLen := (length+2-3)/c.stride + 1
		var dst []float32
		if li == len(m.encoder)-1 {
			dst = out // outLen is 1: no padding columns
		} else {
			dst = rest[:c.out*(outLen+2)]
			rest = rest[len(dst):]
			clear(dst)
		}
		c.apply(in, length, dst, outLen, li != len(m.encoder)-1)
		in, length = dst, outLen
	}
}

// scratchSize is the number of values features needs.
const scratchSize = contextSize + WindowSize + padRight + bins*(frames+2) + 128*(4+2) + 64*(2+2) + 64*(1+2)

// apply runs the convolution and a ReLU over in, laid out [c.in][length+2]
// with zero padding columns, into dst, laid out [c.out][outLen+2] when
// padded or [c.out][outLen] when not.
func (c *conv) apply(in []float32, length int, dst []float32, outLen int, padded bool) {
	stride := length + 2
	dstStride, dstOff := outLen, 0
	if padded {
		dstStride, dstOff = outLen+2, 1
	}
	for o := range c.out {
		w := c.w[o*c.in*3 : (o+1)*c.in*3]
		for t := range outLen {
			s := c.b[o]
			p := t * c.stride
			for ch := range c.in {
				row := in[ch*stride+p : ch*stride+p+3]
				k := w[ch*3 : ch*3+3]
				s += k[0]*row[0] + k[1]*row[1] + k[2]*row[2]
			}
			dst[o*dstStride+dstOff+t] = max(s, 0)
		}
	}
}

// step runs the LSTM cell and the decoder on one window's features,
// updating h and c, and returns the probability of speech.
func (m *Model) step(x []float32, h, c *[hidden]float32) float32 {
	var gates [4 * hidden]float32
	for g := range gates {
		gates[g] = m.bias[g] + dot(m.wIn[g*hidden:(g+1)*hidden], x) + dot(m.wHid[g*hidden:(g+1)*hidden], h[:])
	}
	s := m.decBias
	for j := range hidden {
		i := sigmoid(gates[j])
		f := sigmoid(gates[hidden+j])
		g := float32(math.Tanh(float64(gates[2*hidden+j])))
		o := sigmoid(gates[3*hidden+j])
		c[j] = f*c[j] + i*g
		h[j] = o * float32(math.Tanh(float64(c[j])))
		s += m.dec[j] * max(h[j], 0)
	}
	return sigmoid(s)
}

func sigmoid(x float32) float32 {
	return float32(1 / (1 + math.Exp(-float64(x))))
}

// dot returns the dot product of a and b, which have the same length, a
// multiple of 4.
func dot(a, b []float32) float32 {
	b = b[:len(a)]
	var s0, s1, s2, s3 float32
	for i := 0; i+4 <= len(a); i += 4 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
	}
	return (s0 + s1) + (s2 + s3)
}

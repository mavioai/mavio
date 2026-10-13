// Command silerogen reads the weights of Silero VAD's 16 kHz model from its
// ONNX file into the format package vad embeds.
//
// The ONNX file is silero_vad_16k_op15.onnx of github.com/snakers4/silero-vad
// (MIT), at the commit and hash in vad.ModelSource:
//
//	go run ./internal/cmd/silerogen -onnx silero_vad_16k_op15.onnx -out weights.bin
//
// The output is "SVAD", a little-endian uint32 count of tensors, then for each
// tensor its name (uint16 length and bytes), its number of dimensions (uint8),
// the dimensions (uint32 each) and its values (float32 each).
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	in := flag.String("onnx", "", "Silero VAD's ONNX file")
	out := flag.String("out", "weights.bin", "output file")
	flag.Parse()
	if err := run(*in, *out); err != nil {
		slog.Error("silerogen", "err", err)
		os.Exit(1)
	}
}

// tensor is an ONNX initializer.
type tensor struct {
	name   string
	dims   []uint32
	values []float32
}

func run(in, out string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	graph, err := field(data, 7) // ModelProto.graph
	if err != nil {
		return fmt.Errorf("model: %w", err)
	}
	var tensors []tensor
	err = each(graph, func(num protowire.Number, typ protowire.Type, v []byte) error {
		if num != 5 || typ != protowire.BytesType { // GraphProto.initializer
			return nil
		}
		t, err := parseTensor(v)
		if err != nil {
			return err
		}
		if strings.HasPrefix(t.name, "model.") {
			tensors = append(tensors, t)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("graph: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("SVAD")
	le := binary.LittleEndian
	buf.Write(le.AppendUint32(nil, uint32(len(tensors))))
	for _, t := range tensors {
		buf.Write(le.AppendUint16(nil, uint16(len(t.name))))
		buf.WriteString(t.name)
		buf.WriteByte(byte(len(t.dims)))
		for _, d := range t.dims {
			buf.Write(le.AppendUint32(nil, d))
		}
		for _, v := range t.values {
			buf.Write(le.AppendUint32(nil, math.Float32bits(v)))
		}
	}
	return os.WriteFile(out, buf.Bytes(), 0o644)
}

// parseTensor reads a TensorProto of floats.
func parseTensor(b []byte) (tensor, error) {
	var t tensor
	var raw []byte
	dataType := uint64(0)
	err := each(b, func(num protowire.Number, typ protowire.Type, v []byte) error {
		switch num {
		case 1: // dims
			if typ == protowire.BytesType {
				for len(v) > 0 {
					d, n := protowire.ConsumeVarint(v)
					if n < 0 {
						return protowire.ParseError(n)
					}
					t.dims = append(t.dims, uint32(d))
					v = v[n:]
				}
			} else {
				d, _ := protowire.ConsumeVarint(v)
				t.dims = append(t.dims, uint32(d))
			}
		case 2: // data_type
			dataType, _ = protowire.ConsumeVarint(v)
		case 4: // float_data
			for i := 0; i+4 <= len(v); i += 4 {
				t.values = append(t.values, math.Float32frombits(binary.LittleEndian.Uint32(v[i:])))
			}
		case 8: // name
			t.name = string(v)
		case 9: // raw_data
			raw = v
		}
		return nil
	})
	if err != nil {
		return t, err
	}
	if dataType != 1 {
		return t, fmt.Errorf("tensor %s: data type %d, want float", t.name, dataType)
	}
	for i := 0; i+4 <= len(raw); i += 4 {
		t.values = append(t.values, math.Float32frombits(binary.LittleEndian.Uint32(raw[i:])))
	}
	n := 1
	for _, d := range t.dims {
		n *= int(d)
	}
	if n != len(t.values) {
		return t, fmt.Errorf("tensor %s: %d values for dimensions %v", t.name, len(t.values), t.dims)
	}
	return t, nil
}

// field returns the bytes of the first field num of a message.
func field(b []byte, num protowire.Number) ([]byte, error) {
	var out []byte
	err := each(b, func(n protowire.Number, typ protowire.Type, v []byte) error {
		if n == num && typ == protowire.BytesType && out == nil {
			out = v
		}
		return nil
	})
	if err == nil && out == nil {
		err = fmt.Errorf("field %d missing", num)
	}
	return out, err
}

// each calls fn with every field of a message: varints as their encoding,
// length-delimited fields as their bytes.
func each(b []byte, fn func(protowire.Number, protowire.Type, []byte) error) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		var v []byte
		switch typ {
		case protowire.BytesType:
			v, n = protowire.ConsumeBytes(b)
		case protowire.VarintType:
			_, n = protowire.ConsumeVarint(b)
			v = b[:max(n, 0)]
		case protowire.Fixed32Type:
			_, n = protowire.ConsumeFixed32(b)
			v = b[:max(n, 0)]
		case protowire.Fixed64Type:
			_, n = protowire.ConsumeFixed64(b)
			v = b[:max(n, 0)]
		default:
			return errors.New("unsupported wire type")
		}
		if n < 0 {
			return protowire.ParseError(n)
		}
		if err := fn(num, typ, v); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

// Package abi defines the byte formats exchanged between the WebAssembly host
// and guest: HTTP-shaped call envelopes for Connect requests, and the JSON
// documents of the http_fetch host function.
//
// Envelopes are sequences of fields; strings and byte slices are prefixed
// with their length as a little-endian uint32.
//
//	request:  path, header count, (name, value)…, body
//	response: status (uint32), header count, (name, value)…, body
package abi

import (
	"encoding/binary"
	"errors"
	"net/http"
	"sort"
)

// Names of the module exports and host imports. Guests log to stderr, which
// the host forwards to its logger.
const (
	ExportInit  = "_initialize"
	ExportAlloc = "mavio_alloc"
	ExportFree  = "mavio_free"
	ExportCall  = "mavio_call"
	HostModule  = "mavio"
	ImportFetch = "http_fetch"
	ImportRead  = "http_read"
)

// ErrMalformed reports an envelope that cannot be decoded.
var ErrMalformed = errors.New("abi: malformed envelope")

// Request is a Connect call routed to a guest handler.
type Request struct {
	Path   string
	Header http.Header
	Body   []byte
}

// Response is the guest handler's reply.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// EncodeRequest serializes r.
func EncodeRequest(r Request) []byte {
	var b []byte
	b = appendBytes(b, []byte(r.Path))
	b = appendHeader(b, r.Header)
	return appendBytes(b, r.Body)
}

// DecodeRequest parses an encoded request.
func DecodeRequest(b []byte) (Request, error) {
	var r Request
	path, b, err := readBytes(b)
	if err != nil {
		return r, err
	}
	r.Path = string(path)
	if r.Header, b, err = readHeader(b); err != nil {
		return r, err
	}
	r.Body, _, err = readBytes(b)
	return r, err
}

// EncodeResponse serializes r.
func EncodeResponse(r Response) []byte {
	b := binary.LittleEndian.AppendUint32(nil, uint32(r.Status))
	b = appendHeader(b, r.Header)
	return appendBytes(b, r.Body)
}

// DecodeResponse parses an encoded response.
func DecodeResponse(b []byte) (Response, error) {
	var r Response
	if len(b) < 4 {
		return r, ErrMalformed
	}
	r.Status = int(binary.LittleEndian.Uint32(b))
	var err error
	if r.Header, b, err = readHeader(b[4:]); err != nil {
		return r, err
	}
	r.Body, _, err = readBytes(b)
	return r, err
}

func appendBytes(b, v []byte) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(v)))
	return append(b, v...)
}

func readBytes(b []byte) (v, rest []byte, err error) {
	if len(b) < 4 {
		return nil, nil, ErrMalformed
	}
	n := binary.LittleEndian.Uint32(b)
	if uint64(len(b)-4) < uint64(n) {
		return nil, nil, ErrMalformed
	}
	return b[4 : 4+n], b[4+n:], nil
}

func appendHeader(b []byte, h http.Header) []byte {
	var pairs [][2]string
	for k, vs := range h {
		for _, v := range vs {
			pairs = append(pairs, [2]string{k, v})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i][0] < pairs[j][0] })
	b = binary.LittleEndian.AppendUint32(b, uint32(len(pairs)))
	for _, p := range pairs {
		b = appendBytes(b, []byte(p[0]))
		b = appendBytes(b, []byte(p[1]))
	}
	return b
}

func readHeader(b []byte) (http.Header, []byte, error) {
	if len(b) < 4 {
		return nil, nil, ErrMalformed
	}
	n := binary.LittleEndian.Uint32(b)
	b = b[4:]
	h := http.Header{}
	for range n {
		k, rest, err := readBytes(b)
		if err != nil {
			return nil, nil, err
		}
		v, rest, err := readBytes(rest)
		if err != nil {
			return nil, nil, err
		}
		h.Add(string(k), string(v))
		b = rest
	}
	return h, b, nil
}

// FetchRequest is the JSON argument of the http_fetch host function.
type FetchRequest struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Header http.Header `json:"header,omitempty"`
	Body   []byte      `json:"body,omitempty"`
}

// FetchResponse is the JSON result of the http_fetch host function. Error is
// set when the request was denied or failed before a response arrived.
type FetchResponse struct {
	Status int         `json:"status,omitempty"`
	Header http.Header `json:"header,omitempty"`
	Body   []byte      `json:"body,omitempty"`
	Error  string      `json:"error,omitempty"`
}

package abi

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	req := Request{
		Path:   "/mavio.plugin.v1.PluginService/Describe",
		Header: http.Header{"Content-Type": {"application/proto"}, "X-Multi": {"a", "b"}},
		Body:   []byte{0, 1, 2, 255},
	}
	got, err := DecodeRequest(EncodeRequest(req))
	if err != nil || got.Path != req.Path || !bytes.Equal(got.Body, req.Body) ||
		got.Header.Get("Content-Type") != "application/proto" || len(got.Header.Values("X-Multi")) != 2 {
		t.Errorf("request round trip = %+v, %v", got, err)
	}

	resp := Response{Status: 404, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"code":"not_found"}`)}
	gotResp, err := DecodeResponse(EncodeResponse(resp))
	if err != nil || gotResp.Status != 404 || string(gotResp.Body) != string(resp.Body) {
		t.Errorf("response round trip = %+v, %v", gotResp, err)
	}

	route := Request{Path: HTTPPrefix + "/page", Method: http.MethodGet, Query: "a=1&b=2"}
	if got, err := DecodeRequest(EncodeRequest(route)); err != nil || got.Method != http.MethodGet || got.Query != "a=1&b=2" || got.Path != route.Path {
		t.Errorf("route request round trip = %+v, %v", got, err)
	}
	// Requests without method and query read as before, as POSTs.
	if got, err := DecodeRequest(EncodeRequest(req)); err != nil || got.Method != "" || got.Query != "" {
		t.Errorf("request without method = %+v, %v", got, err)
	}

	empty, err := DecodeRequest(EncodeRequest(Request{}))
	if err != nil || empty.Path != "" || len(empty.Body) != 0 {
		t.Errorf("empty request = %+v, %v", empty, err)
	}
}

func TestMalformed(t *testing.T) {
	full := EncodeRequest(Request{Path: "/x", Body: []byte("body")})
	for n := range len(full) - 1 {
		if _, err := DecodeRequest(full[:n]); !errors.Is(err, ErrMalformed) {
			t.Errorf("truncated to %d bytes: error = %v, want ErrMalformed", n, err)
		}
	}
	if _, err := DecodeResponse([]byte{1, 2}); !errors.Is(err, ErrMalformed) {
		t.Errorf("short response: %v", err)
	}
}

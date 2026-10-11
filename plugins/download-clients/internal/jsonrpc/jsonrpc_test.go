package jsonrpc

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestNewRequest(t *testing.T) {
	var ids IDs
	if a, b := ids.Next(), ids.Next(); a != 1 || b != 2 {
		t.Errorf("Next() = %d, %d, want = 1, 2", a, b)
	}
	tests := []struct {
		params []any
		want   string
	}{
		{nil, `{"id":7,"jsonrpc":"2.0","method":"core.get_torrents_status","params":[]}`},
		{[]any{"a", 1, []string{"x"}}, `{"id":7,"jsonrpc":"2.0","method":"core.get_torrents_status","params":["a",1,["x"]]}`},
	}
	for _, tt := range tests {
		req, err := NewRequest(context.Background(), "http://host/json", 7, "core.get_torrents_status", tt.params...)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(req.Body)
		if req.Method != http.MethodPost || string(body) != tt.want {
			t.Errorf("request = %s %s, want = POST %s", req.Method, body, tt.want)
		}
		if req.Header.Get("Content-Type") != "application/json" || req.Header.Get("Accept") != "application/json-rpc, application/json" {
			t.Errorf("headers = %v, want JSON-RPC's", req.Header)
		}
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		body              string
		failed, hasResult bool
	}{
		{`{"id":1,"result":{"a":1},"error":null}`, false, true},
		{`{"id":1,"result":null,"error":{"message":"Not authenticated","code":1}}`, true, false},
		{`{"id":1,"result":true}`, false, true},
		{`{"id":1,"result":null}`, false, false},
		{`{"id":1,"result":false,"error":"boom"}`, true, true},
	}
	for _, tt := range tests {
		r, err := Decode([]byte(tt.body))
		if err != nil {
			t.Fatal(err)
		}
		if r.Failed() != tt.failed || r.HasResult() != tt.hasResult {
			t.Errorf("%s: Failed(), HasResult() = %v, %v, want = %v, %v", tt.body, r.Failed(), r.HasResult(), tt.failed, tt.hasResult)
		}
	}
	if _, err := Decode([]byte("<html>")); err == nil {
		t.Errorf("Decode(html) error = nil, want an error")
	}
}

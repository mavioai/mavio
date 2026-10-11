// Package jsonrpc is the JSON-RPC 2.0 the clients that speak it share,
// Deluge, Hadouken and NZBGet, as Radarr's JsonRpcRequestBuilder and
// JsonRpcResponse.
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
)

// IDs numbers a client's requests.
type IDs struct{ n atomic.Int64 }

// Next returns the next request's ID, from 1.
func (i *IDs) Next() int64 { return i.n.Add(1) }

// NewRequest returns the POST request calling a method with params, none
// being an empty list.
func NewRequest(ctx context.Context, url string, id int64, method string, params ...any) (*http.Request, error) {
	if params == nil {
		params = []any{}
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": id})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json-rpc, application/json")
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// Response is a JSON-RPC answer: its result, or its error.
type Response struct {
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

// Decode reads a JSON-RPC answer.
func Decode(body []byte) (*Response, error) {
	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("unexpected response: %w", err)
	}
	return &r, nil
}

// Failed reports whether the answer is an error.
func (r *Response) Failed() bool { return isSet(r.Error) }

// HasResult reports whether the answer has a result other than null.
func (r *Response) HasResult() bool { return isSet(r.Result) }

func isSet(raw json.RawMessage) bool {
	return len(raw) > 0 && string(bytes.TrimSpace(raw)) != "null"
}

package apidocs

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mavioai/mavio/libs/proto/openapi"
)

//go:embed index.html
var index []byte

// Handler serves GET /spec.json, the OpenAPI document of version with the
// public procedures, such as "/mavio.auth.v1.AuthService/Login", marked
// as taking no token, and GET /, the API reference.
func Handler(version string, public []string) (http.Handler, error) {
	spec, err := Spec(version, public)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /spec.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	return mux, nil
}

// Spec returns the OpenAPI document of version, with public procedures
// marked as taking no token and the streaming procedures it cannot
// describe left out.
func Spec(version string, public []string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(openapi.Spec, &doc); err != nil {
		return nil, fmt.Errorf("openapi document: %w", err)
	}
	if info, ok := doc["info"].(map[string]any); ok {
		info["version"] = version
	}
	paths, _ := doc["paths"].(map[string]any)
	for path, item := range paths {
		if ops, ok := item.(map[string]any); ok && len(ops) == 0 {
			delete(paths, path)
		}
	}
	for _, procedure := range public {
		item, ok := paths[procedure].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("openapi document: no public procedure %s", procedure)
		}
		for method, op := range item {
			if op, ok := op.(map[string]any); ok && method == strings.ToLower(method) {
				op["security"] = []any{}
			}
		}
	}
	return json.Marshal(doc)
}

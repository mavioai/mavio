package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	url := newServer(t)
	health := url + "/mavio.system.v1.SystemService/GetHealth"

	t.Run("preflight", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, health, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://other.example")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type, connect-protocol-version")
		req.Header.Set("Access-Control-Request-Private-Network", "true")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		for name, want := range map[string]string{
			"Access-Control-Allow-Origin":          "*",
			"Access-Control-Allow-Headers":         "authorization, content-type, connect-protocol-version",
			"Access-Control-Allow-Private-Network": "true",
		} {
			if got := resp.Header.Get(name); got != want {
				t.Errorf("%s = %q, want = %q", name, got, want)
			}
		}
		if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
			t.Errorf("Access-Control-Allow-Methods = %q, want = POST among them", got)
		}
	})

	t.Run("request", func(t *testing.T) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, health, strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://other.example")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want = *", got)
		}
		if got := resp.Header.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "Grpc-Status") {
			t.Errorf("Access-Control-Expose-Headers = %q, want = Grpc-Status among them", got)
		}
	})

	t.Run("same origin", func(t *testing.T) {
		resp, err := http.Post(health, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want = none", got)
		}
	})
}

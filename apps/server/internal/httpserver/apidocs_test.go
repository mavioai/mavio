package httpserver_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/grpcreflect"
)

func TestAPIDocs(t *testing.T) {
	url := newServer(t)
	get := func(path string) (string, string) {
		t.Helper()
		resp, err := http.Get(url + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want = 200", path, resp.StatusCode)
		}
		return resp.Header.Get("Content-Type"), string(body)
	}

	// The reference needs no token.
	if typ, body := get("/"); !strings.HasPrefix(typ, "text/html") || !strings.Contains(body, "spec.json") {
		t.Errorf("GET / = %s %.80q, want = the reference of spec.json", typ, body)
	}
	typ, body := get("/spec.json")
	if typ != "application/json" {
		t.Errorf("spec content type = %q", typ)
	}
	var spec struct {
		Info  struct{ Version string }
		Paths map[string]map[string]struct {
			Security *[]any
		}
	}
	if err := json.Unmarshal([]byte(body), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Info.Version != "v-test" {
		t.Errorf("version = %q, want = v-test", spec.Info.Version)
	}
	for _, tc := range []struct {
		path   string
		public bool
	}{
		{"/mavio.auth.v1.AuthService/Login", true},
		{"/mavio.system.v1.SystemService/GetHealth", true},
		{"/mavio.library.v1.ItemService/GetItem", false},
		{"/mavio.system.v1.BackupService/CreateBackup", false},
	} {
		op, ok := spec.Paths[tc.path]["post"]
		if !ok {
			t.Errorf("%s missing", tc.path)
			continue
		}
		if public := op.Security != nil && len(*op.Security) == 0; public != tc.public {
			t.Errorf("%s public = %v, want = %v", tc.path, public, tc.public)
		}
	}
	for path, item := range spec.Paths {
		if len(item) == 0 {
			t.Errorf("%s has no operations", path)
		}
		if strings.HasPrefix(path, "/mavio.plugin.") {
			t.Errorf("%s is a plugin contract", path)
		}
	}
	if _, ok := spec.Paths["/media/{playback}/{file}"]; !ok {
		t.Error("media route missing")
	}
}

func TestReflection(t *testing.T) {
	h, _ := newHandler(t, nil)
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	// Reflection needs no token, and lists every service the server
	// serves: those of the OpenAPI document and EventService, whose only
	// method streams.
	stream := grpcreflect.NewClient(srv.Client(), srv.URL).NewStream(t.Context())
	t.Cleanup(func() { _, _ = stream.Close() })
	names, err := stream.ListServices()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(names))
	for _, n := range names {
		if !strings.HasPrefix(string(n), "grpc.reflection.") {
			got = append(got, string(n))
		}
	}
	slices.Sort(got)

	resp, err := srv.Client().Get(srv.URL + "/spec.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var spec struct{ Paths map[string]any }
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatal(err)
	}
	want := []string{"mavio.session.v1.EventService"}
	for path := range spec.Paths {
		if service, _, ok := strings.Cut(strings.TrimPrefix(path, "/"), "/"); ok && strings.HasPrefix(service, "mavio.") && !slices.Contains(want, service) {
			want = append(want, service)
		}
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("reflected services = %v, want = %v", got, want)
	}
}

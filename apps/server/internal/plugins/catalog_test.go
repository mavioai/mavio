package plugins_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/store"
)

// pack zips the smoke plugin as a version, its manifest saying manifestVersion.
func pack(t *testing.T, manifestVersion string) []byte {
	t.Helper()
	wasm, err := os.ReadFile(wasmPlugin)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"org.mavio.smoke","name":"Smoke","version":"` + manifestVersion + `","runtime":"RUNTIME_WASM",
		"capabilities":["CAPABILITY_METADATA_PROVIDER"],"configSchema":"{}","apiVersion":"1.0"}`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"manifest.json": []byte(manifest), "plugin.wasm": wasm} {
		w, _ := zw.Create(name)
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestCatalogInstall(t *testing.T) {
	ctx := t.Context()
	good, other := pack(t, "0.1.0"), pack(t, "0.1.0")
	sum := sha256.Sum256(good)
	versions := []map[string]string{
		{"version": "0.1.0", "api_version": "1.0", "runtime": "RUNTIME_WASM", "url": "good.zip", "sha256": hex.EncodeToString(sum[:])},
		// A newer version whose package does not match its hash.
		{"version": "0.10.0", "api_version": "1.0", "runtime": "RUNTIME_WASM", "url": "bad.zip", "sha256": hex.EncodeToString(sum[:4])},
		{"version": "9.0.0", "api_version": "2.0", "runtime": "RUNTIME_WASM", "url": "future.zip"},
		{"version": "8.0.0", "api_version": "1.0", "runtime": "RUNTIME_PROCESS", "os": "plan9", "arch": "mips", "url": "x.zip"},
	}
	index, _ := json.Marshal(map[string]any{"plugins": []any{map[string]any{"id": "org.mavio.smoke", "name": "Smoke", "versions": versions}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write(index)
		case "/good.zip":
			_, _ = w.Write(good)
		case "/bad.zip":
			_, _ = w.Write(other)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	root := t.TempDir()
	m, err := plugins.Open(ctx, plugins.Config{
		Dir: root, CacheDir: t.TempDir(), Store: db,
		Catalogs: func() []string { return []string{srv.URL + "/missing.json", srv.URL + "/index.json"} },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(ctx) })

	cat, err := m.Catalog(ctx)
	if err != nil || len(cat) != 1 || len(cat[0].Versions) != 2 || cat[0].Versions[0].Version != "0.10.0" {
		t.Fatalf("Catalog = %+v, %v; want 0.10.0 and 0.1.0 only", cat, err)
	}
	if _, err := m.Install(ctx, "org.mavio.smoke", "0.1.0"); err != nil {
		t.Fatal(err)
	}
	// The newest package does not match its hash; the installed version
	// keeps running.
	if _, err := m.Install(ctx, "org.mavio.smoke", ""); err == nil {
		t.Error("installing a package with the wrong hash: no error")
	}
	if p := state(t, m, "org.mavio.smoke"); p.State != plugins.Ready || p.Manifest.GetVersion() != "0.1.0" {
		t.Errorf("plugin after the failed upgrade = %+v", p)
	}
	if _, err := m.Install(ctx, "org.example.none", ""); err == nil {
		t.Error("installing an unknown plugin: no error")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "org.mavio.smoke" {
		t.Errorf("plugin folder = %v", entries)
	}
	// A server started again finds the installed plugin.
	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}
	again := open(t, root, db)
	if p := state(t, again, "org.mavio.smoke"); p.State != plugins.Ready {
		t.Errorf("plugin after a restart = %+v", p)
	}
}

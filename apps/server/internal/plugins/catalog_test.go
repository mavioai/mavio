package plugins_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

// pack zips the smoke plugin as a version, its manifest saying manifestVersion.
func pack(t *testing.T, manifestVersion string) []byte {
	t.Helper()
	wasm, err := os.ReadFile(wasmPlugin)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"dev.mavio.smoke","name":"Smoke","version":"` + manifestVersion + `","runtime":"RUNTIME_WASM",
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
	index, _ := json.Marshal(map[string]any{"plugins": []any{map[string]any{"id": "dev.mavio.smoke", "name": "Smoke", "versions": versions}}})
	// An earlier catalog lists the plugin with no version, and another not
	// released yet.
	early, _ := json.Marshal(map[string]any{"plugins": []any{
		map[string]any{"id": "dev.mavio.smoke", "name": "Smoke"},
		map[string]any{"id": "dev.mavio.unreleased", "name": "Unreleased", "category": "CATEGORY_DEVICES"},
	}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			_, _ = w.Write(index)
		case "/early.json":
			_, _ = w.Write(early)
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
		Dir: root, CacheDir: compiled, Store: db,
		Catalogs: func() []string {
			return []string{srv.URL + "/missing.json", srv.URL + "/early.json", srv.URL + "/index.json"}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(ctx) })

	cat, err := m.Catalog(ctx)
	if err != nil || len(cat) != 2 || len(cat[0].Versions) != 2 || cat[0].Versions[0].Version != "0.10.0" || cat[0].CatalogURL != srv.URL+"/index.json" {
		t.Fatalf("Catalog = %+v, %v; want smoke 0.10.0 and 0.1.0 only, from the catalog with versions", cat, err)
	}
	if u := cat[1]; u.ID != "dev.mavio.unreleased" || len(u.Versions) != 0 || u.Category != "CATEGORY_DEVICES" {
		t.Errorf("unreleased plugin = %+v, want listed with no version", u)
	}
	if _, err := m.Install(ctx, "dev.mavio.unreleased", ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("installing a plugin with no version: %v, want not found", err)
	}
	if _, err := m.Install(ctx, "dev.mavio.smoke", "0.1.0"); err != nil {
		t.Fatal(err)
	}
	// The newest package does not match its hash; the installed version
	// keeps running.
	if _, err := m.Install(ctx, "dev.mavio.smoke", ""); err == nil {
		t.Error("installing a package with the wrong hash: no error")
	}
	if p := state(t, m, "dev.mavio.smoke"); p.State != plugins.Ready || p.Manifest.GetVersion() != "0.1.0" {
		t.Errorf("plugin after the failed upgrade = %+v", p)
	}
	if _, err := m.Install(ctx, "org.example.none", ""); err == nil {
		t.Error("installing an unknown plugin: no error")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "dev.mavio.smoke" {
		t.Errorf("plugin folder = %v", entries)
	}
	// A server started again finds the installed plugin.
	if err := m.Close(ctx); err != nil {
		t.Fatal(err)
	}
	again := open(t, root, db)
	if p := state(t, again, "dev.mavio.smoke"); p.State != plugins.Ready {
		t.Errorf("plugin after a restart = %+v", p)
	}
}

// TestFileCatalog installs from a catalog file whose packages are file
// URLs, as tools/plugindist builds for a development server.
func TestFileCatalog(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	pkg := pack(t, "0.1.0")
	zipFile := filepath.Join(dir, "dist", "smoke-0.1.0.zip")
	if err := os.MkdirAll(filepath.Dir(zipFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipFile, pkg, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pkg)
	catalog, _ := json.Marshal(map[string]any{"plugins": []any{map[string]any{"id": "dev.mavio.smoke", "name": "Smoke", "versions": []any{map[string]string{
		"version": "0.1.0", "api_version": "1.0", "runtime": "RUNTIME_WASM", "url": fileURL(zipFile), "sha256": hex.EncodeToString(sum[:]),
	}}}}})
	catalogFile := filepath.Join(dir, "catalog.json")
	if err := os.WriteFile(catalogFile, catalog, 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m, err := plugins.Open(ctx, plugins.Config{Dir: t.TempDir(), CacheDir: compiled, Store: db, Catalogs: func() []string { return []string{fileURL(catalogFile)} }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close(ctx) })
	if got, err := m.Catalog(ctx); err != nil || len(got) != 1 || len(got[0].Versions) != 1 {
		t.Fatalf("Catalog = %+v, %v; want smoke 0.1.0", got, err)
	}
	if _, err := m.Install(ctx, "dev.mavio.smoke", ""); err != nil {
		t.Fatal(err)
	}
	if p := state(t, m, "dev.mavio.smoke"); p.State != plugins.Ready {
		t.Errorf("plugin installed from a file = %+v", p)
	}
}

// fileURL is the file URL of an absolute path.
func fileURL(abs string) string {
	return (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(abs), "/")}).String()
}

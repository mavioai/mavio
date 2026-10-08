// Package manifest loads and validates plugin manifests and plugin
// configuration.
//
// A plugin is a directory containing manifest.json (a mavio.plugin.v1.Manifest
// in protobuf JSON form) and the plugin itself: plugin.wasm for the WASM
// runtime, or an executable named "plugin" ("plugin.exe" on Windows) for the
// process runtime.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/protobuf/encoding/protojson"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// File is the manifest's file name within a plugin directory.
const File = "manifest.json"

// APIVersion is the contract version this host implements. Plugins built
// against the same major version are compatible.
const APIVersion = "1.0"

var (
	idPattern      = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9]+(-[a-z0-9]+)*)+$`)
	versionPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	hostPattern    = regexp.MustCompile(`^(\*\.)?([a-z0-9-]+\.)*[a-z0-9-]+(:\d+)?$`)
)

// Load reads and validates the manifest of the plugin in dir.
func Load(dir string) (*pluginv1.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, File))
	if err != nil {
		return nil, err
	}
	m := &pluginv1.Manifest{}
	if err := protojson.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", File, err)
	}
	if err := Validate(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Validate checks a manifest.
func Validate(m *pluginv1.Manifest) error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if !idPattern.MatchString(m.GetId()) {
		add("id %q must be a reverse-DNS identifier such as org.example.my-plugin", m.GetId())
	}
	if m.GetName() == "" {
		add("name is required")
	}
	if !versionPattern.MatchString(m.GetVersion()) {
		add("version %q must be a semantic version", m.GetVersion())
	}
	switch m.GetRuntime() {
	case pluginv1.Runtime_RUNTIME_WASM, pluginv1.Runtime_RUNTIME_PROCESS:
	default:
		add("runtime must be RUNTIME_WASM or RUNTIME_PROCESS")
	}
	if len(m.GetCapabilities()) == 0 {
		add("at least one capability is required")
	}
	for _, c := range m.GetCapabilities() {
		if c == pluginv1.Capability_CAPABILITY_UNSPECIFIED || pluginv1.Capability_name[int32(c)] == "" {
			add("unknown capability %v", c)
		}
	}
	if major, _, _ := strings.Cut(m.GetApiVersion(), "."); major != strings.Split(APIVersion, ".")[0] {
		add("api_version %q is not compatible with host API %s", m.GetApiVersion(), APIVersion)
	}
	for _, h := range m.GetPermissions().GetHttpHosts() {
		if !hostPattern.MatchString(h) {
			add("http host %q must be a host name, optionally prefixed by *. and suffixed by :port", h)
		}
	}
	for _, p := range m.GetPermissions().GetReadPaths() {
		if !filepath.IsAbs(p) {
			add("read path %q must be absolute", p)
		}
	}
	if s := m.GetConfigSchema(); s != "" {
		if _, err := CompileSchema(s); err != nil {
			add("config_schema: %v", err)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid manifest %s: %s", m.GetId(), strings.Join(problems, "; "))
	}
	return nil
}

// HasCapability reports whether the manifest declares c.
func HasCapability(m *pluginv1.Manifest, c pluginv1.Capability) bool {
	return slices.Contains(m.GetCapabilities(), c)
}

// Executable returns the path of the plugin's executable or module in dir.
func Executable(dir string, m *pluginv1.Manifest) string {
	if m.GetRuntime() == pluginv1.Runtime_RUNTIME_WASM {
		return filepath.Join(dir, "plugin.wasm")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "plugin.exe")
	}
	return filepath.Join(dir, "plugin")
}

// AllowsHost reports whether a host name (with optional port) matches the
// manifest's HTTP permissions. "*.example.com" matches subdomains of
// example.com but not example.com itself.
func AllowsHost(m *pluginv1.Manifest, host string) bool {
	host = strings.ToLower(host)
	for _, pattern := range m.GetPermissions().GetHttpHosts() {
		pattern = strings.ToLower(pattern)
		if suffix, ok := strings.CutPrefix(pattern, "*"); ok {
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

// Schema validates plugin configuration.
type Schema struct{ s *jsonschema.Schema }

// CompileSchema compiles a JSON Schema document.
func CompileSchema(doc string) (*Schema, error) {
	parsed, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("config.json", parsed); err != nil {
		return nil, err
	}
	s, err := c.Compile("config.json")
	if err != nil {
		return nil, err
	}
	return &Schema{s}, nil
}

// Validate checks a configuration document against the schema.
func (s *Schema) Validate(configJSON string) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(configJSON)))
	if err != nil {
		return fmt.Errorf("config is not valid JSON: %w", err)
	}
	return s.s.Validate(doc)
}

// ValidateConfig checks configJSON against the manifest's schema; a manifest
// without a schema accepts any JSON object.
func ValidateConfig(m *pluginv1.Manifest, configJSON string) error {
	if m.GetConfigSchema() == "" {
		var obj map[string]any
		if err := json.Unmarshal([]byte(configJSON), &obj); err != nil {
			return fmt.Errorf("config must be a JSON object: %w", err)
		}
		return nil
	}
	s, err := CompileSchema(m.GetConfigSchema())
	if err != nil {
		return err
	}
	return s.Validate(configJSON)
}

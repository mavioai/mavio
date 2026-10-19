package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"uuid"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// OptionsKeyword is the configuration schema's keyword naming where the
// values of a string property come from, an object of the source and,
// optionally, the sibling properties taking an option's settings and name:
//
//	"definition": {"type": "string", "x-mavio-options":
//		{"source": "indexer-definitions", "settings": "settings", "name": "name"}}
//
// Clients offer the source's options to choose from, draw the settings
// property by the chosen option's settings schema, and fill an empty name
// property with the option's name.
const OptionsKeyword = "x-mavio-options"

// The sources of options.
const (
	// OptionsIndexerDefinitions are the definitions of the plugin's
	// indexer provider, with their settings.
	OptionsIndexerDefinitions = "indexer-definitions"
	// OptionsDownloadClientDefinitions are the definitions of the plugin's
	// download client provider, with their settings.
	OptionsDownloadClientDefinitions = "download-client-definitions"
	// OptionsSpeechRecognizers are the recognizers the plugin's
	// SPEECH_TO_TEXT service implements, with their settings.
	OptionsSpeechRecognizers = "speech-recognizers"
	// OptionsSpeechSynthesizers are the synthesizers the plugin's
	// TEXT_TO_SPEECH service implements, with their settings.
	OptionsSpeechSynthesizers = "speech-synthesizers"
	// OptionsSubtitleProviders are the subtitle sites the plugin's
	// SUBTITLE_PROVIDER service implements, with their settings.
	OptionsSubtitleProviders = "subtitle-providers"
	// OptionsSubtitleTranslators are the translators the plugin's
	// SUBTITLE_TRANSLATOR service implements, with their settings.
	OptionsSubtitleTranslators = "subtitle-translators"
	// OptionsDownloadClients are the download clients of every download
	// client provider, as "<plugin>/<client>".
	OptionsDownloadClients = "download-clients"
	// OptionsProxies are the server's proxies, by ID. A plugin taking them
	// reads them with HostService.ListProxies and is configured again when
	// they change.
	OptionsProxies = "proxies"
)

// GenerateIDs returns configJSON with every property its schema marks
// generated, a string of format uuid that is readOnly, set to a new UUID
// where it is missing or empty; the values it has are kept. Properties
// are found through properties and items.
func GenerateIDs(m *pluginv1.Manifest, configJSON string) (string, error) {
	if m.GetConfigSchema() == "" {
		return configJSON, nil
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(m.GetConfigSchema()), &schema); err != nil {
		return "", fmt.Errorf("config schema: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(configJSON)))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return "", fmt.Errorf("config is not valid JSON: %w", err)
	}
	if !generate(schema, doc) {
		return configJSON, nil
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// generate sets the generated properties of v, as schema describes it,
// reporting whether it set any.
func generate(schema map[string]any, v any) bool {
	changed := false
	switch v := v.(type) {
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		for k, p := range props {
			ps, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if generated(ps) {
				if s, _ := v[k].(string); s == "" {
					v[k] = uuid.NewV7().String()
					changed = true
				}
				continue
			}
			if child, ok := v[k]; ok && generate(ps, child) {
				changed = true
			}
		}
	case []any:
		items, ok := schema["items"].(map[string]any)
		if !ok {
			return false
		}
		for _, item := range v {
			if generate(items, item) {
				changed = true
			}
		}
	}
	return changed
}

// generated reports whether a property's schema marks it generated.
func generated(s map[string]any) bool {
	return s["readOnly"] == true && s["format"] == "uuid"
}

// UsesOptions reports whether the manifest's configuration schema has a
// property taking its values from source, found through properties and
// items.
func UsesOptions(m *pluginv1.Manifest, source string) bool {
	if m.GetConfigSchema() == "" {
		return false
	}
	var schema map[string]any
	if json.Unmarshal([]byte(m.GetConfigSchema()), &schema) != nil {
		return false
	}
	return usesOptions(schema, source)
}

func usesOptions(schema map[string]any, source string) bool {
	if o, ok := schema[OptionsKeyword].(map[string]any); ok && o["source"] == source {
		return true
	}
	if items, ok := schema["items"].(map[string]any); ok && usesOptions(items, source) {
		return true
	}
	props, _ := schema["properties"].(map[string]any)
	for _, p := range props {
		if ps, ok := p.(map[string]any); ok && usesOptions(ps, source) {
			return true
		}
	}
	return false
}

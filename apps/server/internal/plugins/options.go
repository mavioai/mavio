package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/host"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// ConfigOption is a value a property of a plugin's configuration takes,
// from the source its schema names (manifest.OptionsKeyword).
type ConfigOption struct {
	Value, Name, Description string
	// Tags set the option apart: its protocol, privacy and language.
	Tags []string
	// SettingsSchema is the JSON Schema of the settings the option takes;
	// empty for none.
	SettingsSchema string
}

// Definitions returns the definitions of a started plugin's provider, the
// options of manifest.OptionsIndexerDefinitions or
// manifest.OptionsDownloadClientDefinitions.
func (m *Manager) Definitions(ctx context.Context, id, source string) ([]ConfigOption, error) {
	pl, man, err := m.started(id)
	if err != nil {
		return nil, err
	}
	var out []ConfigOption
	switch source {
	case manifest.OptionsIndexerDefinitions:
		if !manifest.HasCapability(man, pluginv1.Capability_CAPABILITY_INDEXER_PROVIDER) || pl.Indexers() == nil {
			return nil, fmt.Errorf("%w: plugin %s provides no indexers", core.ErrInvalid, id)
		}
		resp, err := pl.Indexers().ListDefinitions(ctx, &pluginv1.ListDefinitionsRequest{})
		if err != nil {
			return nil, fmt.Errorf("list the definitions of plugin %s: %w", id, err)
		}
		for _, d := range resp.GetDefinitions() {
			out = append(out, ConfigOption{
				Value: d.GetId(), Name: d.GetName(), Description: d.GetDescription(),
				Tags:           tags(d.GetProtocol(), d.GetPrivacy(), d.GetLanguage()),
				SettingsSchema: SettingsSchema(d.GetFields()),
			})
		}
	case manifest.OptionsDownloadClientDefinitions:
		if !manifest.HasCapability(man, pluginv1.Capability_CAPABILITY_DOWNLOAD_CLIENT_PROVIDER) || pl.DownloadClients() == nil {
			return nil, fmt.Errorf("%w: plugin %s provides no download clients", core.ErrInvalid, id)
		}
		resp, err := pl.DownloadClients().ListDownloadClientDefinitions(ctx, &pluginv1.ListDownloadClientDefinitionsRequest{})
		if err != nil {
			return nil, fmt.Errorf("list the definitions of plugin %s: %w", id, err)
		}
		for _, d := range resp.GetDefinitions() {
			out = append(out, ConfigOption{
				Value: d.GetId(), Name: d.GetName(), Description: d.GetDescription(),
				Tags:           tags(d.GetProtocol(), pluginv1.IndexerPrivacy_INDEXER_PRIVACY_UNSPECIFIED, ""),
				SettingsSchema: SettingsSchema(d.GetFields()),
			})
		}
	default:
		return nil, fmt.Errorf("%w: unknown source of options %q", core.ErrInvalid, source)
	}
	return out, nil
}

// started returns a started plugin, configured or not, with its manifest.
func (m *Manager) started(id string) (host.Plugin, *pluginv1.Manifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(id)
	if err != nil {
		return nil, nil, err
	}
	if e.plugin == nil {
		return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("plugin %s is not running: %w", id, e.err))
	}
	return e.plugin, e.manifest, nil
}

var (
	protocolTags = map[pluginv1.DownloadProtocol]string{
		pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT: "torrent",
		pluginv1.DownloadProtocol_DOWNLOAD_PROTOCOL_USENET:  "usenet",
	}
	privacyTags = map[pluginv1.IndexerPrivacy]string{
		pluginv1.IndexerPrivacy_INDEXER_PRIVACY_PUBLIC:       "public",
		pluginv1.IndexerPrivacy_INDEXER_PRIVACY_SEMI_PRIVATE: "semi-private",
		pluginv1.IndexerPrivacy_INDEXER_PRIVACY_PRIVATE:      "private",
	}
)

// ProtocolTag returns the tag of a download protocol, "" for none.
func ProtocolTag(p pluginv1.DownloadProtocol) string { return protocolTags[p] }

// tags returns the tags of an option's protocol, privacy and language.
func tags(protocol pluginv1.DownloadProtocol, privacy pluginv1.IndexerPrivacy, language string) []string {
	var out []string
	for _, t := range []string{protocolTags[protocol], privacyTags[privacy], language} {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// SettingsSchema returns the JSON Schema of an object of settings, its
// properties in the fields' order: a select as constants with titles,
// a secret writeOnly, and text shown to administrators a readOnly
// property of no type. It returns "" for no fields.
func SettingsSchema(fields []*pluginv1.SettingField) string {
	if len(fields) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString(`{"type":"object","properties":{`)
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSON(&b, f.GetName())
		b.WriteByte(':')
		var p members
		p.add("title", f.GetLabel())
		if f.GetHelpText() != "" {
			p.add("description", f.GetHelpText())
		}
		choices := func() []any {
			var out []any
			for _, o := range f.GetOptions() {
				var v any
				if json.Unmarshal([]byte(o.GetValueJson()), &v) == nil {
					out = append(out, map[string]any{"const": v, "title": o.GetLabel()})
				}
			}
			return out
		}
		switch f.GetType() {
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_NUMBER:
			p.add("type", "number")
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_CHECKBOX:
			p.add("type", "boolean")
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_SELECT:
			p.add("oneOf", choices())
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_MULTI_SELECT:
			p.add("type", "array")
			p.add("items", map[string]any{"oneOf": choices()})
			p.add("uniqueItems", true)
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_TAG:
			p.add("type", "array")
			p.add("items", map[string]any{"type": "string"})
		case pluginv1.SettingFieldType_SETTING_FIELD_TYPE_INFO:
			p.add("readOnly", true)
		default:
			p.add("type", "string")
		}
		var def any
		if f.GetDefaultJson() != "" && json.Unmarshal([]byte(f.GetDefaultJson()), &def) == nil && def != nil {
			p.add("default", def)
		}
		if f.GetSecret() {
			p.add("writeOnly", true)
		}
		p.write(&b)
	}
	b.WriteString("}}")
	return b.String()
}

// members are the members of a JSON object, in order.
type members []struct {
	key   string
	value any
}

func (m *members) add(key string, value any) {
	*m = append(*m, struct {
		key   string
		value any
	}{key, value})
}

func (m members) write(b *bytes.Buffer) {
	b.WriteByte('{')
	for i, kv := range m {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSON(b, kv.key)
		b.WriteByte(':')
		writeJSON(b, kv.value)
	}
	b.WriteByte('}')
}

// writeJSON writes v as JSON, leaving <, > and & as they are.
func writeJSON(b *bytes.Buffer, v any) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	b.Truncate(b.Len() - 1) // Encode's newline
}

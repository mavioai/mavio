package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/language"
	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// LocalesDir is the folder of a plugin's translations within its folder:
// a file per language, named by its BCP 47 tag, such as zh-Hans.json.
const LocalesDir = "locales"

// SourceLanguage is the language of a manifest's strings.
const SourceLanguage = "en"

// Locales are a plugin's translations: by BCP 47 tag, a translation of
// each of its strings, keyed by the string in English.
type Locales map[string]map[string]string

// LoadLocales reads the translations of the plugin in dir, none when it
// has no locales folder.
func LoadLocales(dir string) (Locales, error) {
	files, err := os.ReadDir(filepath.Join(dir, LocalesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := Locales{}
	for _, f := range files {
		tag, ok := strings.CutSuffix(f.Name(), ".json")
		if f.IsDir() || !ok {
			continue
		}
		if t, err := language.Parse(tag); err != nil || t.String() != tag || tag == SourceLanguage {
			return nil, fmt.Errorf("%s/%s: %q must be a BCP 47 tag in canonical form, other than %s", LocalesDir, f.Name(), tag, SourceLanguage)
		}
		data, err := os.ReadFile(filepath.Join(dir, LocalesDir, f.Name()))
		if err != nil {
			return nil, err
		}
		var strs map[string]string
		if err := json.Unmarshal(data, &strs); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", LocalesDir, f.Name(), err)
		}
		for k, v := range strs {
			if k == "" || v == "" {
				return nil, fmt.Errorf("%s/%s: %q: strings and translations must not be empty", LocalesDir, f.Name(), k)
			}
		}
		out[tag] = strs
	}
	return out, nil
}

// Match returns the tag among tags best matching the languages wanted,
// most wanted first, or "" when English matches better or none matches.
func Match(tags []string, wanted []language.Tag) string {
	if len(tags) == 0 || len(wanted) == 0 {
		return ""
	}
	tags = slices.Sorted(slices.Values(tags))
	supported := []language.Tag{language.Make(SourceLanguage)}
	for _, t := range tags {
		supported = append(supported, language.Make(t))
	}
	_, i, c := language.NewMatcher(supported).Match(wanted...)
	if i == 0 || c == language.No {
		return ""
	}
	return tags[i-1]
}

// Translator returns the translation of the plugin's strings into the
// language best matching those wanted, most wanted first; it returns
// strings it has no translation of, and every string when English matches
// best, as they are.
func (l Locales) Translator(wanted []language.Tag) func(string) string {
	strs := l[Match(slices.Collect(maps.Keys(l)), wanted)]
	return func(s string) string {
		if t, ok := strs[s]; ok {
			return t
		}
		return s
	}
}

// Localize returns a copy of m with its strings translated by tr: its
// name and description, its tasks' names and descriptions, the names of
// its kinds of external IDs, and the titles and descriptions of its
// configuration schema.
func Localize(m *pluginv1.Manifest, tr func(string) string) *pluginv1.Manifest {
	out := proto.Clone(m).(*pluginv1.Manifest)
	out.SetName(tr(m.GetName()))
	if m.HasDescription() {
		out.SetDescription(tr(m.GetDescription()))
	}
	for _, t := range out.GetTasks() {
		t.SetName(tr(t.GetName()))
		if t.HasDescription() {
			t.SetDescription(tr(t.GetDescription()))
		}
	}
	for _, k := range out.GetExternalIdKinds() {
		k.SetName(tr(k.GetName()))
	}
	if m.GetConfigSchema() != "" {
		out.SetConfigSchema(localizeSchema(m.GetConfigSchema(), tr))
	}
	return out
}

// Strings returns the strings of m that locales translate, in the order
// Localize meets them, each once.
func Strings(m *pluginv1.Manifest) []string {
	var out []string
	Localize(m, func(s string) string {
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
		return s
	})
	return out
}

// localizeSchema returns a JSON Schema with its titles and descriptions
// translated by tr, its members kept in their order, which forms follow;
// a schema that cannot be read is returned as it is.
func localizeSchema(schema string, tr func(string) string) string {
	dec := json.NewDecoder(strings.NewReader(schema))
	dec.UseNumber()
	var b bytes.Buffer
	tok, err := dec.Token()
	if err == nil {
		err = copyJSON(dec, &b, tok, isSchema, tr)
	}
	if err == nil {
		if _, end := dec.Token(); end != io.EOF {
			err = errors.New("trailing data")
		}
	}
	if err != nil {
		return schema
	}
	return b.String()
}

// What a JSON value of a schema holds.
type schemaPart int

const (
	isData         schemaPart = iota // a value, such as a default
	isSchema                         // a schema
	isSchemaMap                      // schemas by name, such as properties
	isSchemaList                     // a list of schemas, such as anyOf
	isSchemaOrList                   // a schema, or a list of them (items)
)

// subschemas are the keywords whose values hold schemas.
var subschemas = map[string]schemaPart{
	"properties": isSchemaMap, "patternProperties": isSchemaMap, "$defs": isSchemaMap, "definitions": isSchemaMap,
	"dependentSchemas": isSchemaMap,
	"items":            isSchemaOrList, "additionalItems": isSchema, "additionalProperties": isSchema,
	"unevaluatedItems": isSchema, "unevaluatedProperties": isSchema, "contains": isSchema, "propertyNames": isSchema,
	"not": isSchema, "if": isSchema, "then": isSchema, "else": isSchema,
	"allOf": isSchemaList, "anyOf": isSchemaList, "oneOf": isSchemaList, "prefixItems": isSchemaList,
}

// copyJSON writes the value starting with tok to b, translating the
// titles and descriptions of the schemas it holds.
func copyJSON(dec *json.Decoder, b *bytes.Buffer, tok json.Token, part schemaPart, tr func(string) string) error {
	switch d := tok.(type) {
	case json.Delim:
		closing := byte('}')
		if d == '[' {
			closing = ']'
		}
		b.WriteByte(byte(d))
		for i := 0; dec.More(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			child := isData
			if d == '[' && (part == isSchemaList || part == isSchemaOrList) {
				child = isSchema
			}
			if d == '{' {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				key, _ := k.(string)
				writeString(b, key)
				b.WriteByte(':')
				switch part {
				case isSchemaMap:
					child = isSchema
				case isSchema, isSchemaOrList:
					child = subschemas[key]
				}
				v, err := dec.Token()
				if err != nil {
					return err
				}
				if s, ok := v.(string); ok && (part == isSchema || part == isSchemaOrList) && (key == "title" || key == "description") {
					v = tr(s)
				}
				if err := copyJSON(dec, b, v, child, tr); err != nil {
					return err
				}
				continue
			}
			v, err := dec.Token()
			if err != nil {
				return err
			}
			if err := copyJSON(dec, b, v, child, tr); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return err
		}
		b.WriteByte(closing)
	case string:
		writeString(b, d)
	case json.Number:
		b.WriteString(d.String())
	case bool:
		fmt.Fprint(b, d)
	case nil:
		b.WriteString("null")
	}
	return nil
}

// writeString writes s as JSON, leaving <, > and & as they are.
func writeString(b *bytes.Buffer, s string) {
	enc := json.NewEncoder(b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b.Truncate(b.Len() - 1) // Encode's newline
}

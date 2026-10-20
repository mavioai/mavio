package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// schema is the subset of JSON Schema the form draws, as plugins describe
// their configuration (docs/clients.md §6.3.3).
type schema struct {
	types       []string
	title       string
	description string
	format      string
	properties  []namedSchema
	required    []string
	items       *schema
	enum        []any
	def         any
	hasDefault  bool
	minimum     *float64
	maximum     *float64
	writeOnly   bool
	readOnly    bool
	oneOf       []*schema
	constant    any
	hasConst    bool
	options     *optionsSpec
	// requires is the sibling property whose value names the container
	// dependency a switch needs on.
	requires string
}

// namedSchema is a property's schema, by its name.
type namedSchema struct {
	name   string
	schema *schema
}

// optionsSpec is where a property's values come from, and the sibling
// properties a chosen option fills: settings takes the chosen option's
// settings, by their schema, and name the option's name while it has
// none of its own.
type optionsSpec struct {
	Source   string `json:"source"`
	Settings string `json:"settings"`
	Name     string `json:"name"`
}

// UnmarshalJSON reads a schema, keeping its properties in order.
func (s *schema) UnmarshalJSON(b []byte) error {
	var raw struct {
		Type              json.RawMessage `json:"type"`
		Title             string          `json:"title"`
		Description       string          `json:"description"`
		Format            string          `json:"format"`
		Properties        json.RawMessage `json:"properties"`
		Required          []string        `json:"required"`
		Items             *schema         `json:"items"`
		Enum              []any           `json:"enum"`
		Default           json.RawMessage `json:"default"`
		Minimum           *float64        `json:"minimum"`
		Maximum           *float64        `json:"maximum"`
		WriteOnly         bool            `json:"writeOnly"`
		ReadOnly          bool            `json:"readOnly"`
		OneOf             []*schema       `json:"oneOf"`
		Const             json.RawMessage `json:"const"`
		Options           *optionsSpec    `json:"x-mavio-options"`
		RequiresContainer string          `json:"x-mavio-requires-container"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*s = schema{
		title: raw.Title, description: raw.Description, format: raw.Format, required: raw.Required, items: raw.Items,
		enum: raw.Enum, minimum: raw.Minimum, maximum: raw.Maximum, writeOnly: raw.WriteOnly, readOnly: raw.ReadOnly,
		oneOf: raw.OneOf, options: raw.Options, requires: raw.RequiresContainer,
	}
	if len(raw.Type) > 0 {
		var one string
		if json.Unmarshal(raw.Type, &one) == nil {
			s.types = []string{one}
		} else if err := json.Unmarshal(raw.Type, &s.types); err != nil {
			return fmt.Errorf("type: %w", err)
		}
	}
	if len(raw.Default) > 0 {
		s.hasDefault = true
		if err := json.Unmarshal(raw.Default, &s.def); err != nil {
			return fmt.Errorf("default: %w", err)
		}
	}
	if len(raw.Const) > 0 {
		s.hasConst = true
		if err := json.Unmarshal(raw.Const, &s.constant); err != nil {
			return fmt.Errorf("const: %w", err)
		}
	}
	if len(raw.Properties) > 0 && !bytes.Equal(raw.Properties, []byte("null")) {
		d := json.NewDecoder(bytes.NewReader(raw.Properties))
		if _, err := d.Token(); err != nil {
			return fmt.Errorf("properties: %w", err)
		}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return fmt.Errorf("properties: %w", err)
			}
			p := &schema{}
			if err := d.Decode(p); err != nil {
				return fmt.Errorf("property %v: %w", k, err)
			}
			s.properties = append(s.properties, namedSchema{name: fmt.Sprint(k), schema: p})
		}
	}
	return nil
}

// parseSchema returns a schema of JSON, nil when it does not parse.
func parseSchema(text string) *schema {
	s := &schema{}
	if json.Unmarshal([]byte(text), s) != nil {
		return nil
	}
	return s
}

// optionSources returns the sources of options a schema names.
func optionSources(s *schema) []string {
	var out []string
	var walk func(s *schema)
	walk = func(s *schema) {
		if s == nil {
			return
		}
		if s.options != nil && !slices.Contains(out, s.options.Source) {
			out = append(out, s.options.Source)
		}
		for _, p := range s.properties {
			walk(p.schema)
		}
		walk(s.items)
	}
	walk(s)
	return out
}

// generated reports whether the server generates a property: a readOnly
// UUID.
func (s *schema) generated() bool { return s.readOnly && s.format == "uuid" }

// schemaChoice is a choice of an enumeration.
type schemaChoice struct {
	value any
	label string
}

// choices returns the choices of an enumeration, of enum or of oneOf's
// constants with their titles, nil when it is none.
func (s *schema) choices() []schemaChoice {
	if s.enum != nil {
		out := make([]schemaChoice, len(s.enum))
		for i, v := range s.enum {
			out[i] = schemaChoice{v, jsonLabel(v)}
		}
		return out
	}
	if len(s.oneOf) == 0 || slices.ContainsFunc(s.oneOf, func(o *schema) bool { return !o.hasConst }) {
		return nil
	}
	out := make([]schemaChoice, len(s.oneOf))
	for i, o := range s.oneOf {
		out[i] = schemaChoice{o.constant, cmpOr(o.title, jsonLabel(o.constant))}
	}
	return out
}

// typeOf returns the type a schema's value takes: the first of its types
// but null, else "enum" or "object" as its keys show.
func (s *schema) typeOf() string {
	for _, t := range s.types {
		if t != "null" {
			return t
		}
	}
	switch {
	case s.choices() != nil:
		return "enum"
	case s.properties != nil:
		return "object"
	}
	return ""
}

func (s *schema) isRequired(name string) bool { return slices.Contains(s.required, name) }

// jsonLabel returns a value as text: a string as it is, else its JSON.
func jsonLabel(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func jsonEqual(a, b any) bool { return jsonLabel(a) == jsonLabel(b) }

var (
	labelSeparators = regexp.MustCompile(`[_-]+`)
	labelCamel      = regexp.MustCompile(`([a-z])([A-Z])`)
	labelAcronyms   = regexp.MustCompile(`\b(api|url|id|ip|ssdp|http|https|dns|tls|rss|nzb|tmdb|imdb|tvdb)\b`)
	secretKey       = regexp.MustCompile(`(?i)(^|_)(password|secret|token|api_key|key|passkey)$`)
)

// labelOf returns a property's title, else its key as words: api_key
// becomes "API key".
func labelOf(key string, s *schema) string {
	if s != nil && s.title != "" {
		return s.title
	}
	w := labelSeparators.ReplaceAllString(key, " ")
	w = strings.ToLower(labelCamel.ReplaceAllString(w, "$1 $2"))
	w = labelAcronyms.ReplaceAllStringFunc(w, strings.ToUpper)
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}

// secret reports whether a property holds a secret, to draw as a
// password.
func (s *schema) secret(key string) bool {
	return s.writeOnly || s.format == "password" || secretKey.MatchString(key)
}

// initial returns an empty value of a schema: an object with its required
// properties' defaults, an empty list, else its default.
func (s *schema) initial() any {
	switch s.typeOf() {
	case "object":
		o := map[string]any{}
		for _, p := range s.properties {
			if s.isRequired(p.name) && p.schema.hasDefault {
				o[p.name] = p.schema.def
			}
		}
		return o
	case "array":
		return []any{}
	}
	return s.def
}

// schemaForm draws a form of a JSON Schema, as the web UI's: objects as
// groups of rows, arrays of objects as cards to add and remove, lists of
// strings as lines, enumerations as choices, values of a source the schema
// names (x-mavio-options) as a searchable choice whose option gives its
// settings' schema, and what has no shape the form knows, such as an
// object of arbitrary keys, as JSON. Properties the server generates are
// left out; a switch needing a container dependency
// (x-mavio-requires-container) stays off until the dependency is set up.
type schemaForm struct {
	c *ui.Context
	// options returns the options of a source, nil while they load or
	// when they cannot be had.
	options func(source string) []*systemv1.PluginConfigOption
	// unset reports whether a container dependency is not set up, by its
	// name.
	unset func(name string) bool
}

// draw builds the form of a configuration object, calling set with the
// value changed.
func (f *schemaForm) draw(s *schema, value any, set func(any)) {
	o, _ := value.(map[string]any)
	f.objectFields("", s, o, func(v map[string]any) { set(v) }, true)
}

// objectFields builds an object's properties as the rows of a group; top
// is the form's own group, a nested object's is bordered.
func (f *schemaForm) objectFields(path string, s *schema, value map[string]any, set func(map[string]any), top bool) {
	c := f.c
	colors := style.Colors(c)
	var props []namedSchema
	for _, p := range s.properties {
		if !p.schema.generated() {
			props = append(props, p)
		}
	}
	if len(props) == 0 {
		f.jsonField(path, value, func(v any) {
			o, _ := v.(map[string]any)
			set(o)
		})
		return
	}
	put := func(k string, v any) {
		next := cloneObject(value)
		if v == nil {
			delete(next, k)
		} else {
			next[k] = v
		}
		set(next)
	}
	// The properties whose schema a chosen option gives, nil when it takes
	// no settings.
	given := map[string]*schema{}
	for _, p := range props {
		spec := p.schema.options
		if spec == nil || spec.Settings == "" {
			continue
		}
		i := slices.IndexFunc(f.options(spec.Source), func(o *systemv1.PluginConfigOption) bool { return jsonEqual(o.GetValue(), value[p.name]) })
		if i < 0 {
			continue
		}
		chosen := f.options(spec.Source)[i]
		var gs *schema
		if chosen.GetSettingsSchema() != "" {
			gs = parseSchema(chosen.GetSettingsSchema())
		}
		given[spec.Settings] = gs
	}
	box := ui.Column(c.Key("fields" + path)).Clip()
	if top {
		box.Radius(14).Background(style.Color(colors.Surface))
	} else {
		box.Radius(12).Border(1, style.Color(colors.Border))
	}
	box.Children(func() {
		g := &settingsGroup{c: c}
		for _, p := range props {
			k, ps := p.name, p.schema
			at := path + "/" + k
			if gs, ok := given[k]; ok {
				if gs == nil || len(gs.properties) == 0 {
					continue
				}
				shown := *gs
				shown.title, shown.description = ps.title, ps.description
				f.property(g, at, k, &shown, true, value[k], func(v any) { put(k, v) }, nil, nil, "")
				continue
			}
			if spec := ps.options; spec != nil {
				if list := f.options(spec.Source); list != nil {
					f.property(g, at, k, ps, s.isRequired(k), value[k], func(v any) { put(k, v) }, list, func(v string) {
						f.choose(value, set, k, spec, list, v)
					}, "")
					continue
				}
			}
			waiting := ""
			if ps.requires != "" {
				if dep, ok := value[ps.requires].(string); ok && dep != "" && f.unset(dep) {
					waiting = dep
				}
			}
			f.property(g, at, k, ps, s.isRequired(k), value[k], func(v any) { put(k, v) }, nil, nil, waiting)
		}
	})
}

// choose sets a property to an option: choosing another leaves the
// settings of the one before, and names the object after it while it
// bears that one's name or none.
func (f *schemaForm) choose(value map[string]any, set func(map[string]any), k string, spec *optionsSpec, list []*systemv1.PluginConfigOption, v string) {
	find := func(x any) *systemv1.PluginConfigOption {
		for _, o := range list {
			if jsonEqual(o.GetValue(), x) {
				return o
			}
		}
		return nil
	}
	before, after := find(value[k]), find(v)
	next := cloneObject(value)
	next[k] = v
	if spec.Settings != "" && before.GetValue() != after.GetValue() {
		delete(next, spec.Settings)
	}
	if spec.Name != "" && after != nil {
		if n, _ := next[spec.Name].(string); n == "" || n == before.GetName() {
			next[spec.Name] = after.GetName()
		}
	}
	set(next)
}

func cloneObject(o map[string]any) map[string]any {
	next := make(map[string]any, len(o)+1)
	for k, v := range o {
		next[k] = v
	}
	return next
}

// property builds a property's row: its label and description, and its
// control beside or below them. list and choose are the options of the
// source its schema names; waiting the container dependency not set up
// that a switch waits for, which keeps it off.
func (f *schemaForm) property(g *settingsGroup, path, name string, s *schema, required bool, value any, set func(any),
	list []*systemv1.PluginConfigOption, choose func(string), waiting string,
) {
	c := f.c
	colors := style.Colors(c)
	label := labelOf(name, s)
	head := func(detail string, control func()) {
		g.divider()
		ui.Row(c.Key(path)).MinHeight(52).Padding(10, 16).Gap(16).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(224).Children(func() {
				ui.Row(c).Children(func() {
					style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
					if required {
						style.Text(ui.Text(c, " *"), style.Type(c).Body).TextColor(style.Color(colors.Danger))
					}
				})
				if detail != "" {
					style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				}
			})
			if control != nil {
				control()
			}
		})
	}
	below := func(control func()) {
		head(s.description, nil)
		ui.Column(c.Key(path+"#below")).Padding(0, 16, 12).Children(control)
	}
	if list != nil && choose != nil {
		head(s.description, func() { f.optionPicker(path, label, s, value, list, choose) })
		return
	}
	t := s.typeOf()
	// Text shown to administrators, which takes no value.
	if s.readOnly && t == "" {
		head(s.description, nil)
		return
	}
	switch t {
	case "boolean":
		if waiting != "" {
			head(style.T(c, "schema.needsContainer", "name", waiting), func() {
				off := false
				ui.Switch(c, &off).Label(label).Disabled(true)
			})
			return
		}
		head(s.description, func() {
			on, ok := value.(bool)
			if !ok {
				on = s.def == true
			}
			if ui.Switch(c, &on).Label(label).Changed() {
				set(on)
			}
		})
	case "enum", "string":
		if s.choices() != nil {
			head(s.description, func() { f.enumField(label, s, required, value, set) })
			return
		}
		head(s.description, func() { f.textField(path, label, s.secret(name), s, required, value, set) })
	case "integer", "number":
		head(s.description, func() { f.numberField(path, label, t == "integer", s, value, set) })
	case "object":
		if s.properties == nil {
			below(func() { f.jsonField(path, cmpAny(value, map[string]any{}), set) })
			return
		}
		// An optional object is set or left out as a whole.
		o, isSet := value.(map[string]any)
		var control func()
		if !required {
			control = func() {
				on := isSet
				if ui.Switch(c, &on).Label(label).Changed() {
					if on {
						set(s.initial())
					} else {
						set(nil)
					}
				}
			}
		}
		head(s.description, control)
		if isSet || required {
			ui.Column(c.Key(path+"#object")).Padding(0, 16, 12).Children(func() {
				f.objectFields(path, s, o, func(v map[string]any) { set(v) }, false)
			})
		}
	case "array":
		items := s.items
		if items == nil {
			items = &schema{}
		}
		values, _ := value.([]any)
		orNil := func(v []any) any {
			if len(v) == 0 {
				return nil
			}
			return v
		}
		it := items.typeOf()
		switch {
		case it == "object" && items.properties != nil:
			below(func() { f.objectList(path, items, values, func(v []any) { set(orNil(v)) }) })
		case len(items.choices()) > tagsAbove:
			shown := values
			if len(shown) == 0 {
				shown, _ = s.def.([]any)
			}
			below(func() { f.tagSet(path, label, items, shown, func(v []any) { set(orNil(v)) }) })
		case items.choices() != nil:
			below(func() { f.enumSet(items, values, func(v []any) { set(orNil(v)) }) })
		case it == "string" || it == "integer" || it == "number":
			below(func() { f.lines(path, label, it != "string", values, func(v []any) { set(orNil(v)) }) })
		default:
			below(func() { f.jsonField(path, cmpAny(value, []any{}), set) })
		}
	default:
		below(func() { f.jsonField(path, value, set) })
	}
}

// cmpAny returns v, or or when v is nil.
func cmpAny(v, or any) any {
	if v == nil {
		return or
	}
	return v
}

// tagsAbove is how many options a multiple choice shows as checkboxes;
// more are tags.
const tagsAbove = 8

// textField builds a text input, revealed by a button when it holds a
// secret.
func (f *schemaForm) textField(path, label string, secret bool, s *schema, required bool, value any, set func(any)) {
	c := f.c
	colors := style.Colors(c)
	text, _ := value.(string)
	row := ui.Row(c.Key(path + "#text")).Width(320).Gap(4).AlignItems(ui.Center)
	shown := ui.Local(row, "shown", func() bool { return false })
	row.Children(func() {
		in := ui.TextInput(c, &text).Grow(1).Label(label)
		if d, ok := s.def.(string); ok {
			in.Placeholder(d)
		}
		if secret && !*shown {
			in.Password()
		}
		if in.Changed() {
			if text == "" && !required {
				set(nil)
			} else {
				set(text)
			}
		}
		if secret {
			tip := style.T(c, "schema.show")
			icon := "key"
			if *shown {
				tip, icon = style.T(c, "schema.hide"), "lock"
			}
			b := ui.ButtonBase(c).Size(28, 28).Radius(6).Center().Label(tip).Tooltip(tip)
			if b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				ui.Icon(c, style.Icon(icon, false)).Size(16, 16).TextColor(style.Color(colors.TextSecondary))
			})
			if b.Clicked() {
				*shown = !*shown
			}
		}
	})
}

// numberField builds a number input, empty while unset.
func (f *schemaForm) numberField(path, label string, integer bool, s *schema, value any, set func(any)) {
	c := f.c
	text := ""
	if n, ok := value.(float64); ok {
		text = strconv.FormatFloat(n, 'f', -1, 64)
	}
	in := ui.TextInput(c.Key(path+"#number"), &text).Width(160).Label(label)
	if s.hasDefault {
		in.Placeholder(jsonLabel(s.def))
	}
	if !in.Changed() {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		set(nil)
		return
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(n) {
		return
	}
	if integer {
		n = math.Trunc(n)
	}
	if s.minimum != nil {
		n = max(n, *s.minimum)
	}
	if s.maximum != nil {
		n = min(n, *s.maximum)
	}
	set(n)
}

// enumField builds a choice of an enumeration; unset shows the default.
func (f *schemaForm) enumField(label string, s *schema, required bool, value any, set func(any)) {
	c := f.c
	options := s.choices()
	index := slices.IndexFunc(options, func(o schemaChoice) bool { return jsonEqual(o.value, value) })
	var labels []string
	var values []any
	if !required || index < 0 {
		empty := "—"
		if s.hasDefault {
			d := jsonLabel(s.def)
			if i := slices.IndexFunc(options, func(o schemaChoice) bool { return jsonEqual(o.value, s.def) }); i >= 0 {
				d = options[i].label
			}
			empty = style.T(c, "schema.default", "value", d)
		}
		labels, values = append(labels, empty), append(values, nil)
	}
	for _, o := range options {
		labels, values = append(labels, o.label), append(values, o.value)
	}
	i := 0
	if index >= 0 {
		i = len(labels) - len(options) + index
	}
	if choose(c, label, &i, labels) {
		set(values[i])
	}
}

// enumSet builds values chosen among an enumeration, as checkboxes.
func (f *schemaForm) enumSet(s *schema, value []any, set func([]any)) {
	c := f.c
	ui.Row(c).Gap(20).Wrap().Children(func() {
		for i, o := range s.choices() {
			on := slices.ContainsFunc(value, func(v any) bool { return jsonEqual(v, o.value) })
			if ui.Checkbox(c.Key(i), &on, o.label).Changed() {
				if on {
					set(append(slices.Clone(value), o.value))
				} else {
					set(slices.DeleteFunc(slices.Clone(value), func(v any) bool { return jsonEqual(v, o.value) }))
				}
			}
		}
	})
}

// tagSet builds values chosen among many, in their order, as tags: each
// removed by its button, but the last, and the others added from a
// searchable choice.
func (f *schemaForm) tagSet(path, label string, s *schema, value []any, set func([]any)) {
	c := f.c
	colors := style.Colors(c)
	options := s.choices()
	title := func(v any) string {
		if i := slices.IndexFunc(options, func(o schemaChoice) bool { return jsonEqual(o.value, v) }); i >= 0 {
			return options[i].label
		}
		return jsonLabel(v)
	}
	var rest []schemaChoice
	for _, o := range options {
		if !slices.ContainsFunc(value, func(v any) bool { return jsonEqual(v, o.value) }) {
			rest = append(rest, o)
		}
	}
	ui.Row(c.Key(path + "#tags")).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		for i, v := range value {
			ui.Row(c.Key(i)).MinHeight(28).Padding(0, 4, 0, 10).Gap(4).Radius(999).AlignItems(ui.Center).
				Background(style.Color(colors.Fill)).Children(func() {
				style.Text(ui.Text(c, title(v)), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary))
				if len(value) < 2 {
					return
				}
				remove := style.T(c, "schema.remove") + " " + title(v)
				b := ui.ButtonBase(c).Size(20, 20).Radius(999).Center().Label(remove)
				if b.Hovered() {
					b.Background(style.Color(colors.FillSelected))
				}
				b.Children(func() {
					ui.Icon(c, style.Icon("close", false)).Size(12, 12).TextColor(style.Color(colors.TextSecondary))
				})
				if b.Clicked() {
					set(slices.Delete(slices.Clone(value), i, i+1))
				}
			})
		}
		if len(rest) == 0 {
			return
		}
		labels := make([]string, len(rest))
		for i, o := range rest {
			labels[i] = o.label
		}
		chosen := ""
		if ui.Combobox(c.Key("add"), &chosen, labels).Width(192).Label(label).Placeholder(style.T(c, "schema.add")).Changed() {
			if i := slices.Index(labels, chosen); i >= 0 {
				set(append(slices.Clone(value), rest[i].value))
			}
		}
	})
}

// optionPicker builds a searchable choice among a source's options, by
// name: the proxies none first, then the system proxy, then the others.
func (f *schemaForm) optionPicker(path, label string, s *schema, value any, list []*systemv1.PluginConfigOption, choose func(string)) {
	c := f.c
	type option struct{ value, label string }
	var options []option
	sorted := slices.Clone(list)
	slices.SortStableFunc(sorted, func(a, b *systemv1.PluginConfigOption) int {
		return strings.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
	})
	if s.options.Source == "proxies" {
		options = append(options, option{"", style.T(c, "proxies.none")})
		if slices.ContainsFunc(list, func(o *systemv1.PluginConfigOption) bool { return o.GetValue() == systemProxyID }) {
			options = append(options, option{systemProxyID, style.T(c, "proxies.system")})
		}
		sorted = slices.DeleteFunc(sorted, func(o *systemv1.PluginConfigOption) bool { return o.GetValue() == systemProxyID })
	}
	seen := map[string]int{}
	for _, o := range sorted {
		l := o.GetName()
		// Options of one name are told apart by their value.
		if seen[l]++; seen[l] > 1 || slices.ContainsFunc(sorted, func(x *systemv1.PluginConfigOption) bool { return x != o && x.GetName() == l }) {
			l += " (" + o.GetValue() + ")"
		}
		options = append(options, option{o.GetValue(), l})
	}
	current, ok := value.(string)
	if !ok {
		current, _ = s.def.(string)
	}
	labels := make([]string, len(options))
	chosen := ""
	for i, o := range options {
		labels[i] = o.label
		if o.value == current && (current != "" || s.options.Source == "proxies") {
			chosen = o.label
		}
	}
	if ui.Combobox(c.Key(path+"#picker"), &chosen, labels).Width(320).Label(label).Placeholder(style.T(c, "schema.choose")).Changed() {
		if i := slices.Index(labels, chosen); i >= 0 {
			choose(options[i].value)
		}
	}
}

// lines builds a list of strings or numbers, one per line.
func (f *schemaForm) lines(path, label string, numeric bool, value []any, set func([]any)) {
	c := f.c
	area := ui.Column(c.Key(path + "#lines"))
	text := ui.Local(area, "text", func() string {
		parts := make([]string, len(value))
		for i, v := range value {
			parts[i] = jsonLabel(v)
		}
		return strings.Join(parts, "\n")
	})
	area.Children(func() {
		in := ui.TextArea(c, text).MinHeight(80).Label(label).Placeholder(style.T(c, "schema.onePerLine")).Font("SF Mono, Menlo, monospace")
		if !in.Changed() {
			return
		}
		var out []any
		for l := range strings.SplitSeq(*text, "\n") {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if !numeric {
				out = append(out, l)
			} else if n, err := strconv.ParseFloat(l, 64); err == nil {
				out = append(out, n)
			}
		}
		set(out)
	})
}

// jsonField builds a value of no shape the form knows, edited as JSON.
func (f *schemaForm) jsonField(path string, value any, set func(any)) {
	c := f.c
	colors := style.Colors(c)
	box := ui.Column(c.Key(path + "#json")).Gap(4)
	type state struct {
		text string
		bad  bool
	}
	st := ui.Local(box, "json", func() state {
		if value == nil {
			return state{}
		}
		b, _ := json.MarshalIndent(value, "", "  ")
		return state{text: string(b)}
	})
	box.Children(func() {
		in := ui.TextArea(c, &st.text).MinHeight(96).Font("SF Mono, Menlo, monospace")
		if st.bad {
			in.Border(1, style.Color(colors.Danger))
		}
		if in.Changed() {
			if strings.TrimSpace(st.text) == "" {
				st.bad = false
				set(nil)
			} else {
				var v any
				if err := json.Unmarshal([]byte(st.text), &v); err != nil {
					st.bad = true
				} else {
					st.bad = false
					set(v)
				}
			}
		}
		if st.bad {
			style.Text(ui.Text(c, style.T(c, "schema.invalidJson")), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
		}
	})
}

// itemTitle returns the title of an object in a list: its name or ID,
// else its place.
func itemTitle(c *ui.Context, v any, i int) string {
	if o, ok := v.(map[string]any); ok {
		for _, k := range []string{"name", "id", "definition", "kind", "host"} {
			if s, ok := o[k].(string); ok && s != "" {
				return s
			}
		}
	}
	return style.T(c, "schema.item", "number", strconv.Itoa(i+1))
}

// objectList builds the objects of a list, as cards that open to their
// fields.
func (f *schemaForm) objectList(path string, s *schema, value []any, set func([]any)) {
	c := f.c
	colors := style.Colors(c)
	list := ui.Column(c.Key(path + "#list")).Gap(8)
	open := ui.Local(list, "open", func() int { return -1 })
	list.Children(func() {
		for i, v := range value {
			// Items have no identity of their own: their place and the
			// list's length key them, so that removing one draws the rest
			// anew.
			ui.Column(c.Key(fmt.Sprintf("%d/%d", i, len(value)))).Radius(12).Border(1, style.Color(colors.Border)).Clip().Children(func() {
				title := itemTitle(c, v, i)
				ui.Row(c).Padding(6, 12).Gap(8).AlignItems(ui.Center).Children(func() {
					b := ui.ButtonBase(c).MinHeight(36).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Justify(ui.Start).Label(title)
					b.Children(func() {
						icon := ui.Icon(c, style.Icon("expand", false)).Size(16, 16).TextColor(style.Color(colors.TextPrimary))
						if *open != i {
							icon.Rotate(-90)
						}
						style.Text(ui.Text(c, title), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
					})
					if b.Clicked() {
						if *open == i {
							*open = -1
						} else {
							*open = i
						}
					}
					if dangerButton(c.Key("remove"), style.T(c, "schema.remove"), false) {
						*open = -1
						set(slices.Delete(slices.Clone(value), i, i+1))
					}
				})
				if *open == i {
					ui.Box(c).Height(1).Background(style.Color(colors.Border))
					o, _ := v.(map[string]any)
					f.objectFields(fmt.Sprintf("%s/%d", path, i), s, o, func(o map[string]any) {
						next := slices.Clone(value)
						next[i] = o
						set(next)
					}, false)
				}
			})
		}
		ui.Row(c.Key("add")).Children(func() {
			if smallButton(c, style.T(c, "schema.add"), false) {
				*open = len(value)
				set(append(slices.Clone(value), s.initial()))
			}
		})
	})
}

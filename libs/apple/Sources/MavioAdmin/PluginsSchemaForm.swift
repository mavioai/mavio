#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    // A form drawn from a JSON Schema, as plugins describe their configuration
    // (docs/clients.md §6.3.3), as the web UI's: objects as groups of rows,
    // arrays of objects as cards to add and remove, lists of strings as lines,
    // enumerations as choices, values of a source the schema names
    // (x-mavio-options) as a searchable choice whose option gives its
    // settings' schema, and what has no shape the form knows, such as an
    // object of arbitrary keys, as JSON. Properties the server generates are
    // left out; a switch needing a container dependency
    // (x-mavio-requires-container) stays off until the dependency is set up.

    /// An option of a source a schema names.
    typealias PluginsOption = Mavio_System_V1_PluginConfigOption

    /// What the form's fields share: the options of the sources the schema
    /// names, nil while they load or when they cannot be had, and whether a
    /// container dependency is not set up, by its name.
    private struct FormContext {
        let options: (String) -> [PluginsOption]?
        let unset: (String) -> Bool
    }

    /// The form of a configuration object, its properties the rows of the
    /// card around it.
    struct PluginsSchemaForm: View {
        let schema: PluginsSchema
        let value: PluginsJSON?
        let options: [String: [PluginsOption]]
        /// Whether a container dependency is not set up, which the switches needing it wait for.
        let unset: (String) -> Bool
        let onChange: (PluginsJSON?) -> Void

        var body: some View {
            ObjectFields(
                schema: schema, value: value?.object ?? PluginsJSONObject(),
                context: FormContext(options: { options[$0] }, unset: unset), nested: false
            ) { onChange(.object($0)) }
        }
    }

    /// An object's properties, as the rows of a card, or, nested, as rows
    /// with lines between them.
    private struct ObjectFields: View {
        let schema: PluginsSchema
        let value: PluginsJSONObject
        let context: FormContext
        let nested: Bool
        let onChange: (PluginsJSONObject) -> Void

        private var props: [(name: String, schema: PluginsSchema)] {
            (schema.properties ?? []).filter { !$0.schema.generated }
        }

        var body: some View {
            if props.isEmpty {
                JsonField(value: .object(value)) { onChange($0?.object ?? PluginsJSONObject()) }
                    .modifier(NestedRow(nested: nested))
            } else if nested {
                VStack(alignment: .leading, spacing: 0) {
                    let shown = props.filter { visible($0.name) }
                    ForEach(Array(shown.enumerated()), id: \.element.name) { i, p in
                        if i > 0 { Divider().padding(.leading, MavioSpacing.s16) }
                        row(p.name, p.schema).modifier(NestedRow(nested: true))
                    }
                }
            } else {
                ForEach(props.filter { visible($0.name) }, id: \.name) { p in
                    row(p.name, p.schema)
                }
            }
        }

        /// The properties whose schema a chosen option gives, nil when it takes no settings.
        private var given: [String: PluginsSchema?] {
            var out: [String: PluginsSchema?] = [:]
            for (k, p) in props {
                guard let spec = p.options, let settings = spec.settings,
                    let chosen = context.options(spec.source)?.first(where: { .string($0.value) == value[k] })
                else { continue }
                out.updateValue(
                    chosen.settingsSchema.isEmpty ? nil : PluginsSchema(parsing: chosen.settingsSchema),
                    forKey: settings)
            }
            return out
        }

        /// Whether a property shows: one an option gives shows only when the
        /// option takes settings.
        private func visible(_ k: String) -> Bool {
            guard let s = given[k] else { return true }
            return s?.hasProperties ?? false
        }

        private func set(_ k: String, _ v: PluginsJSON?) {
            var next = value
            next[k] = v
            onChange(next)
        }

        /// Choosing another option leaves the settings of the one before, and
        /// names the object after it while it bears that one's name or none.
        private func choose(_ k: String, _ spec: PluginsSchema.Options, _ list: [PluginsOption], _ v: String?) {
            let before = list.first { .string($0.value) == value[k] }
            let after = list.first { $0.value == v }
            var next = value
            next[k] = v.map(PluginsJSON.string)
            if let settings = spec.settings, before?.value != after?.value { next[settings] = nil }
            if let name = spec.name, let after {
                let current = next[name]?.string ?? ""
                if current.isEmpty || current == before?.name { next[name] = .string(after.name) }
            }
            onChange(next)
        }

        @ViewBuilder private func row(_ k: String, _ p: PluginsSchema) -> some View {
            let required = schema.required.contains(k)
            if let entry = given[k] {
                if let s = entry {
                    Property(
                        name: k, schema: PluginsSchema(s, title: p.title, description: p.description), required: true,
                        value: value[k], context: context
                    ) { set(k, $0) }
                }
            } else if let spec = p.options, let list = context.options(spec.source) {
                Property(
                    name: k, schema: p, required: required, value: value[k], context: context,
                    choices: list, onChoose: { choose(k, spec, list, $0) }
                ) { set(k, $0) }
            } else {
                let dep = p.requiresContainer.flatMap { value[$0]?.string } ?? ""
                Property(
                    name: k, schema: p, required: required, value: value[k], context: context,
                    waiting: !dep.isEmpty && context.unset(dep) ? dep : nil
                ) { set(k, $0) }
            }
        }
    }

    /// A row inside a nested group, padded as a card's rows are.
    private struct NestedRow: ViewModifier {
        let nested: Bool

        func body(content: Content) -> some View {
            if nested {
                content
                    .frame(maxWidth: .infinity, minHeight: 32, alignment: .leading)
                    .padding(.horizontal, MavioSpacing.s16)
                    .padding(.vertical, 10)
            } else {
                content
            }
        }
    }

    /// A nested group's border.
    private struct Boxed: ViewModifier {
        @Environment(\.theme) private var theme

        func body(content: Content) -> some View {
            content.overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(theme.colors.border))
        }
    }

    /// A property: its label and description, and its control beside or below them.
    private struct Property: View {
        let name: String
        let schema: PluginsSchema
        let required: Bool
        let value: PluginsJSON?
        let context: FormContext
        /// The options of the source the property's schema names, chosen by onChoose.
        var choices: [PluginsOption]?
        var onChoose: ((String?) -> Void)?
        /// The container dependency not set up that a switch waits for, which keeps it off.
        var waiting: String?
        let onChange: (PluginsJSON?) -> Void
        @Environment(\.theme) private var theme

        private var label: String { schema.label(name) }

        var body: some View {
            let s = schema
            if let list = choices, let onChoose {
                head {
                    let source = s.options?.source
                    PluginsSearchPicker(
                        options: source == "proxies" ? proxyOptions(list) : pickerOptions(list),
                        filters: source == "proxies" ? [] : pickerFilters(list),
                        value: value?.string ?? s.defaultValue?.string,
                        placeholder: MavioStrings.schemaChoose, onChange: onChoose
                    )
                    .frame(width: 320)
                }
            } else if s.readOnly && s.kind == nil {
                // Text shown to administrators, which takes no value.
                head(description: s.description) { EmptyView() }
            } else {
                control(s)
            }
        }

        @ViewBuilder private func control(_ s: PluginsSchema) -> some View {
            switch s.kind {
            case "boolean":
                if let waiting {
                    head(description: MavioStrings.schemaNeedsContainer(name: waiting)) {
                        Toggle(label, isOn: .constant(false)).labelsHidden().toggleStyle(.switch).disabled(true)
                    }
                } else {
                    head {
                        Toggle(
                            label,
                            isOn: Binding(
                                get: { value?.bool ?? (s.defaultValue == .bool(true)) }, set: { onChange(.bool($0)) })
                        )
                        .labelsHidden().toggleStyle(.switch)
                    }
                }
            case "enum", "string":
                if s.choices != nil {
                    head { EnumField(label: label, schema: s, required: required, value: value, onChange: onChange) }
                } else {
                    head {
                        TextInput(
                            label: label, secret: s.secret(name), text: value?.string ?? "",
                            placeholder: s.defaultValue?.string
                        ) { onChange($0.isEmpty && !required ? nil : .string($0)) }
                        .frame(width: 320)
                    }
                }
            case "integer", "number":
                head {
                    NumberInput(
                        label: label, value: value?.number, minimum: s.minimum, maximum: s.maximum,
                        integer: s.kind == "integer", placeholder: s.defaultValue.map(\.display)
                    ) { onChange($0.map(PluginsJSON.number)) }
                    .frame(width: 160)
                }
            case "object":
                if s.properties == nil {
                    below { JsonField(value: value ?? .object(PluginsJSONObject()), onChange: onChange) }
                } else {
                    // An optional object is set or left out as a whole.
                    let set = value?.object
                    VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                        head {
                            if !required {
                                Toggle(
                                    label, isOn: Binding(get: { set != nil }, set: { onChange($0 ? s.initial : nil) })
                                )
                                .labelsHidden().toggleStyle(.switch)
                            }
                        }
                        if set != nil || required {
                            ObjectFields(
                                schema: s, value: set ?? PluginsJSONObject(), context: context, nested: true
                            ) { onChange(.object($0)) }
                            .modifier(Boxed())
                        }
                    }
                }
            case "array":
                array(s)
            default:
                below { JsonField(value: value, onChange: onChange) }
            }
        }

        @ViewBuilder private func array(_ s: PluginsSchema) -> some View {
            let items = s.items ?? PluginsSchema(PluginsJSONObject())
            let list = value?.array ?? []
            let itemKind = items.kind
            let lists = { (v: [PluginsJSON]) in onChange(v.isEmpty ? nil : .array(v)) }
            if itemKind == "object" && items.properties != nil {
                below { ObjectList(schema: items, value: list, context: context, onChange: onChange) }
            } else if (items.choices?.count ?? 0) > tagsAbove {
                below {
                    TagSet(
                        schema: items, label: label, value: list.isEmpty ? (s.defaultValue?.array ?? []) : list,
                        onChange: lists)
                }
            } else if items.choices != nil {
                below { EnumSet(schema: items, value: list, onChange: lists) }
            } else if itemKind == "string" || itemKind == "integer" || itemKind == "number" {
                below { Lines(label: label, numeric: itemKind != "string", value: list, onChange: lists) }
            } else {
                below { JsonField(value: value ?? .array([]), onChange: onChange) }
            }
        }

        /// The label, with a red star when the property is required, and the
        /// description under it, then the control at the row's end.
        private func head(description: String? = nil, @ViewBuilder control: () -> some View) -> some View {
            HStack(spacing: MavioSpacing.s16) {
                VStack(alignment: .leading, spacing: 2) {
                    (Text(label) + Text(required ? " *" : "").foregroundStyle(theme.colors.danger))
                        .type(theme.type.body)
                    if let d = description ?? schema.description {
                        Text(d).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                control()
            }
        }

        /// The label and description, and the control below them.
        private func below(@ViewBuilder control: () -> some View) -> some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                head { EmptyView() }
                control()
            }
        }
    }

    /// How many options a multiple choice shows as checkboxes; more are tags.
    private let tagsAbove = 8

    // MARK: Options

    /// The tags of options a picker filters by, in groups, with their words and tones.
    @MainActor private let tagGroups: [[(key: String, label: String, tone: PluginsTone)]] = [
        [("torrent", MavioStrings.schemaTagTorrent, .neutral), ("usenet", MavioStrings.schemaTagUsenet, .neutral)],
        [
            ("public", MavioStrings.schemaTagPublic, .success),
            ("semi-private", MavioStrings.schemaTagSemiPrivate, .warning),
            ("private", MavioStrings.schemaTagPrivate, .danger),
        ],
    ]

    @MainActor private func knownTag(_ key: String) -> (key: String, label: String, tone: PluginsTone)? {
        tagGroups.joined().first { $0.key == key }
    }

    /// A language's name without its region, the tag itself when it has none.
    private func languageName(_ tag: String) -> String {
        let code = Locale.Language(identifier: tag).languageCode?.identifier ?? tag
        return Locale.current.localizedString(forLanguageCode: code) ?? tag
    }

    /// Options as a picker lists them, by name: protocol and privacy as
    /// badges, the language beside them.
    @MainActor private func pickerOptions(_ list: [PluginsOption]) -> [PluginsPickerItem] {
        list.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }.map { o in
            let meta = o.tags.filter { knownTag($0) == nil }.map(languageName).joined(separator: " · ")
            return PluginsPickerItem(
                value: o.value, label: o.name, detail: o.description_p, meta: meta.isEmpty ? nil : meta,
                badges: o.tags.compactMap { knownTag($0).map { ($0.label, $0.tone) } }, keys: o.tags)
        }
    }

    /// The system proxy's ID.
    private let systemProxy = "system"

    /// The server's proxies as a picker lists them: none first, then the
    /// system proxy, then the others by name, each with its address.
    @MainActor private func proxyOptions(_ list: [PluginsOption]) -> [PluginsPickerItem] {
        var out = [
            PluginsPickerItem(value: "", label: MavioStrings.proxiesNone, detail: MavioStrings.proxiesNoneDetail)
        ]
        if let system = list.first(where: { $0.value == systemProxy }) {
            out.append(
                PluginsPickerItem(
                    value: systemProxy, label: MavioStrings.proxiesSystem,
                    detail: system.description_p.isEmpty ? MavioStrings.proxiesSystemUnset : system.description_p))
        }
        let others = list.filter { $0.value != systemProxy }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        for o in others {
            out.append(PluginsPickerItem(value: o.value, label: o.name, detail: o.description_p))
        }
        return out
    }

    /// The groups of filters a list of options can be told apart by, those
    /// of two tags or more.
    @MainActor private func pickerFilters(_ list: [PluginsOption]) -> [[(key: String, label: String)]] {
        let present = Set(list.flatMap(\.tags))
        return tagGroups.map { group in group.filter { present.contains($0.key) }.map { ($0.key, $0.label) } }
            .filter { $0.count > 1 }
    }

    // MARK: Fields

    /// A text input, revealed by a button when it holds a secret.
    private struct TextInput: View {
        let label: String
        let secret: Bool
        let text: String
        let placeholder: String?
        let onChange: (String) -> Void
        @State private var shown = false
        @Environment(\.theme) private var theme

        var body: some View {
            let binding = Binding(get: { text }, set: { onChange($0) })
            HStack(spacing: MavioSpacing.s4) {
                Group {
                    if secret && !shown {
                        SecureField(label, text: binding, prompt: placeholder.map { Text($0) })
                    } else {
                        TextField(label, text: binding, prompt: placeholder.map { Text($0) })
                            .autocorrectionDisabled()
                    }
                }
                .fieldLabelHidden()
                .textFieldStyle(.roundedBorder)
                if secret {
                    Button {
                        shown.toggle()
                    } label: {
                        Icon(shown ? "lock" : "key", size: 16).foregroundStyle(theme.colors.textSecondary)
                    }
                    .buttonStyle(.borderless)
                    .help(shown ? MavioStrings.schemaHide : MavioStrings.schemaShow)
                    .accessibilityLabel(shown ? MavioStrings.schemaHide : MavioStrings.schemaShow)
                }
            }
        }
    }

    /// A number's input, red while it is outside its range.
    private struct NumberInput: View {
        let label: String
        let value: Double?
        let minimum: Double?
        let maximum: Double?
        let integer: Bool
        let placeholder: String?
        let onChange: (Double?) -> Void
        @State private var text: String
        @Environment(\.theme) private var theme

        init(
            label: String, value: Double?, minimum: Double?, maximum: Double?, integer: Bool, placeholder: String?,
            onChange: @escaping (Double?) -> Void
        ) {
            (self.label, self.value, self.minimum, self.maximum) = (label, value, minimum, maximum)
            (self.integer, self.placeholder, self.onChange) = (integer, placeholder, onChange)
            _text = State(initialValue: value.map { PluginsJSON.number($0).display } ?? "")
        }

        private func parse(_ t: String) -> Double? {
            let trimmed = t.trimmingCharacters(in: .whitespaces)
            guard let n = Double(trimmed) else { return nil }
            return integer ? n.rounded(.towardZero) : n
        }

        private var outside: Bool {
            guard let n = parse(text) else { return !text.trimmingCharacters(in: .whitespaces).isEmpty }
            return (minimum.map { n < $0 } ?? false) || (maximum.map { n > $0 } ?? false)
        }

        var body: some View {
            TextField(label, text: $text, prompt: placeholder.map { Text($0) })
                .fieldLabelHidden()
                .textFieldStyle(.roundedBorder)
                .multilineTextAlignment(.trailing)
                .foregroundStyle(outside ? theme.colors.danger : theme.colors.textPrimary)
                .onChange(of: text) { onChange(parse(text)) }
                .onChange(of: value) {
                    if parse(text) != value { text = value.map { PluginsJSON.number($0).display } ?? "" }
                }
        }
    }

    /// A choice of an enumeration, with the plugin's default while unset.
    private struct EnumField: View {
        let label: String
        let schema: PluginsSchema
        let required: Bool
        let value: PluginsJSON?
        let onChange: (PluginsJSON?) -> Void

        var body: some View {
            let options = schema.choices ?? []
            let index = options.firstIndex { $0.value == value } ?? -1
            let fallback = schema.defaultValue.map { d in options.first { $0.value == d }?.label ?? d.display }
            Picker(
                label,
                selection: Binding(
                    get: { index }, set: { onChange($0 < 0 ? nil : options[$0].value) })
            ) {
                if !required || index < 0 {
                    Text(fallback.map { MavioStrings.schemaDefault(value: $0) } ?? "—").tag(-1)
                }
                ForEach(options.indices, id: \.self) { Text(options[$0].label).tag($0) }
            }
            .labelsHidden()
            .pickerStyle(.menu)
            .fixedSize()
        }
    }

    /// Values chosen among an enumeration, as checkboxes.
    private struct EnumSet: View {
        let schema: PluginsSchema
        let value: [PluginsJSON]
        let onChange: ([PluginsJSON]) -> Void

        var body: some View {
            PluginsFlow(spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s8) {
                ForEach(Array((schema.choices ?? []).enumerated()), id: \.offset) { _, o in
                    Toggle(
                        o.label,
                        isOn: Binding(
                            get: { value.contains(o.value) },
                            set: { on in onChange(on ? value + [o.value] : value.filter { $0 != o.value }) })
                    )
                    .toggleStyle(.checkbox)
                }
            }
        }
    }

    /// Values chosen among many, in their order, as tags: each removed by its
    /// button, but the last, and the others added from a picker. Unset, it
    /// shows the default the plugin takes.
    private struct TagSet: View {
        let schema: PluginsSchema
        let label: String
        let value: [PluginsJSON]
        let onChange: ([PluginsJSON]) -> Void
        @Environment(\.theme) private var theme

        var body: some View {
            let options = schema.choices ?? []
            let title = { (v: PluginsJSON) in options.first { $0.value == v }?.label ?? v.display }
            let rest = options.filter { o in !value.contains(o.value) }
            PluginsFlow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                ForEach(Array(value.enumerated()), id: \.offset) { _, v in
                    HStack(spacing: MavioSpacing.s4) {
                        Text(title(v)).type(theme.type.footnote)
                        if value.count > 1 {
                            Button {
                                onChange(value.filter { $0 != v })
                            } label: {
                                Icon("close", size: 12).foregroundStyle(theme.colors.textSecondary)
                                    .frame(width: 20, height: 20)
                                    .contentShape(.circle)
                            }
                            .buttonStyle(.borderless)
                            .accessibilityLabel("\(MavioStrings.schemaRemove) \(title(v))")
                        }
                    }
                    .padding(.leading, 10)
                    .padding(.trailing, value.count > 1 ? 4 : 10)
                    .frame(minHeight: 28)
                    .background(theme.colors.fill, in: .capsule)
                }
                PluginsSearchPicker(
                    options: rest.map { PluginsPickerItem(value: $0.value.text, label: $0.label) },
                    filters: [], value: nil, placeholder: MavioStrings.schemaAdd
                ) { k in
                    if let o = rest.first(where: { $0.value.text == k }) { onChange(value + [o.value]) }
                }
                .fixedSize()
            }
        }
    }

    /// A list of strings or numbers, one per line.
    private struct Lines: View {
        let label: String
        let numeric: Bool
        let value: [PluginsJSON]
        let onChange: ([PluginsJSON]) -> Void
        @State private var text: String

        init(label: String, numeric: Bool, value: [PluginsJSON], onChange: @escaping ([PluginsJSON]) -> Void) {
            (self.label, self.numeric, self.value, self.onChange) = (label, numeric, value, onChange)
            _text = State(initialValue: value.map(\.display).joined(separator: "\n"))
        }

        var body: some View {
            PluginsTextArea(
                text: $text, placeholder: MavioStrings.schemaOnePerLine,
                lines: min(8, max(2, value.count + 1))
            )
            .accessibilityLabel(label)
            .onChange(of: text) {
                let lines = text.split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }
                    .filter { !$0.isEmpty }
                onChange(
                    numeric ? lines.compactMap { Double($0).map(PluginsJSON.number) } : lines.map(PluginsJSON.string))
            }
        }
    }

    /// A value of no shape the form knows, edited as JSON.
    private struct JsonField: View {
        let value: PluginsJSON?
        let onChange: (PluginsJSON?) -> Void
        @State private var text: String
        @State private var bad = false
        @Environment(\.theme) private var theme

        init(value: PluginsJSON?, onChange: @escaping (PluginsJSON?) -> Void) {
            (self.value, self.onChange) = (value, onChange)
            _text = State(initialValue: value?.pretty ?? "")
        }

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                PluginsTextArea(
                    text: $text,
                    lines: min(14, max(3, text.split(separator: "\n", omittingEmptySubsequences: false).count)),
                    footnote: true, bad: bad)
                if bad {
                    Text(MavioStrings.schemaInvalidJson).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                }
            }
            .onChange(of: text) {
                if text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    bad = false
                    onChange(nil)
                } else if let v = PluginsJSON(parsing: text) {
                    bad = false
                    onChange(v)
                } else {
                    bad = true
                }
            }
        }
    }

    /// The title of an object in a list: its name or ID, else its place.
    private func itemTitle(_ v: PluginsJSON, _ i: Int) -> String {
        if let o = v.object {
            for k in ["name", "id", "definition", "kind", "host"] {
                if let s = o[k]?.string, !s.isEmpty { return s }
            }
        }
        return MavioStrings.schemaItem(number: String(i + 1))
    }

    /// Objects of a list, as cards that open to their fields.
    private struct ObjectList: View {
        let schema: PluginsSchema
        let value: [PluginsJSON]
        let context: FormContext
        let onChange: (PluginsJSON?) -> Void
        @State private var open: Int?
        @Environment(\.theme) private var theme

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                // Items have no identity of their own: their place and the
                // list's length key them, so that removing one draws the rest anew.
                ForEach(Array(value.enumerated()), id: \.offset) { i, v in
                    item(i, v).id("\(i)/\(value.count)")
                }
                Button {
                    open = value.count
                    onChange(.array(value + [schema.initial ?? .null]))
                } label: {
                    Label {
                        Text(MavioStrings.schemaAdd)
                    } icon: {
                        Icon("add", size: 18)
                    }
                }
                .buttonStyle(.bordered)
            }
        }

        private func item(_ i: Int, _ v: PluginsJSON) -> some View {
            VStack(alignment: .leading, spacing: 0) {
                HStack(spacing: MavioSpacing.s8) {
                    Button {
                        open = open == i ? nil : i
                    } label: {
                        HStack(spacing: MavioSpacing.s8) {
                            Icon("expand", size: 16).rotationEffect(.degrees(open == i ? 0 : -90))
                            Text(itemTitle(v, i)).type(theme.type.headline).lineLimit(1)
                            Spacer(minLength: 0)
                        }
                        .contentShape(.rect)
                        .frame(minHeight: 36)
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(open == i ? .isSelected : [])
                    Button(MavioStrings.schemaRemove, role: .destructive) {
                        let next = value.enumerated().filter { $0.offset != i }.map(\.element)
                        open = nil
                        onChange(next.isEmpty ? nil : .array(next))
                    }
                    .buttonStyle(.borderless)
                    .foregroundStyle(theme.colors.danger)
                }
                .padding(.horizontal, MavioSpacing.s12)
                .padding(.vertical, 6)
                if open == i {
                    Divider()
                    ObjectFields(schema: schema, value: v.object ?? PluginsJSONObject(), context: context, nested: true)
                    {
                        o in
                        onChange(.array(value.enumerated().map { $0.offset == i ? .object(o) : $0.element }))
                    }
                }
            }
            .modifier(Boxed())
        }
    }

    // MARK: Controls

    /// A tone of a picker's badge.
    enum PluginsTone {
        case neutral, success, warning, danger
    }

    /// An option of a searchable picker.
    struct PluginsPickerItem: Identifiable {
        let value: String
        let label: String
        var detail: String = ""
        /// Quiet text beside the label, such as a language.
        var meta: String?
        var badges: [(label: String, tone: PluginsTone)] = []
        /// The keys the filters match.
        var keys: [String] = []

        var id: String { value }

        /// Whether the option holds what was typed, in its label, detail,
        /// meta or badges.
        func matches(_ query: String) -> Bool {
            let q = query.trimmingCharacters(in: .whitespaces)
            guard !q.isEmpty else { return true }
            return ([label, detail, meta ?? ""] + badges.map(\.label)).contains {
                $0.localizedCaseInsensitiveContains(q)
            }
        }
    }

    /// A choice among many options, as a button showing the option chosen
    /// or the placeholder, whose popover lists the options holding what is
    /// typed into its field, one row each, under filters chosen one per
    /// group and the count of the options matching.
    struct PluginsSearchPicker: View {
        let options: [PluginsPickerItem]
        let filters: [[(key: String, label: String)]]
        let value: String?
        let placeholder: String
        let onChange: (String?) -> Void
        @State private var open = false
        @State private var query = ""
        @State private var chosen: [Int: String] = [:]
        @Environment(\.theme) private var theme

        /// How many options the popover lists at once; typing finds the others.
        private static let limit = 100

        var body: some View {
            let selected = options.first { $0.value == value }
            Button {
                open = true
            } label: {
                HStack(spacing: MavioSpacing.s8) {
                    Text(selected?.label ?? placeholder)
                        .foregroundStyle(selected == nil ? theme.colors.textTertiary : theme.colors.textPrimary)
                        .lineLimit(1)
                    Spacer(minLength: 0)
                    Icon("expand", size: 14).foregroundStyle(theme.colors.textSecondary)
                }
                .padding(.horizontal, 10)
                .frame(minHeight: 28)
                .background(theme.colors.surface, in: .rect(cornerRadius: 6))
                .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(theme.colors.border))
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .popover(isPresented: $open, arrowEdge: .bottom) { popover }
            .onChange(of: open) { if !open { query = "" } }
        }

        private var popover: some View {
            let items = options.filter { o in chosen.values.allSatisfy { o.keys.contains($0) } }
            let matching = items.filter { $0.matches(query) }
            return VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                TextField(MavioStrings.navSearch, text: $query)
                    .textFieldStyle(.roundedBorder)
                if !filters.isEmpty || matching.count > Self.limit {
                    HStack(spacing: MavioSpacing.s12) {
                        ForEach(filters.indices, id: \.self) { g in
                            HStack(spacing: 2) {
                                chip(MavioStrings.schemaAll, on: chosen[g] == nil) { chosen[g] = nil }
                                ForEach(filters[g], id: \.key) { f in
                                    chip(f.label, on: chosen[g] == f.key) { chosen[g] = f.key }
                                }
                            }
                        }
                        Spacer(minLength: 0)
                        Text(
                            matching.count > Self.limit
                                ? MavioStrings.schemaMatchesFirst(
                                    shown: String(Self.limit), count: String(matching.count))
                                : MavioStrings.schemaMatches(count: matching.count)
                        )
                        .type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                if matching.isEmpty {
                    Text(MavioStrings.schemaNoMatches).foregroundStyle(theme.colors.textSecondary)
                        .padding(.vertical, MavioSpacing.s8)
                } else {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(matching.prefix(Self.limit)) { o in row(o) }
                        }
                    }
                    .frame(maxHeight: 360)
                }
            }
            .padding(MavioSpacing.s12)
            .frame(width: 420)
        }

        private func chip(_ title: String, on: Bool, action: @escaping () -> Void) -> some View {
            Button(action: action) {
                Text(title).type(theme.type.caption).fontWeight(.medium)
                    .foregroundStyle(on ? theme.colors.textPrimary : theme.colors.textSecondary)
                    .padding(.horizontal, MavioSpacing.s8)
                    .frame(minHeight: 24)
                    .background(on ? theme.colors.fillSelected : .clear, in: .capsule)
                    .contentShape(.capsule)
            }
            .buttonStyle(.plain)
        }

        private func row(_ o: PluginsPickerItem) -> some View {
            Button {
                onChange(o.value)
                open = false
            } label: {
                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: MavioSpacing.s8) {
                        Text(o.label).type(theme.type.callout).lineLimit(1)
                        ForEach(Array(o.badges.enumerated()), id: \.offset) { _, b in
                            Text(b.label).type(theme.type.caption).fontWeight(.medium)
                                .foregroundStyle(tone(b.tone))
                                .padding(.horizontal, 6)
                                .background(tone(b.tone).opacity(0.15), in: .capsule)
                        }
                        if let meta = o.meta {
                            Text(meta).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                                .lineLimit(1)
                        }
                        Spacer(minLength: 0)
                        if o.value == value { Icon("check", size: 14).foregroundStyle(theme.colors.accent) }
                    }
                    if !o.detail.isEmpty {
                        Text(o.detail).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(1)
                    }
                }
                .padding(.horizontal, MavioSpacing.s8)
                .padding(.vertical, 6)
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
        }

        private func tone(_ t: PluginsTone) -> Color {
            switch t {
            case .neutral: theme.colors.textSecondary
            case .success: Color(hex: 0x34C759)
            case .warning: Color(hex: 0xFF9500)
            case .danger: theme.colors.danger
            }
        }
    }

    /// A text area of a number of lines, monospaced, with a placeholder.
    struct PluginsTextArea: View {
        @Binding var text: String
        var placeholder: String?
        let lines: Int
        var footnote = false
        var bad = false
        @Environment(\.theme) private var theme

        var body: some View {
            let style = footnote ? theme.type.footnote : theme.type.body
            TextEditor(text: $text)
                .font(style.font.monospaced())
                .scrollContentBackground(.hidden)
                .autocorrectionDisabled()
                .padding(.horizontal, MavioSpacing.s4)
                .padding(.vertical, 6)
                .frame(height: CGFloat(lines) * (footnote ? 17 : 20) + 14)
                .background(theme.colors.surface, in: .rect(cornerRadius: 6))
                .overlay(
                    RoundedRectangle(cornerRadius: 6).strokeBorder(bad ? theme.colors.danger : theme.colors.border)
                )
                .overlay(alignment: .topLeading) {
                    if text.isEmpty, let placeholder {
                        Text(placeholder).font(style.font.monospaced()).foregroundStyle(theme.colors.textTertiary)
                            .padding(.horizontal, 9)
                            .padding(.vertical, 6)
                            .allowsHitTesting(false)
                    }
                }
        }
    }

    /// Views in lines, as many on a line as fit, wrapping.
    struct PluginsFlow: Layout {
        var spacing: CGFloat
        var lineSpacing: CGFloat

        func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
            let rows = arrange(width: proposal.width ?? .infinity, subviews)
            let width = rows.map { $0.width }.max() ?? 0
            let height = rows.map(\.height).reduce(0, +) + lineSpacing * CGFloat(max(0, rows.count - 1))
            return CGSize(width: proposal.width ?? width, height: height)
        }

        func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
            var y = bounds.minY
            for row in arrange(width: bounds.width, subviews) {
                var x = bounds.minX
                for i in row.indices {
                    let size = subviews[i].sizeThatFits(.unspecified)
                    subviews[i].place(
                        at: CGPoint(x: x, y: y + (row.height - size.height) / 2), proposal: ProposedViewSize(size))
                    x += size.width + spacing
                }
                y += row.height + lineSpacing
            }
        }

        private func arrange(width: CGFloat, _ subviews: Subviews) -> [(
            indices: [Int], width: CGFloat, height: CGFloat
        )] {
            var rows: [(indices: [Int], width: CGFloat, height: CGFloat)] = []
            var current: (indices: [Int], width: CGFloat, height: CGFloat) = ([], 0, 0)
            for i in subviews.indices {
                let size = subviews[i].sizeThatFits(.unspecified)
                let next = current.indices.isEmpty ? size.width : current.width + spacing + size.width
                if next > width && !current.indices.isEmpty {
                    rows.append(current)
                    current = ([i], size.width, size.height)
                } else {
                    current = (current.indices + [i], next, max(current.height, size.height))
                }
            }
            if !current.indices.isEmpty { rows.append(current) }
            return rows
        }
    }
#endif

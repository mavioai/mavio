#if os(macOS)
    import AppKit
    import MavioMac

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
    public typealias PluginsOption = Mavio_System_V1_PluginConfigOption

    /// A step of the way from the form's value to a value within it.
    private enum Step: Hashable, CustomStringConvertible {
        case key(String)
        case index(Int)

        var description: String {
            switch self {
            case .key(let k): "." + k
            case .index(let i): "[\(i)]"
            }
        }
    }

    /// The form of a plugin's configuration object, drawn from its schema:
    /// the object's properties as rows with lines between them, to place as
    /// the one row of a form's card (FormSection). It edits a copy of the
    /// value it is given and tells each change; typing changes nothing else,
    /// while choosing an option, adding or removing an item and turning an
    /// optional object on or off draw the form anew.
    public final class PluginsSchemaForm: NSView {
        private let schema: PluginsSchema
        private let theme: Theme
        private let unset: @MainActor (String) -> Bool
        private let onChange: @MainActor (PluginsJSON?) -> Void
        private var root: PluginsJSON?
        private let stack: NSStackView
        /// The item each list of objects shows open, by the list's path.
        private var open: [String: Int] = [:]
        /// The paths of the secrets shown.
        private var revealed: Set<String> = []
        /// What redraws text that follows the value, as the titles of items.
        private var refreshers: [@MainActor () -> Void] = []

        /// The options of the sources the schema names, by source; a source
        /// missing is one whose options load or cannot be had, whose
        /// properties are edited as their kind says until they come.
        /// Setting them draws the form anew.
        public var options: [String: [PluginsOption]] {
            didSet { render() }
        }

        /// The form of schema editing value, with the options of the sources
        /// the schema names, in theme. unset tells whether a container
        /// dependency, by its name, is not set up, which the switches needing
        /// it wait for; onChange is called with the whole value after each
        /// change, always an object.
        public init(
            schema: PluginsSchema, value: PluginsJSON?, options: [String: [PluginsOption]] = [:], theme: Theme,
            unset: @escaping @MainActor (String) -> Bool = { _ in false },
            onChange: @escaping @MainActor (PluginsJSON?) -> Void
        ) {
            self.schema = schema
            self.theme = theme
            self.unset = unset
            self.onChange = onChange
            self.options = options
            root = value
            stack = .vertical([], spacing: theme.isTV ? MavioSpacing.s8 : 0)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(stack)
            render()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// The value the form edits; setting it draws the form anew.
        public var value: PluginsJSON? {
            get { root }
            set {
                root = newValue
                render()
            }
        }

        /// Draws the form anew, as when a container dependency was set up.
        public func refresh() { render() }

        // MARK: Drawing

        private func render() {
            refreshers = []
            let rows = objectRows(schema, path: [], nested: false)
            stack.setViews([])
            for (i, row) in rows.enumerated() {
                if i > 0 && !theme.isTV { add(divider(), to: stack) }
                add(ContentRow(row, theme: theme), to: stack)
            }
            FocusEngine.active(for: self)?.schedule()
        }

        private func add(_ view: NSView, to stack: NSStackView) {
            stack.addArrangedSubview(view)
            view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
        }

        private func divider() -> NSView {
            let line = Box(fill: theme.colors.border)
            line.size(height: 1)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(line, insets: NSEdgeInsets(top: 0, left: MavioSpacing.s16, bottom: 0, right: 0))
            return holder
        }

        // MARK: The value

        private func get(_ path: [Step]) -> PluginsJSON? {
            var v = root
            for step in path {
                switch step {
                case .key(let k): v = v?.object?[k]
                case .index(let i): v = v?.array.flatMap { $0.indices.contains(i) ? $0[i] : nil }
                }
            }
            return v
        }

        private static func put(_ v: PluginsJSON?, at path: ArraySlice<Step>, in base: PluginsJSON?) -> PluginsJSON? {
            guard let first = path.first else { return v }
            switch first {
            case .key(let k):
                var o = base?.object ?? PluginsJSONObject()
                o[k] = put(v, at: path.dropFirst(), in: o[k])
                return .object(o)
            case .index(let i):
                guard var a = base?.array, a.indices.contains(i) else { return base }
                a[i] = put(v, at: path.dropFirst(), in: a[i]) ?? .null
                return .array(a)
            }
        }

        /// Sets the value at path and tells the form's whole value.
        private func set(_ path: [Step], _ v: PluginsJSON?, redraw: Bool = false) {
            root = Self.put(v, at: path[...], in: root)
            if root == nil { root = .object(PluginsJSONObject()) }
            onChange(root)
            if redraw {
                render()
            } else {
                refreshers.forEach { $0() }
            }
        }

        private func key(_ path: [Step]) -> String { path.map(\.description).joined() }

        // MARK: Objects

        private func props(_ s: PluginsSchema) -> [(name: String, schema: PluginsSchema)] {
            (s.properties ?? []).filter { !$0.schema.generated }
        }

        /// The properties whose schema a chosen option gives, nil when it
        /// takes no settings.
        private func given(_ s: PluginsSchema, _ value: PluginsJSONObject) -> [String: PluginsSchema?] {
            var out: [String: PluginsSchema?] = [:]
            for (k, p) in props(s) {
                guard let spec = p.options, let settings = spec.settings,
                    let chosen = options[spec.source]?.first(where: { .string($0.value) == value[k] })
                else { continue }
                out.updateValue(
                    chosen.settingsSchema.isEmpty ? nil : PluginsSchema(parsing: chosen.settingsSchema),
                    forKey: settings)
            }
            return out
        }

        /// An object's properties as rows; an object of no properties the
        /// form knows is edited as JSON.
        private func objectRows(_ s: PluginsSchema, path: [Step], nested: Bool) -> [NSView] {
            let value = get(path)?.object ?? PluginsJSONObject()
            let list = props(s)
            if list.isEmpty {
                return [
                    jsonField(.object(value)) { [weak self] in
                        self?.set(path, .object($0?.object ?? PluginsJSONObject()))
                    }
                ]
            }
            let given = given(s, value)
            return list.compactMap { p in
                // A property an option gives shows only when the option takes settings.
                if let entry = given[p.name], !(entry?.hasProperties ?? false) { return nil }
                return row(p.name, p.schema, in: s, value: value, given: given, path: path)
            }
        }

        private func row(
            _ k: String, _ p: PluginsSchema, in s: PluginsSchema, value: PluginsJSONObject,
            given: [String: PluginsSchema?], path: [Step]
        ) -> NSView? {
            let required = s.required.contains(k)
            if let entry = given[k] {
                guard let schema = entry else { return nil }
                return property(
                    k, PluginsSchema(schema, title: p.title, description: p.description), required: true,
                    path: path + [.key(k)])
            } else if let spec = p.options, let list = options[spec.source] {
                return choice(k, p, required: required, spec: spec, list: list, value: value, path: path)
            } else {
                let dep = p.requiresContainer.flatMap { value[$0]?.string } ?? ""
                return property(
                    k, p, required: required, path: path + [.key(k)], waiting: !dep.isEmpty && unset(dep) ? dep : nil)
            }
        }

        /// A property whose value is an option of a source, as a searchable
        /// choice. Choosing another option leaves the settings of the one
        /// before, and names the object after it while it bears that one's
        /// name or none.
        private func choice(
            _ k: String, _ p: PluginsSchema, required: Bool, spec: PluginsSchema.Options, list: [PluginsOption],
            value: PluginsJSONObject, path: [Step]
        ) -> NSView {
            let proxies = spec.source == "proxies"
            let picker = SearchPicker(
                items: proxies ? proxyItems(list) : pickerItems(list), filters: proxies ? [] : pickerFilters(list),
                value: value[k]?.string ?? p.defaultValue?.string, placeholder: MavioStrings.schemaChoose, width: 320,
                limit: 100, bordered: true, theme: theme
            ) { [weak self] v in
                guard let self else { return }
                var next = self.get(path)?.object ?? PluginsJSONObject()
                let before = list.first { .string($0.value) == next[k] }
                let after = list.first { $0.value == v }
                next[k] = .string(v)
                if let settings = spec.settings, before?.value != after?.value { next[settings] = nil }
                if let name = spec.name, let after {
                    let current = next[name]?.string ?? ""
                    if current.isEmpty || current == before?.name { next[name] = .string(after.name) }
                }
                self.set(path, .object(next), redraw: true)
            }
            return head(p.label(k), required: required, description: p.description, control: picker)
        }

        // MARK: Properties

        private func property(
            _ name: String, _ s: PluginsSchema, required: Bool, path: [Step], waiting: String? = nil
        ) -> NSView {
            let label = s.label(name)
            if s.readOnly && s.kind == nil {
                // Text shown to administrators, which takes no value.
                return head(label, required: required, description: s.description, control: nil)
            }
            let value = get(path)
            switch s.kind {
            case "boolean":
                if let waiting {
                    let toggle = NSSwitch()
                    toggle.state = .off
                    toggle.isEnabled = false
                    return head(
                        label, required: required, description: MavioStrings.schemaNeedsContainer(name: waiting),
                        control: toggle)
                }
                let toggle = ActionSwitch(isOn: value?.bool ?? (s.defaultValue == .bool(true))) { [weak self] on in
                    self?.set(path, .bool(on))
                }
                toggle.setAccessibilityLabel(label)
                return head(label, required: required, description: s.description, control: toggle)
            case "enum", "string":
                if let choices = s.choices {
                    return head(
                        label, required: required, description: s.description,
                        control: enumField(label, s, choices: choices, required: required, value: value, path: path))
                }
                return head(
                    label, required: required, description: s.description,
                    control: textInput(
                        label, secret: s.secret(name), text: value?.string ?? "", placeholder: s.defaultValue?.string,
                        path: path, required: required))
            case "integer", "number":
                return head(
                    label, required: required, description: s.description,
                    control: numberInput(label, s, value: value?.number, path: path))
            case "object":
                guard s.properties != nil else {
                    return below(
                        label, required: required, description: s.description,
                        control: jsonField(value ?? .object(PluginsJSONObject())) { [weak self] in
                            self?.set(path, $0)
                        })
                }
                // An optional object is set or left out as a whole.
                let isSet = value?.object != nil
                var toggle: NSView?
                if !required {
                    let control = ActionSwitch(isOn: isSet) { [weak self] on in
                        self?.set(path, on ? s.initial : nil, redraw: true)
                    }
                    control.setAccessibilityLabel(label)
                    toggle = control
                }
                let top = head(label, required: required, description: s.description, control: toggle)
                guard isSet || required else { return top }
                let column = NSStackView.vertical([top, group(objectRows(s, path: path, nested: true))], spacing: 12)
                column.arrangedSubviews.forEach {
                    $0.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
                }
                return column
            case "array":
                return array(label, s, required: required, value: value, path: path)
            default:
                return below(
                    label, required: required, description: s.description,
                    control: jsonField(value) { [weak self] in self?.set(path, $0) })
            }
        }

        private func array(
            _ label: String, _ s: PluginsSchema, required: Bool, value: PluginsJSON?, path: [Step]
        ) -> NSView {
            let items = s.items ?? PluginsSchema(PluginsJSONObject())
            let list = value?.array ?? []
            let itemKind = items.kind
            let lists: @MainActor ([PluginsJSON], Bool) -> Void = { [weak self] v, redraw in
                self?.set(path, v.isEmpty ? nil : .array(v), redraw: redraw)
            }
            let control: NSView
            if itemKind == "object" && items.properties != nil {
                control = objectList(items, value: list, path: path)
            } else if let choices = items.choices, choices.count > tagsAbove {
                control = tagSet(
                    choices, label: label, value: list.isEmpty ? (s.defaultValue?.array ?? []) : list,
                    onChange: { lists($0, true) })
            } else if let choices = items.choices {
                control = enumSet(choices, value: list) { lists($0, false) }
            } else if itemKind == "string" || itemKind == "integer" || itemKind == "number" {
                control = linesField(label, numeric: itemKind != "string", value: list) { lists($0, false) }
            } else {
                control = jsonField(value ?? .array([])) { [weak self] in self?.set(path, $0) }
            }
            return below(label, required: required, description: s.description, control: control)
        }

        // MARK: Layouts

        /// The label, with a red star when the property is required, and the
        /// description under it, then the control at the row's end.
        private func head(_ label: String, required: Bool, description: String?, control: NSView?) -> NSView {
            let title = NSTextField.label(label, theme.fonts.body, color: theme.colors.textPrimary, lines: 0)
            if required {
                let text = NSMutableAttributedString(
                    string: label,
                    attributes: [.font: theme.fonts.body.font, .foregroundColor: theme.colors.textPrimary])
                text.append(
                    NSAttributedString(
                        string: " *", attributes: [.font: theme.fonts.body.font, .foregroundColor: theme.colors.danger])
                )
                title.attributedStringValue = text
            }
            title.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            var lines: [NSView] = [title]
            if let description, !description.isEmpty {
                lines.append(note(description, theme.fonts.footnote, theme.colors.textSecondary))
            }
            let text = NSStackView.vertical(lines, spacing: 2)
            text.setHuggingPriority(.init(1), for: .horizontal)
            text.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            var views: [NSView] = [text]
            if let control {
                let shown = focusable(control, theme: theme)
                shown.setContentCompressionResistancePriority(.defaultHigh, for: .horizontal)
                views.append(shown)
            }
            let row = NSStackView.horizontal(views, spacing: MavioSpacing.s16)
            row.heightAnchor.constraint(greaterThanOrEqualToConstant: 32).isActive = true
            return row
        }

        /// The label and description, and the control below them.
        private func below(_ label: String, required: Bool, description: String?, control: NSView) -> NSView {
            let top = head(label, required: required, description: description, control: nil)
            let column = NSStackView.vertical([top, control], spacing: MavioSpacing.s8)
            column.arrangedSubviews.forEach { $0.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            return column
        }

        /// A nested group: its rows padded as a card's, lines between them,
        /// in a border.
        private func group(_ rows: [NSView]) -> NSView {
            let box = Box(radius: 12)
            box.border = theme.colors.border
            let column = NSStackView.vertical([], spacing: 0)
            for (i, row) in rows.enumerated() {
                if i > 0 { add(divider(), to: column) }
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: 10))
                add(holder, to: column)
            }
            box.addFilling(column)
            return box
        }

        // MARK: Fields

        /// A text input, revealed by a button when it holds a secret.
        private func textInput(
            _ label: String, secret: Bool, text: String, placeholder: String?, path: [Step], required: Bool
        ) -> NSView {
            let id = key(path)
            let field = InputField(text: text, placeholder: placeholder, secure: secret && !revealed.contains(id))
            field.setAccessibilityLabel(label)
            field.onChange = { [weak self] t in self?.set(path, t.isEmpty && !required ? nil : .string(t)) }
            guard secret else {
                field.size(width: 320)
                return field
            }
            let shown = revealed.contains(id)
            let reveal = MavioButton(icon: shown ? "lock" : "key", kind: .plain, theme: theme)
            let describe = { [theme] (button: MavioButton, on: Bool) in
                let text = on ? MavioStrings.schemaHide : MavioStrings.schemaShow
                button.setAccessibilityLabel(text)
                if !theme.isTV { button.toolTip = text }
            }
            describe(reveal, shown)
            reveal.onPress = { [weak self, weak field, weak reveal] in
                guard let self, let field else { return }
                let on = !self.revealed.contains(id)
                if on { self.revealed.insert(id) } else { self.revealed.remove(id) }
                field.secure = !on
                if let reveal {
                    reveal.icon = on ? "lock" : "key"
                    describe(reveal, on)
                }
            }
            let row = NSStackView.horizontal([focusable(field, theme: theme), reveal], spacing: MavioSpacing.s4)
            field.widthAnchor.constraint(equalToConstant: 320 - 28 - MavioSpacing.s4).isActive = true
            return row
        }

        /// A number's input, red while it is outside its range.
        private func numberInput(_ label: String, _ s: PluginsSchema, value: Double?, path: [Step]) -> NSView {
            let field = InputField(
                text: value.map { PluginsJSON.number($0).display } ?? "", placeholder: s.defaultValue.map(\.display),
                width: 160)
            field.field.alignment = .right
            field.setAccessibilityLabel(label)
            let integer = s.kind == "integer"
            let parse = { (t: String) -> Double? in
                guard let n = Double(t.trimmingCharacters(in: .whitespaces)) else { return nil }
                return integer ? n.rounded(.towardZero) : n
            }
            let color = { [theme] (t: String) -> NSColor in
                guard let n = parse(t) else {
                    return t.trimmingCharacters(in: .whitespaces).isEmpty
                        ? theme.colors.textPrimary : theme.colors.danger
                }
                let outside = (s.minimum.map { n < $0 } ?? false) || (s.maximum.map { n > $0 } ?? false)
                return outside ? theme.colors.danger : theme.colors.textPrimary
            }
            field.field.textColor = color(field.text)
            field.onChange = { [weak self, weak field] t in
                field?.field.textColor = color(t)
                self?.set(path, parse(t).map(PluginsJSON.number))
            }
            return field
        }

        /// A choice of an enumeration, with the plugin's default while unset.
        private func enumField(
            _ label: String, _ s: PluginsSchema, choices: [(value: PluginsJSON, label: String)], required: Bool,
            value: PluginsJSON?, path: [Step]
        ) -> NSView {
            let index = choices.firstIndex { $0.value == value }
            let fallback = s.defaultValue.map { d in choices.first { $0.value == d }?.label ?? d.display }
            var titles: [String] = []
            var values: [PluginsJSON?] = []
            if !required || index == nil {
                titles.append(fallback.map { MavioStrings.schemaDefault(value: $0) } ?? "—")
                values.append(nil)
            }
            for c in choices {
                titles.append(c.label)
                values.append(c.value)
            }
            let selected = index.map { $0 + (values.first == nil ? 1 : 0) } ?? 0
            let popup = ActionPopUp(options: titles, selected: selected) { [weak self] i in
                self?.set(path, values[i], redraw: true)
            }
            popup.setAccessibilityLabel(label)
            return popup
        }

        /// Values chosen among an enumeration, as check boxes.
        private func enumSet(
            _ choices: [(value: PluginsJSON, label: String)], value: [PluginsJSON],
            onChange: @escaping @MainActor ([PluginsJSON]) -> Void
        ) -> NSView {
            var chosen = value
            let boxes = choices.map { c -> NSView in
                let box = ActionCheckBox(c.label, isOn: value.contains(c.value)) { on in
                    chosen = on ? chosen + [c.value] : chosen.filter { $0 != c.value }
                    onChange(chosen)
                }
                return focusable(box, theme: theme)
            }
            return WrapView(boxes, spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s8)
        }

        /// Values chosen among many, in their order, as tags: each removed by
        /// its button, but the last, and the others added from a picker.
        /// Unset, it shows the default the plugin takes.
        private func tagSet(
            _ choices: [(value: PluginsJSON, label: String)], label: String, value: [PluginsJSON],
            onChange: @escaping @MainActor ([PluginsJSON]) -> Void
        ) -> NSView {
            let title = { (v: PluginsJSON) in choices.first { $0.value == v }?.label ?? v.display }
            let rest = choices.filter { c in !value.contains(c.value) }
            var views: [NSView] = value.map { v in
                let text = NSTextField.label(title(v), theme.fonts.footnote, color: theme.colors.textPrimary)
                var parts: [NSView] = [text]
                if value.count > 1 {
                    let remove = MavioButton(icon: "close", kind: .plain, theme: theme) {
                        onChange(value.filter { $0 != v })
                    }
                    remove.setAccessibilityLabel("\(MavioStrings.schemaRemove) \(title(v))")
                    parts.append(remove)
                }
                let tag = Box(fill: theme.colors.fill, radius: 999)
                let row = NSStackView.horizontal(parts, spacing: MavioSpacing.s4)
                tag.addFilling(
                    row, insets: NSEdgeInsets(top: 2, left: 10, bottom: 2, right: value.count > 1 ? 4 : 10))
                tag.heightAnchor.constraint(greaterThanOrEqualToConstant: 28).isActive = true
                return tag
            }
            let picker = SearchPicker(
                items: rest.map { PickerItem(value: $0.value.text, label: $0.label) }, value: nil,
                placeholder: MavioStrings.schemaAdd, width: nil, bordered: true, theme: theme
            ) { k in
                if let c = rest.first(where: { $0.value.text == k }) { onChange(value + [c.value]) }
            }
            picker.setAccessibilityLabel("\(MavioStrings.schemaAdd) \(label)")
            views.append(picker)
            return WrapView(views, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8)
        }

        /// A list of strings or numbers, one per line.
        private func linesField(
            _ label: String, numeric: Bool, value: [PluginsJSON],
            onChange: @escaping @MainActor ([PluginsJSON]) -> Void
        ) -> NSView {
            let font = NSFont.monospacedSystemFont(ofSize: theme.fonts.body.size, weight: .regular)
            let area = TextArea(
                text: value.map(\.display).joined(separator: "\n"), placeholder: MavioStrings.schemaOnePerLine,
                font: font, lines: min(8, max(2, value.count + 1)), theme: theme)
            area.textView.setAccessibilityLabel(label)
            area.onChange = { text in
                let lines = text.split(separator: "\n").map { $0.trimmingCharacters(in: .whitespaces) }
                    .filter { !$0.isEmpty }
                onChange(
                    numeric ? lines.compactMap { Double($0).map(PluginsJSON.number) } : lines.map(PluginsJSON.string))
            }
            return area
        }

        /// A value of no shape the form knows, edited as JSON.
        private func jsonField(_ value: PluginsJSON?, onChange: @escaping @MainActor (PluginsJSON?) -> Void) -> NSView {
            let font = NSFont.monospacedSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
            let line: CGFloat = 17
            let area = TextArea(
                text: value?.pretty ?? "", font: font, minHeight: 3 * line + 14, maxHeight: 14 * line + 14,
                theme: theme)
            let invalid = note(MavioStrings.schemaInvalidJson, theme.fonts.footnote, theme.colors.danger)
            invalid.isHidden = true
            area.onChange = { [weak area, weak invalid] text in
                let bad: Bool
                if text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    bad = false
                    onChange(nil)
                } else if let v = PluginsJSON(parsing: text) {
                    bad = false
                    onChange(v)
                } else {
                    bad = true
                }
                area?.bad = bad
                invalid?.isHidden = !bad
            }
            let column = NSStackView.vertical([area, invalid], spacing: MavioSpacing.s4)
            area.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            return column
        }

        // MARK: Lists of objects

        /// Objects of a list, as cards that open to their fields.
        private func objectList(_ s: PluginsSchema, value: [PluginsJSON], path: [Step]) -> NSView {
            let id = key(path)
            var views: [NSView] = value.indices.map { i in item(s, index: i, count: value.count, list: id, path: path) }
            let add = MavioButton(MavioStrings.schemaAdd, icon: "add", kind: .secondary, theme: theme) {
                [weak self] in
                guard let self else { return }
                let list = self.get(path)?.array ?? []
                self.open[id] = list.count
                self.set(path, .array(list + [s.initial ?? .null]), redraw: true)
            }
            views.append(add)
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s8)
            for v in views.dropLast() { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            return column
        }

        private func item(_ s: PluginsSchema, index i: Int, count: Int, list id: String, path: [Step]) -> NSView {
            let isOpen = open[id] == i
            let itemPath = path + [.index(i)]
            let title = { [weak self] in
                itemTitle(s, options: self?.options ?? [:], self?.get(itemPath) ?? .null, i)
            }
            let toggle = MavioButton(title(), icon: isOpen ? "expand" : "forward", kind: .plain, theme: theme) {
                [weak self] in
                guard let self else { return }
                self.open[id] = self.open[id] == i ? nil : i
                self.render()
            }
            toggle.selected = false
            toggle.setAccessibilityValue(isOpen)
            refreshers.append { [weak toggle] in toggle?.title = title() }
            let remove = MavioButton(MavioStrings.schemaRemove, kind: .destructive, theme: theme) { [weak self] in
                guard let self else { return }
                let next = (self.get(path)?.array ?? []).enumerated().filter { $0.offset != i }.map(\.element)
                self.open[id] = nil
                self.set(path, next.isEmpty ? nil : .array(next), redraw: true)
            }
            let header = NSStackView.horizontal([toggle, Spacer(), remove], spacing: MavioSpacing.s8)
            let headerHolder = NSView()
            headerHolder.translatesAutoresizingMaskIntoConstraints = false
            headerHolder.addFilling(header, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: 6))
            headerHolder.heightAnchor.constraint(greaterThanOrEqualToConstant: 48).isActive = true
            var parts: [NSView] = [headerHolder]
            if isOpen {
                let line = Box(fill: theme.colors.border)
                line.size(height: 1)
                parts.append(line)
                let rows = objectRows(s, path: itemPath, nested: true)
                let column = NSStackView.vertical([], spacing: 0)
                for (r, row) in rows.enumerated() {
                    if r > 0 { add(divider(), to: column) }
                    let holder = NSView()
                    holder.translatesAutoresizingMaskIntoConstraints = false
                    holder.addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: 10))
                    add(holder, to: column)
                }
                parts.append(column)
            }
            let stack = NSStackView.vertical(parts, spacing: 0)
            parts.forEach { $0.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            let box = Box(radius: 12)
            box.border = theme.colors.border
            box.addFilling(stack)
            return box
        }
    }

    /// How many options a multiple choice shows as check boxes; more are tags.
    private let tagsAbove = 8

    /// The title of an object of schema s in a list: its name or ID, the
    /// name of the option when a source's option is that, among options by
    /// source, else its place.
    @MainActor private func itemTitle(
        _ s: PluginsSchema, options: [String: [PluginsOption]], _ v: PluginsJSON, _ i: Int
    ) -> String {
        if let o = v.object {
            for k in ["name", "id", "definition", "kind", "host"] {
                guard let x = o[k]?.string, !x.isEmpty else { continue }
                let spec = s.properties?.first { $0.name == k }?.schema.options
                let name = spec.flatMap { options[$0.source]?.first { $0.value == x }?.name } ?? ""
                return name.isEmpty ? x : name
            }
        }
        return MavioStrings.schemaItem(number: String(i + 1))
    }

    /// A check box that tells each change.
    final class ActionCheckBox: NSButton {
        private let onChange: @MainActor (Bool) -> Void

        /// A check box titled title, on or off.
        init(_ title: String, isOn: Bool, onChange: @escaping @MainActor (Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            setButtonType(.switch)
            self.title = title
            state = isOn ? .on : .off
            target = self
            action = #selector(changed)
            translatesAutoresizingMaskIntoConstraints = false
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { onChange(state == .on) }
    }

    // MARK: Options

    /// The tags of options a picker filters by, in groups, with their words
    /// and tones.
    @MainActor private var tagGroups: [[(key: String, label: String, tone: PickerTone)]] {
        [
            [("torrent", MavioStrings.schemaTagTorrent, .neutral), ("usenet", MavioStrings.schemaTagUsenet, .neutral)],
            [
                ("public", MavioStrings.schemaTagPublic, .success),
                ("semi-private", MavioStrings.schemaTagSemiPrivate, .warning),
                ("private", MavioStrings.schemaTagPrivate, .danger),
            ],
        ]
    }

    @MainActor private func knownTag(_ key: String) -> (key: String, label: String, tone: PickerTone)? {
        tagGroups.joined().first { $0.key == key }
    }

    /// A language's name without its region, the tag itself when it has none.
    private func languageName(_ tag: String) -> String {
        let code = Locale.Language(identifier: tag).languageCode?.identifier ?? tag
        return Locale.current.localizedString(forLanguageCode: code) ?? tag
    }

    /// Options as a picker lists them, by name: protocol and privacy as
    /// badges, the language beside them.
    @MainActor private func pickerItems(_ list: [PluginsOption]) -> [PickerItem] {
        list.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }.map { o in
            let meta = o.tags.filter { knownTag($0) == nil }.map(languageName).joined(separator: " · ")
            return PickerItem(
                value: o.value, label: o.name, detail: o.description_p, meta: meta.isEmpty ? nil : meta,
                badges: o.tags.compactMap { knownTag($0).map { ($0.label, $0.tone) } }, keys: o.tags)
        }
    }

    /// The system proxy's ID.
    private let systemProxy = "system"

    /// The server's proxies as a picker lists them: none first, then the
    /// system proxy, then the others by name, each with its address.
    @MainActor private func proxyItems(_ list: [PluginsOption]) -> [PickerItem] {
        var out = [PickerItem(value: "", label: MavioStrings.proxiesNone, detail: MavioStrings.proxiesNoneDetail)]
        if let system = list.first(where: { $0.value == systemProxy }) {
            out.append(
                PickerItem(
                    value: systemProxy, label: MavioStrings.proxiesSystem,
                    detail: system.description_p.isEmpty ? MavioStrings.proxiesSystemUnset : system.description_p))
        }
        let others = list.filter { $0.value != systemProxy }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
        for o in others { out.append(PickerItem(value: o.value, label: o.name, detail: o.description_p)) }
        return out
    }

    /// The groups of filters a list of options can be told apart by, those
    /// of two tags or more.
    @MainActor private func pickerFilters(_ list: [PluginsOption]) -> [[(key: String, label: String)]] {
        let present = Set(list.flatMap(\.tags))
        return tagGroups.map { group in group.filter { present.contains($0.key) }.map { ($0.key, $0.label) } }
            .filter { $0.count > 1 }
    }
#endif

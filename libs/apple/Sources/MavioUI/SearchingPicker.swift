import MavioDesign
import SwiftUI

/// A choice of options by their names. On iPhone and iPad sixteen options
/// or more open a page of their own searching them, as the web UI's long
/// selects; fewer, and on the television and the Mac, it is a list
/// picker.
public struct SearchingPicker<Label: View>: View {
    private let title: String
    @Binding private var selection: String
    private let options: [(value: String, name: String)]
    private let label: Label

    /// A picker of options, titled title on its page, label naming it in
    /// its row.
    public init(
        _ title: String, selection: Binding<String>, options: [(value: String, name: String)],
        @ViewBuilder label: () -> Label
    ) {
        self.title = title
        _selection = selection
        self.options = options
        self.label = label()
    }

    /// The picker's row.
    public var body: some View {
        #if os(iOS)
            if options.count >= 16 {
                NavigationLink {
                    OptionSearchList(title: title, selection: $selection, options: options)
                } label: {
                    LabeledContent {
                        Text(options.first { $0.value == selection }?.name ?? "")
                    } label: {
                        label
                    }
                }
            } else {
                picker
            }
        #else
            picker
        #endif
    }

    private var picker: some View {
        Picker(selection: $selection) {
            ForEach(options, id: \.value) { Text($0.name).tag($0.value) }
        } label: {
            label
        }
        .listPicker()
    }
}

extension SearchingPicker where Label == Text {
    /// A picker of options titled title.
    public init(_ title: String, selection: Binding<String>, options: [(value: String, name: String)]) {
        self.init(title, selection: selection, options: options) { Text(title) }
    }
}

#if os(iOS)
    /// The options of a searching picker holding what is typed, the one
    /// chosen checked; choosing one goes back.
    private struct OptionSearchList: View {
        let title: String
        @Binding var selection: String
        let options: [(value: String, name: String)]
        @State private var query = ""
        @Environment(\.dismiss) private var dismiss
        @Environment(\.theme) private var theme

        var body: some View {
            let q = query.trimmingCharacters(in: .whitespaces)
            let shown = options.filter {
                q.isEmpty || $0.name.range(of: q, options: [.caseInsensitive, .diacriticInsensitive]) != nil
            }
            List {
                ForEach(shown, id: \.value) { o in
                    Button {
                        selection = o.value
                        dismiss()
                    } label: {
                        HStack {
                            Text(o.name).foregroundStyle(theme.colors.textPrimary)
                            Spacer()
                            if o.value == selection {
                                Image(systemName: "checkmark").foregroundStyle(theme.colors.accentText)
                            }
                        }
                    }
                }
                if shown.isEmpty {
                    Text(MavioStrings.schemaNoMatches).foregroundStyle(theme.colors.textSecondary)
                }
            }
            .searchable(text: $query, placement: .navigationBarDrawer(displayMode: .always))
            .navigationTitle(title)
            .inlineTitle()
        }
    }
#endif

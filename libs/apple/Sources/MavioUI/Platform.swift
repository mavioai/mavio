import SwiftUI

#if canImport(UIKit)
    import UIKit
#elseif canImport(AppKit)
    import AppKit
#endif

// What the screens shared by the iPhone, Apple TV and the Mac do as each
// platform can: a modifier or call here does nothing where the platform has
// no such thing.

extension View {
    /// The navigation title small in the bar, where the bar has sizes.
    public func inlineTitle() -> some View {
        #if os(iOS)
            navigationBarTitleDisplayMode(.inline)
        #else
            self
        #endif
    }

    /// The navigation title large at the top of the page, where the bar
    /// has sizes.
    public func largeTitle() -> some View {
        #if os(iOS)
            navigationBarTitleDisplayMode(.large)
        #else
            self
        #endif
    }

    /// A field of a web address: the keyboard and suggestions for one, where
    /// there are.
    public func addressField() -> some View {
        #if os(macOS)
            self
        #else
            textContentType(.URL).keyboardType(.URL)
        #endif
    }

    /// A list or form without its background, the page's showing, where a
    /// list has one to hide.
    public func hiddenListBackground() -> some View {
        #if os(tvOS)
            self
        #else
            scrollContentBackground(.hidden)
        #endif
    }

    /// A keyboard of digits, and a decimal point if decimals, where there
    /// is an on-screen keyboard.
    public func numberKeyboard(decimals: Bool = false) -> some View {
        #if os(macOS)
            self
        #else
            keyboardType(decimals ? .decimalPad : .numberPad)
        #endif
    }

    /// No automatic capitals, for addresses, paths and names of things,
    /// where the keyboard capitalizes.
    public func noCapitals() -> some View {
        #if os(macOS)
            self
        #else
            textInputAutocapitalization(.never)
        #endif
    }

    /// A row's actions: swiped to on the iPhone and the Mac; on Apple TV,
    /// which has no swipe, a menu of them when menu, else none, the row
    /// having its own.
    public func rowActions(menu: Bool = false, @ViewBuilder _ actions: () -> some View) -> some View {
        #if os(tvOS)
            // A menu with nothing in it does not open.
            contextMenu {
                if menu { actions() }
            }
        #else
            swipeActions(content: actions)
        #endif
    }
}

extension ToolbarItemPlacement {
    /// The trailing end of the navigation bar, or of the window's toolbar.
    public static var trailingBar: ToolbarItemPlacement {
        #if os(iOS)
            .topBarTrailing
        #else
            .primaryAction
        #endif
    }
}

extension ToolbarItemPlacement {
    /// The bar at the bottom of a sheet, or beside its other buttons where
    /// it has none.
    public static var bottomOrSecondary: ToolbarItemPlacement {
        #if os(iOS)
            .bottomBar
        #else
            .secondaryAction
        #endif
    }
}

extension SearchFieldPlacement {
    /// Search's field always shown: under the navigation bar's title, or in
    /// the window's toolbar.
    public static var pageField: SearchFieldPlacement {
        #if os(iOS)
            .navigationBarDrawer(displayMode: .always)
        #else
            .toolbar
        #endif
    }
}

/// Whether text can be copied for another app: not on Apple TV.
public let canCopy: Bool = {
    #if os(tvOS)
        false
    #else
        true
    #endif
}()

/// Copies text to the clipboard, where there is one.
@MainActor public func copyToClipboard(_ text: String) {
    #if os(iOS)
        UIPasteboard.general.string = text
    #elseif os(macOS)
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    #endif
}

/// Closes the keyboard, or leaves the field being typed in.
@MainActor public func endEditing() {
    #if canImport(UIKit)
        UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
    #elseif canImport(AppKit)
        NSApp.keyWindow?.makeFirstResponder(nil)
    #endif
}

extension View {
    /// Text that can be selected to copy, where text can be.
    public func selectable() -> some View {
        #if os(tvOS)
            self
        #else
            textSelection(.enabled)
        #endif
    }
}

extension View {
    /// A field with a rounded border, where fields take styles.
    public func borderedField() -> some View {
        #if os(tvOS)
            self
        #else
            textFieldStyle(.roundedBorder)
        #endif
    }
}

/// Whether a file can be saved where the user chooses: not on Apple TV.
public let canSaveFiles: Bool = {
    #if os(tvOS)
        false
    #else
        true
    #endif
}()

extension View {
    /// Asks where to save file once it is set, then unsets it, where files
    /// can be saved.
    public func savesFile(_ file: Binding<URL?>) -> some View {
        #if os(tvOS)
            self
        #else
            fileMover(
                isPresented: Binding(get: { file.wrappedValue != nil }, set: { if !$0 { file.wrappedValue = nil } }),
                file: file.wrappedValue
            ) { _ in file.wrappedValue = nil }
        #endif
    }
}

extension View {
    /// A picker that opens its choices as a page of their own, where
    /// pages are pushed; a menu on the Mac.
    public func listPicker() -> some View {
        #if os(macOS)
            pickerStyle(.menu)
        #else
            pickerStyle(.navigationLink)
        #endif
    }
}

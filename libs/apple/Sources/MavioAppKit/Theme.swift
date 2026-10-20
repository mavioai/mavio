#if os(macOS)
    import AppKit
    import MavioCommon
    import MavioDesign
    import Observation

    /// The tokens views draw with: the colors, following the appearance;
    /// the type scale and the layout of the desktop or of television mode.
    public struct Theme: Sendable {
        /// The colors.
        public var colors: MavioNSColors
        /// The type scale.
        public var fonts: MavioFonts
        /// The sizes of the form factor.
        public var layout: MavioLayout

        /// The desktop's theme, as the web UI's in a wide window.
        public static let desktop = Theme(colors: .system, fonts: .desktop, layout: .desktop)

        /// Television mode's theme (docs/tv.md §2).
        public static let tv = Theme(colors: .system, fonts: .tv, layout: .tv)

        /// Whether this is television mode's.
        public var isTV: Bool { layout.focusScale > 1 }
    }

    /// Runs apply now and again whenever what it read of an observable
    /// object changes, on the main actor, until the token is dropped or
    /// cancelled: AppKit's views follow the models as SwiftUI's do.
    @MainActor
    public final class Observer {
        private var active = true
        private let apply: @MainActor () -> Void

        /// Starts observing what apply reads.
        public init(_ apply: @escaping @MainActor () -> Void) {
            self.apply = apply
            track()
        }

        /// Stops observing.
        public func cancel() { active = false }

        private func track() {
            guard active else { return }
            withObservationTracking {
                apply()
            } onChange: { [weak self] in
                Task { @MainActor in self?.track() }
            }
        }
    }

    extension NSImage {
        /// An icon of the design system, as icons.json names it, as a
        /// template image tinted by what draws it; fill for its active state.
        @MainActor public static func icon(_ name: String, fill: Bool = false) -> NSImage {
            let asset = fill ? name + "Fill" : name
            let image = MavioStrings.bundle.image(forResource: asset) ?? NSImage()
            image.isTemplate = true
            return image
        }
    }

    extension NSColor {
        /// A color of its sRGB hex value, as the web UI writes the tiles'.
        public convenience init(hex: UInt32, alpha: CGFloat = 1) {
            self.init(
                srgbRed: CGFloat(hex >> 16 & 0xFF) / 255, green: CGFloat(hex >> 8 & 0xFF) / 255,
                blue: CGFloat(hex & 0xFF) / 255, alpha: alpha)
        }

        /// A color that is light in a light appearance and dark in a dark one.
        public static func appearance(light: NSColor, dark: NSColor) -> NSColor {
            NSColor(name: nil) { appearance in
                appearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua ? dark : light
            }
        }

        /// The color resolved in the appearance view draws in, for layers,
        /// which take a CGColor.
        @MainActor public func cg(in view: NSView) -> CGColor {
            var color = cgColor
            view.effectiveAppearance.performAsCurrentDrawingAppearance { color = self.cgColor }
            return color
        }
    }
#endif

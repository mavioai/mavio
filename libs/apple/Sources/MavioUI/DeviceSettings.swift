import MavioCommon
import SwiftUI

extension Appearance {
    /// The color scheme it forces, if any.
    public var scheme: ColorScheme? {
        switch self {
        case .system: nil
        case .light: .light
        case .dark: .dark
        }
    }
}

import MavioKit
import MavioUI
import SwiftUI

#if canImport(UIKit)
    import UIKit
#endif

/// About, as the Android apps', the Apple TV's and the Mac's: the app — its
/// name as the servers know it, its version and build, the core's version
/// and the license — and the device — its name and ID as the servers know
/// them, its maker, model, system and architecture.
struct AboutSettingsView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme

    var body: some View {
        let device = model.device
        let info = Bundle.main.infoDictionary ?? [:]
        let version = info["CFBundleShortVersionString"] as? String ?? device.clientVersion
        let build = info["CFBundleVersion"] as? String
        SettingsList {
            Section(MavioStrings.aboutApp) {
                row(MavioStrings.aboutAppName, device.client)
                row(MavioStrings.aboutClientVersion, build.map { "\(version) (\($0))" } ?? version)
                row(MavioStrings.aboutCoreVersion, MavioKit.coreVersion)
                row(MavioStrings.aboutLicense, "GPL-3.0")
            }
            Section {
                row(MavioStrings.aboutDeviceName, device.name)
                row(MavioStrings.aboutDeviceId, device.id)
                row(MavioStrings.aboutManufacturer, "Apple")
                row(MavioStrings.aboutModel, Self.model)
                row(MavioStrings.aboutSystem, Self.system)
                row(MavioStrings.aboutArchitecture, Self.architecture)
            } header: {
                Text(MavioStrings.aboutDevice)
            } footer: {
                Text(MavioStrings.aboutDeviceFooter)
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsAbout)
        .inlineTitle()
    }

    /// A label and its value, which wraps when it is long, as a device's ID,
    /// and can be selected to copy.
    private func row(_ label: String, _ value: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s16) {
            Text(label)
            Spacer(minLength: 0)
            Text(value)
                .foregroundStyle(theme.colors.textSecondary)
                .multilineTextAlignment(.trailing)
                .textSelection(.enabled)
        }
    }

    /// The device's kind and model identifier, such as iPhone
    /// (iPhone16,2); the simulator gives the one it simulates.
    private static var model: String {
        var system = utsname()
        uname(&system)
        var id = withUnsafeBytes(of: &system.machine) { bytes in
            String(decoding: bytes.prefix { $0 != 0 }, as: UTF8.self)
        }
        if let simulated = ProcessInfo.processInfo.environment["SIMULATOR_MODEL_IDENTIFIER"] { id = simulated }
        #if canImport(UIKit)
            return "\(UIDevice.current.model) (\(id))"
        #else
            return id
        #endif
    }

    /// The system and its version, such as iOS 27.0.
    private static var system: String {
        #if canImport(UIKit)
            "\(UIDevice.current.systemName) \(UIDevice.current.systemVersion)"
        #else
            "macOS \(ProcessInfo.processInfo.operatingSystemVersionString)"
        #endif
    }

    /// The architecture the app runs on.
    private static var architecture: String {
        #if arch(arm64)
            "arm64"
        #else
            "x86_64"
        #endif
    }
}

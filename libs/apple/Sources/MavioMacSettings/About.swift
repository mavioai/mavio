#if os(macOS)
    import AppKit
    import MavioMac

    /// About, at the end of the settings, as the Android apps': the app —
    /// its name as the servers know it, its version and build, the core's
    /// version and the license — and the Mac — its name and ID as the
    /// servers know them, its maker, model, system and architecture.
    final class AboutPage: Page {
        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsAbout
            let device = model.device
            let app: [NSView] = [
                FormRow.value(MavioStrings.aboutAppName, device.client, theme: theme),
                FormRow.value(
                    MavioStrings.aboutClientVersion, Bundle.main.mavioVersionAndBuild, theme: theme),
                FormRow.value(MavioStrings.aboutCoreVersion, coreVersion, theme: theme),
                FormRow.value(MavioStrings.aboutLicense, "GPL-3.0", theme: theme),
            ]
            let system = ProcessInfo.processInfo.operatingSystemVersion
            let mac: [NSView] = [
                FormRow.value(MavioStrings.aboutDeviceName, device.name, theme: theme),
                FormRow.value(MavioStrings.aboutDeviceId, device.id, theme: theme),
                FormRow.value(MavioStrings.aboutManufacturer, "Apple", theme: theme),
                FormRow.value(MavioStrings.aboutModel, Self.model, theme: theme),
                FormRow.value(
                    MavioStrings.aboutSystem,
                    "macOS \(system.majorVersion).\(system.minorVersion).\(system.patchVersion)", theme: theme),
                FormRow.value(MavioStrings.aboutArchitecture, Self.architecture, theme: theme),
            ]
            let form = FormPage(title: MavioStrings.settingsAbout, theme: theme)
            form.add(FormSection(MavioStrings.aboutApp, rows: app, theme: theme))
            form.add(
                FormSection(MavioStrings.aboutDevice, footer: MavioStrings.aboutDeviceFooter, rows: mac, theme: theme))
            show(form)
        }

        /// The Mac's model identifier, such as Mac16,10.
        private static var model: String {
            var size = 0
            sysctlbyname("hw.model", nil, &size, nil, 0)
            guard size > 0 else { return "Mac" }
            var bytes = [CChar](repeating: 0, count: size)
            sysctlbyname("hw.model", &bytes, &size, nil, 0)
            return String(decoding: bytes.prefix { $0 != 0 }.map { UInt8(bitPattern: $0) }, as: UTF8.self)
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
#endif

#if os(macOS)
    import AppKit
    import MavioMac

    /// Devices, as the web UI's: the user's devices signed in to the server,
    /// to sign the others out or control one that is online.
    final class DevicesPage: ServerPage {
        private var devices: [DeviceSession]?
        private var failure: String?
        private var busy = false
        private lazy var form = FormPage(title: MavioStrings.settingsDevices, theme: theme)
        private lazy var list = section(MavioStrings.settingsDevicesMessage)

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsDevices
            form.add(list)
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func render() {
            list.setRows((devices ?? []).map(row))
            list.footer = failure
            if devices == nil, let failure {
                showError(failure) { [weak self] in
                    self?.show(LoadingView())
                    self?.appeared()
                }
            } else {
                show(form)
            }
            refocus()
        }

        private func row(_ device: DeviceSession) -> NSView {
            let tile = Box(fill: theme.colors.fill, radius: 9)
            tile.size(width: 36, height: 36)
            let icon = IconView("devices", size: 20, color: theme.colors.textPrimary)
            tile.addSubview(icon)
            icon.center(in: tile)
            let lines = text(Self.shownName(device), detail(device))
            for case let label as NSTextField in lines.arrangedSubviews {
                label.maximumNumberOfLines = 1
                label.lineBreakMode = .byTruncatingTail
            }
            let content = NSStackView.horizontal([tile, lines], spacing: MavioSpacing.s12)
            var controls: [NSView] = []
            if device.current {
                controls.append(
                    NSTextField.label(
                        MavioStrings.settingsThisDevice, theme.fonts.callout, color: theme.colors.textSecondary))
            } else {
                if device.online {
                    controls.append(
                        button(MavioStrings.remoteControl, kind: .link) { [weak self] in
                            guard let self else { return }
                            context.openRemote(server, device.id)
                        })
                }
                let signOut = button(MavioStrings.authSignOut, kind: .destructive) { [weak self] in
                    self?.signOut(device)
                }
                signOut.isEnabled = !busy
                controls.append(signOut)
            }
            return ControlRow(content, controls: controls, theme: theme)
        }

        /// The device's name, else its client's.
        private static func shownName(_ d: DeviceSession) -> String {
            d.deviceName.isEmpty ? (d.client.isEmpty ? "—" : d.client) : d.deviceName
        }

        private func detail(_ d: DeviceSession) -> String {
            let state: String? =
                if let playing = d.playing {
                    MavioStrings.remotePlaying(name: playing.card.title)
                } else if d.online {
                    MavioStrings.remoteOnline
                } else {
                    d.lastSeen.map { $0.formatted(date: .abbreviated, time: .shortened) }
                }
            return ([d.client] + [state].compactMap(\.self)).filter { !$0.isEmpty }.joined(separator: " · ")
        }

        private func load() async {
            do {
                devices = try await model.client.devices(serverId: server.id)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func signOut(_ device: DeviceSession) {
            busy = true
            render()
            run { [weak self] in
                guard let self else { return }
                do {
                    try await model.client.signOutDevice(serverId: server.id, sessionId: device.id)
                } catch {
                    failure = error.reason
                }
                busy = false
                await load()
            }
        }
    }
#endif

#if os(macOS)
    import CoreAudio

    /// An audio device the player's sound can play to.
    struct AudioDevice: Equatable {
        /// Its unique ID, as AVPlayer takes it.
        let id: String
        /// Its name.
        let name: String

        /// The devices with outputs, by Core Audio.
        static func outputs() -> [AudioDevice] {
            var address = AudioObjectPropertyAddress(
                mSelector: kAudioHardwarePropertyDevices, mScope: kAudioObjectPropertyScopeGlobal,
                mElement: kAudioObjectPropertyElementMain)
            var size: UInt32 = 0
            guard
                AudioObjectGetPropertyDataSize(AudioObjectID(kAudioObjectSystemObject), &address, 0, nil, &size)
                    == noErr
            else { return [] }
            var ids = [AudioObjectID](repeating: 0, count: Int(size) / MemoryLayout<AudioObjectID>.size)
            guard
                AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &address, 0, nil, &size, &ids)
                    == noErr
            else { return [] }
            return ids.compactMap { device in
                guard hasOutput(device), let id = string(device, kAudioDevicePropertyDeviceUID),
                    let name = string(device, kAudioObjectPropertyName)
                else { return nil }
                return AudioDevice(id: id, name: name)
            }
        }

        private static func hasOutput(_ device: AudioObjectID) -> Bool {
            var address = AudioObjectPropertyAddress(
                mSelector: kAudioDevicePropertyStreams, mScope: kAudioObjectPropertyScopeOutput,
                mElement: kAudioObjectPropertyElementMain)
            var size: UInt32 = 0
            return AudioObjectGetPropertyDataSize(device, &address, 0, nil, &size) == noErr && size > 0
        }

        private static func string(_ device: AudioObjectID, _ selector: AudioObjectPropertySelector) -> String? {
            var address = AudioObjectPropertyAddress(
                mSelector: selector, mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
            var value: Unmanaged<CFString>?
            var size = UInt32(MemoryLayout<Unmanaged<CFString>?>.size)
            guard AudioObjectGetPropertyData(device, &address, 0, nil, &size, &value) == noErr, let value else {
                return nil
            }
            return value.takeRetainedValue() as String
        }
    }
#endif

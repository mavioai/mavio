import AVFoundation
import MediaPlayer
import MavioUI
import SwiftUI

/// What a vertical swipe over the player changes: the screen's brightness
/// on the left half, the volume on the right (docs/clients.md §6.5).
enum SwipeLevel: Equatable {
    case brightness
    case volume

    var icon: String { self == .brightness ? "lightMode" : "volume" }
}

/// The level a swipe sets and what it set it to, which the player shows.
struct LevelShown: Equatable {
    let kind: SwipeLevel
    var value: Double
}

/// The swipe in progress: what it changes and the level it started from.
struct SwipeStart {
    let kind: SwipeLevel
    let value: Double
}

/// The system's volume, which only `MPVolumeView`'s slider sets; the view
/// sits in the player hidden, which also keeps the system's volume HUD away.
@MainActor
final class SystemVolume {
    fileprivate weak var slider: UISlider?

    /// The volume, 0 to 1.
    var value: Double {
        get { Double(slider?.value ?? AVAudioSession.sharedInstance().outputVolume) }
        set { slider?.setValue(Float(min(max(newValue, 0), 1)), animated: false) }
    }
}

/// The hidden `MPVolumeView` whose slider `SystemVolume` moves.
struct VolumeView: UIViewRepresentable {
    let volume: SystemVolume

    func makeUIView(context: Context) -> MPVolumeView {
        let view = MPVolumeView(frame: CGRect(x: -100, y: -100, width: 1, height: 1))
        view.alpha = 0.01
        volume.slider = view.subviews.compactMap { $0 as? UISlider }.first
        return view
    }

    func updateUIView(_ view: MPVolumeView, context: Context) {
        if volume.slider == nil { volume.slider = view.subviews.compactMap { $0 as? UISlider }.first }
    }
}

/// The screen's brightness, 0 to 1, of the scene the app shows in.
@MainActor
enum Brightness {
    private static var screen: UIScreen? {
        UIApplication.shared.connectedScenes.compactMap { ($0 as? UIWindowScene)?.screen }.first
    }

    static var value: Double {
        get { Double(screen?.brightness ?? 0.5) }
        set { screen?.brightness = CGFloat(min(max(newValue, 0), 1)) }
    }
}

/// The level a swipe sets, shown at the top of the player: its icon over a
/// bar, in glass.
struct LevelIndicator: View {
    let level: LevelShown

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            Icon(level.kind.icon, fill: true, size: 18)
            GeometryReader { geo in
                Capsule().fill(.white.opacity(0.25))
                    .overlay(alignment: .leading) {
                        Capsule().fill(.white).frame(width: geo.size.width * level.value)
                    }
            }
            .frame(width: 120, height: 4)
        }
        .foregroundStyle(.white)
        .padding(.horizontal, MavioSpacing.s16)
        .frame(height: 40)
        .glass(in: Capsule())
        .environment(\.colorScheme, .dark)
        .accessibilityElement()
        .accessibilityValue(Text(level.value, format: .percent.precision(.fractionLength(0))))
    }
}

/// The speed a held press plays at, shown at the top of the player while
/// it is held.
struct HeldSpeedIndicator: View {
    var body: some View {
        HStack(spacing: MavioSpacing.s8) {
            Icon("speed", fill: true, size: 18)
            Text("2×").type(MavioTypography.standard.callout).fontWeight(.semibold).monospacedDigit()
        }
        .foregroundStyle(.white)
        .padding(.horizontal, MavioSpacing.s16)
        .frame(height: 40)
        .glass(in: Capsule())
        .environment(\.colorScheme, .dark)
    }
}

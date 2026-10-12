import AVFoundation
import AVKit
import AetherEngine
import SwiftUI

/// The picture of a player, with the subtitles AetherEngine hands over.
public struct PlayerSurface: View {
    private let player: Player

    /// The picture of player.
    public init(player: Player) {
        self.player = player
    }

    /// The engine's surface.
    public var body: some View {
        ZStack {
            Color.black
            if let avPlayer = player.avPlayer {
                AVPlayerSurface(player: avPlayer) { player.pictureInPicture = $0 }
            } else if let engine = player.aether {
                AetherPlayerSurface(engine: engine)
                SubtitleOverlay(cues: player.cues, time: player.cueTime, videoSize: player.videoSize)
            }
        }
        .ignoresSafeArea()
    }
}

#if canImport(UIKit)
    /// A view whose layer is an AVPlayerLayer.
    final class PlayerLayerView: UIView {
        override class var layerClass: AnyClass { AVPlayerLayer.self }
        var playerLayer: AVPlayerLayer { layer as! AVPlayerLayer }
    }

    /// AVPlayer's picture, with Picture in Picture.
    struct AVPlayerSurface: UIViewRepresentable {
        let player: AVPlayer
        let onPictureInPicture: (AVPictureInPictureController?) -> Void

        func makeUIView(context: Context) -> PlayerLayerView {
            let view = PlayerLayerView()
            view.backgroundColor = .black
            view.playerLayer.player = player
            view.playerLayer.videoGravity = .resizeAspect
            let pip = AVPictureInPictureController(playerLayer: view.playerLayer)
            #if os(iOS)
                pip?.canStartPictureInPictureAutomaticallyFromInline = true
            #endif
            onPictureInPicture(pip)
            return view
        }

        func updateUIView(_ view: PlayerLayerView, context: Context) {
            if view.playerLayer.player !== player { view.playerLayer.player = player }
        }
    }
#else
    /// A view whose layer is an AVPlayerLayer.
    final class PlayerLayerView: NSView {
        let playerLayer = AVPlayerLayer()

        override init(frame: NSRect) {
            super.init(frame: frame)
            wantsLayer = true
            layer = playerLayer
        }

        required init?(coder: NSCoder) { nil }
    }

    /// AVPlayer's picture, with Picture in Picture.
    struct AVPlayerSurface: NSViewRepresentable {
        let player: AVPlayer
        let onPictureInPicture: (AVPictureInPictureController?) -> Void

        func makeNSView(context: Context) -> PlayerLayerView {
            let view = PlayerLayerView()
            view.playerLayer.player = player
            view.playerLayer.videoGravity = .resizeAspect
            onPictureInPicture(AVPictureInPictureController(playerLayer: view.playerLayer))
            return view
        }

        func updateNSView(_ view: PlayerLayerView, context: Context) {
            if view.playerLayer.player !== player { view.playerLayer.player = player }
        }
    }
#endif

/// Draws the cues showing at a time inside the picture: text along its
/// bottom, bitmaps where their canvas puts them.
struct SubtitleOverlay: View {
    let cues: [SubtitleCue]
    let time: TimeInterval
    let videoSize: CGSize

    var body: some View {
        let showing = cues.filter { $0.startTime <= time && time < $0.endTime }
        GeometryReader { geo in
            let picture = Self.fit(videoSize, in: geo.size)
            ZStack {
                ForEach(showing) { cue in
                    switch cue.body {
                    case let .image(image):
                        bitmap(image, in: picture)
                    default:
                        if let text = cue.text {
                            Text(text)
                                .font(.system(size: max(15, picture.height / 18), weight: .medium))
                                .multilineTextAlignment(.center)
                                .foregroundStyle(.white)
                                .shadow(color: .black, radius: 2)
                                .shadow(color: .black, radius: 4)
                                .padding(.horizontal, 24)
                                .padding(.bottom, picture.height * 0.06)
                                .frame(width: picture.width, height: picture.height, alignment: .bottom)
                                .position(x: picture.midX, y: picture.midY)
                        }
                    }
                }
            }
        }
        .allowsHitTesting(false)
    }

    /// The rectangle a picture of a size takes when fitted into a space;
    /// all of it before the size is known.
    static func fit(_ size: CGSize, in space: CGSize) -> CGRect {
        guard size.width > 0, size.height > 0 else { return CGRect(origin: .zero, size: space) }
        let scale = min(space.width / size.width, space.height / size.height)
        let fitted = CGSize(width: size.width * scale, height: size.height * scale)
        return CGRect(
            x: (space.width - fitted.width) / 2, y: (space.height - fitted.height) / 2,
            width: fitted.width, height: fitted.height)
    }

    private func bitmap(_ image: SubtitleImage, in picture: CGRect) -> some View {
        let canvas =
            image.canvasSize.width > 0
            ? image.canvasSize : CGSize(width: image.position.maxX, height: image.position.maxY)
        let area = Self.fit(canvas, in: picture.size).offsetBy(dx: picture.minX, dy: picture.minY)
        let scale = area.width / max(canvas.width, 1)
        return Image(decorative: image.cgImage, scale: 1)
            .resizable()
            .frame(width: image.position.width * scale, height: image.position.height * scale)
            .position(
                x: area.minX + image.position.midX * scale,
                y: area.minY + image.position.midY * scale
            )
    }
}

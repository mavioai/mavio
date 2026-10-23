#if os(macOS)
    import AVFoundation
    import AVKit
    import AetherEngine
    import AppKit
    import MavioMac
    import MavioPlayer

    /// The picture of a player, in AppKit: AVPlayer's layer, with Picture in
    /// Picture, or AetherEngine's view, with the still it shows after a
    /// paused seek and the subtitles it hands over — an ASS track drawn in
    /// its styles by libass, other cues as text along the picture's bottom
    /// or as bitmaps where their canvas puts them.
    @MainActor
    final class PictureView: FlippedView {
        private let player: Player
        private var avView: AVPlayerLayerView?
        private var aetherView: AetherPlayerView?
        /// The engine aetherView is bound to, which it unbinds from.
        private weak var boundEngine: AetherEngine?
        private let stillView = NSView()
        private let styledView = NSView()
        private let subtitleView = SubtitleView()
        private var observer: Observer?
        private var styledTask: Task<Void, Never>?

        /// The picture of player.
        init(player: Player) {
            self.player = player
            super.init(frame: .zero)
            wantsLayer = true
            layer?.backgroundColor = .black
            translatesAutoresizingMaskIntoConstraints = false
            for view in [stillView, styledView] {
                view.wantsLayer = true
                view.layer?.contentsGravity = .resize
                view.layer?.actions = ["contents": NSNull()]
            }
            addSubview(stillView)
            addSubview(styledView)
            addSubview(subtitleView)
            observer = Observer { [weak self] in self?.update() }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Lets the engines go: unbinds AetherEngine's view and stops
        /// drawing the subtitles.
        func release() {
            observer?.cancel()
            observer = nil
            styledTask?.cancel()
            styledTask = nil
            if let aetherView { boundEngine?.unbind(view: aetherView) }
            avView?.playerLayer.player = nil
        }

        isolated deinit { release() }

        /// How far above the view's bottom text subtitles keep, out of the
        /// way of the controls while they show.
        var subtitleLift: CGFloat = 0 {
            didSet { if subtitleLift != oldValue { subtitleView.lift = subtitleLift } }
        }

        /// Clicks go to what the picture lies in.
        override func hitTest(_ point: NSPoint) -> NSView? { nil }

        // MARK: Following the player

        private func update() {
            updateEngines()
            stillView.layer?.contents = player.still
            stillView.isHidden = player.still == nil
            subtitleView.isHidden = player.aether == nil || player.styledSubtitles != nil
            subtitleView.set(cues: player.cues, time: player.cueTime, videoSize: player.videoSize)
            updateStyled()
            needsLayout = true
        }

        private func updateEngines() {
            if let avPlayer = player.avPlayer {
                if avView?.playerLayer.player !== avPlayer {
                    avView?.removeFromSuperview()
                    let view = AVPlayerLayerView()
                    view.playerLayer.player = avPlayer
                    view.playerLayer.videoGravity = .resizeAspect
                    addSubview(view, positioned: .below, relativeTo: stillView)
                    avView = view
                    player.pictureInPicture = AVPictureInPictureController(playerLayer: view.playerLayer)
                }
            } else if let view = avView {
                view.playerLayer.player = nil
                view.removeFromSuperview()
                avView = nil
            }
            if let engine = player.aether {
                if boundEngine !== engine {
                    if let old = aetherView { boundEngine?.unbind(view: old) }
                    aetherView?.removeFromSuperview()
                    let view = AetherPlayerView()
                    addSubview(view, positioned: .below, relativeTo: stillView)
                    view.frame = bounds
                    engine.bind(view: view)
                    aetherView = view
                    boundEngine = engine
                }
            } else if let view = aetherView {
                boundEngine?.unbind(view: view)
                view.removeFromSuperview()
                aetherView = nil
                boundEngine = nil
            }
        }

        /// Draws the ASS track at the source time of each frame, thirty
        /// times a second, while there is one.
        private func updateStyled() {
            let styled = player.styledSubtitles != nil && player.aether != nil
            styledView.isHidden = !styled
            if styled, styledTask == nil {
                styledTask = Task { [weak self] in
                    while !Task.isCancelled {
                        self?.drawStyled()
                        try? await Task.sleep(for: .seconds(1.0 / 30))
                    }
                }
            } else if !styled {
                styledTask?.cancel()
                styledTask = nil
                styledView.layer?.contents = nil
            }
        }

        private func drawStyled() {
            guard let renderer = player.styledSubtitles, let engine = player.aether else { return }
            let picture = Self.fit(player.videoSize, in: bounds.size)
            let scale = window?.backingScaleFactor ?? 2
            let size = CGSize(width: (picture.width * scale).rounded(), height: (picture.height * scale).rounded())
            guard size.width > 0, size.height > 0 else { return }
            let rendered = renderer.render(at: engine.clock.sourceTime - player.subtitleOffset, size: size)
            if rendered.changed || styledView.layer?.contents == nil {
                styledView.layer?.contents = rendered.image
            }
        }

        override func layout() {
            super.layout()
            avView?.frame = bounds
            aetherView?.frame = bounds
            let picture = Self.fit(player.videoSize, in: bounds.size)
            stillView.frame = picture
            styledView.frame = picture
            subtitleView.frame = bounds
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
    }

    /// A view whose layer is an AVPlayerLayer.
    @MainActor
    private final class AVPlayerLayerView: NSView {
        let playerLayer = AVPlayerLayer()

        override init(frame: NSRect) {
            super.init(frame: frame)
            layer = playerLayer
            wantsLayer = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// Draws the cues showing at a time inside the picture: text along its
    /// bottom, bitmaps where their canvas puts them.
    @MainActor
    private final class SubtitleView: FlippedView {
        private var cues: [SubtitleCue] = []
        private var time: TimeInterval = 0
        private var videoSize: CGSize = .zero
        private var showing: [Int] = []
        /// How far above the bottom the text keeps.
        var lift: CGFloat = 0 { didSet { needsDisplay = true } }

        init() {
            super.init(frame: .zero)
            wantsLayer = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows the cues of cues showing at time, over a picture of
        /// videoSize; redraws only when they or the size change.
        func set(cues: [SubtitleCue], time: TimeInterval, videoSize: CGSize) {
            let showing = cues.filter { $0.startTime <= time && time < $0.endTime }.map(\.id)
            let changed = showing != self.showing || videoSize != self.videoSize
            (self.cues, self.time, self.videoSize, self.showing) = (cues, time, videoSize, showing)
            if changed { needsDisplay = true }
        }

        override func draw(_ dirtyRect: NSRect) {
            let picture = PictureView.fit(videoSize, in: bounds.size)
            let showing = cues.filter { $0.startTime <= time && time < $0.endTime }
            var lines: [String] = []
            for cue in showing {
                if case let .image(image) = cue.body {
                    bitmap(image, in: picture)
                } else if let text = cue.text {
                    lines.append(text)
                }
            }
            guard !lines.isEmpty else { return }
            let paragraph = NSMutableParagraphStyle()
            paragraph.alignment = .center
            let font = NSFont.systemFont(ofSize: max(15, picture.height / 18), weight: .medium)
            let text = lines.joined(separator: "\n")
            let width = max(picture.width - 48, 1)
            for blur in [CGFloat(4), 2] {
                let shadow = NSShadow()
                shadow.shadowColor = .black
                shadow.shadowBlurRadius = blur
                let string = NSAttributedString(
                    string: text,
                    attributes: [
                        .font: font, .foregroundColor: NSColor.white, .paragraphStyle: paragraph, .shadow: shadow,
                    ]
                )
                let size = string.boundingRect(
                    with: NSSize(width: width, height: .greatestFiniteMagnitude),
                    options: [.usesLineFragmentOrigin, .usesFontLeading]
                ).size
                let bottom = min(picture.maxY - picture.height * 0.06, bounds.height - lift)
                string.draw(
                    with: NSRect(
                        x: picture.minX + 24, y: bottom - ceil(size.height), width: width, height: ceil(size.height)),
                    options: [.usesLineFragmentOrigin, .usesFontLeading])
            }
        }

        private func bitmap(_ image: SubtitleImage, in picture: CGRect) {
            let canvas =
                image.canvasSize.width > 0
                ? image.canvasSize : CGSize(width: image.position.maxX, height: image.position.maxY)
            let area = PictureView.fit(canvas, in: picture.size).offsetBy(dx: picture.minX, dy: picture.minY)
            let scale = area.width / max(canvas.width, 1)
            let rect = CGRect(
                x: area.minX + image.position.minX * scale, y: area.minY + image.position.minY * scale,
                width: image.position.width * scale, height: image.position.height * scale)
            NSImage(cgImage: image.cgImage, size: rect.size).draw(
                in: rect, from: .zero, operation: .sourceOver, fraction: 1, respectFlipped: true, hints: nil)
        }
    }
#endif

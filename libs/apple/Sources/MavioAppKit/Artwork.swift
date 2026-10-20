#if os(macOS)
    import AppKit
    import MavioDesign
    import Nuke

    /// An image loaded from a server, over a placeholder of its final size: a
    /// plain fill while it loads, into which it fades, and the icon of what it
    /// shows only when there is none, so that no icon flashes before an image.
    /// It fills its bounds, cropping, unless fit shows it whole. The image is
    /// asked of its server at the width it is drawn at, in steps, as the
    /// web UI's `maxWidth`, once the view has a width.
    public final class ArtworkView: Box {
        private let imageLayer = CALayer()
        private let placeholder: IconView
        private let fade = CAGradientLayer()
        private var task: Task<Void, Never>?
        private var loaded: String?
        /// The width in pixels the image was asked at, 0 before it was.
        private var loadedWidth = 0
        /// The steps of width in pixels images are asked at, so that views
        /// of about one size share them in the caches.
        private static let widths = [160, 240, 320, 480, 640, 800, 1080, 1280, 1600, 1920, 2560, 3840]
        /// Whether the image is shown whole, as a logo, rather than filling.
        public var fit = false {
            didSet { imageLayer.contentsGravity = fit ? .resizeAspect : .resizeAspectFill }
        }
        /// Whether the fill shows behind the image; a logo has none.
        public var showsFill = true { didSet { fill = showsFill ? colors.fill : nil } }
        private let colors: MavioNSColors

        /// The image at url, which shows title; without one, icon on the
        /// placeholder.
        public init(_ url: String? = nil, title: String? = nil, icon: String = "movies", theme: Theme = .desktop) {
            colors = theme.colors
            placeholder = IconView(icon, size: 56, color: theme.colors.textTertiary)
            super.init(fill: theme.colors.fill)
            layer?.masksToBounds = true
            imageLayer.contentsGravity = .resizeAspectFill
            imageLayer.opacity = 0
            layer?.addSublayer(fade)
            layer?.addSublayer(imageLayer)
            fade.startPoint = CGPoint(x: 0.33, y: 1)
            fade.endPoint = CGPoint(x: 0.67, y: 0)
            fade.locations = [0, 0.7]
            fade.isHidden = true
            placeholder.alphaValue = 0.6
            placeholder.isHidden = true
            addSubview(placeholder)
            placeholder.center(in: self)
            let side = placeholder.widthAnchor.constraint(lessThanOrEqualTo: widthAnchor, multiplier: 0.26)
            side.priority = .required
            side.isActive = true
            for c in placeholder.constraints where c.firstAttribute == .width { c.priority = .defaultHigh }
            setAccessibilityElement(true)
            setAccessibilityRole(.image)
            setAccessibilityLabel(title ?? "")
            set(url, icon: icon)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Shows the image at url, or icon on the placeholder without one.
        public func set(_ url: String?, title: String? = nil, icon: String? = nil) {
            if let icon { placeholder.name = icon }
            if let title { setAccessibilityLabel(title) }
            guard url != loaded || url == nil else { return }
            loaded = url
            loadedWidth = 0
            task?.cancel()
            imageLayer.contents = nil
            imageLayer.opacity = 0
            showPlaceholder(false)
            guard let url, URL(string: url) != nil else {
                showPlaceholder(true)
                return
            }
            load()
        }

        /// The step of width the view needs, 0 before it has a width.
        private var neededWidth: Int {
            guard bounds.width > 0 else { return 0 }
            let pixels = Int((bounds.width * (window?.backingScaleFactor ?? 2)).rounded(.up))
            return Self.widths.first { $0 >= pixels } ?? Self.widths[Self.widths.count - 1]
        }

        /// Loads the image at the width needed, once the view has one, and
        /// again when it grows past the width loaded.
        private func load() {
            let width = neededWidth
            guard let loaded, width > loadedWidth,
                var components = URLComponents(string: loaded)
            else { return }
            // A server's images are scaled by it; others, as TMDB's, come at
            // the size their URL names.
            if components.path.contains("/images/") {
                components.queryItems =
                    (components.queryItems ?? []) + [URLQueryItem(name: "maxWidth", value: "\(width)")]
            } else if loadedWidth > 0 {
                return
            }
            guard let parsed = components.url else { return }
            let first = loadedWidth == 0
            loadedWidth = width
            task?.cancel()
            // An image in memory shows at once, without fading.
            if let cached = ImagePipeline.shared.cache.cachedImage(for: ImageRequest(url: parsed)) {
                show(cached.image, animated: false)
                return
            }
            task = Task { [weak self] in
                do {
                    let image = try await ImagePipeline.shared.image(for: parsed)
                    guard !Task.isCancelled else { return }
                    self?.show(image, animated: first)
                } catch {
                    guard !Task.isCancelled else { return }
                    self?.showPlaceholder(true)
                }
            }
        }

        /// Shows the fill alone, as a card loading.
        public func setEmpty() {
            loaded = nil
            loadedWidth = 0
            task?.cancel()
            imageLayer.contents = nil
            imageLayer.opacity = 0
            showPlaceholder(false)
        }

        private func show(_ image: NSImage, animated: Bool) {
            // The decoded bitmap itself: an NSImage's layer contents would
            // draw it anew at its size in points times the screen's scale,
            // which for a loaded image is its size in pixels.
            imageLayer.contents =
                image.cgImage(forProposedRect: nil, context: nil, hints: nil)
                ?? image.layerContents(forContentsScale: window?.backingScaleFactor ?? 2)
            CATransaction.begin()
            CATransaction.setDisableActions(!animated)
            CATransaction.setAnimationDuration(MavioDuration.slow)
            imageLayer.opacity = 1
            CATransaction.commit()
        }

        private func showPlaceholder(_ shown: Bool) {
            placeholder.isHidden = !shown
            fade.isHidden = !shown
        }

        override public func updateLayer() {
            super.updateLayer()
            fade.colors = [colors.fill.cg(in: self), NSColor.clear.cgColor]
        }

        override public func layout() {
            super.layout()
            CATransaction.begin()
            CATransaction.setDisableActions(true)
            imageLayer.frame = bounds
            fade.frame = bounds
            CATransaction.commit()
            load()
        }

        override public func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            load()
        }

        isolated deinit {
            task?.cancel()
        }
    }
#endif

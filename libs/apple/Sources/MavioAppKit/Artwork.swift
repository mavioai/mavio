#if os(macOS)
    import AppKit
    import MavioDesign
    import Nuke

    /// An image loaded from a server, over a placeholder of its final size: a
    /// plain fill while it loads, into which it fades, and the icon of what it
    /// shows only when there is none, so that no icon flashes before an image.
    /// It fills its bounds, cropping, unless fit shows it whole.
    public final class ArtworkView: Box {
        private let imageLayer = CALayer()
        private let placeholder: IconView
        private let fade = CAGradientLayer()
        private var task: Task<Void, Never>?
        private var loaded: String?
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
            task?.cancel()
            imageLayer.contents = nil
            imageLayer.opacity = 0
            showPlaceholder(false)
            guard let url, let parsed = URL(string: url) else {
                showPlaceholder(true)
                return
            }
            // An image in memory shows at once, without fading.
            if let cached = ImagePipeline.shared.cache.cachedImage(for: ImageRequest(url: parsed)) {
                show(cached.image, animated: false)
                return
            }
            task = Task { [weak self] in
                do {
                    let image = try await ImagePipeline.shared.image(for: parsed)
                    guard !Task.isCancelled else { return }
                    self?.show(image, animated: true)
                } catch {
                    guard !Task.isCancelled else { return }
                    self?.showPlaceholder(true)
                }
            }
        }

        /// Shows the fill alone, as a card loading.
        public func setEmpty() {
            loaded = nil
            task?.cancel()
            imageLayer.contents = nil
            imageLayer.opacity = 0
            showPlaceholder(false)
        }

        private func show(_ image: NSImage, animated: Bool) {
            imageLayer.contents = image.layerContents(forContentsScale: window?.backingScaleFactor ?? 2)
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
        }

        isolated deinit {
            task?.cancel()
        }
    }
#endif

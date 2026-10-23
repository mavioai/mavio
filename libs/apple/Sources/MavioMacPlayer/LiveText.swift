#if os(macOS)
    import AppKit
    import VisionKit

    /// Live Text over a paused picture: the text VisionKit finds in the
    /// frame, to select, copy and look up. It takes the pointer only over
    /// text and while text is selected, so that clicks elsewhere still play
    /// and pause; the controller lies over it.
    @MainActor
    final class LiveTextView: NSView {
        private let overlay = ImageAnalysisOverlayView()
        private let analyzer = ImageAnalyzer()
        /// The frame's size, for where the picture is in the view.
        private var frameSize = CGSize.zero
        private var task: Task<Void, Never>?

        init() {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            overlay.delegate = self
            overlay.preferredInteractionTypes = .automatic
            overlay.translatesAutoresizingMaskIntoConstraints = false
            addSubview(overlay)
            NSLayoutConstraint.activate([
                overlay.leadingAnchor.constraint(equalTo: leadingAnchor),
                overlay.trailingAnchor.constraint(equalTo: trailingAnchor),
                overlay.topAnchor.constraint(equalTo: topAnchor),
                overlay.bottomAnchor.constraint(equalTo: bottomAnchor),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Whether the Mac can find text in pictures.
        static var isSupported: Bool { ImageAnalyzer.isSupported }

        /// Finds the text of a frame after a moment, unless cleared first.
        func analyze(_ frame: @escaping @MainActor () async -> CGImage?) {
            clear()
            task = Task { [weak self] in
                try? await Task.sleep(for: .milliseconds(400))
                guard !Task.isCancelled, let image = await frame(), !Task.isCancelled, let self else { return }
                let configuration = ImageAnalyzer.Configuration([.text, .machineReadableCode])
                guard
                    let analysis = try? await self.analyzer.analyze(
                        image, orientation: .up, configuration: configuration),
                    !Task.isCancelled
                else { return }
                self.frameSize = CGSize(width: image.width, height: image.height)
                self.overlay.analysis = analysis
            }
        }

        /// Lets the text go, as the picture moves on.
        func clear() {
            task?.cancel()
            task = nil
            overlay.analysis = nil
        }

        override func hitTest(_ point: NSPoint) -> NSView? {
            guard overlay.analysis != nil else { return nil }
            let local = overlay.convert(point, from: superview)
            guard overlay.hasActiveTextSelection || overlay.hasInteractiveItem(at: local) else { return nil }
            return super.hitTest(point)
        }
    }

    extension LiveTextView: ImageAnalysisOverlayViewDelegate {
        /// The picture's place in the view, fitted to it with its aspect.
        nonisolated func contentsRect(for overlayView: ImageAnalysisOverlayView) -> CGRect {
            MainActor.assumeIsolated {
                let size = frameSize
                let bounds = overlayView.bounds.size
                guard size.width > 0, size.height > 0, bounds.width > 0, bounds.height > 0 else {
                    return CGRect(x: 0, y: 0, width: 1, height: 1)
                }
                let scale = min(bounds.width / size.width, bounds.height / size.height)
                let width = size.width * scale / bounds.width
                let height = size.height * scale / bounds.height
                return CGRect(x: (1 - width) / 2, y: (1 - height) / 2, width: width, height: height)
            }
        }
    }
#endif

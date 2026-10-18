#if os(iOS)
    import AVFoundation
    import AVKit
    import AetherEngine

    /// Picture in Picture while AetherEngine plays: around the native
    /// path's `AVPlayerLayer`, or around the software path's sample buffer
    /// layer with the transport its source answers. The engine learns when
    /// the window is open, which keeps its pipeline alive in the background
    /// and draws the subtitles into the software path's frames.
    @MainActor
    final class AetherPictureInPicture: NSObject {
        /// The controller, which the player's button starts and stops.
        let controller: AVPictureInPictureController
        /// The layer it was made around, which a new session replaces.
        let layer: CALayer
        private weak var engine: AetherEngine?
        private let playback: SampleBufferPlayback?

        /// Picture in Picture of the session the engine runs, unless it has
        /// no picture yet.
        init?(engine: AetherEngine) {
            guard AVPictureInPictureController.isPictureInPictureSupported() else { return nil }
            if let layer = engine.nativePlayerLayer, let controller = AVPictureInPictureController(playerLayer: layer) {
                (self.layer, self.controller, playback) = (layer, controller, nil)
            } else if let source = engine.softwarePiPSource {
                let playback = SampleBufferPlayback(source: source)
                controller = AVPictureInPictureController(
                    contentSource: AVPictureInPictureController.ContentSource(
                        sampleBufferDisplayLayer: source.layer, playbackDelegate: playback))
                (layer, self.playback) = (source.layer, playback)
            } else {
                return nil
            }
            self.engine = engine
            super.init()
            controller.delegate = self
            controller.canStartPictureInPictureAutomaticallyFromInline = true
        }

        /// Tells the window the playback changed, which it asks the software
        /// path's source about.
        func invalidate() {
            if playback != nil { controller.invalidatePlaybackState() }
        }

        /// The layer of the session the engine runs, Picture in Picture or not.
        static func layer(of engine: AetherEngine) -> CALayer? {
            engine.nativePlayerLayer ?? engine.softwarePiPSource?.layer
        }
    }

    extension AetherPictureInPicture: @preconcurrency AVPictureInPictureControllerDelegate {
        func pictureInPictureControllerWillStartPictureInPicture(_ controller: AVPictureInPictureController) {
            engine?.pictureInPictureActive = true
        }

        func pictureInPictureControllerDidStopPictureInPicture(_ controller: AVPictureInPictureController) {
            engine?.pictureInPictureActive = false
        }

        func pictureInPictureController(
            _ controller: AVPictureInPictureController, failedToStartPictureInPictureWithError error: any Error
        ) {
            engine?.pictureInPictureActive = false
        }
    }

    /// The transport of the window over the software path, answered by the
    /// engine's source on the axis of the frames it enqueues.
    @MainActor
    final class SampleBufferPlayback: NSObject {
        private let source: SoftwarePiPSource

        init(source: SoftwarePiPSource) {
            self.source = source
        }
    }

    extension SampleBufferPlayback: @preconcurrency AVPictureInPictureSampleBufferPlaybackDelegate {
        func pictureInPictureController(_ controller: AVPictureInPictureController, setPlaying playing: Bool) {
            source.setPlaying(playing)
            controller.invalidatePlaybackState()
        }

        func pictureInPictureControllerTimeRangeForPlayback(_ controller: AVPictureInPictureController) -> CMTimeRange {
            source.timeRange()
        }

        func pictureInPictureControllerIsPlaybackPaused(_ controller: AVPictureInPictureController) -> Bool {
            source.isPaused
        }

        func pictureInPictureController(
            _ controller: AVPictureInPictureController, didTransitionToRenderSize newRenderSize: CMVideoDimensions
        ) {}

        func pictureInPictureController(
            _ controller: AVPictureInPictureController, skipByInterval skipInterval: CMTime,
            completion: @escaping () -> Void
        ) {
            source.skip(by: skipInterval.seconds)
            completion()
        }
    }
#endif

import AetherEngine
import CoreGraphics
import Foundation
import Libass
import os

/// Renders ASS subtitles with libass: the script header and the event lines
/// AetherEngine hands over with `preserveASSMarkup`, in the fonts the
/// server lists as the item's attachments, with Core Text for the rest.
public final class ASSRenderer {
    private let library: OpaquePointer
    private let renderer: OpaquePointer
    private let track: UnsafeMutablePointer<ASS_Track>
    private var frameSize = CGSize.zero

    /// A renderer of the track the script header describes, with fonts,
    /// font file contents by name, available to it.
    public init?(header: String, fonts: [String: Data] = [:]) {
        guard let library = ass_library_init() else { return nil }
        // Only its errors and warnings are logged.
        ass_set_message_cb(
            library,
            { level, format, args, _ in
                guard level <= 2, let format, let args else { return }
                var buffer = [UInt8](repeating: 0, count: 512)
                let length = buffer.withUnsafeMutableBufferPointer {
                    $0.withMemoryRebound(to: CChar.self) { vsnprintf($0.baseAddress, $0.count, format, args) }
                }
                let message = String(decoding: buffer.prefix(max(0, min(Int(length), buffer.count - 1))), as: UTF8.self)
                Logger(subsystem: "dev.mavio.player", category: "libass").warning("\(message, privacy: .public)")
            }, nil)
        for (name, data) in fonts {
            data.withUnsafeBytes { bytes in
                let base = UnsafeMutableRawPointer(mutating: bytes.baseAddress)?.assumingMemoryBound(to: CChar.self)
                ass_add_font(library, name, base, Int32(bytes.count))
            }
        }
        guard let renderer = ass_renderer_init(library) else {
            ass_library_done(library)
            return nil
        }
        // Core Text finds the fonts the attachments lack.
        ass_set_fonts(renderer, nil, "sans-serif", Int32(ASS_FONTPROVIDER_AUTODETECT.rawValue), nil, 1)
        guard let track = ass_new_track(library) else {
            ass_renderer_done(renderer)
            ass_library_done(library)
            return nil
        }
        var header = Array(header.utf8CString)
        ass_process_codec_private(track, &header, Int32(header.count - 1))
        self.library = library
        self.renderer = renderer
        self.track = track
    }

    deinit {
        ass_free_track(track)
        ass_renderer_done(renderer)
        ass_library_done(library)
    }

    /// Adds an event line (`ReadOrder,Layer,Style,Name,MarginL,MarginR,
    /// MarginV,Effect,Text`) shown from start for duration, in seconds; a
    /// line whose ReadOrder the track has is ignored.
    public func add(event: String, start: TimeInterval, duration: TimeInterval) {
        var line = Array(event.utf8CString)
        ass_process_chunk(track, &line, Int32(line.count - 1), Int64(start * 1000), Int64(duration * 1000))
    }

    /// Adds the event lines of an AetherEngine cue of an ASS track loaded
    /// with `preserveASSMarkup`, one per line of its text.
    public func add(cue: SubtitleCue) {
        guard case let .text(text) = cue.body else { return }
        for line in text.split(separator: "\n") {
            add(event: String(line), start: cue.startTime, duration: cue.endTime - cue.startTime)
        }
    }

    /// Drops every event, as after a seek.
    public func flush() {
        ass_flush_events(track)
    }

    /// The subtitles shown at a time, drawn over a transparent picture of a
    /// size, with whether they differ from the last ones drawn; nil when
    /// nothing is shown.
    public func render(at time: TimeInterval, size: CGSize) -> (image: CGImage?, changed: Bool) {
        let width = Int(size.width), height = Int(size.height)
        guard width > 0, height > 0 else { return (nil, false) }
        if size != frameSize {
            frameSize = size
            ass_set_frame_size(renderer, Int32(width), Int32(height))
            ass_set_storage_size(renderer, Int32(width), Int32(height))
        }
        var change: Int32 = 0
        var image = ass_render_frame(renderer, track, Int64(time * 1000), &change)
        guard image != nil else { return (nil, change != 0) }
        // Premultiplied RGBA, each image's alpha bitmap blended in its color.
        var pixels = [UInt8](repeating: 0, count: width * height * 4)
        while let current = image {
            Self.blend(current.pointee, into: &pixels, width: width, height: height)
            image = current.pointee.next
        }
        let data = CFDataCreate(nil, pixels, pixels.count)!
        let picture = CGImage(
            width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: width * 4,
            space: CGColorSpace(name: CGColorSpace.sRGB)!,
            bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
            provider: CGDataProvider(data: data)!, decode: nil, shouldInterpolate: false, intent: .defaultIntent)
        return (picture, change != 0)
    }

    private static func blend(_ image: ASS_Image, into pixels: inout [UInt8], width: Int, height: Int) {
        // color is 0xRRGGBBAA, AA its transparency.
        let r = Int(image.color >> 24 & 0xFF), g = Int(image.color >> 16 & 0xFF), b = Int(image.color >> 8 & 0xFF)
        let opacity = 255 - Int(image.color & 0xFF)
        guard opacity > 0, let bitmap = image.bitmap else { return }
        for y in 0..<Int(image.h) {
            let row = Int(image.dst_y) + y
            guard row >= 0, row < height else { continue }
            for x in 0..<Int(image.w) {
                let column = Int(image.dst_x) + x
                guard column >= 0, column < width else { continue }
                let a = Int(bitmap[y * Int(image.stride) + x]) * opacity / 255
                guard a > 0 else { continue }
                let i = (row * width + column) * 4
                pixels[i] = UInt8((r * a + Int(pixels[i]) * (255 - a)) / 255)
                pixels[i + 1] = UInt8((g * a + Int(pixels[i + 1]) * (255 - a)) / 255)
                pixels[i + 2] = UInt8((b * a + Int(pixels[i + 2]) * (255 - a)) / 255)
                pixels[i + 3] = UInt8(a + Int(pixels[i + 3]) * (255 - a) / 255)
            }
        }
    }
}

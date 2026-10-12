import AetherEngine
import CoreGraphics
import MavioPlayer
import Testing

/// A script whose one style is opaque red text without outline or shadow.
let header = """
    [Script Info]
    ScriptType: v4.00+
    PlayResX: 640
    PlayResY: 360

    [V4+ Styles]
    Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, \
    Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, \
    MarginR, MarginV, Encoding
    Style: Default,Helvetica,48,&H000000FF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,0,0,2,10,10,20,1

    [Events]
    Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text

    """

/// The opaque pixels of an image and how many of them are red.
func opaque(_ image: CGImage) -> (all: Int, red: Int) {
    let bytes = CFDataGetBytePtr(image.dataProvider!.data!)!
    var all = 0, red = 0
    for i in stride(from: 0, to: image.bytesPerRow * image.height, by: 4) where bytes[i + 3] == 255 {
        all += 1
        if bytes[i] == 255, bytes[i + 1] == 0, bytes[i + 2] == 0 { red += 1 }
    }
    return (all, red)
}

@Test func rendersAnEventInItsStyle() throws {
    let renderer = try #require(ASSRenderer(header: header))
    renderer.add(event: "0,0,Default,,0,0,0,,Mavio {\\b1}subtitles", start: 1, duration: 2)
    let size = CGSize(width: 1280, height: 720)

    #expect(renderer.render(at: 0.5, size: size).image == nil)
    let shown = renderer.render(at: 1.5, size: size)
    let image = try #require(shown.image)
    #expect(shown.changed)
    #expect(image.width == 1280 && image.height == 720)
    let pixels = opaque(image)
    #expect(pixels.all > 1000, "got = \(pixels.all) opaque pixels, want > 1000")
    #expect(pixels.red == pixels.all, "got = \(pixels.red) red of \(pixels.all), want all")
    #expect(!renderer.render(at: 1.6, size: size).changed)
    #expect(renderer.render(at: 3.5, size: size).image == nil)

    renderer.flush()
    #expect(renderer.render(at: 1.5, size: size).image == nil)
}

@Test func rendersTheLinesOfACue() throws {
    let renderer = try #require(ASSRenderer(header: header))
    // Two events of one packet, the second above the first.
    let text = "0,0,Default,,0,0,0,,first\n1,0,Default,,0,0,100,,second"
    renderer.add(cue: SubtitleCue(id: 0, startTime: 1, endTime: 3, body: .text(text)))
    let size = CGSize(width: 640, height: 360)
    let both = opaque(try #require(renderer.render(at: 2, size: size).image)).all

    let one = try #require(ASSRenderer(header: header))
    one.add(event: "0,0,Default,,0,0,0,,first", start: 1, duration: 2)
    let first = opaque(try #require(one.render(at: 2, size: size).image)).all
    #expect(both > first, "got = \(both) opaque pixels, want more than one line's \(first)")
}

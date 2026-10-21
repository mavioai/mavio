import CoreGraphics
import Foundation

/// The assistant orb's states, taken from the voice session's events
/// (libs/design/components.md, Assistant).
public enum OrbMode: Sendable {
    case opening, idle, hearing, thinking, speaking, resting, error, ended
}

/// Draws the assistant's orb (libs/design/components.md, Assistant), as the
/// web UI's canvas does: a sphere of glass with three colored lights flowing
/// inside it. Its size, spread, speed and brightness follow the voice
/// session's state and the audio's level. Drawn each frame, it eases toward
/// its state's look.
public final class OrbRenderer {
    /// The orb's side, in points.
    public static let size: CGFloat = 84
    /// How far the drawing reaches past the orb's box on every side, for its
    /// swell and glow.
    public static let bleed: CGFloat = 1.5
    /// The side of the drawing, in points.
    public static var canvasSize: CGFloat { (size * bleed).rounded() }

    private struct Look {
        var size: Double
        var spread: Double
        var speed: Double
        var bright: Double
        var ring = false
        /// How much the audio's level swells, spreads, speeds and brightens
        /// it.
        var swing = 0.0
    }

    private static func look(_ mode: OrbMode, pausing: Bool) -> Look {
        switch mode {
        case .opening: Look(size: 0.55, spread: 0.12, speed: 0.35, bright: 0.45)
        case .idle: Look(size: 0.78, spread: 0.2, speed: 0.6, bright: 0.85, swing: 0.15)
        case .hearing:
            pausing
                ? Look(size: 0.7, spread: 0.16, speed: 0.5, bright: 0.8)
                : Look(size: 0.82, spread: 0.24, speed: 1.3, bright: 1, swing: 1)
        case .thinking: Look(size: 0.62, spread: 0.4, speed: 3.4, bright: 0.95, ring: true)
        case .speaking: Look(size: 0.8, spread: 0.2, speed: 1.2, bright: 1, swing: 0.55)
        case .resting, .error, .ended: Look(size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85)
        }
    }

    // Magenta, cyan and blue: added up on a dark page, deeper and laid over
    // each other on a light one.
    private static let blobs: [(dark: [Double], light: [Double], sp: Double, ph: Double)] = [
        ([255, 70, 190], [235, 40, 160], 0.9, 0),
        ([40, 220, 255], [0, 170, 235], -1.15, 2.1),
        ([100, 90, 255], [90, 70, 245], 0.7, 4.2),
    ]

    private var size = 0.55
    private var spread = 0.12
    private var speed = 0.35
    private var bright = 0.45
    private var ring = 0.0
    private var swing = 0.0
    private var lv = 0.0
    private var phase = 0.0
    private var last: TimeInterval?
    private var mode = OrbMode.opening
    private var since: TimeInterval = 0
    private var interrupts = 0
    private var flashAt = -Double.infinity
    private var quietSince: TimeInterval = 0
    private var shake = 0.0

    /// An orb opening.
    public init() {}

    /// How far the orb is shaken sideways now, in points, as an error
    /// shakes it once.
    public var offset: CGFloat { CGFloat(shake) }

    /// Moves the orb on to now: mode is the session's state; level the
    /// audio's level, the microphone's or the playback's as the mode wants;
    /// interrupts the count of the session's interrupts, each flashing it.
    /// Reduced motion keeps it still at its state's look.
    public func step(
        now: TimeInterval, mode: OrbMode, level: Double, interrupts: Int, reduceMotion: Bool
    ) {
        if mode != self.mode {
            self.mode = mode
            since = now
        }
        if interrupts != self.interrupts {
            self.interrupts = interrupts
            flashAt = now
        }
        let dt = min(0.05, now - (last ?? now))
        last = now
        let motion = !reduceMotion
        let raw = motion && (mode == .hearing || mode == .speaking || mode == .idle) ? level : 0
        // The user falls silent but the turn goes on: the orb draws in.
        if raw > 0.25 { quietSince = now }
        let look = Self.look(mode, pausing: now - quietSince > 0.4)
        let k = motion ? 1 - exp(-dt * 6) : 1
        size += (look.size - size) * k
        spread += (look.spread - spread) * k
        speed += (look.speed - speed) * k
        bright += (look.bright - bright) * k
        ring += ((look.ring ? 1 : 0) - ring) * k
        swing += (look.swing - swing) * k
        // A syllable swells it at once; it settles back more slowly.
        let target = raw * swing
        lv += (target - lv) * (1 - exp(-dt * (target > lv ? 22 : 8)))
        if motion { phase += dt * (speed + lv * 2.6) }
        let t = now - since
        shake = motion && mode == .error && t < 0.6 ? sin(t * 40) * 6 * (1 - t / 0.6) : 0
    }

    /// Draws the orb into a square context of side pixels, at now, on a
    /// light or dark page.
    public func draw(in g: CGContext, side W: CGFloat, now: TimeInterval, light: Bool, reduceMotion: Bool) {
        let R = W / 2
        let base = (R / Self.bleed) * 0.58
        let flash = reduceMotion ? 0 : max(0, 1 - (now - flashAt) / 0.45)
        let red = mode == .error ? max(0, 1 - (now - since) / 1.4) : 0
        let shrink = 1 - flash * 0.35
        let edge = base * (size + lv * 0.9) * shrink * 0.92
        let space = CGColorSpace(name: CGColorSpace.sRGB)!

        func color(_ c: [Double], _ a: Double) -> CGColor {
            CGColor(
                colorSpace: space, components: [c[0] / 255, c[1] / 255, c[2] / 255, CGFloat(max(0, min(1, a)))])!
        }
        func radial(_ x: CGFloat, _ y: CGFloat, _ r0: CGFloat, _ r1: CGFloat, _ stops: [(CGFloat, CGColor)]) {
            guard
                let grd = CGGradient(
                    colorsSpace: space, colors: stops.map(\.1) as CFArray, locations: stops.map(\.0))
            else { return }
            g.drawRadialGradient(
                grd, startCenter: CGPoint(x: x, y: y), startRadius: r0, endCenter: CGPoint(x: x, y: y),
                endRadius: r1, options: [.drawsAfterEndLocation])
        }

        let brightness = bright + lv * 0.1
        func lights(_ alpha: Double) {
            for (i, b) in Self.blobs.enumerated() {
                let a = phase * b.sp * 2 + b.ph
                // Each light pulses on its own beat with the level, so that
                // the colors surge against each other as the user speaks.
                let pulse = 1 + lv * 0.35 * sin(phase * 5.3 + Double(i) * 2.1)
                let orbit = base * (spread * (1 - ring) + ring * 0.55 + lv * 0.4 * pulse) * shrink
                let x = R + cos(a) * orbit
                let y = R + sin(a * (1 + 0.3 * (1 - ring))) * orbit
                let rad = base * (size + lv * 0.9 * pulse) * shrink
                let tint = [255.0, 70, 80]
                let col = (light ? b.light : b.dark).enumerated().map { j, v in v + (tint[j] - v) * red }
                guard
                    let grd = CGGradient(
                        colorsSpace: space,
                        colors: [
                            color(col, min(1, 0.9 * alpha * brightness)),
                            color(col, (light ? 0.5 : 0.35) * alpha * brightness),
                            color(col, 0),
                        ] as CFArray, locations: [0, 0.55, 1])
                else { continue }
                g.drawRadialGradient(
                    grd, startCenter: CGPoint(x: x, y: y), startRadius: 0, endCenter: CGPoint(x: x, y: y),
                    endRadius: rad, options: [])
            }
        }

        g.saveGState()
        // The drawing's y runs down, as the canvas's.
        g.translateBy(x: 0, y: W)
        g.scaleBy(x: 1, y: -1)
        g.clear(CGRect(x: 0, y: 0, width: W, height: W))
        // The glow around the sphere, on a dark page.
        if !light {
            g.setBlendMode(.plusLighter)
            lights(0.3 + lv * 0.2)
        }
        g.saveGState()
        let clip = CGMutablePath()
        clip.addEllipse(in: CGRect(x: R - edge, y: R - edge, width: edge * 2, height: edge * 2))
        if ring > 0.02 {
            let r = edge * 0.5 * ring
            clip.addEllipse(in: CGRect(x: R - r, y: R - r, width: r * 2, height: r * 2))
        }
        g.addPath(clip)
        g.clip(using: .evenOdd)
        g.setBlendMode(.normal)
        g.setFillColor(light ? color(Self.blobs[2].light, 0.55 * bright) : color([18, 14, 58], 0.85 * bright))
        g.fill(CGRect(x: 0, y: 0, width: W, height: W))
        g.setBlendMode(light ? .normal : .plusLighter)
        lights(1)
        g.setBlendMode(.normal)
        let ink = [10.0, 8, 40]
        let white = [255.0, 255, 255]
        // Depth, darker toward the lower edge; light bounced through the
        // glass along the bottom; a soft reflection at the top left.
        radial(
            R - edge * 0.25, R - edge * 0.3, edge * 0.2, edge * 1.3,
            [(0, color(ink, 0)), (0.7, color(ink, 0)), (1, color(ink, light ? 0.42 : 0.45))])
        radial(
            R + edge * 0.1, R + edge * 0.78, 0, edge * 0.55,
            [(0, color(white, light ? 0.22 : 0.12)), (1, color(white, 0))])
        g.saveGState()
        g.translateBy(x: R - edge * 0.32, y: R - edge * 0.46)
        g.scaleBy(x: 1, y: 0.62)
        radial(
            0, 0, 0, edge * 0.5,
            [
                (0, color(white, light ? 0.9 : 0.55)), (0.35, color(white, light ? 0.38 : 0.18)),
                (1, color(white, 0)),
            ])
        g.restoreGState()
        g.restoreGState()
        if flash > 0 {
            radial(R, R, 0, base * 0.9, [(0, color(white, 0.8 * flash)), (1, color(white, 0))])
        }
        g.restoreGState()
    }
}

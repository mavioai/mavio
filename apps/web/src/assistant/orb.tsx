// The assistant's orb (libs/design/components.md, Assistant): a sphere of
// glass with three colored lights flowing inside it, whose size, spread,
// speed and brightness follow the voice session's state and the level of
// the audio, drawn on a canvas each frame.
import { useEffect, useRef } from "react";

/** The orb's states, from the voice session's events. */
export type OrbMode = "opening" | "idle" | "hearing" | "thinking" | "speaking" | "resting" | "error" | "ended";

interface Look {
  size: number;
  spread: number;
  speed: number;
  bright: number;
  ring?: boolean;
}

const looks: Record<OrbMode | "pausing", Look> = {
  opening: { size: 0.55, spread: 0.12, speed: 0.35, bright: 0.45 },
  idle: { size: 0.78, spread: 0.2, speed: 0.6, bright: 0.85 },
  hearing: { size: 0.8, spread: 0.22, speed: 1.1, bright: 1 },
  pausing: { size: 0.7, spread: 0.16, speed: 0.5, bright: 0.8 },
  thinking: { size: 0.62, spread: 0.4, speed: 3.4, bright: 0.95, ring: true },
  speaking: { size: 0.8, spread: 0.2, speed: 1.2, bright: 1 },
  resting: { size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85 },
  error: { size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85 },
  ended: { size: 0.76, spread: 0.16, speed: 0.18, bright: 0.85 },
};

// Magenta, cyan and blue: added up on a dark page, deeper and laid over
// each other on a light one.
const blobs = [
  { dark: [255, 70, 190], light: [235, 40, 160], sp: 0.9, ph: 0 },
  { dark: [40, 220, 255], light: [0, 170, 235], sp: -1.15, ph: 2.1 },
  { dark: [100, 90, 255], light: [90, 70, 245], sp: 0.7, ph: 4.2 },
];

/** The orb's size on the page, in CSS pixels. */
export const orbSize = 84;

function isLight(): boolean {
  const t = document.documentElement.dataset.theme;
  if (t === "light" || t === "dark") return t === "light";
  return matchMedia("(prefers-color-scheme: light)").matches;
}

/**
 * Draws the orb. level reads the audio's level, the microphone's or the
 * playback's as the mode wants; interrupts counts the session's interrupts,
 * each flashing it.
 */
export function Orb({ mode, level, interrupts }: { mode: OrbMode; level: () => number; interrupts: number }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const props = useRef({ mode, level, interrupts });
  props.current = { mode, level, interrupts };

  useEffect(() => {
    const cv = canvas.current;
    const g = cv?.getContext("2d");
    if (!cv || !g) return;
    const dpr = Math.min(2, devicePixelRatio || 1);
    cv.width = cv.height = Math.round(orbSize * dpr);
    const still = matchMedia("(prefers-reduced-motion: reduce)");
    const cur = { size: 0.55, spread: 0.12, speed: 0.35, bright: 0.45, ring: 0, lv: 0 };
    let phase = 0;
    let last = performance.now();
    let mode = props.current.mode;
    let since = last;
    let interrupts = props.current.interrupts;
    let flashAt = -Infinity;
    let quietSince = last;
    let frame = 0;

    const draw = (now: number) => {
      frame = requestAnimationFrame(draw);
      const p = props.current;
      if (p.mode !== mode) {
        mode = p.mode;
        since = now;
      }
      if (p.interrupts !== interrupts) {
        interrupts = p.interrupts;
        flashAt = now;
      }
      const dt = Math.min(0.05, (now - last) / 1000);
      last = now;
      const t = (now - since) / 1000;
      const motion = !still.matches;
      const raw = motion && (mode === "hearing" || mode === "speaking" || mode === "idle") ? p.level() : 0;
      // The user falls silent but the turn goes on: the orb draws in.
      if (raw > 0.25) quietSince = now;
      const look = looks[mode === "hearing" && now - quietSince > 400 ? "pausing" : mode];
      const k = motion ? 1 - Math.exp(-dt * 6) : 1;
      cur.size += (look.size - cur.size) * k;
      cur.spread += (look.spread - cur.spread) * k;
      cur.speed += (look.speed - cur.speed) * k;
      cur.bright += (look.bright - cur.bright) * k;
      cur.ring += ((look.ring ? 1 : 0) - cur.ring) * k;
      cur.lv += (raw * (mode === "idle" ? 0.15 : 1) - cur.lv) * (1 - Math.exp(-dt * 14));
      if (motion) phase += dt * cur.speed;
      const flash = motion ? Math.max(0, 1 - (now - flashAt) / 450) : 0;

      const W = cv.width;
      const R = W / 2;
      const base = R * 0.58;
      // An error shakes the orb once and flushes it red.
      const shake = motion && mode === "error" && t < 0.6 ? Math.sin(t * 40) * 6 * (1 - t / 0.6) : 0;
      cv.style.transform = shake ? `translateX(${shake}px)` : "";
      const red = mode === "error" ? Math.max(0, 1 - t / 1.4) : 0;
      const light = isLight();
      const shrink = 1 - flash * 0.35;
      const edge = base * (cur.size + cur.lv * 0.45) * shrink * 0.92;

      g.clearRect(0, 0, W, W);
      const lights = (alpha: number) => {
        for (const b of blobs) {
          const a = phase * b.sp * 2 + b.ph;
          const orbit = base * (cur.spread * (1 - cur.ring) + cur.ring * 0.55 + cur.lv * 0.25) * shrink;
          const x = R + Math.cos(a) * orbit;
          const y = R + Math.sin(a * (1 + 0.3 * (1 - cur.ring))) * orbit;
          const rad = base * (cur.size + cur.lv * 0.45) * shrink;
          const col = (light ? b.light : b.dark).map((v, j) => Math.round(v + ([255, 70, 80][j]! - v) * red)).join(",");
          const grd = g.createRadialGradient(x, y, 0, x, y, rad);
          grd.addColorStop(0, `rgba(${col},${0.9 * alpha * cur.bright})`);
          grd.addColorStop(0.55, `rgba(${col},${(light ? 0.5 : 0.35) * alpha * cur.bright})`);
          grd.addColorStop(1, `rgba(${col},0)`);
          g.fillStyle = grd;
          g.fillRect(0, 0, W, W);
        }
      };
      const at = (x: number, y: number, r0: number, r1: number, stops: [number, string][]) => {
        const grd = g.createRadialGradient(x, y, r0, x, y, r1);
        for (const [o, c] of stops) grd.addColorStop(o, c);
        g.fillStyle = grd;
        g.fillRect(-2 * W, -2 * W, 4 * W, 4 * W);
      };
      // The glow around the sphere, on a dark page.
      if (!light) {
        g.globalCompositeOperation = "lighter";
        lights(0.3);
      }
      g.save();
      g.beginPath();
      g.arc(R, R, edge, 0, Math.PI * 2);
      if (cur.ring > 0.02) g.arc(R, R, edge * 0.5 * cur.ring, 0, Math.PI * 2, true);
      g.clip("evenodd");
      g.globalCompositeOperation = "source-over";
      g.fillStyle = light ? `rgba(${blobs[2]!.light.join(",")},${0.55 * cur.bright})` : `rgba(18,14,58,${0.85 * cur.bright})`;
      g.fillRect(0, 0, W, W);
      g.globalCompositeOperation = light ? "source-over" : "lighter";
      lights(1);
      g.globalCompositeOperation = "source-over";
      // Depth, darker toward the lower edge; light bounced through the
      // glass along the bottom; a soft reflection at the top left.
      at(R - edge * 0.25, R - edge * 0.3, edge * 0.2, edge * 1.3, [
        [0, "rgba(10,8,40,0)"],
        [0.7, "rgba(10,8,40,0)"],
        [1, `rgba(10,8,40,${light ? 0.42 : 0.45})`],
      ]);
      at(R + edge * 0.1, R + edge * 0.78, 0, edge * 0.55, [
        [0, `rgba(255,255,255,${light ? 0.22 : 0.12})`],
        [1, "rgba(255,255,255,0)"],
      ]);
      g.save();
      g.translate(R - edge * 0.32, R - edge * 0.46);
      g.scale(1, 0.62);
      at(0, 0, 0, edge * 0.5, [
        [0, `rgba(255,255,255,${light ? 0.9 : 0.55})`],
        [0.35, `rgba(255,255,255,${light ? 0.38 : 0.18})`],
        [1, "rgba(255,255,255,0)"],
      ]);
      g.restore();
      g.restore();
      if (flash > 0) {
        at(R, R, 0, base * 0.9, [
          [0, `rgba(255,255,255,${0.8 * flash})`],
          [1, "rgba(255,255,255,0)"],
        ]);
      }
    };
    frame = requestAnimationFrame(draw);
    return () => cancelAnimationFrame(frame);
  }, []);

  return (
    <canvas
      ref={canvas}
      aria-hidden
      style={{ width: orbSize, height: orbSize }}
      className="block [filter:var(--assistant-orb-shadow)]"
    />
  );
}

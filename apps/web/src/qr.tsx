// QR codes, for a phone to scan: rclone's drawn in its help, and those the
// web UI encodes itself with uqr, as the link page beside a television's
// link code and a new server's address on its television setup.
import { useMemo } from "react";
import { encode } from "uqr";

/** A QR code's modules, row by row, true where dark. */
export function qrModules(text: string): boolean[][] {
  return encode(text, { ecc: "M", border: 0 }).data;
}

/**
 * A QR code, drawn dark on light whatever the theme, for a phone to scan;
 * a quiet zone of a module is drawn around it when `quiet` is set.
 */
export function QRCode({ dark, label = "QR", className = "", quiet = false }: { dark: boolean[][]; label?: string; className?: string; quiet?: boolean }) {
  const width = Math.max(...dark.map((r) => r.length));
  const m = quiet ? 2 : 0;
  const d = dark.flatMap((row, y) => row.flatMap((on, x) => (on ? [`M${x + m} ${y + m}h1v1h-1z`] : []))).join("");
  return (
    <svg viewBox={`0 0 ${width + 2 * m} ${dark.length + 2 * m}`} role="img" aria-label={label} className={className} shapeRendering="crispEdges">
      <rect width={width + 2 * m} height={dark.length + 2 * m} fill="#fff" />
      <path d={d} fill="#000" />
    </svg>
  );
}

/** The QR code of text, as QRCode draws it. */
export function QRCodeOf({ text, className, quiet }: { text: string; className?: string; quiet?: boolean }) {
  const dark = useMemo(() => qrModules(text), [text]);
  return <QRCode dark={dark} label={text} className={className} quiet={quiet} />;
}

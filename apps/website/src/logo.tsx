// The logo of libs/design/brand, as the web UI draws it: the wordmark in
// its gradient, inline so that it needs no request and scales with its box.
import { useId } from "react";

const wordmark =
  "M0 64V0H14L30 30L46 0H60V64H48V24L34 50H26L12 24V64ZM80 20H114V64H80L70 54V30ZM86 32L82 36V48L86 52H102V32ZM124 20H137L146 44.7L155 20H168L152 64H140ZM178 20H190V64H178ZM178 2H190V14H178ZM210 20H234L244 30V54L234 64H210L200 54V30ZM216 32L212 36V48L216 52H228L232 48V36L228 32Z";

/** The Mavio wordmark, as tall as height. */
export function Wordmark({ height = 22, className }: { height?: number; className?: string }) {
  const id = `wordmark${useId().replace(/[^\w-]/g, "")}`;
  return (
    <svg viewBox="0 0 244 64" height={height} width={(height * 244) / 64} className={className} role="img" aria-label="Mavio">
      <defs>
        <linearGradient id={id} gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="244" y2="64">
          <stop offset="0" stopColor="#FFD34E" />
          <stop offset="0.45" stopColor="#FF9A2E" />
          <stop offset="1" stopColor="#F0412F" />
        </linearGradient>
      </defs>
      <path d={wordmark} fill={`url(#${id})`} fillRule="evenodd" />
    </svg>
  );
}

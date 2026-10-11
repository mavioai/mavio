import { type IconName, iconViewBox, icons } from "@mavio/design/icons";

export type { IconName };

/**
 * An icon of libs/design: Regular when inactive, Fill when active
 * (docs/clients.md §6.2). Decorative unless labeled.
 */
export function Icon({
  name,
  fill = false,
  size = 24,
  label,
  className,
}: {
  name: IconName;
  fill?: boolean;
  size?: number;
  label?: string;
  className?: string;
}) {
  const paths = icons[name][fill ? "fill" : "regular"];
  return (
    <svg
      viewBox={`0 0 ${iconViewBox} ${iconViewBox}`}
      width={size}
      height={size}
      fill="currentColor"
      className={className}
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      {paths.map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}

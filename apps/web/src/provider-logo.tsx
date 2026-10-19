import { providerLogo } from "@mavio/design/providers";

import { Icon } from "./icon.tsx";

/**
 * The logo of an AI provider, from libs/design/providers: in its own colors,
 * or the text's for a monochrome one; a chip's icon for a provider without.
 * Decorative, beside the provider's name.
 */
export function ProviderLogo({ provider, size = 16, className }: { provider: string; size?: number; className?: string }) {
  const logo = providerLogo(provider);
  if (!logo) return <Icon name="cpu" size={size} className={`shrink-0 text-text-tertiary ${className ?? ""}`} />;
  return (
    <span
      aria-hidden
      className={`inline-flex shrink-0 [&>svg]:size-full ${className ?? ""}`}
      style={{ width: size, height: size }}
      // The SVGs are libs/design's own, generated from its sources.
      dangerouslySetInnerHTML={{ __html: logo.svg }}
    />
  );
}

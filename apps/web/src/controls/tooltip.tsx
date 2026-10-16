// Tooltips (docs/clients.md §6.4): Base UI's tooltip naming a control
// whose icon alone shows what it does, on hover and keyboard focus after
// a moment; none on the television, where focus shows the control.
import { Tooltip } from "@base-ui/react/tooltip";
import type { HTMLAttributes, ReactElement, ReactNode, Ref } from "react";

import { usePrefs } from "../prefs.ts";

/** The tooltips' shared timing, once one shows the next shows at once. */
export function TooltipProvider({ children }: { children: ReactNode }) {
  return <Tooltip.Provider delay={600}>{children}</Tooltip.Provider>;
}

/**
 * A tooltip of label over children, a control; other props go on to the
 * control, so that a Tip can itself be a menu's or a dialog's trigger.
 */
export function Tip({
  label,
  side = "top",
  disabled = false,
  children,
  ...props
}: { label: ReactNode; side?: "top" | "bottom" | "left" | "right"; disabled?: boolean; children: ReactElement } & HTMLAttributes<HTMLButtonElement> & { ref?: Ref<HTMLButtonElement> }) {
  const { tv } = usePrefs();
  return (
    <Tooltip.Root disabled={tv || disabled}>
      <Tooltip.Trigger {...props} render={children} />
      <Tooltip.Portal>
        <Tooltip.Positioner side={side} sideOffset={6} className="z-[70]">
          <Tooltip.Popup className="max-w-xs origin-[var(--transform-origin)] rounded-md bg-on-background px-2 py-1 type-footnote text-background shadow-level2 transition-[opacity,scale] duration-100 data-ending-style:scale-95 data-ending-style:opacity-0 data-instant:transition-none data-starting-style:scale-95 data-starting-style:opacity-0">
            {label}
          </Tooltip.Popup>
        </Tooltip.Positioner>
      </Tooltip.Portal>
    </Tooltip.Root>
  );
}

// The classes the controls share (docs/clients.md §6.4): popups of
// glass over the page, their items highlighted as they are hovered or
// reached with the keys, and the fields text is typed into.

/** A popup's shape and motion, without its fill. */
const popupShape =
  "origin-[var(--transform-origin)] rounded-[14px] border border-border p-1.5 text-on-background shadow-level3 outline-none transition-[opacity,scale] duration-100 data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0";

/** A popup over the page: a menu, a select's options, a field's suggestions. */
export const popup = `${popupShape} bg-surface-overlay/90 backdrop-blur-xl`;

/** A popup of frosted glass, letting more of what is under it through: on the television, and over a video. */
export const glassPopup = `${popupShape} bg-surface-overlay/45 backdrop-blur-3xl backdrop-saturate-150`;

/** An item of a popup, highlighted by the pointer and the keys alike. */
export const popupItem =
  "flex min-h-10 w-full cursor-default items-center gap-2.5 rounded-lg px-2.5 text-start type-body desk:min-h-7 desk:gap-2 desk:rounded-md desk:px-2 outline-none select-none data-disabled:opacity-50 data-highlighted:bg-fill";

/** A dialog's backdrop, dimming and blurring the page. */
export const backdrop =
  "fixed inset-0 z-50 bg-black/60 backdrop-blur-sm transition-opacity duration-150 data-ending-style:opacity-0 data-starting-style:opacity-0";

/** A dialog's frame, centered over the backdrop. */
export const dialogFrame =
  "fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] -translate-x-1/2 -translate-y-1/2 flex-col rounded-[18px] border border-border bg-surface-overlay text-on-background shadow-level3 outline-none transition-[opacity,scale] duration-150 data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0";

/** A text field's style in a dialog. */
export const field =
  "min-h-11 min-w-0 flex-1 rounded-[10px] border border-border bg-surface px-3 type-body desk:min-h-8 desk:rounded-md desk:px-2.5 placeholder:text-text-tertiary focus:border-focus focus:outline-none";

/** A text field's style inside a settings row. */
export const input =
  "min-h-9 w-full min-w-0 rounded-md border border-border bg-surface px-3 type-body desk:min-h-7 desk:px-2 text-on-background placeholder:text-text-tertiary focus:border-focus focus:outline-none";

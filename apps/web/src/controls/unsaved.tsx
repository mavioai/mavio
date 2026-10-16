// Leaving a form whose changes are not saved asks first: going to another
// page, Back, and closing or reloading the browser's tab.
import { useBlocker } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef } from "react";

import { t } from "../i18n.ts";
import { Confirm } from "./dialog.tsx";

/**
 * Guards a form while changed: dialog is the alert to render, and leave
 * lets the next navigation through, as the one following a save.
 */
export function useUnsavedGuard(changed: boolean): { dialog: ReactNode; leave: () => void } {
  const state = useRef({ changed, allowed: false });
  useEffect(() => {
    state.current.changed = changed;
  });
  const blocker = useBlocker({
    shouldBlockFn: () => {
      if (state.current.allowed) {
        state.current.allowed = false;
        return false;
      }
      return state.current.changed;
    },
    enableBeforeUnload: () => state.current.changed,
    withResolver: true,
  });
  const dialog =
    blocker.status === "blocked" ? (
      <Confirm
        title={t("unsaved.title")}
        message={t("unsaved.message")}
        action={t("unsaved.discard")}
        busy={false}
        onCancel={blocker.reset}
        onConfirm={blocker.proceed}
      />
    ) : null;
  return {
    dialog,
    leave: () => {
      state.current.allowed = true;
    },
  };
}

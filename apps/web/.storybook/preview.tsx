// The stories drawn as the web UI draws its screens: its styles, a theme
// chosen in the toolbar and the form factors of the snapshot tests as
// viewports, the TV's in TV mode (src/prefs.ts sets data-theme and data-tv);
// inside the router, query client and servers of ./app.tsx.
import screens from "@mavio/design/snapshots/screens.json" with { type: "json" };
import type { Preview } from "@storybook/react-vite";
import { type ReactNode, useLayoutEffect } from "react";

import { TooltipProvider } from "../src/controls/tooltip.tsx";
import { type Appearance, setPrefs, usePrefs } from "../src/prefs.ts";
import "../src/styles.css";
import { withApp } from "./app.tsx";

const viewports = Object.fromEntries(
  Object.entries(screens.sizes).map(([name, { width, height }]) => [
    name,
    {
      name: `${name[0].toUpperCase()}${name.slice(1)}`,
      styles: { width: `${width}px`, height: `${height}px` },
      type: name === "phone" ? "mobile" : name === "tablet" ? "tablet" : "desktop",
    },
  ]),
);

/** Sets the theme and TV mode as the settings do, drawing the story once they hold. */
function Prefs({ theme, tv, children }: { theme: Appearance; tv: boolean; children: ReactNode }) {
  const current = usePrefs();
  useLayoutEffect(() => setPrefs({ appearance: theme, tv }), [theme, tv]);
  return current.appearance === theme && current.tv === tv ? children : null;
}

const preview: Preview = {
  globalTypes: {
    theme: {
      description: "The theme the components are drawn in",
      toolbar: {
        title: "Theme",
        icon: "mirror",
        items: [
          { value: "dark", title: "Dark" },
          { value: "light", title: "Light" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { theme: "dark" },
  parameters: {
    viewport: { options: viewports },
    // The page's background is the theme's, as the web UI's body is.
    backgrounds: { disable: true },
    controls: { expanded: true },
  },
  // The router, query client and servers, inside the theme and TV mode.
  decorators: [
    withApp,
    (Story, { globals }) => {
      const viewport: unknown = globals.viewport;
      const size = typeof viewport === "string" ? viewport : (viewport as { value?: string } | undefined)?.value;
      return (
        <Prefs theme={globals.theme === "light" ? "light" : "dark"} tv={size === "tv"}>
          <TooltipProvider>
            <Story />
          </TooltipProvider>
        </Prefs>
      );
    },
  ],
};

export default preview;

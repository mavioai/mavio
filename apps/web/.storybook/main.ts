// Storybook of the web UI's components (docs/clients.md §6.8), built with
// the UI's own Vite configuration: Tailwind, the design tokens and Inter.
import type { StorybookConfig } from "@storybook/react-vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.tsx"],
  framework: "@storybook/react-vite",
  // Generated artwork for the cards, served beside the stories.
  staticDirs: [{ from: "./art", to: "/art" }],
  core: { disableTelemetry: true, disableWhatsNewNotifications: true },
  // The stories bundle the UI's components with the API's descriptors.
  viteFinal: (config) => ({ ...config, build: { ...config.build, chunkSizeWarningLimit: 2048 } }),
};

export default config;

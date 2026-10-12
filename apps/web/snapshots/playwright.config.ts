// The web UI's snapshot tests (docs/clients.md §6.8). Chromium runs in the
// Playwright image of the version below, whose browser server run.sh
// starts, so that every machine draws the same pixels; the tests drive it
// from the host.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  snapshotPathTemplate: "{testDir}/references/{arg}{ext}",
  outputDir: "../../../dist/snapshots/web/results",
  reporter: [["list"]],
  fullyParallel: true,
  expect: { toHaveScreenshot: { animations: "disabled", caret: "hide", scale: "device", maxDiffPixelRatio: 0.002 } },
  use: {
    browserName: "chromium",
    colorScheme: "dark",
    connectOptions: { wsEndpoint: process.env.MAVIO_BROWSER ?? "ws://127.0.0.1:3790/" },
  },
});

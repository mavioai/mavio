// The screens of libs/design/snapshots/screens.json, drawn by the web UI
// at the sizes of a phone, a tablet and a desktop window, compared with
// the references in references/ (docs/clients.md §6.8).
import { expect, type Page, test } from "@playwright/test";

import { fixtures, screens, services, signedIn } from "./fixtures.ts";
import { origin, serve } from "./server.ts";

const sizes = ["phone", "tablet", "desktop"] as const;

/** Opens path with the servers of signed-in kept, as the web UI keeps them. */
async function open(page: Page, path: string, servers: unknown[]) {
  await page.clock.setFixedTime(new Date(screens.now));
  await page.addInitScript((list) => {
    localStorage.setItem("mavio.servers", JSON.stringify(list));
    localStorage.setItem("mavio.device", "00000000-0000-4000-8000-000000000000");
  }, servers);
  await serve(page, services, fixtures());
  await page.goto(`${origin}${path}`);
  await page.evaluate(() => document.fonts.ready);
}

for (const size of sizes) {
  test.describe(size, () => {
    const { width, height, scale } = screens.sizes[size];
    test.use({ viewport: { width, height }, deviceScaleFactor: scale, locale: "en-US", timezoneId: "UTC" });

    test("sign-in", async ({ page }) => {
      await open(page, "/sign-in", []);
      await expect(page.getByRole("textbox").first()).toBeVisible();
      await expect(page).toHaveScreenshot(`sign-in-${size}.png`);
    });

    test("home", async ({ page }) => {
      await open(page, "/", signedIn);
      await expect(page.getByText(screens.items.lantern.name).first()).toBeVisible();
      await page.waitForFunction(() => [...document.images].every((i) => i.complete));
      await expect(page).toHaveScreenshot(`home-${size}.png`);
    });
  });
}

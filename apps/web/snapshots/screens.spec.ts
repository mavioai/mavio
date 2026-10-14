// The screens of libs/design/snapshots/screens.json, drawn by the web UI
// at the sizes of a phone, a tablet and a desktop window, and in TV mode,
// compared with the references in references/ (docs/clients.md §6.8).
import { expect, type Page, test } from "@playwright/test";

import { fixtures, screens, services, signedIn } from "./fixtures.ts";
import { origin, serve } from "./server.ts";

const sizes = ["phone", "tablet", "desktop"] as const;

const server = screens.server.id;

/** Opens path with the servers of signed-in kept, as the web UI keeps them, and these settings. */
async function open(page: Page, path: string, servers: unknown[], prefs?: object) {
  await page.clock.setFixedTime(new Date(screens.now));
  await page.addInitScript(
    ([list, p]) => {
      localStorage.setItem("mavio.servers", JSON.stringify(list));
      localStorage.setItem("mavio.device", "00000000-0000-4000-8000-000000000000");
      if (p) localStorage.setItem("mavio.prefs", JSON.stringify(p));
    },
    [servers, prefs] as const,
  );
  await serve(page, services, fixtures());
  await page.goto(`${origin}${path}`);
  await page.evaluate(() => document.fonts.ready);
}

/** Waits for the images on the page to load. */
async function images(page: Page) {
  await page.waitForFunction(() => [...document.images].every((i) => i.complete));
}

for (const size of sizes) {
  test.describe(size, () => {
    const { width, height, scale } = screens.sizes[size];
    test.use({ viewport: { width, height }, deviceScaleFactor: scale, locale: "en-US", timezoneId: "UTC" });

    test("sign-in", async ({ page }) => {
      await open(page, "/sign-in", []);
      await expect(page.getByRole("button", { name: "Connect to a Server" })).toBeVisible();
      await expect(page).toHaveScreenshot(`sign-in-${size}.png`);
    });

    test("home", async ({ page }) => {
      await open(page, "/", signedIn);
      await expect(page.getByText(screens.items.lantern.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`home-${size}.png`);
    });

    test("library", async ({ page }) => {
      await open(page, `/s/${server}/library/lib-movies?tab=all`, signedIn);
      await expect(page.getByText(screens.items.summit.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`library-${size}.png`);
    });

    test("details", async ({ page }) => {
      await open(page, `/s/${server}/item/orbit`, signedIn);
      await expect(page.getByRole("heading", { name: screens.items.orbit.name })).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`details-${size}.png`);
    });

    test("settings", async ({ page }) => {
      await open(page, size === "phone" ? "/settings/playback" : "/settings", signedIn);
      await expect(page.getByText(size === "phone" ? "Maximum Quality" : "Appearance").first()).toBeVisible();
      await expect(page).toHaveScreenshot(`settings-${size}.png`);
    });
  });
}

test.describe("tv", () => {
  const { width, height, scale } = screens.sizes.tv;
  test.use({ viewport: { width, height }, deviceScaleFactor: scale, locale: "en-US", timezoneId: "UTC" });

  test("sign-in", async ({ page }) => {
    await open(page, "/sign-in", [], { tv: true });
    await expect(page.getByRole("tab").first()).toBeVisible();
    await expect(page).toHaveScreenshot("sign-in-tv.png");
  });

  test("home", async ({ page }) => {
    await open(page, "/", signedIn, { tv: true });
    await expect(page.getByText(screens.items.lantern.name).first()).toBeVisible();
    await images(page);
    await expect(page).toHaveScreenshot("home-tv.png");
  });
});

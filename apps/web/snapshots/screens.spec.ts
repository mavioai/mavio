// The screens of libs/design/snapshots/screens.json, drawn by the web UI
// at the sizes of a phone, a tablet and a desktop window, and in TV mode,
// compared with the references in references/ (docs/clients.md §6.8).
import { expect, type Page, test } from "@playwright/test";

import { fixtures, screens, services, signedIn } from "./fixtures.ts";
import { origin, serve } from "./server.ts";

const sizes = ["phone", "tablet", "desktop"] as const;

const server = screens.server.id;
const discover = screens.discover;

/** The page of a discovered title. */
const titlePath = (id: string) => `/s/${server}/title/${discover.titles[id].kind}/${discover.titles[id].tmdb}`;

/** The popular movies of Discover, filtered by the first genre, as the page's address keeps them. */
const catalogPath = `/s/${server}/discover/${encodeURIComponent(discover.catalogs[1].id)}?kind=movie&f=${encodeURIComponent(
  JSON.stringify({ genres: [discover.genres[0].id] }),
)}`;

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

/**
 * Has the player's video, which the fake server does not serve, stand
 * paused at the movie's position with its runtime, as one that loaded.
 */
async function paused(page: Page) {
  await page.waitForFunction(() => document.querySelector("video")?.error);
  await page.evaluate(
    ([position, runtime]) => {
      const v = document.querySelector("video")!;
      Object.defineProperty(v, "currentTime", { get: () => position, set: () => {} });
      Object.defineProperty(v, "duration", { get: () => runtime });
      for (const e of ["durationchange", "timeupdate", "canplay"]) v.dispatchEvent(new Event(e));
    },
    [(screens.home[0].items[0].progressMinutes ?? 0) * 60, (screens.items.orbit.runtimeMinutes ?? 0) * 60],
  );
}

/** Opens the Watch Together dialog from a movie's More menu and joins the server's group. */
async function together(page: Page) {
  await page.getByRole("button", { name: "More" }).first().click();
  await page.getByRole("menuitem", { name: "Watch Together" }).click();
  await page.getByRole("button", { name: "Join" }).click();
  await expect(page.getByRole("region", { name: "Queue" })).toBeVisible();
  await page.mouse.move(0, 0);
}

for (const size of sizes) {
  test.describe(size, () => {
    const { width, height, scale } = screens.sizes[size];
    test.use({ viewport: { width, height }, deviceScaleFactor: scale, locale: "en-US", timezoneId: "UTC" });

    test("sign-in", async ({ page }) => {
      await open(page, "/sign-in", []);
      await expect(page.getByLabel("Password")).toBeVisible();
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

    test("details-series", async ({ page }) => {
      await open(page, `/s/${server}/item/atlas`, signedIn);
      await expect(page.getByRole("heading", { name: screens.items.atlas.name })).toBeVisible();
      await expect(page.getByText(screens.items["atlas-s2e3"].name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`details-series-${size}.png`);
    });

    test("player", async ({ page }) => {
      await open(page, `/s/${server}/play/orbit`, signedIn);
      await paused(page);
      await expect(page.getByText(screens.items.orbit.name).first()).toBeVisible();
      await expect(page).toHaveScreenshot(`player-${size}.png`);
    });

    test("search", async ({ page }) => {
      await open(page, `/search?q=${screens.search.query}`, signedIn);
      await expect(page.getByText(screens.items.fernwood.name).first()).toBeVisible();
      await expect(page.getByText(discover.titles.paper.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`search-${size}.png`);
    });

    test("settings", async ({ page }) => {
      await open(page, size === "phone" ? "/settings/playback" : "/settings", signedIn);
      await expect(page.getByText(size === "phone" ? "Maximum Quality" : "Appearance").first()).toBeVisible();
      await expect(page).toHaveScreenshot(`settings-${size}.png`);
    });

    test("plugins-available", async ({ page }) => {
      await open(page, "/settings/plugins", signedIn);
      await expect(page.getByText(screens.plugins.catalog[2].name).first()).toBeVisible();
      await expect(page).toHaveScreenshot(`plugins-available-${size}.png`);
    });

    test("plugins-installed", async ({ page }) => {
      await open(page, "/settings/plugins?tab=installed", signedIn);
      await expect(page.getByText(screens.plugins.installed[3].name ?? "").first()).toBeVisible();
      await expect(page).toHaveScreenshot(`plugins-installed-${size}.png`);
    });

    test("discover", async ({ page }) => {
      await open(page, "/discover", signedIn);
      await expect(page.getByText(discover.titles.comet.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`discover-${size}.png`);
    });

    test("discover-catalog", async ({ page }) => {
      await open(page, catalogPath, signedIn);
      await expect(page.getByText(discover.titles.glacier.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`discover-catalog-${size}.png`);
    });

    test("discover-filters", async ({ page }) => {
      await open(page, catalogPath, signedIn);
      await page.getByRole("button", { name: /^Filters/ }).click();
      await expect(page.getByRole("dialog").getByText(discover.genres[1].name)).toBeVisible();
      await expect(page).toHaveScreenshot(`discover-filters-${size}.png`);
    });

    test("discover-title", async ({ page }) => {
      await open(page, titlePath("comet"), signedIn);
      await expect(page.getByRole("heading", { name: discover.titles.comet.name })).toBeVisible();
      await expect(page.getByText(discover.titles.paper.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`discover-title-${size}.png`);
    });

    test("requests", async ({ page }) => {
      await open(page, `/s/${server}/requests`, signedIn);
      await expect(page.getByText(discover.titles.drift.name).first()).toBeVisible();
      await images(page);
      await expect(page).toHaveScreenshot(`requests-${size}.png`);
    });

    test("watch-together", async ({ page }) => {
      await open(page, `/s/${server}/item/orbit`, signedIn);
      await together(page);
      await images(page);
      await expect(page).toHaveScreenshot(`watch-together-${size}.png`);
    });
  });
}

test.describe("tv", () => {
  const { width, height, scale } = screens.sizes.tv;
  test.use({ viewport: { width, height }, deviceScaleFactor: scale, locale: "en-US", timezoneId: "UTC" });

  test("sign-in", async ({ page }) => {
    await open(page, "/sign-in", [], { tv: true });
    await expect(page.getByLabel("Password")).toBeVisible();
    await expect(page).toHaveScreenshot("sign-in-tv.png");
  });

  test("home", async ({ page }) => {
    await open(page, "/", signedIn, { tv: true });
    await expect(page.getByText(screens.items.lantern.name).first()).toBeVisible();
    await images(page);
    await expect(page).toHaveScreenshot("home-tv.png");
  });

  test("details-series", async ({ page }) => {
    await open(page, `/s/${server}/item/atlas`, signedIn, { tv: true });
    await expect(page.getByRole("heading", { name: screens.items.atlas.name })).toBeVisible();
    await images(page);
    await expect(page).toHaveScreenshot("details-series-tv.png");
  });

  test("player", async ({ page }) => {
    await open(page, `/s/${server}/play/orbit`, signedIn, { tv: true });
    await paused(page);
    await expect(page.getByText(screens.items.orbit.name).first()).toBeVisible();
    await expect(page).toHaveScreenshot("player-tv.png");
  });

  test("search-popular", async ({ page }) => {
    await open(page, "/search", signedIn, { tv: true });
    await expect(page.getByText(screens.popularSearches[0])).toBeVisible();
    await expect(page).toHaveScreenshot("search-popular-tv.png");
  });

  test("discover", async ({ page }) => {
    await open(page, "/discover", signedIn, { tv: true });
    await expect(page.getByText(discover.titles.comet.name).first()).toBeVisible();
    await images(page);
    await expect(page).toHaveScreenshot("discover-tv.png");
  });

  test("discover-title", async ({ page }) => {
    await open(page, titlePath("comet"), signedIn, { tv: true });
    await expect(page.getByRole("heading", { name: discover.titles.comet.name })).toBeVisible();
    await images(page);
    await expect(page).toHaveScreenshot("discover-title-tv.png");
  });
});

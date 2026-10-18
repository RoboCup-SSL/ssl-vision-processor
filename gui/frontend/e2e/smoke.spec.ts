// Smoke tests against the real host: every page renders without a script
// error, and the main paths through the API (edit, reject, unlock) work from
// the UI. The host runs on testdata/vision.yml: two cameras, camera 0 with a
// locked calibration.
import { readFileSync, writeFileSync } from "node:fs";
import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";

// Every page, as #<camera>/<page>. advanced is shown only to expert users.
const PAGES = [
  "0/overview",
  "0/network",
  "0/field",
  "0/layout",
  "0/advanced",
  "0/alerts",
  "0/camera",
  "0/geometry",
  "0/color",
  "0/stream",
  "0/debug",
  "1/camera",
  "1/geometry",
  "1/color",
];

// Script errors on page, collected for the test to assert there are none.
function pageErrors(page: Page): string[] {
  const errors: string[] = [];

  page.on("pageerror", (err) => errors.push(err.message));

  return errors;
}

test.beforeEach(async ({ page, request }) => {
  // Back to the file on disk, so each test starts from the fixture. Nothing
  // here saves, so the file itself never changes.
  expect((await request.post("/api/config/reload")).ok()).toBe(true);

  await page.addInitScript(() => {
    localStorage.setItem(
      "vision-processor-gui:preferences",
      JSON.stringify({ tooltipsEnabled: true, expertUser: true }),
    );
  });
});

test("loads with the host's alerts", async ({ page }) => {
  const errors = pageErrors(page);

  await page.goto("/#0/field");

  await expect(page.getByText("User Interface - OPEN")).toBeVisible();
  // The fixture's locked calibration predates its field: a field_changed
  // warning, counted from the host's alerts.state.
  await expect(
    page.getByRole("link", { name: /^Warnings [1-9]/ }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test("every page renders", async ({ page }) => {
  const errors = pageErrors(page);

  await page.goto("/#0/overview");

  for (const hash of PAGES) {
    // Same-document navigation: only the hash changes, as when clicking
    // through the sidebar.
    await page.goto(`/#${hash}`);
    await expect(page).toHaveURL(new RegExp(`#${hash}$`));
    await expect(
      page.locator("main").getByRole("heading").first(),
    ).toBeVisible();
  }

  expect(errors).toEqual([]);
});

test("an edit applies live and shows as unsaved", async ({ page }) => {
  await page.goto("/#0/field");

  const put = page.waitForResponse(
    (r) => r.url().endsWith("/api/config") && r.request().method() === "PUT",
  );
  await page.getByRole("spinbutton", { name: "Field width" }).fill("6100");
  expect((await put).status()).toBe(200);

  await expect(
    page.getByRole("link", { name: "Field Dimensions*" }).first(),
  ).toBeVisible();

  const geometry = (await (await page.request.get("/api/geometry")).json()) as {
    geometry: { field: { fieldWidth: number } };
  };
  expect(geometry.geometry.field.fieldWidth).toBe(6100);
});

test("a rejected edit shows the host's reason", async ({ page }) => {
  await page.goto("/#0/field");

  await page.getByRole("spinbutton", { name: "Field width" }).fill("0");

  await expect(page.getByRole("alert")).toContainText(
    "field_width: must be greater than 0",
  );
});

test("unlocking a calibration goes through the API", async ({ page }) => {
  await page.goto("/#0/geometry");

  await page
    .getByRole("button", { name: "Unlock (recalibrates on processor restart)" })
    .click();

  const unlock = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/config/cameras/0/calibration") &&
      r.request().method() === "DELETE",
  );
  await page.getByRole("button", { name: "Unlock", exact: true }).click();
  expect((await unlock).status()).toBe(200);

  const state = (await (await page.request.get("/api/config")).json()) as {
    document: { cameras: { calibration?: unknown }[] };
  };
  expect(state.document.cameras[0]?.calibration).toBeUndefined();
});

test("leaving a page right after an edit", async ({ page }) => {
  // Known bug: flowbite-svelte 1.33.1's Input calls its onblur prop through
  // state Svelte has already torn down when a focused input unmounts
  // (derived_inert, then "L(...) is not a function"). The edit itself is
  // saved first. Remove test.fail once it's fixed, and this test guards it.
  test.fail();

  const errors = pageErrors(page);

  await page.goto("/#0/field");

  const put = page.waitForResponse(
    (r) => r.url().endsWith("/api/config") && r.request().method() === "PUT",
  );
  await page.getByRole("spinbutton", { name: "Field width" }).fill("6100");
  await put;

  await page.goto("/#0/overview");
  await expect(page.locator("main").getByRole("heading").first()).toBeVisible();

  expect(errors).toEqual([]);
});

// The config file the host edits, and its contents, restored after fn so
// later tests start from the fixture again.
async function withConfigFile(
  request: APIRequestContext,
  fn: (path: string, original: string) => Promise<void>,
): Promise<void> {
  const state = (await (await request.get("/api/config")).json()) as {
    state: { path: string };
  };
  const path = state.state.path;
  const original = readFileSync(path, "utf8");

  try {
    await fn(path, original);
  } finally {
    writeFileSync(path, original);
    await request.post("/api/config/reload");
  }
}

test("an edit from a stale page is refused, not clobbering the other", async ({
  browser,
}) => {
  const context = await browser.newContext();
  const fresh = await context.newPage();
  const stale = await context.newPage();

  // The stale page never hears the host's updates, so it edits from the
  // revision it loaded at.
  await stale.routeWebSocket("**/ws", () => {
    // Accepted, never connected to the host.
  });

  await fresh.goto("/#0/field");
  await stale.goto("/#0/field");

  const freshPut = fresh.waitForResponse(
    (r) => r.url().endsWith("/api/config") && r.request().method() === "PUT",
  );
  await fresh.getByRole("spinbutton", { name: "Field width" }).fill("6100");
  expect((await freshPut).status()).toBe(200);

  const stalePut = stale.waitForResponse(
    (r) => r.url().endsWith("/api/config") && r.request().method() === "PUT",
  );
  await stale.getByRole("spinbutton", { name: "Goal width" }).fill("1100");
  expect((await stalePut).status()).toBe(409);

  await expect(stale.getByRole("alert")).toContainText("Edited elsewhere");
  await expect(
    stale.getByRole("spinbutton", { name: "Field width" }),
  ).toHaveValue("6100");

  const doc = (await (await fresh.request.get("/api/config")).json()) as {
    document: { field: { fieldWidth: number; goalWidth: number } };
  };
  expect(doc.document.field).toMatchObject({
    fieldWidth: 6100,
    goalWidth: 1000,
  });

  await context.close();
});

test("saving writes the file and clears the unsaved marker", async ({
  page,
  request,
}) => {
  await withConfigFile(request, async (path) => {
    await page.goto("/#0/field");

    const put = page.waitForResponse(
      (r) => r.url().endsWith("/api/config") && r.request().method() === "PUT",
    );
    await page.getByRole("spinbutton", { name: "Field width" }).fill("6100");
    await put;

    await page.keyboard.press("Control+s");
    await page.getByRole("button", { name: "Save 1 change" }).click();

    await expect(
      page.getByRole("link", { name: "Field Dimensions", exact: true }).first(),
    ).toBeVisible();
    expect(readFileSync(path, "utf8")).toContain("field_width: 6100");
  });
});

test("a file changed on disk can be loaded", async ({ page, request }) => {
  await withConfigFile(request, async (path, original) => {
    await page.goto("/#0/field");
    await expect(
      page.getByRole("spinbutton", { name: "Field width" }),
    ).toHaveValue("6000");

    writeFileSync(
      path,
      original.replace("field_width: 6000", "field_width: 6200"),
    );

    // The host checks the file once a second.
    await expect(page.getByText("Configuration changed on disk")).toBeVisible({
      timeout: 5000,
    });
    await page.getByRole("button", { name: "Load from disk" }).click();

    await expect(
      page.getByRole("spinbutton", { name: "Field width" }),
    ).toHaveValue("6200");
  });
});

test("debug snapshots reach the page", async ({ page }) => {
  await page.goto("/#0/geometry");

  // The corner picker's calibration input, and the snapshot grid.
  const picker = page
    .locator('img[src*="/api/snapshot/0/geomcalib_input"]')
    .first();
  await expect(picker).toBeVisible();
  expect(
    await picker.evaluate(
      (img) => (img as unknown as { naturalWidth: number }).naturalWidth,
    ),
  ).toBeGreaterThan(0);

  await expect(page.getByText("cam 0 / raw")).toBeVisible();
});

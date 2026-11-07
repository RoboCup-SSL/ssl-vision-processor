// The first thing a new user sees: the host on a config it just created, which
// publishes nothing until saved. See playwright.config.ts for this second host.
import { expect, test } from "@playwright/test";

test.use({ baseURL: "http://127.0.0.1:8097" });

test("a first run opens the wizard, and saving starts publishing", async ({
  page,
  request,
}) => {
  const firstRun = async (): Promise<boolean> => {
    const res = await request.get("/api/config");
    const body = (await res.json()) as { state: { firstRun: boolean } };

    return body.state.firstRun;
  };

  expect(await firstRun()).toBe(true);

  await page.goto("/");
  await expect(page.getByRole("button", { name: /Division B/ })).toBeVisible();
  await page.getByRole("button", { name: /Division B/ }).click();
  await page.getByRole("button", { name: "Apply" }).click();

  await page.getByRole("button", { name: "Save and start publishing" }).click();
  await expect.poll(firstRun).toBe(false);

  // Saved, so it's no longer a first run: the wizard stays closed.
  await page.reload();
  await expect(
    page.getByRole("button", { name: /Division B/ }),
  ).not.toBeVisible();
});

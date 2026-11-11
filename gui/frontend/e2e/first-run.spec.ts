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

  // The setup checklist stays closed until asked for.
  const checklist = page.getByRole("complementary", {
    name: "Setup checklist",
  });
  await expect(page.getByRole("button", { name: /Division B/ })).toBeVisible();
  await expect(checklist).toHaveCount(0);

  // Close the wizard, then come back to it through the checklist.
  await page.keyboard.press("Escape");
  await page.getByTitle(/Open the setup checklist/).click();
  await expect(checklist).toBeVisible();
  await expect(
    checklist.getByRole("button", { name: /To do\s*Field dimensions/ }),
  ).toBeVisible();

  // Opening expands the first step still to do, which may be this one.
  const openWizard = checklist.getByRole("button", {
    name: "Open setup wizard",
  });
  if (!(await openWizard.isVisible())) {
    await checklist
      .getByRole("button", { name: /To do\s*Field dimensions/ })
      .click();
  }
  await openWizard.click();
  await page.getByRole("button", { name: /Division B/ }).click();
  await page.getByRole("button", { name: "Apply" }).click();

  // Applying the wizard confirms the field.
  await expect(
    checklist.getByRole("button", { name: /Done\s*Field dimensions/ }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Save and start publishing" }).click();
  await expect.poll(firstRun).toBe(false);

  // Saved, so it's no longer a first run: the wizard stays closed.
  await page.reload();
  await expect(
    page.getByRole("button", { name: /Division B/ }),
  ).not.toBeVisible();
});

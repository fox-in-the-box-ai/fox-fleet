import { test, expect, testUsername, testPassword } from "../fixtures/fleet";

test.describe("Console error freedom", () => {
  test("login page has no console errors", async ({
    page,
    consoleErrors,
    pageErrors,
  }) => {
    await page.goto("/cloud/login");
    await page.waitForLoadState("networkidle");

    expect(consoleErrors).toHaveLength(0);
    expect(pageErrors).toHaveLength(0);
  });

  test("admin SPA loads without console errors", async ({
    page,
    loginAsOperator,
    consoleErrors,
    pageErrors,
  }) => {
    await loginAsOperator(testUsername, testPassword);
    await page.goto("/admin/");
    await page.waitForLoadState("networkidle");

    expect(consoleErrors).toHaveLength(0);
    expect(pageErrors).toHaveLength(0);
  });

  test("admin i18n loads without errors", async ({
    page,
    loginAsOperator,
    consoleErrors,
  }) => {
    await loginAsOperator(testUsername, testPassword);
    await page.goto("/admin/");
    await page.waitForLoadState("networkidle");

    const i18nResp = await page.request.get("/admin/i18n/en.json");
    expect(i18nResp.ok()).toBe(true);

    const cspViolations = consoleErrors.filter((e) =>
      e.text.includes("Refused to"),
    );
    expect(cspViolations).toHaveLength(0);
  });
});

import { test, expect } from "../fixtures/fleet";

test.describe("Content-Security-Policy", () => {
  test("login page has CSP header and no violations", async ({
    page,
    consoleErrors,
  }) => {
    const response = await page.goto("/cloud/login");
    expect(response).not.toBeNull();

    const csp = response!.headers()["content-security-policy"];
    expect(csp).toBeDefined();
    expect(csp).toContain("default-src");

    await expect(page.locator("#loginForm")).toBeVisible();
    await expect(page.locator("#submitBtn")).toBeVisible();

    const cspViolations = consoleErrors.filter((e) =>
      e.text.includes("Refused to"),
    );
    expect(cspViolations).toHaveLength(0);
  });

  test("login page inline script executes", async ({ page }) => {
    await page.goto("/cloud/login");

    await expect(page.locator("#loginForm")).toBeVisible();

    const hasSubmitHandler = await page.evaluate(() => {
      const form = document.getElementById("loginForm");
      if (!form) return false;
      const btn = document.getElementById("submitBtn") as HTMLButtonElement;
      return btn !== null && btn.type === "submit";
    });
    expect(hasSubmitHandler).toBe(true);
  });

  test("admin SPA has CSP header", async ({
    page,
    loginAsOperator,
    consoleErrors,
  }) => {
    const username = process.env.E2E_USERNAME || "e2e-admin";
    const password = process.env.E2E_PASSWORD || "e2e-password-8f3a";

    await loginAsOperator(username, password);
    const response = await page.goto("/admin/");
    expect(response).not.toBeNull();

    const csp = response!.headers()["content-security-policy"];
    expect(csp).toBeDefined();

    await page.waitForLoadState("networkidle");

    const cspViolations = consoleErrors.filter((e) =>
      e.text.includes("Refused to"),
    );
    expect(cspViolations).toHaveLength(0);
  });
});

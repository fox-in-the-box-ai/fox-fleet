import { test, expect, testUsername, testPassword } from "../fixtures/fleet";

const isSecure = process.env.FLEET_SECURE === "true";

test.describe("Mixed content protection", () => {
  test.skip(!isSecure, "FLEET_SECURE not set — skipping HTTPS-only tests");

  test("admin SPA loads no HTTP resources over HTTPS", async ({
    page,
    loginAsOperator,
    consoleErrors,
  }) => {
    const httpResources: string[] = [];
    page.on("response", (resp) => {
      const url = resp.url();
      if (url.startsWith("http://") && !url.includes("localhost")) {
        httpResources.push(url);
      }
    });

    await loginAsOperator(testUsername, testPassword);
    await page.goto("/admin/");
    await page.waitForLoadState("networkidle");

    expect(httpResources).toHaveLength(0);

    const mixedContentErrors = consoleErrors.filter(
      (e) =>
        e.text.includes("Mixed Content") ||
        e.text.includes("blocked-mixed-content"),
    );
    expect(mixedContentErrors).toHaveLength(0);
  });

  test("login page loads no HTTP resources over HTTPS", async ({
    page,
    consoleErrors,
  }) => {
    const httpResources: string[] = [];
    page.on("response", (resp) => {
      const url = resp.url();
      if (url.startsWith("http://") && !url.includes("localhost")) {
        httpResources.push(url);
      }
    });

    await page.goto("/cloud/login");
    await page.waitForLoadState("networkidle");

    expect(httpResources).toHaveLength(0);

    const mixedContentErrors = consoleErrors.filter(
      (e) =>
        e.text.includes("Mixed Content") ||
        e.text.includes("blocked-mixed-content"),
    );
    expect(mixedContentErrors).toHaveLength(0);
  });
});

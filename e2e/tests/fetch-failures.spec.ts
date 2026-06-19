import { test, expect, testUsername, testPassword } from "../fixtures/fleet";

test.describe("Fetch and API failures", () => {
  test("API returns JSON with correct content-type", async ({ request }) => {
    const resp = await request.get("/healthz");
    expect(resp.ok()).toBe(true);

    const ct = resp.headers()["content-type"] || "";
    expect(ct).toContain("application/json");
  });

  test("admin SPA fetches succeed after login", async ({
    page,
    loginAsOperator,
    fetchFailures,
    consoleErrors,
  }) => {
    await loginAsOperator(testUsername, testPassword);
    await page.goto("/admin/");
    await page.waitForLoadState("networkidle");

    const apiFailures = fetchFailures.filter((f) =>
      f.url.includes("/api/"),
    );
    expect(apiFailures).toHaveLength(0);

    const corsErrors = consoleErrors.filter(
      (e) =>
        e.text.includes("CORS") ||
        e.text.includes("Access-Control-Allow-Origin"),
    );
    expect(corsErrors).toHaveLength(0);
  });

  test("i18n resources load successfully", async ({
    page,
    loginAsOperator,
  }) => {
    await loginAsOperator(testUsername, testPassword);

    const resp = await page.request.get("/admin/i18n/en.json");
    expect(resp.ok()).toBe(true);

    const ct = resp.headers()["content-type"] || "";
    expect(ct).toContain("application/json");

    const body = await resp.json();
    expect(body).toBeDefined();
    expect(typeof body).toBe("object");
  });

  test("unauthenticated API requests return 401 not 500", async ({
    request,
  }) => {
    const resp = await request.get("/api/instances", {
      headers: { Authorization: "Bearer wrong-secret" },
    });
    expect(resp.status()).toBe(401);

    const ct = resp.headers()["content-type"] || "";
    expect(ct).toContain("application/json");
  });
});

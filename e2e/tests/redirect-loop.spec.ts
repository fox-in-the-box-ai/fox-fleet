import { test, expect, testUsername, testPassword } from "../fixtures/fleet";

test.describe("Redirect loop protection", () => {
  test("unauthenticated root redirects to login without looping", async ({
    page,
  }) => {
    const redirects: string[] = [];
    page.on("response", (resp) => {
      if (resp.status() >= 300 && resp.status() < 400) {
        redirects.push(resp.url());
      }
    });

    await page.goto("/");
    await page.waitForLoadState("networkidle");

    expect(page.url()).toContain("/cloud/login");
    expect(redirects.length).toBeLessThanOrEqual(3);
  });

  test("authenticated user does not redirect back to login", async ({
    page,
  }) => {
    await page.goto("/cloud/login");
    await page.fill("#username", testUsername);
    await page.fill("#password", testPassword);
    await page.click("#submitBtn");
    await page.waitForURL("**/admin/**");

    await page.goto("/cloud/login");
    await page.waitForLoadState("networkidle");

    expect(page.url()).toContain("/admin/");
  });

  test("login page does not redirect loop on bad cookie", async ({
    page,
    context,
  }) => {
    const baseURL = process.env.FLEET_BASE_URL || "http://localhost:19090";
    const url = new URL(baseURL);

    await context.addCookies([
      {
        name: "fox_cloud_session",
        value: "invalid-garbage-token",
        domain: url.hostname,
        path: "/",
      },
    ]);

    const redirects: string[] = [];
    page.on("response", (resp) => {
      if (resp.status() >= 300 && resp.status() < 400) {
        redirects.push(resp.url());
      }
    });

    await page.goto("/cloud/login");
    await page.waitForLoadState("networkidle");

    expect(page.url()).toContain("/cloud/login");
    expect(redirects.length).toBeLessThanOrEqual(2);
  });
});

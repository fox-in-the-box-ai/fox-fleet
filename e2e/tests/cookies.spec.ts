import { test, expect, testUsername, testPassword } from "../fixtures/fleet";

const COOKIE_NAME = "fox_cloud_session";

test.describe("Cookie security attributes", () => {
  test("login sets session cookie with correct attributes", async ({
    request,
  }) => {
    const resp = await request.post("/cloud/login", {
      data: { username: testUsername, password: testPassword },
    });
    expect(resp.ok()).toBe(true);

    const setCookie = resp.headers()["set-cookie"];
    expect(setCookie).toBeDefined();
    expect(setCookie).toContain(COOKIE_NAME);
    expect(setCookie.toLowerCase()).toContain("httponly");
    expect(setCookie.toLowerCase()).toContain("samesite=strict");
    expect(setCookie.toLowerCase()).toContain("path=/");

    if (process.env.FLEET_SECURE === "true") {
      expect(setCookie.toLowerCase()).toContain("secure");
    }
  });

  test("login does not set Domain attribute (host-only scoping)", async ({
    request,
  }) => {
    const resp = await request.post("/cloud/login", {
      data: { username: testUsername, password: testPassword },
    });
    expect(resp.ok()).toBe(true);

    const setCookie = resp.headers()["set-cookie"] || "";
    const parts = setCookie.toLowerCase().split(";").map((s: string) => s.trim());
    const hasDomain = parts.some((p: string) => p.startsWith("domain="));
    expect(hasDomain).toBe(false);
  });

  test("repeated login overwrites session cookie", async ({ request }) => {
    const resp1 = await request.post("/cloud/login", {
      data: { username: testUsername, password: testPassword },
    });
    expect(resp1.ok()).toBe(true);

    const resp2 = await request.post("/cloud/login", {
      data: { username: testUsername, password: testPassword },
    });
    expect(resp2.ok()).toBe(true);

    const cookie1 = extractCookieValue(resp1.headers()["set-cookie"]);
    const cookie2 = extractCookieValue(resp2.headers()["set-cookie"]);
    expect(cookie1).not.toEqual(cookie2);
  });

  test("logout clears session cookie", async ({ request }) => {
    const loginResp = await request.post("/cloud/login", {
      data: { username: testUsername, password: testPassword },
    });
    expect(loginResp.ok()).toBe(true);

    const loginCookie = loginResp.headers()["set-cookie"] || "";
    const tokenMatch = loginCookie.match(new RegExp(`${COOKIE_NAME}=([^;]+)`));
    expect(tokenMatch).not.toBeNull();

    const logoutResp = await request.post("/cloud/logout", {
      headers: { Cookie: `${COOKIE_NAME}=${tokenMatch![1]}` },
    });
    expect(logoutResp.status()).toBe(204);

    const setCookie = logoutResp.headers()["set-cookie"] || "";
    expect(setCookie).toContain(COOKIE_NAME);
    const maxAge = setCookie.match(/max-age=(-?\d+)/i);
    expect(maxAge).not.toBeNull();
    expect(parseInt(maxAge![1])).toBeLessThanOrEqual(0);
  });
});

function extractCookieValue(setCookie: string): string {
  const match = setCookie.match(new RegExp(`${COOKIE_NAME}=([^;]+)`));
  return match ? match[1] : "";
}

import { test, expect } from "../fixtures/fleet";

const adminSecret = process.env.E2E_ADMIN_SECRET || "e2e-admin-secret-00ff";
const canProvision = process.env.E2E_PROVISION === "true";

test.describe("Provision + user-instance binding (#181)", () => {
  test.describe("API-level validation", () => {
    test("POST /api/instances/provision rejects username != slug", async ({
      request,
    }) => {
      const resp = await request.post("/api/instances/provision", {
        headers: {
          Authorization: `Bearer ${adminSecret}`,
          "Content-Type": "application/json",
        },
        data: {
          slug: "e2e-bind-test",
          username: "different-name",
          password: "e2e-test-password-secure",
        },
      });
      expect(resp.status()).toBe(400);

      const body = await resp.json();
      expect(body.error).toBe("bad_request");
      expect(body.message).toContain("username must equal slug");
    });
  });

  test.describe("On-target provisioning", () => {
    test.skip(
      !canProvision,
      "E2E_PROVISION not set — these tests require Docker container provisioning",
    );

    const testSlug = "e2e-bind-test";

    test.afterEach(async ({ request }) => {
      await request.delete(`/api/instances/${testSlug}`, {
        headers: { Authorization: `Bearer ${adminSecret}` },
      });
      await request.delete(`/api/users/${testSlug}`, {
        headers: { Authorization: `Bearer ${adminSecret}` },
      });
    });

    test("POST /api/instances/provision binds user to instance", async ({
      request,
    }) => {
      const provResp = await request.post("/api/instances/provision", {
        headers: {
          Authorization: `Bearer ${adminSecret}`,
          "Content-Type": "application/json",
        },
        data: {
          slug: testSlug,
          password: "e2e-test-password-secure",
        },
      });
      expect(provResp.status()).toBe(201);

      const provBody = await provResp.json();
      expect(provBody.instance_id).toBe(testSlug);
      expect(provBody.username).toBe(testSlug);
      expect(provBody.status).toBe("provisioning");

      let instanceReady = false;
      for (let i = 0; i < 30; i++) {
        const statusResp = await request.get(`/api/instances/${testSlug}`, {
          headers: { Authorization: `Bearer ${adminSecret}` },
        });
        if (statusResp.ok()) {
          const inst = await statusResp.json();
          if (inst.status === "running") {
            instanceReady = true;
            break;
          }
        }
        await new Promise((r) => setTimeout(r, 2000));
      }
      expect(instanceReady).toBe(true);

      const userResp = await request.get(`/api/users/${testSlug}`, {
        headers: { Authorization: `Bearer ${adminSecret}` },
      });
      expect(userResp.ok()).toBe(true);
      const user = await userResp.json();
      expect(user.username).toBe(testSlug);
      expect(user.instance_id).toBe(testSlug);

      const tlsResp = await request.get(
        `/cloud/tls-check?domain=${testSlug}.fleet.test`,
      );
      expect(tlsResp.status()).toBe(200);
    });

    test("POST /api/instances with owner auto-binds in Cloud mode", async ({
      request,
    }) => {
      const createUserResp = await request.post("/api/users", {
        headers: {
          Authorization: `Bearer ${adminSecret}`,
          "Content-Type": "application/json",
        },
        data: { username: testSlug, password: "e2e-test-password-secure" },
      });
      expect(createUserResp.status()).toBe(201);

      const createResp = await request.post("/api/instances", {
        headers: {
          Authorization: `Bearer ${adminSecret}`,
          "Content-Type": "application/json",
        },
        data: { id: testSlug, owner: testSlug },
      });
      expect(createResp.status()).toBe(201);

      let instanceReady = false;
      for (let i = 0; i < 30; i++) {
        const statusResp = await request.get(`/api/instances/${testSlug}`, {
          headers: { Authorization: `Bearer ${adminSecret}` },
        });
        if (statusResp.ok()) {
          const inst = await statusResp.json();
          if (inst.status === "running") {
            instanceReady = true;
            break;
          }
        }
        await new Promise((r) => setTimeout(r, 2000));
      }
      expect(instanceReady).toBe(true);

      const userResp = await request.get(`/api/users/${testSlug}`, {
        headers: { Authorization: `Bearer ${adminSecret}` },
      });
      expect(userResp.ok()).toBe(true);
      const user = await userResp.json();
      expect(user.instance_id).toBe(testSlug);
    });
  });
});

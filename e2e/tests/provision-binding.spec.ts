import { test, expect } from "../fixtures/fleet";

const adminSecret = process.env.E2E_ADMIN_SECRET || "e2e-admin-secret-00ff";
const testSlug = "e2e-bind-test";

test.describe("Provision + user-instance binding (#181)", () => {
  test.afterEach(async ({ request }) => {
    // Best-effort cleanup: destroy instance + delete user.
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
    // Provision via the Cloud provision endpoint.
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

    // Wait for provisioning to complete (poll instance status).
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

    // Verify user has instance_id bound.
    const userResp = await request.get(`/api/users/${testSlug}`, {
      headers: { Authorization: `Bearer ${adminSecret}` },
    });
    expect(userResp.ok()).toBe(true);
    const user = await userResp.json();
    expect(user.username).toBe(testSlug);
    expect(user.instance_id).toBe(testSlug);

    // Verify TLS check endpoint accepts this subdomain.
    const tlsResp = await request.get(
      `/cloud/tls-check?domain=${testSlug}.fleet.test`,
    );
    expect(tlsResp.status()).toBe(200);
  });

  test("POST /api/instances/provision rejects username != slug", async ({
    request,
  }) => {
    const resp = await request.post("/api/instances/provision", {
      headers: {
        Authorization: `Bearer ${adminSecret}`,
        "Content-Type": "application/json",
      },
      data: {
        slug: testSlug,
        username: "different-name",
        password: "e2e-test-password-secure",
      },
    });
    expect(resp.status()).toBe(400);

    const body = await resp.json();
    expect(body.error).toBe("bad_request");
    expect(body.message).toContain("username must equal slug");
  });

  test("POST /api/instances with owner auto-binds in Cloud mode", async ({
    request,
  }) => {
    // Pre-create the user.
    const createUserResp = await request.post("/api/users", {
      headers: {
        Authorization: `Bearer ${adminSecret}`,
        "Content-Type": "application/json",
      },
      data: { username: testSlug, password: "e2e-test-password-secure" },
    });
    expect(createUserResp.status()).toBe(201);

    // Create instance via admin API with owner field.
    const createResp = await request.post("/api/instances", {
      headers: {
        Authorization: `Bearer ${adminSecret}`,
        "Content-Type": "application/json",
      },
      data: { id: testSlug, owner: testSlug },
    });
    expect(createResp.status()).toBe(201);

    // Wait for provisioning.
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

    // Verify user now has instance_id bound.
    const userResp = await request.get(`/api/users/${testSlug}`, {
      headers: { Authorization: `Bearer ${adminSecret}` },
    });
    expect(userResp.ok()).toBe(true);
    const user = await userResp.json();
    expect(user.instance_id).toBe(testSlug);
  });
});

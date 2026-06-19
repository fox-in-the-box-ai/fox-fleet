import { test as base, type Page } from "@playwright/test";

export const testUsername = process.env.E2E_USERNAME || "e2e-admin";
export const testPassword = process.env.E2E_PASSWORD || "e2e-password-8f3a";

export type ConsoleEntry = {
  type: string;
  text: string;
};

export type FleetFixtures = {
  consoleErrors: ConsoleEntry[];
  pageErrors: Error[];
  fetchFailures: { url: string; status: number }[];
  loginAsOperator: (username: string, password: string) => Promise<void>;
};

export const test = base.extend<FleetFixtures>({
  consoleErrors: async ({ page }, use) => {
    const errors: ConsoleEntry[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        errors.push({ type: msg.type(), text: msg.text() });
      }
    });
    await use(errors);
  },

  pageErrors: async ({ page }, use) => {
    const errors: Error[] = [];
    page.on("pageerror", (err) => errors.push(err));
    await use(errors);
  },

  fetchFailures: async ({ page }, use) => {
    const failures: { url: string; status: number }[] = [];
    page.on("response", (resp) => {
      if (resp.status() >= 400) {
        failures.push({ url: resp.url(), status: resp.status() });
      }
    });
    await use(failures);
  },

  loginAsOperator: async ({ request }, use) => {
    const login = async (username: string, password: string) => {
      const resp = await request.post("/cloud/login", {
        data: { username, password },
      });
      if (!resp.ok()) {
        throw new Error(`Login failed: ${resp.status()} ${await resp.text()}`);
      }
    };
    await use(login);
  },
});

export { expect } from "@playwright/test";

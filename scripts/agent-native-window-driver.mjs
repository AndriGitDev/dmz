// SPDX-License-Identifier: GPL-3.0-only
// Development-only driver for a browser opened by DMZ through xdg-open.
// The parent supplies a private Chromium profile on its own virtual display.
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
const args = process.argv.slice(2);
const option = (name) => {
  const i = args.indexOf(name);
  return i < 0 ? undefined : args[i + 1];
};
let browser;
let checks = 0;
const check = (value, code) => {
  if (!value) throw Error(code);
  checks++;
};
try {
  const { chromium } = createRequire(import.meta.url)(
    resolve(option("--playwright")),
  );
  const profile = option("--profile");
  let endpoint;
  for (let n = 0; n < 120; n++) {
    try {
      const [port, path] = String(
        await readFile(join(profile, "DevToolsActivePort")),
      )
        .trim()
        .split("\n");
      if (/^\d+$/.test(port) && path.startsWith("/devtools/browser/")) {
        endpoint = `ws://127.0.0.1:${port}${path}`;
        break;
      }
    } catch {}
    await delay(50);
  }
  check(!!endpoint, "native_browser_cdp_not_ready");
  browser = await chromium.connectOverCDP(endpoint);
  const origin = option("--origin");
  let page;
  for (let n = 0; n < 120; n++) {
    page = browser
      .contexts()
      .flatMap((c) => c.pages())
      .find((p) => p.url().startsWith(origin + "/"));
    if (page) break;
    await delay(50);
  }
  check(!!page, "native_browser_did_not_open_app");
  const errors = [];
  page.on("pageerror", () => errors.push(true));
  await page.locator("#clients .tool-card").first().waitFor();
  check(!new URL(page.url()).hash, "native_browser_handoff_not_cleared");
  if ((await page.locator("html").getAttribute("lang")) !== "en")
    await page.locator("#language").click();
  const client = option("--client");
  if (!args.includes("--reopen")) {
    await page.locator("#project").fill(option("--project"));
    await page
      .locator("#api-key-" + client)
      .fill(
        client === "codex"
          ? "sk-dmz-synthetic-fixture-openai-000000000000"
          : "sk-ant-dmz-synthetic-fixture-000000000000",
      );
    await page.locator("#start-" + client).click();
  }
  await page
    .locator(".session .status")
    .filter({ hasText: "Stopped" })
    .waitFor({ timeout: 30000 });
  check(
    await page
      .locator(".session-stats")
      .innerText()
      .then((s) => /\b1 text parts masked\b/.test(s)),
    "native_browser_masked_count_missing",
  );
  check(
    !(await page.locator("#error").isVisible()),
    "native_browser_setup_error",
  );
  check(
    (await page.locator("#api-key-" + client).inputValue()) === "",
    "native_browser_retained_key",
  );
  check(errors.length === 0, "native_browser_script_error");
  console.log(
    JSON.stringify({
      checks,
      passed: true,
      reopened: args.includes("--reopen"),
      setup_form_actions: args.includes("--reopen") ? 0 : 3,
    }),
  );
} catch (error) {
  console.error(
    /^[a-z_]+$/.test(error.message)
      ? error.message
      : "native_browser_driver_failed",
  );
  process.exitCode = 1;
} finally {
  if (browser) {
    try {
      await (await browser.newBrowserCDPSession()).send("Browser.close");
    } catch {}
    await browser.close().catch(() => {});
  }
}

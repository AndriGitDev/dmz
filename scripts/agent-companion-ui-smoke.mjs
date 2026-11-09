#!/usr/bin/env node
// SPDX-License-Identifier: GPL-3.0-only
// Development-only browser check. Playwright is supplied by the operator; no
// runtime dependency, actual provider traffic, or user authentication is used.
import { createRequire } from "node:module";
import { spawn } from "node:child_process";
import {
  mkdtemp,
  readFile,
  mkdir,
  rm,
  writeFile,
  chmod,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
const args = process.argv.slice(2);
const option = (name, fallback) => {
  const i = args.indexOf(name);
  return i < 0 ? fallback : args[i + 1];
};
const binary = resolve(option("--dmz", "/tmp/dmz-companion-ui"));
const modulePath = option("--playwright", process.env.DMZ_TEST_PLAYWRIGHT);
const executablePath = option("--browser", process.env.DMZ_TEST_BROWSER);
if (!modulePath) {
  console.error("ui_smoke_test_dependencies_required");
  process.exit(2);
}
const { chromium } = createRequire(import.meta.url)(resolve(modulePath));
const work = await mkdtemp(join(tmpdir(), "dmz-ui-smoke-"));
const descriptor = join(work, "control.json");
const project = join(work, "Synthetic project ' $() Í");
await mkdir(project);
const stubbin = join(work, "bin");
await mkdir(stubbin);
for (const name of ["codex", "claude"]) {
  const path = join(stubbin, name);
  await writeFile(path, "#!/bin/sh\nexit 0\n");
  await chmod(path, 0o700);
}
const env = {
  PATH: stubbin + ":/usr/bin:/bin",
  HOME: join(work, "home"),
  XDG_CACHE_HOME: join(work, "cache"),
  LANG: "C.UTF-8",
  OPENAI_API_KEY: "synthetic-env-key",
  ANTHROPIC_API_KEY: "synthetic-env-key",
};
await mkdir(env.HOME);
const child = spawn(
  binary,
  ["app", "--no-open", "--control-file", descriptor],
  { env, stdio: "ignore" },
);
let browser,
  origin,
  token,
  checks = 0,
  external = 0;
const check = (cond, code) => {
  if (!cond) throw Error(code);
  checks++;
};
const control = async (path, body) => {
  const r = await fetch(origin + "/api/" + path, {
    method: body === undefined ? "GET" : "POST",
    headers: {
      Authorization: "Bearer " + token,
      "Content-Type": "application/json",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  check(r.ok, "control_request_failed");
  return r.status === 204 ? null : r.json();
};
let exitCode = 0;
try {
  for (let n = 0; n < 100; n++) {
    try {
      ({ origin, token } = JSON.parse(await readFile(descriptor, "utf8")));
      break;
    } catch {}
    if (child.exitCode !== null) throw Error("app_exited_before_ready");
    await delay(50);
  }
  check(!!origin && !!token, "app_not_ready");
  const handoff = await control("window", {});
  check(!handoff.address.includes(token), "control_token_in_window_address");
  browser = await chromium.launch({
    executablePath,
    headless: true,
    args: ["--no-sandbox", "--disable-background-networking"],
  });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 1000 },
    colorScheme: "light",
  });
  await context.route("**/*", async (route) => {
    if (new URL(route.request().url()).origin === origin)
      await route.continue();
    else {
      external++;
      await route.abort();
    }
  });
  const page = await context.newPage();
  let lastStart;
  page.on("request", (r) => {
    if (r.url() === origin + "/api/start") lastStart = r.postDataJSON();
  });
  const errors = [];
  page.on("pageerror", () => errors.push("page_error"));
  await page.goto(handoff.address);
  await page.locator("#clients input").first().waitFor();
  check(new URL(page.url()).hash === "", "window_handoff_not_cleared");
  check(
    (await page.locator("html").getAttribute("lang")) === "is",
    "icelandic_default_missing",
  );
  check(
    (await page.locator("fieldset").count()) === 3,
    "setup_sections_changed",
  );
  const stored = await page.evaluate(() =>
    sessionStorage.getItem("dmz.control"),
  );
  check(stored === token, "browser_bootstrap_failed");
  const replay = await fetch(origin + "/api/bootstrap", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code: new URL(handoff.address).hash.slice(6) }),
  });
  check(replay.status === 401, "window_handoff_replayed");
  await page.locator("#language").click();
  check(
    (await page.locator("html").getAttribute("lang")) === "en",
    "language_switch_failed",
  );
  check(
    (await page.locator("#start").innerText()) === "Start with DMZ →",
    "english_action_missing",
  );
  const detected = await page.locator("#clients input:not(:disabled)").count();
  check(detected > 0, "installed_cli_not_detected");
  check(
    (await page.locator("#environment-option").isVisible()) &&
      !(await page.locator("#use-environment").isChecked()),
    "environment_key_selected_without_consent",
  );
  await page.locator("#project").fill(join(work, "missing"));
  await page.locator("#api-key").fill("synthetic-ui-key");
  await page.locator("#start").click();
  await page
    .locator("#error")
    .filter({ hasText: "Choose an existing project folder." })
    .waitFor();
  check(
    (await page.locator("#api-key").inputValue()) === "synthetic-ui-key",
    "validation_lost_key",
  );
  if (detected > 1) {
    const selected = await page.locator("#clients input:checked").inputValue();
    await page
      .locator("#clients input:not(:disabled)")
      .filter({ visible: true })
      .last()
      .check();
    if (
      (await page.locator("#clients input:checked").inputValue()) !== selected
    )
      check(
        (await page.locator("#api-key").inputValue()) === "",
        "client_change_retained_key",
      );
  }
  await page.locator("#api-key").fill("");
  await page.locator("#project").fill(project);
  await page.locator("#start").click();
  await page
    .locator("#error")
    .filter({ hasText: "DMZ could not open a desktop terminal." })
    .waitFor();
  check(lastStart.auth === "account", "environment_key_forced_api_billing");
  check(
    (await page.locator(".session .status").innerText()) === "Stopped",
    "failed_launch_left_active_session",
  );
  const state = await control("state");
  check(
    state.sessions.length === 1 && state.sessions[0].requests === 0,
    "headless_launch_generated_model_traffic",
  );
  await page.reload();
  await page.locator("#clients input").first().waitFor();
  check(
    (await page.locator("html").getAttribute("lang")) === "en",
    "language_preference_not_retained",
  );
  await page.setViewportSize({ width: 390, height: 844 });
  check(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "mobile_horizontal_overflow",
  );
  await page.emulateMedia({ colorScheme: "dark" });
  check(await page.locator("#start").isVisible(), "dark_mode_action_missing");
  if (option("--screenshot", null))
    await page.screenshot({
      path: resolve(option("--screenshot")),
      fullPage: true,
    });
  check(errors.length === 0, "browser_script_error");
  check(external === 0, "unexpected_external_browser_request");
  await page.locator("#quit").click();
  await page
    .getByRole("heading", { name: "DMZ is closed. You can close this window." })
    .waitFor();
  check(
    (await page.evaluate(() => sessionStorage.getItem("dmz.control"))) === null,
    "quit_retained_control_credential",
  );
  for (let n = 0; n < 100 && child.exitCode === null; n++) await delay(50);
  check(child.exitCode === 0, "quit_did_not_stop_app");
  check(errors.length === 0, "quit_browser_script_error");
  console.log(
    JSON.stringify({
      evidence_kind: "local_headless_browser_synthetic",
      checks,
      external_requests: external,
      passed: true,
      native_desktop_qualified: false,
    }),
  );
} catch (err) {
  console.error(
    /^[a-z_]+$/.test(err.message) ? err.message : "ui_smoke_failed",
  );
  exitCode = 1;
} finally {
  await browser?.close();
  child.kill("SIGTERM");
  for (let n = 0; n < 40 && child.exitCode === null; n++) await delay(50);
  if (child.exitCode === null) child.kill("SIGKILL");
  await rm(work, { recursive: true, force: true });
}
process.exitCode = exitCode;

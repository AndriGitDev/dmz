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
  await page.locator("#clients .tool-card").first().waitFor();
  check(new URL(page.url()).hash === "", "window_handoff_not_cleared");
  check(
    (await page.locator("html").getAttribute("lang")) === "is",
    "icelandic_default_missing",
  );
  check(
    (await page.locator("fieldset").count()) === 2,
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
    (await page.locator("#start-codex").innerText()) === "Open Codex CLI →",
    "english_action_missing",
  );
  await page.locator("#project").fill(project);
  await page.route("**/api/folder", (route) =>
    route.fulfill({ json: { cancelled: true } }),
  );
  await page.locator("#choose-folder").click();
  await page.waitForFunction(
    () => !document.querySelector("#choose-folder").disabled,
  );
  check(
    (await page.locator("#project").inputValue()) === project,
    "cancelled_picker_changed_project",
  );
  check(
    !(await page.locator("#error").isVisible()),
    "cancelled_picker_reported_error",
  );
  await page.unroute("**/api/folder");
  const detected = await page.locator(".tool-start:not(:disabled)").count();
  check(detected > 0, "installed_cli_not_detected");
  check(
    (await page.locator("#use-environment-codex").isVisible()) &&
      !(await page.locator("#use-environment-codex").isChecked()),
    "environment_key_selected_without_consent",
  );
  await page.locator("#project").fill(join(work, "missing"));
  await page.locator("#api-key-codex").fill("synthetic-ui-key");
  await page.locator("#start-codex").click();
  await page
    .locator("#error")
    .filter({ hasText: "Choose an existing project folder." })
    .waitFor();
  check(
    (await page.locator("#api-key-codex").inputValue()) === "synthetic-ui-key",
    "validation_lost_key",
  );
  await page.locator("#project").fill(project);
  await page.locator("#start-claude").click();
  await page
    .locator("#error")
    .filter({ hasText: "DMZ could not open a desktop terminal." })
    .waitFor();
  check(lastStart.auth === "account", "environment_key_forced_api_billing");
  check(
    lastStart.client === "claude" && lastStart.api_key === "",
    "tool_button_used_another_tools_key",
  );
  check(
    (await page.locator("#api-key-codex").inputValue()) === "synthetic-ui-key",
    "other_tool_changed_pending_key",
  );
  check(
    (await page.locator(".session .status").innerText()) === "Stopped",
    "failed_launch_left_active_session",
  );
  const state = await control("state");
  check(
    state.sessions.length === 1 && state.sessions[0].requests === 0,
    "headless_launch_generated_model_traffic",
  );
  await page.locator("#api-key-codex").fill("");
  await page.locator("#use-environment-codex").check();
  await page.locator("#start-codex").click();
  await page.locator("#start-codex").waitFor({ state: "visible" });
  await page.waitForFunction(
    () => !document.querySelector("#start-codex").disabled,
  );
  check(
    lastStart.client === "codex" &&
      lastStart.auth === "api" &&
      lastStart.api_key === "",
    "environment_key_opt_in_failed",
  );
  // Polling must keep the actual Stop button and its keyboard focus, even
  // when counters change. These are synthetic UI states, not client evidence.
  let syntheticRequests = 1;
  let changedClients = false;
  await page.route("**/api/state", async (route) => {
    const response = await route.fetch();
    const value = await response.json();
    value.sessions = [
      {
        id: "synthetic-focus-session",
        client: "codex",
        state: "running",
        requests: syntheticRequests,
        changed_parts: 1,
        warnings: {},
      },
    ];
    if (changedClients) value.clients[0].api_key_available = false;
    await route.fulfill({ response, json: value });
  });
  const stop = page.locator('[data-session-stop="synthetic-focus-session"]');
  await stop.waitFor();
  await stop.focus();
  const originalStop = await stop.elementHandle();
  syntheticRequests = 2;
  await page.waitForFunction(() =>
    document.querySelector(".session-stats")?.textContent.startsWith("2 "),
  );
  check(
    await originalStop.evaluate(
      (el) => el.isConnected && document.activeElement === el,
    ),
    "session_poll_lost_keyboard_focus",
  );
  check(
    !(await page.locator("#sessions").getAttribute("aria-live")),
    "whole_session_list_is_live_region",
  );
  let releaseStart;
  let startHeld;
  const held = new Promise((resolve) => {
    startHeld = resolve;
  });
  await page.route("**/api/start", async (route) => {
    startHeld();
    await new Promise((resolve) => {
      releaseStart = resolve;
    });
    await route.fulfill({ json: { error: "choose_project" } });
  });
  await page.locator("#start-codex").click();
  await held;
  changedClients = true;
  await delay(1800);
  check(
    (await page.locator(".tool-start:not(:disabled)").count()) === 0,
    "client_poll_reenabled_pending_launch",
  );
  releaseStart();
  await page.waitForFunction(
    () => !document.querySelector("#start-codex").disabled,
  );
  check(
    !(await page.locator("#use-environment-codex").count()),
    "deferred_client_update_missing",
  );
  await page.unroute("**/api/start");
  await page.unroute("**/api/state");
  await page.reload();
  await page.locator("#clients .tool-card").first().waitFor();
  check(
    (await page.locator("html").getAttribute("lang")) === "en",
    "language_preference_not_retained",
  );
  let backgroundRoutes = [
    {
      client: "codex",
      name: "Codex CLI",
      detected: true,
      routed: true,
      state: "restart_required",
      requests: 0,
      changed_parts: 0,
    },
    {
      client: "claude",
      name: "Claude Code",
      detected: true,
      routed: false,
      state: "needs_attention",
      issue: "routing_custom_provider_present",
      requests: 0,
      changed_parts: 0,
    },
  ];
  const backgroundActions = [];
  await page.route("**/api/state", async (route) => {
    const response = await route.fetch();
    const value = await response.json();
    value.background_enabled = true;
    value.background = backgroundRoutes;
    await route.fulfill({ response, json: value });
  });
  await page.route("**/api/background", async (route) => {
    const body = route.request().postDataJSON();
    backgroundActions.push(body);
    const item = backgroundRoutes.find((r) => r.client === body.client);
    item.routed = body.connected;
    item.state = body.connected ? "restart_required" : "disconnected";
    await route.fulfill({ status: 204 });
  });
  await page.locator("#background-section").waitFor({ state: "visible" });
  check(
    await page.locator("#start-form").isHidden(),
    "background_requires_project_launch_form",
  );
  check(
    (await page.locator("#background-connections article").count()) === 2,
    "background_connections_missing",
  );
  check(
    (await page.locator("h1").innerText()) === "DMZ runs in the background.",
    "background_heading_missing",
  );
  const connection = page.locator('[data-background-client="codex"]');
  check(
    (await connection.innerText()) === "Disconnect",
    "routed_connection_cannot_disconnect",
  );
  await connection.focus();
  const originalConnection = await connection.elementHandle();
  backgroundRoutes[0].requests = 2;
  backgroundRoutes[0].state = "traffic_observed";
  await page.waitForFunction(() =>
    document
      .querySelector("#background-connections .session-stats")
      ?.textContent.startsWith("2 "),
  );
  check(
    await originalConnection.evaluate(
      (el) => el.isConnected && document.activeElement === el,
    ),
    "background_poll_lost_keyboard_focus",
  );
  check(
    (await page.locator("#background-connections").innerText()).includes(
      "DMZ left it unchanged",
    ),
    "custom_gateway_warning_missing",
  );
  const backgroundCard = page.locator("#background-connections article").first();
  backgroundRoutes[0].warnings = { attachment_unchanged: 1, mapping_evicted_history_may_degrade: 1 };
  backgroundRoutes[0].recent_warnings = [];
  backgroundRoutes[0].requests = 3;
  await page.waitForFunction(() => document.querySelector("#background-connections .session-stats")?.textContent.startsWith("3 "));
  check(await backgroundCard.locator(".session-warning").count() === 0, "routine_background_notice_requires_attention");
  backgroundRoutes[0].warnings.upstream_unavailable = 1;
  backgroundRoutes[0].recent_warnings = ["upstream_unavailable"];
  backgroundRoutes[0].state = "needs_attention";
  await backgroundCard.locator(".session-warning").waitFor();
  check((await backgroundCard.innerText()).includes("last two minutes"), "recent_warning_not_disclosed");
  backgroundRoutes[0].recent_warnings = [];
  backgroundRoutes[0].state = "traffic_observed";
  await backgroundCard.locator(".session-warning").waitFor({ state: "detached" });
  check((await backgroundCard.innerText()).includes("Processing warnings recorded since connection: 1."), "old_warning_history_lost");
  await connection.click();
  await page.waitForFunction(
    () =>
      document.querySelector('[data-background-client="codex"]')
        ?.textContent === "Connect",
  );
  check(
    backgroundActions[0]?.connected === false,
    "background_disconnect_not_requested",
  );
  await connection.click();
  await page.waitForFunction(
    () =>
      document.querySelector('[data-background-client="codex"]')
        ?.textContent === "Disconnect",
  );
  check(
    backgroundActions[1]?.connected === true,
    "background_reconnect_used_stale_action",
  );
  let backgroundQuitMessage = "";
  page.once("dialog", async (dialog) => {
    backgroundQuitMessage = dialog.message();
    await dialog.dismiss();
  });
  await page.locator("#quit").click();
  check(backgroundQuitMessage.includes("without DMZ masking"), "quit_did_not_explain_direct_connection");
  check(child.exitCode === null && await page.locator("#quit").isEnabled(), "cancelled_background_quit_stopped_app");
  await page.unroute("**/api/background");
  await page.unroute("**/api/state");
  await page.reload();
  await page.locator("#clients .tool-card").first().waitFor();
  await page.setViewportSize({ width: 390, height: 844 });
  check(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    "mobile_horizontal_overflow",
  );
  await page.emulateMedia({ colorScheme: "dark" });
  check(
    await page.locator("#start-codex").isVisible(),
    "dark_mode_action_missing",
  );
  if (option("--screenshot", null))
    await page.screenshot({
      path: resolve(option("--screenshot")),
      fullPage: true,
    });
  check(errors.length === 0, "browser_script_error");
  check(external === 0, "unexpected_external_browser_request");
  const restricted = await context.newPage();
  const restrictedErrors = [];
  restricted.on("pageerror", () => restrictedErrors.push(true));
  await restricted.addInitScript(() => {
    for (const name of ["localStorage", "sessionStorage"])
      Object.defineProperty(window, name, {
        get() {
          throw Error("storage blocked");
        },
      });
  });
  const restrictedHandoff = await control("window", {});
  await restricted.goto(restrictedHandoff.address);
  await restricted.locator("#clients .tool-card").first().waitFor();
  check(
    !(await restricted.locator("#error").isVisible()),
    "blocked_storage_prevented_setup",
  );
  await restricted.locator("#language").click();
  check(
    (await restricted.locator("html").getAttribute("lang")) === "en",
    "blocked_storage_prevented_language_switch",
  );
  check(restrictedErrors.length === 0, "blocked_storage_script_error");
  await restricted.close();
  await page.route("**/api/quit", (route) =>
    route.fulfill({ status: 503, body: "synthetic failure" }),
  );
  await page.locator("#quit").click();
  await page
    .locator("#error")
    .filter({ hasText: "Open DMZ again to reconnect to the app." })
    .waitFor();
  check(
    await page.locator("#quit").isEnabled(),
    "failed_quit_left_controls_disabled",
  );
  check(child.exitCode === null, "failed_quit_closed_app");
  await page.unroute("**/api/quit");
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
